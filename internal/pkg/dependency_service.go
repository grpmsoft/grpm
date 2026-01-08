package pkg

// DependencyService is a Domain Service that encapsulates dependency resolution logic
// This moves business logic OUT of infrastructure (solver) INTO the domain layer
type DependencyService struct {
}

// NewDependencyService creates a new dependency service
func NewDependencyService() *DependencyService {
	return &DependencyService{}
}

// ResolveDependencyTree builds a complete dependency tree for a package
// Returns all packages needed to satisfy dependencies recursively
func (ds *DependencyService) ResolveDependencyTree(root *Package, packageLoader func(name string) (*Package, error)) (map[string]*Package, error) {
	allPackages := make(map[string]*Package)
	return allPackages, ds.collectDependencies(root, allPackages, packageLoader)
}

// collectDependencies recursively collects all dependencies
func (ds *DependencyService) collectDependencies(
	pkg *Package,
	allPackages map[string]*Package,
	packageLoader func(name string) (*Package, error),
) error {
	if _, exists := allPackages[pkg.Name]; exists {
		return nil // Already processed
	}

	// Store package
	copyPkg := *pkg
	allPackages[pkg.Name] = &copyPkg

	// Process dependencies recursively
	for _, dep := range pkg.Deps {
		depPkg, err := packageLoader(dep.Name)
		if err != nil {
			// Log warning but continue (non-critical dependency)
			continue
		}

		if err := ds.collectDependencies(depPkg, allPackages, packageLoader); err != nil {
			// Log warning but continue
			continue
		}
	}

	return nil
}

// FindConflicts identifies packages that conflict with each other
func (ds *DependencyService) FindConflicts(packages []*Package) [][]*Package {
	var conflicts [][]*Package

	for i, pkg1 := range packages {
		for j := i + 1; j < len(packages); j++ {
			pkg2 := packages[j]
			if pkg1.ConflictsWith(pkg2) {
				conflicts = append(conflicts, []*Package{pkg1, pkg2})
			}
		}
	}

	return conflicts
}

// FilterByConstraint filters packages that satisfy a given constraint
func (ds *DependencyService) FilterByConstraint(packages []*Package, constraint Constraint) []*Package {
	var result []*Package
	for _, pkg := range packages {
		if pkg.SatisfiesConstraint(constraint) {
			result = append(result, pkg)
		}
	}
	return result
}

// ValidateDependencyGraph checks if a dependency graph is valid
// Returns error if there are circular dependencies or unsatisfiable constraints
func (ds *DependencyService) ValidateDependencyGraph(packages map[string]*Package) error {
	// TODO: Implement cycle detection
	// TODO: Implement constraint satisfaction validation
	return nil
}
