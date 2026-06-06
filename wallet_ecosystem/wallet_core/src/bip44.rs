// ============================================================================
// BIP44 - Multi-Account Hierarchy for Deterministic Wallets
// Purpose, Coin Type, Account, Change, Address Index
// ============================================================================

/// BIP44 Purpose
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BIP44Purpose {
    Legacy = 44,          // BIP44 - Legacy
    SegWit = 49,         // BIP49 - P2SH-SegWit
    NativeSegWit = 84,    // BIP84 - Native SegWit
    Solflare = 777,       // Solflare custom
}

impl BIP44Purpose {
    pub fn from_value(value: u32) -> Option<Self> {
        match value {
            44 => Some(BIP44Purpose::Legacy),
            49 => Some(BIP44Purpose::SegWit),
            84 => Some(BIP44Purpose::NativeSegWit),
            777 => Some(BIP44Purpose::Solflare),
            _ => None,
        }
    }
    
    pub fn value(&self) -> u32 {
        *self as u32
    }
}

/// BIP44 Coin Types
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BIP44CoinType {
    Ethereum = 60,
    Bitcoin = 0,
    BitcoinCash = 145,
    Litecoin = 2,
    Dogecoin = 3,
    Ripple = 144,
    Tezos = 1729,
    Cosmos = 118,
    Osmosis = 118,
    Solana = 501,
    Aptos = 637,
    Sui = 784,
    Polygon = 966,
    Arbitrum = 1103,
    Optimism = 1114,
    Base = 8453,
    Avalanche = 9000,
    Fantom = 250,
    Cronos = 25,
    Kava = 2222,
    Mantle = 5000,
    zkSync = 324,
    Linea = 59144,
    Scroll = 534352,
}

impl BIP44CoinType {
    pub fn from_value(value: u32) -> Option<Self> {
        match value {
            0 => Some(BIP44CoinType::Bitcoin),
            2 => Some(BIP44CoinType::Litecoin),
            3 => Some(BIP44CoinType::Dogecoin),
            60 => Some(BIP44CoinType::Ethereum),
            145 => Some(BIP44CoinType::BitcoinCash),
            144 => Some(BIP44CoinType::Ripple),
            1729 => Some(BIP44CoinType::Tezos),
            118 => Some(BIP44CoinType::Cosmos),
            501 => Some(BIP44CoinType::Solana),
            637 => Some(BIP44CoinType::Aptos),
            784 => Some(BIP44CoinType::Sui),
            966 => Some(BIP44CoinType::Polygon),
            1103 => Some(BIP44CoinType::Arbitrum),
            1114 => Some(BIP44CoinType::Optimism),
            8453 => Some(BIP44CoinType::Base),
            9000 => Some(BIP44CoinType::Avalanche),
            250 => Some(BIP44CoinType::Fantom),
            25 => Some(BIP44CoinType::Cronos),
            2222 => Some(BIP44CoinType::Kava),
            5000 => Some(BIP44CoinType::Mantle),
            324 => Some(BIP44CoinType::zkSync),
            59144 => Some(BIP44CoinType::Linea),
            534352 => Some(BIP44CoinType::Scroll),
            _ => None,
        }
    }
    
    pub fn value(&self) -> u32 {
        *self as u32
    }
    
    pub fn symbol(&self) -> &'static str {
        match self {
            BIP44CoinType::Bitcoin => "BTC",
            BIP44CoinType::Litecoin => "LTC",
            BIP44CoinType::Dogecoin => "DOGE",
            BIP44CoinType::Ethereum => "ETH",
            BIP44CoinType::BitcoinCash => "BCH",
            BIP44CoinType::Ripple => "XRP",
            BIP44CoinType::Tezos => "XTZ",
            BIP44CoinType::Cosmos => "ATOM",
            BIP44CoinType::Solana => "SOL",
            BIP44CoinType::Aptos => "APT",
            BIP44CoinType::Sui => "SUI",
            BIP44CoinType::Polygon => "MATIC",
            BIP44CoinType::Arbitrum => "ETH",
            BIP44CoinType::Optimism => "ETH",
            BIP44CoinType::Base => "ETH",
            BIP44CoinType::Avalanche => "AVAX",
            BIP44CoinType::Fantom => "FTM",
            BIP44CoinType::Cronos => "CRO",
            BIP44CoinType::Kava => "KAVA",
            BIP44CoinType::Mantle => "MNT",
            BIP44CoinType::zkSync => "ETH",
            BIP44CoinType::Linea => "ETH",
            BIP44CoinType::Scroll => "ETH",
        }
    }
}

/// BIP44 Account
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BIP44Account {
    pub purpose: BIP44Purpose,
    pub coin_type: BIP44CoinType,
    pub account: u32,
}

impl BIP44Account {
    pub fn new(purpose: BIP44Purpose, coin_type: BIP44CoinType, account: u32) -> Self {
        BIP44Account { purpose, coin_type, account }
    }
    
    pub fn ethereum(account: u32) -> Self {
        BIP44Account {
            purpose: BIP44Purpose::Legacy,
            coin_type: BIP44CoinType::Ethereum,
            account,
        }
    }
    
    pub fn solana(account: u32) -> Self {
        BIP44Account {
            purpose: BIP44Purpose::Solflare,
            coin_type: BIP44CoinType::Solana,
            account,
        }
    }
    
    pub fn path(&self) -> String {
        format!("m/{}'/{}/{}'", 
            self.purpose.value(), 
            self.coin_type.value(), 
            self.account
        )
    }
    
    pub fn derive_change(&self, change: u32, index: u32) -> BIP44Change {
        BIP44Change {
            account: *self,
            change,
            index,
        }
    }
}

/// BIP44 Change (external/internal)
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BIP44Change {
    pub account: BIP44Account,
    pub change: u32,  // 0 = external, 1 = internal
    pub index: u32,
}

impl BIP44Change {
    pub fn external(&self) -> Self {
        BIP44Change { change: 0, ..*self }
    }
    
    pub fn internal(&self) -> Self {
        BIP44Change { change: 1, ..*self }
    }
    
    pub fn path(&self) -> String {
        format!("{}/{}/{}", 
            self.account.path(), 
            self.change, 
            self.index
        )
    }
}

/// BIP44 Address Index
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BIP44Index {
    pub purpose: BIP44Purpose,
    pub coin_type: BIP44CoinType,
    pub account: u32,
    pub change: u32,
    pub index: u32,
}

impl BIP44Index {
    pub fn from_path(path: &str) -> Option<Self> {
        let parts: Vec<&str> = path.split('/').collect();
        if parts.len() != 6 {
            return None;
        }
        
        let purpose: u32 = parts[1].trim_matches('\'').parse().ok()?;
        let coin: u32 = parts[2].trim_matches('\'').parse().ok()?;
        let account: u32 = parts[3].trim_matches('\'').parse().ok()?;
        let change: u32 = parts[4].parse().ok()?;
        let index: u32 = parts[5].parse().ok()?;
        
        Some(BIP44Index {
            purpose: BIP44Purpose::from_value(purpose)?,
            coin_type: BIP44CoinType::from_value(coin)?,
            account,
            change,
            index,
        })
    }
    
    pub fn path(&self) -> String {
        format!("m/{}/{}/{}/{}/{}", 
            self.purpose.value(),
            self.coin_type.value(),
            self.account,
            self.change,
            self.index
        )
    }
    
    /// Get Ethereum address path
    pub fn ethereum(index: u32) -> Self {
        BIP44Index {
            purpose: BIP44Purpose::Legacy,
            coin_type: BIP44CoinType::Ethereum,
            account: 0,
            change: 0,
            index,
        }
    }
    
    /// Get Solana address path
    pub fn solana(index: u32) -> Self {
        BIP44Index {
            purpose: BIP44Purpose::Solflare,
            coin_type: BIP44CoinType::Solana,
            account: 0,
            change: 0,
            index,
        }
    }
}