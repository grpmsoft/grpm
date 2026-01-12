package analyze

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/grpmsoft/grpm/internal/eclass"
	"github.com/grpmsoft/grpm/internal/repo"
)

// SupportedEAPIs lists the EAPI versions GRPM supports.
// Reference: PMS Section 5.
var SupportedEAPIs = map[string]bool{
	"0": true,
	"1": true,
	"2": true,
	"3": true,
	"4": true,
	"5": true,
	"6": true,
	"7": true,
	"8": true,
}

// Analyzer analyzes a Portage repository for GRPM compatibility.
type Analyzer struct {
	repoPath    string
	eclassCache *eclass.Cache
	helpers     map[string]bool
	verbose     bool
	category    string // Filter to specific category (optional)

	// Concurrency control
	workers int
	mu      sync.Mutex
}

// AnalyzerOption configures the Analyzer.
type AnalyzerOption func(*Analyzer)

// WithVerbose enables verbose output.
func WithVerbose(verbose bool) AnalyzerOption {
	return func(a *Analyzer) {
		a.verbose = verbose
	}
}

// WithCategory filters analysis to a specific category.
func WithCategory(category string) AnalyzerOption {
	return func(a *Analyzer) {
		a.category = category
	}
}

// WithWorkers sets the number of concurrent workers.
func WithWorkers(n int) AnalyzerOption {
	return func(a *Analyzer) {
		if n > 0 {
			a.workers = n
		}
	}
}

// NewAnalyzer creates a new Analyzer for the given repository.
func NewAnalyzer(repoPath string, opts ...AnalyzerOption) (*Analyzer, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	// Verify repository exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository not found: %s", absPath)
	}

	// Create eclass cache for the repository
	eclassDir := filepath.Join(absPath, "eclass")
	eclassCache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		// Log warning but continue - eclass dir may not exist
		log.Printf("Warning: could not load eclasses from %s: %v", eclassDir, err)
		eclassCache = eclass.NewCache()
	}

	a := &Analyzer{
		repoPath:    absPath,
		eclassCache: eclassCache,
		helpers:     buildHelperMap(),
		workers:     4, // Default worker count
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

// buildHelperMap returns a map of all implemented helper functions.
// This mirrors the command map in internal/ebuild/interpreter.go.
func buildHelperMap() map[string]bool {
	return map[string]bool{
		// Messaging functions
		"die": true, "assert": true, "einfo": true, "einfon": true,
		"ewarn": true, "eerror": true, "elog": true, "ebegin": true,
		"eend": true, "nonfatal": true,

		// Debug functions
		"debug-print": true, "debug-print-function": true, "debug-print-section": true,

		// USE flag functions
		"has": true, "use": true, "usev": true, "usex": true,
		"in_iuse": true, "use_enable": true, "use_with": true,

		// Toolchain functions
		"tc-getCC": true, "tc-getCXX": true, "tc-getLD": true, "tc-arch": true,
		"tc-is-gcc": true, "tc-is-clang": true, "tc-export": true,
		"tc-getAR": true, "tc-getRANLIB": true, "tc-getNM": true,
		"tc-getSTRIP": true, "tc-getOBJCOPY": true, "tc-getBUILD_CC": true,
		"tc-endian": true,

		// Directory/option functions
		"into": true, "insinto": true, "exeinto": true, "docinto": true,
		"insopts": true, "exeopts": true, "diropts": true,

		// Binary installation functions
		"dobin": true, "dosbin": true, "newbin": true, "newsbin": true, "doexe": true,

		// File installation functions
		"doins": true, "newins": true,

		// Documentation functions
		"dodoc": true, "newdoc": true, "doman": true, "newman": true,
		"doinfo": true, "domo": true,

		// Library/header functions
		"dolib": true, "dolib.so": true, "dolib.a": true, "doheader": true,

		// Directory functions
		"dodir": true, "keepdir": true,

		// Build helpers
		"emake": true, "econf": true, "unpack": true, "eapply": true, "eapply_user": true,

		// Default phase functions
		"default": true, "default_pkg_nofetch": true, "default_src_unpack": true,
		"default_src_prepare": true, "default_src_configure": true,
		"default_src_compile": true, "default_src_test": true, "default_src_install": true,

		// Version functions
		"ver_cut": true, "ver_rs": true, "ver_test": true,

		// Additional installation helpers
		"dosym": true, "fperms": true, "fowners": true, "doconfd": true,
		"doinitd": true, "doenvd": true, "dostrip": true, "einstalldocs": true,
		"inherit": true,

		// File system utilities
		"sed": true, "cat": true, "mkdir": true, "rm": true, "cp": true,
		"mv": true, "chmod": true, "ln": true, "find": true, "grep": true,
		"xargs": true, "which": true, "touch": true, "install": true, "pkg-config": true,

		// Eclass support functions
		"epatch": true, "eshopts_push": true, "eshopts_pop": true,
		"estack_push": true, "estack_pop": true,

		// Multilib functions
		"get_libdir": true, "multilib_native_use_with": true, "multilib_native_use_enable": true,

		// Flag-o-matic functions
		"append-cflags": true, "append-cxxflags": true, "append-cppflags": true,
		"append-ldflags": true, "append-flags": true, "append-lfs-flags": true,
		"filter-flags": true, "filter-ldflags": true, "filter-lfs-flags": true,
		"replace-flags": true, "replace-cpu-flags": true,
		"strip-flags": true, "strip-unsupported-flags": true,
		"test-flags-CC": true, "test-flags-CXX": true, "test-flags-F77": true,
		"test-flags-FC": true, "test-flags": true,
		"get-flag": true, "is-flag": true, "is-ldflag": true,
		"is-flag-supported": true, "no-as-needed": true, "raw-ldflags": true,

		// Linux-info functions
		"get_version": true, "linux_config_exists": true,
		"linux_config_src_exists": true, "require_configured_kernel": true,

		// Additional eclass functions
		"EXPORT_FUNCTIONS": true, "eqawarn": true, "edosym": true,
		"has_version": true, "best_version": true,

		// CMake functions
		"cmake": true, "cmake_src_prepare": true, "cmake_src_configure": true,
		"cmake_src_compile": true, "cmake_src_test": true, "cmake_src_install": true,
		"cmake_use": true, "cmake_use_find_package": true,
		"cmake_comment_add_subdirectory": true, "cmake_run_in": true,
		"cmake_build_type": true, "cmake_multilib_src_configure": true, "eninja": true,

		// Meson functions
		"meson": true, "meson_src_configure": true, "meson_src_compile": true,
		"meson_src_test": true, "meson_src_install": true,
		"meson_use": true, "meson_feature": true, "meson_use_bool": true,
	}
}

// Analyze performs the repository analysis.
func (a *Analyzer) Analyze(ctx context.Context) (*Result, error) {
	result := NewResult(a.repoPath)

	// Get list of categories to analyze
	categories, err := a.listCategories()
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	// Filter to specific category if requested
	if a.category != "" {
		found := false
		for _, cat := range categories {
			if cat == a.category {
				categories = []string{cat}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("category not found: %s", a.category)
		}
	}

	// Process each category
	for _, category := range categories {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if a.verbose {
			log.Printf("Analyzing category: %s", category)
		}

		if err := a.analyzeCategory(ctx, category, result); err != nil {
			log.Printf("Warning: error analyzing category %s: %v", category, err)
			// Continue with other categories
		}
	}

	result.Finalize()
	return result, nil
}

// listCategories returns all valid category directories.
func (a *Analyzer) listCategories() ([]string, error) {
	entries, err := os.ReadDir(a.repoPath)
	if err != nil {
		return nil, err
	}

	var categories []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Valid categories contain a hyphen (e.g., "app-misc", "sys-libs")
		// Skip special directories
		if !strings.Contains(name, "-") {
			continue
		}
		if name == "profiles" || name == "metadata" || name == "eclass" {
			continue
		}

		categories = append(categories, name)
	}

	return categories, nil
}

// analyzeCategory analyzes all packages in a category.
func (a *Analyzer) analyzeCategory(ctx context.Context, category string, result *Result) error {
	catPath := filepath.Join(a.repoPath, category)

	entries, err := os.ReadDir(catPath)
	if err != nil {
		return fmt.Errorf("reading category %s: %w", category, err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !entry.IsDir() {
			continue
		}

		pkgName := entry.Name()
		pkgPath := filepath.Join(catPath, pkgName)

		// Find and analyze ebuilds
		if err := a.analyzePackage(ctx, category, pkgName, pkgPath, result); err != nil {
			if a.verbose {
				log.Printf("Warning: error analyzing %s/%s: %v", category, pkgName, err)
			}
			// Continue with other packages
		}
	}

	return nil
}

// analyzePackage analyzes all ebuilds in a package directory.
func (a *Analyzer) analyzePackage(ctx context.Context, category, pkgName, pkgPath string, result *Result) error {
	entries, err := os.ReadDir(pkgPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".ebuild") {
			continue
		}

		// Parse version from filename
		// hello-2.10.ebuild -> version = 2.10
		version := strings.TrimSuffix(name, ".ebuild")
		version = strings.TrimPrefix(version, pkgName+"-")

		ebuildPath := filepath.Join(pkgPath, name)
		pr := a.analyzeEbuild(ctx, category, pkgName, version, ebuildPath)

		a.mu.Lock()
		result.AddPackageResult(pr)
		a.mu.Unlock()
	}

	return nil
}

// analyzeEbuild analyzes a single ebuild file.
func (a *Analyzer) analyzeEbuild(_ context.Context, category, pkgName, version, ebuildPath string) *PackageResult {
	pr := &PackageResult{
		Atom:      fmt.Sprintf("%s/%s-%s", category, pkgName, version),
		Category:  category,
		Name:      pkgName,
		Version:   version,
		Supported: true, // Assume supported until proven otherwise
	}

	// Read ebuild content
	content, err := os.ReadFile(ebuildPath)
	if err != nil {
		pr.AddBlocker(BlockerParseError, "read_error", err.Error())
		return pr
	}

	// Parse ebuild
	meta := repo.NewPackageMetadata(category, pkgName, version)
	parser := repo.NewEbuildParserWithMetadata(string(content), meta)

	// Extract EAPI
	pr.EAPI = a.extractEAPI(string(content))
	if pr.EAPI == "" {
		pr.EAPI = "0" // Default EAPI is 0
	}

	// Check EAPI support
	if !SupportedEAPIs[pr.EAPI] {
		pr.AddBlocker(BlockerUnsupportedEAPI, pr.EAPI, "EAPI not supported by GRPM")
	}

	// Extract INHERIT (eclasses)
	pr.Inherits = a.extractInherit(string(content))

	// Check eclass availability
	for _, eclassName := range pr.Inherits {
		if !a.eclassCache.Has(eclassName) {
			pr.AddBlocker(BlockerMissingEclass, eclassName, "eclass not found in repository")
		}
	}

	// Extract RESTRICT
	pr.Restrict = parser.ExtractRestrict()

	// Check for fetch restrictions
	for _, restrict := range pr.Restrict {
		if restrict == "fetch" || restrict == "mirror" {
			pr.AddBlocker(BlockerFetchRestricted, restrict, "RESTRICT="+restrict)
		}
	}

	// Extract SRC_URI
	pr.SrcURI = parser.ExtractVariable("SRC_URI")

	// Check for packages that require sources but have none
	// Virtual and acct-* packages typically don't need SRC_URI
	if pr.SrcURI == "" && !a.isMetaPackage(pr) && len(pr.Inherits) > 0 {
		// Package inherits eclasses but has no SRC_URI - may be a problem
		// This is informational, not a hard blocker
		_ = pr // Suppress unused warning - SrcURI is stored for inspection
	}

	return pr
}

// extractEAPI extracts the EAPI value from ebuild content.
func (a *Analyzer) extractEAPI(content string) string {
	// EAPI must be the first non-comment, non-blank line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for EAPI declaration
		if strings.HasPrefix(line, "EAPI=") {
			eapi := strings.TrimPrefix(line, "EAPI=")
			// Remove quotes if present
			eapi = strings.Trim(eapi, "\"'")
			return eapi
		}

		// EAPI must be first non-comment line, stop searching
		break
	}

	return ""
}

// extractInherit extracts inherited eclasses from ebuild content.
func (a *Analyzer) extractInherit(content string) []string {
	var inherits []string

	// Find all inherit statements
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Check for inherit statement
		if strings.HasPrefix(line, "inherit ") {
			eclasses := strings.TrimPrefix(line, "inherit ")
			// Handle line continuation
			for strings.HasSuffix(eclasses, "\\") {
				eclasses = strings.TrimSuffix(eclasses, "\\")
			}

			// Split into individual eclasses
			for _, ec := range strings.Fields(eclasses) {
				ec = strings.TrimSpace(ec)
				if ec != "" && ec != "\\" {
					inherits = append(inherits, ec)
				}
			}
		}
	}

	return inherits
}

// isMetaPackage checks if this is a metapackage that doesn't need sources.
// Metapackages include virtuals, acct-* (user/group), and packages with RESTRICT=bindist.
func (a *Analyzer) isMetaPackage(pr *PackageResult) bool {
	// Check for common binary package indicators
	for _, restrict := range pr.Restrict {
		if restrict == "bindist" {
			return true
		}
	}

	// Virtual packages don't need sources
	if strings.HasPrefix(pr.Category, "virtual") {
		return true
	}

	// Acct packages (user/group) don't need sources
	if strings.HasPrefix(pr.Category, "acct-") {
		return true
	}

	return false
}

// HasHelper checks if a helper function is implemented.
func (a *Analyzer) HasHelper(name string) bool {
	return a.helpers[name]
}

// EclassAvailable checks if an eclass is available.
func (a *Analyzer) EclassAvailable(name string) bool {
	return a.eclassCache.Has(name)
}

// AvailableEclasses returns the list of available eclasses.
func (a *Analyzer) AvailableEclasses() []string {
	return a.eclassCache.List()
}

// AvailableHelpers returns the list of implemented helpers.
func (a *Analyzer) AvailableHelpers() []string {
	helpers := make([]string, 0, len(a.helpers))
	for h := range a.helpers {
		helpers = append(helpers, h)
	}
	return helpers
}
