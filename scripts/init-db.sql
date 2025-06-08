-- Initialize database schema for Dependable Call Exchange

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set timezone
SET timezone = 'UTC';

-- Create enum types
CREATE TYPE account_type AS ENUM ('buyer', 'seller', 'admin');
CREATE TYPE account_status AS ENUM ('pending', 'active', 'suspended', 'banned', 'closed');
CREATE TYPE call_status AS ENUM ('pending', 'queued', 'ringing', 'in_progress', 'completed', 'failed', 'canceled', 'no_answer', 'busy');
CREATE TYPE call_direction AS ENUM ('inbound', 'outbound');
CREATE TYPE bid_status AS ENUM ('pending', 'active', 'winning', 'won', 'lost', 'expired', 'canceled');
CREATE TYPE auction_status AS ENUM ('pending', 'active', 'completed', 'canceled', 'expired');
CREATE TYPE rule_type AS ENUM ('tcpa', 'gdpr', 'ccpa', 'dnc', 'custom');
CREATE TYPE rule_status AS ENUM ('draft', 'active', 'inactive', 'expired');
CREATE TYPE violation_type AS ENUM ('tcpa', 'gdpr', 'dnc', 'time_restriction', 'consent', 'fraud');
CREATE TYPE severity AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE consent_type AS ENUM ('tcpa', 'gdpr', 'ccpa', 'marketing');
CREATE TYPE consent_status AS ENUM ('active', 'expired', 'revoked', 'pending');

-- Accounts table
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    type account_type NOT NULL,
    status account_status NOT NULL DEFAULT 'pending',
    company VARCHAR(255),
    phone_number VARCHAR(50) NOT NULL,
    
    -- Address
    street VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    zip_code VARCHAR(20),
    country VARCHAR(100),
    
    -- Financial
    balance DECIMAL(12,2) DEFAULT 0.00,
    credit_limit DECIMAL(12,2) DEFAULT 1000.00,
    payment_terms INTEGER DEFAULT 30,
    
    -- Compliance
    tcpa_consent BOOLEAN DEFAULT false,
    gdpr_consent BOOLEAN DEFAULT false,
    compliance_flags TEXT[],
    
    -- Quality metrics
    quality_score DECIMAL(3,2) DEFAULT 5.00,
    fraud_score DECIMAL(3,2) DEFAULT 0.00,
    
    -- Settings (JSON)
    settings JSONB DEFAULT '{}',
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

-- Calls table
CREATE TABLE calls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_number VARCHAR(50) NOT NULL,
    to_number VARCHAR(50) NOT NULL,
    status call_status NOT NULL DEFAULT 'pending',
    direction call_direction NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    duration INTEGER, -- in seconds
    cost DECIMAL(10,4),
    
    -- Routing
    route_id UUID,
    buyer_id UUID NOT NULL REFERENCES accounts(id),
    seller_id UUID REFERENCES accounts(id),
    
    -- Telephony
    call_sid VARCHAR(255) UNIQUE NOT NULL,
    session_id VARCHAR(255),
    
    -- Metadata
    user_agent TEXT,
    ip_address INET,
    location JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Auctions table
CREATE TABLE auctions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    call_id UUID NOT NULL REFERENCES calls(id),
    status auction_status NOT NULL DEFAULT 'pending',
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    winning_bid UUID,
    
    -- Auction parameters
    reserve_price DECIMAL(10,4) NOT NULL,
    bid_increment DECIMAL(10,4) DEFAULT 0.01,
    max_duration INTEGER DEFAULT 30, -- seconds
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Bids table
CREATE TABLE bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    call_id UUID NOT NULL REFERENCES calls(id),
    buyer_id UUID NOT NULL REFERENCES accounts(id),
    seller_id UUID NOT NULL REFERENCES accounts(id),
    amount DECIMAL(10,4) NOT NULL,
    status bid_status NOT NULL DEFAULT 'pending',
    
    -- Auction details
    auction_id UUID NOT NULL REFERENCES auctions(id),
    rank INTEGER,
    
    -- Targeting criteria (JSON)
    criteria JSONB NOT NULL DEFAULT '{}',
    
    -- Quality metrics (JSON)
    quality JSONB NOT NULL DEFAULT '{}',
    
    placed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Compliance rules table
CREATE TABLE compliance_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type rule_type NOT NULL,
    status rule_status NOT NULL DEFAULT 'draft',
    priority INTEGER DEFAULT 1,
    
    -- Rule definition (JSON)
    conditions JSONB NOT NULL DEFAULT '[]',
    actions JSONB NOT NULL DEFAULT '[]',
    
    -- Geographic scope (JSON)
    geography JSONB DEFAULT '{}',
    
    -- Time restrictions (JSON)
    time_windows JSONB DEFAULT '[]',
    
    description TEXT,
    created_by UUID NOT NULL REFERENCES accounts(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    effective_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Compliance violations table
CREATE TABLE compliance_violations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    call_id UUID NOT NULL REFERENCES calls(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    rule_id UUID NOT NULL REFERENCES compliance_rules(id),
    violation_type violation_type NOT NULL,
    severity severity NOT NULL,
    description TEXT NOT NULL,
    resolved BOOLEAN DEFAULT false,
    resolved_by UUID REFERENCES accounts(id),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Consent records table
CREATE TABLE consent_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number VARCHAR(50) NOT NULL,
    consent_type consent_type NOT NULL,
    status consent_status NOT NULL DEFAULT 'active',
    source VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT NOT NULL,
    opt_in_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    opt_out_timestamp TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add indexes for performance
CREATE INDEX idx_accounts_email ON accounts(email);
CREATE INDEX idx_accounts_type ON accounts(type);
CREATE INDEX idx_accounts_status ON accounts(status);

CREATE INDEX idx_calls_status ON calls(status);
CREATE INDEX idx_calls_buyer_id ON calls(buyer_id);
CREATE INDEX idx_calls_seller_id ON calls(seller_id);
CREATE INDEX idx_calls_start_time ON calls(start_time);
CREATE INDEX idx_calls_from_number ON calls(from_number);
CREATE INDEX idx_calls_to_number ON calls(to_number);

CREATE INDEX idx_bids_auction_id ON bids(auction_id);
CREATE INDEX idx_bids_buyer_id ON bids(buyer_id);
CREATE INDEX idx_bids_seller_id ON bids(seller_id);
CREATE INDEX idx_bids_status ON bids(status);
CREATE INDEX idx_bids_amount ON bids(amount);

CREATE INDEX idx_auctions_call_id ON auctions(call_id);
CREATE INDEX idx_auctions_status ON auctions(status);
CREATE INDEX idx_auctions_start_time ON auctions(start_time);

CREATE INDEX idx_compliance_rules_type ON compliance_rules(type);
CREATE INDEX idx_compliance_rules_status ON compliance_rules(status);

CREATE INDEX idx_compliance_violations_call_id ON compliance_violations(call_id);
CREATE INDEX idx_compliance_violations_account_id ON compliance_violations(account_id);
CREATE INDEX idx_compliance_violations_resolved ON compliance_violations(resolved);

CREATE INDEX idx_consent_records_phone_number ON consent_records(phone_number);
CREATE INDEX idx_consent_records_status ON consent_records(status);
CREATE INDEX idx_consent_records_type ON consent_records(consent_type);

-- Add foreign key for winning bid
ALTER TABLE auctions ADD CONSTRAINT fk_auctions_winning_bid 
    FOREIGN KEY (winning_bid) REFERENCES bids(id);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add updated_at triggers
CREATE TRIGGER set_timestamp_accounts
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_calls
    BEFORE UPDATE ON calls
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_auctions
    BEFORE UPDATE ON auctions
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_bids
    BEFORE UPDATE ON bids
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_compliance_rules
    BEFORE UPDATE ON compliance_rules
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_consent_records
    BEFORE UPDATE ON consent_records
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();