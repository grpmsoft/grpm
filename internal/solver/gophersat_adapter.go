package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/crillab/gophersat/solver"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// GophersatAdapter adapts the gophersat SAT solver for package dependency resolution
type GophersatAdapter struct {
	clauses      [][]int
	vars         map[string]int            // name@version -> var ID
	varNames     map[int]string            // var ID -> name@version
	packages     map[string][]*pkg.Package // name -> []versions
	addedClauses map[string]struct{}       // to prevent duplicate clauses
}

func NewGophersatAdapter() *GophersatAdapter {
	return &GophersatAdapter{
		vars:         make(map[string]int),
		varNames:     make(map[int]string),
		packages:     make(map[string][]*pkg.Package),
		addedClauses: make(map[string]struct{}),
	}
}

func (g *GophersatAdapter) getVarID(key string) int {
	if id, exists := g.vars[key]; exists {
		return id
	}

	// Create new variable
	id := len(g.vars) + 1
	g.vars[key] = id
	g.varNames[id] = key
	return id
}

func (g *GophersatAdapter) addClause(clause []int) {
	// Create unique key for clause (sorted)
	sortedClause := make([]int, len(clause))
	copy(sortedClause, clause)
	sort.Ints(sortedClause)
	key := fmt.Sprintf("%v", sortedClause)

	// Skip if clause already added
	if _, exists := g.addedClauses[key]; exists {
		return
	}

	g.clauses = append(g.clauses, clause)
	g.addedClauses[key] = struct{}{}
	logging.Debug("Added clause: %v", clause)
}

// AddPackage registers a package version as a SAT variable
func (g *GophersatAdapter) AddPackage(p *pkg.Package) {
	key := p.Name + "@" + p.Version

	// Register package
	if _, exists := g.packages[p.Name]; !exists {
		g.packages[p.Name] = []*pkg.Package{}
	}

	// Ensure this version hasn't been added yet
	for _, existing := range g.packages[p.Name] {
		if existing.Version == p.Version {
			return
		}
	}

	g.packages[p.Name] = append(g.packages[p.Name], p)

	// Register variable
	g.getVarID(key)

	// Logging
	logging.Debug("Added package: %s-%s", p.Name, p.Version)
}

func (g *GophersatAdapter) AddConstraint(c pkg.Constraint) error {
	switch c.Type {
	case pkg.ConstraintTypeVersion:
		return g.addVersionConstraint(c)
	case pkg.ConstraintTypeSlot:
		return g.addSlotConstraint(c)
	case pkg.ConstraintTypeUseFlag:
		return g.addUseFlagConstraint(c)
	default:
		return fmt.Errorf("unsupported constraint type: %d", c.Type)
	}
}

// AddAtomConstraint adds a constraint from a PMS-compliant Atom.
// Uses Atom.Matches() for more accurate version/slot matching.
func (g *GophersatAdapter) AddAtomConstraint(atom *pkg.Atom) error {
	if atom == nil {
		return fmt.Errorf("atom is nil")
	}

	pkgName := atom.CP()
	logging.Debug("Processing atom constraint: %s", atom.String())

	// Collect all packages that match this atom
	var satisfiedVars []int
	for _, p := range g.packages[pkgName] {
		if atom.Matches(p) {
			key := p.Name + "@" + p.Version
			varID := g.getVarID(key)
			satisfiedVars = append(satisfiedVars, varID)
			logging.Debug("Package %s satisfies atom %s", key, atom.String())
		} else {
			logging.Debug("Package %s does NOT satisfy atom %s",
				p.Name+"@"+p.Version, atom.String())
		}
	}

	if len(satisfiedVars) == 0 {
		logging.Debug("Warning: no package satisfies atom %s", atom.String())
		return nil
	}

	// Add clause: at least one of the satisfying packages must be selected
	g.addClause(satisfiedVars)
	return nil
}

// AddOrGroupConstraint adds an OR-group constraint (at-least-one alternative)
// For example: || ( mysql postgresql ) means "pick mysql OR postgresql"
func (g *GophersatAdapter) AddOrGroupConstraint(alternatives []pkg.Constraint) error {
	var allSatisfyingVars []int

	// Collect all package versions that satisfy ANY of the alternatives
	for _, alt := range alternatives {
		var altVars []int

		// For version constraints
		if alt.Version != nil {
			for _, p := range g.packages[alt.Name] {
				if alt.Version.Satisfies(p.Version) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					altVars = append(altVars, varID)
				}
			}
		} else {
			// No version constraint - any version satisfies
			for _, p := range g.packages[alt.Name] {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				altVars = append(altVars, varID)
			}
		}

		logging.Debug("  Alternative %s: %d matching versions", alt.Name, len(altVars))
		allSatisfyingVars = append(allSatisfyingVars, altVars...)
	}

	if len(allSatisfyingVars) == 0 {
		return fmt.Errorf("no packages satisfy OR-group alternatives")
	}

	// Add single clause: at-least-one from all alternatives
	logging.Debug("Adding OR-group clause with %d total options", len(allSatisfyingVars))
	g.addClause(allSatisfyingVars)
	return nil
}

func (g *GophersatAdapter) addVersionConstraint(c pkg.Constraint) error {
	logging.Debug("Processing constraint: %s %s", c.Name, c.Version)

	// For simple constraints without version
	if c.Version == nil {
		return g.addSimpleConstraint(c.Name)
	}

	// Collect all packages satisfying the constraint
	var satisfiedVars []int
	for _, p := range g.packages[c.Name] {
		if c.Version.Satisfies(p.Version) {
			key := p.Name + "@" + p.Version
			varID := g.getVarID(key)
			satisfiedVars = append(satisfiedVars, varID)
			logging.Debug("Package %s satisfies constraint %s %s", key, c.Name, c.Version.String())
		} else {
			logging.Debug("Package %s does NOT satisfy constraint %s %s",
				p.Name+"@"+p.Version, c.Name, c.Version.String())
		}
	}

	if len(satisfiedVars) == 0 {
		if c.Required {
			return fmt.Errorf("unsatisfiable: no package provides %s %s", c.Name, c.Version.String())
		}
		logging.Debug("Warning: no package satisfies %s %s (non-required, skipping)", c.Name, c.Version.String())
		return nil
	}

	// Add clause: at least one of the satisfying packages must be selected
	g.addClause(satisfiedVars)
	return nil
}

func (g *GophersatAdapter) addSimpleConstraint(name string) error {
	// Fixed: check package existence
	if versions, exists := g.packages[name]; exists && len(versions) > 0 {
		var packageVars []int
		for _, p := range versions {
			key := p.Name + "@" + p.Version
			varID := g.getVarID(key)
			packageVars = append(packageVars, varID)
		}
		g.addClause(packageVars)
		return nil
	}
	return fmt.Errorf("package %s not found in repository", name)
}

// AddExactlyOneConstraint ensures exactly one version of a package is selected
func (g *GophersatAdapter) AddExactlyOneConstraint(pkgName string, versions []string) {
	var versionVars []int
	for _, version := range versions {
		key := pkgName + "@" + version
		if varID, exists := g.vars[key]; exists {
			versionVars = append(versionVars, varID)
		}
	}

	if len(versionVars) == 0 {
		return
	}

	// For a single package - just mandatory installation
	if len(versionVars) == 1 {
		g.addClause([]int{versionVars[0]})
		logging.Debug("Added mandatory constraint for %s: [%d]", pkgName, versionVars[0])
		return
	}

	// Add clauses for "exactly one version" constraint
	for _, clause := range exactlyOne(versionVars) {
		g.addClause(clause)
	}
	logging.Debug("Added exactly-one constraint for %s: %d versions", pkgName, len(versions))
}

func (g *GophersatAdapter) addSlotConstraint(c pkg.Constraint) error {
	// Find all packages matching name AND slot
	var slotVars []int
	for _, pkgList := range g.packages {
		for _, p := range pkgList {
			if p.Name == c.Name && p.Slot.Name == c.Slot {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				slotVars = append(slotVars, varID)
			}
		}
	}

	if len(slotVars) == 0 {
		return fmt.Errorf("no package %s provides slot %s", c.Name, c.Slot)
	}

	// Add clause: at least one package in this slot must be installed
	g.addClause(slotVars)
	return nil
}

func (g *GophersatAdapter) addUseFlagConstraint(c pkg.Constraint) error {
	if c.Required {
		// Create variable for USE flag
		flagVar := g.getVarID("USE_" + c.Flag)
		g.addClause([]int{flagVar})
	}
	return nil
}

// exactlyOne generates clauses ensuring exactly one variable from the list is true
func exactlyOne(vars []int) [][]int {
	// Add clause: at least one is true
	clause := make([]int, len(vars))
	copy(clause, vars)
	clauses := [][]int{clause}

	// Add pairwise negations: at most one is true
	for i := 0; i < len(vars); i++ {
		for j := i + 1; j < len(vars); j++ {
			clauses = append(clauses, []int{-vars[i], -vars[j]})
		}
	}
	return clauses
}

// Solve runs the SAT solver and returns the solution
func (g *GophersatAdapter) Solve() (pkg.Status, map[string]string, error) {
	// Logging before solving
	logging.Debug("Solving SAT problem with %d variables and %d clauses", len(g.vars), len(g.clauses))

	// REMOVE: duplicate "exactly one version" constraints
	// (this is already done in AddExactlyOneConstraint)

	// REMOVE: slot conflicts (temporarily, not yet implemented)
	/*
	   for _, versions1 := range g.packages {
	       for _, p1 := range versions1 {
	           for _, versions2 := range g.packages {
	               for _, p2 := range versions2 {
	                   if p1.ConflictsWith(p2) {
	                       key1 := p1.Name + "@" + p1.Version
	                       key2 := p2.Name + "@" + p2.Version
	                       varID1 := g.vars[key1]
	                       varID2 := g.vars[key2]
	                       g.addClause([]int{-varID1, -varID2})
	                   }
	               }
	           }
	       }
	   }
	*/

	// Create SAT problem
	pb := solver.ParseSlice(g.clauses)

	// Create solver
	s := solver.New(pb)
	s.Verbose = false

	// Solve the problem
	status := s.Solve()

	if status == solver.Sat {
		logging.Debug("SAT solution found")
		solution := make(map[string]string)
		model := s.Model()

		// Iterate over all registered variables
		// Key by name@version to support multi-slot (same name, different slots)
		for key, varID := range g.vars {
			if varID <= len(model) && model[varID-1] {
				parts := strings.Split(key, "@")
				if len(parts) == 2 {
					// Use full key (name@version) to avoid multi-slot overwrites
					solution[key] = parts[1]
				}
			}
		}
		return pkg.StatusSat, solution, nil
	}

	if status == solver.Unsat {
		logging.Debug("UNSAT: no solution possible")
		return pkg.StatusUnsat, nil, nil
	}

	logging.Debug("INDETERMINATE: solver timeout")
	return pkg.StatusIndet, nil, fmt.Errorf("solver timeout")
}

// AddImplication adds an implication clause: if dependent is selected,
// then at least one of the providers must be selected.
// Encodes as: (-dependent | provider1 | provider2 | ...)
func (g *GophersatAdapter) AddImplication(dependent int, providers []int) {
	if len(providers) == 0 {
		return
	}
	clause := make([]int, 0, 1+len(providers))
	clause = append(clause, -dependent)
	clause = append(clause, providers...)
	g.addClause(clause)
	logging.Debug("Added implication: -%d => %v", dependent, providers)
}

// AddImplicationConstraint adds an implication clause for a version constraint.
// If dependentVarID is selected, at least one package satisfying constraint c must be selected.
// Returns error if the constraint is Required and no package satisfies it.
func (g *GophersatAdapter) AddImplicationConstraint(dependentVarID int, c pkg.Constraint) error {
	providers := g.findSatisfyingVars(c)
	if len(providers) == 0 {
		if c.Required {
			return fmt.Errorf("unsatisfiable: no package provides %s", c.String())
		}
		logging.Debug("Warning: no package satisfies %s (non-required, skipping implication)", c.String())
		return nil
	}
	g.AddImplication(dependentVarID, providers)
	return nil
}

// AddImplicationSlotConstraint adds an implication for a slot constraint.
// If dependentVarID is selected, at least one package in the given slot must be selected.
func (g *GophersatAdapter) AddImplicationSlotConstraint(dependentVarID int, c pkg.Constraint) error {
	var slotVars []int
	for _, p := range g.packages[c.Name] {
		if p.Slot.Name == c.Slot {
			key := p.Name + "@" + p.Version
			varID := g.getVarID(key)
			slotVars = append(slotVars, varID)
		}
	}
	if len(slotVars) == 0 {
		return fmt.Errorf("no package %s provides slot %s", c.Name, c.Slot)
	}
	g.AddImplication(dependentVarID, slotVars)
	return nil
}

// AddImplicationOrGroup adds an implication for an OR-group.
// If dependentVarID is selected, at least one alternative must be selected.
func (g *GophersatAdapter) AddImplicationOrGroup(dependentVarID int, alternatives []pkg.Constraint) error {
	var allSatisfyingVars []int
	for _, alt := range alternatives {
		if alt.Version != nil {
			for _, p := range g.packages[alt.Name] {
				if alt.Version.Satisfies(p.Version) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					allSatisfyingVars = append(allSatisfyingVars, varID)
				}
			}
		} else {
			for _, p := range g.packages[alt.Name] {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				allSatisfyingVars = append(allSatisfyingVars, varID)
			}
		}
	}
	if len(allSatisfyingVars) == 0 {
		return fmt.Errorf("no packages satisfy OR-group alternatives")
	}
	g.AddImplication(dependentVarID, allSatisfyingVars)
	return nil
}

// AddAtMostOnePerSlot adds pairwise exclusion clauses for package versions
// sharing the same (name, slot). This ensures at most one version is installed
// per slot. Clause form: (-vi | -vj) for all i < j within each slot group.
func (g *GophersatAdapter) AddAtMostOnePerSlot() {
	type slotGroup struct {
		name string
		slot string
	}
	groups := make(map[slotGroup][]int)

	for _, versions := range g.packages {
		for _, p := range versions {
			key := p.Name + "@" + p.Version
			varID, exists := g.vars[key]
			if !exists {
				continue
			}
			sg := slotGroup{name: p.Name, slot: p.Slot.Name}
			groups[sg] = append(groups[sg], varID)
		}
	}

	for sg, vars := range groups {
		if len(vars) <= 1 {
			continue
		}
		logging.Debug("At-most-one per slot %s:%s — %d versions, %d pairwise clauses",
			sg.name, sg.slot, len(vars), len(vars)*(len(vars)-1)/2)
		for i := 0; i < len(vars); i++ {
			for j := i + 1; j < len(vars); j++ {
				g.addClause([]int{-vars[i], -vars[j]})
			}
		}
	}
}

// findSatisfyingVars returns SAT variable IDs for all registered packages
// that satisfy the given constraint.
func (g *GophersatAdapter) findSatisfyingVars(c pkg.Constraint) []int {
	var result []int

	switch c.Type {
	case pkg.ConstraintTypeVersion:
		for _, p := range g.packages[c.Name] {
			if c.Version == nil || c.Version.Satisfies(p.Version) {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				result = append(result, varID)
			}
		}
	case pkg.ConstraintTypeSlot:
		for _, p := range g.packages[c.Name] {
			if p.Slot.Name == c.Slot {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				result = append(result, varID)
			}
		}
	}

	return result
}

// GetVarID returns the SAT variable ID for a package key (name@version).
// Returns 0 if the key is not registered.
func (g *GophersatAdapter) GetVarID(key string) int {
	if id, exists := g.vars[key]; exists {
		return id
	}
	return 0
}

// GetPackageVersions returns all registered versions for a package
func (g *GophersatAdapter) GetPackageVersions(name string) []string {
	var versions []string
	for _, p := range g.packages[name] {
		versions = append(versions, p.Version)
	}
	return versions
}
