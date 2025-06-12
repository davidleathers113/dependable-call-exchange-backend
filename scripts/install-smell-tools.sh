#!/bin/bash

# Install code smell testing tools

echo "=== Installing Code Smell Testing Tools ==="

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install golangci-lint
if ! command_exists golangci-lint; then
    echo "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
else
    echo "✓ golangci-lint already installed"
fi

# Install other Go tools
echo "Installing Go analysis tools..."

go install honnef.co/go/tools/cmd/staticcheck@latest || echo "Failed to install staticcheck"
go install github.com/securego/gosec/v2/cmd/gosec@latest || echo "Failed to install gosec"
go install github.com/go-critic/go-critic/cmd/gocritic@latest || echo "Failed to install go-critic"
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest || echo "Failed to install gocyclo"
go install github.com/uudashr/gocognit/cmd/gocognit@latest || echo "Failed to install gocognit"
go install github.com/gordonklaus/ineffassign@latest || echo "Failed to install ineffassign"
go install github.com/tetafro/godot/cmd/godot@latest || echo "Failed to install godot"
go install golang.org/x/tools/cmd/goimports@latest || echo "Failed to install goimports"

echo ""
echo "=== Installation Summary ==="
echo "Checking installed tools:"

tools=(
    "golangci-lint"
    "staticcheck"
    "gosec"
    "gocritic"
    "gocyclo"
    "gocognit"
    "ineffassign"
    "godot"
    "goimports"
)

for tool in "${tools[@]}"; do
    if command_exists "$tool"; then
        echo "✓ $tool"
    else
        echo "✗ $tool (not found)"
    fi
done

# Check for jq (needed for CI script)
if ! command_exists jq; then
    echo ""
    echo "⚠️  Warning: jq is not installed. This is required for CI scripts."
    echo "  Install with:"
    echo "    macOS: brew install jq"
    echo "    Linux: sudo apt-get install jq"
fi

echo ""
echo "Installation complete!"
