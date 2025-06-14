# DCE Work Verification & Gap Analysis

Check your work assuming you've missed several crucial details.

## Command Purpose

This command triggers a comprehensive self-review process to identify overlooked requirements, edge cases, security vulnerabilities, performance issues, and implementation gaps that may have been missed during initial development.

## Critical Analysis Areas

### 🔍 **Implementation Completeness**
- Verify all requirements from specifications are implemented
- Check for missing error handling and edge cases
- Validate business logic against domain rules
- Ensure proper validation in domain constructors

### 🏗️ **Architecture & Design**
- Confirm DDD patterns are correctly followed
- Validate service layer orchestration (no business logic)
- Check dependency injection and interface usage
- Verify proper separation of concerns

### 🚀 **Performance & Quality**
- Validate latency targets (< 1ms routing, < 50ms API p99)
- Check for potential memory leaks or goroutine issues
- Verify database query optimization
- Ensure proper caching strategies

### 🔒 **Security & Compliance**
- Review authentication/authorization implementation
- Check for input validation gaps
- Verify TCPA/GDPR compliance measures
- Validate rate limiting and security headers

### 🧪 **Testing & Documentation**
- Assess test coverage (target: 80%+ overall, 90%+ domain)
- Check for missing property-based or synctest tests
- Verify integration tests exist for critical paths
- Validate API documentation completeness

## When to Use

- After completing major feature implementation
- Before creating pull requests
- When debugging complex system issues
- During pre-deployment quality checks
- When system behavior seems inconsistent

## Execution Method

1. **Compilation Check**: Run `go build -gcflags="-e" ./...` first
2. **Critical Path Analysis**: Focus on call routing, bidding, compliance
3. **Performance Validation**: Verify sub-millisecond requirements
4. **Security Review**: Check for vulnerabilities and compliance gaps
5. **Documentation Audit**: Ensure all changes are properly documented

## Expected Deliverables

- Detailed findings report with severity levels
- Specific remediation steps for each issue
- Performance benchmark comparisons
- Security vulnerability assessment
- Test coverage gap analysis
- Documentation completeness review