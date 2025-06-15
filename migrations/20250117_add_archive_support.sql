-- =====================================================================
-- Migration: Add Archive Support to Audit Events
-- Date: 2025-01-17
-- Description: Adds archive tracking columns to support S3 archival
-- =====================================================================

-- Add archived flag and archive_id to events table
-- Note: We need to add these to the parent table and all existing partitions

-- Function to add columns to all partitions
CREATE OR REPLACE FUNCTION audit.add_archive_columns_to_partitions()
RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
BEGIN
    -- Add columns to parent table first
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'audit' 
        AND table_name = 'events' 
        AND column_name = 'archived'
    ) THEN
        ALTER TABLE audit.events ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;
        ALTER TABLE audit.events ADD COLUMN archive_id VARCHAR(255);
        
        -- Add indexes for archive queries
        CREATE INDEX idx_audit_events_archived ON audit.events (archived) WHERE archived = TRUE;
        CREATE INDEX idx_audit_events_archive_id ON audit.events (archive_id) WHERE archive_id IS NOT NULL;
        
        -- Add comments
        COMMENT ON COLUMN audit.events.archived IS 'Flag indicating if event has been archived to S3';
        COMMENT ON COLUMN audit.events.archive_id IS 'S3 archive ID where this event is stored';
    END IF;
    
    -- Now add to all existing partitions
    FOR partition_record IN 
        SELECT schemaname, tablename 
        FROM pg_tables 
        WHERE schemaname = 'audit' 
        AND tablename LIKE 'events_%'
    LOOP
        -- Check if column already exists in partition
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_schema = partition_record.schemaname 
            AND table_name = partition_record.tablename 
            AND column_name = 'archived'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I.%I ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE',
                partition_record.schemaname,
                partition_record.tablename
            );
            
            EXECUTE format(
                'ALTER TABLE %I.%I ADD COLUMN archive_id VARCHAR(255)',
                partition_record.schemaname,
                partition_record.tablename
            );
            
            RAISE NOTICE 'Added archive columns to partition %', partition_record.tablename;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Execute the function
SELECT audit.add_archive_columns_to_partitions();

-- Drop the function as it's no longer needed
DROP FUNCTION audit.add_archive_columns_to_partitions();

-- =====================================================================
-- Archive Index Table
-- =====================================================================

-- Create archive index table for efficient archive lookups
CREATE TABLE IF NOT EXISTS audit.archive_index (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    archive_id VARCHAR(255) NOT NULL UNIQUE,
    
    -- Archive file information
    s3_bucket VARCHAR(255) NOT NULL,
    s3_key VARCHAR(500) NOT NULL,
    
    -- Event range information
    start_sequence BIGINT NOT NULL,
    end_sequence BIGINT NOT NULL,
    start_timestamp TIMESTAMPTZ NOT NULL,
    end_timestamp TIMESTAMPTZ NOT NULL,
    event_count BIGINT NOT NULL,
    
    -- Archive metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    compressed_size BIGINT NOT NULL,
    uncompressed_size BIGINT NOT NULL,
    compression_ratio NUMERIC(5,2) NOT NULL,
    compression_type VARCHAR(50) NOT NULL DEFAULT 'snappy',
    
    -- Integrity information
    hash_chain_valid BOOLEAN NOT NULL DEFAULT TRUE,
    first_event_hash VARCHAR(64) NOT NULL,
    last_event_hash VARCHAR(64) NOT NULL,
    
    -- Compliance metadata
    compliance_flags JSONB DEFAULT '{}',
    
    -- Retention information
    retention_days INTEGER NOT NULL DEFAULT 2555, -- 7 years
    expires_at TIMESTAMPTZ NOT NULL,
    legal_hold BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, EXPIRED, DELETED
    
    -- Search optimization
    CONSTRAINT archive_index_sequence_range CHECK (start_sequence <= end_sequence),
    CONSTRAINT archive_index_timestamp_range CHECK (start_timestamp <= end_timestamp)
);

-- Indexes for efficient lookups
CREATE INDEX idx_archive_index_sequences ON audit.archive_index (start_sequence, end_sequence);
CREATE INDEX idx_archive_index_timestamps ON audit.archive_index (start_timestamp, end_timestamp);
CREATE INDEX idx_archive_index_status ON audit.archive_index (status) WHERE status != 'DELETED';
CREATE INDEX idx_archive_index_expires ON audit.archive_index (expires_at) WHERE status = 'ACTIVE';
CREATE INDEX idx_archive_index_compliance ON audit.archive_index USING GIN (compliance_flags);

COMMENT ON TABLE audit.archive_index IS 
'Index for archived audit events stored in S3, enables efficient archive queries';

-- =====================================================================
-- Archive Operations Log
-- =====================================================================

CREATE TABLE IF NOT EXISTS audit.archive_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    operation_type VARCHAR(50) NOT NULL, -- ARCHIVE, RESTORE, DELETE, VERIFY
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    
    -- Operation details
    archive_id VARCHAR(255),
    event_count BIGINT,
    success BOOLEAN,
    error_message TEXT,
    
    -- Performance metrics
    duration_ms BIGINT,
    events_per_second NUMERIC(10,2),
    
    -- Operator information
    initiated_by VARCHAR(100) NOT NULL,
    initiated_from INET,
    
    -- Additional context
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_archive_operations_type ON audit.archive_operations (operation_type, started_at DESC);
CREATE INDEX idx_archive_operations_archive ON audit.archive_operations (archive_id);

COMMENT ON TABLE audit.archive_operations IS 
'Log of all archive operations for audit and performance tracking';

-- =====================================================================
-- Functions for Archive Management
-- =====================================================================

-- Function to get events ready for archival
CREATE OR REPLACE FUNCTION audit.get_archivable_events(
    older_than TIMESTAMPTZ,
    batch_size INTEGER DEFAULT 1000
) RETURNS TABLE (
    id UUID,
    sequence_num BIGINT,
    timestamp TIMESTAMPTZ,
    type VARCHAR(100),
    event_hash VARCHAR(64)
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.id,
        e.sequence_num,
        e.timestamp,
        e.type,
        e.event_hash
    FROM audit.events e
    WHERE e.timestamp < older_than
        AND e.archived = FALSE
    ORDER BY e.sequence_num
    LIMIT batch_size;
END;
$$ LANGUAGE plpgsql;

-- Function to mark events as archived
CREATE OR REPLACE FUNCTION audit.mark_events_archived(
    event_ids UUID[],
    p_archive_id VARCHAR(255)
) RETURNS INTEGER AS $$
DECLARE
    updated_count INTEGER;
BEGIN
    UPDATE audit.events
    SET 
        archived = TRUE,
        archive_id = p_archive_id
    WHERE id = ANY(event_ids)
        AND archived = FALSE;
    
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    
    RETURN updated_count;
END;
$$ LANGUAGE plpgsql;

-- Function to find archive containing a specific event
CREATE OR REPLACE FUNCTION audit.find_archive_for_event(
    event_id UUID
) RETURNS TABLE (
    archive_id VARCHAR(255),
    s3_bucket VARCHAR(255),
    s3_key VARCHAR(500)
) AS $$
BEGIN
    -- First check if event is archived
    RETURN QUERY
    SELECT 
        e.archive_id,
        ai.s3_bucket,
        ai.s3_key
    FROM audit.events e
    JOIN audit.archive_index ai ON e.archive_id = ai.archive_id
    WHERE e.id = event_id
        AND e.archived = TRUE;
END;
$$ LANGUAGE plpgsql;

-- Function to find archives by sequence range
CREATE OR REPLACE FUNCTION audit.find_archives_by_sequence(
    start_seq BIGINT,
    end_seq BIGINT
) RETURNS TABLE (
    archive_id VARCHAR(255),
    s3_bucket VARCHAR(255),
    s3_key VARCHAR(500),
    start_sequence BIGINT,
    end_sequence BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        ai.archive_id,
        ai.s3_bucket,
        ai.s3_key,
        ai.start_sequence,
        ai.end_sequence
    FROM audit.archive_index ai
    WHERE ai.status = 'ACTIVE'
        AND (
            (ai.start_sequence <= start_seq AND ai.end_sequence >= start_seq) OR
            (ai.start_sequence <= end_seq AND ai.end_sequence >= end_seq) OR
            (ai.start_sequence >= start_seq AND ai.end_sequence <= end_seq)
        )
    ORDER BY ai.start_sequence;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- Monitoring Views
-- =====================================================================

-- View for archive statistics
CREATE OR REPLACE VIEW audit.archive_stats AS
SELECT 
    COUNT(*) as total_archives,
    COUNT(*) FILTER (WHERE status = 'ACTIVE') as active_archives,
    COUNT(*) FILTER (WHERE status = 'EXPIRED') as expired_archives,
    COUNT(*) FILTER (WHERE legal_hold = TRUE) as legal_hold_archives,
    SUM(event_count) as total_archived_events,
    SUM(compressed_size) as total_compressed_size,
    SUM(uncompressed_size) as total_uncompressed_size,
    AVG(compression_ratio)::NUMERIC(5,2) as avg_compression_ratio,
    MIN(start_timestamp) as oldest_archive,
    MAX(end_timestamp) as newest_archive,
    COUNT(*) FILTER (WHERE expires_at < CURRENT_TIMESTAMP) as archives_pending_deletion
FROM audit.archive_index;

-- View for archival progress
CREATE OR REPLACE VIEW audit.archival_progress AS
WITH event_counts AS (
    SELECT 
        COUNT(*) FILTER (WHERE archived = FALSE) as unarchived_count,
        COUNT(*) FILTER (WHERE archived = TRUE) as archived_count,
        MIN(timestamp) FILTER (WHERE archived = FALSE) as oldest_unarchived,
        MAX(timestamp) FILTER (WHERE archived = FALSE) as newest_unarchived
    FROM audit.events
    WHERE timestamp < CURRENT_TIMESTAMP - INTERVAL '90 days'
)
SELECT 
    unarchived_count,
    archived_count,
    CASE 
        WHEN (unarchived_count + archived_count) > 0 
        THEN ROUND((archived_count::NUMERIC / (unarchived_count + archived_count)) * 100, 2)
        ELSE 100
    END as archival_percentage,
    oldest_unarchived,
    newest_unarchived,
    CASE 
        WHEN unarchived_count > 0 
        THEN CURRENT_TIMESTAMP - oldest_unarchived
        ELSE NULL
    END as archival_backlog
FROM event_counts;

-- =====================================================================
-- Security Updates
-- =====================================================================

-- Grant permissions for archive tables
GRANT SELECT ON audit.archive_index TO audit_reader;
GRANT SELECT ON audit.archive_operations TO audit_reader;
GRANT SELECT ON audit.archive_stats TO audit_reader;
GRANT SELECT ON audit.archival_progress TO audit_reader;

GRANT INSERT ON audit.archive_index TO audit_admin;
GRANT INSERT, UPDATE ON audit.archive_operations TO audit_admin;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA audit TO audit_admin;

-- =====================================================================
-- Migration Completion
-- =====================================================================

-- Log the migration
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
    'add_archive_support',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250117',
    '',
    encode(digest('add_archive_support_migration', 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250117_add_archive_support.sql',
        'created_at', CURRENT_TIMESTAMP,
        'description', 'Added S3 archive support with tracking columns and index tables'
    )
);

-- =====================================================================
-- END OF MIGRATION
-- =====================================================================