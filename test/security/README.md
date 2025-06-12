# Security Testing Suite

This directory contains comprehensive security tests for the Dependable Call Exchange Backend.

## Overview

The security test suite validates the following security aspects:

1. **Authentication & Authorization**
   - JWT token validation
   - Role-based access control (RBAC)
   - Token refresh security
   - Session management

2. **Input Validation**
   - SQL injection prevention
   - XSS (Cross-Site Scripting) prevention
   - NoSQL injection prevention
   - Path traversal prevention
   - Command injection prevention
   - XXE (XML External Entity) prevention

3. **Rate Limiting**
   - API rate limiting
   - Per-endpoint rate limits
   - Distributed rate limiting
   - Rate limit recovery

4. **Data Protection**
   - Sensitive data masking (credit cards, SSN, API keys)
   - PII (Personally Identifiable Information) access control
   - Security headers validation
   - CORS configuration

## Running Security Tests

### Run All Security Tests
```bash
make test-security
```

### Run Specific Security Test Categories

```bash
# Authentication and authorization tests only
make test-security-auth

# Input validation tests only
make test-security-input

# Rate limiting tests only
make test-security-rate

# Data protection tests only
make test-security-data

# Complete security test suite with detailed reporting
make test-security-suite
```

### Run Individual Test Files

```bash
# Run with security build tag
go test -tags=security -v ./test/security/auth_security_complete_test.go
go test -tags=security -v ./test/security/input_validation_test.go
go test -tags=security -v ./test/security/rate_limiting_data_protection_test.go
```

## Test Structure

### Helper Files
- `security_test_helpers.go` - Common helper functions for security tests
- `token_helpers_test.go` - JWT token generation utilities

### Test Files
- `auth_security_complete_test.go` - Authentication and authorization tests
- `input_validation_test.go` - Input validation and injection prevention tests
- `rate_limiting_data_protection_test.go` - Rate limiting and data protection tests
- `security_suite_test.go` - Complete security test suite runner

## Security Test Requirements

### Environment Setup
The security tests require:
- PostgreSQL database (provided by Testcontainers)
- Redis for rate limiting (provided by Testcontainers)
- JWT secret configured in test environment

### Test User Credentials
Test users are created with the following pattern:
- Email: `{role}@test.com` (e.g., `buyer@test.com`, `seller@test.com`, `admin@test.com`)
- Password: `TestPass123!`
- Roles: `buyer`, `seller`, `admin`

## Security Vulnerabilities Tested

### Authentication Vulnerabilities
- Missing or invalid tokens
- Expired tokens
- Invalid token signatures
- Invalid token issuers
- Missing required claims
- Token reuse attacks
- Session hijacking attempts

### Injection Vulnerabilities
- SQL injection in various input fields
- XSS in user-generated content
- NoSQL injection attempts
- Path traversal in file operations
- Command injection in system calls
- XXE attacks in XML parsing

### Access Control Issues
- Unauthorized access to restricted endpoints
- Role-based permission violations
- PII data exposure
- Debug endpoint exposure
- Configuration file access

### Rate Limiting & DoS
- Excessive API requests
- Authentication brute force
- Distributed attack patterns
- Resource exhaustion

## Integration with CI/CD

Add to your CI/CD pipeline:

```yaml
# Example GitHub Actions workflow
- name: Run Security Tests
  run: |
    make test-security
    make security-scan
    make security-deps-ci
```

## Security Best Practices Enforced

1. **Never expose sensitive data** - All PII and payment data is masked
2. **Validate all inputs** - Every user input is sanitized and validated
3. **Use proper authentication** - JWT tokens with expiration and refresh
4. **Implement rate limiting** - Prevent abuse and DoS attacks
5. **Set security headers** - HSTS, CSP, X-Frame-Options, etc.
6. **Handle errors securely** - No internal details in error messages
7. **Audit all actions** - Security events are logged for monitoring

## Troubleshooting

### Common Issues

1. **Tests failing due to rate limiting**
   - The tests include appropriate delays
   - If running multiple times, wait for rate limit reset

2. **Authentication tests failing**
   - Ensure JWT secret is properly configured
   - Check that test database is clean

3. **Container startup issues**
   - Ensure Docker is running
   - Check that required ports are available
   - Run `make docker-clean` to cleanup old containers

### Debug Mode

To run tests with verbose output:
```bash
go test -tags=security -v -run TestSecurity_Authentication ./test/security/
```

## Contributing

When adding new security tests:

1. Follow the existing pattern for test organization
2. Use the helper functions for common operations
3. Add appropriate test tags (`//go:build security`)
4. Document any new security vulnerabilities tested
5. Update this README with new test categories

## Security Reporting

If you discover a security vulnerability:
1. Do NOT create a public issue
2. Email security@dependablecallexchange.com
3. Include steps to reproduce
4. Wait for confirmation before disclosure
