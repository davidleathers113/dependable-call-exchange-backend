-- =====================================================================
-- DNC (DO NOT CALL) INTEGRATION SCHEMA
-- Feature: DNC-001
-- Priority: Critical
-- 
-- This migration creates a comprehensive DNC system with:
-- - DNC Provider management with sync tracking
-- - DNC Entry storage with optimized phone number indexing
-- - DNC Check Result caching with TTL support
-- - Performance optimizations for high-frequency lookups
-- - Compliance audit trails and GDPR support
-- =====================================================================

-- Create DNC schema for namespace isolation
CREATE SCHEMA IF NOT EXISTS dnc;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- =====================================================================
-- SEQUENCES
-- =====================================================================

-- Global sequence for DNC check results
CREATE SEQUENCE IF NOT EXISTS dnc.check_results_sequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MAXVALUE
    NO CYCLE;

COMMENT ON SEQUENCE dnc.check_results_sequence_seq IS 
'Global sequence for DNC check results ordering';

-- =====================================================================
-- DNC PROVIDERS TABLE
-- =====================================================================

CREATE TABLE IF NOT EXISTS dnc.providers (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    provider_type VARCHAR(50) NOT NULL CHECK (provider_type IN ('federal', 'state', 'wireless', 'internal', 'third_party')),
    
    -- Provider configuration
    api_endpoint TEXT,
    api_version VARCHAR(20),
    auth_type VARCHAR(50) CHECK (auth_type IN ('none', 'basic', 'bearer', 'api_key', 'oauth2')),
    auth_config JSONB DEFAULT '{}',
    
    -- Sync configuration
    sync_enabled BOOLEAN NOT NULL DEFAULT true,
    sync_frequency_hours INTEGER NOT NULL DEFAULT 24,
    max_batch_size INTEGER NOT NULL DEFAULT 10000,
    timeout_seconds INTEGER NOT NULL DEFAULT 30,
    retry_attempts INTEGER NOT NULL DEFAULT 3,
    
    -- Provider metadata
    priority INTEGER NOT NULL DEFAULT 100,
    description TEXT,
    contact_info JSONB DEFAULT '{}',
    compliance_certifications TEXT[],
    
    -- Sync tracking
    last_sync_at TIMESTAMPTZ,
    last_sync_status VARCHAR(20) CHECK (last_sync_status IN ('success', 'failure', 'partial', 'in_progress')),
    last_sync_error TEXT,
    last_sync_duration_ms INTEGER,
    last_sync_records_count BIGINT DEFAULT 0,
    next_sync_at TIMESTAMPTZ,
    
    -- Status and lifecycle
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'maintenance', 'deprecated')),
    
    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Data retention and compliance
    retention_days INTEGER NOT NULL DEFAULT 2555, -- 7 years
    gdpr_compliant BOOLEAN NOT NULL DEFAULT true,
    
    -- Version for optimistic locking
    version INTEGER NOT NULL DEFAULT 1
);

-- Table documentation
COMMENT ON TABLE dnc.providers IS 
'DNC providers with sync configuration and status tracking';

COMMENT ON COLUMN dnc.providers.provider_type IS 
'Type of DNC provider: federal (FTC), state, wireless (CTIA), internal, third_party';
COMMENT ON COLUMN dnc.providers.auth_config IS 
'JSON configuration for API authentication (encrypted)';
COMMENT ON COLUMN dnc.providers.priority IS 
'Provider priority for conflict resolution (lower = higher priority)';
COMMENT ON COLUMN dnc.providers.compliance_certifications IS 
'Array of compliance certifications (TCPA, CCPA, etc)';

-- =====================================================================
-- DNC ENTRIES TABLE (PARTITIONED)
-- =====================================================================

CREATE TABLE IF NOT EXISTS dnc.entries (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number VARCHAR(20) NOT NULL, -- E.164 format
    phone_hash VARCHAR(64) NOT NULL,   -- SHA-256 hash for privacy
    
    -- DNC information
    provider_id UUID NOT NULL REFERENCES dnc.providers(id) ON DELETE RESTRICT,
    registration_date DATE NOT NULL,
    expiration_date DATE,
    
    -- Entry metadata
    entry_type VARCHAR(20) NOT NULL DEFAULT 'standard' CHECK (entry_type IN ('standard', 'wireless', 'permanent', 'temporary')),
    area_code VARCHAR(3),
    country_code VARCHAR(5),
    
    -- Compliance and verification
    verified BOOLEAN NOT NULL DEFAULT false,
    verification_date TIMESTAMPTZ,
    verification_method VARCHAR(50),
    
    -- Source tracking
    source_reference VARCHAR(255),
    source_batch_id UUID,
    import_timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Data lineage
    raw_data JSONB DEFAULT '{}',
    processing_flags JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Data retention
    retention_until DATE,
    
    -- Constraints
    CONSTRAINT dnc_entries_phone_provider_unique UNIQUE (phone_number, provider_id),
    CONSTRAINT dnc_entries_hash_unique UNIQUE (phone_hash),
    CONSTRAINT dnc_entries_valid_phone CHECK (phone_number ~ '^\+[1-9]\d{1,14}$'),
    CONSTRAINT dnc_entries_area_code_check CHECK (area_code IS NULL OR area_code ~ '^\d{3}$'),
    CONSTRAINT dnc_entries_country_code_check CHECK (country_code IS NULL OR country_code ~ '^\+\d{1,4}$')
) PARTITION BY RANGE (registration_date);

-- Table documentation
COMMENT ON TABLE dnc.entries IS 
'DNC entries partitioned by registration date for performance and archival';

COMMENT ON COLUMN dnc.entries.phone_hash IS 
'SHA-256 hash of phone number for privacy-preserving queries';
COMMENT ON COLUMN dnc.entries.entry_type IS 
'Type of DNC entry affecting handling and TTL';
COMMENT ON COLUMN dnc.entries.verification_method IS 
'Method used to verify DNC entry (api_lookup, manual_check, etc)';
COMMENT ON COLUMN dnc.entries.raw_data IS 
'Original data from provider for audit and debugging';
COMMENT ON COLUMN dnc.entries.processing_flags IS 
'Flags for processing rules (auto_verified, needs_review, etc)';

-- =====================================================================
-- DNC CHECK RESULTS TABLE (WITH TTL)
-- =====================================================================

CREATE TABLE IF NOT EXISTS dnc.check_results (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sequence_num BIGINT NOT NULL DEFAULT nextval('dnc.check_results_sequence_seq'),
    phone_number VARCHAR(20) NOT NULL, -- E.164 format
    phone_hash VARCHAR(64) NOT NULL,   -- SHA-256 hash
    
    -- Check result
    is_dnc BOOLEAN NOT NULL,
    confidence_score DECIMAL(3,2) NOT NULL DEFAULT 1.00 CHECK (confidence_score >= 0.00 AND confidence_score <= 1.00),
    match_count INTEGER NOT NULL DEFAULT 0,
    
    -- Provider results aggregation
    provider_results JSONB NOT NULL DEFAULT '[]',
    primary_provider_id UUID REFERENCES dnc.providers(id),
    
    -- Check metadata
    check_type VARCHAR(20) NOT NULL DEFAULT 'api' CHECK (check_type IN ('api', 'cache', 'manual', 'batch')),
    check_source VARCHAR(50) NOT NULL,
    check_reason VARCHAR(100),
    
    -- Performance tracking
    check_duration_ms INTEGER,
    cache_hit BOOLEAN NOT NULL DEFAULT false,
    providers_checked INTEGER NOT NULL DEFAULT 0,
    
    -- TTL and caching
    checked_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    ttl_seconds INTEGER NOT NULL,
    
    -- Request context
    request_id VARCHAR(100),
    session_id VARCHAR(100),
    user_id UUID,
    business_id UUID,
    
    -- Compliance context
    legal_basis VARCHAR(50),
    purpose VARCHAR(100) NOT NULL,
    data_classes TEXT[] DEFAULT ARRAY['contact_info'],
    
    -- Audit metadata
    client_ip INET,
    user_agent TEXT,
    environment VARCHAR(50) NOT NULL,
    service_version VARCHAR(50),
    
    -- Constraints
    CONSTRAINT dnc_check_results_sequence_unique UNIQUE (sequence_num),
    CONSTRAINT dnc_check_results_valid_phone CHECK (phone_number ~ '^\+[1-9]\d{1,14}$'),
    CONSTRAINT dnc_check_results_expires_after_check CHECK (expires_at > checked_at),
    CONSTRAINT dnc_check_results_ttl_positive CHECK (ttl_seconds > 0)
);

-- Table documentation
COMMENT ON TABLE dnc.check_results IS 
'Cached DNC check results with TTL for performance optimization';

COMMENT ON COLUMN dnc.check_results.confidence_score IS 
'Confidence in DNC status (0.00-1.00, accounts for conflicts between providers)';
COMMENT ON COLUMN dnc.check_results.provider_results IS 
'Array of individual provider results for audit and debugging';
COMMENT ON COLUMN dnc.check_results.check_source IS 
'Source of the check (api_gateway, batch_processor, admin_portal, etc)';
COMMENT ON COLUMN dnc.check_results.ttl_seconds IS 
'Time-to-live in seconds, copied to expires_at for query optimization';

-- =====================================================================
-- PERFORMANCE INDEXES
-- =====================================================================

-- DNC Providers indexes
CREATE INDEX idx_dnc_providers_type ON dnc.providers (provider_type);
CREATE INDEX idx_dnc_providers_status ON dnc.providers (status) WHERE status = 'active';
CREATE INDEX idx_dnc_providers_sync ON dnc.providers (next_sync_at) WHERE sync_enabled = true;
CREATE INDEX idx_dnc_providers_priority ON dnc.providers (priority, status);
CREATE INDEX idx_dnc_providers_created_at ON dnc.providers (created_at);

-- DNC Entries indexes (will be inherited by partitions)
CREATE INDEX idx_dnc_entries_phone ON dnc.entries (phone_number);
CREATE INDEX idx_dnc_entries_hash ON dnc.entries (phone_hash);
CREATE INDEX idx_dnc_entries_provider ON dnc.entries (provider_id, registration_date);
CREATE INDEX idx_dnc_entries_area_code ON dnc.entries (area_code) WHERE area_code IS NOT NULL;
CREATE INDEX idx_dnc_entries_country_code ON dnc.entries (country_code) WHERE country_code IS NOT NULL;
CREATE INDEX idx_dnc_entries_expiration ON dnc.entries (expiration_date) WHERE expiration_date IS NOT NULL;
CREATE INDEX idx_dnc_entries_import_batch ON dnc.entries (source_batch_id) WHERE source_batch_id IS NOT NULL;
CREATE INDEX idx_dnc_entries_verification ON dnc.entries (verified, verification_date);

-- Composite indexes for common queries
CREATE INDEX idx_dnc_entries_phone_provider_date ON dnc.entries (phone_number, provider_id, registration_date);
CREATE INDEX idx_dnc_entries_provider_verified ON dnc.entries (provider_id, verified, registration_date);

-- DNC Check Results indexes
CREATE INDEX idx_dnc_check_results_phone ON dnc.check_results (phone_number, expires_at DESC);
CREATE INDEX idx_dnc_check_results_hash ON dnc.check_results (phone_hash, expires_at DESC);
CREATE INDEX idx_dnc_check_results_expires ON dnc.check_results (expires_at) WHERE expires_at > CURRENT_TIMESTAMP;
CREATE INDEX idx_dnc_check_results_checked_at ON dnc.check_results (checked_at DESC);
CREATE INDEX idx_dnc_check_results_request ON dnc.check_results (request_id) WHERE request_id IS NOT NULL;
CREATE INDEX idx_dnc_check_results_session ON dnc.check_results (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_dnc_check_results_business ON dnc.check_results (business_id, checked_at DESC) WHERE business_id IS NOT NULL;

-- GIN indexes for JSONB columns
CREATE INDEX idx_dnc_providers_auth_config ON dnc.providers USING GIN (auth_config);
CREATE INDEX idx_dnc_entries_raw_data ON dnc.entries USING GIN (raw_data);
CREATE INDEX idx_dnc_entries_processing_flags ON dnc.entries USING GIN (processing_flags);
CREATE INDEX idx_dnc_check_results_provider_results ON dnc.check_results USING GIN (provider_results);
CREATE INDEX idx_dnc_check_results_data_classes ON dnc.check_results USING GIN (data_classes);

-- =====================================================================
-- PARTITION MANAGEMENT FUNCTIONS
-- =====================================================================

-- Function to create yearly partitions for DNC entries
CREATE OR REPLACE FUNCTION dnc.create_yearly_partition(
    year INTEGER
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := DATE(year || '-01-01');
    end_date := DATE((year + 1) || '-01-01');
    partition_name := 'entries_' || year;
    
    -- Check if partition already exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_tables 
        WHERE schemaname = 'dnc' 
        AND tablename = partition_name
    ) THEN
        -- Create partition
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS dnc.%I PARTITION OF dnc.entries
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        
        RAISE NOTICE 'Created DNC entries partition % for year %', partition_name, year;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically create partitions
CREATE OR REPLACE FUNCTION dnc.auto_create_partitions()
RETURNS VOID AS $$
DECLARE
    current_year INTEGER;
    target_year INTEGER;
BEGIN
    current_year := EXTRACT(YEAR FROM CURRENT_DATE);
    
    -- Create partitions for previous year, current year, and next 2 years
    FOR target_year IN (current_year - 1)..(current_year + 2) LOOP
        PERFORM dnc.create_yearly_partition(target_year);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- TTL CLEANUP FUNCTIONS
-- =====================================================================

-- Function to clean up expired check results
CREATE OR REPLACE FUNCTION dnc.cleanup_expired_check_results()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM dnc.check_results 
    WHERE expires_at < CURRENT_TIMESTAMP;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    RAISE NOTICE 'Cleaned up % expired DNC check results', deleted_count;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up old DNC entries based on retention policy
CREATE OR REPLACE FUNCTION dnc.cleanup_old_entries()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM dnc.entries 
    WHERE retention_until IS NOT NULL 
    AND retention_until < CURRENT_DATE;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    RAISE NOTICE 'Cleaned up % old DNC entries past retention period', deleted_count;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- PHONE NUMBER UTILITIES
-- =====================================================================

-- Function to generate phone number hash
CREATE OR REPLACE FUNCTION dnc.hash_phone_number(phone_number TEXT)
RETURNS VARCHAR(64) AS $$
BEGIN
    RETURN encode(digest(phone_number, 'sha256'), 'hex');
END;
$$ LANGUAGE plpgsql;

-- Function to normalize phone number to E.164
CREATE OR REPLACE FUNCTION dnc.normalize_phone_number(phone_number TEXT)
RETURNS TEXT AS $$
BEGIN
    -- Basic E.164 normalization (extend as needed)
    phone_number := regexp_replace(phone_number, '[^0-9+]', '', 'g');
    
    -- Add + if missing for US numbers
    IF phone_number ~ '^1[0-9]{10}$' THEN
        phone_number := '+' || phone_number;
    ELSIF phone_number ~ '^[0-9]{10}$' THEN
        phone_number := '+1' || phone_number;
    ELSIF NOT phone_number ~ '^\+' THEN
        phone_number := '+' || phone_number;
    END IF;
    
    -- Validate E.164 format
    IF NOT phone_number ~ '^\+[1-9]\d{1,14}$' THEN
        RAISE EXCEPTION 'Invalid phone number format: %', phone_number;
    END IF;
    
    RETURN phone_number;
END;
$$ LANGUAGE plpgsql;

-- Function to extract area code from US phone numbers
CREATE OR REPLACE FUNCTION dnc.extract_area_code(phone_number TEXT)
RETURNS VARCHAR(3) AS $$
BEGIN
    -- Extract area code from +1AAANNNNNNN format
    IF phone_number ~ '^\+1[0-9]{10}$' THEN
        RETURN substring(phone_number from 3 for 3);
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Function to extract country code
CREATE OR REPLACE FUNCTION dnc.extract_country_code(phone_number TEXT)
RETURNS VARCHAR(5) AS $$
BEGIN
    -- Extract country code (simplified logic)
    IF phone_number ~ '^\+1[0-9]{10}$' THEN
        RETURN '+1';
    ELSIF phone_number ~ '^\+44[0-9]+$' THEN
        RETURN '+44';
    ELSIF phone_number ~ '^\+33[0-9]+$' THEN
        RETURN '+33';
    ELSIF phone_number ~ '^\+49[0-9]+$' THEN
        RETURN '+49';
    ELSIF phone_number ~ '^\+[1-9]\d{1,3}' THEN
        -- Extract up to 4 digits after +
        RETURN substring(phone_number from '^\+[1-9]\d{0,3}');
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- TRIGGERS AND AUTOMATION
-- =====================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION dnc.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to auto-populate derived fields
CREATE OR REPLACE FUNCTION dnc.populate_derived_fields()
RETURNS TRIGGER AS $$
BEGIN
    -- Normalize phone number
    NEW.phone_number := dnc.normalize_phone_number(NEW.phone_number);
    
    -- Generate phone hash
    NEW.phone_hash := dnc.hash_phone_number(NEW.phone_number);
    
    -- Extract area code for US numbers
    NEW.area_code := dnc.extract_area_code(NEW.phone_number);
    
    -- Extract country code
    NEW.country_code := dnc.extract_country_code(NEW.phone_number);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to auto-populate check result expiration
CREATE OR REPLACE FUNCTION dnc.populate_check_result_expiration()
RETURNS TRIGGER AS $$
BEGIN
    -- Normalize phone number
    NEW.phone_number := dnc.normalize_phone_number(NEW.phone_number);
    
    -- Generate phone hash
    NEW.phone_hash := dnc.hash_phone_number(NEW.phone_number);
    
    -- Set expires_at based on TTL
    IF NEW.expires_at IS NULL THEN
        NEW.expires_at := NEW.checked_at + (NEW.ttl_seconds || ' seconds')::INTERVAL;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers
CREATE TRIGGER update_dnc_providers_updated_at 
    BEFORE UPDATE ON dnc.providers
    FOR EACH ROW EXECUTE FUNCTION dnc.update_updated_at_column();

CREATE TRIGGER update_dnc_entries_updated_at 
    BEFORE UPDATE ON dnc.entries
    FOR EACH ROW EXECUTE FUNCTION dnc.update_updated_at_column();

CREATE TRIGGER populate_dnc_entries_derived_fields 
    BEFORE INSERT OR UPDATE ON dnc.entries
    FOR EACH ROW EXECUTE FUNCTION dnc.populate_derived_fields();

CREATE TRIGGER populate_dnc_check_results_expiration 
    BEFORE INSERT OR UPDATE ON dnc.check_results
    FOR EACH ROW EXECUTE FUNCTION dnc.populate_check_result_expiration();

-- =====================================================================
-- MONITORING VIEWS
-- =====================================================================

-- Health check view
CREATE OR REPLACE VIEW dnc.health_check AS
SELECT 
    'providers_active' as metric,
    COUNT(*)::TEXT as value
FROM dnc.providers
WHERE status = 'active'
UNION ALL
SELECT 
    'entries_total' as metric,
    COUNT(*)::TEXT as value
FROM dnc.entries
UNION ALL
SELECT 
    'check_results_cached' as metric,
    COUNT(*)::TEXT as value
FROM dnc.check_results
WHERE expires_at > CURRENT_TIMESTAMP
UNION ALL
SELECT 
    'cache_hit_rate_24h' as metric,
    ROUND(
        (COUNT(*) FILTER (WHERE cache_hit = true)::DECIMAL / NULLIF(COUNT(*), 0)) * 100, 2
    )::TEXT || '%' as value
FROM dnc.check_results
WHERE checked_at > CURRENT_TIMESTAMP - INTERVAL '24 hours';

-- Statistics view
CREATE OR REPLACE VIEW dnc.stats AS
WITH provider_stats AS (
    SELECT 
        p.name,
        p.provider_type,
        p.status,
        p.last_sync_at,
        p.last_sync_status,
        COUNT(e.id) as entry_count,
        MIN(e.registration_date) as oldest_entry,
        MAX(e.registration_date) as newest_entry
    FROM dnc.providers p
    LEFT JOIN dnc.entries e ON e.provider_id = p.id
    GROUP BY p.id, p.name, p.provider_type, p.status, p.last_sync_at, p.last_sync_status
)
SELECT 
    name,
    provider_type,
    status,
    entry_count,
    oldest_entry,
    newest_entry,
    last_sync_at,
    last_sync_status,
    CASE 
        WHEN last_sync_at IS NULL THEN 'never'
        WHEN last_sync_at < CURRENT_TIMESTAMP - INTERVAL '48 hours' THEN 'stale'
        ELSE 'current'
    END as sync_health
FROM provider_stats
ORDER BY entry_count DESC;

-- Cache performance view
CREATE OR REPLACE VIEW dnc.cache_performance AS
SELECT 
    DATE_TRUNC('hour', checked_at) as hour,
    COUNT(*) as total_checks,
    COUNT(*) FILTER (WHERE cache_hit = true) as cache_hits,
    COUNT(*) FILTER (WHERE cache_hit = false) as cache_misses,
    ROUND(
        (COUNT(*) FILTER (WHERE cache_hit = true)::DECIMAL / COUNT(*)) * 100, 2
    ) as hit_rate_percent,
    AVG(check_duration_ms) as avg_duration_ms,
    MAX(check_duration_ms) as max_duration_ms
FROM dnc.check_results
WHERE checked_at > CURRENT_TIMESTAMP - INTERVAL '7 days'
GROUP BY DATE_TRUNC('hour', checked_at)
ORDER BY hour DESC;

-- =====================================================================
-- SECURITY ROLES
-- =====================================================================

-- Create roles if they don't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dnc_reader') THEN
        CREATE ROLE dnc_reader;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dnc_writer') THEN
        CREATE ROLE dnc_writer;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dnc_admin') THEN
        CREATE ROLE dnc_admin;
    END IF;
END
$$;

-- Grant permissions
GRANT USAGE ON SCHEMA dnc TO dnc_reader, dnc_writer, dnc_admin;

-- Reader permissions (read-only)
GRANT SELECT ON ALL TABLES IN SCHEMA dnc TO dnc_reader;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA dnc TO dnc_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA dnc GRANT SELECT ON TABLES TO dnc_reader;

-- Writer permissions (can insert/update check results and entries)
GRANT SELECT, INSERT, UPDATE ON dnc.entries, dnc.check_results TO dnc_writer;
GRANT SELECT ON dnc.providers TO dnc_writer;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA dnc TO dnc_writer;
ALTER DEFAULT PRIVILEGES IN SCHEMA dnc GRANT INSERT, UPDATE ON TABLES TO dnc_writer;

-- Admin permissions (full access)
GRANT ALL ON ALL TABLES IN SCHEMA dnc TO dnc_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA dnc TO dnc_admin;
GRANT ALL ON ALL FUNCTIONS IN SCHEMA dnc TO dnc_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA dnc GRANT ALL ON TABLES TO dnc_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA dnc GRANT ALL ON SEQUENCES TO dnc_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA dnc GRANT ALL ON FUNCTIONS TO dnc_admin;

-- =====================================================================
-- INITIAL PARTITION CREATION
-- =====================================================================

-- Create partitions for current and adjacent years
DO $$
DECLARE
    current_year INTEGER;
BEGIN
    current_year := EXTRACT(YEAR FROM CURRENT_DATE);
    
    -- Create partitions for 2 years back, current year, and 2 years forward
    FOR target_year IN (current_year - 2)..(current_year + 2) LOOP
        PERFORM dnc.create_yearly_partition(target_year);
    END LOOP;
END
$$;

-- =====================================================================
-- INITIAL DATA SEEDING
-- =====================================================================

-- Insert default Federal Trade Commission provider
INSERT INTO dnc.providers (
    id,
    name,
    provider_type,
    api_endpoint,
    description,
    priority,
    sync_frequency_hours,
    compliance_certifications,
    status
) VALUES (
    uuid_generate_v4(),
    'Federal Trade Commission (FTC)',
    'federal',
    'https://www.donotcall.gov/api/v1',
    'Official Federal Do Not Call Registry managed by the FTC',
    1,
    24,
    ARRAY['TCPA', 'TSR'],
    'active'
) ON CONFLICT (name) DO NOTHING;

-- Insert default Wireless provider (CTIA)
INSERT INTO dnc.providers (
    id,
    name,
    provider_type,
    api_endpoint,
    description,
    priority,
    sync_frequency_hours,
    compliance_certifications,
    status
) VALUES (
    uuid_generate_v4(),
    'Cellular Telecommunications Industry Association (CTIA)',
    'wireless',
    'https://www.ctia.org/api/v1/dnc',
    'Wireless Do Not Call Registry managed by CTIA',
    2,
    24,
    ARRAY['TCPA', 'CTIA_COMPLIANCE'],
    'active'
) ON CONFLICT (name) DO NOTHING;

-- =====================================================================
-- SCHEDULED MAINTENANCE (requires pg_cron extension)
-- =====================================================================

-- Note: Uncomment these if pg_cron is installed
-- SELECT cron.schedule('dnc-create-partitions', '0 0 1 1 *', 'SELECT dnc.auto_create_partitions()');
-- SELECT cron.schedule('dnc-cleanup-expired-results', '0 */6 * * *', 'SELECT dnc.cleanup_expired_check_results()');
-- SELECT cron.schedule('dnc-cleanup-old-entries', '0 1 * * 0', 'SELECT dnc.cleanup_old_entries()');

-- =====================================================================
-- MIGRATION METADATA
-- =====================================================================

COMMENT ON SCHEMA dnc IS 
'DNC (Do Not Call) integration system for TCPA compliance and call filtering';

-- Log the migration completion in audit system
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
    'create_dnc_schema',
    'success',
    current_setting('application_name', true),
    'database_migration',
    '20250614',
    '',
    encode(digest('dnc_schema_creation_' || CURRENT_TIMESTAMP::TEXT, 'sha256'), 'hex'),
    jsonb_build_object(
        'migration_file', '20250614_191246_create_dnc_schema.up.sql',
        'created_at', CURRENT_TIMESTAMP,
        'description', 'Created comprehensive DNC schema with providers, entries, and check results',
        'features', ARRAY['partitioning', 'ttl_cleanup', 'phone_normalization', 'compliance_audit']
    )
);

-- =====================================================================
-- END OF MIGRATION
-- =====================================================================