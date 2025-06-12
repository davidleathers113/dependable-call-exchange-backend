package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDomainCrossDependencies ensures domains don't directly depend on each other
func TestNoDomainCrossDependencies(t *testing.T) {
	domains := []string{"account", "bid", "call", "compliance", "financial"}

	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			domainPath := filepath.Join("../../internal/domain", domain)
			files, err := filepath.Glob(filepath.Join(domainPath, "*.go"))
			if err != nil {
				t.Skip("Domain not found")
				return
			}

			for _, file := range files {
				imports := getFileImports(file)
				for _, imp := range imports {
					for _, otherDomain := range domains {
						if domain != otherDomain && strings.Contains(imp, "domain/"+otherDomain) {
							t.Errorf("Domain %s imports %s (violation in %s: %s)",
								domain, otherDomain, file, imp)
						}
					}
				}
			}
		})
	}
}

// isOrchestratorService checks if a service is an orchestrator that coordinates multiple subsystems
func isOrchestratorService(serviceName string) bool {
	return strings.HasSuffix(serviceName, "OrchestrationService") ||
		strings.HasSuffix(serviceName, "CoordinatorService") ||
		serviceName == "coordinatorService" // specific exception for bidding coordinator
}

// TestServiceMaxDependencies ensures services don't have more than 5 dependencies
func TestServiceMaxDependencies(t *testing.T) {
	const maxDeps = 5

	services := []string{
		"analytics",
		"bidding",
		"buyer_routing",
		"callrouting",
		"fraud",
		"marketplace",
		"seller_distribution",
		"telephony",
	}

	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			servicePath := filepath.Join("../../internal/service", service)
			files, err := filepath.Glob(filepath.Join(servicePath, "*.go"))
			if err != nil || len(files) == 0 {
				t.Skip("Service not found")
				return
			}

			for _, file := range files {
				checkServiceDependenciesInFile(t, file)
			}
		})
	}
}

// TestDomainNotDependOnInfrastructure ensures domain layer doesn't depend on infrastructure
func TestDomainNotDependOnInfrastructure(t *testing.T) {
	forbiddenImports := []string{
		"database/sql",
		"github.com/lib/pq",
		"github.com/jackc/pgx",
		"github.com/go-redis/redis",
		"net/http",
		"google.golang.org/grpc",
		"github.com/gorilla/mux",
		"github.com/gin-gonic/gin",
	}

	domainFiles, err := filepath.Glob("../../internal/domain/**/*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range domainFiles {
		if strings.Contains(file, "_test.go") {
			continue
		}

		imports := getFileImports(file)
		for _, imp := range imports {
			for _, forbidden := range forbiddenImports {
				if strings.Contains(imp, forbidden) {
					t.Errorf("Domain file %s imports infrastructure: %s", file, imp)
				}
			}
		}
	}
}

// TestValueObjectsAreImmutable ensures value objects don't have setters
func TestValueObjectsAreImmutable(t *testing.T) {
	valueFiles, err := filepath.Glob("../../internal/domain/values/*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range valueFiles {
		if strings.Contains(file, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", file, err)
			continue
		}

		// Check for setter methods
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if fn.Recv != nil && strings.HasPrefix(fn.Name.Name, "Set") {
					t.Errorf("Value object in %s has setter method: %s", file, fn.Name.Name)
				}
			}
			return true
		})
	}
}

// Helper functions

func checkServiceDependenciesInFile(t *testing.T, filename string) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read %s: %v", filename, err)
		return
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		t.Errorf("Failed to parse %s: %v", filename, err)
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						serviceName := typeSpec.Name.Name
						if strings.HasSuffix(serviceName, "Service") {
							deps := 0
							for _, field := range structType.Fields.List {
								// Count fields that look like dependencies
								if field.Type != nil {
									typeStr := getTypeString(field.Type)
									if strings.Contains(typeStr, "Repository") ||
										strings.Contains(typeStr, "Service") ||
										strings.Contains(typeStr, "Client") ||
										strings.Contains(typeStr, "Cache") ||
										strings.Contains(typeStr, "Bus") ||
										strings.Contains(typeStr, "MetricsCollector") ||
										strings.Contains(typeStr, "NotificationService") ||
										strings.Contains(typeStr, "Config") {
										deps++
									}
								}
							}

							// Check dependency count - orchestrators are allowed up to 8 dependencies
							// per ADR-001, while regular services are limited to 5
							maxDeps := 5
							if isOrchestratorService(serviceName) {
								maxDeps = 8 // Orchestrators can have more dependencies
							}

							if deps > maxDeps {
								t.Errorf("Service %s has %d dependencies (max allowed: %d) in %s",
									serviceName, deps, maxDeps, filename)
							}
						}
					}
				}
			}
		}
		return true
	})
}

func getFileImports(filename string) []string {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ImportsOnly)
	if err != nil {
		return nil
	}

	var imports []string
	for _, imp := range node.Imports {
		if imp.Path != nil {
			imports = append(imports, strings.Trim(imp.Path.Value, `"`))
		}
	}
	return imports
}

func countServiceDependencies(filename string) int {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return 0
	}

	maxDeps := 0
	ast.Inspect(node, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						if strings.HasSuffix(typeSpec.Name.Name, "Service") {
							deps := 0
							for _, field := range structType.Fields.List {
								// Count fields that look like dependencies
								if field.Type != nil {
									typeStr := getTypeString(field.Type)
									if strings.Contains(typeStr, "Repository") ||
										strings.Contains(typeStr, "Service") ||
										strings.Contains(typeStr, "Client") ||
										strings.Contains(typeStr, "Cache") ||
										strings.Contains(typeStr, "Bus") {
										deps++
									}
								}
							}
							if deps > maxDeps {
								maxDeps = deps
							}
						}
					}
				}
			}
		}
		return true
	})
	return maxDeps
}

func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return getTypeString(t.X)
	case *ast.SelectorExpr:
		return getTypeString(t.X) + "." + t.Sel.Name
	default:
		return ""
	}
}
