//! The liquidation, rebalance and sniper bots, plus sandwich detection.
//!
//! These three share a shape: they watch a condition and emit orders only when
//! it is met, so a quiet market produces no traffic.

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, MarketSnapshot, OrderKind, Side};

/// A borrow position the liquidation bot can act on.
#[derive(Clone, Debug)]
pub struct BorrowPosition {
    pub id: String,
    /// Value of the collateral in quote currency.
    pub collateral_value: Fixed,
    /// Outstanding debt in quote currency.
    pub debt_value: Fixed,
    /// The health factor below which the position may be liquidated.
    pub liquidation_threshold: Fixed,
    /// The bonus a liquidator earns, in basis points of the repaid debt.
    pub liquidation_bonus_bps: i64,
}

impl BorrowPosition {
    /// The health factor: collateral value times the threshold, over debt.
    ///
    /// A factor below one means the position is liquidatable.
    pub fn health_factor(&self) -> BotResult<Fixed> {
        if self.debt_value <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("position has no debt"));
        }
        let weighted = self.collateral_value.mul(self.liquidation_threshold)?;
        weighted.div(self.debt_value)
    }

    pub fn is_liquidatable(&self) -> BotResult<bool> {
        Ok(self.health_factor()? < Fixed::ONE)
    }
}

/// Operator settings for the liquidation bot.
#[derive(Clone, Debug)]
pub struct LiquidationConfig {
    /// Fraction of the debt to repay per liquidation, in basis points.
    pub close_factor_bps: i64,
    /// Refuse to liquidate when the bonus is below this, in basis points.
    pub min_bonus_bps: i64,
}

impl LiquidationConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.close_factor_bps <= 0 || self.close_factor_bps > 10_000 {
            return Err(BotError::InvalidConfig(
                "close_factor_bps must be within 1..=10000",
            ));
        }
        if self.min_bonus_bps < 0 {
            return Err(BotError::InvalidConfig("min_bonus_bps cannot be negative"));
        }
        Ok(())
    }
}

/// The liquidation bot.
#[derive(Debug)]
pub struct LiquidationBot {
    config: LiquidationConfig,
}

impl LiquidationBot {
    pub fn new(config: LiquidationConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self { config })
    }

    pub fn config(&self) -> &LiquidationConfig {
        &self.config
    }

    /// The debt to repay and the collateral seized for one position.
    ///
    /// Returns `None` when the position is healthy or the bonus is too thin.
    pub fn evaluate(&self, position: &BorrowPosition) -> BotResult<Option<Liquidation>> {
        if !position.is_liquidatable()? {
            return Ok(None);
        }
        if Fixed::bps(position.liquidation_bonus_bps)? < Fixed::bps(self.config.min_bonus_bps)? {
            return Ok(None);
        }

        let repay = position
            .debt_value
            .mul(Fixed::ratio(self.config.close_factor_bps as i128, 10_000)?)?;
        if repay <= Fixed::ZERO {
            return Ok(None);
        }

        let bonus = repay.mul(Fixed::ratio(position.liquidation_bonus_bps as i128, 10_000)?)?;
        let seize = repay.add(bonus)?;

        Ok(Some(Liquidation {
            position_id: position.id.clone(),
            repay_amount: repay,
            collateral_seized: seize,
            bonus_earned: bonus,
            health_factor: position.health_factor()?,
        }))
    }
}

/// A liquidation the bot wants executed.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Liquidation {
    pub position_id: String,
    pub repay_amount: Fixed,
    pub collateral_seized: Fixed,
    pub bonus_earned: Fixed,
    pub health_factor: Fixed,
}

/// A holding in the rebalance bot's target portfolio.
#[derive(Clone, Debug)]
pub struct Target {
    pub symbol: String,
    /// Share of the portfolio, in basis points. The set must total 10_000.
    pub weight_bps: i64,
    /// Current holding value in quote currency.
    pub value: Fixed,
}

/// Operator settings for the rebalance bot.
#[derive(Clone, Debug)]
pub struct RebalanceConfig {
    /// Only trade when a holding drifts this far from its target, in basis
    /// points of the total portfolio.
    pub drift_threshold_bps: i64,
}

impl RebalanceConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.drift_threshold_bps <= 0 {
            return Err(BotError::InvalidConfig(
                "drift_threshold_bps must be positive",
            ));
        }
        Ok(())
    }
}

/// The rebalance bot.
#[derive(Debug)]
pub struct RebalanceBot {
    config: RebalanceConfig,
}

impl RebalanceBot {
    pub fn new(config: RebalanceConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self { config })
    }

    /// Computes the trades needed to bring the portfolio back to its targets.
    ///
    /// Weights must total 10_000 basis points, so a mis-specified portfolio is
    /// rejected rather than silently normalised.
    pub fn plan(&self, holdings: &[Target]) -> BotResult<Vec<RebalanceTrade>> {
        if holdings.is_empty() {
            return Err(BotError::InvalidConfig("portfolio is empty"));
        }

        let total_weight: i64 = holdings.iter().map(|holding| holding.weight_bps).sum();
        if total_weight != 10_000 {
            return Err(BotError::InvalidConfig("target weights must total 100%"));
        }
        for holding in holdings {
            if holding.weight_bps < 0 {
                return Err(BotError::InvalidConfig("weights cannot be negative"));
            }
        }

        let mut total = Fixed::ZERO;
        for holding in holdings {
            total = total.add(holding.value)?;
        }
        if total <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("portfolio has no value"));
        }

        let mut trades = Vec::new();
        for holding in holdings {
            let target_value =
                total.mul(Fixed::ratio(holding.weight_bps as i128, 10_000)?)?;
            let drift = holding.value.sub(target_value)?;
            let drift_bps = drift.abs()?.div(total)?;

            if drift_bps < Fixed::bps(self.config.drift_threshold_bps)? {
                continue;
            }

            let side = if drift.is_positive() {
                Side::Sell
            } else {
                Side::Buy
            };

            trades.push(RebalanceTrade {
                symbol: holding.symbol.clone(),
                side,
                amount: drift.abs()?,
                drift_bps: drift_bps.to_bps()?,
            });
        }

        Ok(trades)
    }
}

/// One leg of a rebalance.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct RebalanceTrade {
    pub symbol: String,
    pub side: Side,
    pub amount: Fixed,
    pub drift_bps: i64,
}

/// Operator settings for the sniper bot.
#[derive(Clone, Debug)]
pub struct SniperConfig {
    /// Refuse to buy when the price impact of the order exceeds this.
    pub max_price_impact_bps: i64,
    /// Skip a launch whose liquidity is below this, in quote currency.
    pub min_liquidity: Fixed,
    /// Base quantity to buy on a qualifying launch.
    pub buy_size: Fixed,
}

impl SniperConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.max_price_impact_bps <= 0 || self.max_price_impact_bps > 10_000 {
            return Err(BotError::InvalidConfig(
                "max_price_impact_bps must be within 1..=10000",
            ));
        }
        if self.min_liquidity.is_negative() {
            return Err(BotError::InvalidConfig("min_liquidity cannot be negative"));
        }
        if !self.buy_size.is_positive() {
            return Err(BotError::InvalidConfig("buy_size must be positive"));
        }
        Ok(())
    }
}

/// A newly listed pool the sniper is watching.
#[derive(Clone, Debug)]
pub struct NewListing {
    pub symbol: String,
    pub snapshot: MarketSnapshot,
    /// Pool liquidity in quote currency.
    pub liquidity: Fixed,
}

/// The sniper bot.
///
/// The price-impact guard is the safety property that matters: a launch with no
/// depth is rejected rather than bought at any price.
#[derive(Debug)]
pub struct SniperBot {
    config: SniperConfig,
}

impl SniperBot {
    pub fn new(config: SniperConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self { config })
    }

    pub fn config(&self) -> &SniperConfig {
        &self.config
    }

    /// Returns a buy order when the listing clears both guards.
    pub fn evaluate(&self, listing: &NewListing) -> BotResult<Option<BotOrder>> {
        if listing.liquidity < self.config.min_liquidity {
            return Ok(None);
        }

        let impact = self.price_impact_bps(listing)?;
        if impact > self.config.max_price_impact_bps {
            return Ok(None);
        }

        Ok(Some(BotOrder::new(
            listing.symbol.clone(),
            Side::Buy,
            OrderKind::Market,
            Fixed::ZERO,
            self.config.buy_size,
            false,
            "sniper",
        )?))
    }

    /// The price impact of `buy_size` against the visible asks, in basis points.
    pub fn price_impact_bps(&self, listing: &NewListing) -> BotResult<i64> {
        let best_ask = listing
            .snapshot
            .best_ask()
            .ok_or(BotError::InvalidMarket("listing has no asks"))?;

        let mut remaining = self.config.buy_size;
        let mut quote_spent = Fixed::ZERO;
        let mut base_filled = Fixed::ZERO;

        for level in &listing.snapshot.asks {
            if remaining <= Fixed::ZERO {
                break;
            }
            let take = if level.quantity < remaining {
                level.quantity
            } else {
                remaining
            };
            quote_spent = quote_spent.add(take.mul(level.price)?)?;
            base_filled = base_filled.add(take)?;
            remaining = remaining.sub(take)?;
        }

        if base_filled <= Fixed::ZERO {
            return Err(BotError::InvalidMarket("listing has no depth"));
        }

        // The book cannot fill the whole order; the unfilled remainder would
        // move the price further, so the impact is unbounded and the bot must
        // refuse rather than guess.
        if remaining > Fixed::ZERO {
            return Ok(i64::MAX);
        }

        let average_price = quote_spent.div(base_filled)?;
        let impact = average_price.sub(best_ask)?.div(best_ask)?;
        impact.to_bps()
    }
}

/// A detected sandwich attack.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SandwichAlert {
    pub victim_tx: String,
    pub frontrun_tx: String,
    pub backrun_tx: String,
    /// The loss inflicted on the victim, in quote currency.
    pub victim_loss: Fixed,
}

/// Detects the classic three-transaction sandwich around a victim swap.
///
/// The check is structural: the front-run must sit in the same block as the
/// victim, buy on the same side, and be closed out by a back-run that returns
/// the attacker to their starting position.
pub fn detect_sandwich(block: &[SwapEvent]) -> BotResult<Vec<SandwichAlert>> {
    let mut alerts = Vec::new();

    for (index, victim) in block.iter().enumerate() {
        if victim.is_router_internal {
            continue;
        }

        let Some(frontrun) = block[..index]
            .iter()
            .rev()
            .find(|event| event.side == victim.side && event.sender != victim.sender)
        else {
            continue;
        };

        let Some(backrun) = block[index + 1..]
            .iter()
            .find(|event| {
                event.side == frontrun.side.opposite() && event.sender == frontrun.sender
            })
        else {
            continue;
        };

        // The attacker must end up ahead: the back-run sells at least what the
        // front-run bought.
        if backrun.amount_out < frontrun.amount_in {
            continue;
        }

        let victim_loss = victim
            .amount_in
            .sub(backrun.amount_in)?
            .max(Fixed::ZERO);

        alerts.push(SandwichAlert {
            victim_tx: victim.tx_hash.clone(),
            frontrun_tx: frontrun.tx_hash.clone(),
            backrun_tx: backrun.tx_hash.clone(),
            victim_loss,
        });
    }

    Ok(alerts)
}

/// One swap observed in a block, used by the sandwich detector.
#[derive(Clone, Debug)]
pub struct SwapEvent {
    pub tx_hash: String,
    pub sender: String,
    pub side: Side,
    /// Base quantity the sender paid in.
    pub amount_in: Fixed,
    /// Base quantity the sender received.
    pub amount_out: Fixed,
    /// True when the swap came from a TigerSwap router rather than a user.
    pub is_router_internal: bool,
}
