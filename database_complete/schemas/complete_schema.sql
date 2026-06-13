-- TigerSwap Complete Database Schema
-- PostgreSQL / MySQL Compatible

-- ============================================================================
-- Users Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(64) NOT NULL,
    wallet_address VARCHAR(100),
    wallet_type VARCHAR(50) DEFAULT 'internal',
    is_verified BOOLEAN DEFAULT FALSE,
    is_banned BOOLEAN DEFAULT FALSE,
    ban_reason TEXT,
    kyc_status VARCHAR(50) DEFAULT 'none',
    kyc_level INT DEFAULT 0,
    referral_code VARCHAR(50) UNIQUE,
    referred_by VARCHAR(64),
    total_volume DECIMAL(30, 8) DEFAULT 0,
    total_fees_paid DECIMAL(30, 8) DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_login BIGINT,
    INDEX idx_email (email),
    INDEX idx_wallet (wallet_address),
    INDEX idx_referral (referral_code)
);

-- ============================================================================
-- Admin Users Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS admins (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(64) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    permissions JSON,
    is_active BOOLEAN DEFAULT TRUE,
    is_super_admin BOOLEAN DEFAULT FALSE,
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(255),
    failed_login_attempts INT DEFAULT 0,
    locked_until BIGINT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_login BIGINT,
    INDEX idx_email (email),
    INDEX idx_role (role)
);

-- ============================================================================
-- Blockchains Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS blockchains (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    chain_id INT,
    type VARCHAR(50) NOT NULL,
    rpc_url TEXT NOT NULL,
    explorer_url TEXT,
    explorer_api_url TEXT,
    native_currency_name VARCHAR(100),
    native_currency_symbol VARCHAR(20),
    native_currency_decimals INT,
    native_currency_address VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    gas_limit INT DEFAULT 21000,
    gas_price_multiplier DECIMAL(5, 2) DEFAULT 1.0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_type (type),
    INDEX idx_chain_id (chain_id),
    INDEX idx_active (is_active)
);

-- ============================================================================
-- Tokens Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS tokens (
    id VARCHAR(64) PRIMARY KEY,
    blockchain_id VARCHAR(50) NOT NULL,
    address VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    decimals INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    total_supply DECIMAL(40, 0),
    coingecko_id VARCHAR(100),
    logo_url TEXT,
    website TEXT,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    is_whitelisted BOOLEAN DEFAULT FALSE,
    listing_fee_paid BOOLEAN DEFAULT FALSE,
    listing_fee_amount DECIMAL(30, 8) DEFAULT 0,
    listed_by VARCHAR(64),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    INDEX idx_blockchain (blockchain_id),
    INDEX idx_symbol (symbol),
    INDEX idx_active (is_active)
);

-- ============================================================================
-- Fee Configuration Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS fee_configs (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    value DECIMAL(20, 8) NOT NULL,
    min_value DECIMAL(20, 8),
    max_value DECIMAL(20, 8),
    recipient_address VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    updated_by VARCHAR(64),
    updated_at BIGINT NOT NULL,
    INDEX idx_type (type)
);

-- ============================================================================
-- White Label Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS white_labels (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255) UNIQUE NOT NULL,
    api_key VARCHAR(100) UNIQUE NOT NULL,
    api_secret VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    is_approved BOOLEAN DEFAULT FALSE,
    approved_by VARCHAR(64),
    approved_at BIGINT,
    owner_admin_id VARCHAR(64) NOT NULL,
    fee_sharing_percent INT DEFAULT 20,
    branding_logo TEXT,
    branding_primary_color VARCHAR(20),
    branding_secondary_color VARCHAR(20),
    custom_domain TEXT,
    cloud_provider VARCHAR(50),
    storage_provider VARCHAR(50),
    features JSON,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (owner_admin_id) REFERENCES admins(id),
    INDEX idx_domain (domain),
    INDEX idx_owner (owner_admin_id)
);

-- ============================================================================
-- Bot Clients Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS bot_clients (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    api_key VARCHAR(100) UNIQUE NOT NULL,
    api_secret VARCHAR(100) NOT NULL,
    bot_type VARCHAR(50) NOT NULL,
    subscription_tier VARCHAR(50) DEFAULT 'free',
    subscription_starts_at BIGINT,
    subscription_expires_at BIGINT,
    is_active BOOLEAN DEFAULT TRUE,
    is_approved BOOLEAN DEFAULT FALSE,
    approved_by VARCHAR(64),
    approved_at BIGINT,
    white_label_id VARCHAR(64),
    max_daily_volume DECIMAL(30, 8),
    max_position_size DECIMAL(30, 8),
    max_open_positions INT,
    permissions JSON,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (white_label_id) REFERENCES white_labels(id),
    INDEX idx_email (email),
    INDEX idx_type (bot_type),
    INDEX idx_white_label (white_label_id)
);

-- ============================================================================
-- External APIs Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS external_apis (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    api_key VARCHAR(100) UNIQUE NOT NULL,
    api_secret VARCHAR(100) NOT NULL,
    permissions JSON,
    rate_limit INT DEFAULT 100,
    is_active BOOLEAN DEFAULT TRUE,
    whitelisted_ips JSON,
    created_by VARCHAR(64),
    created_at BIGINT NOT NULL,
    last_used BIGINT,
    INDEX idx_type (type),
    INDEX idx_api_key (api_key)
);

-- ============================================================================
-- CEX Connectors Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS cex_connectors (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    api_key VARCHAR(255),
    api_secret_encrypted TEXT,
    passphrase_encrypted TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    is_connected BOOLEAN DEFAULT FALSE,
    last_sync BIGINT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_exchange (exchange),
    INDEX idx_active (is_active)
);

-- ============================================================================
-- DEX Connectors Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS dex_connectors (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    dex_type VARCHAR(50) NOT NULL,
    blockchain_id VARCHAR(50),
    router_address VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    is_connected BOOLEAN DEFAULT FALSE,
    last_sync BIGINT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    INDEX idx_dex_type (dex_type),
    INDEX idx_blockchain (blockchain_id)
);

-- ============================================================================
-- Wallets Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS wallets (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    address VARCHAR(100) NOT NULL,
    blockchain_id VARCHAR(50) NOT NULL,
    wallet_type VARCHAR(50) DEFAULT 'internal',
    is_primary BOOLEAN DEFAULT FALSE,
    label VARCHAR(100),
    created_at BIGINT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    UNIQUE INDEX idx_address_blockchain (address, blockchain_id),
    INDEX idx_user (user_id)
);

-- ============================================================================
-- Transactions Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    wallet_id VARCHAR(64),
    blockchain_id VARCHAR(50) NOT NULL,
    token_id VARCHAR(64),
    type VARCHAR(50) NOT NULL,
    hash VARCHAR(100),
    from_address VARCHAR(100),
    to_address VARCHAR(100),
    amount DECIMAL(40, 0) NOT NULL,
    fee DECIMAL(30, 0),
    status VARCHAR(50) DEFAULT 'pending',
    block_number BIGINT,
    created_at BIGINT NOT NULL,
    confirmed_at BIGINT,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (wallet_id) REFERENCES wallets(id),
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    FOREIGN KEY (token_id) REFERENCES tokens(id),
    INDEX idx_user (user_id),
    INDEX idx_type (type),
    INDEX idx_status (status),
    INDEX idx_hash (hash),
    INDEX idx_created (created_at)
);

-- ============================================================================
-- Swaps Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS swaps (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    from_blockchain_id VARCHAR(50) NOT NULL,
    to_blockchain_id VARCHAR(50) NOT NULL,
    from_token_id VARCHAR(64) NOT NULL,
    to_token_id VARCHAR(64) NOT NULL,
    from_amount DECIMAL(40, 0) NOT NULL,
    to_amount DECIMAL(40, 0),
    fee_amount DECIMAL(30, 0),
    fee_recipient VARCHAR(100),
    dex_used VARCHAR(100),
    status VARCHAR(50) DEFAULT 'pending',
    tx_hash VARCHAR(100),
    created_at BIGINT NOT NULL,
    completed_at BIGINT,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (from_blockchain_id) REFERENCES blockchains(id),
    FOREIGN KEY (to_blockchain_id) REFERENCES blockchains(id),
    INDEX idx_user (user_id),
    INDEX idx_status (status),
    INDEX idx_created (created_at)
);

-- ============================================================================
-- Orders Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    blockchain_id VARCHAR(50) NOT NULL,
    token_id VARCHAR(64) NOT NULL,
    order_type VARCHAR(50) NOT NULL,
    side VARCHAR(10) NOT NULL,
    price DECIMAL(40, 8) NOT NULL,
    amount DECIMAL(40, 0) NOT NULL,
    filled_amount DECIMAL(40, 0) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'open',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    expires_at BIGINT,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    FOREIGN KEY (token_id) REFERENCES tokens(id),
    INDEX idx_user (user_id),
    INDEX idx_status (status),
    INDEX idx_created (created_at)
);

-- ============================================================================
-- Audit Logs Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    admin_id VARCHAR(64),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(64),
    details JSON,
    ip_address VARCHAR(50),
    user_agent TEXT,
    timestamp BIGINT NOT NULL,
    FOREIGN KEY (admin_id) REFERENCES admins(id),
    INDEX idx_admin (admin_id),
    INDEX idx_action (action),
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_timestamp (timestamp)
);

-- ============================================================================
-- Sessions Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    session_type VARCHAR(50) NOT NULL,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user (user_id),
    INDEX idx_expires (expires_at)
);

-- ============================================================================
-- Bot Trades Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS bot_trades (
    id VARCHAR(64) PRIMARY KEY,
    bot_client_id VARCHAR(64) NOT NULL,
    blockchain_id VARCHAR(50) NOT NULL,
    token_a_id VARCHAR(64) NOT NULL,
    token_b_id VARCHAR(64) NOT NULL,
    side VARCHAR(10) NOT NULL,
    amount DECIMAL(40, 0) NOT NULL,
    price DECIMAL(40, 8),
    pnl DECIMAL(30, 8),
    status VARCHAR(50) DEFAULT 'pending',
    tx_hash VARCHAR(100),
    created_at BIGINT NOT NULL,
    completed_at BIGINT,
    FOREIGN KEY (bot_client_id) REFERENCES bot_clients(id),
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    INDEX idx_bot (bot_client_id),
    INDEX idx_created (created_at)
);

-- ============================================================================
-- Price Feeds Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS price_feeds (
    id VARCHAR(64) PRIMARY KEY,
    blockchain_id VARCHAR(50) NOT NULL,
    token_a_id VARCHAR(64) NOT NULL,
    token_b_id VARCHAR(64) NOT NULL,
    price DECIMAL(40, 16) NOT NULL,
    volume_24h DECIMAL(40, 8),
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (blockchain_id) REFERENCES blockchains(id),
    FOREIGN KEY (token_a_id) REFERENCES tokens(id),
    FOREIGN KEY (token_b_id) REFERENCES tokens(id),
    INDEX idx_pair (token_a_id, token_b_id),
    INDEX idx_updated (updated_at)
);

-- Insert default data
INSERT INTO fee_configs (id, name, type, value, is_active, updated_at) VALUES
('swap_fee', 'Swap Fee', 'swap_fee', '30', TRUE, UNIX_TIMESTAMP() * 1000),
('trading_fee', 'Trading Fee', 'trading_fee', '30', TRUE, UNIX_TIMESTAMP() * 1000),
('withdraw_fee', 'Withdrawal Fee', 'withdraw_fee', '10', TRUE, UNIX_TIMESTAMP() * 1000),
('deposit_fee', 'Deposit Fee', 'deposit_fee', '0', TRUE, UNIX_TIMESTAMP() * 1000),
('transfer_fee', 'Transfer Fee', 'transfer_fee', '5', TRUE, UNIX_TIMESTAMP() * 1000),
('listing_fee', 'Token Listing Fee', 'listing_fee', '1000000000000000000', TRUE, UNIX_TIMESTAMP() * 1000),
('bot_subscription_fee', 'Bot Subscription Fee', 'bot_subscription_fee', '100000000000000000', TRUE, UNIX_TIMESTAMP() * 1000),
('whitelabel_fee', 'White Label Fee', 'whitelabel_fee', '2000', TRUE, UNIX_TIMESTAMP() * 1000),
('api_key_fee', 'API Key Fee', 'api_key_fee', '50000000000000000', TRUE, UNIX_TIMESTAMP() * 1000),
('cross_chain_fee', 'Cross-Chain Fee', 'cross_chain_fee', '50', TRUE, UNIX_TIMESTAMP() * 1000);