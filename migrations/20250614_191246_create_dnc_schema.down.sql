-- =====================================================================
-- DNC (DO NOT CALL) INTEGRATION SCHEMA ROLLBACK
-- Feature: DNC-001
-- 
-- This migration removes the DNC system safely:
-- - Drops all DNC tables and dependencies
-- - Removes functions and triggers
-- - Cleans up roles and permissions
-- - Archives data if needed
-- =====================================================================

-- Log the rollback start
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
    'dnc_schema',
    'database_schema',
    'rollback_dnc_schema',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250614',
    '',
    encode(digest('dnc_schema_rollback_' || CURRENT_TIMESTAMP::TEXT, 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250614_191246_create_dnc_schema.down.sql',
        'rollback_at', CURRENT_TIMESTAMP,
        'description', 'Rolling back DNC schema and all related objects',
        'warning', 'This will permanently delete all DNC data'
    )
);

-- =====================================================================
-- DATA ARCHIVAL (OPTIONAL - UNCOMMENT IF NEEDED)
-- =====================================================================

-- Uncomment these sections if you need to archive data before deletion

-- -- Create temporary archive table for check results
-- CREATE TABLE IF NOT EXISTS public.dnc_check_results_archive AS 
-- SELECT *, CURRENT_TIMESTAMP as archived_at
-- FROM dnc.check_results;

-- -- Create temporary archive table for entries
-- CREATE TABLE IF NOT EXISTS public.dnc_entries_archive AS 
-- SELECT *, CURRENT_TIMESTAMP as archived_at
-- FROM dnc.entries;

-- -- Create temporary archive table for providers
-- CREATE TABLE IF NOT EXISTS public.dnc_providers_archive AS 
-- SELECT *, CURRENT_TIMESTAMP as archived_at
-- FROM dnc.providers;

-- COMMENT ON TABLE public.dnc_check_results_archive IS 
-- 'Archived DNC check results from migration rollback';
-- COMMENT ON TABLE public.dnc_entries_archive IS 
-- 'Archived DNC entries from migration rollback';
-- COMMENT ON TABLE public.dnc_providers_archive IS 
-- 'Archived DNC providers from migration rollback';

-- =====================================================================
-- REMOVE SCHEDULED JOBS (if pg_cron is enabled)
-- =====================================================================

-- Note: Uncomment these if pg_cron was used
-- SELECT cron.unschedule('dnc-create-partitions');
-- SELECT cron.unschedule('dnc-cleanup-expired-results');
-- SELECT cron.unschedule('dnc-cleanup-old-entries');

-- =====================================================================
-- DROP VIEWS
-- =====================================================================

DROP VIEW IF EXISTS dnc.cache_performance;
DROP VIEW IF EXISTS dnc.stats;
DROP VIEW IF EXISTS dnc.health_check;

-- =====================================================================
-- DROP TRIGGERS
-- =====================================================================

DROP TRIGGER IF EXISTS populate_dnc_check_results_expiration ON dnc.check_results;
DROP TRIGGER IF EXISTS populate_dnc_entries_derived_fields ON dnc.entries;
DROP TRIGGER IF EXISTS update_dnc_entries_updated_at ON dnc.entries;
DROP TRIGGER IF EXISTS update_dnc_providers_updated_at ON dnc.providers;

-- =====================================================================
-- DROP FUNCTIONS
-- =====================================================================

DROP FUNCTION IF EXISTS dnc.populate_check_result_expiration();
DROP FUNCTION IF EXISTS dnc.populate_derived_fields();
DROP FUNCTION IF EXISTS dnc.update_updated_at_column();
DROP FUNCTION IF EXISTS dnc.extract_country_code(TEXT);
DROP FUNCTION IF EXISTS dnc.extract_area_code(TEXT);
DROP FUNCTION IF EXISTS dnc.normalize_phone_number(TEXT);
DROP FUNCTION IF EXISTS dnc.hash_phone_number(TEXT);
DROP FUNCTION IF EXISTS dnc.cleanup_old_entries();
DROP FUNCTION IF EXISTS dnc.cleanup_expired_check_results();
DROP FUNCTION IF EXISTS dnc.auto_create_partitions();
DROP FUNCTION IF EXISTS dnc.create_yearly_partition(INTEGER);

-- =====================================================================
-- DROP INDEXES (will be dropped with tables, but explicit for clarity)
-- =====================================================================

-- Note: Indexes will be automatically dropped with their tables
-- This section is for documentation purposes

-- =====================================================================
-- DROP TABLES (in dependency order)
-- =====================================================================

-- Drop check results table first (no foreign key dependencies)
DROP TABLE IF EXISTS dnc.check_results CASCADE;

-- Drop partitioned entries table and all partitions
DROP TABLE IF EXISTS dnc.entries CASCADE;

-- Drop providers table last
DROP TABLE IF EXISTS dnc.providers CASCADE;

-- =====================================================================
-- DROP SEQUENCES
-- =====================================================================

DROP SEQUENCE IF EXISTS dnc.check_results_sequence_seq;

-- =====================================================================
-- REVOKE PERMISSIONS AND DROP ROLES
-- =====================================================================

-- Revoke all permissions from DNC roles
REVOKE ALL ON SCHEMA dnc FROM dnc_reader, dnc_writer, dnc_admin;

-- Note: We don't drop the roles themselves as they might be used elsewhere
-- Uncomment these lines if you want to completely remove the roles:
-- DROP ROLE IF EXISTS dnc_reader;
-- DROP ROLE IF EXISTS dnc_writer;
-- DROP ROLE IF EXISTS dnc_admin;

-- =====================================================================
-- DROP SCHEMA
-- =====================================================================

-- Drop the DNC schema (CASCADE will remove any remaining objects)
DROP SCHEMA IF EXISTS dnc CASCADE;

-- =====================================================================
-- ROLLBACK CONFIRMATION
-- =====================================================================

-- Log successful rollback
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
    'dnc_schema',
    'database_schema',
    'rollback_dnc_schema_complete',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250614',
    '',
    encode(digest('dnc_schema_rollback_complete_' || CURRENT_TIMESTAMP::TEXT, 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250614_191246_create_dnc_schema.down.sql',
        'completed_at', CURRENT_TIMESTAMP,
        'description', 'Successfully rolled back DNC schema',
        'objects_removed', ARRAY['dnc.providers', 'dnc.entries', 'dnc.check_results', 'dnc schema', 'functions', 'triggers', 'views']
    )
);

-- =====================================================================
-- CLEANUP EXTENSIONS (OPTIONAL)
-- =====================================================================

-- Note: These extensions might be used by other parts of the system
-- Only uncomment if you're sure they're not needed elsewhere:

-- DROP EXTENSION IF EXISTS btree_gin;
-- DROP EXTENSION IF EXISTS pgcrypto;
-- DROP EXTENSION IF EXISTS "uuid-ossp";

-- =====================================================================
-- END OF ROLLBACK MIGRATION
-- =====================================================================

-- Provide confirmation message
DO $$
BEGIN
    RAISE NOTICE 'DNC schema rollback completed successfully';
    RAISE NOTICE 'All DNC tables, functions, triggers, and views have been removed';
    RAISE NOTICE 'If data archival was needed, check public.dnc_*_archive tables';
    RAISE NOTICE 'DNC roles (dnc_reader, dnc_writer, dnc_admin) were not dropped - remove manually if needed';
END
$$;