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
$$ LANGUAGE plpgsql;