-- Create consent management schema tables
-- This migration creates the core tables for the consent management system

-- Create consumers table
CREATE TABLE IF NOT EXISTS consent_consumers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20),
    email VARCHAR(255),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT check_contact CHECK (phone_number IS NOT NULL OR email IS NOT NULL)
);

-- Create indexes for consumers
CREATE INDEX idx_consent_consumers_phone ON consent_consumers(phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX idx_consent_consumers_email ON consent_consumers(email) WHERE email IS NOT NULL;
CREATE INDEX idx_consent_consumers_created_at ON consent_consumers(created_at);

-- Create consent_aggregates table
CREATE TABLE IF NOT EXISTS consent_aggregates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_id UUID NOT NULL REFERENCES consent_consumers(id),
    business_id UUID NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_consent_consumer_business UNIQUE (consumer_id, business_id)
);

-- Create indexes for consent_aggregates
CREATE INDEX idx_consent_aggregates_consumer_id ON consent_aggregates(consumer_id);
CREATE INDEX idx_consent_aggregates_business_id ON consent_aggregates(business_id);
CREATE INDEX idx_consent_aggregates_consumer_business ON consent_aggregates(consumer_id, business_id);
CREATE INDEX idx_consent_aggregates_created_at ON consent_aggregates(created_at);

-- Create consent_versions table
CREATE TABLE IF NOT EXISTS consent_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id UUID NOT NULL REFERENCES consent_aggregates(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'active', 'revoked', 'expired')),
    channels TEXT[] NOT NULL CHECK (array_length(channels, 1) > 0),
    purpose VARCHAR(50) NOT NULL,
    source VARCHAR(50) NOT NULL,
    source_details JSONB DEFAULT '{}',
    consented_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_consent_version UNIQUE (consent_id, version_number)
);

-- Create indexes for consent_versions
CREATE INDEX idx_consent_versions_consent_id ON consent_versions(consent_id);
CREATE INDEX idx_consent_versions_status ON consent_versions(status);
CREATE INDEX idx_consent_versions_channels ON consent_versions USING GIN (channels);
CREATE INDEX idx_consent_versions_expires_at ON consent_versions(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_consent_versions_created_at ON consent_versions(created_at);

-- Create consent_proofs table
CREATE TABLE IF NOT EXISTS consent_proofs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_version_id UUID NOT NULL REFERENCES consent_versions(id) ON DELETE CASCADE,
    proof_type VARCHAR(50) NOT NULL,
    storage_location TEXT NOT NULL,
    hash VARCHAR(256) NOT NULL,
    algorithm VARCHAR(20) NOT NULL DEFAULT 'SHA256',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for consent_proofs
CREATE INDEX idx_consent_proofs_version_id ON consent_proofs(consent_version_id);
CREATE INDEX idx_consent_proofs_hash ON consent_proofs(hash);
CREATE INDEX idx_consent_proofs_created_at ON consent_proofs(created_at);

-- Create consent_events table for event sourcing
CREATE TABLE IF NOT EXISTS consent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL
);

-- Create indexes for consent_events
CREATE INDEX idx_consent_events_aggregate_id ON consent_events(aggregate_id);
CREATE INDEX idx_consent_events_event_type ON consent_events(event_type);
CREATE INDEX idx_consent_events_occurred_at ON consent_events(occurred_at);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_consent_consumers_updated_at BEFORE UPDATE ON consent_consumers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_consent_aggregates_updated_at BEFORE UPDATE ON consent_aggregates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create view for active consents
CREATE OR REPLACE VIEW active_consents AS
SELECT 
    ca.id,
    ca.consumer_id,
    ca.business_id,
    cv.status,
    cv.channels,
    cv.purpose,
    cv.consented_at,
    cv.expires_at,
    cc.phone_number,
    cc.email
FROM consent_aggregates ca
JOIN consent_versions cv ON cv.consent_id = ca.id AND cv.version_number = ca.current_version
JOIN consent_consumers cc ON cc.id = ca.consumer_id
WHERE cv.status = 'active'
AND (cv.expires_at IS NULL OR cv.expires_at > NOW());

-- Create materialized view for consent analytics
CREATE MATERIALIZED VIEW consent_analytics AS
SELECT 
    ca.business_id,
    cv.purpose,
    cv.source,
    DATE_TRUNC('day', cv.created_at) as consent_date,
    COUNT(DISTINCT ca.consumer_id) as total_consents,
    COUNT(DISTINCT CASE WHEN cv.status = 'active' THEN ca.consumer_id END) as active_consents,
    COUNT(DISTINCT CASE WHEN cv.status = 'revoked' THEN ca.consumer_id END) as revoked_consents,
    array_agg(DISTINCT cv.channels) as channels_used
FROM consent_aggregates ca
JOIN consent_versions cv ON cv.consent_id = ca.id
GROUP BY ca.business_id, cv.purpose, cv.source, DATE_TRUNC('day', cv.created_at);

-- Create index on materialized view
CREATE INDEX idx_consent_analytics_business_date ON consent_analytics(business_id, consent_date);

-- Add comments for documentation
COMMENT ON TABLE consent_consumers IS 'Stores consumer information for consent management';
COMMENT ON TABLE consent_aggregates IS 'Root aggregate for consent records, links consumers to businesses';
COMMENT ON TABLE consent_versions IS 'Stores versioned consent records with full history';
COMMENT ON TABLE consent_proofs IS 'Stores proof of consent (recordings, documents, etc.)';
COMMENT ON TABLE consent_events IS 'Event store for consent domain events';
COMMENT ON VIEW active_consents IS 'Convenient view of currently active consents';
COMMENT ON MATERIALIZED VIEW consent_analytics IS 'Aggregated consent metrics for analytics';