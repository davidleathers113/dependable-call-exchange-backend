-- =====================================================================
-- ROLLBACK: IMMUTABLE AUDIT LOGGING SCHEMA
-- 
-- WARNING: This will remove ALL audit logging infrastructure
-- Ensure audit data has been exported/archived before proceeding
-- =====================================================================

-- Drop scheduled jobs if pg_cron is enabled
-- SELECT cron.unschedule('audit-create-partitions');
-- SELECT cron.unschedule('audit-create-checkpoint');
-- SELECT cron.unschedule('audit-verify-integrity');

-- Drop views
DROP VIEW IF EXISTS audit.partition_status CASCADE;
DROP VIEW IF EXISTS audit.stats CASCADE;
DROP VIEW IF EXISTS audit.health_check CASCADE;

-- Drop all partition tables
DO $$
DECLARE
    partition_record RECORD;
BEGIN
    FOR partition_record IN 
        SELECT tablename 
        FROM pg_tables 
        WHERE schemaname = 'audit' 
        AND tablename LIKE 'events_%'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS audit.%I CASCADE', partition_record.tablename);
        RAISE NOTICE 'Dropped partition table audit.%', partition_record.tablename;
    END LOOP;
END
$$;

-- Drop main tables
DROP TABLE IF EXISTS audit.events CASCADE;
DROP TABLE IF EXISTS audit.access_log CASCADE;
DROP TABLE IF EXISTS audit.archives CASCADE;
DROP TABLE IF EXISTS audit.hash_chain CASCADE;

-- Drop functions
DROP FUNCTION IF EXISTS audit.create_hash_checkpoint() CASCADE;
DROP FUNCTION IF EXISTS audit.verify_hash_chain(BIGINT, BIGINT) CASCADE;
DROP FUNCTION IF EXISTS audit.drop_old_partition(INTEGER, INTEGER) CASCADE;
DROP FUNCTION IF EXISTS audit.auto_create_partitions() CASCADE;
DROP FUNCTION IF EXISTS audit.create_monthly_partition(INTEGER, INTEGER) CASCADE;
DROP FUNCTION IF EXISTS audit.prevent_audit_modification() CASCADE;

-- Drop sequences
DROP SEQUENCE IF EXISTS audit.events_sequence_seq CASCADE;

-- Revoke permissions
REVOKE ALL ON SCHEMA audit FROM audit_reader, audit_writer, audit_admin CASCADE;

-- Drop roles if they exist and have no dependencies
DO $$
BEGIN
    -- Check if roles have any dependencies before dropping
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles r
        JOIN pg_auth_members m ON r.oid = m.roleid
        WHERE r.rolname = 'audit_reader'
    ) THEN
        DROP ROLE IF EXISTS audit_reader;
    ELSE
        RAISE NOTICE 'Role audit_reader has dependencies and was not dropped';
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles r
        JOIN pg_auth_members m ON r.oid = m.roleid
        WHERE r.rolname = 'audit_writer'
    ) THEN
        DROP ROLE IF EXISTS audit_writer;
    ELSE
        RAISE NOTICE 'Role audit_writer has dependencies and was not dropped';
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM pg_roles r
        JOIN pg_auth_members m ON r.oid = m.roleid
        WHERE r.rolname = 'audit_admin'
    ) THEN
        DROP ROLE IF EXISTS audit_admin;
    ELSE
        RAISE NOTICE 'Role audit_admin has dependencies and was not dropped';
    END IF;
END
$$;

-- Drop schema
DROP SCHEMA IF EXISTS audit CASCADE;

-- Log the rollback
DO $$
BEGIN
    RAISE NOTICE 'Audit schema rollback completed at %', CURRENT_TIMESTAMP;
    RAISE NOTICE 'IMPORTANT: Ensure all audit data has been archived before this rollback';
END
$$;

-- =====================================================================
-- END OF ROLLBACK
-- =====================================================================