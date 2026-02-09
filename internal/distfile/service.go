// Package distfile provides unified distfile resolution.
//
// This is the single source of truth for SRC_URI → distfiles conversion.
// Used by both fetch command and ebuild executor.
package distfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// SrcURIEvaluator evaluates SRC_URI from an ebuild file.
// This interface breaks the import cycle between distfile and ebuild packages.
type SrcURIEvaluator interface {
	EvaluateSrcURI(ctx context.Context, ebuildPath, repoPath string, pkgInfo *pkg.Package) (string, error)
}

// Service provides unified distfile resolution for packages.
type Service struct {
	repoPath  string
	mirrors   fetch.ThirdPartyMirrors
	evaluator SrcURIEvaluator
}

// NewService creates a new distfile service with the given SRC_URI evaluator.
func NewService(repoPath string, evaluator SrcURIEvaluator) *Service {
	return &Service{
		repoPath:  repoPath,
		mirrors:   fetch.ParseThirdPartyMirrors(repoPath),
		evaluator: evaluator,
	}
}

// ResolveDistfiles returns distfiles for a specific package version.
//
// Uses EvaluateSrcURI for proper variable expansion (MY_P, eclasses, etc.)
// and filters manifest entries to only include version-specific files.
//
// Parameters:
//   - ctx: Context for cancellation
//   - pkgInfo: Package with name and version
//   - ebuildPath: Path to the ebuild file
//   - manifest: Parsed manifest with checksums
//
// Returns distfiles with checksums from manifest and URIs from SRC_URI.
func (s *Service) ResolveDistfiles(
	ctx context.Context,
	pkgInfo *pkg.Package,
	ebuildPath string,
	manifest *fetch.Manifest,
) ([]fetch.Distfile, error) {
	// Build USE flag set for filtering
	activeFlags := make(map[string]bool)
	if pkgInfo != nil {
		for k, v := range pkgInfo.UseFlags {
			activeFlags[k] = v
		}
	}

	if ebuildPath == "" {
		return filterSignatureFiles(manifest.GetDistfiles(), activeFlags), nil
	}

	// Parse package metadata for variable expansion
	category, pkgName := splitCatPkg(pkgInfo.Name)
	meta := repo.NewPackageMetadata(category, pkgName, pkgInfo.Version)
	vars := map[string]string{
		"P":        meta.P,
		"PN":       meta.PN,
		"PV":       meta.PV,
		"PR":       meta.PR,
		"PVR":      meta.PVR,
		"PF":       meta.PF,
		"CATEGORY": meta.Category,
	}

	// Evaluate SRC_URI with full bash/eclass support
	srcURI, err := s.evaluateSrcURI(ctx, pkgInfo, ebuildPath)
	if err != nil {
		logging.Debug("SRC_URI evaluation failed: %v, trying raw extraction", err)
		// Fallback: extract SRC_URI directly from ebuild text.
		// This handles cases where eclass loading fails in the Go interpreter
		// but SRC_URI is a simple variable assignment in the ebuild.
		srcURI = extractRawSrcURI(ebuildPath, vars)
	}

	if srcURI == "" {
		return filterSignatureFiles(manifest.GetDistfiles(), activeFlags), nil
	}

	// Parse SRC_URI entries with USE flag filtering.
	// This ensures conditional blocks like verify-sig? ( .sig ) are only
	// included when the corresponding USE flag is enabled.
	// IMPORTANT: We always pass a non-nil map (even if empty) so that
	// conditionMet() filters conditionals. nil means "include everything"
	// which would download .sig files even when verify-sig is disabled.
	entries, err := repo.ParseSrcURI(srcURI, activeFlags, vars)
	if err != nil {
		logging.Warn("failed to parse SRC_URI: %v, using filtered manifest", err)
		return filterSignatureFiles(manifest.GetDistfiles(), activeFlags), nil
	}

	// Build filename set and URI map
	srcURIFiles := make(map[string]bool)
	uriMap := make(map[string][]string)

	for _, entry := range entries {
		srcURIFiles[entry.Filename] = true
		if entry.URL != "" {
			expandedURIs := s.mirrors.ExpandMirrorURL(entry.URL)
			uriMap[entry.Filename] = append(uriMap[entry.Filename], expandedURIs...)
		}
	}

	// Create distfiles filtered by SRC_URI
	var distfiles []fetch.Distfile
	for _, entry := range manifest.DistFiles {
		if !srcURIFiles[entry.Filename] {
			continue
		}

		df := fetch.NewDistfile(entry.Filename, entry.Size, entry.Checksums)
		if uris, ok := uriMap[entry.Filename]; ok && len(uris) > 0 {
			df = df.WithURIs(uris)
		}
		distfiles = append(distfiles, df)
	}

	return distfiles, nil
}

// ResolveDistfilesForAtom resolves distfiles for a package atom string.
//
// This is a convenience method that handles:
//   - Finding the best ebuild version (if not specified)
//   - Parsing the manifest
//   - Resolving distfiles
//
// Parameters:
//   - ctx: Context for cancellation
//   - atom: Package atom (e.g., "app-misc/mc" or "=app-misc/mc-4.8.33")
//
// Returns distfiles with checksums and URIs.
func (s *Service) ResolveDistfilesForAtom(ctx context.Context, atom string) ([]fetch.Distfile, error) {
	// Parse atom
	parsedAtom, err := pkg.ParseAtom(atom)
	if err != nil {
		return nil, fmt.Errorf("invalid atom %q: %w", atom, err)
	}

	catPkg := parsedAtom.CP()

	// Parse manifest
	manifestPath := fetch.ManifestPath(s.repoPath, catPkg)
	manifest, err := fetch.ParseManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Manifest: %w", err)
	}

	// Find ebuild path and version
	_, pkgName := splitCatPkg(catPkg)
	var ebuildPath, version string

	if parsedAtom.HasVersion() {
		version = parsedAtom.Version
		ebuildPath = filepath.Join(s.repoPath, catPkg, fmt.Sprintf("%s-%s.ebuild", pkgName, version))
		if _, err := os.Stat(ebuildPath); err != nil {
			return nil, fmt.Errorf("ebuild not found for version %s: %w", version, err)
		}
	} else {
		ebuildPath, version, err = s.findBestEbuild(catPkg, pkgName)
		if err != nil {
			return nil, err
		}
	}

	// Create package info
	pkgInfo := &pkg.Package{
		Name:    catPkg,
		Version: version,
		Slot:    pkg.NewSlot("0", ""),
	}

	return s.ResolveDistfiles(ctx, pkgInfo, ebuildPath, manifest)
}

// evaluateSrcURI evaluates SRC_URI using the injected evaluator.
func (s *Service) evaluateSrcURI(ctx context.Context, pkgInfo *pkg.Package, ebuildPath string) (string, error) {
	if s.evaluator == nil {
		return "", fmt.Errorf("no SRC_URI evaluator configured")
	}
	return s.evaluator.EvaluateSrcURI(ctx, ebuildPath, s.repoPath, pkgInfo)
}

// findBestEbuild finds the highest version ebuild in a package directory.
func (s *Service) findBestEbuild(catPkg, pkgName string) (string, string, error) {
	pkgDir := filepath.Join(s.repoPath, catPkg)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return "", "", fmt.Errorf("reading package directory: %w", err)
	}

	type ebuildInfo struct {
		path    string
		version string
	}
	var ebuilds []ebuildInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".ebuild")
		if !strings.HasPrefix(name, pkgName+"-") {
			continue
		}

		version := strings.TrimPrefix(name, pkgName+"-")
		ebuilds = append(ebuilds, ebuildInfo{
			path:    filepath.Join(pkgDir, entry.Name()),
			version: version,
		})
	}

	if len(ebuilds) == 0 {
		return "", "", fmt.Errorf("no ebuilds found in %s", pkgDir)
	}

	// Sort by version descending
	sort.Slice(ebuilds, func(i, j int) bool {
		return pkg.CompareVersions(ebuilds[i].version, ebuilds[j].version) > 0
	})

	best := ebuilds[0]
	logging.Debug("Best ebuild: %s (version %s)", filepath.Base(best.path), best.version)

	return best.path, best.version, nil
}

// signatureExtensions lists file extensions typically behind verify-sig USE conditional.
var signatureExtensions = []string{".sig", ".asc", ".sign"}

// filterSignatureFiles removes GPG signature files from distfiles when
// the verify-sig USE flag is not enabled.
//
// In Gentoo ebuilds, signature files (.sig, .asc, .sign) are always behind
// a "verify-sig? ( ... )" conditional in SRC_URI. When SRC_URI evaluation
// fails and we fall back to manifest entries, we must filter these out
// to avoid downloading files that aren't needed.
//
// This is the safety net for all fallback paths in ResolveDistfiles.
func filterSignatureFiles(distfiles []fetch.Distfile, activeFlags map[string]bool) []fetch.Distfile {
	// If verify-sig is enabled, keep all files
	if activeFlags != nil && activeFlags["verify-sig"] {
		return distfiles
	}

	filtered := make([]fetch.Distfile, 0, len(distfiles))
	for _, df := range distfiles {
		if isSignatureFile(df.Filename) {
			logging.Debug("filtering out signature file %s (verify-sig not enabled)", df.Filename)
			continue
		}
		filtered = append(filtered, df)
	}
	return filtered
}

// isSignatureFile checks if a filename is a GPG signature file.
func isSignatureFile(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range signatureExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// extractRawSrcURI extracts SRC_URI directly from ebuild text as a fallback.
//
// When the bash interpreter fails to evaluate the ebuild (e.g., due to
// unsupported syntax in eclasses), this function reads the raw SRC_URI
// assignments from the ebuild text. It handles both:
//   - SRC_URI="..." (initial assignment)
//   - SRC_URI+="..." (append assignment, e.g., for verify-sig conditionals)
//
// The returned string preserves USE conditionals (e.g., "verify-sig? ( ... )")
// so that ParseSrcURI can properly filter them.
//
// Limitations:
//   - Cannot evaluate bash conditionals (if/elif/else blocks)
//   - Cannot call eclass functions (e.g., get_gcc_src_uri)
//   - Only handles literal string assignments
func extractRawSrcURI(ebuildPath string, vars map[string]string) string {
	data, err := os.ReadFile(ebuildPath)
	if err != nil {
		return ""
	}

	content := string(data)
	var parts []string

	// Extract initial SRC_URI="..." assignment
	if val := extractQuotedVariable(content, "SRC_URI"); val != "" {
		parts = append(parts, val)
	}

	// Extract append SRC_URI+="..." assignments
	parts = append(parts, extractAppendAssignments(content, "SRC_URI")...)

	if len(parts) == 0 {
		return ""
	}

	// Join all parts and expand variables
	raw := strings.Join(parts, "\n")
	return expandVars(raw, vars)
}

// extractQuotedVariable extracts the value of a variable assignment: VAR="value".
// Handles multi-line quoted strings.
func extractQuotedVariable(content, varName string) string {
	// Search for VAR=" at word boundary
	pattern := varName + `="`
	idx := findAssignment(content, pattern)
	if idx == -1 {
		return ""
	}

	start := idx + len(pattern)
	// Find matching closing quote
	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '\\':
			i++ // skip escaped character
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '"':
			return content[start:i]
		}
	}

	return ""
}

// extractAppendAssignments finds all VAR+="value" assignments in content.
func extractAppendAssignments(content, varName string) []string {
	pattern := varName + `+="`
	var results []string
	searchFrom := 0

	for searchFrom < len(content) {
		idx := findAssignment(content[searchFrom:], pattern)
		if idx == -1 {
			break
		}
		absIdx := searchFrom + idx + len(pattern)
		// Find matching closing quote
		for i := absIdx; i < len(content); i++ {
			if content[i] == '\\' {
				i++
				continue
			}
			if content[i] == '"' {
				results = append(results, content[absIdx:i])
				searchFrom = i + 1
				break
			}
		}
		if searchFrom <= absIdx {
			break // no closing quote found
		}
	}

	return results
}

// findAssignment finds a variable assignment pattern at a word boundary.
// Returns the index of the pattern, or -1 if not found.
func findAssignment(content, pattern string) int {
	idx := 0
	for {
		pos := strings.Index(content[idx:], pattern)
		if pos == -1 {
			return -1
		}
		absPos := idx + pos
		// Check word boundary: must be at start of line or after whitespace/newline
		if absPos == 0 || content[absPos-1] == '\n' || content[absPos-1] == '\t' || content[absPos-1] == ' ' {
			return absPos
		}
		idx = absPos + 1
	}
}

// expandVars performs simple variable expansion in a string.
// Expands both ${VAR} and $VAR forms.
func expandVars(s string, vars map[string]string) string {
	result := s
	for name, value := range vars {
		result = strings.ReplaceAll(result, "${"+name+"}", value)
	}
	// Expand $VAR (followed by non-identifier chars)
	for name, value := range vars {
		for _, sep := range []string{"/", ".", "-", " ", "\t", "\n", "\"", "'"} {
			result = strings.ReplaceAll(result, "$"+name+sep, value+sep)
		}
	}
	return result
}

// splitCatPkg splits "category/package" into category and package name.
func splitCatPkg(catPkg string) (string, string) {
	parts := strings.SplitN(catPkg, "/", 2)
	if len(parts) != 2 {
		return "", catPkg
	}
	return parts[0], parts[1]
}
