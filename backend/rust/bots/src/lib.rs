//! TigerSwap self-built trading bots.
//!
//! Every bot here is an in-process strategy: it takes a [`market::MarketSnapshot`]
//! and returns [`market::BotOrder`]s for the caller to execute. Nothing reaches
//! out to an external venue, an Arrakis or Gamma vault, or a third-party bot
//! API, which is the zero-dependency requirement.
//!
//! Arithmetic is fixed-point throughout ([`fixed::Fixed`]) so a bot's decision
//! is identical on every host and never depends on a float rounding mode.
//!
//! ```
//! use tigerswap_bots::{Fixed, MarketSnapshot, MarketMaker, MarketMakerConfig, Level};
//!
//! let config = MarketMakerConfig {
//!     spread_bps: 10,
//!     target_depth: Fixed::parse("1.5").unwrap(),
//!     rebalance_threshold_bps: 10,
//!     min_spread_bps: 1,
//!     max_spread_bps: 500,
//!     min_book_depth: Fixed::ZERO,
//! };
//! let mut bot = MarketMaker::new(config).unwrap();
//!
//! let snapshot = MarketSnapshot::new(
//!     "BTC/RUSD",
//!     vec![Level::new(Fixed::parse("60000").unwrap(), Fixed::parse("2").unwrap())],
//!     vec![Level::new(Fixed::parse("60010").unwrap(), Fixed::parse("2").unwrap())],
//!     Fixed::parse("60005").unwrap(),
//!     1_700_000_000_000,
//! )
//! .unwrap();
//!
//! let orders = bot.quote(&snapshot).unwrap();
//! assert_eq!(orders.len(), 2);
//! ```

pub mod arbitrage;
pub mod fixed;
pub mod grid;
pub mod guard;
pub mod market;
pub mod market_maker;
pub mod schedule;
pub mod wide;

pub use arbitrage::{ArbitrageBot, ArbitrageConfig, Opportunity, Venue};
pub use fixed::{BotError, BotResult, Fixed, SCALE};
pub use grid::{GridBot, GridConfig};
pub use guard::{
    detect_sandwich, BorrowPosition, Liquidation, LiquidationBot, LiquidationConfig, NewListing,
    RebalanceBot, RebalanceConfig, RebalanceTrade, SandwichAlert, SniperBot, SniperConfig,
    SwapEvent, Target,
};
pub use market::{BotOrder, Level, MarketSnapshot, OrderKind, Side};
pub use market_maker::{MarketMaker, MarketMakerConfig};
pub use schedule::{DcaBot, DcaConfig, TwapBot, TwapConfig};
