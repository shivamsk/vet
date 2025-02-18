package main

import (
	"time"

	"github.com/safedep/dry/utils"
	"github.com/safedep/vet/pkg/analyzer"
	"github.com/safedep/vet/pkg/readers"
	"github.com/safedep/vet/pkg/reporter"
	"github.com/safedep/vet/pkg/scanner"
	"github.com/spf13/cobra"
)

var (
	queryFilterExpression    string
	queryFilterSuiteFile     string
	queryFilterFailOnMatch   bool
	queryLoadDirectory       string
	queryEnableConsoleReport bool
	queryEnableSummaryReport bool
	queryMarkdownReportPath  string
	queryExceptionsFile      string
	queryExceptionsTill      string
	queryExceptionsFilter    string

	queryDefaultExceptionExpiry = time.Now().Add(90 * 24 * time.Hour)
)

func newQueryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query JSON dump and run filters or render reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			startQuery()
			return nil
		},
	}

	cmd.Flags().StringVarP(&queryLoadDirectory, "from", "F", "",
		"The directory to load JSON dump files")
	cmd.Flags().StringVarP(&queryFilterExpression, "filter", "", "",
		"Filter and print packages using CEL")
	cmd.Flags().StringVarP(&queryFilterSuiteFile, "filter-suite", "", "",
		"Filter packages using CEL Filter Suite from file")
	cmd.Flags().BoolVarP(&queryFilterFailOnMatch, "filter-fail", "", false,
		"Fail the command if filter matches any package (for security gate)")
	cmd.Flags().StringVarP(&queryExceptionsFile, "exceptions-generate", "", "",
		"Generate exception records to file (YAML)")
	cmd.Flags().StringVarP(&queryExceptionsTill, "exceptions-till", "",
		queryDefaultExceptionExpiry.Format("2006-01-02"),
		"Generated exceptions are valid till")
	cmd.Flags().StringVarP(&queryExceptionsFilter, "exceptions-filter", "", "",
		"Generate exception records for packages matching filter")
	cmd.Flags().BoolVarP(&queryEnableConsoleReport, "report-console", "", false,
		"Minimal summary of package manifest")
	cmd.Flags().BoolVarP(&queryEnableSummaryReport, "report-summary", "", false,
		"Show an actionable summary based on scan data")
	cmd.Flags().StringVarP(&queryMarkdownReportPath, "report-markdown", "", "",
		"Generate markdown report to file")
	return cmd
}

func startQuery() {
	failOnError("query", internalStartQuery())
}

func internalStartQuery() error {
	readerList := []readers.PackageManifestReader{}
	analyzers := []analyzer.Analyzer{}
	reporters := []reporter.Reporter{}
	enrichers := []scanner.PackageMetaEnricher{}

	reader, err := readers.NewJsonDumpReader(queryLoadDirectory)
	if err != nil {
		return err
	}

	readerList = append(readerList, reader)

	if !utils.IsEmptyString(queryFilterExpression) {
		task, err := analyzer.NewCelFilterAnalyzer(queryFilterExpression,
			queryFilterFailOnMatch)
		if err != nil {
			return err
		}

		analyzers = append(analyzers, task)
	}

	if !utils.IsEmptyString(queryFilterSuiteFile) {
		task, err := analyzer.NewCelFilterSuiteAnalyzer(queryFilterSuiteFile,
			queryFilterFailOnMatch)
		if err != nil {
			return err
		}

		analyzers = append(analyzers, task)
	}

	if !utils.IsEmptyString(queryExceptionsFile) {
		task, err := analyzer.NewExceptionsGenerator(analyzer.ExceptionsGeneratorConfig{
			Path:      queryExceptionsFile,
			ExpiresOn: queryExceptionsTill,
			Filter:    queryExceptionsFilter,
		})

		if err != nil {
			return err
		}

		analyzers = append(analyzers, task)
	}

	if queryEnableConsoleReport {
		rp, err := reporter.NewConsoleReporter()
		if err != nil {
			return err
		}

		reporters = append(reporters, rp)
	}

	if queryEnableSummaryReport {
		rp, err := reporter.NewSummaryReporter()
		if err != nil {
			return err
		}

		reporters = append(reporters, rp)
	}

	if !utils.IsEmptyString(queryMarkdownReportPath) {
		rp, err := reporter.NewMarkdownReportGenerator(reporter.MarkdownReportingConfig{
			Path: queryMarkdownReportPath,
		})

		if err != nil {
			return err
		}

		reporters = append(reporters, rp)
	}

	pmScanner := scanner.NewPackageManifestScanner(scanner.Config{
		TransitiveAnalysis: false,
	}, readerList, enrichers, analyzers, reporters)

	redirectLogToFile(logFile)
	return pmScanner.Start()
}
