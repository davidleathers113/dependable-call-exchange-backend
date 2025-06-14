# Immutable Audit Logging Schema

## Overview

This migration implements a comprehensive immutable audit logging system for the Dependable Call Exchange Backend, designed to meet strict compliance requirements (TCPA, GDPR, CCPA, SOX) with cryptographic integrity guarantees.

## Features

### 1. **Immutable Storage**
- Cryptographic hash chain linking all events
- Triggers prevent any UPDATE or DELETE operations
- Sequence numbers ensure no gaps in the audit trail

### 2. **Partitioning Strategy**
- Monthly partitions for efficient querying
- Automatic partition creation for future months
- Support for 7-year retention with archival

### 3. **Performance Optimization**
- Comprehensive indexes for common query patterns
- GIN indexes for JSONB fields (metadata, compliance_flags)
- Partition-aware queries for fast retrieval

### 4. **Security Controls**
- Separate schema (`audit`) for namespace isolation
- Role-based access control (reader, writer, admin)
- Audit log for who accesses the audit logs

### 5. **Monitoring & Health Checks**
- Built-in views for system health monitoring
- Partition status tracking
- Event statistics and anomaly detection

## Migration Files

1. **`20250115_create_audit_schema.up.sql`**
   - Creates the complete audit infrastructure
   - Sets up partitioned tables, indexes, and functions
   - Implements security controls and monitoring views

2. **`20250115_create_audit_schema.down.sql`**
   - Safely rolls back all audit infrastructure
   - Preserves data integrity during rollback

3. **`20250115_audit_maintenance_scripts.sql`**
   - Operational queries for monitoring and maintenance
   - Compliance report queries
   - Integrity verification procedures

## Key Tables

### `audit.events`
The main partitioned table storing all audit events with:
- Unique sequence numbers
- Cryptographic hash chain
- Comprehensive event metadata
- Compliance flags and data classification

### `audit.hash_chain`
Periodic checkpoints for efficient hash chain verification

### `audit.archives`
Metadata for archived partitions stored in S3

### `audit.access_log`
Tracks who accessed the audit logs (auditing the auditors)

## Usage Examples

### Writing Audit Events

```sql
INSERT INTO audit.events (
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
    compliance_flags,
    metadata
) VALUES (
    EXTRACT(EPOCH FROM CURRENT_TIMESTAMP) * 1000000000,
    'consent.granted',
    'INFO',
    'compliance',
    'user_123',
    'user',
    '+14155551234',
    'phone_number',
    'grant_consent',
    'success',
    'production',
    'call_service',
    '1.0.0',
    (SELECT event_hash FROM audit.events ORDER BY sequence_num DESC LIMIT 1),
    encode(digest('event_data_here', 'sha256'), 'hex'),
    '{"tcpa_compliant": true, "gdpr_relevant": false}'::jsonb,
    '{"consent_type": "calls", "duration_days": 365}'::jsonb
);
```

### Querying Audit Events

```sql
-- Recent compliance events
SELECT * FROM audit.events
WHERE compliance_flags->>'tcpa_relevant' = 'true'
  AND timestamp > CURRENT_TIMESTAMP - INTERVAL '24 hours';

-- Actor activity summary
SELECT * FROM audit.get_actor_summary('user_123', 7);

-- Target history
SELECT * FROM audit.get_target_history('+14155551234');
```

### Monitoring

```sql
-- Check system health
SELECT * FROM audit.health_check;

-- View partition status
SELECT * FROM audit.partition_status;

-- Verify hash chain integrity
SELECT * FROM audit.verify_hash_chain(1, 1000);
```

## Maintenance

### Automatic Tasks (with pg_cron)

```sql
-- Create future partitions monthly
SELECT cron.schedule('audit-create-partitions', '0 0 1 * *', 
    'SELECT audit.auto_create_partitions()');

-- Create hash checkpoints hourly
SELECT cron.schedule('audit-create-checkpoint', '0 * * * *', 
    'SELECT audit.create_hash_checkpoint()');

-- Verify integrity daily
SELECT cron.schedule('audit-verify-integrity', '0 2 * * *', 
    'SELECT * FROM audit.verify_hash_chain(1, 1000000)');
```

### Manual Maintenance

```sql
-- Create next month's partition
SELECT audit.auto_create_partitions();

-- Create hash checkpoint
SELECT audit.create_hash_checkpoint();

-- Archive old partition
-- 1. Export data to S3
-- 2. Verify archive integrity
-- 3. Drop partition using audit.drop_old_partition(year, month)
```

## Security Roles

### `audit_reader`
- SELECT permission on all audit tables
- For compliance officers and auditors

### `audit_writer`
- INSERT permission on audit.events
- For application services

### `audit_admin`
- Full permissions (except DELETE/UPDATE on events)
- For maintenance and administration

## Best Practices

1. **Always include previous_hash** when inserting events
2. **Use appropriate indexes** - query patterns are optimized for time-based and actor/target lookups
3. **Monitor partition sizes** - archive partitions older than 6 months
4. **Verify hash chain regularly** - automated daily verification recommended
5. **Log access to audit logs** - maintain chain of custody

## Compliance Features

### TCPA Compliance
- Track consent management events
- Enforce calling hour restrictions
- Maintain opt-out requests

### GDPR Compliance
- Track data access and modifications
- Support right to erasure (via metadata, not deletion)
- Maintain legal basis for processing

### CCPA Compliance
- Track data sales opt-outs
- Maintain consumer request audit trail
- Support data portability exports

### SOX Compliance
- Immutable financial transaction logs
- Access control and segregation of duties
- Tamper-evident audit trail

## Performance Considerations

- Partition pruning ensures queries only scan relevant months
- Indexes optimized for common query patterns
- Async write support via application layer buffering
- Target: < 5ms write latency, < 1s query for 1M events

## Troubleshooting

### Common Issues

1. **Sequence gaps detected**
   - Check for failed transactions
   - Verify no manual sequence manipulation

2. **Hash chain verification failures**
   - Check for system clock issues
   - Verify no direct table modifications

3. **Partition not created**
   - Run `SELECT audit.auto_create_partitions()`
   - Check for disk space issues

4. **Query performance degradation**
   - Analyze partitions: `ANALYZE audit.events_YYYY_MM`
   - Check for missing indexes
   - Verify partition pruning is working

## Future Enhancements

1. **Blockchain Integration**
   - Periodic anchoring to public blockchain
   - Distributed verification network

2. **Machine Learning**
   - Anomaly detection models
   - Predictive compliance alerts

3. **Advanced Analytics**
   - Real-time compliance dashboards
   - Trend analysis and forecasting