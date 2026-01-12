// Package analyze provides coverage analysis for Portage repositories.
//
// This package analyzes a Portage repository to determine what percentage
// of packages GRPM can successfully build. It checks for:
//   - Supported EAPI versions
//   - Available eclasses
//   - Implemented helper functions
//   - Fetch restrictions
//   - External tool dependencies
//
// Reference: GRPM v0.6.0-004 specification
package analyze

import (
	"fmt"
	"sort"
	"strings"
)

// BlockerType categorizes why a package cannot be built.
type BlockerType string

const (
	// BlockerMissingEclass indicates an eclass is not available.
	BlockerMissingEclass BlockerType = "missing_eclass"

	// BlockerMissingHelper indicates a helper function is not implemented.
	BlockerMissingHelper BlockerType = "missing_helper"

	// BlockerUnsupportedEAPI indicates the EAPI version is not supported.
	BlockerUnsupportedEAPI BlockerType = "unsupported_eapi"

	// BlockerFetchRestricted indicates RESTRICT=fetch is set.
	BlockerFetchRestricted BlockerType = "fetch_restricted"

	// BlockerBinaryOnly indicates no source is available.
	BlockerBinaryOnly BlockerType = "binary_only"

	// BlockerExternalTool indicates an unavailable external tool is required.
	BlockerExternalTool BlockerType = "external_tool"

	// BlockerParseError indicates the ebuild could not be parsed.
	BlockerParseError BlockerType = "parse_error"
)

// Blocker represents a specific blocker preventing package building.
type Blocker struct {
	// Type categorizes the blocker.
	Type BlockerType

	// Name is the specific item blocking (e.g., eclass name, helper name).
	Name string

	// Details provides additional context.
	Details string
}

// String returns a human-readable blocker description.
func (b Blocker) String() string {
	if b.Details != "" {
		return fmt.Sprintf("%s:%s (%s)", b.Type, b.Name, b.Details)
	}
	return fmt.Sprintf("%s:%s", b.Type, b.Name)
}

// PackageResult represents the analysis result for a single package.
type PackageResult struct {
	// Atom is the package atom (e.g., "app-misc/hello-2.10").
	Atom string

	// Category is the package category (e.g., "app-misc").
	Category string

	// Name is the package name without category (e.g., "hello").
	Name string

	// Version is the package version (e.g., "2.10").
	Version string

	// EAPI is the ebuild API version (0-8).
	EAPI string

	// Supported indicates if GRPM can build this package.
	Supported bool

	// Inherits lists eclasses inherited by this ebuild.
	Inherits []string

	// Blockers lists reasons why the package is unsupported.
	Blockers []Blocker

	// SrcURI is the source URI (for fetch analysis).
	SrcURI string

	// Restrict lists RESTRICT values.
	Restrict []string
}

// AddBlocker adds a blocker to the package result.
func (pr *PackageResult) AddBlocker(blockerType BlockerType, name, details string) {
	pr.Blockers = append(pr.Blockers, Blocker{
		Type:    blockerType,
		Name:    name,
		Details: details,
	})
	pr.Supported = false
}

// CategoryResult aggregates analysis results for a category.
type CategoryResult struct {
	// Name is the category name (e.g., "app-misc").
	Name string

	// TotalPackages is the number of packages in this category.
	TotalPackages int

	// SupportedPackages is the number of buildable packages.
	SupportedPackages int

	// UnsupportedPackages is the number of packages that cannot be built.
	UnsupportedPackages int

	// Coverage is the percentage of supported packages.
	Coverage float64

	// TopBlockers maps blocker string to count for this category.
	TopBlockers map[string]int
}

// EclassResult aggregates analysis results for an eclass.
type EclassResult struct {
	// Name is the eclass name (e.g., "cmake").
	Name string

	// Available indicates if this eclass is available in GRPM.
	Available bool

	// PackagesUsing is the count of packages inheriting this eclass.
	PackagesUsing int

	// SupportedPackages is the count of supported packages using this eclass.
	SupportedPackages int
}

// Result is the complete analysis result for a repository.
type Result struct {
	// RepoPath is the path to the analyzed repository.
	RepoPath string

	// TotalPackages is the total number of packages analyzed.
	TotalPackages int

	// TotalEbuilds is the total number of ebuilds analyzed (packages may have multiple versions).
	TotalEbuilds int

	// SupportedPackages is the number of packages GRPM can build.
	SupportedPackages int

	// UnsupportedPackages is the number of packages GRPM cannot build.
	UnsupportedPackages int

	// Coverage is the overall percentage of supported packages.
	Coverage float64

	// ByCategory maps category name to category results.
	ByCategory map[string]*CategoryResult

	// ByEclass maps eclass name to eclass results.
	ByEclass map[string]*EclassResult

	// ByBlocker maps blocker string to count.
	ByBlocker map[string]int

	// ByEAPI maps EAPI version to count.
	ByEAPI map[string]int

	// Packages contains detailed results for each package (optional, for verbose mode).
	Packages []*PackageResult
}

// NewResult creates a new empty Result.
func NewResult(repoPath string) *Result {
	return &Result{
		RepoPath:   repoPath,
		ByCategory: make(map[string]*CategoryResult),
		ByEclass:   make(map[string]*EclassResult),
		ByBlocker:  make(map[string]int),
		ByEAPI:     make(map[string]int),
		Packages:   make([]*PackageResult, 0),
	}
}

// AddPackageResult adds a package result and updates aggregates.
func (r *Result) AddPackageResult(pr *PackageResult) {
	r.TotalPackages++
	r.TotalEbuilds++

	if pr.Supported {
		r.SupportedPackages++
	} else {
		r.UnsupportedPackages++
	}

	// Update EAPI stats
	if pr.EAPI != "" {
		r.ByEAPI[pr.EAPI]++
	}

	// Update category stats
	cat := r.getOrCreateCategory(pr.Category)
	cat.TotalPackages++
	if pr.Supported {
		cat.SupportedPackages++
	} else {
		cat.UnsupportedPackages++
	}

	// Update eclass stats
	for _, eclass := range pr.Inherits {
		ec := r.getOrCreateEclass(eclass)
		ec.PackagesUsing++
		if pr.Supported {
			ec.SupportedPackages++
		}
	}

	// Update blocker stats
	for _, blocker := range pr.Blockers {
		key := blocker.String()
		r.ByBlocker[key]++
		cat.TopBlockers[key]++
	}

	// Store detailed result
	r.Packages = append(r.Packages, pr)
}

// getOrCreateCategory returns existing or creates new CategoryResult.
func (r *Result) getOrCreateCategory(name string) *CategoryResult {
	if cat, ok := r.ByCategory[name]; ok {
		return cat
	}
	cat := &CategoryResult{
		Name:        name,
		TopBlockers: make(map[string]int),
	}
	r.ByCategory[name] = cat
	return cat
}

// getOrCreateEclass returns existing or creates new EclassResult.
func (r *Result) getOrCreateEclass(name string) *EclassResult {
	if ec, ok := r.ByEclass[name]; ok {
		return ec
	}
	ec := &EclassResult{
		Name: name,
	}
	r.ByEclass[name] = ec
	return ec
}

// Finalize calculates final statistics after all packages are processed.
func (r *Result) Finalize() {
	// Calculate overall coverage
	if r.TotalPackages > 0 {
		r.Coverage = float64(r.SupportedPackages) / float64(r.TotalPackages) * 100
	}

	// Calculate per-category coverage
	for _, cat := range r.ByCategory {
		if cat.TotalPackages > 0 {
			cat.Coverage = float64(cat.SupportedPackages) / float64(cat.TotalPackages) * 100
		}
	}
}

// TopBlockers returns the top N blockers by count.
func (r *Result) TopBlockers(n int) []struct {
	Blocker string
	Count   int
} {
	type blockerCount struct {
		Blocker string
		Count   int
	}

	blockers := make([]blockerCount, 0, len(r.ByBlocker))
	for blocker, count := range r.ByBlocker {
		blockers = append(blockers, blockerCount{blocker, count})
	}

	// Sort by count descending
	sort.Slice(blockers, func(i, j int) bool {
		return blockers[i].Count > blockers[j].Count
	})

	if n > len(blockers) {
		n = len(blockers)
	}

	result := make([]struct {
		Blocker string
		Count   int
	}, n)
	for i := 0; i < n; i++ {
		result[i].Blocker = blockers[i].Blocker
		result[i].Count = blockers[i].Count
	}

	return result
}

// SortedCategories returns categories sorted by coverage (descending).
func (r *Result) SortedCategories() []*CategoryResult {
	cats := make([]*CategoryResult, 0, len(r.ByCategory))
	for _, cat := range r.ByCategory {
		cats = append(cats, cat)
	}

	sort.Slice(cats, func(i, j int) bool {
		return cats[i].Coverage > cats[j].Coverage
	})

	return cats
}

// SortedEclasses returns eclasses sorted by usage count (descending).
func (r *Result) SortedEclasses() []*EclassResult {
	eclasses := make([]*EclassResult, 0, len(r.ByEclass))
	for _, ec := range r.ByEclass {
		eclasses = append(eclasses, ec)
	}

	sort.Slice(eclasses, func(i, j int) bool {
		return eclasses[i].PackagesUsing > eclasses[j].PackagesUsing
	})

	return eclasses
}

// FilterByCategory returns a new Result containing only the specified category.
func (r *Result) FilterByCategory(category string) *Result {
	filtered := NewResult(r.RepoPath)

	for _, pr := range r.Packages {
		if pr.Category == category {
			filtered.AddPackageResult(pr)
		}
	}

	filtered.Finalize()
	return filtered
}

// GetUnsupportedPackages returns all unsupported packages.
func (r *Result) GetUnsupportedPackages() []*PackageResult {
	var unsupported []*PackageResult
	for _, pr := range r.Packages {
		if !pr.Supported {
			unsupported = append(unsupported, pr)
		}
	}
	return unsupported
}

// String returns a brief summary of the result.
func (r *Result) String() string {
	return fmt.Sprintf("Coverage: %.1f%% (%d/%d packages supported)",
		r.Coverage, r.SupportedPackages, r.TotalPackages)
}

// BlockersByType groups blockers by type and returns counts.
func (r *Result) BlockersByType() map[BlockerType]int {
	result := make(map[BlockerType]int)
	for blocker, count := range r.ByBlocker {
		// Parse blocker type from "type:name" format
		if idx := strings.Index(blocker, ":"); idx > 0 {
			blockerType := BlockerType(blocker[:idx])
			result[blockerType] += count
		}
	}
	return result
}
