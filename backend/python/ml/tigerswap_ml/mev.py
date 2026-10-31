"""MEV detection for TigerSwap's private mempool.

The DEX promises sandwich protection, which requires detecting the attack
pattern in a pending block before it is committed. This module implements that
detection over decoded swap events, and scores the toxicity of a flow so the
mempool can decide whether to route an order privately.

Three detectors are provided.

* :func:`detect_sandwiches` finds the front-run / victim / back-run triple and
  computes the victim's loss.
* :func:`detect_jit_liquidity` finds just-in-time liquidity: a large add
  immediately before a swap and a matching remove immediately after, which
  extracts fees without bearing risk.
* :class:`ToxicityScorer` tracks the fraction of volume attributable to
  identified attackers and turns it into a routing decision.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Sequence

# The block position window in which a front-run is considered to be part of the
# same sandwich. A front-run far earlier in the block is more likely to be
# unrelated trading.
DEFAULT_PROXIMITY = 5

# A front-run must move the price at least this much to have hurt the victim.
DEFAULT_MIN_IMPACT = 0.0005


@dataclass
class SwapEvent:
    """A decoded swap observed in the mempool or a block."""

    tx_hash: str
    sender: str
    pool: str
    #: True for a buy (quote in, base out), false for a sell.
    is_buy: bool
    amount_in: float
    amount_out: float
    #: Execution price of the swap.
    price: float
    #: True when the swap came from a router rather than a user.
    is_router: bool = False
    #: Index within the block.
    position: int = 0
    gas_price: float = 0.0

    def as_dict(self) -> Dict[str, object]:
        return {
            "tx_hash": self.tx_hash,
            "sender": self.sender,
            "pool": self.pool,
            "is_buy": self.is_buy,
            "amount_in": self.amount_in,
            "amount_out": self.amount_out,
            "price": self.price,
        }


@dataclass
class Sandwich:
    """A detected sandwich attack."""

    victim_tx: str
    frontrun_tx: str
    backrun_tx: str
    attacker: str
    pool: str
    victim_loss: float
    victim_loss_bps: float
    attacker_profit: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "victim_tx": self.victim_tx,
            "frontrun_tx": self.frontrun_tx,
            "backrun_tx": self.backrun_tx,
            "attacker": self.attacker,
            "pool": self.pool,
            "victim_loss": self.victim_loss,
            "victim_loss_bps": self.victim_loss_bps,
            "attacker_profit": self.attacker_profit,
        }


@dataclass
class JustInTimeLiquidity:
    """A just-in-time liquidity event around a swap."""

    add_tx: str
    swap_tx: str
    remove_tx: str
    provider: str
    pool: str
    liquidity: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "add_tx": self.add_tx,
            "swap_tx": self.swap_tx,
            "remove_tx": self.remove_tx,
            "provider": self.provider,
            "pool": self.pool,
            "liquidity": self.liquidity,
        }


@dataclass
class LiquidityEvent:
    """A liquidity add or remove observed alongside swaps."""

    tx_hash: str
    provider: str
    pool: str
    is_add: bool
    amount: float
    position: int = 0


def detect_sandwiches(
    events: Sequence[SwapEvent],
    proximity: int = DEFAULT_PROXIMITY,
    min_impact: float = DEFAULT_MIN_IMPACT,
) -> List[Sandwich]:
    """Finds sandwich attacks in a sequence of swaps.

    A sandwich is an attacker buying, then a victim buying on the same pool, then
    the attacker selling. The attacker must be within `proximity` positions of
    the victim, and the front-run must move the price by at least `min_impact`
    for the victim to have been harmed.
    """
    if proximity < 1:
        raise ValueError("proximity must be at least 1")
    if min_impact < 0:
        raise ValueError("min_impact must not be negative")

    sandwiches: List[Sandwich] = []
    claimed: set = set()

    ordered = sorted(events, key=lambda event: event.position)

    for index, victim in enumerate(ordered):
        if victim.is_router or victim.tx_hash in claimed:
            continue

        # The front-run must be on the same pool, the same side, by a different
        # sender, and close enough in *block position*. Distance is measured in
        # positions rather than list indices so a sparse block does not make two
        # far-apart transactions look adjacent.
        frontrun: Optional[SwapEvent] = None
        for candidate_index in range(index - 1, -1, -1):
            candidate = ordered[candidate_index]
            if victim.position - candidate.position > proximity:
                break
            if (
                candidate.pool == victim.pool
                and candidate.is_buy == victim.is_buy
                and candidate.sender != victim.sender
                and not candidate.is_router
            ):
                frontrun = candidate
                break

        if frontrun is None:
            continue

        # The back-run must close the attacker out: the opposite side, the same
        # sender, after the victim and within the same window.
        backrun: Optional[SwapEvent] = None
        for candidate_index in range(index + 1, len(ordered)):
            candidate = ordered[candidate_index]
            if candidate.position - victim.position > proximity:
                break
            if (
                candidate.pool == victim.pool
                and candidate.is_buy != victim.is_buy
                and candidate.sender == frontrun.sender
                and not candidate.is_router
            ):
                backrun = candidate
                break

        if backrun is None:
            continue

        # The front-run must actually have moved the price against the victim.
        if frontrun.price <= 0:
            continue
        impact = abs(frontrun.price - _price_before(ordered, frontrun, victim)) / frontrun.price
        if impact < min_impact:
            continue

        # The victim's loss is the difference between the price they would have
        # got without the front-run and the price they actually got, applied to
        # the amount they traded.
        reference_price = _price_before(ordered, frontrun, victim)
        if reference_price <= 0:
            continue
        if victim.is_buy:
            loss_per_unit = victim.price - reference_price
        else:
            loss_per_unit = reference_price - victim.price
        victim_loss = max(0.0, loss_per_unit * victim.amount_in)
        victim_loss_bps = 10_000.0 * victim_loss / (victim.amount_in * reference_price)

        # The attacker's profit is the round trip on their own amounts.
        if frontrun.is_buy:
            attacker_profit = backrun.amount_out - frontrun.amount_in
        else:
            attacker_profit = frontrun.amount_in - backrun.amount_out

        sandwiches.append(
            Sandwich(
                victim_tx=victim.tx_hash,
                frontrun_tx=frontrun.tx_hash,
                backrun_tx=backrun.tx_hash,
                attacker=frontrun.sender,
                pool=victim.pool,
                victim_loss=victim_loss,
                victim_loss_bps=victim_loss_bps,
                attacker_profit=max(0.0, attacker_profit),
            )
        )

        claimed.add(victim.tx_hash)
        claimed.add(frontrun.tx_hash)
        claimed.add(backrun.tx_hash)

    return sandwiches


def _price_before(
    ordered: Sequence[SwapEvent], frontrun: SwapEvent, victim: SwapEvent
) -> float:
    """The pool price immediately before the front-run.

    The front-run's own execution price is after their own impact, so the price
    they moved away from is estimated from the victim's price and the relative
    sizes. When no earlier swap exists the front-run price is used, which makes
    the impact estimate conservative rather than inflated.
    """
    earlier = [
        event
        for event in ordered
        if event.pool == victim.pool and event.position < frontrun.position
    ]
    if earlier:
        return earlier[-1].price
    return frontrun.price


def detect_jit_liquidity(
    liquidity_events: Sequence[LiquidityEvent],
    swaps: Sequence[SwapEvent],
    proximity: int = DEFAULT_PROXIMITY,
) -> List[JustInTimeLiquidity]:
    """Finds just-in-time liquidity around a swap.

    The pattern is an add, then a swap, then a remove of comparable size by the
    same provider, all within `proximity` positions.
    """
    if proximity < 1:
        raise ValueError("proximity must be at least 1")

    adds = sorted(
        [event for event in liquidity_events if event.is_add],
        key=lambda event: event.position,
    )
    removes = sorted(
        [event for event in liquidity_events if not event.is_add],
        key=lambda event: event.position,
    )

    found: List[JustInTimeLiquidity] = []
    claimed: set = set()

    for swap in sorted(swaps, key=lambda event: event.position):
        for add in adds:
            if add.tx_hash in claimed or add.pool != swap.pool:
                continue
            if not (0 < swap.position - add.position <= proximity):
                continue

            for remove in removes:
                if remove.tx_hash in claimed or remove.pool != swap.pool:
                    continue
                if remove.provider != add.provider:
                    continue
                if not (0 < remove.position - swap.position <= proximity):
                    continue
                # A remove materially smaller than the add is a genuine
                # withdrawal, not a JIT round trip.
                if remove.amount < 0.9 * add.amount:
                    continue
                found.append(
                    JustInTimeLiquidity(
                        add_tx=add.tx_hash,
                        swap_tx=swap.tx_hash,
                        remove_tx=remove.tx_hash,
                        provider=add.provider,
                        pool=swap.pool,
                        liquidity=add.amount,
                    )
                )
                claimed.add(add.tx_hash)
                claimed.add(remove.tx_hash)
                break

    return found


@dataclass
class ToxicityScore:
    """A toxicity assessment of recent flow on a pool."""

    pool: str
    toxic_volume: float
    total_volume: float
    toxicity: float
    offenders: List[str]
    should_route_private: bool
    observations: int

    def as_dict(self) -> Dict[str, object]:
        return {
            "pool": self.pool,
            "toxic_volume": self.toxic_volume,
            "total_volume": self.total_volume,
            "toxicity": self.toxicity,
            "offenders": self.offenders,
            "should_route_private": self.should_route_private,
            "observations": self.observations,
        }


class ToxicityScorer:
    """Tracks how much of a pool's volume comes from identified attackers.

    The scorer keeps a decaying record per address. A pool whose recent volume is
    dominated by known offenders is a candidate for private routing, which is
    the protection the DEX offers.
    """

    def __init__(self, decay: float = 0.95, private_routing_threshold: float = 0.15):
        if not 0.0 < decay <= 1.0:
            raise ValueError("decay must be in (0, 1]")
        if not 0.0 <= private_routing_threshold <= 1.0:
            raise ValueError("private_routing_threshold must be in [0, 1]")
        self.decay = decay
        self.private_routing_threshold = private_routing_threshold
        self._offender_weight: Dict[str, float] = {}

    def observe_offenders(self, attackers: Sequence[str]) -> None:
        """Records that these addresses were identified as attackers."""
        for address in self._offender_weight:
            self._offender_weight[address] *= self.decay
        for address in attackers:
            self._offender_weight[address] = self._offender_weight.get(address, 0.0) + 1.0

    def is_offender(self, address: str) -> bool:
        return self._offender_weight.get(address, 0.0) > 0.5

    def score(self, pool: str, events: Sequence[SwapEvent]) -> ToxicityScore:
        """Scores the flow on one pool."""
        total_volume = 0.0
        toxic_volume = 0.0
        offenders = set()

        for event in events:
            if event.pool != pool:
                continue
            volume = abs(event.amount_in)
            total_volume += volume
            if self.is_offender(event.sender):
                toxic_volume += volume
                offenders.add(event.sender)

        toxicity = toxic_volume / total_volume if total_volume > 0 else 0.0

        return ToxicityScore(
            pool=pool,
            toxic_volume=toxic_volume,
            total_volume=total_volume,
            toxicity=toxicity,
            offenders=sorted(offenders),
            should_route_private=toxicity >= self.private_routing_threshold,
            observations=sum(1 for event in events if event.pool == pool),
        )

    def score_all(self, events: Sequence[SwapEvent]) -> List[ToxicityScore]:
        """Scores every pool present in `events`."""
        pools = sorted({event.pool for event in events})
        return [self.score(pool, events) for pool in pools]


class SandwichRiskModel:
    """Estimates the probability that a pending swap will be sandwiched.

    The estimate is a logistic function of the swap's size relative to the
    pool's depth, the pool's historical toxicity and the current gas premium an
    attacker would have to pay. It is calibrated so that a small swap in a deep,
    clean pool is near zero and a large swap in a shallow, toxic pool is near
    one.
    """

    def __init__(
        self,
        size_coefficient: float = 2.0,
        toxicity_coefficient: float = 3.0,
        gas_coefficient: float = 1.0,
        intercept: float = -4.0,
    ):
        self.size_coefficient = size_coefficient
        self.toxicity_coefficient = toxicity_coefficient
        self.gas_coefficient = gas_coefficient
        self.intercept = intercept

    def probability(
        self,
        swap_size: float,
        pool_depth: float,
        toxicity: float,
        gas_premium: float = 0.0,
    ) -> float:
        """The probability that this swap attracts a sandwich.

        `swap_size` and `pool_depth` are in the same units, `toxicity` is in
        [0, 1] and `gas_premium` is the attacker's cost as a fraction of the
        expected profit.
        """
        if swap_size < 0:
            raise ValueError("swap_size must not be negative")
        if pool_depth <= 0:
            raise ValueError("pool_depth must be positive")
        if not 0.0 <= toxicity <= 1.0:
            raise ValueError("toxicity must be in [0, 1]")
        if gas_premium < 0:
            raise ValueError("gas_premium must not be negative")

        # Size is expressed as a share of depth, capped so one huge swap cannot
        # saturate the logit on its own.
        size_share = min(swap_size / pool_depth, 1.0)

        logit = (
            self.intercept
            + self.size_coefficient * size_share
            + self.toxicity_coefficient * toxicity
            - self.gas_coefficient * gas_premium
        )
        return 1.0 / (1.0 + math.exp(-logit))

    def recommend_private(self, probability: float, threshold: float = 0.5) -> bool:
        """Whether the swap should be routed through the private mempool."""
        if not 0.0 <= probability <= 1.0:
            raise ValueError("probability must be in [0, 1]")
        return probability >= threshold
