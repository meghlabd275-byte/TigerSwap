//! TigerSwap Admin Platform
//! 
//! Complete admin management system with:
//! - Super admin and role-based access
//! - Fee management
//! - Blockchain and token management
//! - White label system

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};

// ==================== USER TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum UserRole {
    SuperAdmin,
    Admin,
    BotManager,
    Trader,
    User,
    WhiteLabelAdmin,
    WhiteLabelClient,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum UserStatus {
    Active,
    Inactive,
    Suspended,
    Pending,
    Banned,
}

// ==================== USER ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: String,
    pub username: String,
    pub email: String,
    pub password_hash: String,
    pub role: UserRole,
    pub status: UserStatus,
    pub created_at: u64,
    pub last_login: u64,
    pub permissions: Vec<String>,
    pub white_label_id: Option<String>,
    pub api_keys: Vec<ApiKey>,
    pub two_factor_enabled: bool,
    pub kyc_verified: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiKey {
    pub id: String,
    pub key: String,
    pub name: String,
    pub permissions: Vec<String>,
    pub rate_limit: u32,
    pub created_at: u64,
    pub expires_at: Option<u64>,
    pub is_active: bool,
}

// ==================== FEE CONFIGURATION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeeConfig {
    pub id: String,
    pub fee_type: FeeType,
    pub chain: String,
    pub token: String,
    pub percentage: f64,      // 0-100
    pub fixed_amount: f64,
    pub min_fee: f64,
    pub max_fee: f64,
    pub recipient_address: String,
    pub is_active: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum FeeType {
    Swap,
    Trading,
    Withdrawal,
    Deposit,
    Listing,
    BotSubscription,
    NftTransfer,
    Staking,
    CrossChain,
    Network,
}

// ==================== WHITE LABEL ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WhiteLabel {
    pub id: String,
    pub name: String,
    pub domain: String,
    pub logo_url: String,
    pub primary_color: String,
    pub secondary_color: String,
    pub owner_id: String,
    pub status: WhiteLabelStatus,
    pub created_at: u64,
    pub fee_sharing_percentage: f64,  // 0-20%
    pub api_key: String,
    pub api_secret: String,
    pub is_active: bool,
    pub custom_domains: Vec<String>,
    pub features_enabled: Vec<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum WhiteLabelStatus {
    Pending,
    Active,
    Suspended,
    Terminated,
}

// ==================== PLATFORM STATS ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlatformStats {
    pub total_users: u64,
    pub total_volume_24h: f64,
    pub total_fees_collected: f64,
    pub total_white_labels: u64,
    pub total_bot_subscriptions: u64,
    pub active_users_24h: u64,
    pub total_transactions: u64,
}

// ==================== ADMIN MANAGER ====================

pub struct AdminManager {
    users: Arc<RwLock<HashMap<String, User>>>,
    fee_configs: Arc<RwLock<HashMap<String, FeeConfig>>>,
    white_labels: Arc<RwLock<HashMap<String, WhiteLabel>>>,
    super_admin_id: Arc<RwLock<Option<String>>>,
}

impl AdminManager {
    pub fn new() -> Self {
        Self {
            users: Arc::new(RwLock::new(HashMap::new())),
            fee_configs: Arc::new(RwLock::new(HashMap::new())),
            white_labels: Arc::new(RwLock::new(HashMap::new())),
            super_admin_id: Arc::new(RwLock::new(None)),
        }
    }

    // ==================== SUPER ADMIN ====================
    
    pub async fn create_super_admin(&self, username: String, email: String, password: String) -> Result<User, AdminError> {
        let user = User {
            id: Self::generate_id(),
            username,
            email,
            password_hash: Self::hash_password(&password),
            role: UserRole::SuperAdmin,
            status: UserStatus::Active,
            created_at: current_timestamp(),
            last_login: current_timestamp(),
            permissions: vec!["*".to_string()],
            white_label_id: None,
            api_keys: Vec::new(),
            two_factor_enabled: false,
            kyc_verified: true,
        };
        
        self.users.write().await.insert(user.id.clone(), user.clone());
        *self.super_admin_id.write().await = Some(user.id.clone());
        
        Ok(user)
    }

    pub async fn super_admin_login(&self, email: &str, password: &str) -> Result<User, AdminError> {
        let users = self.users.read().await;
        
        for user in users.values() {
            if user.email == email && user.password_hash == Self::hash_password(password) {
                if user.role != UserRole::SuperAdmin {
                    return Err(AdminError::Unauthorized);
                }
                return Ok(user.clone());
            }
        }
        
        Err(AdminError::InvalidCredentials)
    }

    // ==================== ADMIN MANAGEMENT ====================

    pub async fn create_admin(
        &self,
        super_admin_id: &str,
        username: String,
        email: String,
        password: String,
        role: UserRole,
    ) -> Result<User, AdminError> {
        // Verify super admin
        if !self.is_super_admin(super_admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let user = User {
            id: Self::generate_id(),
            username,
            email,
            password_hash: Self::hash_password(&password),
            role,
            status: UserStatus::Active,
            created_at: current_timestamp(),
            last_login: current_timestamp(),
            permissions: Self::get_default_permissions(role),
            white_label_id: None,
            api_keys: Vec::new(),
            two_factor_enabled: false,
            kyc_verified: false,
        };
        
        self.users.write().await.insert(user.id.clone(), user.clone());
        
        Ok(user)
    }

    pub async fn update_user_role(&self, admin_id: &str, user_id: &str, new_role: UserRole) -> Result<(), AdminError> {
        if !self.is_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut users = self.users.write().await;
        
        if let Some(user) = users.get_mut(user_id) {
            user.role = new_role;
            user.permissions = Self::get_default_permissions(new_role);
            Ok(())
        } else {
            Err(AdminError::UserNotFound)
        }
    }

    pub async fn suspend_user(&self, admin_id: &str, user_id: &str) -> Result<(), AdminError> {
        if !self.is_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut users = self.users.write().await;
        
        if let Some(user) = users.get_mut(user_id) {
            user.status = UserStatus::Suspended;
            Ok(())
        } else {
            Err(AdminError::UserNotFound)
        }
    }

    pub async fn grant_permission(&self, admin_id: &str, user_id: &str, permission: String) -> Result<(), AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut users = self.users.write().await;
        
        if let Some(user) = users.get_mut(user_id) {
            if !user.permissions.contains(&permission) {
                user.permissions.push(permission);
            }
            Ok(())
        } else {
            Err(AdminError::UserNotFound)
        }
    }

    // ==================== FEE MANAGEMENT ====================

    pub async fn set_fee(&self, admin_id: &str, config: FeeConfig) -> Result<(), AdminError> {
        if !self.is_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        self.fee_configs.write().await.insert(config.id.clone(), config);
        
        Ok(())
    }

    pub async fn update_fee_recipient(&self, admin_id: &str, fee_type: FeeType, address: String) -> Result<(), AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut configs = self.fee_configs.write().await;
        
        for config in configs.values_mut() {
            if config.fee_type == fee_type {
                config.recipient_address = address;
            }
        }
        
        Ok(())
    }

    pub async fn get_fees(&self) -> Vec<FeeConfig> {
        self.fee_configs.read().await.values().cloned().collect()
    }

    // ==================== WHITE LABEL ====================

    pub async fn create_white_label(
        &self,
        admin_id: &str,
        name: String,
        domain: String,
        owner_id: String,
    ) -> Result<WhiteLabel, AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let white_label = WhiteLabel {
            id: Self::generate_id(),
            name,
            domain,
            logo_url: String::new(),
            primary_color: "#FF6B35".to_string(),
            secondary_color: "#F7931A".to_string(),
            owner_id,
            status: WhiteLabelStatus::Pending,
            created_at: current_timestamp(),
            fee_sharing_percentage: 20.0,  // 20% by default
            api_key: Self::generate_api_key(),
            api_secret: Self::generate_api_secret(),
            is_active: false,
            custom_domains: Vec::new(),
            features_enabled: vec!["*".to_string()],
        };
        
        self.white_labels.write().await.insert(white_label.id.clone(), white_label.clone());
        
        Ok(white_label)
    }

    pub async fn approve_white_label(&self, admin_id: &str, white_label_id: &str) -> Result<(), AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut white_labels = self.white_labels.write().await;
        
        if let Some(wl) = white_labels.get_mut(white_label_id) {
            wl.status = WhiteLabelStatus::Active;
            wl.is_active = true;
            Ok(())
        } else {
            Err(AdminError::WhiteLabelNotFound)
        }
    }

    pub async fn suspend_white_label(&self, admin_id: &str, white_label_id: &str) -> Result<(), AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut white_labels = self.white_labels.write().await;
        
        if let Some(wl) = white_labels.get_mut(white_label_id) {
            wl.status = WhiteLabelStatus::Suspended;
            wl.is_active = false;
            Ok(())
        } else {
            Err(AdminError::WhiteLabelNotFound)
        }
    }

    pub async fn terminate_white_label(&self, admin_id: &str, white_label_id: &str) -> Result<(), AdminError> {
        if !self.is_super_admin(admin_id).await {
            return Err(AdminError::Unauthorized);
        }
        
        let mut white_labels = self.white_labels.write().await;
        
        if let Some(wl) = white_labels.get_mut(white_label_id) {
            wl.status = WhiteLabelStatus::Terminated;
            wl.is_active = false;
            Ok(())
        } else {
            Err(AdminError::WhiteLabelNotFound)
        }
    }

    pub async fn get_white_labels(&self) -> Vec<WhiteLabel> {
        self.white_labels.read().await.values().cloned().collect()
    }

    // ==================== PLATFORM STATS ====================

    pub async fn get_platform_stats(&self) -> PlatformStats {
        let users = self.users.read().await;
        
        PlatformStats {
            total_users: users.len() as u64,
            total_volume_24h: 0.0,
            total_fees_collected: 0.0,
            total_white_labels: self.white_labels.read().await.len() as u64,
            total_bot_subscriptions: 0,
            active_users_24h: 0,
            total_transactions: 0,
        }
    }

    // ==================== API KEYS ====================

    pub async fn create_api_key(&self, user_id: &str, name: String, permissions: Vec<String>) -> Result<ApiKey, AdminError> {
        let mut users = self.users.write().await;
        
        if let Some(user) = users.get_mut(user_id) {
            let api_key = ApiKey {
                id: Self::generate_id(),
                key: Self::generate_api_key(),
                name,
                permissions,
                rate_limit: 1000,
                created_at: current_timestamp(),
                expires_at: None,
                is_active: true,
            };
            
            user.api_keys.push(api_key.clone());
            
            Ok(api_key)
        } else {
            Err(AdminError::UserNotFound)
        }
    }

    pub async fn revoke_api_key(&self, user_id: &str, key_id: &str) -> Result<(), AdminError> {
        let mut users = self.users.write().await;
        
        if let Some(user) = users.get_mut(user_id) {
            for key in user.api_keys.iter_mut() {
                if key.id == key_id {
                    key.is_active = false;
                    return Ok(());
                }
            }
            Err(AdminError::ApiKeyNotFound)
        } else {
            Err(AdminError::UserNotFound)
        }
    }

    // ==================== HELPERS ====================

    async fn is_super_admin(&self, user_id: &str) -> bool {
        let users = self.users.read().await;
        
        if let Some(user) = users.get(user_id) {
            user.role == UserRole::SuperAdmin
        } else {
            false
        }
    }

    async fn is_admin(&self, user_id: &str) -> bool {
        let users = self.users.read().await;
        
        if let Some(user) = users.get(user_id) {
            matches!(user.role, UserRole::SuperAdmin | UserRole::Admin | UserRole::WhiteLabelAdmin)
        } else {
            false
        }
    }

    fn hash_password(password: &str) -> String {
        format!("hashed_{}", password)
    }

    fn get_default_permissions(role: UserRole) -> Vec<String> {
        match role {
            UserRole::SuperAdmin => vec!["*".to_string()],
            UserRole::Admin => vec!["manage_users".to_string(), "manage_fees".to_string()],
            UserRole::BotManager => vec!["manage_bots".to_string()],
            UserRole::Trader => vec!["trade".to_string()],
            UserRole::User => vec!["basic".to_string()],
            UserRole::WhiteLabelAdmin => vec!["whitelabel_manage".to_string()],
            UserRole::WhiteLabelClient => vec!["basic".to_string()],
        }
    }

    fn generate_id() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        
        format!("id_{}", timestamp)
    }

    fn generate_api_key() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        
        format!("ts_{}", timestamp)
    }

    fn generate_api_secret() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        
        format!("secret_{}", timestamp)
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AdminError {
    Unauthorized,
    InvalidCredentials,
    UserNotFound,
    WhiteLabelNotFound,
    ApiKeyNotFound,
    DuplicateUser,
}

impl std::fmt::Display for AdminError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            AdminError::Unauthorized => write!(f, "Unauthorized"),
            AdminError::InvalidCredentials => write!(f, "Invalid credentials"),
            AdminError::UserNotFound => write!(f, "User not found"),
            AdminError::WhiteLabelNotFound => write!(f, "White label not found"),
            AdminError::ApiKeyNotFound => write!(f, "API key not found"),
            AdminError::DuplicateUser => write!(f, "Duplicate user"),
        }
    }
}

// ==================== HELPER ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}