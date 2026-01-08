// Package solver provides SAT-based dependency resolution for GRPM.
//
// This file implements slot collision detection and resolution, inspired by
// Portage's slot_collision.py. It detects conflicts between packages that
// cannot be installed simultaneously and suggests resolutions.
package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// SlotCollision represents a detected slot conflict in the dependency graph.
// A slot collision occurs when multiple package versions are pulled into
// the same slot, which is not allowed by Portage semantics.
type SlotCollision struct {
	// SlotAtom is the slot being contested (e.g., "sys-libs/zlib:0")
	SlotAtom string

	// Packages contains all packages pulled into this slot
	Packages []*pkg.Package

	// Parents maps each conflicting package to its parent atoms
	// (the packages/atoms that caused it to be pulled in)
	Parents map[string][]ParentAtom

	// CollisionType indicates the nature of the conflict
	CollisionType SlotCollisionType

	// IsVersionConflict indicates if this is caused by incompatible version requirements
	IsVersionConflict bool

	// IsUnspecific indicates if the conflict is caused by unspecific atoms
	// (e.g., "sys-libs/zlib" without version constraints)
	IsUnspecific bool
}

// SlotCollisionType categorizes the type of slot collision.
type SlotCollisionType int

const (
	// CollisionTypeVersion indicates incompatible version requirements.
	// Example: one parent needs foo-1:0, another needs foo-2:0
	CollisionTypeVersion SlotCollisionType = iota

	// CollisionTypeUnspecific indicates all parents could use another package.
	// This is usually resolved by enabling --update --newuse.
	CollisionTypeUnspecific

	// CollisionTypeSpecific indicates a USE flag mismatch that might be resolvable.
	// This is the case where we can suggest USE flag changes.
	CollisionTypeSpecific

	// CollisionTypeSubslot indicates a subslot mismatch.
	// Example: one needs zlib:0/1.2, another needs zlib:0/1.3
	CollisionTypeSubslot
)

// String returns a human-readable name for the collision type.
func (ct SlotCollisionType) String() string {
	switch ct {
	case CollisionTypeVersion:
		return "version conflict"
	case CollisionTypeUnspecific:
		return "unspecific conflict"
	case CollisionTypeSpecific:
		return "USE flag conflict"
	case CollisionTypeSubslot:
		return "subslot conflict"
	default:
		return "unknown conflict"
	}
}

// ParentAtom represents a parent package and the atom that caused a dependency.
type ParentAtom struct {
	// Parent is the package requiring this dependency (nil for command-line args)
	Parent *pkg.Package

	// Atom is the dependency specification string
	Atom string

	// Constraint is the parsed constraint from the atom
	Constraint pkg.Constraint

	// IsCommandLine indicates if this came from command-line arguments
	IsCommandLine bool
}

// UseChangeSuggestion represents a suggested USE flag change to resolve a conflict.
type UseChangeSuggestion struct {
	// Package is the package that needs USE flag changes
	Package *pkg.Package

	// FlagChanges maps flag name to desired state (true=enable, false=disable)
	FlagChanges map[string]bool

	// Reason explains why this change is suggested
	Reason string
}

// String returns a human-readable representation of the USE change suggestion.
func (s *UseChangeSuggestion) String() string {
	if len(s.FlagChanges) == 0 {
		return fmt.Sprintf("%s: no changes needed", s.Package.Name)
	}

	var changes []string
	for flag, enable := range s.FlagChanges {
		if enable {
			changes = append(changes, "+"+flag)
		} else {
			changes = append(changes, "-"+flag)
		}
	}
	sort.Strings(changes)

	return fmt.Sprintf("%s (Change USE: %s)", s.Package.FullName(), strings.Join(changes, " "))
}

// CollisionSolution represents a possible solution to resolve slot collisions.
type CollisionSolution struct {
	// UseChanges contains all USE flag changes needed for this solution
	UseChanges []*UseChangeSuggestion

	// SelectedPackages maps slot atom to the selected package version
	SelectedPackages map[string]*pkg.Package

	// Description provides a human-readable explanation of the solution
	Description string
}

// SlotCollisionDetector detects slot conflicts in a dependency graph.
type SlotCollisionDetector struct {
	graph     *DependencyGraph
	useSolver *UseFlagSolver
}

// NewSlotCollisionDetector creates a new slot collision detector.
func NewSlotCollisionDetector(graph *DependencyGraph, useSolver *UseFlagSolver) *SlotCollisionDetector {
	return &SlotCollisionDetector{
		graph:     graph,
		useSolver: useSolver,
	}
}

// DetectCollisions analyzes the dependency graph for slot conflicts.
func (d *SlotCollisionDetector) DetectCollisions() []*SlotCollision {
	collisions := make([]*SlotCollision, 0)

	// Group packages by slot atom (category/name:slot)
	packagesBySlot := d.groupPackagesBySlot()

	// Check each slot group for conflicts
	for slotAtom, packages := range packagesBySlot {
		if len(packages) <= 1 {
			continue // No conflict with a single package
		}

		collision := d.analyzeSlotConflict(slotAtom, packages)
		if collision != nil {
			collisions = append(collisions, collision)
		}
	}

	return collisions
}

// groupPackagesBySlot groups all packages by their slot atom.
func (d *SlotCollisionDetector) groupPackagesBySlot() map[string][]*pkg.Package {
	result := make(map[string][]*pkg.Package)

	for _, node := range d.graph.nodes {
		p := node.Package
		slotAtom := d.getSlotAtom(p)
		result[slotAtom] = append(result[slotAtom], p)
	}

	return result
}

// getSlotAtom returns the slot atom string for a package.
// Format: "category/name:slot" (e.g., "sys-libs/zlib:0")
func (d *SlotCollisionDetector) getSlotAtom(p *pkg.Package) string {
	baseName := extractBaseName(p.Name)
	return baseName + ":" + p.Slot.Name
}

// analyzeSlotConflict analyzes a specific slot conflict and determines its type.
func (d *SlotCollisionDetector) analyzeSlotConflict(slotAtom string, packages []*pkg.Package) *SlotCollision {
	collision := &SlotCollision{
		SlotAtom: slotAtom,
		Packages: packages,
		Parents:  make(map[string][]ParentAtom),
	}

	// Collect parent atoms for each package
	for _, p := range packages {
		collision.Parents[p.FullName()] = d.collectParentAtoms(p)
	}

	// Determine collision type
	collision.CollisionType = d.determineCollisionType(collision)
	collision.IsVersionConflict = collision.CollisionType == CollisionTypeVersion
	collision.IsUnspecific = collision.CollisionType == CollisionTypeUnspecific

	return collision
}

// collectParentAtoms collects all parent atoms that pulled in a package.
func (d *SlotCollisionDetector) collectParentAtoms(p *pkg.Package) []ParentAtom {
	var parents []ParentAtom

	node, exists := d.graph.GetNode(p.Name)
	if !exists {
		return parents
	}

	for _, edge := range node.Dependents {
		parentNode, exists := d.graph.GetNode(edge.From)
		if !exists {
			continue
		}

		parents = append(parents, ParentAtom{
			Parent:     parentNode.Package,
			Atom:       edge.Constraint.String(),
			Constraint: edge.Constraint,
		})
	}

	// Check if this is a root package (command-line argument)
	for _, root := range d.graph.roots {
		if root == p.Name {
			parents = append(parents, ParentAtom{
				Atom:          p.Name,
				IsCommandLine: true,
			})
			break
		}
	}

	return parents
}

// determineCollisionType analyzes the collision to determine its type.
func (d *SlotCollisionDetector) determineCollisionType(collision *SlotCollision) SlotCollisionType {
	// Check for subslot conflicts first
	hasSubslotConflict := d.checkSubslotConflict(collision.Packages)
	if hasSubslotConflict {
		return CollisionTypeSubslot
	}

	// Check for version-based conflicts
	hasVersionConflict := d.checkVersionConflict(collision)
	if hasVersionConflict {
		return CollisionTypeVersion
	}

	// Check if conflict is unspecific (all atoms could use any version)
	if d.checkUnspecificConflict(collision) {
		return CollisionTypeUnspecific
	}

	// Otherwise it's a specific USE-based conflict
	return CollisionTypeSpecific
}

// checkSubslotConflict checks if packages have different subslots.
func (d *SlotCollisionDetector) checkSubslotConflict(packages []*pkg.Package) bool {
	if len(packages) < 2 {
		return false
	}

	baseSubslot := packages[0].Slot.Subslot
	for _, p := range packages[1:] {
		if p.Slot.Subslot != baseSubslot && p.Slot.Subslot != "" && baseSubslot != "" {
			return true
		}
	}

	return false
}

// checkVersionConflict checks if the conflict is caused by incompatible version requirements.
func (d *SlotCollisionDetector) checkVersionConflict(collision *SlotCollision) bool {
	// For each pair of packages, check if any parent's constraint
	// explicitly excludes the other package
	for i, pkg1 := range collision.Packages {
		for _, pkg2 := range collision.Packages[i+1:] {
			// Check if parents of pkg1 exclude pkg2
			for _, parent := range collision.Parents[pkg1.FullName()] {
				if parent.Constraint.Version != nil {
					if !parent.Constraint.Version.Satisfies(pkg2.Version) {
						return true
					}
				}
			}
			// Check if parents of pkg2 exclude pkg1
			for _, parent := range collision.Parents[pkg2.FullName()] {
				if parent.Constraint.Version != nil {
					if !parent.Constraint.Version.Satisfies(pkg1.Version) {
						return true
					}
				}
			}
		}
	}

	return false
}

// checkUnspecificConflict checks if all parent atoms are unspecific.
func (d *SlotCollisionDetector) checkUnspecificConflict(collision *SlotCollision) bool {
	// An unspecific conflict occurs when all atoms could match any of the conflicting packages
	for _, parents := range collision.Parents {
		for _, parent := range parents {
			// If any constraint has specific version requirements, it's not unspecific
			if parent.Constraint.Version != nil {
				return false
			}
			// If any constraint has USE flag conditions, it's not unspecific
			if parent.Constraint.Condition != "" {
				return false
			}
		}
	}

	return true
}

// CollisionResolver attempts to find solutions for slot collisions.
type CollisionResolver struct {
	detector          *SlotCollisionDetector
	useSolver         *UseFlagSolver
	maxConfigurations int
}

// NewCollisionResolver creates a new collision resolver.
func NewCollisionResolver(detector *SlotCollisionDetector, useSolver *UseFlagSolver) *CollisionResolver {
	return &CollisionResolver{
		detector:          detector,
		useSolver:         useSolver,
		maxConfigurations: 1024, // Same limit as Portage
	}
}

// ResolveCollision attempts to find solutions for a slot collision.
func (r *CollisionResolver) ResolveCollision(collision *SlotCollision) []*CollisionSolution {
	// Version conflicts cannot be resolved with USE flag changes
	if collision.IsVersionConflict {
		return nil
	}

	// Generate configurations (each configuration picks one package from each collision)
	solutions := make([]*CollisionSolution, 0)
	configurations := r.generateConfigurations(collision)

	for _, config := range configurations {
		solution := r.checkConfiguration(collision, config)
		if solution != nil {
			solutions = append(solutions, solution)

			// If first configuration (all-ebuild) has solution, use it
			if len(solutions) == 1 {
				break
			}
		}
	}

	return solutions
}

// generateConfigurations generates all possible configurations for a collision.
// A configuration picks exactly one package version from the conflicting set.
func (r *CollisionResolver) generateConfigurations(collision *SlotCollision) [][]*pkg.Package {
	// For simplicity, we generate single-package configurations
	// (picking which package version to keep)
	var configs [][]*pkg.Package

	for _, p := range collision.Packages {
		configs = append(configs, []*pkg.Package{p})
	}

	return configs
}

// checkConfiguration checks if a configuration can resolve the collision.
func (r *CollisionResolver) checkConfiguration(collision *SlotCollision, config []*pkg.Package) *CollisionSolution {
	if len(config) == 0 {
		return nil
	}

	selectedPkg := config[0]
	solution := &CollisionSolution{
		UseChanges:       make([]*UseChangeSuggestion, 0),
		SelectedPackages: make(map[string]*pkg.Package),
	}

	solution.SelectedPackages[collision.SlotAtom] = selectedPkg

	// Check what USE flag changes are needed for all parents to accept this package
	useChanges := r.computeRequiredUseChanges(collision, selectedPkg)
	if useChanges == nil {
		return nil // No valid solution
	}

	solution.UseChanges = useChanges
	solution.Description = fmt.Sprintf("Use %s and apply USE changes", selectedPkg.FullName())

	return solution
}

// computeRequiredUseChanges computes USE flag changes needed for all parents.
func (r *CollisionResolver) computeRequiredUseChanges(collision *SlotCollision, selected *pkg.Package) []*UseChangeSuggestion {
	var suggestions []*UseChangeSuggestion

	// For each other package in the collision, determine what changes its parents need
	for _, p := range collision.Packages {
		if p.FullName() == selected.FullName() {
			continue
		}

		for _, parent := range collision.Parents[p.FullName()] {
			if parent.Parent == nil {
				continue // Skip command-line arguments
			}

			// Check if this parent's constraint has USE conditions
			if parent.Constraint.Condition == "" {
				continue
			}

			// Determine if we can adjust USE flags to make this parent accept selected
			suggestion := r.computeUseFlagChange(parent, selected)
			if suggestion != nil {
				suggestions = append(suggestions, suggestion)
			}
		}
	}

	return suggestions
}

// computeUseFlagChange determines USE flag changes for a parent to accept a package.
func (r *CollisionResolver) computeUseFlagChange(parent ParentAtom, target *pkg.Package) *UseChangeSuggestion {
	if parent.Constraint.Flag == "" {
		return nil
	}

	// Determine if flag needs to be enabled or disabled
	flagName := parent.Constraint.Flag
	needEnabled := !strings.HasPrefix(flagName, "-")
	flagName = strings.TrimPrefix(flagName, "-")

	// Check if parent package has this flag
	if parent.Parent.UseFlags == nil {
		return nil
	}

	currentEnabled, hasFlag := parent.Parent.UseFlags[flagName]
	if !hasFlag {
		return nil // Flag not available on parent
	}

	// If current state matches needed state, no change required
	if currentEnabled == needEnabled {
		return nil
	}

	return &UseChangeSuggestion{
		Package: parent.Parent,
		FlagChanges: map[string]bool{
			flagName: needEnabled,
		},
		Reason: fmt.Sprintf("Required to use %s", target.FullName()),
	}
}

// GenerateConflictReport generates a human-readable report of slot collisions.
func GenerateConflictReport(collisions []*SlotCollision) string {
	if len(collisions) == 0 {
		return "No slot conflicts detected."
	}

	var sb strings.Builder

	sb.WriteString("\n!!! Multiple package instances within a single package slot have been pulled\n")
	sb.WriteString("!!! into the dependency graph, resulting in a slot conflict:\n\n")

	for _, collision := range collisions {
		sb.WriteString(fmt.Sprintf("%s\n\n", collision.SlotAtom))

		for _, p := range collision.Packages {
			sb.WriteString(fmt.Sprintf("  %s", p.FullName()))

			// Show USE flags if any
			if len(p.UseFlags) > 0 {
				var flags []string
				for flag, enabled := range p.UseFlags {
					if enabled {
						flags = append(flags, flag)
					} else {
						flags = append(flags, "-"+flag)
					}
				}
				sort.Strings(flags)
				sb.WriteString(fmt.Sprintf(" USE=\"%s\"", strings.Join(flags, " ")))
			}
			sb.WriteString("\n")

			// Show parents
			parents := collision.Parents[p.FullName()]
			if len(parents) > 0 {
				sb.WriteString("    pulled in by\n")
				for _, parent := range parents {
					if parent.IsCommandLine {
						sb.WriteString(fmt.Sprintf("      %s (Argument)\n", parent.Atom))
					} else if parent.Parent != nil {
						sb.WriteString(fmt.Sprintf("      %s required by %s\n",
							parent.Atom, parent.Parent.FullName()))
					}
				}
			} else {
				sb.WriteString("    (no parents)\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// GenerateSolutionReport generates a human-readable report of collision solutions.
func GenerateSolutionReport(solutions []*CollisionSolution) string {
	if len(solutions) == 0 {
		return "No solutions found for slot conflicts."
	}

	var sb strings.Builder

	if len(solutions) == 1 {
		sb.WriteString("It might be possible to solve this slot collision\n")
		sb.WriteString("by applying all of the following changes:\n\n")
	} else {
		sb.WriteString("It might be possible to solve this slot collision\n")
		sb.WriteString("by applying one of the following solutions:\n\n")
	}

	for i, solution := range solutions {
		if len(solutions) > 1 {
			sb.WriteString(fmt.Sprintf("  Solution %d:\n", i+1))
		}

		if len(solution.UseChanges) == 0 {
			sb.WriteString("    No USE changes required.\n")
		} else {
			for _, change := range solution.UseChanges {
				sb.WriteString(fmt.Sprintf("    - %s\n", change.String()))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
