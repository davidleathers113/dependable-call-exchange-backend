-- Create audit events table with partitioning support
-- Following DCE patterns: PostgreSQL partitioning by month for scalability

-- Create sequence for monotonic sequence numbers
CREATE SEQUENCE IF NOT EXISTS audit_events_sequence_number_seq;

-- Create main partitioned table
CREATE TABLE IF NOT EXISTS audit_events (
    -- Primary identifiers
    id UUID PRIMARY KEY,
    sequence_number BIGINT NOT NULL UNIQUE DEFAULT nextval('audit_events_sequence_number_seq'),
    
    -- Event classification
    event_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    
    -- Actor information
    actor_id VARCHAR(255) NOT NULL,
    actor_type VARCHAR(50),
    
    -- Target information
    target_id VARCHAR(255) NOT NULL,
    target_type VARCHAR(50),
    
    -- Action details
    action VARCHAR(100) NOT NULL,
    result VARCHAR(20) NOT NULL,
    
    -- Flexible metadata storage
    metadata JSONB DEFAULT '{}',
    compliance_flags JSONB DEFAULT '{}',
    
    -- Request context
    ip_address VARCHAR(45),
    user_agent TEXT,
    session_id VARCHAR(255),
    correlation_id VARCHAR(255),
    
    -- Cryptographic integrity
    hash VARCHAR(64) NOT NULL,
    previous_hash VARCHAR(64),
    
    -- Temporal data
    timestamp TIMESTAMPTZ NOT NULL,
    
    -- Archival flag
    archived BOOLEAN DEFAULT FALSE,
    
    -- Constraints
    CONSTRAINT audit_events_result_check CHECK (result IN ('success', 'failure', 'partial')),
    CONSTRAINT audit_events_severity_check CHECK (severity IN ('INFO', 'WARNING', 'ERROR', 'CRITICAL'))
) PARTITION BY RANGE (timestamp);

-- Create indexes on parent table (will be inherited by partitions)
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events (timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_events_event_type ON audit_events (event_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor_id ON audit_events (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_target_id ON audit_events (target_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_correlation_id ON audit_events (correlation_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_session_id ON audit_events (session_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_compliance_flags ON audit_events USING GIN (compliance_flags);
CREATE INDEX IF NOT EXISTS idx_audit_events_metadata ON audit_events USING GIN (metadata);

-- Create function to automatically create monthly partitions
CREATE OR REPLACE FUNCTION create_monthly_audit_partition(start_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_timestamp TEXT;
    end_timestamp TEXT;
BEGIN
    -- Generate partition name (e.g., audit_events_2025_01)
    partition_name := 'audit_events_' || to_char(start_date, 'YYYY_MM');
    
    -- Calculate partition boundaries
    start_timestamp := to_char(start_date, 'YYYY-MM-DD');
    end_timestamp := to_char(start_date + INTERVAL '1 month', 'YYYY-MM-DD');
    
    -- Create partition if it doesn't exist
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_events
        FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_timestamp, end_timestamp);
    
    -- Log partition creation
    RAISE NOTICE 'Created partition % for range % to %', partition_name, start_timestamp, end_timestamp;
END;
$$ LANGUAGE plpgsql;

-- Create partitions for the next 12 months
DO $$
DECLARE
    i INTEGER;
    partition_date DATE;
BEGIN
    FOR i IN 0..11 LOOP
        partition_date := DATE_TRUNC('month', CURRENT_DATE) + (i * INTERVAL '1 month');
        PERFORM create_monthly_audit_partition(partition_date);
    END LOOP;
END $$;

-- Create function to automatically maintain partitions
CREATE OR REPLACE FUNCTION maintain_audit_partitions()
RETURNS VOID AS $$
DECLARE
    future_date DATE;
    old_date DATE;
    partition_name TEXT;
BEGIN
    -- Create partition for 3 months ahead if it doesn't exist
    future_date := DATE_TRUNC('month', CURRENT_DATE + INTERVAL '3 months');
    PERFORM create_monthly_audit_partition(future_date);
    
    -- Optionally: Drop partitions older than retention period (e.g., 7 years)
    -- Uncomment and adjust as needed:
    -- old_date := DATE_TRUNC('month', CURRENT_DATE - INTERVAL '7 years');
    -- partition_name := 'audit_events_' || to_char(old_date, 'YYYY_MM');
    -- EXECUTE format('DROP TABLE IF EXISTS %I', partition_name);
END;
$$ LANGUAGE plpgsql;

-- Create a scheduled job to maintain partitions (requires pg_cron extension)
-- Uncomment if pg_cron is available:
-- SELECT cron.schedule('maintain-audit-partitions', '0 0 1 * *', 'SELECT maintain_audit_partitions()');

-- Create view for easy querying of recent events (last 30 days)
CREATE OR REPLACE VIEW audit_events_recent AS
SELECT * FROM audit_events
WHERE timestamp >= CURRENT_TIMESTAMP - INTERVAL '30 days'
  AND archived = FALSE;

-- Create function to check partition health
CREATE OR REPLACE FUNCTION check_audit_partition_health()
RETURNS TABLE(
    partition_name TEXT,
    size_pretty TEXT,
    row_count BIGINT,
    date_range TEXT,
    is_current BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        pg_class.relname::TEXT AS partition_name,
        pg_size_pretty(pg_total_relation_size(pg_class.oid)) AS size_pretty,
        pg_class.reltuples::BIGINT AS row_count,
        pg_get_expr(pg_class.relpartbound, pg_class.oid)::TEXT AS date_range,
        (pg_get_expr(pg_class.relpartbound, pg_class.oid)::TEXT LIKE '%' || to_char(CURRENT_DATE, 'YYYY-MM') || '%')::BOOLEAN AS is_current
    FROM pg_class
    JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
    WHERE pg_class.relname LIKE 'audit_events_%'
      AND pg_class.relkind = 'r'
      AND pg_namespace.nspname = 'public'
    ORDER BY pg_class.relname;
END;
$$ LANGUAGE plpgsql;

-- Add comment documentation
COMMENT ON TABLE audit_events IS 'Immutable audit log with cryptographic hash chain and monthly partitioning';
COMMENT ON COLUMN audit_events.sequence_number IS 'Global monotonic sequence number for ordering and gap detection';
COMMENT ON COLUMN audit_events.hash IS 'SHA-256 hash of event data for integrity verification';
COMMENT ON COLUMN audit_events.previous_hash IS 'Hash of previous event for chain verification';
COMMENT ON COLUMN audit_events.compliance_flags IS 'JSONB flags for compliance filtering (gdpr_relevant, tcpa_relevant, etc)';
COMMENT ON COLUMN audit_events.metadata IS 'Flexible JSONB storage for event-specific data';

-- Create rollback migration
-- DROP TABLE IF EXISTS audit_events CASCADE;
-- DROP SEQUENCE IF EXISTS audit_events_sequence_number_seq CASCADE;
-- DROP FUNCTION IF EXISTS create_monthly_audit_partition(DATE) CASCADE;
-- DROP FUNCTION IF EXISTS maintain_audit_partitions() CASCADE;
-- DROP FUNCTION IF EXISTS check_audit_partition_health() CASCADE;
-- DROP VIEW IF EXISTS audit_events_recent CASCADE;