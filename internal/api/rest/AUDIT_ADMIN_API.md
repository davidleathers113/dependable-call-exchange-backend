# Audit Admin API Documentation

This document describes the admin-only API endpoints for the IMMUTABLE_AUDIT feature management in the Dependable Call Exchange Backend.

## Overview

The Audit Admin API provides comprehensive management capabilities for the audit system, including:

- **Integrity verification**: Manual and automated integrity checks
- **System health monitoring**: Real-time health status and metrics
- **Chain repair operations**: Repair broken audit chains
- **Corruption analysis**: Detailed corruption detection and reporting
- **Performance statistics**: Comprehensive audit system analytics

## Security

All admin endpoints require:
- Valid authentication token with admin privileges
- One of the following permissions: `admin`, `audit:admin`, or `system:admin`
- Rate limiting (more restrictive than regular API endpoints)
- Request logging and audit trails

## API Endpoints

### 1. Trigger Integrity Check

**POST** `/api/v1/admin/audit/verify`

Manually trigger an integrity verification check.

#### Request Body

```json
{
  "check_type": "hash_chain",           // Required: hash_chain, sequence, corruption, comprehensive
  "start_sequence": 1000,               // Optional: Starting sequence number
  "end_sequence": 2000,                 // Optional: Ending sequence number  
  "priority": "normal",                 // Optional: low, normal, high, critical
  "async_mode": true,                   // Optional: Run asynchronously (default: true)
  "criteria": {                         // Optional: Advanced criteria
    "include_compliance_check": true,
    "verify_signatures": true,
    "deep_scan": false
  },
  "metadata": {                         // Optional: Additional metadata
    "initiated_by": "admin_user",
    "reason": "routine_check"
  }
}
```

#### Response (Async Mode)

```json
{
  "success": true,
  "data": {
    "check_id": "550e8400-e29b-41d4-a716-446655440000",
    "check_type": "hash_chain",
    "status": "queued",
    "started_at": "2025-01-15T10:30:00Z",
    "_links": {
      "self": "/api/v1/admin/audit/verify/550e8400-e29b-41d4-a716-446655440000",
      "status": "/api/v1/admin/audit/verify/550e8400-e29b-41d4-a716-446655440000"
    }
  },
  "meta": {
    "request_id": "req_123456",
    "timestamp": "2025-01-15T10:30:00Z",
    "version": "v1"
  }
}
```

#### Response (Sync Mode)

```json
{
  "success": true,
  "data": {
    "check_id": "550e8400-e29b-41d4-a716-446655440000",
    "check_type": "hash_chain",
    "status": "completed",
    "started_at": "2025-01-15T10:30:00Z",
    "completed_at": "2025-01-15T10:32:15Z",
    "result": {
      "check_id": "550e8400-e29b-41d4-a716-446655440000",
      "overall_score": 0.9987,
      "total_events": 1000,
      "verified_events": 999,
      "issues": [
        {
          "sequence_number": 1543,
          "severity": "low",
          "description": "Minor hash inconsistency",
          "auto_resolved": true
        }
      ],
      "duration": "2m15s"
    },
    "_links": {
      "self": "/api/v1/admin/audit/verify/550e8400-e29b-41d4-a716-446655440000",
      "report": "/api/v1/admin/audit/verify/550e8400-e29b-41d4-a716-446655440000/report"
    }
  }
}
```

### 2. Get System Health

**GET** `/api/v1/admin/audit/health`

Retrieve comprehensive audit system health status.

#### Response

```json
{
  "success": true,
  "data": {
    "overall_status": "healthy",        // healthy, degraded, critical, down
    "last_updated": "2025-01-15T10:30:00Z",
    "integrity_status": {
      "is_running": true,
      "last_check": "2025-01-15T10:25:00Z",
      "checks_today": 25,
      "failed_checks": 0,
      "health_status": "healthy",
      "average_check_time": "45.2s"
    },
    "compliance_status": {
      "status": "healthy",
      "active_engines": 3,
      "failed_checks": 0,
      "last_check": "2025-01-15T10:25:00Z",
      "compliance_score": 0.99,
      "violations_today": 2
    },
    "logger_status": {
      "status": "healthy",
      "events_buffered": 150,
      "events_dropped": 0,
      "processing_rate": 1500.5,
      "average_latency": "2.3ms",
      "circuit_state": "closed",
      "workers_active": 4
    },
    "metrics": {
      "events_today": 45230,
      "integrity_score": 0.9987,
      "corruption_rate": 0.0013,
      "compliance_score": 0.9912,
      "system_uptime": "15d 4h 32m",
      "last_full_check": "2025-01-15T08:30:00Z",
      "average_check_time": "45.2s",
      "data_integrity_trend": "stable"
    },
    "active_alerts": [
      {
        "id": "alert-001",
        "severity": "warning",
        "type": "integrity",
        "message": "Minor hash chain inconsistency detected",
        "created_at": "2025-01-15T10:00:00Z",
        "source": "integrity_service",
        "affected_system": "hash_chain"
      }
    ],
    "system_load": {
      "cpu_usage": 45.2,
      "memory_usage": 62.8,
      "disk_usage": 78.3,
      "active_checks": 3,
      "queued_operations": 12
    },
    "_links": {
      "self": "/api/v1/admin/audit/health",
      "stats": "/api/v1/admin/audit/stats",
      "corruption": "/api/v1/admin/audit/corruption",
      "alerts": "/api/v1/admin/audit/alerts"
    }
  }
}
```

### 3. Chain Repair Operations

**POST** `/api/v1/admin/audit/repair`

Initiate chain repair operations for corrupted audit data.

#### Request Body

```json
{
  "start_sequence": 1000,               // Required: Starting sequence number
  "end_sequence": 2000,                 // Required: Ending sequence number
  "repair_strategy": "rebuild",         // Required: rebuild, reconstruct, merge, verify_only
  "dry_run": false,                     // Optional: Test without making changes
  "force_repair": false,                // Optional: Force repair even if risky
  "backup_data": true,                  // Optional: Create backup before repair
  "options": {                          // Optional: Advanced repair options
    "verify_signatures": true,
    "rebuild_hashes": true,
    "fix_sequence_gaps": true,
    "preserve_metadata": true
  }
}
```

#### Response

```json
{
  "success": true,
  "data": {
    "repair_id": "repair-550e8400-e29b-41d4-a716-446655440000",
    "status": "queued",                  // queued, running, completed, failed
    "started_at": "2025-01-15T10:30:00Z",
    "dry_run": false,
    "backup_location": "/var/backups/audit/backup-20250115-103000.sql.gz",
    "_links": {
      "self": "/api/v1/admin/audit/repair/repair-550e8400-e29b-41d4-a716-446655440000",
      "status": "/api/v1/admin/audit/repair/repair-550e8400-e29b-41d4-a716-446655440000",
      "progress": "/api/v1/admin/audit/repair/repair-550e8400-e29b-41d4-a716-446655440000/progress"
    }
  }
}
```

### 4. Detailed Statistics

**GET** `/api/v1/admin/audit/stats`

Get comprehensive audit system statistics and metrics.

#### Query Parameters

- `period` (optional): `last_24h`, `last_7d`, `last_30d`, `all_time` (default: `last_24h`)

#### Response

```json
{
  "success": true,
  "data": {
    "generated_at": "2025-01-15T10:30:00Z",
    "period": "last_24h",
    "event_statistics": {
      "total_events": 50000,
      "events_by_type": {
        "call": 30000,
        "bid": 15000,
        "account": 5000
      },
      "events_by_source": {
        "api": 40000,
        "webhook": 8000,
        "batch": 2000
      },
      "average_per_day": 2083.3,
      "peak_hour": 14,
      "processing_rate": 1250.5
    },
    "integrity_statistics": {
      "total_checks": 145,
      "successful_checks": 142,
      "failed_checks": 3,
      "corruption_detected": 1,
      "auto_repairs": 0,
      "manual_repairs": 1,
      "average_check_time": "45s",
      "integrity_score": 0.9987,
      "checks_by_type": {
        "hash_chain": 120,
        "sequence": 20,
        "corruption": 5
      }
    },
    "compliance_statistics": {
      "total_checks": 234,
      "violations": 5,
      "violations_by_type": {
        "tcpa": 3,
        "gdpr": 1,
        "dnc": 1
      },
      "compliance_score": 0.9912,
      "auto_remediation": 3,
      "manual_remediation": 2
    },
    "performance_statistics": {
      "average_latency": "2.3ms",
      "p95_latency": "8.5ms",
      "p99_latency": "15.2ms",
      "throughput_per_sec": 1250.5,
      "error_rate": 0.0023,
      "system_uptime": "372h32m"
    },
    "alert_statistics": {
      "total_alerts": 23,
      "active_alerts": 1,
      "resolved_alerts": 22,
      "alerts_by_severity": {
        "critical": 0,
        "warning": 1,
        "info": 22
      },
      "alerts_by_type": {
        "integrity": 15,
        "compliance": 5,
        "performance": 3
      },
      "mttr": "25m"
    },
    "trend_analysis": {
      "integrity_trend": "stable",
      "compliance_trend": "improving",
      "performance_trend": "stable",
      "alert_trend": "improving",
      "trend_period": "last_24h",
      "last_analyzed": "2025-01-15T09:30:00Z"
    }
  }
}
```

### 5. Corruption Report

**GET** `/api/v1/admin/audit/corruption`

Get detailed corruption analysis and incident reports.

#### Query Parameters

- `period` (optional): `last_24h`, `last_7d`, `last_30d` (default: `last_7d`)
- `severity` (optional): Filter by severity level
- `limit` (optional): Maximum number of incidents to return (default: 100, max: 1000)

#### Response

```json
{
  "success": true,
  "data": {
    "generated_at": "2025-01-15T10:30:00Z",
    "scan_period": "last_7d",
    "corruption_summary": {
      "total_incidents": 5,
      "high_severity": 0,
      "medium_severity": 1,
      "low_severity": 4,
      "auto_resolved": 3,
      "manual_intervention": 1,
      "data_loss": 0,
      "last_incident": "2025-01-15T08:00:00Z"
    },
    "corruption_incidents": [
      {
        "id": "corruption-001",
        "detected_at": "2025-01-15T08:00:00Z",
        "severity": "medium",
        "type": "hash_mismatch",
        "affected_range": {
          "start_sequence": 15000,
          "end_sequence": 15100
        },
        "description": "Hash chain inconsistency detected in sequence range",
        "root_cause": "Concurrent write operation during maintenance",
        "status": "resolved",
        "resolution_action": "Hash chain rebuilt for affected range",
        "resolved_at": "2025-01-15T09:00:00Z",
        "metadata": {
          "affected_events": 100,
          "repair_duration": "15m",
          "data_recovered": true
        }
      }
    ],
    "affected_systems": [
      "hash_chain",
      "event_store"
    ],
    "recommended_actions": [
      {
        "priority": "medium",
        "action": "schedule_full_integrity_check",
        "description": "Perform comprehensive integrity verification",
        "impact": "Ensures system-wide data integrity",
        "effort": "low"
      },
      {
        "priority": "low",
        "action": "update_monitoring_thresholds",
        "description": "Adjust corruption detection sensitivity",
        "impact": "Earlier detection of similar issues",
        "effort": "low"
      }
    ],
    "_links": {
      "self": "/api/v1/admin/audit/corruption?period=last_7d",
      "repair": "/api/v1/admin/audit/repair"
    }
  }
}
```

## Async Operation Tracking

### Get Check Status

**GET** `/api/v1/admin/audit/verify/{checkId}`

Track the status of an integrity check operation.

### Get Repair Status

**GET** `/api/v1/admin/audit/repair/{repairId}`

Track the status of a repair operation.

### Get Repair Progress

**GET** `/api/v1/admin/audit/repair/{repairId}/progress`

Get detailed progress information for a running repair operation.

## Error Responses

All endpoints follow the standard DCE error response format:

```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Admin privileges required",
    "details": "This endpoint requires admin, audit:admin, or system:admin permissions",
    "trace_id": "trace-123456",
    "help_url": "https://api.dependablecallexchange.com/docs/errors/unauthorized"
  },
  "meta": {
    "request_id": "req_123456",
    "timestamp": "2025-01-15T10:30:00Z",
    "version": "v1"
  }
}
```

## Rate Limiting

Admin endpoints have strict rate limiting:

- **Integrity checks**: 10 requests per minute per admin user
- **Repair operations**: 5 requests per minute per admin user  
- **Health/stats**: 60 requests per minute per admin user
- **Progress tracking**: 120 requests per minute per admin user

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1642262400
```

## Usage Examples

### Trigger Daily Integrity Check

```bash
curl -X POST "https://api.dce.com/api/v1/admin/audit/verify" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "check_type": "comprehensive",
    "priority": "normal",
    "async_mode": true,
    "metadata": {
      "initiated_by": "daily_cron",
      "reason": "scheduled_maintenance"
    }
  }'
```

### Monitor System Health

```bash
curl -X GET "https://api.dce.com/api/v1/admin/audit/health" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Repair Corrupted Chain (Dry Run)

```bash
curl -X POST "https://api.dce.com/api/v1/admin/audit/repair" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "start_sequence": 10000,
    "end_sequence": 11000,
    "repair_strategy": "rebuild",
    "dry_run": true,
    "backup_data": true
  }'
```

## Best Practices

1. **Monitor Regularly**: Use the health endpoint to monitor system status
2. **Async Operations**: Use async mode for comprehensive checks
3. **Dry Run First**: Always test repairs with dry_run before executing
4. **Backup Before Repair**: Always enable backup_data for repair operations
5. **Track Progress**: Monitor long-running operations using progress endpoints
6. **Review Corruption Reports**: Regularly review corruption reports for patterns
7. **Proper Authorization**: Ensure only authorized personnel have admin access
8. **Audit Admin Actions**: All admin actions are logged and audited

## Integration

The Audit Admin API integrates with:

- **Monitoring Systems**: Health and metrics endpoints for dashboards
- **Alerting**: Corruption detection triggers alerts
- **Backup Systems**: Automated backups before repair operations
- **Compliance**: Audit trails for all admin actions
- **Performance**: Real-time performance metrics and trends