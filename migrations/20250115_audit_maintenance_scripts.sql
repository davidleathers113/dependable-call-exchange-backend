-- =====================================================================
-- AUDIT SYSTEM MAINTENANCE SCRIPTS
-- 
-- This file contains useful queries and maintenance scripts for the
-- immutable audit logging system. These are not part of the migration
-- but are provided for operational use.
-- =====================================================================

-- =====================================================================
-- MONITORING QUERIES
-- =====================================================================

-- Check audit system health
SELECT * FROM audit.health_check;

-- View recent audit statistics
SELECT * FROM audit.stats;

-- Check partition status
SELECT * FROM audit.partition_status;

-- Find gaps in sequence numbers (indicates potential data loss)
WITH sequence_gaps AS (
    SELECT 
        sequence_num,
        LAG(sequence_num) OVER (ORDER BY sequence_num) as prev_seq,
        sequence_num - LAG(sequence_num) OVER (ORDER BY sequence_num) as gap
    FROM audit.events
)
SELECT 
    prev_seq + 1 as gap_start,
    sequence_num - 1 as gap_end,
    gap - 1 as missing_events
FROM sequence_gaps
WHERE gap > 1
ORDER BY prev_seq;

-- =====================================================================
-- COMPLIANCE QUERIES
-- =====================================================================

-- TCPA compliance events in last 24 hours
SELECT 
    timestamp,
    actor_id,
    target_id,
    action,
    result,
    metadata
FROM audit.events
WHERE compliance_flags->>'tcpa_relevant' = 'true'
  AND timestamp > CURRENT_TIMESTAMP - INTERVAL '24 hours'
ORDER BY timestamp DESC;

-- GDPR data access events
SELECT 
    timestamp,
    actor_id,
    target_id,
    action,
    data_classes,
    legal_basis,
    metadata
FROM audit.events
WHERE type IN ('data.accessed', 'data.exported', 'data.deleted')
  AND compliance_flags->>'gdpr_relevant' = 'true'
  AND timestamp > CURRENT_TIMESTAMP - INTERVAL '30 days'
ORDER BY timestamp DESC;

-- Consent management audit trail
SELECT 
    timestamp,
    actor_id,
    target_id,
    action,
    result,
    metadata->>'consent_type' as consent_type,
    metadata->>'duration_days' as duration_days
FROM audit.events
WHERE type IN ('consent.granted', 'consent.revoked', 'consent.updated')
ORDER BY target_id, timestamp DESC;

-- =====================================================================
-- SECURITY QUERIES
-- =====================================================================

-- Failed authentication attempts
SELECT 
    timestamp,
    actor_id,
    actor_ip,
    error_message,
    metadata
FROM audit.events
WHERE type = 'security.auth_failure'
  AND timestamp > CURRENT_TIMESTAMP - INTERVAL '1 hour'
ORDER BY timestamp DESC;

-- Suspicious activity patterns
WITH actor_stats AS (
    SELECT 
        actor_id,
        actor_ip,
        COUNT(*) as event_count,
        COUNT(DISTINCT type) as unique_events,
        COUNT(CASE WHEN result = 'failure' THEN 1 END) as failures
    FROM audit.events
    WHERE timestamp > CURRENT_TIMESTAMP - INTERVAL '1 hour'
    GROUP BY actor_id, actor_ip
)
SELECT *
FROM actor_stats
WHERE failures > 10 
   OR event_count > 1000
   OR (failures::FLOAT / NULLIF(event_count, 0)) > 0.5
ORDER BY failures DESC, event_count DESC;

-- Access to sensitive data
SELECT 
    timestamp,
    actor_id,
    target_type,
    action,
    data_classes,
    metadata
FROM audit.events
WHERE 'PII' = ANY(data_classes)
   OR 'financial' = ANY(data_classes)
   OR 'health' = ANY(data_classes)
ORDER BY timestamp DESC
LIMIT 100;

-- =====================================================================
-- PERFORMANCE QUERIES
-- =====================================================================

-- Event ingestion rate (events per minute)
SELECT 
    DATE_TRUNC('minute', timestamp) as minute,
    COUNT(*) as events_per_minute,
    AVG(COUNT(*)) OVER (ORDER BY DATE_TRUNC('minute', timestamp) 
        ROWS BETWEEN 5 PRECEDING AND CURRENT ROW) as rolling_avg
FROM audit.events
WHERE timestamp > CURRENT_TIMESTAMP - INTERVAL '1 hour'
GROUP BY DATE_TRUNC('minute', timestamp)
ORDER BY minute DESC;

-- Partition sizes and growth
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size('audit.'||tablename)) as total_size,
    pg_size_pretty(pg_table_size('audit.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size('audit.'||tablename)) as indexes_size,
    pg_stat_user_tables.n_live_tup as row_count
FROM pg_tables
LEFT JOIN pg_stat_user_tables ON tablename = relname
WHERE schemaname = 'audit' AND tablename LIKE 'events_%'
ORDER BY tablename;

-- =====================================================================
-- INTEGRITY VERIFICATION
-- =====================================================================

-- Verify hash chain for last 1000 events
SELECT * FROM audit.verify_hash_chain(
    (SELECT MAX(sequence_num) - 1000 FROM audit.events),
    (SELECT MAX(sequence_num) FROM audit.events)
);

-- Check for hash collisions (should return 0)
SELECT 
    event_hash,
    COUNT(*) as occurrences
FROM audit.events
GROUP BY event_hash
HAVING COUNT(*) > 1;

-- Verify sequence number continuity
WITH seq_check AS (
    SELECT 
        MIN(sequence_num) as min_seq,
        MAX(sequence_num) as max_seq,
        COUNT(*) as event_count
    FROM audit.events
)
SELECT 
    CASE 
        WHEN max_seq - min_seq + 1 = event_count 
        THEN 'PASS: No gaps in sequence'
        ELSE 'FAIL: Gaps detected in sequence'
    END as integrity_check,
    max_seq - min_seq + 1 as expected_count,
    event_count as actual_count,
    max_seq - min_seq + 1 - event_count as missing_events
FROM seq_check;

-- =====================================================================
-- MAINTENANCE OPERATIONS
-- =====================================================================

-- Create hash checkpoint manually
SELECT audit.create_hash_checkpoint();

-- Create next month's partition manually
SELECT audit.auto_create_partitions();

-- Analyze partition for query optimization
ANALYZE audit.events_2025_01;

-- Vacuum partition (if needed, though autovacuum should handle)
VACUUM ANALYZE audit.events_2025_01;

-- =====================================================================
-- ARCHIVAL OPERATIONS
-- =====================================================================

-- Identify partitions ready for archival (older than 6 months)
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size('audit.'||tablename)) as size,
    EXTRACT(YEAR FROM tablename::text::date) as year,
    EXTRACT(MONTH FROM tablename::text::date) as month
FROM pg_tables
WHERE schemaname = 'audit' 
  AND tablename LIKE 'events_%'
  AND tablename::text < 'events_' || TO_CHAR(CURRENT_DATE - INTERVAL '6 months', 'YYYY_MM')
ORDER BY tablename;

-- Export partition data for archival (example)
-- COPY (SELECT * FROM audit.events_2024_07) 
-- TO '/tmp/audit_events_2024_07.csv' 
-- WITH (FORMAT CSV, HEADER TRUE, DELIMITER ',');

-- =====================================================================
-- ACCESS AUDIT
-- =====================================================================

-- Who accessed audit logs
SELECT 
    accessed_at,
    accessor_id,
    query_type,
    events_accessed,
    purpose
FROM audit.access_log
WHERE accessed_at > CURRENT_TIMESTAMP - INTERVAL '7 days'
ORDER BY accessed_at DESC;

-- Unusual access patterns
WITH access_stats AS (
    SELECT 
        accessor_id,
        COUNT(*) as access_count,
        SUM(events_accessed) as total_events_accessed,
        COUNT(DISTINCT DATE(accessed_at)) as days_accessed
    FROM audit.access_log
    WHERE accessed_at > CURRENT_TIMESTAMP - INTERVAL '30 days'
    GROUP BY accessor_id
)
SELECT *
FROM access_stats
WHERE access_count > 100
   OR total_events_accessed > 10000
   OR days_accessed > 20
ORDER BY total_events_accessed DESC;

-- =====================================================================
-- USEFUL FUNCTIONS FOR AD-HOC QUERIES
-- =====================================================================

-- Function to get audit trail for a specific target
CREATE OR REPLACE FUNCTION audit.get_target_history(
    p_target_id VARCHAR(100),
    p_limit INTEGER DEFAULT 100
) RETURNS TABLE (
    timestamp TIMESTAMPTZ,
    type VARCHAR(100),
    actor_id VARCHAR(100),
    action VARCHAR(100),
    result VARCHAR(50),
    metadata JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.timestamp,
        e.type,
        e.actor_id,
        e.action,
        e.result,
        e.metadata
    FROM audit.events e
    WHERE e.target_id = p_target_id
    ORDER BY e.timestamp DESC
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Function to get actor activity summary
CREATE OR REPLACE FUNCTION audit.get_actor_summary(
    p_actor_id VARCHAR(100),
    p_days INTEGER DEFAULT 7
) RETURNS TABLE (
    event_type VARCHAR(100),
    event_count BIGINT,
    success_count BIGINT,
    failure_count BIGINT,
    last_occurrence TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.type as event_type,
        COUNT(*) as event_count,
        COUNT(CASE WHEN e.result = 'success' THEN 1 END) as success_count,
        COUNT(CASE WHEN e.result = 'failure' THEN 1 END) as failure_count,
        MAX(e.timestamp) as last_occurrence
    FROM audit.events e
    WHERE e.actor_id = p_actor_id
      AND e.timestamp > CURRENT_TIMESTAMP - (p_days || ' days')::INTERVAL
    GROUP BY e.type
    ORDER BY event_count DESC;
END;
$$ LANGUAGE plpgsql;

-- Usage examples:
-- SELECT * FROM audit.get_target_history('user_12345');
-- SELECT * FROM audit.get_actor_summary('admin_user', 30);

-- =====================================================================
-- END OF MAINTENANCE SCRIPTS
-- =====================================================================