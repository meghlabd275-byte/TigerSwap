-- TigerSwap Complete Database Schema
-- Version: 1.0.0
-- All features and functionality with real operational logic

-- ============================================
-- USERS & AUTHENTICATION
-- ============================================

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    two_factor_secret VARCHAR(255),
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    login_otp VARCHAR(10),
    login_otp_expires_at TIMESTAMP,
    session_token VARCHAR(255),
    session_expires_at TIMESTAMP,
    failed_login_attempts INT DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    ip_whitelist TEXT[],
    api_key_whitelist TEXT[],
    referrer_id INT,
    white_label_id INT,
    is_white_label_admin BOOLEAN DEFAULT FALSE
);

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    session_token VARCHAR(255) UNIQUE NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

-- ============================================
-- ADMIN & PERMISSIONS
-- ============================================

CREATE TABLE admin_levels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    level INT UNIQUE NOT NULL,
    permissions TEXT[] NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE admin_permissions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    admin_level_id INT REFERENCES admin_levels(id),
    permissions TEXT[] NOT NULL,
    granted_by INT REFERENCES users(id),
    granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

-- ============================================
-- WALLETS
-- ============================================

CREATE TABLE wallets (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    wallet_type VARCHAR(50) NOT NULL,
    name VARCHAR(100),
    address VARCHAR(255) UNIQUE NOT NULL,
    chain VARCHAR(50) NOT NULL,
    chain_id INT NOT NULL,
    public_key TEXT,
    encrypted_private_key TEXT,
    seed_phrase_encrypted TEXT,
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE wallet_chain_addresses (
    id SERIAL PRIMARY KEY,
    wallet_id INT NOT NULL REFERENCES wallets(id),
    chain VARCHAR(50) NOT NULL,
    chain_id INT NOT NULL,
    address VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(wallet_id, chain)
);

-- ============================================
-- BLOCKCHAINS
-- ============================================

CREATE TABLE blockchains (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    chain_id VARCHAR(20) NOT NULL,
    type VARCHAR(20) NOT NULL,
    rpc_url TEXT NOT NULL,
    explorer_url TEXT,
    explorer_api_url TEXT,
    icon_url TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    is_evm BOOLEAN DEFAULT FALSE,
    decimals INT DEFAULT 18,
    gas_token_symbol VARCHAR(20),
    avg_gas_price BIGINT,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- TOKENS
-- ============================================

CREATE TABLE tokens (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    contract_address VARCHAR(255),
    chain VARCHAR(50) NOT NULL,
    chain_id INT NOT NULL,
    decimals INT DEFAULT 18,
    is_active BOOLEAN DEFAULT TRUE,
    is_native BOOLEAN DEFAULT FALSE,
    is_stablecoin BOOLEAN DEFAULT FALSE,
    logo_url TEXT,
    coingecko_id VARCHAR(100),
    price_feed_enabled BOOLEAN DEFAULT TRUE,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE token_prices (
    id SERIAL PRIMARY KEY,
    token_id INT NOT NULL REFERENCES tokens(id),
    price_usd DECIMAL(30, 10),
    volume_24h DECIMAL(30, 2),
    market_cap DECIMAL(30, 2),
    price_change_24h DECIMAL(10, 4),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- TRANSACTIONS
-- ============================================

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    wallet_id INT REFERENCES wallets(id),
    tx_type VARCHAR(50) NOT NULL,
    chain VARCHAR(50) NOT NULL,
    chain_id INT NOT NULL,
    from_address VARCHAR(255),
    to_address VARCHAR(255),
    amount VARCHAR(100),
    token VARCHAR(50),
    token_id INT REFERENCES tokens(id),
    tx_hash VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    gas_used BIGINT,
    gas_price BIGINT,
    gas_fee VARCHAR(50),
    nonce INT,
    block_number BIGINT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE swaps (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    from_token_id INT NOT NULL REFERENCES tokens(id),
    to_token_id INT NOT NULL REFERENCES tokens(id),
    from_amount VARCHAR(100) NOT NULL,
    to_amount VARCHAR(100),
    from_address VARCHAR(255),
    to_address VARCHAR(255),
    slippage_tolerance INT DEFAULT 50,
    route TEXT,
    tx_hash VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    fee_amount VARCHAR(50),
    fee_token_id INT REFERENCES tokens(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- FEES MANAGEMENT
-- ============================================

CREATE TABLE fee_addresses (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    address VARCHAR(255) NOT NULL,
    chain VARCHAR(50) NOT NULL,
    chain_id INT NOT NULL,
    percentage DECIMAL(5, 2) DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE fee_collections (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    fee_type VARCHAR(50) NOT NULL,
    amount VARCHAR(100) NOT NULL,
    token_id INT REFERENCES tokens(id),
    fee_address_id INT REFERENCES fee_addresses(id),
    tx_hash VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- TRADING FEES
-- ============================================

CREATE TABLE trading_fees (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    fee_type VARCHAR(50) NOT NULL,
    token_id INT REFERENCES tokens(id),
    percentage DECIMAL(5, 2) NOT NULL,
    flat_fee VARCHAR(50),
    min_amount VARCHAR(50),
    max_amount VARCHAR(50),
    is_active BOOLEAN DEFAULT TRUE,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- API KEYS
-- ============================================

CREATE TABLE api_keys (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    key_name VARCHAR(100),
    api_key VARCHAR(255) UNIQUE NOT NULL,
    api_secret VARCHAR(255) NOT NULL,
    permissions TEXT[] NOT NULL,
    rate_limit INT DEFAULT 1000,
    ip_whitelist TEXT[],
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- WHITE LABEL
-- ============================================

CREATE TABLE white_labels (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    brand_name VARCHAR(100) NOT NULL,
    domain VARCHAR(255),
    domain_verified BOOLEAN DEFAULT FALSE,
    custom_css TEXT,
    custom_js TEXT,
    logo_url TEXT,
    primary_color VARCHAR(20),
    secondary_color VARCHAR(20),
    revenue_share_percentage DECIMAL(5, 2) DEFAULT 20,
    status VARCHAR(50) DEFAULT 'pending',
    approved_by INT REFERENCES users(id),
    approved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE white_label_api_keys (
    id SERIAL PRIMARY KEY,
    white_label_id INT NOT NULL REFERENCES white_labels(id),
    api_key VARCHAR(255) UNIQUE NOT NULL,
    api_secret VARCHAR(255) NOT NULL,
    permissions TEXT[] NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- BOT PLATFORM
-- ============================================

CREATE TABLE bot_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    min_deposit VARCHAR(50),
    subscription_price DECIMAL(20, 2),
    subscription_token_id INT REFERENCES tokens(id),
    features TEXT[],
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bot_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    bot_type_id INT NOT NULL REFERENCES bot_types(id),
    white_label_id INT REFERENCES white_labels(id),
    status VARCHAR(50) DEFAULT 'active',
    start_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    end_date TIMESTAMP,
    auto_renew BOOLEAN DEFAULT FALSE,
    subscription_fee_paid DECIMAL(20, 2),
    payment_token_id INT REFERENCES tokens(id),
    payment_tx_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bot_instances (
    id SERIAL PRIMARY KEY,
    subscription_id INT NOT NULL REFERENCES bot_subscriptions(id),
    config JSONB NOT NULL,
    status VARCHAR(50) DEFAULT 'stopped',
    started_at TIMESTAMP,
    stopped_at TIMESTAMP,
    profit_loss VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- CEX CONNECTORS
-- ============================================

CREATE TABLE cex_connectors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    api_key_encrypted TEXT,
    api_secret_encrypted TEXT,
    passphrase_encrypted TEXT,
    subaccount_id VARCHAR(100),
    is_testnet BOOLEAN DEFAULT FALSE,
    permissions TEXT[],
    rate_limit INT DEFAULT 1200,
    is_active BOOLEAN DEFAULT TRUE,
    last_sync_at TIMESTAMP,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- DEX CONNECTORS
-- ============================================

CREATE TABLE dex_connectors (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    protocol VARCHAR(50) NOT NULL,
    router_address VARCHAR(255),
    factory_address VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    supported_chains TEXT[],
    created_by INT REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- LISTINGS & FEES
-- ============================================

CREATE TABLE token_listings (
    id SERIAL PRIMARY KEY,
    token_id INT NOT NULL REFERENCES tokens(id),
    listed_by INT REFERENCES users(id),
    listing_fee DECIMAL(20, 2),
    listing_fee_token_id INT REFERENCES tokens(id),
    listing_fee_tx_hash VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    approved_by INT REFERENCES users(id),
    approved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chain_listings (
    id SERIAL PRIMARY KEY,
    blockchain_id INT NOT NULL REFERENCES blockchains(id),
    listed_by INT REFERENCES users(id),
    listing_fee DECIMAL(20, 2),
    listing_fee_token_id INT REFERENCES tokens(id),
    listing_fee_tx_hash VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    approved_by INT REFERENCES users(id),
    approved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- AUDIT LOG
-- ============================================

CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id INT,
    details JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- INDEXES
-- ============================================

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_wallets_user ON wallets(user_id);
CREATE INDEX idx_wallets_address ON wallets(address);
CREATE INDEX idx_transactions_user ON transactions(user_id);
CREATE INDEX idx_transactions_hash ON transactions(tx_hash);
CREATE INDEX idx_swaps_user ON swaps(user_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_sessions_token ON sessions(session_token);
CREATE INDEX idx_api_keys_key ON api_keys(api_key);
CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at);

-- ============================================
-- SEQUENCES FOR IDs
-- ============================================

CREATE SEQUENCE IF NOT EXISTS users_id_seq;
CREATE SEQUENCE IF NOT EXISTS wallets_id_seq;
CREATE SEQUENCE IF NOT EXISTS transactions_id_seq;
CREATE SEQUENCE IF NOT EXISTS swaps_id_seq;
CREATE SEQUENCE IF NOT EXISTS api_keys_id_seq;