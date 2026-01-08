package solver

import (
	"fmt"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ConflictType categorizes different types of package conflicts
type ConflictType int

const (
	ConflictTypeSlot    ConflictType = iota // Multiple versions in same slot
	ConflictTypeVersion                     // Incompatible version requirements
	ConflictTypeUseFlag                     // USE flag incompatibility (future)
)

// String returns a human-readable name for the conflict type
func (ct ConflictType) String() string {
	switch ct {
	case ConflictTypeSlot:
		return "slot conflict"
	case ConflictTypeVersion:
		return "version conflict"
	case ConflictTypeUseFlag:
		return "USE flag conflict"
	default:
		return "unknown conflict"
	}
}

// ConflictError represents a package installation conflict
type ConflictError struct {
	Type     ConflictType // Type of conflict
	Packages []string     // Package names involved
	Details  string       // Human-readable explanation
	Severity Severity     // How critical this conflict is
}

// Severity indicates how critical a conflict is
type Severity int

const (
	SeverityWarning  Severity = iota // Can be ignored or worked around
	SeverityError                    // Prevents installation but may have solutions
	SeverityCritical                 // Fundamental incompatibility, no solution
)

// String returns a human-readable severity level
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Error implements the error interface
func (c *ConflictError) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s (%s): ", c.Type, c.Severity))

	if len(c.Packages) > 0 {
		sb.WriteString(strings.Join(c.Packages, ", "))
		sb.WriteString(" - ")
	}

	sb.WriteString(c.Details)

	return sb.String()
}

// DetectConflicts analyzes the dependency graph for all types of conflicts
// Returns a slice of detected conflicts (empty if no conflicts)
func (g *DependencyGraph) DetectConflicts() []*ConflictError {
	conflicts := make([]*ConflictError, 0)

	// Check for slot conflicts
	conflicts = append(conflicts, g.detectSlotConflicts()...)

	// Check for version conflicts
	conflicts = append(conflicts, g.detectVersionConflicts()...)

	// TODO: Add blocker conflict detection when blocker constraints are implemented

	return conflicts
}

// detectSlotConflicts finds packages that conflict on slot usage
func (g *DependencyGraph) detectSlotConflicts() []*ConflictError {
	conflicts := make([]*ConflictError, 0)

	// Group packages by category/name (without version)
	packagesByName := make(map[string][]*GraphNode)
	for name, node := range g.nodes {
		// Extract base name (category/name without version)
		baseName := extractBaseName(name)
		packagesByName[baseName] = append(packagesByName[baseName], node)
	}

	// Check each group for slot conflicts
	for baseName, nodes := range packagesByName {
		if len(nodes) <= 1 {
			continue // No conflict possible with single package
		}

		// Check if multiple versions in same slot
		slotMap := make(map[string][]string) // slot -> package names
		for _, node := range nodes {
			slotName := node.Package.Slot.Name
			slotMap[slotName] = append(slotMap[slotName], node.Package.Name)
		}

		// If same slot has multiple packages, it's a conflict
		for slotName, pkgs := range slotMap {
			if len(pkgs) > 1 {
				conflict := &ConflictError{
					Type:     ConflictTypeSlot,
					Packages: pkgs,
					Details: fmt.Sprintf(
						"%s has multiple versions in slot '%s': %s",
						baseName, slotName, strings.Join(pkgs, ", "),
					),
					Severity: SeverityError,
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// detectVersionConflicts finds incompatible version requirements
func (g *DependencyGraph) detectVersionConflicts() []*ConflictError {
	conflicts := make([]*ConflictError, 0)

	// For each package, collect all version constraints from dependents
	for pkgName, node := range g.nodes {
		if len(node.Dependents) <= 1 {
			continue // Need multiple dependents for conflict
		}

		// Collect all version constraints
		constraints := make([]*pkg.VersionConstraint, 0)
		dependentNames := make([]string, 0)

		for _, edge := range node.Dependents {
			if edge.Constraint.Version != nil {
				constraints = append(constraints, edge.Constraint.Version)
				dependentNames = append(dependentNames, edge.From)
			}
		}

		if len(constraints) <= 1 {
			continue // Need multiple constraints for conflict
		}

		// Check if current version satisfies all constraints
		currentVersion := node.Package.Version
		satisfied := true
		for _, constraint := range constraints {
			if !satisfiesVersionConstraint(currentVersion, constraint) {
				satisfied = false
				break
			}
		}

		if !satisfied {
			conflict := &ConflictError{
				Type:     ConflictTypeVersion,
				Packages: append([]string{pkgName}, dependentNames...),
				Details: fmt.Sprintf(
					"%s version %s does not satisfy all dependent constraints",
					pkgName, currentVersion,
				),
				Severity: SeverityError,
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// extractBaseName extracts category/name from a full package name
// Example: "sys-libs/zlib-1.2.13" -> "sys-libs/zlib"
func extractBaseName(fullName string) string {
	// Find last occurrence of version separator (usually dash followed by number)
	// For simplicity, assume package name format: "category/name-version"
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return fullName // Invalid format, return as-is
	}

	category := parts[0]
	nameVersion := parts[1]

	// Find last dash before version
	lastDash := -1
	for i := len(nameVersion) - 1; i >= 0; i-- {
		if nameVersion[i] == '-' {
			// Check if next char is digit (version indicator)
			if i+1 < len(nameVersion) && nameVersion[i+1] >= '0' && nameVersion[i+1] <= '9' {
				lastDash = i
				break
			}
		}
	}

	if lastDash != -1 {
		return category + "/" + nameVersion[:lastDash]
	}

	return fullName // No version found, return as-is
}

// satisfiesVersionConstraint checks if a version satisfies a constraint
func satisfiesVersionConstraint(version string, constraint *pkg.VersionConstraint) bool {
	if constraint == nil {
		return true // No constraint means always satisfied
	}

	// Use the Satisfies method from VersionConstraint
	return constraint.Satisfies(version)
}

// HasConflicts is a convenience method that returns true if any conflicts exist
func (g *DependencyGraph) HasConflicts() bool {
	return len(g.DetectConflicts()) > 0
}

// GetConflictsByType returns conflicts of a specific type
func (g *DependencyGraph) GetConflictsByType(conflictType ConflictType) []*ConflictError {
	all := g.DetectConflicts()
	filtered := make([]*ConflictError, 0)

	for _, conflict := range all {
		if conflict.Type == conflictType {
			filtered = append(filtered, conflict)
		}
	}

	return filtered
}

// GetConflictsBySeverity returns conflicts at or above a severity level
func (g *DependencyGraph) GetConflictsBySeverity(minSeverity Severity) []*ConflictError {
	all := g.DetectConflicts()
	filtered := make([]*ConflictError, 0)

	for _, conflict := range all {
		if conflict.Severity >= minSeverity {
			filtered = append(filtered, conflict)
		}
	}

	return filtered
}

// ConflictReport generates a human-readable report of all conflicts
func (g *DependencyGraph) ConflictReport() string {
	conflicts := g.DetectConflicts()

	if len(conflicts) == 0 {
		return "No conflicts detected"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Detected %d conflicts:\n\n", len(conflicts)))

	// Group by severity
	bySeverity := make(map[Severity][]*ConflictError)
	for _, conflict := range conflicts {
		bySeverity[conflict.Severity] = append(bySeverity[conflict.Severity], conflict)
	}

	// Report in severity order (critical -> error -> warning)
	severityOrder := []Severity{SeverityCritical, SeverityError, SeverityWarning}
	for _, severity := range severityOrder {
		conflicts := bySeverity[severity]
		if len(conflicts) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("=== %s (%d) ===\n", strings.ToUpper(severity.String()), len(conflicts)))
		for i, conflict := range conflicts {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, conflict.Error()))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
