-- =====================================================================
-- Rollback Migration: Remove Archive Support from Audit Events
-- Date: 2025-01-17
-- Description: Removes archive tracking columns and related tables
-- =====================================================================

-- Drop views first
DROP VIEW IF EXISTS audit.archival_progress;
DROP VIEW IF EXISTS audit.archive_stats;

-- Drop functions
DROP FUNCTION IF EXISTS audit.find_archives_by_sequence(BIGINT, BIGINT);
DROP FUNCTION IF EXISTS audit.find_archive_for_event(UUID);
DROP FUNCTION IF EXISTS audit.mark_events_archived(UUID[], VARCHAR(255));
DROP FUNCTION IF EXISTS audit.get_archivable_events(TIMESTAMPTZ, INTEGER);

-- Drop tables
DROP TABLE IF EXISTS audit.archive_operations;
DROP TABLE IF EXISTS audit.archive_index;

-- Function to remove columns from all partitions
CREATE OR REPLACE FUNCTION audit.remove_archive_columns_from_partitions()
RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
BEGIN
    -- Remove from all existing partitions first
    FOR partition_record IN 
        SELECT schemaname, tablename 
        FROM pg_tables 
        WHERE schemaname = 'audit' 
        AND tablename LIKE 'events_%'
    LOOP
        -- Check if column exists in partition
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_schema = partition_record.schemaname 
            AND table_name = partition_record.tablename 
            AND column_name = 'archived'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I.%I DROP COLUMN IF EXISTS archived',
                partition_record.schemaname,
                partition_record.tablename
            );
            
            EXECUTE format(
                'ALTER TABLE %I.%I DROP COLUMN IF EXISTS archive_id',
                partition_record.schemaname,
                partition_record.tablename
            );
            
            RAISE NOTICE 'Removed archive columns from partition %', partition_record.tablename;
        END IF;
    END LOOP;
    
    -- Remove indexes from parent table
    DROP INDEX IF EXISTS audit.idx_audit_events_archived;
    DROP INDEX IF EXISTS audit.idx_audit_events_archive_id;
    
    -- Remove columns from parent table
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'audit' 
        AND table_name = 'events' 
        AND column_name = 'archived'
    ) THEN
        ALTER TABLE audit.events DROP COLUMN IF EXISTS archived;
        ALTER TABLE audit.events DROP COLUMN IF EXISTS archive_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Execute the function
SELECT audit.remove_archive_columns_from_partitions();

-- Drop the function as it's no longer needed
DROP FUNCTION audit.remove_archive_columns_from_partitions();

-- Log the rollback
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
    'WARNING',
    'infrastructure',
    'migration_system',
    'system',
    'audit_schema',
    'database_schema',
    'rollback_archive_support',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250117',
    '',
    encode(digest('rollback_archive_support_migration', 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250117_add_archive_support.down.sql',
        'created_at', CURRENT_TIMESTAMP,
        'description', 'Rolled back S3 archive support'
    )
);

-- =====================================================================
-- END OF ROLLBACK
-- =====================================================================