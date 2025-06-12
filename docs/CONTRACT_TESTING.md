# Contract Testing Documentation

This document describes the contract testing infrastructure and processes for the Dependable Call Exchange Backend API.

## Overview

Contract testing ensures that the API implementation strictly adheres to the OpenAPI 3.0 specification, providing confidence that:

- All endpoints behave as documented
- Request/response schemas are validated
- Breaking changes are detected early
- API contracts remain stable across versions

## Architecture

### Components

1. **Contract Validator** (`contract_validator.go`)
   - Core validation engine using `kin-openapi`
   - Request/response schema validation
   - Path parameter and query validation
   - Content type and header validation

2. **Contract Middleware** (`contract_middleware.go`)
   - Production-ready middleware for runtime validation
   - Configurable validation behavior (strict/lenient/report-only)
   - Performance monitoring and metrics collection
   - Violation reporting and logging

3. **Test Suites**
   - **Unit Tests**: Schema and endpoint-level validation
   - **Integration Tests**: End-to-end contract compliance
   - **Performance Tests**: Validation overhead benchmarks
   - **Compatibility Tests**: Backward compatibility verification

## Getting Started

### Prerequisites

- Go 1.24+
- OpenAPI specification at `api/openapi.yaml`
- Java 11+ (for OpenAPI validation tools)

### Installation

Install contract testing dependencies:

```bash
go mod download
make install-tools
```

### Running Contract Tests

#### Quick Tests (Pre-commit)
```bash
# Quick validation for development
make test-contract
```

#### Full Test Suite
```bash
# Complete contract validation
make test-contract-full
```

#### Individual Test Categories
```bash
# Unit tests only
go test -tags=contract ./internal/api/rest/ -run TestContract

# Integration tests only  
go test -tags=contract ./test/contract/ -run TestAPIContractCompliance

# Performance benchmarks
go test -tags=contract -bench=BenchmarkContract ./internal/api/rest/
```

## CI/CD Integration

### GitHub Actions Workflows

#### 1. Main CI Pipeline (`.github/workflows/ci.yml`)
- Runs contract tests on every PR and push
- Validates OpenAPI specification
- Includes contract test results in coverage reports

#### 2. Dedicated Contract Testing (`.github/workflows/contract-tests.yml`)
- Comprehensive contract testing suite
- Scheduled daily runs
- Matrix testing across multiple validation levels
- Detailed reporting and PR comments

### Pre-commit Hooks

Contract testing is integrated into pre-commit hooks:

```bash
# Install pre-commit hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

## Test Categories

### 1. Schema Validation Tests
- Request body validation against OpenAPI schemas
- Response structure verification
- Field type and format validation
- Required field enforcement

### 2. Endpoint Contract Tests
- HTTP method and path validation
- Status code verification
- Header validation (Content-Type, Authorization, etc.)
- Pagination structure validation

### 3. Error Response Tests
- Error response schema validation
- Consistent error format verification
- Error code and message validation

### 4. Performance Tests
- Validation overhead measurement (target: < 1ms)
- Memory usage optimization
- Concurrent validation testing

### 5. Compatibility Tests
- Backward compatibility verification
- API versioning consistency
- Deprecated field handling

## Configuration

### Validation Levels

1. **Strict Mode** (Default)
   - Fails on any validation error
   - Enforces complete schema compliance
   - Recommended for CI/CD pipelines

2. **Lenient Mode**
   - Allows minor schema deviations
   - Warns on validation issues
   - Useful for development environments

3. **Report-Only Mode**
   - Logs validation issues without failing
   - Collects violation statistics
   - Suitable for monitoring production traffic

### Environment Variables

```bash
# Contract validation level
CONTRACT_VALIDATION_LEVEL=strict|lenient|report-only

# Test environment
TEST_ENVIRONMENT=ci|staging|production

# Enable validation middleware in production
DCE_CONTRACT_VALIDATION_ENABLED=true

# Validation failure behavior  
DCE_CONTRACT_FAIL_ON_ERROR=true
```

## Middleware Integration

### Development Setup

```go
// Add contract validation middleware
config := rest.DefaultContractValidationConfig()
config.FailOnValidationError = true

middleware, err := rest.NewContractValidationMiddleware(
    "api/openapi.yaml", 
    config, 
    logger,
)
if err != nil {
    log.Fatal(err)
}

router.Use(middleware.Middleware())
```

### Production Configuration

```go
// Production-ready configuration
config := rest.ContractValidationConfig{
    ValidateRequests:      true,
    ValidateResponses:     true,
    FailOnValidationError: false, // Log but don't fail
    ReportViolations:      true,
    EnableMetrics:         true,
    MaxViolationLogs:      1000,
}
```

## Monitoring and Reporting

### Metrics

Contract validation exposes the following metrics:

- `contract_validations_total` - Total validation attempts
- `contract_validation_errors_total` - Validation failures
- `contract_validation_duration_seconds` - Validation latency
- `contract_violations_by_endpoint` - Violations per endpoint

### Violation Reports

The system generates detailed violation reports including:

- Request/response details
- Validation error descriptions
- Endpoint and method information
- Timestamp and user context
- Suggested fixes

### Dashboards

Grafana dashboards are available for:

- Contract validation performance
- Violation trends and hotspots
- API compliance overview
- Error rate analysis

## Best Practices

### 1. Development Workflow

1. **Update OpenAPI First**: Always update the specification before implementation
2. **Run Tests Early**: Use pre-commit hooks to catch issues early
3. **Test Incrementally**: Run quick tests during development
4. **Validate Before PR**: Ensure full test suite passes before creating PRs

### 2. Specification Management

1. **Single Source of Truth**: Keep OpenAPI spec synchronized with implementation
2. **Version Carefully**: Use semantic versioning for API changes
3. **Document Everything**: Include examples and descriptions
4. **Review Regularly**: Conduct regular spec reviews

### 3. Performance Optimization

1. **Cache Validators**: Reuse validator instances
2. **Optimize Schemas**: Keep schemas lean and focused
3. **Monitor Overhead**: Track validation performance
4. **Profile Regularly**: Use benchmarks to identify bottlenecks

### 4. Error Handling

1. **Meaningful Messages**: Provide clear validation error messages
2. **Context Information**: Include request details in error reports
3. **Graceful Degradation**: Handle validation failures gracefully
4. **User Experience**: Don't expose internal validation details to end users

## Troubleshooting

### Common Issues

#### 1. OpenAPI Spec Not Found
```bash
Error: OpenAPI specification not found at api/openapi.yaml
```
**Solution**: Ensure the OpenAPI specification exists at the expected path.

#### 2. Validation Failures
```bash
Contract validation failed: field 'email' is required
```
**Solution**: Check request/response against OpenAPI schema and fix discrepancies.

#### 3. Performance Issues
```bash
Contract validation taking > 1ms per request
```
**Solution**: Profile validation code and optimize schema complexity.

#### 4. Build Tag Issues
```bash
No tests found with contract tag
```
**Solution**: Ensure test files have `//go:build contract` build tag.

### Debug Mode

Enable debug logging for detailed validation information:

```bash
export DCE_LOG_LEVEL=debug
go test -tags=contract -v ./internal/api/rest/
```

### Validation Reports

Generate detailed validation reports:

```bash
# Generate contract compliance report
make test-contract-full > contract-report.txt

# Run with coverage
go test -tags=contract -cover ./internal/api/rest/ ./test/contract/
```

## Roadmap

### Short Term
- [ ] Add Spectral linting integration
- [ ] Implement contract drift detection
- [ ] Enhanced error reporting
- [ ] Performance optimizations

### Medium Term
- [ ] Consumer-driven contract testing
- [ ] Multi-version API testing
- [ ] Automated spec generation
- [ ] Integration with API gateway

### Long Term
- [ ] ML-based contract anomaly detection
- [ ] Real-time contract monitoring
- [ ] Advanced compatibility testing
- [ ] Cross-service contract verification

## Contributing

1. **Add New Tests**: Follow existing test patterns
2. **Update Documentation**: Keep this document current
3. **Performance Focus**: Ensure tests are fast and reliable
4. **Error Handling**: Provide meaningful error messages

## Resources

- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [kin-openapi Library](https://github.com/getkin/kin-openapi)
- [Contract Testing Best Practices](https://martinfowler.com/articles/contract-testing.html)
- [API Design Guidelines](https://github.com/microsoft/api-guidelines)

## Support

For questions or issues with contract testing:

1. Check this documentation
2. Review existing test examples
3. Check GitHub issues
4. Contact the development team

---

*This documentation is maintained by the development team and updated regularly.*