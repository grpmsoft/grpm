package analyze

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// OutputFormat specifies the output format for reports.
type OutputFormat string

const (
	// FormatText produces human-readable text output.
	FormatText OutputFormat = "text"

	// FormatJSON produces machine-readable JSON output.
	FormatJSON OutputFormat = "json"

	// FormatMarkdown produces markdown-formatted output for documentation.
	FormatMarkdown OutputFormat = "markdown"
)

// ParseOutputFormat parses a string into OutputFormat.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(s) {
	case "text", "txt", "":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	case "markdown", "md":
		return FormatMarkdown, nil
	default:
		return FormatText, fmt.Errorf("unknown output format: %s (valid: text, json, markdown)", s)
	}
}

// Reporter formats and outputs analysis results.
type Reporter struct {
	format  OutputFormat
	verbose bool
}

// NewReporter creates a new Reporter.
func NewReporter(format OutputFormat, verbose bool) *Reporter {
	return &Reporter{
		format:  format,
		verbose: verbose,
	}
}

// Report writes the analysis result to the given writer.
func (r *Reporter) Report(w io.Writer, result *Result) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(w, result)
	case FormatMarkdown:
		return r.reportMarkdown(w, result)
	default:
		return r.reportText(w, result)
	}
}

// lineWriter wraps an io.Writer and tracks the first error.
type lineWriter struct {
	w   io.Writer
	err error
}

func (lw *lineWriter) println(s string) {
	if lw.err != nil {
		return
	}
	_, lw.err = fmt.Fprintln(lw.w, s)
}

func (lw *lineWriter) printf(format string, args ...interface{}) {
	if lw.err != nil {
		return
	}
	_, lw.err = fmt.Fprintf(lw.w, format, args...)
}

func (lw *lineWriter) newline() {
	if lw.err != nil {
		return
	}
	_, lw.err = fmt.Fprintln(lw.w)
}

// reportText generates human-readable text output.
func (r *Reporter) reportText(w io.Writer, result *Result) error {
	lw := &lineWriter{w: w}

	// Header
	lw.println("GRPM Coverage Analysis")
	lw.println("======================")
	lw.printf("Repository: %s\n", result.RepoPath)
	lw.printf("Total packages: %d\n", result.TotalPackages)
	lw.printf("Supported: %d (%.1f%%)\n", result.SupportedPackages, result.Coverage)
	lw.printf("Unsupported: %d (%.1f%%)\n", result.UnsupportedPackages, 100-result.Coverage)
	lw.newline()

	r.writeTextEAPI(lw, result)
	r.writeTextBlockers(lw, result)
	r.writeTextCategories(lw, result)
	r.writeTextVerbose(lw, result)

	return lw.err
}

func (r *Reporter) writeTextEAPI(lw *lineWriter, result *Result) {
	if len(result.ByEAPI) == 0 {
		return
	}
	lw.println("By EAPI:")
	for eapi, count := range result.ByEAPI {
		lw.printf("  EAPI %s: %d packages\n", eapi, count)
	}
	lw.newline()
}

func (r *Reporter) writeTextBlockers(lw *lineWriter, result *Result) {
	topBlockers := result.TopBlockers(10)
	if len(topBlockers) > 0 {
		lw.println("Top blockers:")
		for _, b := range topBlockers {
			lw.printf("  - %s: %d\n", b.Blocker, b.Count)
		}
		lw.newline()
	}

	byType := result.BlockersByType()
	if len(byType) > 0 {
		lw.println("Blockers by type:")
		for blockerType, count := range byType {
			lw.printf("  - %s: %d\n", blockerType, count)
		}
		lw.newline()
	}
}

func (r *Reporter) writeTextCategories(lw *lineWriter, result *Result) {
	cats := result.SortedCategories()
	if len(cats) == 0 {
		return
	}
	lw.println("By category:")
	limit := 20
	if len(cats) < limit {
		limit = len(cats)
	}
	for i := 0; i < limit; i++ {
		cat := cats[i]
		lw.printf("  %s: %d/%d (%.1f%%)\n",
			cat.Name, cat.SupportedPackages, cat.TotalPackages, cat.Coverage)
	}
	if len(cats) > limit {
		lw.printf("  ... and %d more categories\n", len(cats)-limit)
	}
	lw.newline()
}

func (r *Reporter) writeTextVerbose(lw *lineWriter, result *Result) {
	if !r.verbose {
		return
	}

	// Top eclasses
	eclasses := result.SortedEclasses()
	if len(eclasses) > 0 {
		lw.println("Top eclasses by usage:")
		limit := 20
		if len(eclasses) < limit {
			limit = len(eclasses)
		}
		for i := 0; i < limit; i++ {
			ec := eclasses[i]
			available := "available"
			if !ec.Available {
				available = "MISSING"
			}
			lw.printf("  %s: %d packages (%s)\n", ec.Name, ec.PackagesUsing, available)
		}
		lw.newline()
	}

	// Unsupported packages
	unsupported := result.GetUnsupportedPackages()
	if len(unsupported) > 0 {
		lw.println("Unsupported packages:")
		limit := 50
		if len(unsupported) < limit {
			limit = len(unsupported)
		}
		for i := 0; i < limit; i++ {
			pr := unsupported[i]
			blockers := make([]string, 0, len(pr.Blockers))
			for _, b := range pr.Blockers {
				blockers = append(blockers, b.String())
			}
			lw.printf("  %s: %s\n", pr.Atom, strings.Join(blockers, ", "))
		}
		if len(unsupported) > limit {
			lw.printf("  ... and %d more\n", len(unsupported)-limit)
		}
	}
}

// JSONResult is the JSON-serializable version of Result.
type JSONResult struct {
	Repository          string               `json:"repository"`
	Timestamp           string               `json:"timestamp"`
	TotalPackages       int                  `json:"total_packages"`
	TotalEbuilds        int                  `json:"total_ebuilds"`
	SupportedPackages   int                  `json:"supported_packages"`
	UnsupportedPackages int                  `json:"unsupported_packages"`
	Coverage            float64              `json:"coverage_percent"`
	ByEAPI              map[string]int       `json:"by_eapi"`
	ByBlocker           map[string]int       `json:"by_blocker"`
	ByCategory          []JSONCategoryResult `json:"by_category"`
	ByEclass            []JSONEclassResult   `json:"by_eclass"`
	TopBlockers         []JSONBlocker        `json:"top_blockers"`
	Packages            []JSONPackageResult  `json:"packages,omitempty"`
}

// JSONCategoryResult is the JSON-serializable version of CategoryResult.
type JSONCategoryResult struct {
	Name        string  `json:"name"`
	Total       int     `json:"total"`
	Supported   int     `json:"supported"`
	Unsupported int     `json:"unsupported"`
	Coverage    float64 `json:"coverage_percent"`
}

// JSONEclassResult is the JSON-serializable version of EclassResult.
type JSONEclassResult struct {
	Name          string `json:"name"`
	Available     bool   `json:"available"`
	PackagesUsing int    `json:"packages_using"`
	Supported     int    `json:"supported"`
}

// JSONBlocker is a JSON-serializable blocker with count.
type JSONBlocker struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Details string `json:"details,omitempty"`
	Count   int    `json:"count"`
}

// JSONPackageResult is the JSON-serializable version of PackageResult.
type JSONPackageResult struct {
	Atom      string   `json:"atom"`
	Category  string   `json:"category"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	EAPI      string   `json:"eapi"`
	Supported bool     `json:"supported"`
	Inherits  []string `json:"inherits,omitempty"`
	Blockers  []string `json:"blockers,omitempty"`
}

// reportJSON generates machine-readable JSON output.
func (r *Reporter) reportJSON(w io.Writer, result *Result) error {
	jsonResult := JSONResult{
		Repository:          result.RepoPath,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
		TotalPackages:       result.TotalPackages,
		TotalEbuilds:        result.TotalEbuilds,
		SupportedPackages:   result.SupportedPackages,
		UnsupportedPackages: result.UnsupportedPackages,
		Coverage:            result.Coverage,
		ByEAPI:              result.ByEAPI,
		ByBlocker:           result.ByBlocker,
	}

	// Categories
	for _, cat := range result.SortedCategories() {
		jsonResult.ByCategory = append(jsonResult.ByCategory, JSONCategoryResult{
			Name:        cat.Name,
			Total:       cat.TotalPackages,
			Supported:   cat.SupportedPackages,
			Unsupported: cat.UnsupportedPackages,
			Coverage:    cat.Coverage,
		})
	}

	// Eclasses
	for _, ec := range result.SortedEclasses() {
		jsonResult.ByEclass = append(jsonResult.ByEclass, JSONEclassResult{
			Name:          ec.Name,
			Available:     ec.Available,
			PackagesUsing: ec.PackagesUsing,
			Supported:     ec.SupportedPackages,
		})
	}

	// Top blockers
	for _, b := range result.TopBlockers(50) {
		jsonResult.TopBlockers = append(jsonResult.TopBlockers, parseBlockerString(b.Blocker, b.Count))
	}

	// Packages (if verbose)
	if r.verbose {
		for _, pr := range result.Packages {
			blockers := make([]string, 0, len(pr.Blockers))
			for _, b := range pr.Blockers {
				blockers = append(blockers, b.String())
			}

			jsonResult.Packages = append(jsonResult.Packages, JSONPackageResult{
				Atom:      pr.Atom,
				Category:  pr.Category,
				Name:      pr.Name,
				Version:   pr.Version,
				EAPI:      pr.EAPI,
				Supported: pr.Supported,
				Inherits:  pr.Inherits,
				Blockers:  blockers,
			})
		}
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonResult)
}

// parseBlockerString parses a blocker string "type:name (details)" into JSONBlocker.
func parseBlockerString(blockerStr string, count int) JSONBlocker {
	parts := strings.SplitN(blockerStr, ":", 2)
	blockerType := ""
	name := blockerStr
	details := ""

	if len(parts) == 2 {
		blockerType = parts[0]
		remaining := parts[1]
		// Check for details in parentheses
		if idx := strings.Index(remaining, " ("); idx > 0 {
			name = remaining[:idx]
			details = strings.Trim(remaining[idx+2:], ")")
		} else {
			name = remaining
		}
	}

	return JSONBlocker{
		Type:    blockerType,
		Name:    name,
		Details: details,
		Count:   count,
	}
}

// reportMarkdown generates markdown-formatted output.
func (r *Reporter) reportMarkdown(w io.Writer, result *Result) error {
	lw := &lineWriter{w: w}

	// Header
	lw.println("# GRPM Coverage Analysis Report")
	lw.newline()
	lw.printf("**Repository:** `%s`\n", result.RepoPath)
	lw.printf("**Generated:** %s\n", time.Now().UTC().Format(time.RFC3339))
	lw.newline()

	r.writeMarkdownSummary(lw, result)
	r.writeMarkdownEAPI(lw, result)
	r.writeMarkdownBlockers(lw, result)
	r.writeMarkdownCategories(lw, result)
	r.writeMarkdownVerbose(lw, result)

	// Footer
	lw.println("---")
	lw.println("*Generated by GRPM Coverage Analyzer*")

	return lw.err
}

func (r *Reporter) writeMarkdownSummary(lw *lineWriter, result *Result) {
	lw.println("## Summary")
	lw.newline()
	lw.println("| Metric | Value |")
	lw.println("|--------|-------|")
	lw.printf("| Total Packages | %d |\n", result.TotalPackages)
	lw.printf("| Supported | %d (%.1f%%) |\n", result.SupportedPackages, result.Coverage)
	lw.printf("| Unsupported | %d (%.1f%%) |\n", result.UnsupportedPackages, 100-result.Coverage)
	lw.newline()
}

func (r *Reporter) writeMarkdownEAPI(lw *lineWriter, result *Result) {
	if len(result.ByEAPI) == 0 {
		return
	}
	lw.println("## EAPI Distribution")
	lw.newline()
	lw.println("| EAPI | Packages |")
	lw.println("|------|----------|")
	for eapi, count := range result.ByEAPI {
		lw.printf("| %s | %d |\n", eapi, count)
	}
	lw.newline()
}

func (r *Reporter) writeMarkdownBlockers(lw *lineWriter, result *Result) {
	topBlockers := result.TopBlockers(15)
	if len(topBlockers) > 0 {
		lw.println("## Top Blockers")
		lw.newline()
		lw.println("| Blocker | Count |")
		lw.println("|---------|-------|")
		for _, b := range topBlockers {
			// Escape pipe characters in blocker string
			blocker := strings.ReplaceAll(b.Blocker, "|", "\\|")
			lw.printf("| `%s` | %d |\n", blocker, b.Count)
		}
		lw.newline()
	}

	byType := result.BlockersByType()
	if len(byType) > 0 {
		lw.println("## Blockers by Type")
		lw.newline()
		lw.println("| Type | Count |")
		lw.println("|------|-------|")
		for blockerType, count := range byType {
			lw.printf("| `%s` | %d |\n", blockerType, count)
		}
		lw.newline()
	}
}

func (r *Reporter) writeMarkdownCategories(lw *lineWriter, result *Result) {
	cats := result.SortedCategories()
	if len(cats) == 0 {
		return
	}
	lw.println("## Coverage by Category")
	lw.newline()
	lw.println("| Category | Supported | Total | Coverage |")
	lw.println("|----------|-----------|-------|----------|")

	limit := 30
	if len(cats) < limit {
		limit = len(cats)
	}
	for i := 0; i < limit; i++ {
		cat := cats[i]
		lw.printf("| %s | %d | %d | %.1f%% |\n",
			cat.Name, cat.SupportedPackages, cat.TotalPackages, cat.Coverage)
	}
	if len(cats) > limit {
		lw.printf("\n*... and %d more categories*\n", len(cats)-limit)
	}
	lw.newline()
}

func (r *Reporter) writeMarkdownVerbose(lw *lineWriter, result *Result) {
	if !r.verbose {
		return
	}

	eclasses := result.SortedEclasses()
	if len(eclasses) == 0 {
		return
	}
	lw.println("## Eclass Usage")
	lw.newline()
	lw.println("| Eclass | Packages | Status |")
	lw.println("|--------|----------|--------|")

	limit := 30
	if len(eclasses) < limit {
		limit = len(eclasses)
	}
	for i := 0; i < limit; i++ {
		ec := eclasses[i]
		status := "Available"
		if !ec.Available {
			status = "**MISSING**"
		}
		lw.printf("| %s | %d | %s |\n", ec.Name, ec.PackagesUsing, status)
	}
	lw.newline()
}

// FormatSummary returns a one-line summary suitable for logging.
func FormatSummary(result *Result) string {
	return fmt.Sprintf("Coverage: %.1f%% (%d/%d supported) - %d blockers",
		result.Coverage, result.SupportedPackages, result.TotalPackages, len(result.ByBlocker))
}
