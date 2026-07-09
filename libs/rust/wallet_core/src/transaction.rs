//! Transaction types and utilities

use serde::{Deserialize, Serialize};

/// Generic transaction structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Transaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub from: String,
    pub to: String,
    pub value: String,
    pub data: String,
    pub gas_limit: u64,
    pub gas_price: u64,
    pub max_fee_per_gas: Option<u64>,
    pub max_priority_fee_per_gas: Option<u64>,
    pub signature: Option<String>,
    pub hash: Option<String>,
}

impl Transaction {
    /// Create new EVM transaction
    pub fn new_evm(to: String, value: String, data: String) -> Self {
        Self {
            chain_id: 1,
            nonce: 0,
            from: String::new(),
            to,
            value,
            data,
            gas_limit: 21000,
            gas_price: 0,
            max_fee_per_gas: None,
            max_priority_fee_per_gas: None,
            signature: None,
            hash: None,
        }
    }
    
    /// Set gas parameters
    pub fn set_gas(&mut self, limit: u64, price: u64) {
        self.gas_limit = limit;
        self.gas_price = price;
    }
    
    /// Encode as RLP (simplified)
    pub fn encode_rlp(&self) -> Vec<u8> {
        let mut encoded = Vec::new();
        
        // In production, use proper RLP encoding
        encoded.push(0); // Type 0 transaction
        
        encoded
    }
    
    /// Get transaction hash
    pub fn get_hash(&self) -> String {
        use sha2::{Digest, Sha256};
        
        let mut hasher = Sha256::new();
        hasher.update(self.encode_rlp());
        let result = hasher.finalize();
        
        format!("0x{}", hex::encode(result))
    }
}

/// Swap transaction for DEX
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapTransaction {
    pub path: Vec<String>,      // Token addresses
    pub amounts: Vec<String>,   // Amounts along path
    pub recipient: String,
    pub deadline: u64,
    pub fee: u32,
}

/// Transfer transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransferTransaction {
    pub token: String,          // Token address (0x0 for native)
    pub to: String,
    pub amount: String,
    pub data: Option<String>,
}

/// Order for orderbook trading
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub market_id: u32,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: String,
    pub quantity: String,
    pub time_in_force: TimeInForce,
    pub leverage: u8,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum OrderType {
    Limit,
    Market,
    StopLoss,
    StopLimit,
    TakeProfit,
    TakeProfitLimit,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TimeInForce {
    GTC,  // Good Till Cancel
    IOC,  // Immediate or Cancel
    FOK,  // Fill or Kill
    GTD,  // Good Till Date
}
