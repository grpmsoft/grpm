// Package solver implements dependency cleanup (depclean) for GRPM.
//
// Depclean identifies and removes orphaned packages - packages that are:
//   - Not in @world set (user-selected packages)
//   - Not required by any @world package as a dependency
//
// This implements Portage's `emerge --depclean` functionality.
//
// Algorithm:
//  1. Build reverse dependency graph from installed packages
//  2. Mark all @world packages as "needed"
//  3. Recursively mark all dependencies of @world as "needed"
//  4. Any package not marked as "needed" is orphaned
//
// Example:
//
//	calculator := solver.NewDepcleanCalculator(db, setManager)
//	result, err := calculator.Calculate()
//	for _, orphan := range result.Orphans {
//	    fmt.Println(orphan.Atom)
//	}
package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/state"
)

// ReverseDependencyGraph maps packages to their dependents.
//
// For each package P, the graph stores all packages that depend on P.
// This is the inverse of the normal dependency graph.
//
// Example:
//
//	If sys-libs/glibc is a dependency of app-misc/hello,
//	then graph["sys-libs/glibc"] contains "app-misc/hello"
type ReverseDependencyGraph struct {
	// dependents maps package atom -> list of atoms that depend on it
	dependents map[string][]string

	// dependencies maps package atom -> list of atoms it depends on
	dependencies map[string][]string

	// packages maps atom -> installed package info
	packages map[string]*state.InstalledPackage
}

// NewReverseDependencyGraph creates a new empty reverse dependency graph.
func NewReverseDependencyGraph() *ReverseDependencyGraph {
	return &ReverseDependencyGraph{
		dependents:   make(map[string][]string),
		dependencies: make(map[string][]string),
		packages:     make(map[string]*state.InstalledPackage),
	}
}

// AddPackage adds an installed package to the graph.
//
// The package's dependencies are parsed and added to the reverse mapping.
func (g *ReverseDependencyGraph) AddPackage(pkg *state.InstalledPackage) {
	if pkg == nil || pkg.Package == nil {
		return
	}

	atom := extractPackageAtom(pkg.Package.Name, pkg.Package.Version)
	g.packages[atom] = pkg

	// Initialize dependents list for this package
	if _, exists := g.dependents[atom]; !exists {
		g.dependents[atom] = make([]string, 0)
	}

	// Parse and add dependencies
	deps := g.extractDependencyAtoms(pkg)
	g.dependencies[atom] = deps

	// Update reverse mapping
	for _, dep := range deps {
		g.dependents[dep] = append(g.dependents[dep], atom)
	}
}

// extractDependencyAtoms extracts dependency atoms from an installed package.
func (g *ReverseDependencyGraph) extractDependencyAtoms(pkg *state.InstalledPackage) []string {
	if pkg.Package == nil {
		return nil
	}

	atoms := make([]string, 0, len(pkg.Package.Deps))
	seen := make(map[string]bool)

	for _, dep := range pkg.Package.Deps {
		// Extract base atom (category/name) without version
		atom := extractBaseAtom(dep.Name)
		if atom != "" && !seen[atom] {
			seen[atom] = true
			atoms = append(atoms, atom)
		}
	}

	return atoms
}

// GetDependents returns all packages that depend on the given atom.
func (g *ReverseDependencyGraph) GetDependents(atom string) []string {
	baseAtom := extractBaseAtom(atom)
	if deps, exists := g.dependents[baseAtom]; exists {
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// GetDependencies returns all dependencies of the given atom.
func (g *ReverseDependencyGraph) GetDependencies(atom string) []string {
	baseAtom := extractBaseAtom(atom)
	if deps, exists := g.dependencies[baseAtom]; exists {
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// HasDependents returns true if any package depends on the given atom.
func (g *ReverseDependencyGraph) HasDependents(atom string) bool {
	baseAtom := extractBaseAtom(atom)
	deps, exists := g.dependents[baseAtom]
	return exists && len(deps) > 0
}

// PackageCount returns the number of packages in the graph.
func (g *ReverseDependencyGraph) PackageCount() int {
	return len(g.packages)
}

// GetPackage returns the installed package info for the given atom.
func (g *ReverseDependencyGraph) GetPackage(atom string) *state.InstalledPackage {
	baseAtom := extractBaseAtom(atom)
	return g.packages[baseAtom]
}

// AllAtoms returns all package atoms in the graph.
func (g *ReverseDependencyGraph) AllAtoms() []string {
	atoms := make([]string, 0, len(g.packages))
	for atom := range g.packages {
		atoms = append(atoms, atom)
	}
	sort.Strings(atoms)
	return atoms
}

// OrphanInfo contains information about an orphaned package.
type OrphanInfo struct {
	// Atom is the package atom (category/name).
	Atom string

	// Version is the installed version.
	Version string

	// Slot is the package slot.
	Slot string

	// Size is the installed size in bytes.
	Size int64

	// Reason explains why the package is orphaned.
	Reason OrphanReason
}

// OrphanReason explains why a package is considered orphaned.
type OrphanReason int

const (
	// OrphanReasonNotInWorld - package is not in @world set.
	OrphanReasonNotInWorld OrphanReason = iota

	// OrphanReasonNotRequired - package is not required by any @world package.
	OrphanReasonNotRequired

	// OrphanReasonUnused - package was a dependency but is no longer needed.
	OrphanReasonUnused
)

// String returns a human-readable reason.
func (r OrphanReason) String() string {
	switch r {
	case OrphanReasonNotInWorld:
		return "not in @world"
	case OrphanReasonNotRequired:
		return "not required by @world"
	case OrphanReasonUnused:
		return "no longer needed"
	default:
		return "unknown"
	}
}

// DepcleanResult contains the result of depclean calculation.
type DepcleanResult struct {
	// Orphans is the list of orphaned packages to remove.
	Orphans []*OrphanInfo

	// Protected is the list of protected packages (in @world or required).
	Protected []string

	// TotalSize is the total size of orphaned packages in bytes.
	TotalSize int64

	// Graph is the reverse dependency graph used for calculation.
	Graph *ReverseDependencyGraph
}

// DepcleanOptions contains options for depclean calculation.
type DepcleanOptions struct {
	// Exclude is a list of package atoms to exclude from removal.
	Exclude []string

	// IncludeSystem includes @system packages in protection.
	IncludeSystem bool

	// Verbose enables verbose output during calculation.
	Verbose bool
}

// DefaultDepcleanOptions returns the default depclean options.
func DefaultDepcleanOptions() *DepcleanOptions {
	return &DepcleanOptions{
		Exclude:       []string{},
		IncludeSystem: true,
		Verbose:       false,
	}
}

// DepcleanCalculator calculates orphaned packages for removal.
type DepcleanCalculator struct {
	db         *state.PackageDatabase
	setManager *state.SetManager
	opts       *DepcleanOptions
}

// NewDepcleanCalculator creates a new depclean calculator.
//
// Parameters:
//   - db: Package database with installed packages
//   - setManager: Set manager for @world, @selected, @system
func NewDepcleanCalculator(db *state.PackageDatabase, setManager *state.SetManager) *DepcleanCalculator {
	return &DepcleanCalculator{
		db:         db,
		setManager: setManager,
		opts:       DefaultDepcleanOptions(),
	}
}

// SetOptions sets the depclean options.
func (c *DepcleanCalculator) SetOptions(opts *DepcleanOptions) {
	if opts != nil {
		c.opts = opts
	}
}

// Calculate performs the depclean calculation and returns orphaned packages.
//
// Algorithm:
//  1. Load @world set (or @selected + @system if IncludeSystem)
//  2. Build reverse dependency graph from installed packages
//  3. Mark all @world packages as "needed"
//  4. Recursively mark all dependencies of @world as "needed"
//  5. Any package not marked as "needed" is orphaned
func (c *DepcleanCalculator) Calculate() (*DepcleanResult, error) {
	if c.db == nil {
		return nil, fmt.Errorf("package database is nil")
	}

	// Step 1: Load @world set
	worldSet, err := c.loadWorldSet()
	if err != nil {
		return nil, fmt.Errorf("failed to load @world set: %w", err)
	}

	// Step 2: Build reverse dependency graph
	graph := c.buildGraph()

	// Step 3-4: Mark needed packages
	needed := c.markNeededPackages(worldSet, graph)

	// Step 5: Find orphaned packages
	result := c.findOrphans(graph, needed, worldSet)

	return result, nil
}

// loadWorldSet loads the @world package set.
func (c *DepcleanCalculator) loadWorldSet() (*state.PackageSet, error) {
	if c.setManager == nil {
		// Return empty set if no set manager
		return state.NewPackageSet(state.SetWorld, []string{}), nil
	}

	if c.opts.IncludeSystem {
		return c.setManager.GetWorld()
	}
	return c.setManager.GetSelected()
}

// buildGraph builds the reverse dependency graph from installed packages.
func (c *DepcleanCalculator) buildGraph() *ReverseDependencyGraph {
	graph := NewReverseDependencyGraph()

	for _, pkg := range c.db.List() {
		graph.AddPackage(pkg)
	}

	return graph
}

// markNeededPackages marks all packages needed by @world.
//
// Uses BFS to traverse dependencies starting from @world packages.
func (c *DepcleanCalculator) markNeededPackages(worldSet *state.PackageSet, graph *ReverseDependencyGraph) map[string]bool {
	needed := make(map[string]bool)

	// Add excluded packages to needed set
	for _, exclude := range c.opts.Exclude {
		baseAtom := extractBaseAtom(exclude)
		needed[baseAtom] = true
	}

	// Mark @world packages as needed
	for _, atom := range worldSet.Atoms() {
		baseAtom := extractBaseAtom(atom)
		needed[baseAtom] = true
	}

	// BFS to mark all dependencies as needed
	queue := make([]string, 0, len(needed))
	for atom := range needed {
		queue = append(queue, atom)
	}

	processed := make(map[string]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if processed[current] {
			continue
		}
		processed[current] = true

		// Get dependencies of current package
		deps := graph.GetDependencies(current)
		for _, dep := range deps {
			baseAtom := extractBaseAtom(dep)
			if !needed[baseAtom] {
				needed[baseAtom] = true
				queue = append(queue, baseAtom)
			}
		}
	}

	return needed
}

// findOrphans finds all packages not in the needed set.
func (c *DepcleanCalculator) findOrphans(graph *ReverseDependencyGraph, needed map[string]bool, worldSet *state.PackageSet) *DepcleanResult {
	result := &DepcleanResult{
		Orphans:   make([]*OrphanInfo, 0),
		Protected: make([]string, 0),
		Graph:     graph,
	}

	allAtoms := graph.AllAtoms()

	for _, atom := range allAtoms {
		if needed[atom] {
			result.Protected = append(result.Protected, atom)
			continue
		}

		// Package is orphaned
		pkg := graph.GetPackage(atom)
		if pkg == nil {
			continue
		}

		reason := c.determineOrphanReason(atom, worldSet, graph)

		orphan := &OrphanInfo{
			Atom:    atom,
			Version: pkg.Package.Version,
			Slot:    pkg.Package.Slot.Name,
			Size:    pkg.Size,
			Reason:  reason,
		}

		result.Orphans = append(result.Orphans, orphan)
		result.TotalSize += pkg.Size
	}

	// Sort orphans by atom for consistent output
	sort.Slice(result.Orphans, func(i, j int) bool {
		return result.Orphans[i].Atom < result.Orphans[j].Atom
	})

	return result
}

// determineOrphanReason determines why a package is orphaned.
func (c *DepcleanCalculator) determineOrphanReason(atom string, worldSet *state.PackageSet, graph *ReverseDependencyGraph) OrphanReason {
	// Check if package was ever in @world
	if worldSet.Contains(atom) {
		// Should not happen if calculation is correct
		return OrphanReasonUnused
	}

	// Check if package has any dependents
	if graph.HasDependents(atom) {
		// Was a dependency, but dependents are not needed
		return OrphanReasonUnused
	}

	// Package was never required
	return OrphanReasonNotRequired
}

// FormatResult formats the depclean result for display.
func FormatDepcleanResult(result *DepcleanResult, pretend bool) string {
	var sb strings.Builder

	if len(result.Orphans) == 0 {
		sb.WriteString("No orphaned packages found.\n")
		sb.WriteString(fmt.Sprintf("Protected packages: %d\n", len(result.Protected)))
		return sb.String()
	}

	if pretend {
		sb.WriteString("\n*** Depclean analysis (--pretend mode):\n")
		sb.WriteString("*** The following packages would be removed:\n\n")
	} else {
		sb.WriteString("\n*** Packages to be removed:\n\n")
	}

	for _, orphan := range result.Orphans {
		sb.WriteString(fmt.Sprintf("[uninstall   ] %s-%s (%s)\n",
			orphan.Atom, orphan.Version, orphan.Reason))
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d package(s)\n", len(result.Orphans)))
	sb.WriteString(fmt.Sprintf("Space to be freed: %s\n", formatBytes(result.TotalSize)))

	return sb.String()
}

// extractPackageAtom extracts the base atom from package name and version.
//
// Example: "sys-libs/zlib", "1.2.13" -> "sys-libs/zlib"
func extractPackageAtom(name, version string) string {
	// If name already contains version, extract just category/name
	return extractBaseAtom(name)
}

// extractBaseAtom extracts the category/name from a full package specification.
//
// Handles various formats:
//   - "sys-libs/zlib" -> "sys-libs/zlib"
//   - "sys-libs/zlib-1.2.13" -> "sys-libs/zlib"
//   - ">=sys-libs/zlib-1.2" -> "sys-libs/zlib"
func extractBaseAtom(fullName string) string {
	// Remove version operators
	atom := fullName
	for _, prefix := range []string{">=", "<=", ">", "<", "=", "~"} {
		atom = strings.TrimPrefix(atom, prefix)
	}

	// Remove slot suffix
	if idx := strings.Index(atom, ":"); idx != -1 {
		atom = atom[:idx]
	}

	// Remove USE flag suffix
	if idx := strings.Index(atom, "["); idx != -1 {
		atom = atom[:idx]
	}

	// Find version separator (last hyphen followed by digit)
	for i := len(atom) - 1; i >= 0; i-- {
		if atom[i] == '-' && i+1 < len(atom) {
			nextChar := atom[i+1]
			if nextChar >= '0' && nextChar <= '9' {
				return atom[:i]
			}
		}
	}

	return atom
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
