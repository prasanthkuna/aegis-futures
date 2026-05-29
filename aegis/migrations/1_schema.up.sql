CREATE TABLE IF NOT EXISTS bot_config (
    id TEXT PRIMARY KEY DEFAULT 'default',
    account_capital_usd DOUBLE PRECISION NOT NULL DEFAULT 1000,
    active_capital_usd DOUBLE PRECISION NOT NULL DEFAULT 250,
    max_leverage INT NOT NULL DEFAULT 2,
    risk_per_trade_usd DOUBLE PRECISION NOT NULL DEFAULT 1.25,
    max_open_positions INT NOT NULL DEFAULT 1,
    max_trades_per_day INT NOT NULL DEFAULT 6,
    daily_hard_stop_usd DOUBLE PRECISION NOT NULL DEFAULT 7.5,
    weekly_hard_stop_usd DOUBLE PRECISION NOT NULL DEFAULT 20,
    max_consecutive_losses INT NOT NULL DEFAULT 3,
    cooldown_after_loss_minutes INT NOT NULL DEFAULT 20,
    min_trade_score DOUBLE PRECISION NOT NULL DEFAULT 0.78,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO bot_config (id) VALUES ('default') ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS bot_runs (
    id BIGSERIAL PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'live',
    state TEXT NOT NULL DEFAULT 'IDLE',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    notes TEXT
);

CREATE TABLE IF NOT EXISTS bot_state_log (
    id BIGSERIAL PRIMARY KEY,
    bot_run_id BIGINT REFERENCES bot_runs(id),
    from_state TEXT,
    to_state TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS config_versions (
    id BIGSERIAL PRIMARY KEY,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS account_snapshots (
    id BIGSERIAL PRIMARY KEY,
    balance DOUBLE PRECISION NOT NULL,
    available_margin DOUBLE PRECISION NOT NULL,
    unrealized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS symbol_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    rank INT NOT NULL DEFAULT 0,
    quote_volume_24h DOUBLE PRECISION NOT NULL DEFAULT 0,
    spread_bps DOUBLE PRECISION NOT NULL DEFAULT 0,
    funding_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    volume_surge DOUBLE PRECISION NOT NULL DEFAULT 0,
    tradable BOOLEAN NOT NULL DEFAULT false,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS setup_scores (
    id BIGSERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    trade_score DOUBLE PRECISION NOT NULL,
    volume_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    cvd_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    structure_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    context_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    depth_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    session_component DOUBLE PRECISION NOT NULL DEFAULT 0,
    decision TEXT NOT NULL,
    reason TEXT,
    side_hint TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    bot_run_id BIGINT REFERENCES bot_runs(id),
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    order_type TEXT NOT NULL,
    client_order_id TEXT,
    exchange_order_id BIGINT,
    price DOUBLE PRECISION,
    quantity DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL,
    reduce_only BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fills (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT REFERENCES orders(id),
    symbol TEXT NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    commission DOUBLE PRECISION NOT NULL DEFAULT 0,
    commission_asset TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS positions (
    id BIGSERIAL PRIMARY KEY,
    bot_run_id BIGINT REFERENCES bot_runs(id),
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    entry_price DOUBLE PRECISION NOT NULL,
    leverage INT NOT NULL DEFAULT 2,
    stop_price DOUBLE PRECISION,
    take_profit_price DOUBLE PRECISION,
    status TEXT NOT NULL DEFAULT 'open',
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS trades (
    id BIGSERIAL PRIMARY KEY,
    bot_run_id BIGINT REFERENCES bot_runs(id),
    mode TEXT NOT NULL DEFAULT 'live',
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    entry_time TIMESTAMPTZ NOT NULL,
    exit_time TIMESTAMPTZ,
    entry_price DOUBLE PRECISION NOT NULL,
    exit_price DOUBLE PRECISION,
    quantity DOUBLE PRECISION NOT NULL,
    leverage INT NOT NULL DEFAULT 2,
    gross_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    fees DOUBLE PRECISION NOT NULL DEFAULT 0,
    funding DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    r_multiple DOUBLE PRECISION NOT NULL DEFAULT 0,
    trade_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    entry_reason TEXT,
    exit_reason TEXT,
    session TEXT,
    config_version BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pnl_ledger (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trade_id BIGINT REFERENCES trades(id),
    balance_before DOUBLE PRECISION,
    balance_after DOUBLE PRECISION,
    gross_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    commission DOUBLE PRECISION NOT NULL DEFAULT 0,
    funding DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    realized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    unrealized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS risk_events (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    severity TEXT NOT NULL,
    type TEXT NOT NULL,
    symbol TEXT,
    position_id BIGINT,
    message TEXT NOT NULL,
    action_taken TEXT,
    resolved BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS guardian_checks (
    id BIGSERIAL PRIMARY KEY,
    position_id BIGINT REFERENCES positions(id),
    check_ok BOOLEAN NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS missed_trades (
    id BIGSERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    trade_score DOUBLE PRECISION NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_setup_scores_symbol_created ON setup_scores(symbol, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_trades_exit_time ON trades(exit_time DESC);
CREATE INDEX IF NOT EXISTS idx_risk_events_created ON risk_events(created_at DESC);
