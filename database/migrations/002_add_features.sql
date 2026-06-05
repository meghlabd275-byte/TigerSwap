-- TigerSwap Database Migrations
-- Version tracking and incremental schema changes

-- Migration 001: Initial Schema
-- Migration 002: Add Bot Subscriptions
-- Migration 003: Add CEX Support
-- Migration 004: Add Liquidity Tracking
-- Migration 005: Add Performance Metrics

-- ============================================================================
-- MIGRATION 001: Initial Schema (Already in main_schema.sql)
-- ============================================================================

-- Migration 002: Bot Subscriptions and Billing
CREATE TABLE IF NOT EXISTS bot_subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_name VARCHAR(50) NOT NULL,
    bot_type VARCHAR(30) NOT NULL,
    monthly_fee_usd DECIMAL(10,2) NOT NULL,
    per_dex_fee_usd DECIMAL(10,2) DEFAULT 1000,
    per_cex_fee_usd DECIMAL(10,2) DEFAULT 1000,
    max_bots INTEGER DEFAULT 10,
    features JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO bot_subscription_plans (plan_name, bot_type, monthly_fee_usd, per_dex_fee_usd, per_cex_fee_usd, features) VALUES
('Market Maker Pro', 'market_maker', 5000, 1000, 1000, '{"all_dexs": true, "all_cexs": true, "priority_support": true}'),
('Arbitrage Elite', 'arbitrage', 3000, 750, 750, '{"all_dexs": true, "all_cexs": true}'),
('Sniper Basic', 'sniper', 2500, 500, 500, '{"dex_only": true}'),
('MEV Hunter', 'mev_bot', 2500, 500, 500, '{"mempo_access": true}'),
('Standard Bot', 'other', 2500, 500, 500, '{}');

-- Migration 003: CEX Account Management
CREATE TABLE IF NOT EXISTS cex_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL,
    api_key_encrypted BYTEA NOT NULL,
    api_secret_encrypted BYTEA NOT NULL,
    passphrase_encrypted BYTEA,
    permissions JSONB DEFAULT '{"read": true, "trade": true, "withdraw": false}',
    ip_whitelist JSONB,
    last_auth_at TIMESTAMP,
    auth_failures INTEGER DEFAULT 0,
    is_suspended BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cex_trade_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cex_account_id UUID REFERENCES cex_accounts(id) ON DELETE CASCADE,
    exchange_order_id VARCHAR(100),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    price DECIMAL(20,8),
    qty DECIMAL(30,8) NOT NULL,
    fee DECIMAL(20,8),
    fee_currency VARCHAR(20),
    timestamp TIMESTAMP NOT NULL,
    synced_at TIMESTAMP DEFAULT NOW()
);

-- Migration 004: Liquidity Pool Metrics
CREATE TABLE IF NOT EXISTS liquidity_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id UUID REFERENCES pools(id) NOT NULL,
    user_id UUID REFERENCES users(id),
    event_type VARCHAR(30) NOT NULL, -- mint, burn, swap, fees_collected
    token_a_amount DECIMAL(30,8),
    token_b_amount DECIMAL(30,8),
    liquidity_amount DECIMAL(30,8),
    tx_hash VARCHAR(66),
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fee_rewards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id UUID REFERENCES pools(id) NOT NULL,
    user_id UUID REFERENCES users(id),
    token_a_rewards DECIMAL(30,8) DEFAULT 0,
    token_b_rewards DECIMAL(30,8) DEFAULT 0,
    period_start TIMESTAMP,
    period_end TIMESTAMP,
    is_claimed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Migration 005: Performance Metrics
CREATE TABLE IF NOT EXISTS dex_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dex_id UUID REFERENCES dexes(id) NOT NULL,
    avg_latency_us INTEGER,
    success_rate DECIMAL(5,2),
    volume_24h_usd DECIMAL(20,2),
    uptime_seconds INTEGER,
    errors_count INTEGER DEFAULT 0,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id UUID REFERENCES bot_instances(id) NOT NULL,
    latency_p50_us INTEGER,
    latency_p99_us INTEGER,
    orders_per_minute DECIMAL(10,2),
    success_rate DECIMAL(5,2),
    avg_spread_earned_bps DECIMAL(10,2),
    pnl_delta DECIMAL(20,2),
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS network_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id INTEGER NOT NULL,
    block_time_ms INTEGER,
    gas_price_gwei DECIMAL(20,4),
    txn_per_second DECIMAL(10,2),
    active_addresses INTEGER,
    total_volume_usd DECIMAL(20,2),
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- VIEWS FOR DASHBOARD
-- ============================================================================

CREATE OR REPLACE VIEW v_pool_metrics AS
SELECT 
    p.id,
    p.pool_address,
    t_a.symbol as token_a,
    t_b.symbol as token_b,
    d.name as dex_name,
    p.liquidity_usd,
    p.volume_24h_usd,
    p.tvl_usd,
    p.apr,
    p.is_active,
    p.updated_at
FROM pools p
JOIN tokens t_a ON p.token_a_address = t_a.contract_address
JOIN tokens t_b ON p.token_b_address = t_b.contract_address
JOIN dexes d ON p.dex_id = d.id;

CREATE OR REPLACE VIEW v_user_portfolio AS
SELECT 
    u.id as user_id,
    u.wallet_address,
    COALESCE(u.total_volume_usd, 0) as total_volume,
    COALESCE(u.total_pnl, 0) as total_pnl,
    COALESCE(SUM(lp.liquidity_usd), 0) as liquidity_provided,
    COUNT(DISTINCT bi.id) as active_bots,
    COUNT(DISTINCT o.id) as total_trades
FROM users u
LEFT JOIN liquidity_positions lp ON u.id = lp.user_id AND lp.is_active = true
LEFT JOIN bot_instances bi ON u.id = bi.user_id AND bi.status = 'running'
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.wallet_address;

CREATE OR REPLACE VIEW v_bot_revenue AS
SELECT 
    bs.bot_id,
    u.id as user_id,
    u.wallet_address,
    bs.monthly_fee_usd,
    bs.per_exchange_fee_usd,
    bi.num_dexs,
    bi.num_cexes,
    bs.total_monthly_fee,
    bs.status,
    bs.next_billing_date
FROM bot_subscriptions bs
JOIN bot_instances bi ON bs.bot_id = bi.id
JOIN users u ON bi.user_id = u.id
WHERE bs.status = 'active';

CREATE OR REPLACE VIEW v_recent_trades AS
SELECT 
    t.id,
    t.pair_id,
    tp.token_a_symbol || '/' || tp.token_b_symbol as pair,
    t.side,
    t.price,
    t.qty,
    t.fee_usd,
    t.timestamp,
    t.dex,
    u.wallet_address as user_wallet
FROM trades t
JOIN trading_pairs tp ON t.pair_id = tp.id
JOIN users u ON t.user_id = u.id
ORDER BY t.timestamp DESC
LIMIT 100;