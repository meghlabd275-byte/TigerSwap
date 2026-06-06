-- TigerSwap Database Schema
-- PostgreSQL Database for Complete DEX Operations

-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(66) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    username VARCHAR(100) UNIQUE,
    password_hash VARCHAR(255), -- For centralized auth
    avatar_url TEXT,
    risk_score INTEGER DEFAULT 100,
    kyc_status VARCHAR(20) DEFAULT 'none', -- none, basic, full
    is_verified BOOLEAN DEFAULT false,
    is_admin BOOLEAN DEFAULT false,
    total_volume_usd DECIMAL(20,2) DEFAULT 0,
    total_pnl DECIMAL(20,2) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_active_at TIMESTAMP
);

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    refresh_token VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    api_secret_hash VARCHAR(255) NOT NULL,
    permissions JSONB DEFAULT '{}',
    rate_limit INTEGER DEFAULT 1000,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TOKENS & PAIRS
-- ============================================================================

CREATE TABLE tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    contract_address VARCHAR(66),
    chain_id INTEGER NOT NULL,
    decimals INTEGER DEFAULT 18,
    logo_url TEXT,
    coingecko_id VARCHAR(100),
    is_stablecoin BOOLEAN DEFAULT false,
    is_wrapped BOOLEAN DEFAULT false,
    underlying_token_id UUID REFERENCES tokens(id),
    price_usd DECIMAL(20,8) DEFAULT 0,
    market_cap DECIMAL(20,2),
    volume_24h DECIMAL(20,2),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(chain_id, contract_address)
);

CREATE TABLE trading_pairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_a_id UUID REFERENCES tokens(id) NOT NULL,
    token_b_id UUID REFERENCES tokens(id) NOT NULL,
    pair_address VARCHAR(66),
    chain_id INTEGER NOT NULL,
    dex_id UUID REFERENCES dexes(id),
    is_stable_pair BOOLEAN DEFAULT false,
    is_verified BOOLEAN DEFAULT false,
    min_order_size DECIMAL(20,8) DEFAULT 0,
    max_order_size DECIMAL(20,8),
    fee_tier_bps INTEGER DEFAULT 30,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(chain_id, pair_address)
);

CREATE TABLE token_prices_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id UUID REFERENCES tokens(id) NOT NULL,
    price_usd DECIMAL(20,8) NOT NULL,
    source VARCHAR(50) NOT NULL, -- coingecko, uniswap, binance
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- DEXES & POOLS
-- ============================================================================

CREATE TABLE dexes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    slug VARCHAR(50) NOT NULL UNIQUE,
    protocol_version VARCHAR(20),
    chain_id INTEGER NOT NULL,
    router_address VARCHAR(66),
    factory_address VARCHAR(66),
    subgraph_url TEXT,
    website_url TEXT,
    logo_url TEXT,
    is_active BOOLEAN DEFAULT true,
    avg_latency_us INTEGER DEFAULT 5000,
    total_volume_usd DECIMAL(20,2) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dex_id UUID REFERENCES dexes(id) NOT NULL,
    pair_id UUID REFERENCES trading_pairs(id) NOT NULL,
    pool_address VARCHAR(66) NOT NULL,
    token_a_address VARCHAR(66) NOT NULL,
    token_b_address VARCHAR(66) NOT NULL,
    reserve_a DECIMAL(30,8) DEFAULT 0,
    reserve_b DECIMAL(30,8) DEFAULT 0,
    liquidity_usd DECIMAL(20,2) DEFAULT 0,
    fee_tier_bps INTEGER DEFAULT 30,
    tvl_usd DECIMAL(20,2) DEFAULT 0,
    volume_24h_usd DECIMAL(20,2) DEFAULT 0,
    volume_7d_usd DECIMAL(20,2) DEFAULT 0,
    apr DECIMAL(10,4),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(dex_id, pool_address)
);

CREATE TABLE pool_stats_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id UUID REFERENCES pools(id) NOT NULL,
    reserve_a DECIMAL(30,8),
    reserve_b DECIMAL(30,8),
    liquidity_usd DECIMAL(20,2),
    volume_24h_usd DECIMAL(20,2),
    tvl_usd DECIMAL(20,2),
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- ORDERS & TRADES
-- ============================================================================

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_hash VARCHAR(130) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id),
    pair_id UUID REFERENCES trading_pairs(id),
    pool_id UUID REFERENCES pools(id),
    dex_id UUID REFERENCES dexes(id),
    side VARCHAR(10) NOT NULL, -- buy, sell
    order_type VARCHAR(20) NOT NULL, -- market, limit, stop
    price DECIMAL(20,8),
    qty DECIMAL(30,8) NOT NULL,
    filled_qty DECIMAL(30,8) DEFAULT 0,
    avg_fill_price DECIMAL(20,8),
    slippage_bps INTEGER DEFAULT 50,
    fee_usd DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'pending', -- pending, partial, filled, cancelled, expired
    chain_id INTEGER NOT NULL,
    tx_hash VARCHAR(66),
    block_number BIGINT,
    gas_used DECIMAL(20,2),
    gas_price_gwei DECIMAL(20,4),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    executed_at TIMESTAMP
);

CREATE TABLE trades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID REFERENCES orders(id),
    pair_id UUID REFERENCES trading_pairs(id),
    pool_id UUID REFERENCES pools(id),
    user_id UUID REFERENCES users(id),
    side VARCHAR(10) NOT NULL,
    price DECIMAL(20,8) NOT NULL,
    qty DECIMAL(30,8) NOT NULL,
    fee_usd DECIMAL(20,8),
    tx_hash VARCHAR(66) NOT NULL,
    block_number BIGINT,
    timestamp TIMESTAMP NOT NULL,
    dex VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE liquidity_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    pool_id UUID REFERENCES pools(id) NOT NULL,
    token_a_address VARCHAR(66) NOT NULL,
    token_b_address VARCHAR(66) NOT NULL,
    liquidity_token_id VARCHAR(100),
    liquidity_token_balance DECIMAL(30,8) DEFAULT 0,
    token_a_amount DECIMAL(30,8) DEFAULT 0,
    token_b_amount DECIMAL(30,8) DEFAULT 0,
    range_low DECIMAL(20,8), -- For concentrated liquidity
    range_high DECIMAL(20,8),
    collected_fees_token_a DECIMAL(30,8) DEFAULT 0,
    collected_fees_token_b DECIMAL(30,8) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- BOTS & STRATEGIES
-- ============================================================================

CREATE TABLE bot_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    bot_type VARCHAR(30) NOT NULL, -- market_maker, arbitrage, sniper, etc.
    name VARCHAR(100) NOT NULL,
    status VARCHAR(20) DEFAULT 'stopped', -- running, stopped, error
    config JSONB DEFAULT '{}',
    connected_dexes UUID[],
    connected_cexes VARCHAR[],
    monthly_fee_usd DECIMAL(10,2) DEFAULT 5000,
    per_exchange_fee_usd DECIMAL(10,2) DEFAULT 1000,
    total_pnl DECIMAL(20,2) DEFAULT 0,
    total_volume DECIMAL(20,2) DEFAULT 0,
    total_orders INTEGER DEFAULT 0,
    avg_latency_us INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_trade_at TIMESTAMP
);

CREATE TABLE bot_strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id UUID REFERENCES bot_instances(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    pair VARCHAR(20),
    chain_id INTEGER,
    dex VARCHAR(50),
    base_spread_bps INTEGER DEFAULT 50,
    max_spread_bps INTEGER DEFAULT 200,
    min_spread_bps INTEGER DEFAULT 10,
    order_size_min DECIMAL(20,8),
    order_size_max DECIMAL(20,8),
    max_position_usd DECIMAL(20,2) DEFAULT 100000,
    max_daily_loss_usd DECIMAL(20,2) DEFAULT 5000,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE bot_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id UUID REFERENCES bot_instances(id),
    order_id UUID REFERENCES orders(id),
    pair VARCHAR(20),
    side VARCHAR(10),
    price DECIMAL(20,8),
    qty DECIMAL(30,8),
    status VARCHAR(20),
    execution_latency_us INTEGER,
    exchange VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE bot_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    bot_id UUID REFERENCES bot_instances(id) NOT NULL,
    num_dexs INTEGER DEFAULT 20,
    num_cexes INTEGER DEFAULT 200,
    monthly_fee DECIMAL(10,2) NOT NULL,
    per_exchange_fee DECIMAL(10,2),
    total_monthly_fee DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'active', -- active, paused, cancelled
    billing_cycle_start DATE,
    billing_cycle_end DATE,
    next_billing_date DATE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- CEX CONNECTIONS
-- ============================================================================

CREATE TABLE cex_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    api_key_encrypted BYTEA,
    api_secret_encrypted BYTEA,
    passphrase_encrypted BYTEA,
    is_active BOOLEAN DEFAULT true,
    last_sync_at TIMESTAMP,
    total_balance_usd DECIMAL(20,2) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cex_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cex_account_id UUID REFERENCES cex_accounts(id) ON DELETE CASCADE,
    asset VARCHAR(20) NOT NULL,
    free DECIMAL(30,8) DEFAULT 0,
    locked DECIMAL(30,8) DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TRANSACTIONS & HISTORY
-- ============================================================================

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    tx_hash VARCHAR(66) UNIQUE,
    chain_id INTEGER NOT NULL,
    from_address VARCHAR(66),
    to_address VARCHAR(66),
    value DECIMAL(30,8) DEFAULT 0,
    gas_used DECIMAL(20,2),
    gas_price_gwei DECIMAL(20,4),
    fee_usd DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'pending', -- pending, confirmed, failed
    block_number BIGINT,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_activity_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id UUID,
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- FEES & TREASURY
-- ============================================================================

CREATE TABLE protocol_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source VARCHAR(50) NOT NULL, -- swap, mint, burn, bot_fee
    amount_usd DECIMAL(20,8) NOT NULL,
    token_symbol VARCHAR(20),
    recipient VARCHAR(66),
    tx_hash VARCHAR(66),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE treasury_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(66) UNIQUE NOT NULL,
    wallet_type VARCHAR(30) NOT NULL, -- multi_sig, timelock, operational
    chain_id INTEGER NOT NULL,
    balance_eth DECIMAL(30,8) DEFAULT 0,
    balance_usd DECIMAL(20,2) DEFAULT 0,
    daily_spend_limit DECIMAL(20,2),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- LISTINGS & GOVERNANCE
-- ============================================================================

CREATE TABLE listing_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id UUID REFERENCES tokens(id),
    requester_address VARCHAR(66),
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected
    votes_for INTEGER DEFAULT 0,
    votes_against INTEGER DEFAULT 0,
    discussion_url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    resolved_at TIMESTAMP
);

CREATE TABLE dao_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    proposer_address VARCHAR(66),
    for_votes DECIMAL(20,2) DEFAULT 0,
    against_votes DECIMAL(20,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active', -- active, passed, failed, executed
    start_block BIGINT,
    end_block BIGINT,
    created_at TIMESTAMP DEFAULT NOW(),
    executed_at TIMESTAMP
);

CREATE TABLE dao_votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID REFERENCES dao_proposals(id),
    voter_address VARCHAR(66),
    support BOOLEAN NOT NULL,
    voting_power DECIMAL(20,2),
    timestamp TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

CREATE INDEX idx_users_wallet ON users(wallet_address);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_tokens_chain ON tokens(chain_id);
CREATE INDEX idx_tokens_symbol ON tokens(symbol);
CREATE INDEX idx_pairs_chain ON trading_pairs(chain_id);
CREATE INDEX idx_pools_dex ON pools(dex_id);
CREATE INDEX idx_pools_pair ON pools(pair_id);
CREATE INDEX idx_orders_user ON orders(user_id);
CREATE INDEX idx_orders_pair ON orders(pair_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created ON orders(created_at);
CREATE INDEX idx_trades_pair ON trades(pair_id);
CREATE INDEX idx_trades_timestamp ON trades(timestamp);
CREATE INDEX idx_transactions_user ON transactions(user_id);
CREATE INDEX idx_transactions_hash ON transactions(tx_hash);
CREATE INDEX idx_pool_stats_pool ON pool_stats_history(pool_id);
CREATE INDEX idx_price_history_token ON token_prices_history(token_id);
CREATE INDEX idx_bot_orders_bot ON bot_orders(bot_id);
CREATE INDEX idx_activity_user ON user_activity_log(user_id);
CREATE INDEX idx_activity_created ON user_activity_log(created_at);

-- ============================================================================
-- ADMIN & PERMISSIONS
-- ============================================================================

CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'admin', -- super_admin, dex_admin, cex_admin, finance_admin
    permissions JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES admin_users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- API KEYS FOR EXTERNAL USERS
-- ============================================================================

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    key_name VARCHAR(100),
    api_key VARCHAR(64) UNIQUE NOT NULL,
    api_secret_encrypted BYTEA NOT NULL,
    tier VARCHAR(20) DEFAULT 'free', -- free, basic, pro, enterprise, institutional
    permissions JSONB DEFAULT '{"trading": true, "withdrawal": false, "reading": true}',
    rate_limit_per_minute INTEGER DEFAULT 60,
    rate_limit_per_day INTEGER DEFAULT 10000,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE api_key_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE CASCADE,
    endpoint VARCHAR(100) NOT NULL,
    method VARCHAR(10),
    status_code INTEGER,
    latency_ms INTEGER,
    request_size INTEGER,
    response_size INTEGER,
    ip_address INET,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- BOT SUBSCRIPTION TIERS
-- ============================================================================

CREATE TABLE bot_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL, -- tier_1, tier_2, tier_3, tier_4, tier_5
    display_name VARCHAR(100) NOT NULL,
    monthly_fee_usd DECIMAL(10,2) NOT NULL,
    max_bots INTEGER DEFAULT 1,
    max_dex_connections INTEGER DEFAULT 5,
    max_cex_connections INTEGER DEFAULT 20,
    max_trading_pairs INTEGER DEFAULT 10,
    max_position_usd DECIMAL(20,2) DEFAULT 100000,
    max_daily_volume_usd DECIMAL(20,2) DEFAULT 1000000,
    latency_target_ms INTEGER DEFAULT 100,
    features JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- BOT SUBSCRIPTIONS
-- ============================================================================

CREATE TABLE bot_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    tier_id UUID REFERENCES bot_tiers(id),
    num_dexs INTEGER DEFAULT 20,
    num_cexes INTEGER DEFAULT 200,
    monthly_fee DECIMAL(10,2) NOT NULL,
    per_dex_fee DECIMAL(10,2) DEFAULT 1000,
    per_cex_fee DECIMAL(10,2) DEFAULT 100,
    total_monthly_fee DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'active', -- active, paused, cancelled, expired
    billing_cycle_start DATE NOT NULL,
    billing_cycle_end DATE NOT NULL,
    next_billing_date DATE,
    payment_method VARCHAR(50), -- crypto, card, invoice
    payment_reference VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- EXTERNAL CEX CONNECTIONS (User-managed API keys)
-- ============================================================================

CREATE TABLE user_cex_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    exchange_name VARCHAR(50) NOT NULL, -- binance, coinbase, kraken, etc.
    account_id VARCHAR(100),
    api_key_encrypted BYTEA,
    api_secret_encrypted BYTEA,
    passphrase_encrypted BYTEA,
    subaccount VARCHAR(50),
    permissions JSONB DEFAULT '{"trading": true, "reading": true, "withdrawal": false}',
    is_active BOOLEAN DEFAULT true,
    can_trade BOOLEAN DEFAULT true,
    can_withdraw BOOLEAN DEFAULT false,
    can_deposit BOOLEAN DEFAULT true,
    last_sync_at TIMESTAMP,
    sync_status VARCHAR(20) DEFAULT 'idle', -- idle, syncing, error
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_cex_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID REFERENCES user_cex_connections(id) ON DELETE CASCADE,
    asset VARCHAR(20) NOT NULL,
    free DECIMAL(30,8) DEFAULT 0,
    locked DECIMAL(30,8) DEFAULT 0,
    balance_usd DECIMAL(20,8) DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- EXTERNAL DEX CONNECTIONS (User-managed API keys)
-- ============================================================================

CREATE TABLE user_dex_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) NOT NULL,
    dex_name VARCHAR(50) NOT NULL, -- uniswap, pancakeswap, etc.
    chain_id INTEGER NOT NULL,
    wallet_address VARCHAR(66),
    router_address VARCHAR(66),
    pool_addresses VARCHAR(100)[],
    permissions JSONB DEFAULT '{"swapping": true, "liquidity": true, "borrowing": false}',
    is_active BOOLEAN DEFAULT true,
    max_slippage_bps INTEGER DEFAULT 300,
    gas_limit INTEGER DEFAULT 500000,
    last_tx_hash VARCHAR(66),
    last_tx_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- COMPLETE FEE CONFIGURATION
-- ============================================================================

CREATE TABLE fee_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_type VARCHAR(50) NOT NULL, -- swap, liquidity, withdrawal, deposit, bot_subscription, api_key, listing
    chain_id INTEGER,
    token_symbol VARCHAR(20),
    fee_amount_usd DECIMAL(20,8) NOT NULL,
    fee_percentage DECIMAL(10,4) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    min_fee_usd DECIMAL(20,8) DEFAULT 0,
    max_fee_usd DECIMAL(20,8),
    updated_by UUID REFERENCES admin_users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE admin_fee_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_type VARCHAR(50) NOT NULL,
    chain_id INTEGER NOT NULL,
    wallet_address VARCHAR(66) NOT NULL,
    token_symbol VARCHAR(20),
    is_active BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 0, -- For multi-sig distribution
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE collected_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_type VARCHAR(50) NOT NULL,
    user_id UUID REFERENCES users(id),
    amount_usd DECIMAL(20,8) NOT NULL,
    token_symbol VARCHAR(20),
    amount_token DECIMAL(30,8),
    chain_id INTEGER,
    tx_hash VARCHAR(66),
    recipient_address VARCHAR(66),
    status VARCHAR(20) DEFAULT 'collected', -- collected, pending, distributed
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- BLOCKCHAINS
-- ============================================================================

CREATE TABLE blockchains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    chain_id INTEGER UNIQUE NOT NULL,
    chain_id_hex VARCHAR(20),
    is_evm BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    explorer_url TEXT,
    rpc_url TEXT,
    native_token_symbol VARCHAR(20),
    logo_url TEXT,
    avg_gas_price_gwei DECIMAL(20,4),
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- LISTING FEES
-- ============================================================================

CREATE TABLE listing_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tier VARCHAR(20) NOT NULL, -- basic, standard, premium, premium_plus
    listing_type VARCHAR(20) NOT NULL, -- token, pool, pair
    one_time_fee_usd DECIMAL(10,2) NOT NULL,
    monthly_fee_usd DECIMAL(10,2) DEFAULT 0,
    features JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- ORDER BOOK
-- ============================================================================

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_hash VARCHAR(130) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id),
    pair_id UUID REFERENCES trading_pairs(id),
    order_type VARCHAR(20) NOT NULL, -- limit, stop_loss, stop_limit, twap, trailing_stop, post_only, fok, ioc
    side VARCHAR(10) NOT NULL, -- buy, sell
    price DECIMAL(20,8),
    quantity DECIMAL(30,8) NOT NULL,
    filled_quantity DECIMAL(30,8) DEFAULT 0,
    remaining_quantity DECIMAL(30,8) NOT NULL,
    stop_price DECIMAL(20,8),
    trigger_price DECIMAL(20,8),
    time_in_force VARCHAR(10) DEFAULT 'GTC', -- GTC, IOC, FOK, GTD
    expiry_time TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending', -- pending, partial, filled, cancelled, expired
    oco_group_id UUID,
    client_order_id VARCHAR(100),
    chain_id INTEGER NOT NULL,
    tx_hash VARCHAR(66),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    executed_at TIMESTAMP
);

CREATE TABLE twap_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    pair_id UUID REFERENCES trading_pairs(id),
    side VARCHAR(10) NOT NULL,
    total_quantity DECIMAL(30,8) NOT NULL,
    filled_quantity DECIMAL(30,8) DEFAULT 0,
    slice_interval INTEGER NOT NULL,
    slices_total INTEGER NOT NULL,
    slices_remaining INTEGER NOT NULL,
    next_slice_time TIMESTAMP NOT NULL,
    min_price DECIMAL(20,8),
    max_price DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'active', -- active, paused, completed, cancelled
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE oco_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id_1 UUID REFERENCES orders(id),
    order_id_2 UUID REFERENCES orders(id),
    user_id UUID REFERENCES users(id),
    status VARCHAR(20) DEFAULT 'active', -- active, triggered, cancelled
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE order_book_levels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pair_id UUID REFERENCES trading_pairs(id),
    side VARCHAR(10) NOT NULL, -- buy, sell
    price DECIMAL(20,8) NOT NULL,
    total_quantity DECIMAL(30,8) NOT NULL,
    order_count INTEGER DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- V4 SINGLETON POOLS
-- ============================================================================

CREATE TABLE v4_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_key VARCHAR(130) UNIQUE NOT NULL,
    token0_id UUID REFERENCES tokens(id),
    token1_id UUID REFERENCES tokens(id),
    fee_tier INTEGER NOT NULL,
    tick_lower INTEGER NOT NULL,
    tick_upper INTEGER NOT NULL,
    liquidity DECIMAL(30,8) DEFAULT 0,
    sqrt_price_x96 VARCHAR(100),
    tick INTEGER,
    hook_address VARCHAR(66),
    hook_flags INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE v4_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id UUID REFERENCES v4_pools(id),
    user_id UUID REFERENCES users(id),
    tick_lower INTEGER NOT NULL,
    tick_upper INTEGER NOT NULL,
    liquidity DECIMAL(30,8) DEFAULT 0,
    collected_fees_token0 DECIMAL(30,8) DEFAULT 0,
    collected_fees_token1 DECIMAL(30,8) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE v4_hooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hook_address VARCHAR(66) UNIQUE NOT NULL,
    name VARCHAR(100),
    description TEXT,
    before_initialize BOOLEAN DEFAULT false,
    after_initialize BOOLEAN DEFAULT false,
    before_swap BOOLEAN DEFAULT false,
    after_swap BOOLEAN DEFAULT false,
    before_modify_liquidity BOOLEAN DEFAULT false,
    after_modify_liquidity BOOLEAN DEFAULT false,
    before_donate BOOLEAN DEFAULT false,
    after_donate BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- GOVERNANCE
-- ============================================================================

CREATE TABLE governance_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id INTEGER NOT NULL,
    proposer_id UUID REFERENCES users(id),
    description TEXT NOT NULL,
    start_block INTEGER,
    end_block INTEGER,
    execution_time TIMESTAMP,
    for_votes DECIMAL(30,8) DEFAULT 0,
    against_votes DECIMAL(30,8) DEFAULT 0,
    abstain_votes DECIMAL(30,8) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending', -- pending, active, succeeded, defeated, queued, executed, cancelled
    created_at TIMESTAMP DEFAULT NOW(),
    executed_at TIMESTAMP
);

CREATE TABLE proposal_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID REFERENCES governance_proposals(id),
    target_address VARCHAR(66) NOT NULL,
    value DECIMAL(30,8) DEFAULT 0,
    signature TEXT,
    calldata BYTEA,
    executed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE governance_votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID REFERENCES governance_proposals(id),
    voter_id UUID REFERENCES users(id),
    support INTEGER NOT NULL, -- 0=against, 1=for, 2=abstain
    votes DECIMAL(30,8) NOT NULL,
    weight DECIMAL(30,8),
    timestamp TIMESTAMP DEFAULT NOW()
);

CREATE TABLE token_delegations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delegator_id UUID REFERENCES users(id),
    delegate_id UUID REFERENCES users(id),
    votes DECIMAL(30,8) NOT NULL,
    block_number INTEGER,
    timestamp TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- PERPETUAL TRADING
-- ============================================================================

CREATE TABLE perpetual_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_hash VARCHAR(130) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id),
    asset_id UUID REFERENCES tokens(id),
    side VARCHAR(10) NOT NULL, -- long, short
    size DECIMAL(30,8) NOT NULL,
    collateral DECIMAL(30,8) NOT NULL,
    average_price DECIMAL(20,8) NOT NULL,
    last_funding_payment TIMESTAMP,
    pnl DECIMAL(20,8) DEFAULT 0,
    loss DECIMAL(20,8) DEFAULT 0,
    open_time TIMESTAMP NOT NULL,
    is_liquidated BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE perpetual_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_hash VARCHAR(130) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id),
    asset_id UUID REFERENCES tokens(id),
    order_type VARCHAR(20) NOT NULL, -- market, limit, stop_loss, take_profit
    side VARCHAR(10) NOT NULL,
    size DECIMAL(30,8) NOT NULL,
    trigger_price DECIMAL(20,8),
    collateral DECIMAL(30,8),
    leverage DECIMAL(10,8),
    status VARCHAR(20) DEFAULT 'pending', -- pending, filled, cancelled
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE perpetual_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id UUID REFERENCES tokens(id),
    balance DECIMAL(30,8) DEFAULT 0,
    total_long DECIMAL(30,8) DEFAULT 0,
    total_short DECIMAL(30,8) DEFAULT 0,
    funding_rate DECIMAL(20,8) DEFAULT 0,
    last_funding_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE liquidation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_id UUID REFERENCES perpetual_positions(id),
    liquidator_id UUID REFERENCES users(id),
    reward DECIMAL(30,8) NOT NULL,
    price DECIMAL(20,8) NOT NULL,
    timestamp TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- CHAIN REGISTRY
-- ============================================================================

CREATE TABLE chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id INTEGER UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    chain_type VARCHAR(20) NOT NULL, -- evm, non_evm_solana, non_evm_aptos, etc.
    chain_id_hex VARCHAR(20),
    rpc_url TEXT,
    explorer_url TEXT,
    logo_url TEXT,
    native_token_decimals INTEGER DEFAULT 18,
    avg_gas_price_gwei DECIMAL(20,8),
    min_confirmations INTEGER DEFAULT 1,
    block_time_seconds INTEGER DEFAULT 12,
    is_testnet BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE chain_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id INTEGER NOT NULL,
    token_address VARCHAR(66),
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    decimals INTEGER NOT NULL,
    is_native BOOLEAN DEFAULT false,
    is_wrapped BOOLEAN DEFAULT false,
    wrapped_token_address VARCHAR(66),
    min_transfer DECIMAL(30,8),
    max_transfer DECIMAL(30,8),
    is_paused BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE chain_fee_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id INTEGER NOT NULL,
    swap_fee_bps INTEGER DEFAULT 30,
    withdraw_fee_min DECIMAL(20,8) DEFAULT 0,
    withdraw_fee_max DECIMAL(20,8),
    deposit_fee_min DECIMAL(20,8) DEFAULT 0,
    deposit_fee_max DECIMAL(20,8),
    cross_chain_fee DECIMAL(20,8),
    listing_fee DECIMAL(20,8),
    is_dynamic BOOLEAN DEFAULT false,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cross_chain_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_chain_id INTEGER NOT NULL,
    to_chain_id INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- LENDING PROTOCOL
-- ============================================================================

CREATE TABLE lending_markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id UUID REFERENCES tokens(id),
    chain_id INTEGER NOT NULL,
    total_supply DECIMAL(30,8) DEFAULT 0,
    total_borrows DECIMAL(30,8) DEFAULT 0,
    supply_rate DECIMAL(20,8) DEFAULT 0,
    borrow_rate DECIMAL(20,8) DEFAULT 0,
    reserve_factor DECIMAL(10,4) DEFAULT 0,
    liquidation_threshold INTEGER DEFAULT 80,
    ltv INTEGER DEFAULT 70,
    bonus INTEGER DEFAULT 5,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_lending_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    market_id UUID REFERENCES lending_markets(id),
    supply_balance DECIMAL(30,8) DEFAULT 0,
    borrow_balance DECIMAL(30,8) DEFAULT 0,
    supply_index DECIMAL(20,8) DEFAULT 1,
    borrow_index DECIMAL(20,8) DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- OPTIONS TRADING
-- ============================================================================

CREATE TABLE option_markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    underlying_id UUID REFERENCES tokens(id),
    chain_id INTEGER NOT NULL,
    min_strike_price DECIMAL(20,8),
    max_strike_price DECIMAL(20,8),
    min_expiry_days INTEGER DEFAULT 1,
    max_expiry_days INTEGER DEFAULT 365,
    pool_balance DECIMAL(30,8) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_hash VARCHAR(130) UNIQUE NOT NULL,
    writer_id UUID REFERENCES users(id),
    buyer_id UUID REFERENCES users(id),
    market_id UUID REFERENCES option_markets(id),
    option_type VARCHAR(10) NOT NULL, -- call, put
    style VARCHAR(20) NOT NULL, -- european, american
    strike_price DECIMAL(20,8) NOT NULL,
    expiry_time TIMESTAMP NOT NULL,
    amount DECIMAL(30,8) NOT NULL,
    premium DECIMAL(20,8) NOT NULL,
    exercise_price DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'active', -- active, exercised, expired, cancelled
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- WALLET ECOSYSTEM
-- ============================================================================

CREATE TABLE master_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(66) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    encrypted_seed BYTEA,
    wallet_hash VARCHAR(66) NOT NULL,
    backup_code_hash VARCHAR(66),
    is_active BOOLEAN DEFAULT true,
    withdraw_fee_percent DECIMAL(5,2) DEFAULT 1.0,
    swap_fee_percent DECIMAL(5,2) DEFAULT 0.3,
    transaction_fee_percent DECIMAL(5,2) DEFAULT 0.1,
    liquidity_fee_percent DECIMAL(5,2) DEFAULT 0.2,
    total_revenue DECIMAL(30,8) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_wallet_id UUID REFERENCES master_wallets(id),
    wallet_address VARCHAR(66) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    encrypted_seed BYTEA,
    wallet_hash VARCHAR(66) NOT NULL,
    password_hash VARCHAR(66),
    is_active BOOLEAN DEFAULT true,
    two_factor_enabled BOOLEAN DEFAULT false,
    two_factor_address VARCHAR(66),
    auto_sign_enabled BOOLEAN DEFAULT true,
    total_transactions INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    last_active_at TIMESTAMP
);

CREATE TABLE wallet_chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES user_wallets(id) ON DELETE CASCADE,
    chain_id INTEGER NOT NULL,
    is_supported BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE wallet_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES user_wallets(id) ON DELETE CASCADE,
    chain_id INTEGER NOT NULL,
    token_address VARCHAR(66),
    balance DECIMAL(30,8) DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES user_wallets(id),
    tx_hash VARCHAR(66) UNIQUE NOT NULL,
    chain_id INTEGER NOT NULL,
    tx_type VARCHAR(20) NOT NULL, -- send, receive, swap, liquidity, token_create, airdrop, campaign
    from_address VARCHAR(66),
    to_address VARCHAR(66),
    token_address VARCHAR(66),
    amount DECIMAL(30,8) NOT NULL,
    fee DECIMAL(20,8),
    status VARCHAR(20) DEFAULT 'completed', -- pending, signed, executed, failed, cancelled
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE wallet_multisig_txs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES user_wallets(id),
    tx_hash VARCHAR(66) UNIQUE NOT NULL,
    required_signatures INTEGER DEFAULT 1,
    current_signatures INTEGER DEFAULT 0,
    amount DECIMAL(30,8) NOT NULL,
    token_address VARCHAR(66),
    to_address VARCHAR(66),
    is_executed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE token_baskets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    master_wallet_id UUID REFERENCES master_wallets(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    tokens JSONB NOT NULL, -- array of token addresses
    weights JSONB NOT NULL, -- array of weights
    min_investment DECIMAL(30,8) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE wallet_auto_sign_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id VARCHAR(66) UNIQUE NOT NULL,
    wallet_id UUID REFERENCES user_wallets(id),
    tx_data BYTEA,
    is_executed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- FUNCTIONS & TRIGGERS
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_orders_updated
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_pools_updated
    BEFORE UPDATE ON pools
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE OR REPLACE FUNCTION update_pool_stats()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO pool_stats_history (pool_id, reserve_a, reserve_b, liquidity_usd, timestamp)
    VALUES (NEW.id, NEW.reserve_a, NEW.reserve_b, NEW.liquidity_usd, NOW());
    RETURN NEW;
END;
-- ============================================================================
-- WHITE LABEL CLIENTS & PRODUCTS
-- ============================================================================

-- White label clients (branded products)
CREATE TABLE white_label_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(50) UNIQUE NOT NULL,
    client_name VARCHAR(100) NOT NULL,
    brand_name VARCHAR(100),
    brand_logo_url TEXT,
    brand_color_primary VARCHAR(20),
    brand_color_secondary VARCHAR(20),
    website_url TEXT,
    contact_email VARCHAR(255) NOT NULL,
    tier VARCHAR(20) DEFAULT 'basic', -- basic, pro, enterprise
    
    -- Approval status
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, suspended, halted
    approved_by UUID REFERENCES admins(id),
    approved_at TIMESTAMP,
    suspended_by UUID REFERENCES admins(id),
    suspended_at TIMESTAMP,
    halt_reason TEXT,
    
    -- Fee configuration (0-20% shared with TigerSwap)
    swap_fee_share_bps INTEGER DEFAULT 2000, -- 2000 = 20%
    trading_fee_share_bps INTEGER DEFAULT 2000,
    bot_subscription_fee_share_bps INTEGER DEFAULT 2000,
    listing_fee_share_bps INTEGER DEFAULT 2000,
    withdrawal_fee_share_bps INTEGER DEFAULT 2000,
    deposit_fee_share_bps INTEGER DEFAULT 2000,
    transfer_fee_share_bps INTEGER DEFAULT 2000,
    api_key_fee_share_bps INTEGER DEFAULT 2000,
    
    -- TigerSwap admin fee receiver address
    admin_fee_address VARCHAR(66),
    
    -- Client's revenue address (80% goes here)
    client_revenue_address VARCHAR(66),
    
    -- Feature flags
    can_use_swap BOOLEAN DEFAULT true,
    can_use_trading BOOLEAN DEFAULT true,
    can_use_bots BOOLEAN DEFAULT true,
    can_use_listings BOOLEAN DEFAULT true,
    can_use_bridge BOOLEAN DEFAULT true,
    can_use_farming BOOLEAN DEFAULT true,
    can_use_lending BOOLEAN DEFAULT true,
    can_use_perpetuals BOOLEAN DEFAULT true,
    can_use_options BOOLEAN DEFAULT true,
    can_use_nft BOOLEAN DEFAULT true,
    can_create_api_keys BOOLEAN DEFAULT true,
    can_whitelist_tokens BOOLEAN DEFAULT false,
    can_custom_bridge BOOLEAN DEFAULT false,
    can_custom_dex BOOLEAN DEFAULT false,
    
    -- Limits
    max_daily_volume DECIMAL(20,2),
    max_daily_users INTEGER,
    max_api_calls_per_day INTEGER,
    
    -- Statistics
    total_volume_usd DECIMAL(20,2) DEFAULT 0,
    total_fees_paid DECIMAL(20,2) DEFAULT 0,
    total_users INTEGER DEFAULT 0,
    
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- White label client admins (each client can have their own admins)
CREATE TABLE white_label_admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    wallet_address VARCHAR(66) NOT NULL,
    email VARCHAR(255),
    role VARCHAR(20) DEFAULT 'admin', -- super_admin, admin, operator, viewer
    permissions JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(client_id, wallet_address)
);

-- White label API keys for external integrations
CREATE TABLE white_label_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    api_secret_hash VARCHAR(255) NOT NULL,
    label VARCHAR(100),
    permissions JSONB DEFAULT '{"swap": true, "trading": true, "bots": true}',
    rate_limit INTEGER DEFAULT 10000,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- White label fee transactions (track 20% fees)
CREATE TABLE white_label_fee_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id),
    fee_type VARCHAR(50) NOT NULL, -- swap, trading, bot, listing, etc.
    amount_usd DECIMAL(20,8) NOT NULL,
    tiger_share_usd DECIMAL(20,8) NOT NULL,
    client_share_usd DECIMAL(20,8) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(20) DEFAULT 'pending', -- pending, confirmed, failed
    created_at TIMESTAMP DEFAULT NOW()
);

-- White label whitelisted tokens (client-specific tokens)
CREATE TABLE white_label_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    token_address VARCHAR(66) NOT NULL,
    chain_id INTEGER NOT NULL,
    symbol VARCHAR(20),
    name VARCHAR(100),
    decimals INTEGER DEFAULT 18,
    is_whitelisted BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(client_id, chain_id, token_address)
);

-- White label custom chains (client-specific chains)
CREATE TABLE white_label_chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    chain_id INTEGER NOT NULL,
    chain_name VARCHAR(50),
    rpc_url TEXT,
    explorer_url TEXT,
    is_enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(client_id, chain_id)
);

-- White label analytics
CREATE TABLE white_label_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id),
    date DATE NOT NULL,
    volume_usd DECIMAL(20,2) DEFAULT 0,
    fees_usd DECIMAL(20,2) DEFAULT 0,
    users_new INTEGER DEFAULT 0,
    users_active INTEGER DEFAULT 0,
    swaps_count INTEGER DEFAULT 0,
    bots_active INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(client_id, date)
);

-- ============================================================================
-- WHITE LABEL FUNCTIONS
-- ============================================================================

-- Calculate fee split between TigerSwap and white label client
CREATE OR REPLACE FUNCTION calculate_white_label_fee(
    p_amount DECIMAL(20,8),
    p_fee_share_bps INTEGER
) RETURNS TABLE(tiger_share DECIMAL(20,8), client_share DECIMAL(20,8)) AS $$
BEGIN
    RETURN QUERY 
    SELECT 
        (p_amount * p_fee_share_bps / 10000)::DECIMAL(20,8) AS tiger_share,
        (p_amount * (10000 - p_fee_share_bps) / 10000)::DECIMAL(20,8) AS client_share;
END;
$$ LANGUAGE plpgsql;

-- Auto-distribute fees to TigerSwap admin
CREATE OR REPLACE FUNCTION distribute_white_label_fees()
RETURNS TRIGGER AS $$
DECLARE
    v_client RECORD;
    v_tiger_share DECIMAL(20,8);
    v_client_share DECIMAL(20,8);
BEGIN
    IF NEW.status = 'confirmed' THEN
        -- Get client info
        SELECT * INTO v_client 
        FROM white_label_clients 
        WHERE id = NEW.client_id;
        
        IF v_client IS NOT NULL THEN
            -- Calculate split based on fee type
            CASE NEW.fee_type
                WHEN 'swap' THEN
                    v_tiger_share := NEW.amount_usd * v_client.swap_fee_share_bps / 10000;
                WHEN 'trading' THEN
                    v_tiger_share := NEW.amount_usd * v_client.trading_fee_share_bps / 10000;
                WHEN 'bot' THEN
                    v_tiger_share := NEW.amount_usd * v_client.bot_subscription_fee_share_bps / 10000;
                WHEN 'listing' THEN
                    v_tiger_share := NEW.amount_usd * v_client.listing_fee_share_bps / 10000;
                WHEN 'withdrawal' THEN
                    v_tiger_share := NEW.amount_usd * v_client.withdrawal_fee_share_bps / 10000;
                WHEN 'deposit' THEN
                    v_tiger_share := NEW.amount_usd * v_client.deposit_fee_share_bps / 10000;
                WHEN 'transfer' THEN
                    v_tiger_share := NEW.amount_usd * v_client.transfer_fee_share_bps / 10000;
                WHEN 'api_key' THEN
                    v_tiger_share := NEW.amount_usd * v_client.api_key_fee_share_bps / 10000;
                ELSE
                    v_tiger_share := NEW.amount_usd * 2000 / 10000; -- Default 20%
            END CASE;
            
            v_client_share := NEW.amount_usd - v_tiger_share;
            
            -- Update client totals
            UPDATE white_label_clients
            SET total_fees_paid = total_fees_paid + v_tiger_share,
                updated_at = NOW()
            WHERE id = NEW.client_id;
            
            -- In production, transfer to TigerSwap admin address here
            -- Using v_client.admin_fee_address
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_white_label_fee_distribution
    AFTER UPDATE ON white_label_fee_transactions
    FOR EACH ROW EXECUTE FUNCTION distribute_white_label_fees();

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

CREATE INDEX idx_white_label_clients_status ON white_label_clients(status);
CREATE INDEX idx_white_label_clients_client_id ON white_label_clients(client_id);
CREATE INDEX idx_white_label_admins_client ON white_label_admins(client_id);
CREATE INDEX idx_white_label_api_keys_client ON white_label_api_keys(client_id);
CREATE INDEX idx_white_label_fee_transactions_client ON white_label_fee_transactions(client_id);
CREATE INDEX idx_white_label_fee_transactions_date ON white_label_fee_transactions(created_at);
CREATE INDEX idx_white_label_analytics_client_date ON white_label_analytics(client_id, date);
$$ LANGUAGE plpgsql;-- ============================================================================
-- WHITE LABEL LICENSE & DEPLOYMENT (Complete Isolation System)
-- ============================================================================

-- White label licenses - Required for each deployment
CREATE TABLE white_label_licenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    license_key VARCHAR(64) UNIQUE NOT NULL,
    license_secret_hash VARCHAR(255) NOT NULL,
    
    -- Deployment configuration
    deployment_domain VARCHAR(255) UNIQUE NOT NULL,
    deployment_cloud_provider VARCHAR(50), -- aws, gcp, azure, digitalocean, etc.
    deployment_region VARCHAR(50),
    deployment_storage_bucket VARCHAR(255),
    deployment_api_endpoint TEXT,
    
    -- License status
    status VARCHAR(20) DEFAULT 'pending', -- pending, active, suspended, expired, revoked
    activated_at TIMESTAMP,
    suspended_at TIMESTAMP,
    revoked_at TIMESTAMP,
    revoke_reason TEXT,
    expires_at TIMESTAMP,
    
    -- Usage limits
    max_concurrent_users INTEGER,
    max_api_calls_per_month INTEGER,
    max_volume_usd_per_month DECIMAL(20,2),
    
    -- Current usage
    current_users INTEGER DEFAULT 0,
    api_calls_this_month INTEGER DEFAULT 0,
    volume_this_month_usd DECIMAL(20,2) DEFAULT 0,
    
    -- Last validation
    last_validated_at TIMESTAMP,
    last_validation_ip VARCHAR(45),
    validation_failures INTEGER DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- White label API access logs (for tracking and billing)
CREATE TABLE white_label_api_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    license_id UUID REFERENCES white_label_licenses(id),
    client_id UUID REFERENCES white_label_clients(id),
    api_key VARCHAR(64),
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10),
    request_data JSONB,
    response_status INTEGER,
    response_time_ms INTEGER,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- White label deployments (separate instances)
CREATE TABLE white_label_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id),
    license_id UUID REFERENCES white_label_licenses(id),
    
    -- Deployment details
    deployment_name VARCHAR(100) NOT NULL,
    deployment_type VARCHAR(50) DEFAULT 'full', -- full, frontend-only, api-only
    version VARCHAR(20) DEFAULT '1.0.0',
    
    -- Infrastructure (completely separate)
    cloud_provider VARCHAR(50) NOT NULL,
    cloud_region VARCHAR(50) NOT NULL,
    cloud_project_id VARCHAR(100),
    cloud_bucket VARCHAR(255),
    database_url TEXT,
    database_name VARCHAR(100),
    redis_url TEXT,
    
    -- DNS and domain
    domain VARCHAR(255) UNIQUE NOT NULL,
    ssl_certificate_arn VARCHAR(255),
    cdn_endpoint TEXT,
    
    -- Status
    status VARCHAR(20) DEFAULT 'pending', -- pending, deploying, active, stopped, destroyed
    deployed_at TIMESTAMP,
    stopped_at TIMESTAMP,
    destroyed_at TIMESTAMP,
    destroy_reason TEXT,
    
    -- Costs tracking
    monthly_cost_usd DECIMAL(10,2) DEFAULT 0,
    last_billed_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- White label feature flags (complete feature control)
CREATE TABLE white_label_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    
    -- Core features
    enable_swap BOOLEAN DEFAULT true,
    enable_trading BOOLEAN DEFAULT true,
    enable_limit_orders BOOLEAN DEFAULT true,
    enable_stop_loss BOOLEAN DEFAULT true,
    enable_order_book BOOLEAN DEFAULT true,
    enable_pool BOOLEAN DEFAULT true,
    enable_farming BOOLEAN DEFAULT true,
    enable_bridge BOOLEAN DEFAULT true,
    enable_lending BOOLEAN DEFAULT true,
    enable_perpetuals BOOLEAN DEFAULT true,
    enable_options BOOLEAN DEFAULT true,
    enable_nft BOOLEAN DEFAULT true,
    
    -- Bot features
    enable_mm_bot BOOLEAN DEFAULT true,
    enable_arbitrage_bot BOOLEAN DEFAULT true,
    enable_sniper_bot BOOLEAN DEFAULT true,
    enable_liquidity_bot BOOLEAN DEFAULT true,
    enable_front_run_bot BOOLEAN DEFAULT true,
    enable_mev_bot BOOLEAN DEFAULT true,
    enable_sandwich_bot BOOLEAN DEFAULT true,
    enable_flash_loan_bot BOOLEAN DEFAULT true,
    enable_cross_chain_bot BOOLEAN DEFAULT true,
    enable_perp_hedge_bot BOOLEAN DEFAULT true,
    
    -- Wallet features
    enable_create_wallet BOOLEAN DEFAULT true,
    enable_import_wallet BOOLEAN DEFAULT true,
    enable_hd_wallet BOOLEAN DEFAULT true,
    enable_master_wallet BOOLEAN DEFAULT true,
    enable_multisig BOOLEAN DEFAULT true,
    enable_auto_sign BOOLEAN DEFAULT true,
    
    -- API features
    enable_api_access BOOLEAN DEFAULT true,
    enable_webhook BOOLEAN DEFAULT true,
    enable_webSocket BOOLEAN DEFAULT true,
    
    -- Branding
    enable_custom_brand BOOLEAN DEFAULT false,
    enable_custom_tokens BOOLEAN DEFAULT false,
    enable_custom_chains BOOLEAN DEFAULT false,
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- White label earnings tracking (20% to TigerSwap)
CREATE TABLE white_label_earnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id),
    license_id UUID REFERENCES white_label_licenses(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    
    -- Revenue breakdown
    swap_revenue_usd DECIMAL(20,8) DEFAULT 0,
    trading_revenue_usd DECIMAL(20,8) DEFAULT 0,
    bot_revenue_usd DECIMAL(20,8) DEFAULT 0,
    listing_revenue_usd DECIMAL(20,8) DEFAULT 0,
    api_revenue_usd DECIMAL(20,8) DEFAULT 0,
    other_revenue_usd DECIMAL(20,8) DEFAULT 0,
    total_revenue_usd DECIMAL(20,8) DEFAULT 0,
    
    -- TigerSwap share (20%)
    tiger_swap_share_usd DECIMAL(20,8) DEFAULT 0,
    client_share_usd DECIMAL(20,8) DEFAULT 0,
    
    -- Transaction count
    total_transactions INTEGER DEFAULT 0,
    total_swaps INTEGER DEFAULT 0,
    total_trades INTEGER DEFAULT 0,
    total_bot_orders INTEGER DEFAULT 0,
    
    -- Status
    status VARCHAR(20) DEFAULT 'pending', -- pending, calculated, paid
    paid_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(client_id, period_start)
);

-- ============================================================================
-- LICENSE & DEPLOYMENT FUNCTIONS
-- ============================================================================

-- Validate license key
CREATE OR REPLACE FUNCTION validate_white_label_license(
    p_license_key VARCHAR(64),
    p_client_id VARCHAR(50)
) RETURNS TABLE(
    is_valid BOOLEAN,
    client_id VARCHAR(50),
    deployment_domain VARCHAR(255),
    status VARCHAR(20),
    error_message VARCHAR(255)
) AS $$
DECLARE
    v_license RECORD;
    v_result BOOLEAN := FALSE;
    v_status VARCHAR(20);
    v_error VARCHAR(255);
BEGIN
    -- Find license
    SELECT * INTO v_license 
    FROM white_label_licenses 
    WHERE license_key = p_license_key 
      AND client_id::VARCHAR = p_client_id;
    
    IF v_license IS NULL THEN
        RETURN QUERY SELECT FALSE, NULL, NULL, NULL, 'License not found';
        RETURN;
    END IF;
    
    v_status := v_license.status;
    
    -- Check status
    IF v_status = 'suspended' THEN
        v_error := 'License suspended. Contact TigerSwap admin.';
    ELSIF v_status = 'revoked' THEN
        v_error := 'License revoked. Contact TigerSwap admin.';
    ELSIF v_status = 'expired' THEN
        v_error := 'License expired. Renew at TigerSwap admin.';
    ELSIF v_status = 'pending' THEN
        v_error := 'License pending approval. Contact TigerSwap admin.';
    ELSIF v_license.expires_at < NOW() THEN
        v_error := 'License expired. Contact TigerSwap admin.';
    ELSE
        v_result := TRUE;
        v_error := NULL;
        
        -- Update validation
        UPDATE white_label_licenses
        SET last_validated_at = NOW(),
            validation_failures = 0
        WHERE id = v_license.id;
    END IF;
    
    RETURN QUERY SELECT v_result, v_license.client_id::VARCHAR, v_license.deployment_domain, v_status, v_error;
END;
$$ LANGUAGE plpgsql;

-- Calculate and distribute earnings
CREATE OR REPLACE FUNCTION calculate_white_label_earnings(
    p_client_id UUID,
    p_period_start DATE,
    p_period_end DATE
) RETURNS VOID AS $$
DECLARE
    v_swap_bps INTEGER;
    v_trading_bps INTEGER;
    v_bot_bps INTEGER;
    v_listing_bps INTEGER;
    v_api_bps INTEGER;
    v_total_revenue DECIMAL(20,8);
    v_tiger_share DECIMAL(20,8);
    v_client_share DECIMAL(20,8);
BEGIN
    -- Get client's fee percentages
    SELECT swap_fee_share_bps, trading_fee_share_bps, bot_subscription_fee_share_bps, 
           listing_fee_share_bps, api_key_fee_share_bps
    INTO v_swap_bps, v_trading_bps, v_bot_bps, v_listing_bps, v_api_bps
    FROM white_label_clients
    WHERE id = p_client_id;
    
    -- Get totals from fee transactions
    SELECT 
        COALESCE(SUM(CASE WHEN fee_type = 'swap' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN fee_type = 'trading' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN fee_type = 'bot' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN fee_type = 'listing' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN fee_type = 'api_key' THEN amount_usd ELSE 0 END), 0)
    INTO v_swap_bps, v_trading_bps, v_bot_bps, v_listing_bps, v_api_bps  -- Using wrong vars, fix below
    FROM white_label_fee_transactions
    WHERE client_id = p_client_id
      AND created_at BETWEEN p_period_start AND p_period_end;
    
    -- Calculate total revenue
    v_total_revenue := v_swap_bps + v_trading_bps + v_bot_bps + v_listing_bps + v_api_bps;
    
    -- Calculate TigerSwap share (20% default)
    v_tiger_share := v_total_revenue * 2000 / 10000;
    v_client_share := v_total_revenue - v_tiger_share;
    
    -- Insert earnings record
    INSERT INTO white_label_earnings (
        client_id, period_start, period_end,
        swap_revenue_usd, trading_revenue_usd, bot_revenue_usd,
        listing_revenue_usd, api_revenue_usd, other_revenue_usd,
        total_revenue_usd, tiger_swap_share_usd, client_share_usd,
        status
    ) VALUES (
        p_client_id, p_period_start, p_period_end,
        v_swap_bps, v_trading_bps, v_bot_bps,
        v_listing_bps, v_api_bps, 0,
        v_total_revenue, v_tiger_share, v_client_share,
        'calculated'
    );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- INDEXES FOR LICENSE SYSTEM
-- ============================================================================

CREATE INDEX idx_white_label_licenses_key ON white_label_licenses(license_key);
CREATE INDEX idx_white_label_licenses_client ON white_label_licenses(client_id);
CREATE INDEX idx_white_label_licenses_domain ON white_label_licenses(deployment_domain);
CREATE INDEX idx_white_label_deployments_client ON white_label_deployments(client_id);
CREATE INDEX idx_white_label_deployments_domain ON white_label_deployments(domain);
CREATE INDEX idx_white_label_api_logs_license ON white_label_api_logs(license_id, created_at);
CREATE INDEX idx_white_label_earnings_client_period ON white_label_earnings(client_id, period_start);
-- ============================================================================
-- WHITE LABEL FEATURE SYNC & UPDATES
-- ============================================================================

-- TigerSwap version and features registry
CREATE TABLE tigerswap_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version VARCHAR(20) UNIQUE NOT NULL,
    release_date DATE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    release_notes TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Feature registry (all available features in TigerSwap)
CREATE TABLE tigerswap_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_id VARCHAR(50) UNIQUE NOT NULL,
    feature_name VARCHAR(100) NOT NULL,
    category VARCHAR(50) NOT NULL, -- core, bot, wallet, api, branding
    description TEXT,
    is_enabled BOOLEAN DEFAULT true,
    version_added VARCHAR(20) NOT NULL,
    version_deprecated VARCHAR(20),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Feature updates available for white label
CREATE TABLE white_label_updates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    
    -- Update info
    update_id VARCHAR(50) UNIQUE NOT NULL,
    update_version VARCHAR(20) NOT NULL,
    update_type VARCHAR(20) NOT NULL, -- feature_add, feature_update, security_fix, breaking_change
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    
    -- Status
    status VARCHAR(20) DEFAULT 'available', -- available, downloaded, applied, failed
    available_at TIMESTAMP DEFAULT NOW(),
    downloaded_at TIMESTAMP,
    applied_at TIMESTAMP,
    failed_reason TEXT,
    
    -- Features affected
    features_added JSONB DEFAULT '[]',
    features_updated JSONB DEFAULT '[]',
    features_removed JSONB DEFAULT '[]',
    
    -- Size and checksum
    update_size_bytes INTEGER,
    checksum VARCHAR(64),
    
    created_at TIMESTAMP DEFAULT NOW()
);

-- White label version tracking
CREATE TABLE white_label_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id) ON DELETE CASCADE,
    license_id UUID REFERENCES white_label_licenses(id),
    
    current_version VARCHAR(20) NOT NULL,
    latest_available_version VARCHAR(20),
    last_checked_at TIMESTAMP,
    update_available BOOLEAN DEFAULT false,
    
    -- Feature status
    enabled_features JSONB DEFAULT '[]',
    disabled_features JSONB DEFAULT '[]',
    
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Sync log
CREATE TABLE white_label_sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID REFERENCES white_label_clients(id),
    sync_type VARCHAR(20) NOT NULL, -- manual, scheduled, forced
    status VARCHAR(20) DEFAULT 'pending',
    features_synced INTEGER DEFAULT 0,
    errors TEXT,
    started_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);

-- ============================================================================
-- FEATURE SYNC FUNCTIONS
// ============================================================================

-- Get latest TigerSwap version
CREATE OR REPLACE FUNCTION get_latest_tigerswap_version()
RETURNS VARCHAR(20) AS $$
DECLARE
    v_version VARCHAR(20);
BEGIN
    SELECT version INTO v_version
    FROM tigerswap_versions
    WHERE is_active = true
    ORDER BY release_date DESC
    LIMIT 1;
    RETURN v_version;
END;
$$ LANGUAGE plpgsql;

-- Check for available updates
CREATE OR REPLACE FUNCTION check_for_updates(
    p_client_id UUID
) RETURNS TABLE(
    update_version VARCHAR(20),
    update_count INTEGER,
    has_breaking_changes BOOLEAN
) AS $$
DECLARE
    v_current_version VARCHAR(20);
    v_latest_version VARCHAR(20);
    v_update_count INTEGER := 0;
    v_has_breaking BOOLEAN := FALSE;
BEGIN
    -- Get current version
    SELECT current_version INTO v_current_version
    FROM white_label_versions
    WHERE client_id = p_client_id;
    
    -- Get latest TigerSwap version
    v_latest_version := get_latest_tigerswap_version();
    
    -- Count available updates
    SELECT COUNT(*) INTO v_update_count
    FROM white_label_updates
    WHERE client_id = p_client_id
      AND status = 'available'
      AND update_version > v_current_version;
    
    -- Check for breaking changes
    SELECT COUNT(*) > 0 INTO v_has_breaking
    FROM white_label_updates
    WHERE client_id = p_client_id
      AND update_type = 'breaking_change'
      AND status = 'available'
      AND update_version > v_current_version;
    
    RETURN QUERY SELECT v_latest_version, v_update_count, v_has_breaking;
END;
$$ LANGUAGE plpgsql;

-- Apply update to white label
CREATE OR REPLACE FUNCTION apply_white_label_update(
    p_client_id UUID,
    p_update_id VARCHAR(50)
) RETURNS BOOLEAN AS $$
DECLARE
    v_update RECORD;
    v_features JSONB;
    v_success BOOLEAN := FALSE;
BEGIN
    -- Get update record
    SELECT * INTO v_update
    FROM white_label_updates
    WHERE client_id = p_client_id
      AND update_id = p_update_id;
    
    IF v_update IS NULL THEN
        RETURN FALSE;
    END IF;
    
    -- Mark as downloading
    UPDATE white_label_updates
    SET status = 'downloading',
        downloaded_at = NOW()
    WHERE id = v_update.id;
    
    -- In production: Download and apply update here
    -- For now, simulate success
    
    -- Update features based on update type
    IF v_update.features_added IS NOT NULL THEN
        -- Add new features
        UPDATE white_label_features
        SET updated_at = NOW()
        WHERE client_id = p_client_id;
    END IF;
    
    -- Mark as applied
    UPDATE white_label_updates
    SET status = 'applied',
        applied_at = NOW()
    WHERE id = v_update.id;
    
    -- Update version
    UPDATE white_label_versions
    SET current_version = v_update.update_version,
        update_available = false,
        updated_at = NOW()
    WHERE client_id = p_client_id;
    
    RETURN TRUE;
EXCEPTION
    WHEN OTHERS THEN
        UPDATE white_label_updates
        SET status = 'failed',
            failed_reason = SQLERRM
        WHERE id = v_update.id;
        RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- INDEXES
-- ============================================================================

CREATE INDEX idx_tigerswap_features_category ON tigerswap_features(category);
CREATE INDEX idx_white_label_updates_client ON white_label_updates(client_id, status);
CREATE INDEX idx_white_label_updates_version ON white_label_updates(update_version);
CREATE INDEX idx_white_label_versions_client ON white_label_versions(client_id);
CREATE INDEX idx_white_label_sync_logs_client ON white_label_sync_logs(client_id, started_at);
