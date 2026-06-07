-- TigerSwap Extended Schema
-- Additional tables for blockchain management, wallets, bots, and white-label

-- ============================================================================
-- BLOCKCHAIN MANAGEMENT
-- ============================================================================

CREATE TABLE blockchains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id INTEGER UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    chain_type VARCHAR(20) NOT NULL, -- evm, solana, cosmos, aptos, sui, ton
    rpc_url TEXT,
    rpc_backup_url TEXT,
    explorer_url TEXT,
    explorer_api_url TEXT,
    chain_id_hex VARCHAR(20),
    native_currency_name VARCHAR(50),
    native_currency_symbol VARCHAR(20),
    native_currency_decimals INTEGER DEFAULT 18,
    is_active BOOLEAN DEFAULT true,
    is_paused BOOLEAN DEFAULT false,
    is_halted BOOLEAN DEFAULT false,
    confirmation_blocks INTEGER DEFAULT 12,
    block_time_seconds INTEGER DEFAULT 12,
    min_gas_price_gwei DECIMAL(20,2),
    max_gas_price_gwei DECIMAL(20,2),
    coingecko_chain_id VARCHAR(50),
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Pre-populated EVM Blockchains (20)
INSERT INTO blockchains (chain_id, name, symbol, chain_type, native_currency_name, native_currency_symbol, sort_order) VALUES
(1, 'Ethereum', 'ETH', 'evm', 'Ether', 'ETH', 1),
(56, 'BNB Chain', 'BNB', 'evm', 'BNB', 'BNB', 2),
(137, 'Polygon', 'MATIC', 'evm', 'MATIC', 'MATIC', 3),
(42161, 'Arbitrum One', 'ARB', 'evm', 'Ether', 'ETH', 4),
(10, 'Optimism', 'OP', 'evm', 'Ether', 'ETH', 5),
(8453, 'Base', 'BASE', 'evm', 'Ether', 'ETH', 6),
(43114, 'Avalanche C-Chain', 'AVAX', 'evm', 'Avalanche', 'AVAX', 7),
(25, 'Cronos', 'CRO', 'evm', 'Cronos', 'CRO', 8),
(42220, 'Celo', 'CELO', 'evm', 'Celo', 'CELO', 9),
(1666600000, 'Harmony', 'ONE', 'evm', 'ONE', 'ONE', 10),
(128, 'Huobi ECO Chain', 'HT', 'evm', 'Huobi Token', 'HT', 11),
(8217, 'Klaytn', 'KLAY', 'evm', 'Klaytn', 'KLAY', 12),
(42262, 'Oasis Emerald', 'ROSE', 'evm', 'Oasis', 'ROSE', 13),
(4689, 'IOTA', 'IOTA', 'evm', 'IOTA', 'IOTA', 14),
(1313161554, 'Aurora', 'AURORA', 'evm', 'Aurora', 'ETH', 15),
(1088, 'Metis', 'METIS', 'evm', 'Metis', 'METIS', 16),
(288, 'Boba Network', 'BOBA', 'evm', 'Boba', 'ETH', 17),
(106, ' Velas', 'VLX', 'evm', 'Velas', 'VLX', 18),
(1231, 'Ultron', 'ULX', 'evm', 'Ultron', 'ULX', 19),
(5700, 'Raydium', 'RAY', 'evm', 'Raydium', 'SOL', 20);

-- Pre-populated Non-EVM Blockchains (20)
INSERT INTO blockchains (chain_id, name, symbol, chain_type, native_currency_name, native_currency_symbol, sort_order) VALUES
(0, 'Solana', 'SOL', 'solana', 'Solana', 'SOL', 21),
(100, 'Cosmos', 'ATOM', 'cosmos', 'Cosmos', 'ATOM', 22),
(1100, 'Sui', 'SUI', 'sui', 'Sui', 'SUI', 23),
(1101, 'Aptos', 'APT', 'aptos', 'Aptos', 'APT', 24),
(-13, 'Tron', 'TRX', 'tron', 'Tron', 'TRX', 25),
(-3, 'Bitcoin', 'BTC', 'bitcoin', 'Bitcoin', 'BTC', 26),
(-5, 'Litecoin', 'LTC', 'litecoin', 'Litecoin', 'LTC', 27),
(-14, 'Dogecoin', 'DOGE', 'dogecoin', 'Dogecoin', 'DOGE', 28),
(-12, 'XRP', 'XRP', 'xrp', 'XRP', 'XRP', 29),
(2000, 'Near', 'NEAR', 'near', 'NEAR Protocol', 'NEAR', 30),
(3000, 'Hedera', 'HBAR', 'hedera', 'Hedera', 'HBAR', 31),
(4000, 'Algorand', 'ALGO', 'algorand', 'Algorand', 'ALGO', 32),
(5000, 'MultiversX', 'EGLD', 'multiversx', 'MultiversX', 'EGLD', 33),
(6000, 'Ton', 'TON', 'ton', 'Toncoin', 'TON', 34),
(7000, 'Kava', 'KAVA', 'kava', 'Kava', 'KAVA', 35),
(8000, 'Celestia', 'TIA', 'celestia', 'Celestia', 'TIA', 36),
(9000, 'Injective', 'INJ', 'injective', 'Injective', 'INJ', 37),
(10000, 'Sei', 'SEI', 'sei', 'Sei', 'SEI', 38),
(11000, 'Osmosis', 'OSMO', 'osmosis', 'Osmosis', 'OSMO', 39),
(12000, 'Dymension', 'DYM', 'dymension', 'Dymension', 'DYM', 40);

-- Blockchain tokens/coins
CREATE TABLE blockchain_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    blockchain_id UUID REFERENCES blockchains(id) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    contract_address VARCHAR(66),
    is_native BOOLEAN DEFAULT false,
    decimals INTEGER DEFAULT 18,
    coingecko_id VARCHAR(100),
    logo_url TEXT,
    is_verified BOOLEAN DEFAULT false,
    is_delisted BOOLEAN DEFAULT false,
    is_paused BOOLEAN DEFAULT false,
    price_usd DECIMAL(20,8) DEFAULT 0,
    market_cap DECIMAL(20,2),
    volume_24h DECIMAL(20,2),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(blockchain_id, contract_address)
);

-- Pre-populated top 50 tokens across all chains
INSERT INTO blockchain_tokens (blockchain_id, symbol, name, contract_address, is_native, decimals) 
SELECT b.id, 'ETH', 'Ether', NULL, true, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'USDC', 'USD Coin', '0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48', false, 6 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'USDT', 'Tether USD', '0xdac17f958d2ee523a2206206994597c13d831ec7', false, 6 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'WBTC', 'Wrapped BTC', '0x2260fac5e5542a773aa44fbcfedf7c193bc2c599', false, 8 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'DAI', 'Dai Stablecoin', '0x6b175474e89094c44da98b954eedeac495271d0f', false, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'LINK', 'Chainlink', '0x514910771af9ca656af840dff83e8264ecf986ca', false, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'UNI', 'Uniswap', '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984', false, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'AAVE', 'Aave', '0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9', false, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'MKR', 'Maker', '0x9f8f72aa9304c8b593d555f12ef6589cc3a579a2', false, 18 FROM blockchains b WHERE b.chain_id = 1
UNION ALL SELECT b.id, 'SNX', 'Synthetix', '0xc011a73ee8576fb46f5e1c5751ca3b9fe0af2a6f', false, 18 FROM blockchains b WHERE b.chain_id = 1;

-- ============================================================================
-- USER WALLETS (TigerWallet)
-- ============================================================================

CREATE TABLE user_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    wallet_id VARCHAR(64) UNIQUE NOT NULL,
    wallet_type VARCHAR(20) DEFAULT 'hd', -- hd, imported, ledger, trezor, multiSig
    
    -- HD Wallet derivation
    derivation_path VARCHAR(100),
    hd_key_index INTEGER DEFAULT 0,
    
    -- Security
    encrypted_seed_hash VARCHAR(255),
    is_encrypted BOOLEAN DEFAULT true,
    encryption_version INTEGER DEFAULT 1,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_locked BOOLEAN DEFAULT false,
    failed_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    
    -- Metadata
    nickname VARCHAR(100),
    color VARCHAR(20),
    emoji VARCHAR(10),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_used_at TIMESTAMP
);

-- Wallet addresses for each chain
CREATE TABLE wallet_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES user_wallets(id) ON DELETE CASCADE,
    blockchain_id UUID REFERENCES blockchains(id) NOT NULL,
    
    address VARCHAR(255) NOT NULL,
    public_key TEXT,
    private_key_encrypted TEXT, -- Encrypted private key
    
    derivation_index INTEGER DEFAULT 0,
    
    balance_main DECIMAL(20,8) DEFAULT 0,
    balance_usd DECIMAL(20,2) DEFAULT 0,
    
    is_default BOOLEAN DEFAULT false,
    is_ imported BOOLEAN DEFAULT false,
    is_hidden BOOLEAN DEFAULT false,
    
    label VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(wallet_id, blockchain_id, address)
);

-- Wallet token balances
CREATE TABLE wallet_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address_id UUID REFERENCES wallet_addresses(id) ON DELETE CASCADE,
    token_id UUID REFERENCES blockchain_tokens(id) NOT NULL,
    
    balance DECIMAL(20,8) DEFAULT 0,
    balance_usd DECIMAL(20,2) DEFAULT 0,
    raw_balance VARCHAR(50) DEFAULT '0',
    
    last_synced_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(wallet_address_id, token_id)
);

-- ============================================================================
-- MASTER WALLET (TigerMaster Admin Wallet)
-- ============================================================================

CREATE TABLE master_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id VARCHAR(64) UNIQUE NOT NULL,
    wallet_name VARCHAR(100) DEFAULT 'TigerMaster',
    
    -- Admin ownership
    admin_user_id UUID REFERENCES users(id),
    is_super_master BOOLEAN DEFAULT false,
    
    -- Security
    encrypted_seed_hash VARCHAR(255),
    backup_code_encrypted TEXT,
    is_encrypted BOOLEAN DEFAULT true,
    
    -- Revenue collection
    total_revenue_collected DECIMAL(20,2) DEFAULT 0,
    pending_revenue DECIMAL(20,2) DEFAULT 0,
    last_collection_at TIMESTAMP,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_paused BOOLEAN DEFAULT false,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Master wallet addresses
CREATE TABLE master_wallet_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_wallet_id UUID REFERENCES master_wallets(id) ON DELETE CASCADE,
    blockchain_id UUID REFERENCES blockchains(id) NOT NULL,
    
    address VARCHAR(255) NOT NULL,
    private_key_encrypted TEXT,
    
    is_primary BOOLEAN DEFAULT false,
    balance_main DECIMAL(20,8) DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(master_wallet_id, blockchain_id)
);

-- ============================================================================
-- FEE MANAGEMENT
-- ============================================================================

CREATE TABLE fee_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_code VARCHAR(50) UNIQUE NOT NULL,
    fee_name VARCHAR(100) NOT NULL,
    fee_category VARCHAR(50) NOT NULL, -- swap, trading, withdrawal, listing, bot, subscription
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO fee_types (fee_code, fee_name, fee_category) VALUES
('swap_fee', 'Swap Fee', 'swap'),
('trading_fee', 'Trading Fee', 'trading'),
('withdrawal_fee', 'Withdrawal Fee', 'withdrawal'),
('deposit_fee', 'Deposit Fee', 'deposit'),
('listing_fee', 'Token Listing Fee', 'listing'),
('bot_subscription', 'Bot Subscription Fee', 'bot'),
('api_access_fee', 'API Access Fee', 'api'),
('white_label_fee', 'White Label Fee', 'white_label'),
('mm_profit_share', 'MM Profit Share', 'mm');

CREATE TABLE fee_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_type_id UUID REFERENCES fee_types(id) NOT NULL,
    
    recipient_address VARCHAR(255) NOT NULL,
    recipient_type VARCHAR(20) DEFAULT 'wallet', -- wallet, treasury, bot
    chain_id INTEGER,
    
    percentage DECIMAL(5,2) DEFAULT 100, -- 0-100%
    fixed_amount DECIMAL(20,8) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    
    priority INTEGER DEFAULT 0, -- Lower = higher priority
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Per-chain, per-token fee overrides
CREATE TABLE chain_fee_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    blockchain_id UUID REFERENCES blockchains(id),
    token_id UUID REFERENCES blockchain_tokens(id),
    
    fee_type_id UUID REFERENCES fee_types(id) NOT NULL,
    
    percentage_override DECIMAL(5,2),
    fixed_override DECIMAL(20,8),
    
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- BOT PLATFORM
-- ============================================================================

CREATE TABLE bot_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_code VARCHAR(50) UNIQUE NOT NULL,
    bot_name VARCHAR(100) NOT NULL,
    bot_category VARCHAR(50) NOT NULL, -- mm, arbitrage, grid, dca, signal, copy
    description TEXT,
    icon_url TEXT,
    is_active BOOLEAN DEFAULT true,
    requires_subscription BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO bot_types (bot_code, bot_name, bot_category) VALUES
('mm_standard', 'Market Maker Bot', 'mm'),
('mm_advanced', 'Advanced MM Pro', 'mm'),
('arb_triangular', 'Triangular Arbitrage', 'arbitrage'),
('arb_cross_exchange', 'Cross-Exchange Arbitrage', 'arbitrage'),
('grid_spot', 'Grid Trading Spot', 'grid'),
('grid_perp', 'Grid Trading Perpetual', 'grid'),
('dca_classic', 'DCA Classic', 'dca'),
('dca_smart', 'DCA Smart', 'dca'),
('signal_alerts', 'Signal Alerts', 'signal'),
('copy_trading', 'Copy Trading', 'copy');

CREATE TABLE bot_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    bot_type_id UUID REFERENCES bot_types(id) NOT NULL,
    
    subscription_id VARCHAR(64) UNIQUE NOT NULL,
    status VARCHAR(20) DEFAULT 'active', -- active, paused, cancelled, expired
    
    -- Plan details
    plan_name VARCHAR(50),
    duration_days INTEGER,
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    
    -- Pricing
    subscription_fee DECIMAL(20,8),
    fee_token_id UUID REFERENCES blockchain_tokens(id),
    
    -- Performance
    total_pnl DECIMAL(20,2) DEFAULT 0,
    total_trades INTEGER DEFAULT 0,
    win_rate DECIMAL(5,2) DEFAULT 0,
    
    -- Settings
    settings JSONB DEFAULT '{}',
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Bot client management
CREATE TABLE bot_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    client_id VARCHAR(64) UNIQUE NOT NULL,
    
    client_name VARCHAR(100),
    client_type VARCHAR(30) DEFAULT 'standard', -- standard, premium, enterprise
    
    -- Permissions
    allowed_bot_types JSONB DEFAULT '[]',
    max_bots INTEGER DEFAULT 5,
    api_rate_limit INTEGER DEFAULT 100,
    
    -- Status
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, suspended, rejected
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP,
    
    -- API keys for bot access
    bot_api_key VARCHAR(64),
    bot_api_secret_encrypted TEXT,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- ADMIN MANAGEMENT
-- ============================================================================

CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES users(id),
    admin_id VARCHAR(64) UNIQUE NOT NULL,
    
    admin_level VARCHAR(20) NOT NULL, -- super_admin, admin, moderator, support
    admin_role VARCHAR(50),
    
    -- Permissions
    permissions JSONB DEFAULT '[]',
    feature_access JSONB DEFAULT '{}',
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_locked BOOLEAN DEFAULT false,
    last_login_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Admin action logs
CREATE TABLE admin_action_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES admin_users(id),
    action_type VARCHAR(50) NOT NULL,
    action_details JSONB,
    affected_entity_type VARCHAR(50),
    affected_entity_id UUID,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Feature flags per admin
CREATE TABLE admin_feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES admin_users(id) NOT NULL,
    feature_name VARCHAR(100) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    restrictions JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(admin_id, feature_name)
);

-- ============================================================================
-- API CONNECTIONS (CEX/DEX Integration)
-- ============================================================================

CREATE TABLE api_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    connection_type VARCHAR(20) NOT NULL, -- cex, dex, wallet, external_app
    
    connection_name VARCHAR(100),
    provider VARCHAR(50) NOT NULL, -- binance, coinbase, okx, uniswap, etc.
    
    api_key_encrypted TEXT,
    api_secret_encrypted TEXT,
    api_password_encrypted TEXT,
    passphrase_encrypted TEXT,
    
    permissions JSONB DEFAULT '[]',
    ip_whitelist TEXT[],
    
    is_active BOOLEAN DEFAULT true,
    last_sync_at TIMESTAMP,
    sync_status VARCHAR(20) DEFAULT 'idle',
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- WHITE LABEL CLIENT MANAGEMENT
-- ============================================================================

CREATE TABLE white_label_clients_ext (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(64) UNIQUE NOT NULL,
    client_name VARCHAR(100) NOT NULL,
    
    admin_user_id UUID REFERENCES users(id),
    
    -- Branding
    brand_name VARCHAR(100),
    brand_logo_url TEXT,
    brand_primary_color VARCHAR(20) DEFAULT '#FF6B35',
    brand_secondary_color VARCHAR(20) DEFAULT '#1A1A2E',
    brand_dark_mode_enabled BOOLEAN DEFAULT true,
    
    -- Domain & Hosting
    domain VARCHAR(255),
    deployment_status VARCHAR(20) DEFAULT 'pending',
    deployment_url TEXT,
    
    -- API Keys
    api_key VARCHAR(64),
    api_secret_encrypted TEXT,
    
    -- Revenue sharing (TigerSwap share)
    revenue_share_percentage DECIMAL(5,2) DEFAULT 20.00,
    revenue_share_active BOOLEAN DEFAULT true,
    
    -- Status
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, suspended, rejected
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP,
    
    -- Subscription
    subscription_tier VARCHAR(20) DEFAULT 'starter',
    subscription_expires_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- INDEXES
-- ============================================================================

CREATE INDEX idx_blockchains_chain_id ON blockchains(chain_id);
CREATE INDEX idx_blockchains_type ON blockchains(chain_type);
CREATE INDEX idx_blockchain_tokens_blockchain ON blockchain_tokens(blockchain_id);
CREATE INDEX idx_user_wallets_user ON user_wallets(user_id);
CREATE INDEX idx_wallet_addresses_wallet ON wallet_addresses(wallet_id);
CREATE INDEX idx_wallet_addresses_blockchain ON wallet_addresses(blockchain_id);
CREATE INDEX idx_wallet_balances_address ON wallet_balances(wallet_address_id);
CREATE INDEX idx_master_wallets_admin ON master_wallets(admin_user_id);
CREATE INDEX idx_fee_recipients_type ON fee_recipients(fee_type_id);
CREATE INDEX idx_bot_subscriptions_user ON bot_subscriptions(user_id);
CREATE INDEX idx_bot_subscriptions_status ON bot_subscriptions(status);
CREATE INDEX idx_bot_clients_status ON bot_clients(status);
CREATE INDEX idx_admin_users_level ON admin_users(admin_level);
CREATE INDEX idx_api_connections_user ON api_connections(user_id);
CREATE INDEX idx_white_label_clients_ext_status ON white_label_clients_ext(status);

-- ============================================================================
-- FUNCTIONS
-- ============================================================================

-- Calculate user wallet total balance in USD
CREATE OR REPLACE FUNCTION calculate_wallet_total_usd(p_wallet_id UUID)
RETURNS DECIMAL(20,2) AS $$
DECLARE
    v_total DECIMAL(20,2) := 0;
BEGIN
    SELECT COALESCE(SUM(balance_usd), 0) INTO v_total
    FROM wallet_balances wb
    JOIN wallet_addresses wa ON wb.wallet_address_id = wa.id
    WHERE wa.wallet_id = p_wallet_id;
    
    RETURN v_total;
END;
$$ LANGUAGE plpgsql;

-- Generate wallet ID
CREATE OR REPLACE FUNCTION generate_wallet_id()
RETURNS VARCHAR(64) AS $$
BEGIN
    RETURN 'TW_' || encode(gen_random_bytes(16), 'hex');
END;
$$ LANGUAGE plpgsql;

-- Get admin permissions
CREATE OR REPLACE FUNCTION get_admin_permissions(p_admin_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_permissions JSONB := '[]';
    v_level VARCHAR(20);
BEGIN
    SELECT admin_level INTO v_level FROM admin_users WHERE id = p_admin_id;
    
    -- Base permissions by level
    CASE v_level
        WHEN 'super_admin' THEN
            v_permissions := '["all", "users", "wallets", "tokens", "blockchains", "bots", "fees", "whitelabel", "api", "reports", "settings"]';
        WHEN 'admin' THEN
            v_permissions := '["users", "wallets", "tokens", "bots", "fees", "reports"]';
        WHEN 'moderator' THEN
            v_permissions := '["users", "reports"]';
        WHEN 'support' THEN
            v_permissions := '["users"]';
        ELSE
            v_permissions := '[]';
    END CASE;
    
    -- Add custom permissions
    SELECT COALESCE(jsonb_agg(aff.feature_name), '[]')
    INTO v_permissions
    FROM admin_feature_flags aff
    WHERE aff.admin_id = p_admin_id AND aff.is_enabled = true;
    
    RETURN v_permissions;
END;
$$ LANGUAGE plpgsql;

-- Check if white label revenue share applies
CREATE OR REPLACE FUNCTION calculate_revenue_share(
    p_client_id UUID,
    p_gross_revenue DECIMAL(20,2)
) RETURNS DECIMAL(20,2) AS $$
DECLARE
    v_share_percentage DECIMAL(5,2);
BEGIN
    SELECT revenue_share_percentage INTO v_share_percentage
    FROM white_label_clients_ext
    WHERE id = p_client_id AND revenue_share_active = true;
    
    IF v_share_percentage IS NULL THEN
        v_share_percentage := 0;
    END IF;
    
    RETURN p_gross_revenue * v_share_percentage / 100;
END;
$$ LANGUAGE plpgsql;

-- Collect fees to master wallet
CREATE OR REPLACE FUNCTION collect_fees_to_master(
    p_fee_type VARCHAR(50),
    p_amount DECIMAL(20,8),
    p_token_id UUID,
    p_recipient_address VARCHAR(255)
) RETURNS BOOLEAN AS $$
DECLARE
    v_fee_type_id UUID;
    v_recipient_id UUID;
BEGIN
    -- Get fee type
    SELECT id INTO v_fee_type_id FROM fee_types WHERE fee_code = p_fee_type;
    
    -- Get primary recipient
    SELECT id INTO v_recipient_id
    FROM fee_recipients
    WHERE fee_type_id = v_fee_type_id
      AND is_active = true
    ORDER BY priority ASC
    LIMIT 1;
    
    -- In production: Execute transfer to recipient address
    -- For now, log the collection
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;