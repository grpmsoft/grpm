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
	if ebuildPath == "" {
		return manifest.GetDistfiles(), nil
	}

	// Evaluate SRC_URI with full bash/eclass support
	srcURI, err := s.evaluateSrcURI(ctx, pkgInfo, ebuildPath)
	if err != nil {
		logging.Debug("SRC_URI evaluation failed: %v, using manifest distfiles", err)
		return manifest.GetDistfiles(), nil
	}

	if srcURI == "" {
		return manifest.GetDistfiles(), nil
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

	// Parse SRC_URI entries
	entries, err := repo.ParseSrcURI(srcURI, nil, vars)
	if err != nil {
		logging.Warn("failed to parse SRC_URI: %v, using manifest distfiles", err)
		return manifest.GetDistfiles(), nil
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

// splitCatPkg splits "category/package" into category and package name.
func splitCatPkg(catPkg string) (string, string) {
	parts := strings.SplitN(catPkg, "/", 2)
	if len(parts) != 2 {
		return "", catPkg
	}
	return parts[0], parts[1]
}
