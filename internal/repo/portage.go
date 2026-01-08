package repo

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coregx/coregex"
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

	pkgDir := filepath.Join(pr.Path, category, pkgName)

	// Add path logging
	absPath, _ := filepath.Abs(pkgDir)
	log.Printf("Looking for package in: %s", absPath)

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

	// Take the latest version (for example)
	latestVersion := versions[len(versions)-1]
	return pr.parseEbuild(name, filepath.Join(pkgDir, pkgName+"-"+latestVersion+".ebuild")) // Pass package name
}

func (pr *PortageRepository) parseEbuild(name, path string) (*pkg.Package, error) {
	// Check cache first
	if cached, ok := pr.cache.Load(path); ok {
		log.Printf("Cache hit for ebuild: %s", path)
		return cached.(*pkg.Package), nil
	}

	log.Printf("Parsing ebuild: %s", path)
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Error reading ebuild file: %v", err)
		return nil, err
	}

	// Simplified ebuild parser
	p := &pkg.Package{
		Name:     name, // Fixed! Set package name
		Version:  "",
		Slot:     pkg.Slot{Name: "0"},
		UseFlags: make(map[string]bool),
		Deps:     make([]pkg.Constraint, 0),
		Provides: make([]pkg.Constraint, 0),
	}

	// Extract version from filename
	filename := filepath.Base(path)
	if matches := strings.Split(filename, "-"); len(matches) > 1 {
		p.Version = strings.TrimSuffix(matches[len(matches)-1], ".ebuild")
	}

	// Parse metadata using precompiled regex
	if matches := portageVersionRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		p.Version = matches[1]
	}

	// Parse dependencies using new EbuildParser
	parser := NewEbuildParser(string(content))
	parsedDeps, err := parser.ParseDependencies()
	if err == nil {
		// Convert ParsedDependency to Constraint, preserving OrGroupID
		// Skip blockers - they are conflicts, not dependencies
		realDepsCount := 0
		for _, pd := range parsedDeps {
			if pd.IsBlocker {
				// TODO: Add to p.Conflicts instead of p.Deps
				log.Printf("Skipping blocker: %s for %s", pd.Constraint.Name, name)
				continue
			}
			constraint := pd.Constraint
			constraint.OrGroupID = pd.OrGroupID // Copy OrGroupID
			p.Deps = append(p.Deps, constraint)
			realDepsCount++
		}
		if realDepsCount > 0 {
			log.Printf("Parsed %d dependencies for %s (%d blockers skipped)",
				realDepsCount, name, len(parsedDeps)-realDepsCount)
		}
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
			log.Printf("Warning: failed to parse %s: %v", file.Name(), err)
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
