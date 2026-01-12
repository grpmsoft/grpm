package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpmsoft/grpm/internal/analyze"
)

// runAnalyze handles the 'analyze' command - coverage analysis of Portage repository.
//
// This command analyzes a Portage repository and reports:
//   - Total packages in repository
//   - Packages GRPM can build (have all required eclasses/helpers)
//   - Packages blocked by missing features
//   - Detailed breakdown by category and eclass
//
// Flags:
//   - --repo/-r: Path to Portage repository (default: /var/db/repos/gentoo)
//   - --output/-o: Output format: text, json, markdown (default: text)
//   - --category/-c: Analyze specific category only (e.g., app-misc)
//   - --verbose/-v: Show details for each package
//
// Usage:
//
//	grpm analyze                                    # Analyze default repo
//	grpm analyze --repo /path/to/gentoo            # Custom repo path
//	grpm analyze --category app-misc               # Single category
//	grpm analyze --output json > coverage.json     # JSON output
//	grpm analyze --output markdown > COVERAGE.md   # Markdown report
func (a *App) runAnalyze(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	repoPath := fs.String("repo", "/var/db/repos/gentoo", "Path to Portage repository")
	fs.StringVar(repoPath, "r", "/var/db/repos/gentoo", "Alias for --repo")
	outputFormat := fs.String("output", "text", "Output format: text, json, markdown")
	fs.StringVar(outputFormat, "o", "text", "Alias for --output")
	category := fs.String("category", "", "Analyze specific category only")
	fs.StringVar(category, "c", "", "Alias for --category")
	verbose := fs.Bool("verbose", a.verbose, "Show details for each package")
	fs.BoolVar(verbose, "v", a.verbose, "Alias for --verbose")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse output format
	format, err := analyze.ParseOutputFormat(*outputFormat)
	if err != nil {
		return err
	}

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create analyzer with options
	opts := []analyze.AnalyzerOption{
		analyze.WithVerbose(*verbose),
	}
	if *category != "" {
		opts = append(opts, analyze.WithCategory(*category))
	}

	analyzer, err := analyze.NewAnalyzer(*repoPath, opts...)
	if err != nil {
		return fmt.Errorf("failed to create analyzer: %w", err)
	}

	// Run analysis
	log.Printf("Analyzing repository: %s", *repoPath)
	if *category != "" {
		log.Printf("Filtering to category: %s", *category)
	}

	result, err := analyzer.Analyze(ctx)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Generate report
	reporter := analyze.NewReporter(format, *verbose)
	if err := reporter.Report(os.Stdout, result); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Log summary
	log.Printf("Analysis complete: %s", analyze.FormatSummary(result))

	return nil
}
