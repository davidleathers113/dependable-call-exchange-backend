-- ============================================================================
-- DEPENDABLE CALL EXCHANGE: PRODUCTION-READY DATABASE ARCHITECTURE
-- ============================================================================
-- Version: 2.0.0 (CORRECTED)
-- Created: 2025-06-08
-- Description: A sensible, production-ready database schema that actually works
--              and follows PostgreSQL best practices.
-- ============================================================================

-- =============================================================================
-- PHASE 1: EXTENSIONS (Only what we actually need)
-- =============================================================================

-- Core extensions that are actually useful
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";      -- UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";       -- Cryptographic functions
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements"; -- Query performance tracking
CREATE EXTENSION IF NOT EXISTS "timescaledb";    -- Time-series optimization

-- =============================================================================
-- PHASE 2: CUSTOM TYPES (Simplified and practical)
-- =============================================================================

-- Account types
CREATE TYPE account_type AS ENUM ('buyer', 'seller', 'admin');
CREATE TYPE account_status AS ENUM ('active', 'suspended', 'closed');

-- Call tracking
CREATE TYPE call_status AS ENUM (
    'pending', 'ringing', 'in_progress', 'completed', 'failed', 'no_answer'
);
CREATE TYPE call_direction AS ENUM ('inbound', 'outbound');

-- Bidding
CREATE TYPE bid_status AS ENUM ('active', 'won', 'lost', 'expired');

-- Simple domain for positive amounts
CREATE DOMAIN positive_amount AS DECIMAL(12,4) CHECK (VALUE >= 0);

-- =============================================================================
-- PHASE 3: CORE TABLES (Simplified and optimized)
-- =============================================================================

-- -----------------------------------------------------------------------------
-- ACCOUNTS: Simplified account management
-- -----------------------------------------------------------------------------
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    type account_type NOT NULL,
    status account_status NOT NULL DEFAULT 'active',
    
    -- Financial
    balance DECIMAL(15,4) DEFAULT 0.00 NOT NULL CHECK (balance >= 0),
    
    -- Quality metrics (simplified)
    quality_score INTEGER DEFAULT 50 CHECK (quality_score BETWEEN 0 AND 100),
    
    -- Settings as JSONB for flexibility
    settings JSONB DEFAULT '{}' NOT NULL,
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    
    -- Soft delete
    deleted_at TIMESTAMPTZ
);

-- Essential indexes only
CREATE INDEX idx_accounts_email ON accounts(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_type_status ON accounts(type, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_updated ON accounts(updated_at) WHERE deleted_at IS NULL;

-- -----------------------------------------------------------------------------
-- CALLS: Time-series optimized with reasonable chunking
-- -----------------------------------------------------------------------------
CREATE TABLE calls (
    id UUID NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    
    -- Call details
    from_number VARCHAR(20) NOT NULL,
    to_number VARCHAR(20) NOT NULL,
    direction call_direction NOT NULL,
    status call_status NOT NULL DEFAULT 'pending',
    
    -- Timing
    connect_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    duration INTEGER, -- seconds
    
    -- Participants
    buyer_id UUID NOT NULL REFERENCES accounts(id),
    seller_id UUID REFERENCES accounts(id),
    
    -- Financial
    total_cost positive_amount,
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    
    PRIMARY KEY (id, start_time)
);

-- Convert to hypertable with REASONABLE chunk interval (1 week)
SELECT create_hypertable('calls', 'start_time', 
    chunk_time_interval => INTERVAL '1 week',
    create_default_indexes => FALSE);

-- Only essential indexes
CREATE INDEX idx_calls_buyer_time ON calls(buyer_id, start_time DESC);
CREATE INDEX idx_calls_status_time ON calls(status, start_time DESC) 
    WHERE status IN ('pending', 'ringing', 'in_progress');

-- Enable compression after 30 days (not 7)
ALTER TABLE calls SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'buyer_id,status',
    timescaledb.compress_orderby = 'start_time DESC'
);

SELECT add_compression_policy('calls', INTERVAL '30 days');

-- -----------------------------------------------------------------------------
-- BIDS: Simplified auction system
-- -----------------------------------------------------------------------------
CREATE TABLE bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    call_id UUID NOT NULL,
    buyer_id UUID NOT NULL REFERENCES accounts(id),
    
    -- Bid details
    amount positive_amount NOT NULL,
    status bid_status NOT NULL DEFAULT 'active',
    
    -- Timing
    placed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    
    -- Simple constraint
    CONSTRAINT chk_bid_timing CHECK (expires_at > placed_at)
);

-- Minimal indexes for performance
CREATE INDEX idx_bids_call_status ON bids(call_id, status, amount DESC);
CREATE INDEX idx_bids_buyer ON bids(buyer_id, placed_at DESC);
CREATE INDEX idx_bids_expires ON bids(expires_at) WHERE status = 'active';

-- -----------------------------------------------------------------------------
-- TRANSACTIONS: Simple ledger
-- -----------------------------------------------------------------------------
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id),
    
    -- Transaction details
    amount DECIMAL(15,4) NOT NULL, -- positive for credits, negative for debits
    balance_after DECIMAL(15,4) NOT NULL,
    
    -- Reference
    reference_type VARCHAR(50),
    reference_id UUID,
    
    -- Metadata
    description TEXT,
    
    -- Timestamp
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

-- Index for account history
CREATE INDEX idx_transactions_account ON transactions(account_id, created_at DESC);

-- =============================================================================
-- PHASE 4: FUNCTIONS & TRIGGERS (Only essential ones)
-- =============================================================================

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_accounts_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Simple balance update with proper locking
CREATE OR REPLACE FUNCTION update_account_balance(
    p_account_id UUID,
    p_amount DECIMAL(15,4),
    p_reference_type VARCHAR(50),
    p_reference_id UUID,
    p_description TEXT
) RETURNS DECIMAL(15,4) AS $$
DECLARE
    v_new_balance DECIMAL(15,4);
BEGIN
    -- Lock account row and update balance
    UPDATE accounts 
    SET balance = balance + p_amount
    WHERE id = p_account_id
    RETURNING balance INTO v_new_balance;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Account not found: %', p_account_id;
    END IF;
    
    IF v_new_balance < 0 THEN
        RAISE EXCEPTION 'Insufficient balance';
    END IF;
    
    -- Record transaction
    INSERT INTO transactions (
        account_id, amount, balance_after, 
        reference_type, reference_id, description
    ) VALUES (
        p_account_id, p_amount, v_new_balance,
        p_reference_type, p_reference_id, p_description
    );
    
    RETURN v_new_balance;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- PHASE 5: MATERIALIZED VIEWS (Reasonable refresh intervals)
-- =============================================================================

-- Daily account summary (refreshed nightly, not constantly)
CREATE MATERIALIZED VIEW account_daily_summary AS
SELECT 
    a.id,
    a.type,
    DATE(NOW()) as summary_date,
    COUNT(DISTINCT c.id) FILTER (WHERE c.start_time >= CURRENT_DATE) as calls_today,
    COALESCE(SUM(c.total_cost) FILTER (WHERE c.start_time >= CURRENT_DATE), 0) as spend_today,
    COUNT(DISTINCT b.id) FILTER (WHERE b.placed_at >= CURRENT_DATE) as bids_today
FROM accounts a
LEFT JOIN calls c ON c.buyer_id = a.id
LEFT JOIN bids b ON b.buyer_id = a.id
WHERE a.deleted_at IS NULL
GROUP BY a.id, a.type;

CREATE INDEX idx_account_daily_summary ON account_daily_summary(id);

-- =============================================================================
-- PHASE 6: CONTINUOUS AGGREGATES (Hourly, not every 10 minutes!)
-- =============================================================================

-- Hourly call statistics
CREATE MATERIALIZED VIEW calls_hourly
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 hour', start_time) AS hour,
    buyer_id,
    COUNT(*) AS call_count,
    AVG(duration)::INTEGER AS avg_duration,
    SUM(total_cost) AS total_cost
FROM calls
WHERE start_time > NOW() - INTERVAL '90 days' -- Only recent data
GROUP BY hour, buyer_id;

-- Refresh policy - once per hour is sufficient
SELECT add_continuous_aggregate_policy('calls_hourly',
    start_offset => INTERVAL '2 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

-- =============================================================================
-- PHASE 7: SECURITY (Simple RLS)
-- =============================================================================

-- Enable RLS
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE calls ENABLE ROW LEVEL SECURITY;
ALTER TABLE bids ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;

-- Simple policy: users can see their own data
CREATE POLICY own_account ON accounts
    FOR ALL
    USING (id = current_setting('app.current_user_id', true)::UUID);

CREATE POLICY own_calls ON calls
    FOR SELECT
    USING (
        buyer_id = current_setting('app.current_user_id', true)::UUID OR
        seller_id = current_setting('app.current_user_id', true)::UUID
    );

CREATE POLICY own_bids ON bids
    FOR ALL
    USING (buyer_id = current_setting('app.current_user_id', true)::UUID);

CREATE POLICY own_transactions ON transactions
    FOR SELECT
    USING (account_id = current_setting('app.current_user_id', true)::UUID);

-- =============================================================================
-- PHASE 8: INITIAL DATA
-- =============================================================================

-- System account for internal use
INSERT INTO accounts (
    id, email, name, type, status
) VALUES (
    '00000000-0000-0000-0000-000000000000',
    'system@dependablecall.exchange',
    'System Account',
    'admin',
    'active'
);

-- =============================================================================
-- CONFIGURATION NOTES
-- =============================================================================
-- 
-- Server configuration should be done at the infrastructure level, NOT here.
-- Recommended settings for production:
-- 
-- shared_buffers = 25% of RAM
-- effective_cache_size = 75% of RAM  
-- maintenance_work_mem = 1GB
-- work_mem = 50MB
-- max_connections = 200
-- 
-- For sharding, use a proper solution like Citus or application-level sharding.
-- Do NOT implement naive FDW sharding with hardcoded credentials.
--
-- =============================================================================