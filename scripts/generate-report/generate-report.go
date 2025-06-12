package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CodeSmellReport struct {
	GeneratedAt     time.Time
	ProjectPath     string
	Summary         Summary
	GolangciIssues  []GolangciIssue
	Recommendations []string
}

type Summary struct {
	TotalFiles     int
	TotalIssues    int
	CriticalIssues int
}

type GolangciIssue struct {
	File       string
	Line       int
	Column     int
	Severity   string
	Message    string
	LinterName string
}

type GolangciResult struct {
	Issues []struct {
		FromLinter string `json:"FromLinter"`
		Text       string `json:"Text"`
		Severity   string `json:"Severity"`
		Pos        struct {
			Filename string `json:"Filename"`
			Line     int    `json:"Line"`
			Column   int    `json:"Column"`
		} `json:"Pos"`
	} `json:"Issues"`
}

func main() {
	report := generateReport()
	generateHTMLReport(report)
}

func generateReport() CodeSmellReport {
	// Find the latest golangci-lint report
	reports, _ := filepath.Glob("analysis/reports/golangci-*.json")

	report := CodeSmellReport{
		GeneratedAt: time.Now(),
		ProjectPath: "/Users/davidleathers/projects/DependableCallExchangeBackEnd",
		Summary: Summary{
			TotalFiles: countGoFiles(),
		},
		Recommendations: []string{
			"Review and fix critical security issues first",
			"Address high cyclomatic complexity functions",
			"Check domain boundary violations",
			"Update services with >5 dependencies",
			"Review TODO/FIXME comments for technical debt",
		},
	}

	if len(reports) > 0 {
		// Get the latest report
		latestReport := reports[len(reports)-1]
		data, err := ioutil.ReadFile(latestReport)
		if err == nil {
			var result GolangciResult
			if err := json.Unmarshal(data, &result); err == nil {
				for _, issue := range result.Issues {
					severity := issue.Severity
					if severity == "" {
						severity = "warning"
					}
					if severity == "error" {
						report.Summary.CriticalIssues++
					}

					report.GolangciIssues = append(report.GolangciIssues, GolangciIssue{
						File:       issue.Pos.Filename,
						Line:       issue.Pos.Line,
						Column:     issue.Pos.Column,
						Severity:   severity,
						Message:    issue.Text,
						LinterName: issue.FromLinter,
					})
				}
				report.Summary.TotalIssues = len(report.GolangciIssues)
			}
		}
	}

	return report
}

func countGoFiles() int {
	count := 0
	rootDir := "."
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.Contains(path, "vendor/") {
			count++
		}
		return nil
	})
	return count
}

func generateHTMLReport(report CodeSmellReport) {
	const reportTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Code Smell Analysis Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .critical { color: red; }
        .warning { color: orange; }
        .info { color: blue; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        tr:nth-child(even) { background-color: #f9f9f9; }
        .summary { background-color: #e8f4f8; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <h1>Code Smell Analysis Report</h1>
    <p>Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
    <p>Project: {{.ProjectPath}}</p>
    
    <div class="summary">
        <h2>Summary</h2>
        <ul>
            <li>Total Go Files: {{.Summary.TotalFiles}}</li>
            <li>Total Issues: {{.Summary.TotalIssues}}</li>
            <li class="critical">Critical Issues: {{.Summary.CriticalIssues}}</li>
        </ul>
    </div>
    
    <h2>Issues Found</h2>
    {{if .GolangciIssues}}
    <table>
        <tr>
            <th>File</th>
            <th>Line</th>
            <th>Linter</th>
            <th>Issue</th>
            <th>Severity</th>
        </tr>
        {{range .GolangciIssues}}
        <tr>
            <td>{{.File}}</td>
            <td>{{.Line}}:{{.Column}}</td>
            <td>{{.LinterName}}</td>
            <td>{{.Message}}</td>
            <td class="{{.Severity}}">{{.Severity}}</td>
        </tr>
        {{end}}
    </table>
    {{else}}
    <p>No issues found or no report available.</p>
    {{end}}
    
    <h2>Recommendations</h2>
    <ol>
        {{range .Recommendations}}
        <li>{{.}}</li>
        {{end}}
    </ol>
</body>
</html>
`

	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
		return
	}

	// Create output directory
	os.MkdirAll("analysis/reports", 0755)

	outputFile, err := os.Create("analysis/reports/code-smell-report.html")
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, report); err != nil {
		fmt.Printf("Error executing template: %v\n", err)
		return
	}

	fmt.Println("Report generated successfully at: analysis/reports/code-smell-report.html")
}
