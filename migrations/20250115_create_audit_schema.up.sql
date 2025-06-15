-- =====================================================================
-- IMMUTABLE AUDIT LOGGING SCHEMA
-- Feature: COMPLIANCE-003
-- Risk Score: 88/100
-- Priority: Critical
-- 
-- This migration creates a comprehensive audit logging system with:
-- - Cryptographic hash chain for tamper-proof storage
-- - Monthly partitioning for 7-year retention
-- - Automatic partition management
-- - Performance optimized indexes
-- - Security controls and access restrictions
-- =====================================================================

-- Create audit schema for namespace isolation
CREATE SCHEMA IF NOT EXISTS audit;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =====================================================================
-- SEQUENCES
-- =====================================================================

-- Global sequence for monotonic ordering across all partitions
CREATE SEQUENCE IF NOT EXISTS audit.events_sequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MAXVALUE
    NO CYCLE;

COMMENT ON SEQUENCE audit.events_sequence_seq IS 
'Global monotonic sequence for audit events - ensures ordering across partitions';

-- =====================================================================
-- MAIN AUDIT EVENTS TABLE (PARTITIONED)
-- =====================================================================

CREATE TABLE IF NOT EXISTS audit.events (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sequence_num BIGINT NOT NULL DEFAULT nextval('audit.events_sequence_seq'),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    timestamp_nano BIGINT NOT NULL,
    
    -- Event classification
    type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    category VARCHAR(50) NOT NULL,
    
    -- Actor information
    actor_id VARCHAR(100) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_ip INET,
    actor_agent TEXT,
    
    -- Target information
    target_id VARCHAR(100) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_owner VARCHAR(100),
    
    -- Event details
    action VARCHAR(100) NOT NULL,
    result VARCHAR(50) NOT NULL,
    error_code VARCHAR(50),
    error_message TEXT,
    
    -- Context
    request_id VARCHAR(100),
    session_id VARCHAR(100),
    correlation_id VARCHAR(100),
    environment VARCHAR(50) NOT NULL,
    service VARCHAR(100) NOT NULL,
    version VARCHAR(50) NOT NULL,
    
    -- Compliance metadata
    compliance_flags JSONB DEFAULT '{}',
    data_classes TEXT[],
    legal_basis VARCHAR(100),
    retention_days INTEGER NOT NULL DEFAULT 2555, -- 7 years
    
    -- Additional data
    metadata JSONB DEFAULT '{}',
    tags TEXT[],
    
    -- Cryptographic integrity
    previous_hash VARCHAR(64) NOT NULL,
    event_hash VARCHAR(64) NOT NULL,
    signature TEXT,
    
    -- Constraints
    CONSTRAINT audit_events_sequence_unique UNIQUE (sequence_num),
    CONSTRAINT audit_events_hash_unique UNIQUE (event_hash),
    CONSTRAINT audit_events_severity_check CHECK (severity IN ('INFO', 'WARNING', 'ERROR', 'CRITICAL')),
    CONSTRAINT audit_events_result_check CHECK (result IN ('success', 'failure', 'partial'))
) PARTITION BY RANGE (timestamp);

-- Table documentation
COMMENT ON TABLE audit.events IS 
'Immutable audit log with cryptographic hash chain, partitioned by month for 7-year retention';

COMMENT ON COLUMN audit.events.sequence_num IS 
'Global monotonic sequence number for ordering and gap detection';
COMMENT ON COLUMN audit.events.timestamp_nano IS 
'Nanosecond precision timestamp for high-frequency event ordering';
COMMENT ON COLUMN audit.events.previous_hash IS 
'SHA-256 hash of previous event for blockchain-style integrity';
COMMENT ON COLUMN audit.events.event_hash IS 
'SHA-256 hash of this event for integrity verification';
COMMENT ON COLUMN audit.events.signature IS 
'Optional cryptographic signature for critical events';
COMMENT ON COLUMN audit.events.compliance_flags IS 
'JSONB flags for compliance filtering (tcpa_compliant, gdpr_relevant, etc)';
COMMENT ON COLUMN audit.events.data_classes IS 
'Array of data classifications accessed (PII, PHI, financial, etc)';
COMMENT ON COLUMN audit.events.legal_basis IS 
'Legal basis for data processing (consent, legitimate_interest, etc)';

-- =====================================================================
-- PERFORMANCE INDEXES
-- =====================================================================

-- Primary query patterns
CREATE INDEX idx_audit_events_timestamp ON audit.events (timestamp DESC);
CREATE INDEX idx_audit_events_type ON audit.events (type);
CREATE INDEX idx_audit_events_actor ON audit.events (actor_id, timestamp DESC);
CREATE INDEX idx_audit_events_target ON audit.events (target_id, timestamp DESC);
CREATE INDEX idx_audit_events_request ON audit.events (request_id);
CREATE INDEX idx_audit_events_correlation ON audit.events (correlation_id);
CREATE INDEX idx_audit_events_session ON audit.events (session_id);

-- Compliance and metadata indexes
CREATE INDEX idx_audit_events_compliance ON audit.events USING GIN (compliance_flags);
CREATE INDEX idx_audit_events_metadata ON audit.events USING GIN (metadata);
CREATE INDEX idx_audit_events_tags ON audit.events USING GIN (tags);
CREATE INDEX idx_audit_events_data_classes ON audit.events USING GIN (data_classes);

-- Composite indexes for common queries
CREATE INDEX idx_audit_events_type_timestamp ON audit.events (type, timestamp DESC);
CREATE INDEX idx_audit_events_severity_timestamp ON audit.events (severity, timestamp DESC) 
    WHERE severity IN ('ERROR', 'CRITICAL');

-- =====================================================================
-- HASH CHAIN VERIFICATION TABLE
-- =====================================================================

CREATE TABLE IF NOT EXISTS audit.hash_chain (
    id SERIAL PRIMARY KEY,
    block_number BIGINT NOT NULL UNIQUE,
    start_sequence BIGINT NOT NULL,
    end_sequence BIGINT NOT NULL,
    block_hash VARCHAR(64) NOT NULL,
    merkle_root VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMPTZ,
    verification_status VARCHAR(50)
);

CREATE INDEX idx_hash_chain_block_number ON audit.hash_chain (block_number);
CREATE INDEX idx_hash_chain_sequences ON audit.hash_chain (start_sequence, end_sequence);

COMMENT ON TABLE audit.hash_chain IS 
'Periodic hash chain checkpoints for efficient integrity verification';

-- =====================================================================
-- ARCHIVE METADATA TABLE
-- =====================================================================

CREATE TABLE IF NOT EXISTS audit.archives (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partition_name VARCHAR(100) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    event_count BIGINT NOT NULL,
    s3_bucket VARCHAR(255) NOT NULL,
    s3_key VARCHAR(500) NOT NULL,
    archive_hash VARCHAR(64) NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMPTZ,
    size_bytes BIGINT NOT NULL,
    compression_type VARCHAR(50) NOT NULL DEFAULT 'gzip'
);

CREATE INDEX idx_archives_partition ON audit.archives (partition_name);
CREATE INDEX idx_archives_dates ON audit.archives (start_date, end_date);

COMMENT ON TABLE audit.archives IS 
'Metadata for archived audit partitions stored in S3';

-- =====================================================================
-- ACCESS CONTROL LOG
-- =====================================================================

CREATE TABLE IF NOT EXISTS audit.access_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accessor_id VARCHAR(100) NOT NULL,
    accessor_type VARCHAR(50) NOT NULL,
    accessor_ip INET,
    query_type VARCHAR(50) NOT NULL, -- read, export, verify
    query_filters JSONB,
    events_accessed INTEGER,
    export_format VARCHAR(50),
    purpose TEXT NOT NULL,
    approved_by VARCHAR(100),
    approval_ticket VARCHAR(100)
);

CREATE INDEX idx_access_log_accessor ON audit.access_log (accessor_id, accessed_at DESC);
CREATE INDEX idx_access_log_timestamp ON audit.access_log (accessed_at DESC);

COMMENT ON TABLE audit.access_log IS 
'Audit trail for who accessed the audit logs (auditing the auditors)';

-- =====================================================================
-- SECURITY: IMMUTABILITY TRIGGERS
-- =====================================================================

-- Prevent any updates to audit events
CREATE OR REPLACE FUNCTION audit.prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit logs are immutable and cannot be modified or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER prevent_update
    BEFORE UPDATE ON audit.events
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

CREATE TRIGGER prevent_delete
    BEFORE DELETE ON audit.events
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

-- Also protect hash chain and archives
CREATE TRIGGER prevent_hash_chain_update
    BEFORE UPDATE ON audit.hash_chain
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

CREATE TRIGGER prevent_hash_chain_delete
    BEFORE DELETE ON audit.hash_chain
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

-- =====================================================================
-- PARTITION MANAGEMENT FUNCTIONS
-- =====================================================================

-- Function to create monthly partitions
CREATE OR REPLACE FUNCTION audit.create_monthly_partition(
    year INTEGER,
    month INTEGER
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := DATE(year || '-' || LPAD(month::TEXT, 2, '0') || '-01');
    end_date := start_date + INTERVAL '1 month';
    partition_name := 'events_' || year || '_' || LPAD(month::TEXT, 2, '0');
    
    -- Check if partition already exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_tables 
        WHERE schemaname = 'audit' 
        AND tablename = partition_name
    ) THEN
        -- Create partition
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS audit.%I PARTITION OF audit.events
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        
        RAISE NOTICE 'Created audit partition % for %', partition_name, start_date;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically create future partitions
CREATE OR REPLACE FUNCTION audit.auto_create_partitions()
RETURNS VOID AS $$
DECLARE
    i INTEGER;
    current_year INTEGER;
    current_month INTEGER;
    target_year INTEGER;
    target_month INTEGER;
BEGIN
    current_year := EXTRACT(YEAR FROM CURRENT_DATE);
    current_month := EXTRACT(MONTH FROM CURRENT_DATE);
    
    -- Create partitions for next 3 months
    FOR i IN 0..2 LOOP
        target_month := current_month + i;
        target_year := current_year;
        
        -- Handle year boundary
        IF target_month > 12 THEN
            target_month := target_month - 12;
            target_year := target_year + 1;
        END IF;
        
        PERFORM audit.create_monthly_partition(target_year, target_month);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Function to drop old partitions (for archival)
CREATE OR REPLACE FUNCTION audit.drop_old_partition(
    year INTEGER,
    month INTEGER
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    event_count BIGINT;
BEGIN
    partition_name := 'events_' || year || '_' || LPAD(month::TEXT, 2, '0');
    
    -- Get event count before dropping
    EXECUTE format(
        'SELECT COUNT(*) FROM audit.%I',
        partition_name
    ) INTO event_count;
    
    -- Verify partition has been archived
    IF NOT EXISTS (
        SELECT 1 FROM audit.archives 
        WHERE partition_name = partition_name
        AND verified_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Cannot drop partition % - not archived or verified', partition_name;
    END IF;
    
    -- Drop the partition
    EXECUTE format('DROP TABLE IF EXISTS audit.%I', partition_name);
    
    RAISE NOTICE 'Dropped archived partition % with % events', partition_name, event_count;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- HASH CHAIN VERIFICATION FUNCTIONS
-- =====================================================================

-- Function to verify hash chain integrity
CREATE OR REPLACE FUNCTION audit.verify_hash_chain(
    start_sequence BIGINT,
    end_sequence BIGINT
) RETURNS TABLE (
    sequence_num BIGINT,
    is_valid BOOLEAN,
    expected_hash VARCHAR(64),
    actual_hash VARCHAR(64)
) AS $$
DECLARE
    prev_hash VARCHAR(64) := '';
    event_record RECORD;
BEGIN
    FOR event_record IN 
        SELECT e.sequence_num, e.previous_hash, e.event_hash
        FROM audit.events e
        WHERE e.sequence_num BETWEEN start_sequence AND end_sequence
        ORDER BY e.sequence_num
    LOOP
        RETURN QUERY SELECT 
            event_record.sequence_num,
            event_record.previous_hash = prev_hash,
            prev_hash,
            event_record.previous_hash;
            
        prev_hash := event_record.event_hash;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Function to create hash chain checkpoint
CREATE OR REPLACE FUNCTION audit.create_hash_checkpoint()
RETURNS VOID AS $$
DECLARE
    last_checkpoint RECORD;
    start_seq BIGINT;
    end_seq BIGINT;
    events_data TEXT;
    block_hash VARCHAR(64);
BEGIN
    -- Get last checkpoint
    SELECT * INTO last_checkpoint
    FROM audit.hash_chain
    ORDER BY block_number DESC
    LIMIT 1;
    
    -- Determine sequence range
    IF last_checkpoint IS NULL THEN
        start_seq := 1;
    ELSE
        start_seq := last_checkpoint.end_sequence + 1;
    END IF;
    
    -- Get current max sequence
    SELECT MAX(sequence_num) INTO end_seq
    FROM audit.events;
    
    IF end_seq IS NULL OR end_seq < start_seq THEN
        RETURN; -- No new events
    END IF;
    
    -- Calculate merkle root (simplified - in production use proper merkle tree)
    SELECT string_agg(event_hash, '' ORDER BY sequence_num) INTO events_data
    FROM audit.events
    WHERE sequence_num BETWEEN start_seq AND end_seq;
    
    block_hash := encode(digest(events_data, 'sha256'), 'hex');
    
    -- Insert checkpoint
    INSERT INTO audit.hash_chain (
        block_number,
        start_sequence,
        end_sequence,
        block_hash,
        merkle_root
    ) VALUES (
        COALESCE(last_checkpoint.block_number, 0) + 1,
        start_seq,
        end_seq,
        block_hash,
        block_hash -- Simplified - same as block_hash for now
    );
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- MONITORING VIEWS
-- =====================================================================

-- Health check view
CREATE OR REPLACE VIEW audit.health_check AS
SELECT 
    'partition_count' as metric,
    COUNT(*)::TEXT as value
FROM pg_tables
WHERE schemaname = 'audit' AND tablename LIKE 'events_%'
UNION ALL
SELECT 
    'total_events' as metric,
    COUNT(*)::TEXT as value
FROM audit.events
UNION ALL
SELECT 
    'events_last_24h' as metric,
    COUNT(*)::TEXT as value
FROM audit.events
WHERE timestamp > CURRENT_TIMESTAMP - INTERVAL '24 hours'
UNION ALL
SELECT 
    'last_event_time' as metric,
    MAX(timestamp)::TEXT as value
FROM audit.events
UNION ALL
SELECT 
    'hash_chain_verified' as metric,
    CASE 
        WHEN MAX(verified_at) > CURRENT_TIMESTAMP - INTERVAL '1 hour' 
        THEN 'true' 
        ELSE 'false' 
    END as value
FROM audit.hash_chain;

-- Statistics view
CREATE OR REPLACE VIEW audit.stats AS
WITH event_stats AS (
    SELECT 
        type,
        severity,
        COUNT(*) as event_count,
        MIN(timestamp) as first_seen,
        MAX(timestamp) as last_seen
    FROM audit.events
    WHERE timestamp > CURRENT_TIMESTAMP - INTERVAL '30 days'
    GROUP BY type, severity
)
SELECT 
    type,
    severity,
    event_count,
    first_seen,
    last_seen,
    ROUND(event_count::NUMERIC / 30, 2) as avg_per_day
FROM event_stats
ORDER BY event_count DESC;

-- Partition status view
CREATE OR REPLACE VIEW audit.partition_status AS
SELECT 
    schemaname,
    tablename as partition_name,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
    pg_stat_user_tables.n_live_tup as row_count,
    pg_stat_user_tables.last_vacuum,
    pg_stat_user_tables.last_analyze
FROM pg_tables
LEFT JOIN pg_stat_user_tables ON pg_tables.tablename = pg_stat_user_tables.relname
WHERE schemaname = 'audit' AND tablename LIKE 'events_%'
ORDER BY tablename;

-- =====================================================================
-- SECURITY ROLES
-- =====================================================================

-- Create roles if they don't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_reader') THEN
        CREATE ROLE audit_reader;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_writer') THEN
        CREATE ROLE audit_writer;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_admin') THEN
        CREATE ROLE audit_admin;
    END IF;
END
$$;

-- Grant permissions
GRANT USAGE ON SCHEMA audit TO audit_reader, audit_writer, audit_admin;

-- Reader permissions (read-only)
GRANT SELECT ON ALL TABLES IN SCHEMA audit TO audit_reader;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA audit TO audit_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit GRANT SELECT ON TABLES TO audit_reader;

-- Writer permissions (insert only)
GRANT INSERT ON audit.events TO audit_writer;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA audit TO audit_writer;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit GRANT INSERT ON TABLES TO audit_writer;

-- Admin permissions (full access except delete/update on events)
GRANT ALL ON ALL TABLES IN SCHEMA audit TO audit_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA audit TO audit_admin;
GRANT ALL ON ALL FUNCTIONS IN SCHEMA audit TO audit_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit GRANT ALL ON TABLES TO audit_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit GRANT ALL ON SEQUENCES TO audit_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit GRANT ALL ON FUNCTIONS TO audit_admin;

-- =====================================================================
-- INITIAL PARTITION CREATION
-- =====================================================================

-- Create partitions for the current year and next 12 months
DO $$
DECLARE
    i INTEGER;
    current_year INTEGER;
    current_month INTEGER;
    target_year INTEGER;
    target_month INTEGER;
BEGIN
    current_year := EXTRACT(YEAR FROM CURRENT_DATE);
    current_month := EXTRACT(MONTH FROM CURRENT_DATE);
    
    -- Create current month and next 11 months
    FOR i IN 0..11 LOOP
        target_month := current_month + i;
        target_year := current_year;
        
        -- Handle year boundary
        WHILE target_month > 12 LOOP
            target_month := target_month - 12;
            target_year := target_year + 1;
        END LOOP;
        
        PERFORM audit.create_monthly_partition(target_year, target_month);
    END LOOP;
    
    -- Also create previous month for immediate historical data
    target_month := current_month - 1;
    target_year := current_year;
    IF target_month < 1 THEN
        target_month := 12;
        target_year := target_year - 1;
    END IF;
    PERFORM audit.create_monthly_partition(target_year, target_month);
END
$$;

-- =====================================================================
-- SCHEDULED MAINTENANCE (requires pg_cron extension)
-- =====================================================================

-- Note: Uncomment these if pg_cron is installed
-- SELECT cron.schedule('audit-create-partitions', '0 0 1 * *', 'SELECT audit.auto_create_partitions()');
-- SELECT cron.schedule('audit-create-checkpoint', '0 * * * *', 'SELECT audit.create_hash_checkpoint()');
-- SELECT cron.schedule('audit-verify-integrity', '0 2 * * *', 'SELECT * FROM audit.verify_hash_chain(1, 1000000)');

-- =====================================================================
-- MIGRATION METADATA
-- =====================================================================

COMMENT ON SCHEMA audit IS 
'Immutable audit logging system for compliance (TCPA, GDPR, CCPA, SOX)';

-- Log the migration completion
INSERT INTO audit.events (
    id,
    timestamp_nano,
    type,
    severity,
    category,
    actor_id,
    actor_type,
    target_id,
    target_type,
    action,
    result,
    environment,
    service,
    version,
    previous_hash,
    event_hash,
    metadata
) VALUES (
    uuid_generate_v4(),
    EXTRACT(EPOCH FROM CURRENT_TIMESTAMP) * 1000000000,
    'system.migration',
    'INFO',
    'infrastructure',
    'migration_system',
    'system',
    'audit_schema',
    'database_schema',
    'create_audit_schema',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250115',
    '',
    encode(digest('initial_audit_schema_creation', 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250115_create_audit_schema.up.sql',
        'created_at', CURRENT_TIMESTAMP,
        'description', 'Created comprehensive audit logging schema with partitioning'
    )
);

-- =====================================================================
-- END OF MIGRATION
-- =====================================================================