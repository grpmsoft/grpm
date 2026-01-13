package solver

import (
	"fmt"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

type PortageResolver struct {
	repo repo.Repository
}

func NewResolver(r repo.Repository) *PortageResolver {
	return &PortageResolver{repo: r}
}

// groupDependenciesByOrGroupID groups dependencies by their OrGroupID
// Returns required dependencies (OrGroupID=0) and OR-groups (OrGroupID>0)
func groupDependenciesByOrGroupID(deps []pkg.Constraint) (requiredDeps []pkg.Constraint, orGroups map[int][]pkg.Constraint) {
	orGroups = make(map[int][]pkg.Constraint)
	for _, dep := range deps {
		if dep.OrGroupID == 0 {
			requiredDeps = append(requiredDeps, dep)
		} else {
			orGroups[dep.OrGroupID] = append(orGroups[dep.OrGroupID], dep)
		}
	}
	return
}

func (r *PortageResolver) collectDependencies(p *pkg.Package, allPackages map[string]*pkg.Package) error {
	if _, exists := allPackages[p.Name]; exists {
		return nil // Already processed
	}

	// Store a copy of the package
	copyPkg := *p
	allPackages[p.Name] = &copyPkg

	// Group dependencies by OrGroupID
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	// Process REQUIRED dependencies only
	for _, dep := range requiredDeps {
		depPkg, err := r.repo.LoadPackage(dep.Name)
		if err != nil {
			logging.Debug("Warning: dependency %s for %s not found: %v", dep.Name, p.Name, err)
			continue
		}

		// Recursively collect dependencies
		if err := r.collectDependencies(depPkg, allPackages); err != nil {
			logging.Debug("Warning: %v", err)
		}
	}

	// For OR-groups: Register all alternatives but DON'T collect their dependencies yet
	// The SAT solver will choose ONE alternative from each group
	for groupID, alternatives := range orGroups {
		logging.Debug("OR-group %d for %s: %d alternatives", groupID, p.Name, len(alternatives))
		for _, alt := range alternatives {
			// Just ensure the alternative package exists in the repository
			if _, err := r.repo.LoadPackage(alt.Name); err != nil {
				logging.Debug("Warning: OR-alternative %s not found: %v", alt.Name, err)
			}
		}
	}

	return nil
}

// addPackageConstraints adds all constraints for a single package to the SAT solver
func (r *PortageResolver) addPackageConstraints(adapter *GophersatAdapter, p *pkg.Package, rootPackages []string) {
	// Add constraint that root packages must be installed
	if contains(rootPackages, p.Name) {
		versionStr := "any"
		if p.Version != "" {
			versionStr = p.Version
		}
		logging.Debug("Adding constraint for required package: %s = %s", p.Name, versionStr)

		if err := adapter.AddConstraint(pkg.Constraint{
			Type:    pkg.ConstraintTypeVersion,
			Name:    p.Name,
			Version: pkg.NewVersionConstraint(pkg.OpEqual, p.Version),
		}); err != nil {
			logging.Debug("Warning: failed to add package constraint: %v", err)
		}
	}

	// Group dependencies by OrGroupID
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	// Add REQUIRED dependencies (AND logic)
	for _, dep := range requiredDeps {
		versionStr := "any"
		if dep.Version != nil {
			versionStr = dep.Version.String()
		}
		logging.Debug("Adding required dependency: %s %s", dep.Name, versionStr)

		if err := adapter.AddConstraint(dep); err != nil {
			logging.Debug("Warning: failed to add constraint: %v", err)
		}
	}

	// Add OR-group constraints (OR logic)
	for groupID, alternatives := range orGroups {
		logging.Debug("Adding OR-group %d with %d alternatives", groupID, len(alternatives))
		if err := adapter.AddOrGroupConstraint(alternatives); err != nil {
			logging.Debug("Warning: failed to add OR-group constraint: %v", err)
		}
	}
}

// buildResultFromSolution builds the final result map from the SAT solution
func (r *PortageResolver) buildResultFromSolution(solution map[string]string) (map[string]*pkg.Package, error) {
	result := make(map[string]*pkg.Package)
	for name := range solution {
		p, err := r.repo.LoadPackage(name)
		if err != nil {
			logging.Debug("Warning: package %s not found: %v", name, err)
			continue
		}
		result[name] = p
	}
	return result, nil
}

func (r *PortageResolver) Resolve(packages []string) (map[string]*pkg.Package, error) {
	adapter := NewGophersatAdapter()
	allPackages := make(map[string]*pkg.Package)

	// Load and collect all dependencies
	for _, pkgName := range packages {
		p, err := r.repo.LoadPackage(pkgName)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", pkgName, err)
		}

		logging.Debug("Resolving package: %s-%s with %d dependencies",
			p.Name, p.Version, len(p.Deps))

		if err := r.collectDependencies(p, allPackages); err != nil {
			logging.Debug("Warning: %v", err)
		}
	}

	logging.Debug("Total packages in dependency graph: %d", len(allPackages))

	// First, add ALL packages to the solver
	for _, p := range allPackages {
		adapter.AddPackage(p)
	}

	// Then add constraints for each package
	for _, p := range allPackages {
		r.addPackageConstraints(adapter, p, packages)
	}

	logging.Debug("Total clauses in SAT problem: %d", len(adapter.clauses))

	// Solve
	status, solution, err := adapter.Solve()
	if err != nil {
		return nil, err
	}

	if status != pkg.StatusSat {
		logging.Debug("UNSAT core analysis:")
		for i, clause := range adapter.clauses {
			logging.Debug("Clause %d: %v", i, clause)
		}
		return nil, fmt.Errorf("no solution found")
	}

	// Build result
	result, err := r.buildResultFromSolution(solution)
	if err != nil {
		return nil, err
	}

	// Output formatted package list
	logging.Info("Resolved packages:")
	for name, p := range result {
		logging.Debug("- %s-%s [slot:%s]", name, p.Version, p.Slot.Name)
	}

	return result, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
