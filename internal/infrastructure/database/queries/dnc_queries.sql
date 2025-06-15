-- =====================================================================
-- DNC (DO NOT CALL) HIGH-PERFORMANCE QUERY LIBRARY
-- =====================================================================
-- Feature: DNC Integration
-- Performance Targets: < 5ms lookups, > 10K/sec throughput
-- Created: 2025-01-15
-- Author: Query Optimizer Agent
-- =====================================================================

-- =====================================================================
-- SCHEMA DEFINITIONS
-- =====================================================================

-- Main DNC entries table with performance optimizations
CREATE TABLE IF NOT EXISTS dnc_entries (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20) NOT NULL,  -- E.164 format (+1234567890)
    phone_number_hash VARCHAR(64) NOT NULL,  -- SHA-256 hash for privacy
    list_source VARCHAR(50) NOT NULL,
    suppress_reason VARCHAR(50) NOT NULL,
    
    -- Temporal data
    added_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Reference data
    source_reference VARCHAR(255),  -- External provider ID
    provider_id UUID,  -- FK to dnc_providers
    
    -- Metadata
    notes TEXT,
    metadata JSONB DEFAULT '{}',
    
    -- Audit fields
    added_by UUID NOT NULL,
    updated_by UUID,
    
    -- Computed fields for performance
    is_active BOOLEAN GENERATED ALWAYS AS (
        expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP
    ) STORED,
    
    -- Priority for conflict resolution (higher = more authoritative)
    authority_level INTEGER NOT NULL DEFAULT 1,
    
    -- Compliance tracking
    compliance_flags JSONB DEFAULT '{}',
    
    -- Constraints
    CONSTRAINT dnc_entries_phone_number_check CHECK (
        phone_number ~ '^\+[1-9]\d{1,14}$'  -- E.164 format validation
    ),
    CONSTRAINT dnc_entries_list_source_check CHECK (
        list_source IN ('federal', 'state', 'internal', 'custom')
    ),
    CONSTRAINT dnc_entries_suppress_reason_check CHECK (
        suppress_reason IN (
            'federal_dnc', 'state_dnc', 'company_policy', 'user_request',
            'litigation', 'fraud', 'invalid_number', 'business_hours',
            'excessive_calls', 'opt_out'
        )
    ),
    CONSTRAINT dnc_entries_unique_active UNIQUE (phone_number, list_source) 
        WHERE is_active = true
);

-- DNC providers table
CREATE TABLE IF NOT EXISTS dnc_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,
    base_url VARCHAR(500),
    status VARCHAR(50) NOT NULL DEFAULT 'inactive',
    enabled BOOLEAN NOT NULL DEFAULT false,
    priority INTEGER NOT NULL DEFAULT 100,
    auth_type VARCHAR(50) NOT NULL DEFAULT 'none',
    update_frequency INTERVAL NOT NULL DEFAULT '24 hours',
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    
    -- Performance metrics
    last_sync_duration INTERVAL,
    last_sync_records INTEGER,
    error_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    
    -- Configuration
    timeout_seconds INTEGER NOT NULL DEFAULT 30,
    rate_limit_per_min INTEGER NOT NULL DEFAULT 60,
    retry_attempts INTEGER NOT NULL DEFAULT 3,
    config JSONB DEFAULT '{}',
    
    -- Error tracking
    last_error TEXT,
    
    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL,
    updated_by UUID,
    
    -- Constraints
    CONSTRAINT dnc_providers_type_check CHECK (
        type IN ('federal', 'state', 'internal', 'custom')
    ),
    CONSTRAINT dnc_providers_status_check CHECK (
        status IN ('active', 'inactive', 'error', 'syncing')
    ),
    CONSTRAINT dnc_providers_auth_type_check CHECK (
        auth_type IN ('none', 'api_key', 'oauth', 'basic')
    ),
    CONSTRAINT dnc_providers_priority_check CHECK (priority > 0)
);

-- Sync history for auditing and troubleshooting
CREATE TABLE IF NOT EXISTS dnc_sync_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES dnc_providers(id),
    sync_started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sync_completed_at TIMESTAMPTZ,
    status VARCHAR(50) NOT NULL,
    records_processed INTEGER DEFAULT 0,
    records_added INTEGER DEFAULT 0,
    records_updated INTEGER DEFAULT 0,
    records_deleted INTEGER DEFAULT 0,
    error_message TEXT,
    sync_metadata JSONB DEFAULT '{}',
    
    CONSTRAINT dnc_sync_history_status_check CHECK (
        status IN ('running', 'completed', 'failed', 'cancelled')
    )
);

-- Query performance cache table
CREATE TABLE IF NOT EXISTS dnc_query_cache (
    phone_number_hash VARCHAR(64) PRIMARY KEY,
    can_call BOOLEAN NOT NULL,
    blocking_entries JSONB NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    
    -- Cache hit tracking
    hit_count INTEGER NOT NULL DEFAULT 0,
    last_hit_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT dnc_query_cache_expires_check CHECK (
        expires_at > computed_at
    )
);

-- =====================================================================
-- PERFORMANCE INDEXES
-- =====================================================================

-- Primary lookup indexes (< 5ms target)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_phone_lookup 
    ON dnc_entries (phone_number) 
    WHERE is_active = true;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_phone_hash_lookup 
    ON dnc_entries (phone_number_hash) 
    WHERE is_active = true;

-- Composite index for complex lookups
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_phone_source_active 
    ON dnc_entries (phone_number, list_source, is_active);

-- Authority-based lookup for conflict resolution
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_phone_authority 
    ON dnc_entries (phone_number, authority_level DESC, added_at DESC) 
    WHERE is_active = true;

-- Provider relationship index
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_provider 
    ON dnc_entries (provider_id, added_at DESC);

-- Expiration cleanup index
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_expiration 
    ON dnc_entries (expires_at) 
    WHERE expires_at IS NOT NULL AND is_active = true;

-- Metadata search indexes (GIN for JSONB performance)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_metadata 
    ON dnc_entries USING GIN (metadata);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_compliance 
    ON dnc_entries USING GIN (compliance_flags);

-- Provider management indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_providers_sync_schedule 
    ON dnc_providers (next_sync_at) 
    WHERE enabled = true;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_providers_type_priority 
    ON dnc_providers (type, priority) 
    WHERE enabled = true;

-- Sync history indexes for monitoring
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_sync_history_provider_time 
    ON dnc_sync_history (provider_id, sync_started_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_sync_history_status_time 
    ON dnc_sync_history (status, sync_started_at DESC);

-- Cache performance indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_query_cache_expires 
    ON dnc_query_cache (expires_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_query_cache_hit_stats 
    ON dnc_query_cache (hit_count DESC, last_hit_at DESC);

-- Partial indexes for active entries only (space and performance optimization)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_active_only_phone 
    ON dnc_entries (phone_number) 
    WHERE is_active = true AND expires_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_entries_temporary_expiring 
    ON dnc_entries (expires_at, phone_number) 
    WHERE is_active = true AND expires_at IS NOT NULL;

-- =====================================================================
-- CORE QUERY FUNCTIONS
-- =====================================================================

-- High-performance phone number lookup with caching
-- Target: < 2ms response time
CREATE OR REPLACE FUNCTION dnc_check_phone_number(
    p_phone_number VARCHAR(20),
    p_use_cache BOOLEAN DEFAULT true
) RETURNS TABLE (
    can_call BOOLEAN,
    blocking_entries JSONB,
    cache_hit BOOLEAN,
    response_time_ms NUMERIC
) AS $$
DECLARE
    v_start_time TIMESTAMPTZ;
    v_phone_hash VARCHAR(64);
    v_cache_result RECORD;
    v_can_call BOOLEAN;
    v_blocking JSONB;
    v_cache_hit BOOLEAN := false;
BEGIN
    v_start_time := clock_timestamp();
    
    -- Generate phone number hash for privacy
    v_phone_hash := encode(digest(p_phone_number, 'sha256'), 'hex');
    
    -- Check cache first if enabled
    IF p_use_cache THEN
        SELECT qc.can_call, qc.blocking_entries 
        INTO v_cache_result
        FROM dnc_query_cache qc
        WHERE qc.phone_number_hash = v_phone_hash
        AND qc.expires_at > CURRENT_TIMESTAMP;
        
        IF FOUND THEN
            v_cache_hit := true;
            v_can_call := v_cache_result.can_call;
            v_blocking := v_cache_result.blocking_entries;
            
            -- Update cache hit statistics
            UPDATE dnc_query_cache 
            SET hit_count = hit_count + 1,
                last_hit_at = CURRENT_TIMESTAMP
            WHERE phone_number_hash = v_phone_hash;
        END IF;
    END IF;
    
    -- If not cached, perform actual lookup
    IF NOT v_cache_hit THEN
        -- Get all active blocking entries with authority ordering
        WITH blocking_entries AS (
            SELECT 
                e.id,
                e.phone_number,
                e.list_source,
                e.suppress_reason,
                e.authority_level,
                e.added_at,
                e.expires_at,
                e.source_reference,
                e.notes,
                p.name as provider_name,
                p.type as provider_type
            FROM dnc_entries e
            LEFT JOIN dnc_providers p ON e.provider_id = p.id
            WHERE e.phone_number = p_phone_number
            AND e.is_active = true
            ORDER BY e.authority_level DESC, e.added_at DESC
        )
        SELECT 
            CASE WHEN COUNT(*) = 0 THEN true ELSE false END,
            COALESCE(
                jsonb_agg(
                    jsonb_build_object(
                        'id', be.id,
                        'list_source', be.list_source,
                        'suppress_reason', be.suppress_reason,
                        'authority_level', be.authority_level,
                        'added_at', be.added_at,
                        'expires_at', be.expires_at,
                        'provider_name', be.provider_name,
                        'provider_type', be.provider_type,
                        'source_reference', be.source_reference,
                        'notes', be.notes
                    )
                ),
                '[]'::jsonb
            )
        INTO v_can_call, v_blocking
        FROM blocking_entries be;
        
        -- Cache the result if caching is enabled
        IF p_use_cache THEN
            INSERT INTO dnc_query_cache (
                phone_number_hash,
                can_call,
                blocking_entries,
                expires_at
            ) VALUES (
                v_phone_hash,
                v_can_call,
                v_blocking,
                CURRENT_TIMESTAMP + INTERVAL '1 hour'  -- Cache for 1 hour
            )
            ON CONFLICT (phone_number_hash) DO UPDATE SET
                can_call = EXCLUDED.can_call,
                blocking_entries = EXCLUDED.blocking_entries,
                computed_at = CURRENT_TIMESTAMP,
                expires_at = EXCLUDED.expires_at,
                hit_count = 0;
        END IF;
    END IF;
    
    RETURN QUERY SELECT 
        v_can_call,
        v_blocking,
        v_cache_hit,
        EXTRACT(EPOCH FROM (clock_timestamp() - v_start_time)) * 1000;
END;
$$ LANGUAGE plpgsql;

-- Bulk phone number lookup for high-throughput scenarios
-- Target: > 10K lookups/sec
CREATE OR REPLACE FUNCTION dnc_check_phone_numbers_bulk(
    p_phone_numbers VARCHAR(20)[],
    p_use_cache BOOLEAN DEFAULT true
) RETURNS TABLE (
    phone_number VARCHAR(20),
    can_call BOOLEAN,
    blocking_count INTEGER,
    highest_authority INTEGER,
    cache_hit BOOLEAN
) AS $$
DECLARE
    v_phone TEXT;
    v_phone_hashes TEXT[];
    v_uncached_phones VARCHAR(20)[];
BEGIN
    -- Generate hashes for all numbers
    SELECT array_agg(encode(digest(phone, 'sha256'), 'hex'))
    INTO v_phone_hashes
    FROM unnest(p_phone_numbers) AS phone;
    
    -- Create temp table for efficient joining
    CREATE TEMP TABLE temp_phone_lookup (
        phone_number VARCHAR(20),
        phone_hash VARCHAR(64)
    ) ON COMMIT DROP;
    
    INSERT INTO temp_phone_lookup
    SELECT 
        unnest(p_phone_numbers),
        unnest(v_phone_hashes);
    
    RETURN QUERY
    WITH cached_results AS (
        -- Get cached results
        SELECT 
            tpl.phone_number,
            qc.can_call,
            CASE WHEN qc.can_call THEN 0 
                 ELSE jsonb_array_length(qc.blocking_entries) 
            END as blocking_count,
            CASE WHEN qc.can_call THEN 0
                 ELSE (
                     SELECT MAX((entry->>'authority_level')::INTEGER)
                     FROM jsonb_array_elements(qc.blocking_entries) AS entry
                 )
            END as highest_authority,
            true as cache_hit
        FROM temp_phone_lookup tpl
        JOIN dnc_query_cache qc ON tpl.phone_hash = qc.phone_number_hash
        WHERE qc.expires_at > CURRENT_TIMESTAMP
        AND p_use_cache
    ),
    uncached_results AS (
        -- Get uncached results
        SELECT 
            tpl.phone_number,
            COUNT(e.id) = 0 as can_call,
            COUNT(e.id)::INTEGER as blocking_count,
            COALESCE(MAX(e.authority_level), 0) as highest_authority,
            false as cache_hit
        FROM temp_phone_lookup tpl
        LEFT JOIN dnc_entries e ON e.phone_number = tpl.phone_number 
            AND e.is_active = true
        WHERE NOT EXISTS (
            SELECT 1 FROM cached_results cr 
            WHERE cr.phone_number = tpl.phone_number
        )
        GROUP BY tpl.phone_number
    )
    SELECT * FROM cached_results
    UNION ALL
    SELECT * FROM uncached_results;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- DATA MANIPULATION QUERIES
-- =====================================================================

-- High-performance bulk insert with conflict resolution
-- Target: > 1K inserts/sec
CREATE OR REPLACE FUNCTION dnc_bulk_insert_entries(
    p_entries JSONB,
    p_provider_id UUID DEFAULT NULL,
    p_added_by UUID DEFAULT NULL
) RETURNS TABLE (
    inserted_count INTEGER,
    updated_count INTEGER,
    skipped_count INTEGER,
    error_count INTEGER,
    errors JSONB
) AS $$
DECLARE
    v_inserted INTEGER := 0;
    v_updated INTEGER := 0;
    v_skipped INTEGER := 0;
    v_errors INTEGER := 0;
    v_error_list JSONB := '[]'::jsonb;
    v_entry JSONB;
    v_authority_level INTEGER;
BEGIN
    -- Determine authority level based on provider
    SELECT 
        CASE p.type
            WHEN 'federal' THEN 4
            WHEN 'state' THEN 3
            WHEN 'internal' THEN 2
            ELSE 1
        END
    INTO v_authority_level
    FROM dnc_providers p
    WHERE p.id = p_provider_id;
    
    -- Default authority level if no provider
    v_authority_level := COALESCE(v_authority_level, 1);
    
    -- Process each entry
    FOR v_entry IN SELECT * FROM jsonb_array_elements(p_entries)
    LOOP
        BEGIN
            INSERT INTO dnc_entries (
                phone_number,
                phone_number_hash,
                list_source,
                suppress_reason,
                provider_id,
                added_by,
                authority_level,
                source_reference,
                notes,
                expires_at,
                metadata
            ) VALUES (
                v_entry->>'phone_number',
                encode(digest(v_entry->>'phone_number', 'sha256'), 'hex'),
                v_entry->>'list_source',
                v_entry->>'suppress_reason',
                p_provider_id,
                p_added_by,
                v_authority_level,
                v_entry->>'source_reference',
                v_entry->>'notes',
                CASE WHEN v_entry->>'expires_at' IS NOT NULL 
                     THEN (v_entry->>'expires_at')::TIMESTAMPTZ 
                     ELSE NULL 
                END,
                COALESCE(v_entry->'metadata', '{}'::jsonb)
            )
            ON CONFLICT (phone_number, list_source) 
            WHERE is_active = true
            DO UPDATE SET
                updated_at = CURRENT_TIMESTAMP,
                updated_by = p_added_by,
                source_reference = EXCLUDED.source_reference,
                notes = EXCLUDED.notes,
                metadata = EXCLUDED.metadata
            WHERE dnc_entries.authority_level <= EXCLUDED.authority_level;
            
            IF FOUND THEN
                v_updated := v_updated + 1;
            ELSE
                v_inserted := v_inserted + 1;
            END IF;
            
        EXCEPTION WHEN OTHERS THEN
            v_errors := v_errors + 1;
            v_error_list := v_error_list || jsonb_build_object(
                'entry', v_entry,
                'error', SQLERRM
            );
        END;
    END LOOP;
    
    -- Invalidate cache for affected numbers
    DELETE FROM dnc_query_cache
    WHERE phone_number_hash IN (
        SELECT encode(digest(entry->>'phone_number', 'sha256'), 'hex')
        FROM jsonb_array_elements(p_entries) AS entry
    );
    
    RETURN QUERY SELECT v_inserted, v_updated, v_skipped, v_errors, v_error_list;
END;
$$ LANGUAGE plpgsql;

-- Update provider sync status
CREATE OR REPLACE FUNCTION dnc_update_provider_sync_status(
    p_provider_id UUID,
    p_status VARCHAR(50),
    p_records_processed INTEGER DEFAULT NULL,
    p_error_message TEXT DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    v_sync_duration INTERVAL;
BEGIN
    -- Calculate sync duration if completing
    IF p_status = 'completed' THEN
        SELECT CURRENT_TIMESTAMP - sync_started_at
        INTO v_sync_duration
        FROM dnc_sync_history
        WHERE provider_id = p_provider_id
        AND status = 'running'
        ORDER BY sync_started_at DESC
        LIMIT 1;
    END IF;
    
    -- Update provider status
    UPDATE dnc_providers SET
        status = CASE 
            WHEN p_status = 'completed' THEN 'active'
            WHEN p_status = 'failed' THEN 'error'
            ELSE p_status
        END,
        last_sync_at = CASE WHEN p_status = 'completed' 
                           THEN CURRENT_TIMESTAMP 
                           ELSE last_sync_at 
                      END,
        next_sync_at = CASE WHEN p_status = 'completed' 
                           THEN CURRENT_TIMESTAMP + update_frequency
                           ELSE next_sync_at 
                      END,
        last_sync_duration = COALESCE(v_sync_duration, last_sync_duration),
        last_sync_records = COALESCE(p_records_processed, last_sync_records),
        success_count = CASE WHEN p_status = 'completed' 
                           THEN success_count + 1 
                           ELSE success_count 
                      END,
        error_count = CASE WHEN p_status = 'failed' 
                         THEN error_count + 1 
                         ELSE 0 
                    END,
        last_error = CASE WHEN p_status = 'failed' 
                         THEN p_error_message 
                         ELSE NULL 
                    END,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_provider_id;
    
    -- Update sync history
    UPDATE dnc_sync_history SET
        status = p_status,
        sync_completed_at = CASE WHEN p_status IN ('completed', 'failed') 
                               THEN CURRENT_TIMESTAMP 
                               ELSE sync_completed_at 
                          END,
        records_processed = COALESCE(p_records_processed, records_processed),
        error_message = p_error_message
    WHERE provider_id = p_provider_id
    AND status = 'running'
    AND sync_completed_at IS NULL;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- PROVIDER SYNC QUERIES
-- =====================================================================

-- Get providers that need synchronization
CREATE OR REPLACE FUNCTION dnc_get_providers_needing_sync()
RETURNS TABLE (
    id UUID,
    name VARCHAR(255),
    type VARCHAR(50),
    base_url VARCHAR(500),
    auth_type VARCHAR(50),
    priority INTEGER,
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    config JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.name,
        p.type,
        p.base_url,
        p.auth_type,
        p.priority,
        p.last_sync_at,
        p.next_sync_at,
        p.config
    FROM dnc_providers p
    WHERE p.enabled = true
    AND p.status != 'syncing'
    AND (
        p.next_sync_at IS NULL 
        OR p.next_sync_at <= CURRENT_TIMESTAMP
    )
    ORDER BY p.priority ASC, p.next_sync_at ASC NULLS FIRST;
END;
$$ LANGUAGE plpgsql;

-- Start sync operation for a provider
CREATE OR REPLACE FUNCTION dnc_start_provider_sync(
    p_provider_id UUID
) RETURNS UUID AS $$
DECLARE
    v_sync_id UUID;
BEGIN
    -- Update provider status
    UPDATE dnc_providers 
    SET status = 'syncing', updated_at = CURRENT_TIMESTAMP
    WHERE id = p_provider_id;
    
    -- Create sync history record
    INSERT INTO dnc_sync_history (
        provider_id,
        status
    ) VALUES (
        p_provider_id,
        'running'
    ) RETURNING id INTO v_sync_id;
    
    RETURN v_sync_id;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- COMPLIANCE REPORTING QUERIES
-- =====================================================================

-- Generate DNC compliance report
CREATE OR REPLACE FUNCTION dnc_generate_compliance_report(
    p_start_date TIMESTAMPTZ,
    p_end_date TIMESTAMPTZ,
    p_list_sources VARCHAR(50)[] DEFAULT NULL
) RETURNS TABLE (
    list_source VARCHAR(50),
    total_entries BIGINT,
    active_entries BIGINT,
    expired_entries BIGINT,
    federal_count BIGINT,
    state_count BIGINT,
    internal_count BIGINT,
    recent_additions BIGINT,
    compliance_score NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    WITH entry_stats AS (
        SELECT 
            e.list_source,
            COUNT(*) as total_entries,
            COUNT(*) FILTER (WHERE e.is_active) as active_entries,
            COUNT(*) FILTER (WHERE NOT e.is_active) as expired_entries,
            COUNT(*) FILTER (WHERE e.list_source = 'federal') as federal_count,
            COUNT(*) FILTER (WHERE e.list_source = 'state') as state_count,
            COUNT(*) FILTER (WHERE e.list_source = 'internal') as internal_count,
            COUNT(*) FILTER (WHERE e.added_at >= p_start_date) as recent_additions
        FROM dnc_entries e
        WHERE e.added_at <= p_end_date
        AND (p_list_sources IS NULL OR e.list_source = ANY(p_list_sources))
        GROUP BY e.list_source
    )
    SELECT 
        es.list_source,
        es.total_entries,
        es.active_entries,
        es.expired_entries,
        es.federal_count,
        es.state_count,
        es.internal_count,
        es.recent_additions,
        CASE 
            WHEN es.total_entries = 0 THEN 0
            ELSE ROUND(
                (es.active_entries::NUMERIC / es.total_entries::NUMERIC) * 100,
                2
            )
        END as compliance_score
    FROM entry_stats es
    ORDER BY es.list_source;
END;
$$ LANGUAGE plpgsql;

-- Get DNC entries by phone number pattern (for reporting)
CREATE OR REPLACE FUNCTION dnc_search_entries(
    p_phone_pattern VARCHAR(20) DEFAULT NULL,
    p_list_sources VARCHAR(50)[] DEFAULT NULL,
    p_suppress_reasons VARCHAR(50)[] DEFAULT NULL,
    p_active_only BOOLEAN DEFAULT true,
    p_limit INTEGER DEFAULT 1000,
    p_offset INTEGER DEFAULT 0
) RETURNS TABLE (
    id UUID,
    phone_number VARCHAR(20),
    list_source VARCHAR(50),
    suppress_reason VARCHAR(50),
    added_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN,
    provider_name VARCHAR(255),
    source_reference VARCHAR(255),
    notes TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.id,
        e.phone_number,
        e.list_source,
        e.suppress_reason,
        e.added_at,
        e.expires_at,
        e.is_active,
        p.name as provider_name,
        e.source_reference,
        e.notes
    FROM dnc_entries e
    LEFT JOIN dnc_providers p ON e.provider_id = p.id
    WHERE (p_phone_pattern IS NULL OR e.phone_number LIKE p_phone_pattern)
    AND (p_list_sources IS NULL OR e.list_source = ANY(p_list_sources))
    AND (p_suppress_reasons IS NULL OR e.suppress_reason = ANY(p_suppress_reasons))
    AND (NOT p_active_only OR e.is_active = true)
    ORDER BY e.added_at DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- ANALYTICS AND MONITORING QUERIES
-- =====================================================================

-- Get DNC lookup performance metrics
CREATE OR REPLACE FUNCTION dnc_get_performance_metrics(
    p_hours INTEGER DEFAULT 24
) RETURNS TABLE (
    metric_name VARCHAR(50),
    metric_value NUMERIC,
    metric_unit VARCHAR(20)
) AS $$
BEGIN
    RETURN QUERY
    WITH cache_stats AS (
        SELECT 
            COUNT(*) as total_cached_entries,
            SUM(hit_count) as total_cache_hits,
            AVG(hit_count) as avg_hits_per_entry,
            COUNT(*) FILTER (WHERE expires_at > CURRENT_TIMESTAMP) as active_cache_entries
        FROM dnc_query_cache
        WHERE last_hit_at >= CURRENT_TIMESTAMP - (p_hours || ' hours')::INTERVAL
    ),
    lookup_stats AS (
        SELECT 
            COUNT(*) as total_entries,
            COUNT(*) FILTER (WHERE is_active) as active_entries,
            COUNT(DISTINCT provider_id) as unique_providers
        FROM dnc_entries
    )
    SELECT 'total_entries'::VARCHAR(50), cs.total_cached_entries::NUMERIC, 'count'::VARCHAR(20) FROM cache_stats cs
    UNION ALL
    SELECT 'cache_hits', cs.total_cache_hits::NUMERIC, 'count' FROM cache_stats cs
    UNION ALL
    SELECT 'avg_hits_per_entry', cs.avg_hits_per_entry, 'count' FROM cache_stats cs
    UNION ALL
    SELECT 'active_cache_entries', cs.active_cache_entries::NUMERIC, 'count' FROM cache_stats cs
    UNION ALL
    SELECT 'total_dnc_entries', ls.total_entries::NUMERIC, 'count' FROM lookup_stats ls
    UNION ALL
    SELECT 'active_dnc_entries', ls.active_entries::NUMERIC, 'count' FROM lookup_stats ls
    UNION ALL
    SELECT 'unique_providers', ls.unique_providers::NUMERIC, 'count' FROM lookup_stats ls;
END;
$$ LANGUAGE plpgsql;

-- Provider health check query
CREATE OR REPLACE FUNCTION dnc_get_provider_health()
RETURNS TABLE (
    provider_id UUID,
    provider_name VARCHAR(255),
    provider_type VARCHAR(50),
    status VARCHAR(50),
    enabled BOOLEAN,
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    success_rate NUMERIC,
    error_count INTEGER,
    last_error TEXT,
    entries_count BIGINT,
    health_score INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.name,
        p.type,
        p.status,
        p.enabled,
        p.last_sync_at,
        p.next_sync_at,
        CASE 
            WHEN (p.success_count + p.error_count) = 0 THEN 0
            ELSE ROUND(
                (p.success_count::NUMERIC / (p.success_count + p.error_count)::NUMERIC) * 100,
                2
            )
        END as success_rate,
        p.error_count,
        p.last_error,
        COALESCE(e.entry_count, 0) as entries_count,
        CASE 
            WHEN NOT p.enabled THEN 0
            WHEN p.status = 'error' THEN 25
            WHEN p.status = 'inactive' THEN 50
            WHEN p.error_count > 5 THEN 60
            WHEN p.last_sync_at IS NULL THEN 70
            WHEN p.last_sync_at < CURRENT_TIMESTAMP - INTERVAL '2 days' THEN 80
            ELSE 100
        END as health_score
    FROM dnc_providers p
    LEFT JOIN (
        SELECT provider_id, COUNT(*) as entry_count
        FROM dnc_entries
        WHERE is_active = true
        GROUP BY provider_id
    ) e ON p.id = e.provider_id
    ORDER BY health_score DESC, p.priority ASC;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- MAINTENANCE AND CLEANUP QUERIES
-- =====================================================================

-- Clean up expired cache entries
CREATE OR REPLACE FUNCTION dnc_cleanup_expired_cache()
RETURNS INTEGER AS $$
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM dnc_query_cache
    WHERE expires_at <= CURRENT_TIMESTAMP;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Clean up expired DNC entries
CREATE OR REPLACE FUNCTION dnc_cleanup_expired_entries()
RETURNS INTEGER AS $$
DECLARE
    v_updated_count INTEGER;
BEGIN
    -- Mark expired entries as inactive
    -- Note: We don't delete for audit trail purposes
    UPDATE dnc_entries 
    SET updated_at = CURRENT_TIMESTAMP
    WHERE expires_at IS NOT NULL 
    AND expires_at <= CURRENT_TIMESTAMP 
    AND is_active = true;
    
    GET DIAGNOSTICS v_updated_count = ROW_COUNT;
    
    -- Clear cache for affected numbers
    DELETE FROM dnc_query_cache
    WHERE phone_number_hash IN (
        SELECT phone_number_hash
        FROM dnc_entries
        WHERE expires_at IS NOT NULL 
        AND expires_at <= CURRENT_TIMESTAMP
    );
    
    RETURN v_updated_count;
END;
$$ LANGUAGE plpgsql;

-- Vacuum and analyze DNC tables for optimal performance
CREATE OR REPLACE FUNCTION dnc_optimize_tables()
RETURNS VOID AS $$
BEGIN
    ANALYZE dnc_entries;
    ANALYZE dnc_providers;
    ANALYZE dnc_query_cache;
    ANALYZE dnc_sync_history;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- FULL-TEXT SEARCH FOR PROVIDER MANAGEMENT
-- =====================================================================

-- Add full-text search index for provider management
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dnc_providers_search 
    ON dnc_providers USING GIN (
        to_tsvector('english', name || ' ' || COALESCE(config->>'description', ''))
    );

-- Search providers by text
CREATE OR REPLACE FUNCTION dnc_search_providers(
    p_search_text TEXT,
    p_enabled_only BOOLEAN DEFAULT true
) RETURNS TABLE (
    id UUID,
    name VARCHAR(255),
    type VARCHAR(50),
    status VARCHAR(50),
    enabled BOOLEAN,
    rank REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.name,
        p.type,
        p.status,
        p.enabled,
        ts_rank(
            to_tsvector('english', p.name || ' ' || COALESCE(p.config->>'description', '')),
            plainto_tsquery('english', p_search_text)
        ) as rank
    FROM dnc_providers p
    WHERE to_tsvector('english', p.name || ' ' || COALESCE(p.config->>'description', ''))
          @@ plainto_tsquery('english', p_search_text)
    AND (NOT p_enabled_only OR p.enabled = true)
    ORDER BY rank DESC, p.priority ASC;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- BATCH PROCESSING OPTIMIZATIONS
-- =====================================================================

-- Create a function for batch processing with connection pool optimization
CREATE OR REPLACE FUNCTION dnc_batch_process_sync_queue(
    p_batch_size INTEGER DEFAULT 1000,
    p_max_batches INTEGER DEFAULT 10
) RETURNS TABLE (
    processed_batches INTEGER,
    total_processed INTEGER,
    processing_time_ms NUMERIC
) AS $$
DECLARE
    v_start_time TIMESTAMPTZ;
    v_batch_count INTEGER := 0;
    v_total_count INTEGER := 0;
    v_provider RECORD;
BEGIN
    v_start_time := clock_timestamp();
    
    -- Process providers that need sync in priority order
    FOR v_provider IN 
        SELECT * FROM dnc_get_providers_needing_sync()
        LIMIT p_max_batches
    LOOP
        -- Simulate batch processing logic here
        -- In real implementation, this would trigger sync workers
        v_batch_count := v_batch_count + 1;
        v_total_count := v_total_count + p_batch_size;
        
        -- Update provider next sync time to prevent duplicate processing
        UPDATE dnc_providers 
        SET next_sync_at = CURRENT_TIMESTAMP + update_frequency
        WHERE id = v_provider.id;
    END LOOP;
    
    RETURN QUERY 
    SELECT 
        v_batch_count,
        v_total_count,
        EXTRACT(EPOCH FROM (clock_timestamp() - v_start_time)) * 1000;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- MATERIALIZED VIEWS FOR REPORTING
-- =====================================================================

-- Materialized view for DNC statistics (refreshed periodically)
CREATE MATERIALIZED VIEW IF NOT EXISTS dnc_stats_daily AS
WITH daily_stats AS (
    SELECT 
        DATE(added_at) as stat_date,
        list_source,
        COUNT(*) as entries_added,
        COUNT(*) FILTER (WHERE is_active) as active_entries,
        COUNT(DISTINCT provider_id) as providers_used
    FROM dnc_entries
    WHERE added_at >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY DATE(added_at), list_source
)
SELECT 
    stat_date,
    list_source,
    entries_added,
    active_entries,
    providers_used,
    SUM(entries_added) OVER (
        PARTITION BY list_source 
        ORDER BY stat_date 
        ROWS UNBOUNDED PRECEDING
    ) as cumulative_entries
FROM daily_stats
ORDER BY stat_date DESC, list_source;

-- Index for the materialized view
CREATE INDEX IF NOT EXISTS idx_dnc_stats_daily_date_source 
    ON dnc_stats_daily (stat_date DESC, list_source);

-- Function to refresh the materialized view
CREATE OR REPLACE FUNCTION dnc_refresh_daily_stats()
RETURNS VOID AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY dnc_stats_daily;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- QUERY PERFORMANCE MONITORING
-- =====================================================================

-- View to monitor query performance
CREATE OR REPLACE VIEW dnc_query_performance AS
SELECT 
    'dnc_check_phone_number' as query_name,
    COUNT(*) as total_calls,
    AVG(hit_count) as avg_cache_hits,
    MAX(last_hit_at) as last_execution,
    COUNT(*) FILTER (WHERE hit_count > 0) as cache_hit_rate
FROM dnc_query_cache
WHERE last_hit_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'
UNION ALL
SELECT 
    'bulk_lookups' as query_name,
    COUNT(DISTINCT phone_number_hash) as total_calls,
    AVG(hit_count) as avg_cache_hits,
    MAX(last_hit_at) as last_execution,
    COUNT(*) FILTER (WHERE hit_count > 10) as high_usage_entries
FROM dnc_query_cache
WHERE last_hit_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours';

-- =====================================================================
-- TABLE PARTITIONING FOR LARGE DATASETS
-- =====================================================================

-- Function to create monthly partitions for DNC entries (for very large datasets)
CREATE OR REPLACE FUNCTION dnc_create_monthly_partition(
    p_year INTEGER,
    p_month INTEGER
) RETURNS VOID AS $$
DECLARE
    v_partition_name TEXT;
    v_start_date DATE;
    v_end_date DATE;
BEGIN
    v_start_date := DATE(p_year || '-' || LPAD(p_month::TEXT, 2, '0') || '-01');
    v_end_date := v_start_date + INTERVAL '1 month';
    v_partition_name := 'dnc_entries_' || p_year || '_' || LPAD(p_month::TEXT, 2, '0');
    
    -- Note: This would be used if dnc_entries table was partitioned
    -- Currently kept as reference for future scaling needs
    RAISE NOTICE 'Partition function ready: % for range % to %', 
        v_partition_name, v_start_date, v_end_date;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- COMMENTS AND DOCUMENTATION
-- =====================================================================

COMMENT ON TABLE dnc_entries IS 
'High-performance DNC entries table with computed columns and optimized indexes for < 5ms lookups';

COMMENT ON TABLE dnc_providers IS 
'DNC data providers with sync scheduling and health monitoring capabilities';

COMMENT ON TABLE dnc_query_cache IS 
'Query result cache for DNC lookups to achieve > 10K/sec throughput';

COMMENT ON FUNCTION dnc_check_phone_number(VARCHAR, BOOLEAN) IS 
'Primary DNC lookup function with caching support. Target: < 2ms response time';

COMMENT ON FUNCTION dnc_check_phone_numbers_bulk(VARCHAR[], BOOLEAN) IS 
'Bulk DNC lookup function for high-throughput scenarios. Target: > 10K lookups/sec';

COMMENT ON FUNCTION dnc_bulk_insert_entries(JSONB, UUID, UUID) IS 
'High-performance bulk insert with conflict resolution and authority-based updates';

-- =====================================================================
-- END OF DNC QUERY LIBRARY
-- =====================================================================