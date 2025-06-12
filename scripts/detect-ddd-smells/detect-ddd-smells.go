package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type DomainAnalysis struct {
	AnemicModels []AnemicModel
	FatServices  []FatService
	LeakyDomains []LeakyDomain
}

type AnemicModel struct {
	File        string
	StructName  string
	FieldCount  int
	MethodCount int
}

type FatService struct {
	File         string
	ServiceName  string
	MethodCount  int
	Dependencies int
}

type LeakyDomain struct {
	File  string
	Issue string
}

func main() {
	analysis := &DomainAnalysis{}

	// Start from current directory
	rootDir := "."
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, "vendor") || !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.Contains(path, "/domain/") {
			analyzeDomainFile(path, analysis)
		} else if strings.Contains(path, "/service/") {
			analyzeServiceFile(path, analysis)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output results
	fmt.Println("=== Anemic Domain Models ===")
	for _, model := range analysis.AnemicModels {
		if model.FieldCount > 3 && model.MethodCount <= 2 {
			fmt.Printf("%s: %s (Fields: %d, Methods: %d)\n",
				model.File, model.StructName, model.FieldCount, model.MethodCount)
		}
	}

	fmt.Println("\n=== Fat Services (>5 dependencies) ===")
	for _, service := range analysis.FatServices {
		if service.Dependencies > 5 {
			fmt.Printf("%s: %s (Dependencies: %d, Methods: %d)\n",
				service.File, service.ServiceName, service.Dependencies, service.MethodCount)
		}
	}

	fmt.Println("\n=== Domain Leakage Issues ===")
	for _, leak := range analysis.LeakyDomains {
		fmt.Printf("%s: %s\n", leak.File, leak.Issue)
	}
}

func analyzeDomainFile(path string, analysis *DomainAnalysis) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if structType, ok := x.Type.(*ast.StructType); ok {
				fieldCount := len(structType.Fields.List)
				methodCount := countMethods(node, x.Name.Name)

				analysis.AnemicModels = append(analysis.AnemicModels, AnemicModel{
					File:        path,
					StructName:  x.Name.Name,
					FieldCount:  fieldCount,
					MethodCount: methodCount,
				})
			}
		case *ast.ImportSpec:
			// Check for infrastructure imports in domain
			if x.Path != nil && strings.Contains(x.Path.Value, "database/sql") {
				analysis.LeakyDomains = append(analysis.LeakyDomains, LeakyDomain{
					File:  path,
					Issue: "Domain imports infrastructure (database/sql)",
				})
			}
		}
		return true
	})
}

func analyzeServiceFile(path string, analysis *DomainAnalysis) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// Count service struct and its dependencies
	ast.Inspect(node, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						if strings.HasSuffix(typeSpec.Name.Name, "Service") {
							deps := countStructFields(structType)
							methods := countMethods(node, typeSpec.Name.Name)

							analysis.FatServices = append(analysis.FatServices, FatService{
								File:         path,
								ServiceName:  typeSpec.Name.Name,
								MethodCount:  methods,
								Dependencies: deps,
							})
						}
					}
				}
			}
		}
		return true
	})
}

func countMethods(file *ast.File, structName string) int {
	count := 0
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				if starExpr, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
					if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == structName {
						count++
					}
				}
			}
		}
	}
	return count
}

func countStructFields(structType *ast.StructType) int {
	return len(structType.Fields.List)
}
