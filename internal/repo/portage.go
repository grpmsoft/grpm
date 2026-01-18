package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coregx/coregex"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/metadata"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Precompiled regex patterns for ebuild metadata parsing (compiled once at package init).
var (
	// portageVersionRe matches VERSION="..." in ebuild files.
	portageVersionRe = coregex.MustCompile(`(?m)^VERSION="([^"]+)"`)

	// portageSlotRe matches SLOT="..." in ebuild files.
	portageSlotRe = coregex.MustCompile(`(?m)^SLOT="([^"]+)"`)

	// portageIuseRe matches IUSE="..." in ebuild files.
	portageIuseRe = coregex.MustCompile(`(?m)^IUSE="([^"]+)"`)

	// portageProvideRe matches PROVIDE="..." in ebuild files.
	portageProvideRe = coregex.MustCompile(`(?m)^PROVIDE="([^"]+)"`)

	// portageKeywordsRe matches KEYWORDS="..." in ebuild files.
	// Supports optional leading whitespace (tabs/spaces) for eclasses like toolchain.eclass.
	portageKeywordsRe = coregex.MustCompile(`(?m)^\s*KEYWORDS="([^"]*)"`)
)

type PortageRepository struct {
	Path  string
	cache sync.Map // Cache for parsed ebuilds: path -> *pkg.Package
}

func NewPortageRepository(path string) (*PortageRepository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository directory does not exist: %s", absPath)
	}

	return &PortageRepository{
		Path: absPath,
	}, nil
}

func (pr *PortageRepository) LoadPackages(names []string) ([]*pkg.Package, error) {
	var packages []*pkg.Package

	for _, name := range names {
		pkg, err := pr.LoadPackage(name)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

func (pr *PortageRepository) LoadPackage(name string) (*pkg.Package, error) {
	category, pkgName, found := strings.Cut(name, "/")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", name)
	}

	// Security: Validate category and package name to prevent path traversal (Issue #36)
	if err := pkg.ValidateCategoryPackageName(category, pkgName); err != nil {
		return nil, fmt.Errorf("invalid package name %q: %w", name, err)
	}

	pkgDir := filepath.Join(pr.Path, category, pkgName)

	// Add path logging
	absPath, _ := filepath.Abs(pkgDir)
	logging.Debug("Looking for package in: %s", absPath)

	files, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("error reading package directory: %w", err)
	}

	var versions []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
			continue
		}

		if strings.HasSuffix(file.Name(), ".ebuild") {
			version := strings.TrimSuffix(file.Name(), ".ebuild")
			version = strings.TrimPrefix(version, pkgName+"-")
			versions = append(versions, version)
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no ebuilds found for %s", name)
	}

	// Sort versions using Portage version comparison (highest first)
	sort.Slice(versions, func(i, j int) bool {
		return pkg.CompareVersions(versions[i], versions[j]) > 0
	})

	// Take the highest version
	latestVersion := versions[0]
	return pr.parseEbuild(name, filepath.Join(pkgDir, pkgName+"-"+latestVersion+".ebuild"))
}

// LoadPackageVersion loads a specific version of a package.
// Returns error if the exact version is not found.
func (pr *PortageRepository) LoadPackageVersion(name, version string) (*pkg.Package, error) {
	category, pkgName, found := strings.Cut(name, "/")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", name)
	}

	// Security: Validate category and package name to prevent path traversal (Issue #36)
	if err := pkg.ValidateCategoryPackageName(category, pkgName); err != nil {
		return nil, fmt.Errorf("invalid package name %q: %w", name, err)
	}

	pkgDir := filepath.Join(pr.Path, category, pkgName)
	ebuildPath := filepath.Join(pkgDir, pkgName+"-"+version+".ebuild")

	// Check if the specific version exists
	if _, err := os.Stat(ebuildPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("version %s not found for %s", version, name)
	}

	return pr.parseEbuild(name, ebuildPath)
}

func (pr *PortageRepository) parseEbuild(name, path string) (*pkg.Package, error) {
	// Check cache first
	if cached, ok := pr.cache.Load(path); ok {
		logging.Debug("Cache hit for ebuild: %s", path)
		return cached.(*pkg.Package), nil
	}

	logging.Debug("Parsing ebuild: %s", path)
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Debug("Error reading ebuild file: %v", err)
		return nil, err
	}

	// Simplified ebuild parser
	p := &pkg.Package{
		Name:     name, // Fixed! Set package name
		Version:  "",
		Slot:     pkg.Slot{Name: "0"},
		UseFlags: make(map[string]bool),
		Keywords: make([]string, 0),
		Deps:     make([]pkg.Constraint, 0),
		Provides: make([]pkg.Constraint, 0),
	}

	// Extract category and package name for variable expansion
	category := ""
	pkgName := name
	if parts := strings.SplitN(name, "/", 2); len(parts) == 2 {
		category = parts[0]
		pkgName = parts[1]
	}

	// Extract version from filename correctly
	// For "complex-2.0-r1.ebuild" with pkgName="complex": version = "2.0-r1"
	filename := filepath.Base(path)
	versionFromFile := strings.TrimSuffix(filename, ".ebuild")
	versionFromFile = strings.TrimPrefix(versionFromFile, pkgName+"-")
	p.Version = versionFromFile

	// Override with VERSION from content if present (rare case)
	if matches := portageVersionRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		p.Version = matches[1]
	}

	// Parse dependencies using EbuildParser with package metadata
	// This enables ${P}, ${PN}, ${PV} expansion in DEPEND, SRC_URI, etc.
	meta := NewPackageMetadata(category, pkgName, p.Version)
	parser := NewEbuildParserWithMetadata(string(content), meta)
	parsedDeps, err := parser.ParseDependencies()
	if err == nil {
		// Convert ParsedDependency to Constraint, preserving OrGroupID
		// Skip blockers - they are conflicts, not dependencies
		realDepsCount := 0
		for _, pd := range parsedDeps {
			if pd.IsBlocker {
				// TODO: Add to p.Conflicts instead of p.Deps
				logging.Debug("Skipping blocker: %s for %s", pd.Constraint.Name, name)
				continue
			}
			constraint := pd.Constraint
			constraint.OrGroupID = pd.OrGroupID // Copy OrGroupID
			p.Deps = append(p.Deps, constraint)
			realDepsCount++
		}
		if realDepsCount > 0 {
			logging.Debug("Parsed %d dependencies for %s (%d blockers skipped)",
				realDepsCount, name, len(parsedDeps)-realDepsCount)
		}
	}

	// If no dependencies found via regex, try eclass-aware evaluation.
	// This is necessary for packages like gcc where DEPEND/RDEPEND/BDEPEND
	// are defined dynamically in eclasses (e.g., toolchain.eclass).
	// See: https://github.com/grpmsoft/grpm/issues/50
	//
	// Note: The mvdan.cc/sh bash interpreter has limitations with complex bash
	// features (e.g., @a parameter expansion). The fallback is wrapped in a
	// deferred recover to prevent panics from crashing the entire program.
	if len(p.Deps) == 0 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Eclass evaluation panic (e.g., unsupported bash feature)
					// This is non-fatal - continue with empty deps
					logging.Debug("Eclass evaluation panic for %s: %v", name, r)
				}
			}()

			eclassDeps, err := pr.loadDependenciesWithEclass(path, p)
			if err == nil && len(eclassDeps) > 0 {
				p.Deps = eclassDeps
				logging.Debug("Loaded %d dependencies via eclass evaluation for %s", len(eclassDeps), name)
			} else if err != nil {
				// Non-fatal: log warning and continue with empty deps
				logging.Debug("Eclass evaluation failed for %s: %v", name, err)
			}
		}()
	}

	if matches := portageSlotRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		p.Slot = pkg.ParseSlot(matches[1])
	}

	if matches := portageIuseRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		flags := strings.Fields(matches[1])
		for _, flag := range flags {
			flag = strings.TrimPrefix(flag, "+")
			flag = strings.TrimPrefix(flag, "-")
			p.UseFlags[flag] = true
		}
	}

	// Parse KEYWORDS - critical for keyword masking
	if matches := portageKeywordsRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		keywords := strings.Fields(matches[1])
		p.Keywords = keywords
		logging.Debug("Parsed KEYWORDS for %s: %v", name, keywords)
	}
	// Note: Empty KEYWORDS (len == 0) means unkeyworded/live package

	if matches := portageProvideRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		provides := strings.Fields(matches[1])
		for _, prov := range provides {
			p.Provides = append(p.Provides, pkg.Constraint{
				Type: pkg.ConstraintTypeVersion,
				Name: prov,
			})
		}
	}

	// Store in cache before returning
	pr.cache.Store(path, p)

	return p, nil
}

// FindBySpecification finds packages matching the specification
// Traverses repository filesystem and filters by specification
func (pr *PortageRepository) FindBySpecification(spec Specification) ([]*pkg.Package, error) {
	var matchedPackages []*pkg.Package

	// Traverse categories
	categories, err := os.ReadDir(pr.Path)
	if err != nil {
		return nil, fmt.Errorf("reading repository: %w", err)
	}

	for _, categoryDir := range categories {
		if !categoryDir.IsDir() {
			continue
		}

		categoryPath := filepath.Join(pr.Path, categoryDir.Name())
		packages, err := os.ReadDir(categoryPath)
		if err != nil {
			continue // Skip inaccessible directories
		}

		for _, packageDir := range packages {
			if !packageDir.IsDir() {
				continue
			}

			packageName := categoryDir.Name() + "/" + packageDir.Name()

			// Load all versions of this package
			versions, err := pr.GetAllVersions(packageName)
			if err != nil {
				continue // Skip problematic packages
			}

			// Filter by specification
			for _, p := range versions {
				if spec.IsSatisfiedBy(p) {
					matchedPackages = append(matchedPackages, p)
				}
			}
		}
	}

	return matchedPackages, nil
}

// GetAllVersions returns all versions of a package
// Reads all .ebuild files in package directory
func (pr *PortageRepository) GetAllVersions(packageName string) ([]*pkg.Package, error) {
	category, pkgName, found := strings.Cut(packageName, "/")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", packageName)
	}

	// Security: Validate category and package name to prevent path traversal (Issue #36)
	if err := pkg.ValidateCategoryPackageName(category, pkgName); err != nil {
		return nil, fmt.Errorf("invalid package name %q: %w", packageName, err)
	}

	pkgDir := filepath.Join(pr.Path, category, pkgName)

	// Check if directory exists
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("package directory does not exist: %s", pkgDir)
	}

	files, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("reading package directory: %w", err)
	}

	var packages []*pkg.Package

	// Parse all .ebuild files
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
			continue
		}

		ebuildPath := filepath.Join(pkgDir, file.Name())
		p, err := pr.parseEbuild(packageName, ebuildPath)
		if err != nil {
			logging.Debug("Warning: failed to parse %s: %v", file.Name(), err)
			continue
		}

		packages = append(packages, p)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no valid ebuilds found for %s", packageName)
	}

	return packages, nil
}

// Exists checks if a package exists in the repository
func (pr *PortageRepository) Exists(name string) bool {
	_, err := pr.LoadPackage(name)
	return err == nil
}

// Count returns the total number of packages (unique package names, not versions)
// Traverses entire repository and counts package directories
func (pr *PortageRepository) Count() (int, error) {
	count := 0

	// Traverse categories
	categories, err := os.ReadDir(pr.Path)
	if err != nil {
		return 0, fmt.Errorf("reading repository: %w", err)
	}

	for _, categoryDir := range categories {
		if !categoryDir.IsDir() {
			continue
		}

		categoryPath := filepath.Join(pr.Path, categoryDir.Name())
		packages, err := os.ReadDir(categoryPath)
		if err != nil {
			continue // Skip inaccessible directories
		}

		for _, packageDir := range packages {
			if packageDir.IsDir() {
				count++
			}
		}
	}

	return count, nil
}

// FindByAtom finds all packages matching a PMS-compliant atom.
// Uses Atom.Matches() for version, slot, and subslot matching.
// Optimized: only searches the specific category/package directory.
func (pr *PortageRepository) FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error) {
	if atom == nil {
		return nil, fmt.Errorf("atom is nil")
	}

	// Construct the package directory path from atom's category/package
	packageName := atom.CP()

	// Get all versions of this package
	versions, err := pr.GetAllVersions(packageName)
	if err != nil {
		// Package doesn't exist - return empty slice, not error
		return []*pkg.Package{}, nil
	}

	// Filter by atom matching (version, slot, subslot constraints)
	var result []*pkg.Package
	for _, p := range versions {
		if atom.Matches(p) {
			result = append(result, p)
		}
	}

	return result, nil
}

// loadDependenciesWithEclass extracts dependencies from an ebuild using eclass-aware
// evaluation. This is necessary for packages that define DEPEND/RDEPEND/BDEPEND in
// eclasses rather than directly in the ebuild (e.g., gcc via toolchain.eclass).
//
// This method uses metadata.Evaluator to source the ebuild with eclass support,
// then parses the resulting dependency strings.
//
// Parameters:
//   - ebuildPath: Path to the ebuild file
//   - p: Package to populate with dependencies (must have Name and Version set)
//
// Returns the list of parsed dependencies, or error if extraction fails.
func (pr *PortageRepository) loadDependenciesWithEclass(ebuildPath string, p *pkg.Package) ([]pkg.Constraint, error) {
	// Create metadata evaluator for this repository
	evaluator, err := metadata.NewEvaluator(pr.Path)
	if err != nil {
		return nil, fmt.Errorf("creating metadata evaluator: %w", err)
	}

	// Extract DEPEND, RDEPEND, BDEPEND, IDEPEND, PDEPEND from ebuild with eclass support
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert pkg.Package to metadata.PackageInfo
	pkgInfo := &metadata.PackageInfo{
		Name:     p.Name,
		Version:  p.Version,
		Slot:     p.Slot.String(),
		UseFlags: p.UseFlags,
	}

	varNames := []string{"DEPEND", "RDEPEND", "BDEPEND", "IDEPEND", "PDEPEND"}
	extractedMetadata, err := evaluator.ExtractMetadata(ctx, ebuildPath, pkgInfo, varNames)
	if err != nil {
		return nil, fmt.Errorf("extracting metadata: %w", err)
	}

	// Parse the dependency strings using EbuildParser
	var allDeps []pkg.Constraint
	parser := NewEbuildParser("") // Empty content - we only use parseDependencyString

	// Parse each dependency type
	depTypes := map[string]DependencyType{
		"DEPEND":  DepTypeBuild,
		"RDEPEND": DepTypeRuntime,
		"BDEPEND": DepTypeBuildtime,
		"IDEPEND": DepTypeInstall,
		"PDEPEND": DepTypePostMerge,
	}

	for varName, depType := range depTypes {
		depStr := extractedMetadata[varName]
		if depStr == "" {
			continue
		}

		deps, err := parser.parseDependencyString(depStr, depType)
		if err != nil {
			logging.Debug("Warning: failed to parse %s for %s: %v", varName, p.Name, err)
			continue
		}

		// Convert ParsedDependency to Constraint, skip blockers
		for _, pd := range deps {
			if pd.IsBlocker {
				continue
			}
			constraint := pd.Constraint
			constraint.OrGroupID = pd.OrGroupID
			allDeps = append(allDeps, constraint)
		}
	}

	if len(allDeps) > 0 {
		logging.Debug("Extracted %d dependencies via eclass evaluation for %s", len(allDeps), p.Name)
	}

	return allDeps, nil
}
