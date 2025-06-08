# Contributing to Dependable Call Exchange Backend

Thank you for your interest in contributing to the Dependable Call Exchange Backend! We welcome contributions from the community and are grateful for any help you can provide.

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Process](#development-process)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Community](#community)

## 📜 Code of Conduct

This project adheres to a Code of Conduct that all contributors are expected to follow. Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md) to ensure a welcoming environment for all.

## 🚀 Getting Started

1. **Fork the Repository**
   ```bash
   # Fork via GitHub UI, then:
   git clone https://github.com/YOUR_USERNAME/dependable-call-exchange-backend.git
   cd dependable-call-exchange-backend
   git remote add upstream https://github.com/davidleathers113/dependable-call-exchange-backend.git
   ```

2. **Set Up Development Environment**
   ```bash
   # Install Go 1.24+
   # Install Docker and Docker Compose
   
   # Install development tools
   make install-tools
   
   # Set up pre-commit hooks
   git config core.hooksPath .githooks
   ```

3. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

## 🔄 Development Process

### Branch Naming Convention

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring
- `test/` - Test additions or fixes
- `chore/` - Maintenance tasks

### Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test additions or modifications
- `chore`: Maintenance tasks

**Examples:**
```bash
feat(routing): add weighted round-robin algorithm
fix(bidding): resolve race condition in auction processing
docs(api): update REST endpoint documentation
```

## 🔀 Pull Request Process

1. **Before Submitting**
   - Ensure all tests pass: `make test`
   - Run linters: `make lint`
   - Update documentation if needed
   - Add tests for new functionality
   - Rebase on latest main branch

2. **Pull Request Template**
   ```markdown
   ## Description
   Brief description of changes
   
   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update
   
   ## Testing
   - [ ] Unit tests pass
   - [ ] Integration tests pass
   - [ ] Manual testing completed
   
   ## Checklist
   - [ ] Code follows project style
   - [ ] Self-review completed
   - [ ] Documentation updated
   - [ ] No new warnings
   ```

3. **Review Process**
   - At least one maintainer approval required
   - All CI checks must pass
   - No merge conflicts
   - Discussion threads resolved

## 💻 Coding Standards

### Go Code Style

1. **Formatting**
   ```bash
   # Always format your code
   gofmt -w .
   
   # Use goimports for import management
   goimports -w .
   ```

2. **Naming Conventions**
   - Use descriptive names
   - Follow Go naming conventions
   - Exported functions/types start with capital letters
   - Unexported use camelCase

3. **Error Handling**
   ```go
   // Good
   if err != nil {
       return fmt.Errorf("failed to process call: %w", err)
   }
   
   // Bad
   if err != nil {
       return err
   }
   ```

4. **Comments**
   ```go
   // Package callrouting implements intelligent call routing algorithms
   // for real-time decision making in telephony systems.
   package callrouting
   
   // RouteCall determines the optimal destination for an incoming call
   // based on multiple factors including agent availability, skills, and cost.
   func RouteCall(ctx context.Context, call *Call) (*Route, error) {
       // Implementation
   }
   ```

### Project Structure

- Keep packages focused and cohesive
- Follow Domain-Driven Design principles
- Separate business logic from infrastructure
- Use interfaces for external dependencies

## 🧪 Testing Guidelines

### Test Organization

```go
// call_test.go
package call_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCall_Complete(t *testing.T) {
    // Arrange
    call := call.New(...)
    
    // Act
    err := call.Complete(duration, cost)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, call.StatusCompleted, call.Status)
}
```

### Test Coverage

- Aim for >80% code coverage
- Focus on critical business logic
- Test edge cases and error conditions
- Include integration tests for complex workflows

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/domain/call/...

# Run with race detection
make test-race

# Generate coverage report
make coverage
```

## 📚 Documentation

### Code Documentation

- Document all exported functions, types, and packages
- Use clear, concise language
- Include examples where helpful
- Keep documentation up-to-date with code changes

### API Documentation

- Update OpenAPI/Swagger specs for REST endpoints
- Document gRPC services in proto files
- Include request/response examples
- Document error responses

### Architecture Documentation

- Update ADRs (Architecture Decision Records) for significant changes
- Maintain diagrams in `docs/architecture/`
- Document system dependencies and integrations

## 👥 Community

### Getting Help

- 💬 [Discord Community](https://discord.gg/dce-community)
- 📧 [Mailing List](mailto:dev@dependablecallexchange.com)
- 🐛 [Issue Tracker](https://github.com/davidleathers113/dependable-call-exchange-backend/issues)

### Reporting Issues

When reporting issues, please include:

1. Go version (`go version`)
2. Operating system and version
3. Steps to reproduce
4. Expected vs actual behavior
5. Error messages or logs
6. Minimal code example if applicable

### Feature Requests

We welcome feature requests! Please:

1. Check existing issues first
2. Clearly describe the use case
3. Explain why it would benefit the project
4. Be open to discussion and feedback

## 🎉 Recognition

Contributors are recognized in our [CONTRIBUTORS.md](CONTRIBUTORS.md) file. Thank you for helping make this project better!

## 📄 License

By contributing, you agree that your contributions will be licensed under the MIT License.