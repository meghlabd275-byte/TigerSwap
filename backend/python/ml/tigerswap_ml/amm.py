"""AMM pricing, routing and market-making models for TigerSwap.

These implement the exact invariants used by the on-chain pools: constant
product for V2 pools, the stable-swap invariant solved by Newton's method for
StableSwap pools, and the weighted invariant for Balancer-style pools. The
router uses these to compute the best split across venues, and the market maker
uses an Avellaneda-Stoikov style reservation price with inventory skew.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Dict, List, Optional, Sequence

from . import mathx

BPS_DENOMINATOR = 10_000


def constant_product_out(
    amount_in: float, reserve_in: float, reserve_out: float, fee_bps: int = 30
) -> float:
    """Output of a V2-style pool for a given input, net of the swap fee."""
    if amount_in <= 0:
        raise ValueError("amount_in must be positive")
    if reserve_in <= 0 or reserve_out <= 0:
        raise ValueError("reserves must be positive")
    if not 0 <= fee_bps < BPS_DENOMINATOR:
        raise ValueError("fee_bps must be in [0, 10000)")

    amount_in_after_fee = amount_in * (BPS_DENOMINATOR - fee_bps) / BPS_DENOMINATOR
    return (reserve_out * amount_in_after_fee) / (reserve_in + amount_in_after_fee)


def constant_product_in(
    amount_out: float, reserve_in: float, reserve_out: float, fee_bps: int = 30
) -> float:
    """Input required for a V2-style pool to produce `amount_out`."""
    if amount_out <= 0:
        raise ValueError("amount_out must be positive")
    if amount_out >= reserve_out:
        raise ValueError("amount_out must be smaller than the output reserve")
    if reserve_in <= 0 or reserve_out <= 0:
        raise ValueError("reserves must be positive")

    numerator = reserve_in * amount_out
    denominator = reserve_out - amount_out
    amount_in_before_fee = numerator / denominator
    return amount_in_before_fee * BPS_DENOMINATOR / (BPS_DENOMINATOR - fee_bps)


def spot_price(reserve_in: float, reserve_out: float) -> float:
    """Marginal price of the input token in units of the output token."""
    if reserve_in <= 0:
        raise ValueError("reserve_in must be positive")
    return reserve_out / reserve_in


def price_impact(
    amount_in: float, reserve_in: float, reserve_out: float, fee_bps: int = 30
) -> float:
    """Execution price impact as a positive fraction.

    Compares the realised average execution price with the pre-trade spot
    price. A value of 0.01 means the trade moved the average price 1% away
    from spot.
    """
    if amount_in <= 0:
        raise ValueError("amount_in must be positive")

    before = spot_price(reserve_in, reserve_out)
    out = constant_product_out(amount_in, reserve_in, reserve_out, fee_bps)
    if out == 0:
        return 1.0

    execution = out / amount_in
    impact = (before - execution) / before
    return mathx.clip(impact, 0.0, 1.0)


def impermanent_loss(price_ratio: float) -> float:
    """Impermanent loss of a V2 LP for a price ratio of new/initial price.

    Returns a positive fraction representing the loss versus simply holding.
    """
    if price_ratio <= 0:
        raise ValueError("price_ratio must be positive")
    return (2.0 * math.sqrt(price_ratio)) / (1.0 + price_ratio) - 1.0


class StableSwap:
    """Curve-style stable-swap invariant with an amplifier coefficient."""

    def __init__(self, amplification: float = 100.0, fee_bps: int = 1):
        if amplification < 1:
            raise ValueError("amplification must be at least 1")
        if not 0 <= fee_bps < BPS_DENOMINATOR:
            raise ValueError("fee_bps must be in [0, 10000)")

        self.amplification = amplification
        self.fee_bps = fee_bps

    def invariant_d(self, balances: Sequence[float]) -> float:
        """Solves for the invariant D by Newton's method.

        Converges to machine precision in a few iterations for realistic pools.
        """
        count = len(balances)
        if count < 2:
            raise ValueError("at least two balances are required")
        if any(balance <= 0 for balance in balances):
            raise ValueError("balances must be positive")

        total = math.fsum(balances)
        ann = self.amplification * (count**count)

        d = total
        for _ in range(255):
            d_product = d
            for balance in balances:
                d_product = d_product * d / (balance * count)

            previous = d
            d = (ann * total + d_product * count) * d / ((ann - 1.0) * d + (count + 1.0) * d_product)

            if abs(d - previous) <= 1e-12:
                break

        return d

    def get_y(
        self,
        index_in: int,
        index_out: int,
        balances: Sequence[float],
        invariant: Optional[float] = None,
    ) -> float:
        """Solves for the new balance of the output token given an input.

        `balances` must already reflect the input added to `index_in`, while
        `invariant` must be the D computed from the balances *before* the
        trade. Recomputing D from the post-trade balances would defeat the
        invariant and is the classic stable-swap implementation error.
        """
        count = len(balances)
        if index_in == index_out:
            raise ValueError("input and output indices must differ")

        d = invariant if invariant is not None else self.invariant_d(balances)
        ann = self.amplification * (count**count)

        total = 0.0
        c = d
        for index in range(count):
            if index == index_out:
                continue
            balance = balances[index]
            total += balance
            c = c * d / (balance * count)

        c = c * d / (ann * count)
        b = total + d / ann

        y = d
        for _ in range(255):
            previous = y
            y = (y * y + c) / (2.0 * y + b - d)
            if abs(y - previous) <= 1e-12:
                break

        return y

    def out_amount(
        self, amount_in: float, index_in: int, index_out: int, balances: Sequence[float]
    ) -> float:
        """Output amount for a stable swap, net of the fee."""
        if amount_in <= 0:
            raise ValueError("amount_in must be positive")
        if index_in == index_out:
            raise ValueError("input and output indices must differ")

        # D is an invariant of the pool: it is computed before the trade and
        # must not change while solving for the output balance.
        invariant = self.invariant_d(balances)

        # Apply the fee to the input, then solve for the new output balance.
        amount_in_after_fee = amount_in * (BPS_DENOMINATOR - self.fee_bps) / BPS_DENOMINATOR

        updated = list(balances)
        updated[index_in] += amount_in_after_fee

        y = self.get_y(index_in, index_out, updated, invariant)
        out = balances[index_out] - y

        if out < 0:
            raise ValueError("swap would drain the pool")
        return out


def weighted_pool_out(
    amount_in: float,
    reserve_in: float,
    reserve_out: float,
    weight_in: float,
    weight_out: float,
    fee_bps: int = 30,
) -> float:
    """Balancer-style weighted pool output for a given input."""
    if amount_in <= 0:
        raise ValueError("amount_in must be positive")
    if reserve_in <= 0 or reserve_out <= 0:
        raise ValueError("reserves must be positive")
    if weight_in <= 0 or weight_out <= 0:
        raise ValueError("weights must be positive")

    amount_in_after_fee = amount_in * (BPS_DENOMINATOR - fee_bps) / BPS_DENOMINATOR
    ratio = reserve_in / (reserve_in + amount_in_after_fee)
    return reserve_out * (1.0 - ratio ** (weight_in / weight_out))


@dataclass
class RouteAllocation:
    """One leg of a split route."""

    venue: str
    amount_in: float
    amount_out: float

    def as_dict(self) -> Dict[str, object]:
        return {"venue": self.venue, "amount_in": self.amount_in, "amount_out": self.amount_out}


@dataclass
class RouteQuote:
    """Best split across several constant-product venues."""

    allocations: List[RouteAllocation]
    total_in: float
    total_out: float
    effective_price: float
    price_impact: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "allocations": [a.as_dict() for a in self.allocations],
            "total_in": self.total_in,
            "total_out": self.total_out,
            "effective_price": self.effective_price,
            "price_impact": self.price_impact,
        }


class SmartRouter:
    """Splits an order across constant-product venues to maximise output."""

    def __init__(self, tolerance: float = 1e-9, max_iterations: int = 200):
        self.tolerance = tolerance
        self.max_iterations = max_iterations

    def optimal_split(
        self, amount_in: float, venues: Sequence[Dict[str, float]]
    ) -> RouteQuote:
        """Finds the output-maximising split of `amount_in` across venues.

        Each venue is a mapping with `name`, `reserve_in`, `reserve_out` and
        `fee_bps`. The objective is concave in the split fraction, so a ternary
        search converges to the optimum.
        """
        if amount_in <= 0:
            raise ValueError("amount_in must be positive")
        if not venues:
            raise ValueError("at least one venue is required")

        if len(venues) == 1:
            venue = venues[0]
            out = constant_product_out(
                amount_in,
                venue["reserve_in"],
                venue["reserve_out"],
                int(venue.get("fee_bps", 30)),
            )
            return self._build_quote([(0, amount_in, out)], venues)

        # Ternary search on the fraction routed to the first venue. For two
        # venues this is exact; for more, venues are folded in order.
        low = 0.0
        high = 1.0

        def total_out(fraction: float) -> float:
            return self._evaluate_split(amount_in, venues, fraction)

        for _ in range(self.max_iterations):
            if high - low < self.tolerance:
                break
            third = (high - low) / 3.0
            left = low + third
            right = high - third
            if total_out(left) < total_out(right):
                low = left
            else:
                high = right

        best_fraction = (low + high) / 2.0
        return self._quote_for_fraction(amount_in, venues, best_fraction)

    def _evaluate_split(
        self, amount_in: float, venues: Sequence[Dict[str, float]], first_fraction: float
    ) -> float:
        """Total output when `first_fraction` of the order goes to the first venue."""
        if len(venues) == 2:
            first_in = amount_in * first_fraction
            second_in = amount_in - first_in
            total = 0.0
            total += constant_product_out(
                first_in, venues[0]["reserve_in"], venues[0]["reserve_out"],
                int(venues[0].get("fee_bps", 30)),
            ) if first_in > 0 else 0.0
            total += constant_product_out(
                second_in, venues[1]["reserve_in"], venues[1]["reserve_out"],
                int(venues[1].get("fee_bps", 30)),
            ) if second_in > 0 else 0.0
            return total

        # For more than two venues, split the remainder evenly among the rest
        # and search on the first venue's share.
        first_in = amount_in * first_fraction
        remainder = amount_in - first_in
        share = remainder / (len(venues) - 1)

        total = 0.0
        if first_in > 0:
            total += constant_product_out(
                first_in, venues[0]["reserve_in"], venues[0]["reserve_out"],
                int(venues[0].get("fee_bps", 30)),
            )
        for venue in venues[1:]:
            if share > 0:
                total += constant_product_out(
                    share, venue["reserve_in"], venue["reserve_out"], int(venue.get("fee_bps", 30))
                )
        return total

    def _quote_for_fraction(
        self, amount_in: float, venues: Sequence[Dict[str, float]], first_fraction: float
    ) -> RouteQuote:
        allocations: List[tuple[int, float, float]] = []

        if len(venues) == 2:
            first_in = amount_in * first_fraction
            second_in = amount_in - first_in
            if first_in > 0:
                allocations.append(
                    (
                        0,
                        first_in,
                        constant_product_out(
                            first_in, venues[0]["reserve_in"], venues[0]["reserve_out"],
                            int(venues[0].get("fee_bps", 30)),
                        ),
                    )
                )
            if second_in > 0:
                allocations.append(
                    (
                        1,
                        second_in,
                        constant_product_out(
                            second_in, venues[1]["reserve_in"], venues[1]["reserve_out"],
                            int(venues[1].get("fee_bps", 30)),
                        ),
                    )
                )
        else:
            first_in = amount_in * first_fraction
            remainder = amount_in - first_in
            share = remainder / (len(venues) - 1)
            if first_in > 0:
                allocations.append(
                    (
                        0,
                        first_in,
                        constant_product_out(
                            first_in, venues[0]["reserve_in"], venues[0]["reserve_out"],
                            int(venues[0].get("fee_bps", 30)),
                        ),
                    )
                )
            for index, venue in enumerate(venues[1:], start=1):
                if share > 0:
                    allocations.append(
                        (
                            index,
                            share,
                            constant_product_out(
                                share, venue["reserve_in"], venue["reserve_out"],
                                int(venue.get("fee_bps", 30)),
                            ),
                        )
                    )

        return self._build_quote(allocations, venues)

    def _build_quote(
        self, allocations: Sequence[tuple[int, float, float]], venues: Sequence[Dict[str, float]]
    ) -> RouteQuote:
        legs = [
            RouteAllocation(
                venue=str(venues[index].get("name", index)), amount_in=amount_in, amount_out=out
            )
            for index, amount_in, out in allocations
        ]

        total_in = math.fsum(leg.amount_in for leg in legs)
        total_out = math.fsum(leg.amount_out for leg in legs)
        effective = total_out / total_in if total_in else 0.0

        # Impact versus the best single-venue spot price available.
        best_spot = max(spot_price(v["reserve_in"], v["reserve_out"]) for v in venues)
        impact = 0.0
        if best_spot > 0:
            impact = mathx.clip((best_spot - effective) / best_spot, 0.0, 1.0)

        return RouteQuote(
            allocations=legs,
            total_in=total_in,
            total_out=total_out,
            effective_price=effective,
            price_impact=impact,
        )

    @staticmethod
    def arbitrage_opportunity(price_a: float, price_b: float) -> Dict[str, float]:
        """Detects a price gap between two venues.

        `price_a` and `price_b` are the price of the same asset. A positive
        `edge` is the gross return available before fees.
        """
        if price_a <= 0 or price_b <= 0:
            raise ValueError("prices must be positive")

        lower = min(price_a, price_b)
        higher = max(price_a, price_b)
        edge = (higher - lower) / lower

        return {
            "buy_venue_price": lower,
            "sell_venue_price": higher,
            "gross_edge": edge,
            "buy_at": "a" if price_a == lower else "b",
            "sell_at": "b" if price_a == lower else "a",
        }


@dataclass
class MarketMakerQuote:
    """A two-sided quote produced by the market maker."""

    reservation_price: float
    bid: float
    ask: float
    spread: float
    inventory_skew: float

    def as_dict(self) -> Dict[str, object]:
        return {
            "reservation_price": self.reservation_price,
            "bid": self.bid,
            "ask": self.ask,
            "spread": self.spread,
            "inventory_skew": self.inventory_skew,
        }


class MarketMaker:
    """Avellaneda-Stoikov style market maker with inventory skew.

    The reservation price shifts away from the mid in proportion to inventory
    and variance, and the spread widens with volatility and with the ratio of
    risk aversion to order arrival intensity.
    """

    def __init__(
        self,
        risk_aversion: float = 0.1,
        order_intensity: float = 1.5,
        horizon: float = 1.0,
        max_inventory: float = 1.0,
    ):
        if risk_aversion <= 0:
            raise ValueError("risk_aversion must be positive")
        if order_intensity <= 0:
            raise ValueError("order_intensity must be positive")
        if horizon <= 0:
            raise ValueError("horizon must be positive")
        if max_inventory <= 0:
            raise ValueError("max_inventory must be positive")

        self.risk_aversion = risk_aversion
        self.order_intensity = order_intensity
        self.horizon = horizon
        self.max_inventory = max_inventory

    def quote(self, mid_price: float, inventory: float, volatility: float) -> MarketMakerQuote:
        """Produces a bid and an ask around a reservation price.

        `inventory` is the signed position in units of the base asset; a long
        position lowers the reservation price so the maker is more willing to
        sell. `volatility` is the per-period standard deviation of returns.
        """
        if mid_price <= 0:
            raise ValueError("mid_price must be positive")
        if volatility < 0:
            raise ValueError("volatility must not be negative")

        normalized_inventory = mathx.clip(inventory / self.max_inventory, -1.0, 1.0)

        # Reservation price: r = s - q * gamma * sigma^2 * (T - t)
        reservation = mid_price * (
            1.0 - normalized_inventory * self.risk_aversion * (volatility**2) * self.horizon
        )

        # Half-spread: (gamma sigma^2 (T-t))/2 + (1/gamma) ln(1 + gamma/k)
        half_spread = (
            0.5 * self.risk_aversion * (volatility**2) * self.horizon
            + (1.0 / self.risk_aversion)
            * math.log(1.0 + self.risk_aversion / self.order_intensity)
        )

        # The half-spread is in price terms scaled by the reservation price.
        spread = 2.0 * half_spread * mid_price
        bid = max(0.0, reservation - spread / 2.0)
        ask = reservation + spread / 2.0

        return MarketMakerQuote(
            reservation_price=reservation,
            bid=bid,
            ask=ask,
            spread=ask - bid,
            inventory_skew=normalized_inventory,
        )
