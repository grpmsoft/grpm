package solver

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/crillab/gophersat/explain"
	"github.com/crillab/gophersat/solver"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// ClauseSource identifies why a SAT clause was added.
type ClauseSource int

const (
	ClauseRoot        ClauseSource = iota // root at-least-one
	ClauseImplication                     // if A then B1|B2|...
	ClauseAtMostOne                       // pairwise exclusion (-vi|-vj)
	ClauseProhibit                        // single negative literal: candidate impossible
	ClauseConflict                        // blocker: two packages cannot coexist (-A|-B)
)

func (s ClauseSource) String() string {
	switch s {
	case ClauseRoot:
		return "root"
	case ClauseImplication:
		return "implication"
	case ClauseAtMostOne:
		return "at-most-one"
	case ClauseProhibit:
		return "prohibit"
	case ClauseConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// clauseMeta stores the source and human-readable reason for a clause.
type clauseMeta struct {
	Source ClauseSource
	Reason string
}

// implicationEdge records one implication: if dependent is selected, at least one provider must be.
type implicationEdge struct {
	providers []int
	reason    string
}

// GophersatAdapter adapts the gophersat SAT solver for package dependency resolution
type GophersatAdapter struct {
	clauses      [][]int
	clausesMeta  []clauseMeta              // parallel to clauses: source + reason
	vars         map[string]int            // name@version -> var ID
	varNames     map[int]string            // var ID -> name@version
	packages     map[string][]*pkg.Package // name -> []versions
	addedClauses map[string]struct{}       // to prevent duplicate clauses
	pendingMeta  *clauseMeta               // set before addClause to tag the next clause

	// Structural indices for UNSAT explanation (built alongside clauses)
	implications map[int][]implicationEdge // varID → "if selected, needs one of providers"
	prohibits    map[int]string            // varID → reason why this candidate is impossible
	rootVars     []int                     // var IDs from root at-least-one clauses

	// installed tracks which SAT variable IDs represent packages from VDB
	installed map[int]bool

	// orGroupPrefs tracks OR-group alternative preferences for MAX-SAT.
	// Key: varID of provider, Value: position in OR-group (0 = first/preferred).
	// When multiple OR-groups mention the same var, the lowest position wins.
	orGroupPrefs map[int]int
}

func NewGophersatAdapter() *GophersatAdapter {
	return &GophersatAdapter{
		vars:         make(map[string]int),
		varNames:     make(map[int]string),
		packages:     make(map[string][]*pkg.Package),
		addedClauses: make(map[string]struct{}),
		implications: make(map[int][]implicationEdge),
		prohibits:    make(map[int]string),
		installed:    make(map[int]bool),
		orGroupPrefs: make(map[int]int),
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
	if g.pendingMeta != nil {
		g.clausesMeta = append(g.clausesMeta, *g.pendingMeta)
		g.pendingMeta = nil
	} else {
		g.clausesMeta = append(g.clausesMeta, clauseMeta{})
	}
	g.addedClauses[key] = struct{}{}
	logging.Debug("Added clause: %v", clause)
}

// withMeta sets metadata for the next addClause call.
func (g *GophersatAdapter) withMeta(source ClauseSource, reason string) {
	g.pendingMeta = &clauseMeta{Source: source, Reason: reason}
}

// addRootVars records var IDs that appear in root at-least-one clauses.
func (g *GophersatAdapter) addRootVars(vars []int) {
	g.rootVars = append(g.rootVars, vars...)
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

	// Collect all packages that match this atom (version/slot + USE deps)
	var satisfiedVars []int
	for _, p := range g.packages[pkgName] {
		if atom.Matches(p) {
			if !atomSatisfiesUseDeps(p, atom) {
				logging.Debug("Package %s excluded by USE deps for atom %s",
					p.Name+"@"+p.Version, atom.String())
				continue
			}
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

	g.withMeta(ClauseRoot, fmt.Sprintf("atom %s", atom.String()))
	g.addClause(satisfiedVars)
	return nil
}

// AddOrGroupConstraint adds an OR-group constraint (at-least-one alternative)
// For example: || ( mysql postgresql ) means "pick mysql OR postgresql"
// When an alternative has USE deps (UseRequire/UseBlock), providers are filtered
// to only those whose effective USE flags satisfy the requirements.
func (g *GophersatAdapter) AddOrGroupConstraint(alternatives []pkg.Constraint) error {
	var allSatisfyingVars []int

	// Collect all package versions that satisfy ANY of the alternatives
	for _, alt := range alternatives {
		var altVars []int

		// For version constraints
		if alt.Version != nil {
			for _, p := range g.packages[alt.Name] {
				if alt.Version.Satisfies(p.Version) && alt.PackageSatisfiesUseDeps(p.UseFlags) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					altVars = append(altVars, varID)
				}
			}
		} else {
			// No version constraint - any version satisfies
			for _, p := range g.packages[alt.Name] {
				if alt.PackageSatisfiesUseDeps(p.UseFlags) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					altVars = append(altVars, varID)
				}
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
	g.withMeta(ClauseRoot, "OR-group: at-least-one alternative")
	g.addClause(allSatisfyingVars)
	return nil
}

func (g *GophersatAdapter) addVersionConstraint(c pkg.Constraint) error {
	logging.Debug("Processing constraint: %s %s", c.Name, c.Version)

	// For simple constraints without version
	if c.Version == nil {
		return g.addSimpleConstraint(c.Name)
	}

	// Collect all packages satisfying the constraint (version + USE deps)
	var satisfiedVars []int
	for _, p := range g.packages[c.Name] {
		if c.Version.Satisfies(p.Version) {
			if !c.PackageSatisfiesUseDeps(p.UseFlags) {
				logging.Debug("Package %s excluded by USE deps for %s %s",
					p.Name+"@"+p.Version, c.Name, c.Version.String())
				continue
			}
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

	g.withMeta(ClauseRoot, fmt.Sprintf("version constraint %s %s", c.Name, c.Version.String()))
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
		g.withMeta(ClauseRoot, fmt.Sprintf("simple constraint %s", name))
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
		g.withMeta(ClauseRoot, fmt.Sprintf("mandatory %s", pkgName))
		g.addClause([]int{versionVars[0]})
		logging.Debug("Added mandatory constraint for %s: [%d]", pkgName, versionVars[0])
		return
	}

	// Add clauses for "exactly one version" constraint
	eoClauses := exactlyOne(versionVars)
	for i, clause := range eoClauses {
		if i == 0 {
			g.withMeta(ClauseRoot, fmt.Sprintf("at-least-one %s", pkgName))
		} else {
			g.withMeta(ClauseAtMostOne, fmt.Sprintf("at-most-one %s", pkgName))
		}
		g.addClause(clause)
	}
	logging.Debug("Added exactly-one constraint for %s: %d versions", pkgName, len(versions))
}

func (g *GophersatAdapter) addSlotConstraint(c pkg.Constraint) error {
	// Find all packages matching name AND slot (operators := and :* match any slot)
	isOperator := c.Slot == "=" || c.Slot == "*"
	var slotVars []int
	for _, pkgList := range g.packages {
		for _, p := range pkgList {
			slotMatch := isOperator || p.Slot.Name == c.Slot
			versionMatch := c.Version == nil || c.Version.Satisfies(p.Version)
			if p.Name == c.Name && slotMatch && versionMatch {
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				slotVars = append(slotVars, varID)
			}
		}
	}

	if len(slotVars) == 0 {
		return fmt.Errorf("no package %s provides slot %s", c.Name, c.Slot)
	}

	g.withMeta(ClauseRoot, fmt.Sprintf("slot constraint %s:%s", c.Name, c.Slot))
	g.addClause(slotVars)
	return nil
}

func (g *GophersatAdapter) addUseFlagConstraint(c pkg.Constraint) error {
	if c.Required {
		flagVar := g.getVarID("USE_" + c.Flag)
		g.withMeta(ClauseRoot, fmt.Sprintf("USE flag %s", c.Flag))
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
		return pkg.StatusSat, g.extractSolution(s.Model()), nil
	}

	if status == solver.Unsat {
		logging.Debug("UNSAT: no solution possible")
		return pkg.StatusUnsat, nil, nil
	}

	logging.Debug("INDETERMINATE: solver timeout")
	return pkg.StatusIndet, nil, fmt.Errorf("solver timeout")
}

// extractSolution reads the model from a solver and returns the solution map.
func (g *GophersatAdapter) extractSolution(model []bool) map[string]string {
	solution := make(map[string]string)
	for key, varID := range g.vars {
		if varID <= len(model) && model[varID-1] {
			parts := strings.Split(key, "@")
			if len(parts) == 2 {
				solution[key] = parts[1]
			}
		}
	}
	return solution
}

// versionRank pairs a SAT variable ID with its preference rank (0 = most preferred).
type versionRank struct {
	varID int
	rank  int
}

// buildVersionPreferences assigns preference ranks to each version of each package.
// rank 0 = most preferred (newest in update mode, installed in keep mode).
func (g *GophersatAdapter) buildVersionPreferences(updateMode bool) []versionRank {
	var preferences []versionRank

	for pkgName, versions := range g.packages {
		if len(versions) <= 1 {
			continue
		}

		sorted := make([]*pkg.Package, len(versions))
		copy(sorted, versions)
		sort.Slice(sorted, func(i, j int) bool {
			return pkg.CompareVersions(sorted[i].Version, sorted[j].Version) > 0
		})

		// Check if any version is installed (for keep-mode preference)
		installedRankAssigned := false
		for rank, p := range sorted {
			key := pkgName + "@" + p.Version
			varID, exists := g.vars[key]
			if !exists {
				continue
			}

			actualRank := rank
			if !updateMode && g.installed[varID] && !installedRankAssigned {
				actualRank = 0
				installedRankAssigned = true
			} else if !updateMode && installedRankAssigned {
				actualRank = rank + 1
			}

			preferences = append(preferences, versionRank{varID: varID, rank: actualRank})
		}
	}
	return preferences
}

// SolveOptimal finds an optimal solution using MAX-SAT with version preferences.
// Hard clauses (all existing clauses) are preserved. Soft clauses express version
// preferences: for each package with multiple candidates, prefer the version with
// lowest rank (0 = most preferred).
//
// If updateMode is true, newest version gets rank 0 (prefer upgrading).
// If false, installed version gets rank 0 (prefer keeping), then newest.
//
// Falls back to regular SAT result on timeout or if optimization fails.
func (g *GophersatAdapter) SolveOptimal(timeout time.Duration, updateMode bool) (pkg.Status, map[string]string, error) {
	// First: regular SAT to check satisfiability
	status, fallbackSolution, err := g.Solve()
	if status != pkg.StatusSat {
		return status, fallbackSolution, err
	}

	preferences := g.buildVersionPreferences(updateMode)
	if len(preferences) == 0 && len(g.orGroupPrefs) == 0 {
		return pkg.StatusSat, fallbackSolution, nil
	}

	// Build soft penalty clauses for non-preferred choices.
	allClauses := make([][]int, len(g.clauses))
	copy(allClauses, g.clauses)

	nextVar := len(g.vars) + 1
	var relaxLits []solver.Lit
	var weights []int

	// Version preferences: penalize non-preferred versions
	maxVersionRank := 0
	for _, p := range preferences {
		if p.rank > maxVersionRank {
			maxVersionRank = p.rank
		}
	}
	for _, p := range preferences {
		if p.rank == 0 {
			continue
		}
		relaxVar := nextVar
		nextVar++
		allClauses = append(allClauses, []int{-p.varID, relaxVar})
		relaxLits = append(relaxLits, solver.IntToLit(int32(relaxVar)))
		weights = append(weights, p.rank)
	}

	// OR-group preferences: penalize non-first alternatives.
	// Weight dominates version preferences to prevent OR choice flip.
	orWeight := maxVersionRank + 1
	if orWeight < 2 {
		orWeight = 2
	}
	for varID, pos := range g.orGroupPrefs {
		if pos == 0 {
			continue
		}
		relaxVar := nextVar
		nextVar++
		allClauses = append(allClauses, []int{-varID, relaxVar})
		relaxLits = append(relaxLits, solver.IntToLit(int32(relaxVar)))
		weights = append(weights, orWeight*pos)
	}

	if len(relaxLits) == 0 {
		return pkg.StatusSat, fallbackSolution, nil
	}

	pb := solver.ParseSlice(allClauses)
	pb.SetCostFunc(relaxLits, weights)
	s := solver.New(pb)
	s.Verbose = false

	// Run Minimize with timeout
	type result struct {
		cost  int
		model []bool
	}
	ch := make(chan result, 1)
	go func() {
		cost := s.Minimize()
		var model []bool
		if cost >= 0 {
			model = s.Model()
		}
		ch <- result{cost: cost, model: model}
	}()

	select {
	case r := <-ch:
		if r.cost < 0 {
			logging.Warn("MAX-SAT optimization found no solution, using SAT fallback")
			return pkg.StatusSat, fallbackSolution, nil
		}
		logging.Debug("MAX-SAT optimal cost: %d", r.cost)

		solution := g.extractSolution(r.model)
		return pkg.StatusSat, solution, nil

	case <-time.After(timeout):
		logging.Warn("MAX-SAT optimization timed out after %v — using feasible (not optimal) solution", timeout)
		return pkg.StatusSat, fallbackSolution, nil
	}
}

// solveTieBreak fixes the optimal cost as a hard constraint and re-solves.
// AddImplication adds an implication clause: if dependent is selected,
// then at least one of the providers must be selected.
// Encodes as: (-dependent | provider1 | provider2 | ...)
func (g *GophersatAdapter) AddImplication(dependent int, providers []int) {
	if len(providers) == 0 {
		return
	}
	depName := g.varNames[dependent]
	providerNames := g.literalNames(providers)
	reason := fmt.Sprintf("%s => one of [%s]", depName, strings.Join(providerNames, ", "))
	g.withMeta(ClauseImplication, reason)
	clause := make([]int, 0, 1+len(providers))
	clause = append(clause, -dependent)
	clause = append(clause, providers...)
	g.addClause(clause)

	provCopy := make([]int, len(providers))
	copy(provCopy, providers)
	g.implications[dependent] = append(g.implications[dependent], implicationEdge{
		providers: provCopy,
		reason:    reason,
	})
	logging.Debug("Added implication: -%d => %v", dependent, providers)
}

// AddImplicationConstraint adds an implication clause for a version constraint.
// If dependentVarID is selected, at least one package satisfying constraint c must be selected.
// Returns error if the constraint is Required and no package satisfies it.
func (g *GophersatAdapter) AddImplicationConstraint(dependentVarID int, c pkg.Constraint) error {
	providers := g.findSatisfyingVars(c)
	if len(providers) == 0 {
		reason := fmt.Sprintf("no provider for %s (needed by %s)", c.String(), g.varNames[dependentVarID])
		g.withMeta(ClauseProhibit, reason)
		g.addClause([]int{-dependentVarID})
		g.prohibits[dependentVarID] = reason
		logging.Debug("Prohibiting %d: no package provides %s", dependentVarID, c.String())
		return nil
	}
	g.AddImplication(dependentVarID, providers)
	return nil
}

// AddImplicationSlotConstraint adds an implication for a slot constraint.
// If dependentVarID is selected, at least one package in the given slot must be selected.
// When the constraint has USE deps (UseRequire/UseBlock), providers are filtered
// to only those whose effective USE flags satisfy the requirements.
func (g *GophersatAdapter) AddImplicationSlotConstraint(dependentVarID int, c pkg.Constraint) error {
	isOperator := c.Slot == "=" || c.Slot == "*"
	var slotVars []int
	for _, p := range g.packages[c.Name] {
		slotMatch := isOperator || p.Slot.Name == c.Slot
		versionMatch := c.Version == nil || c.Version.Satisfies(p.Version)
		if slotMatch && versionMatch {
			if !c.PackageSatisfiesUseDeps(p.UseFlags) {
				logging.Debug("Package %s-%s excluded by USE deps (slot %s)",
					p.Name, p.Version, c.Slot)
				continue
			}
			key := p.Name + "@" + p.Version
			varID := g.getVarID(key)
			slotVars = append(slotVars, varID)
		}
	}
	if len(slotVars) == 0 {
		reason := fmt.Sprintf("no provider for %s:%s (needed by %s)", c.Name, c.Slot, g.varNames[dependentVarID])
		g.withMeta(ClauseProhibit, reason)
		g.addClause([]int{-dependentVarID})
		g.prohibits[dependentVarID] = reason
		logging.Debug("Prohibiting %d: no package %s in slot %s", dependentVarID, c.Name, c.Slot)
		return nil
	}
	g.AddImplication(dependentVarID, slotVars)
	return nil
}

// AddImplicationOrGroup adds an implication for an OR-group.
// If dependentVarID is selected, at least one alternative must be selected.
// When an alternative has USE deps (UseRequire/UseBlock), providers are filtered
// to only those whose effective USE flags satisfy the requirements.
func (g *GophersatAdapter) AddImplicationOrGroup(dependentVarID int, alternatives []pkg.Constraint) error {
	var allSatisfyingVars []int
	for altIdx, alt := range alternatives {
		if alt.Version != nil {
			for _, p := range g.packages[alt.Name] {
				if alt.Version.Satisfies(p.Version) && alt.PackageSatisfiesUseDeps(p.UseFlags) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					allSatisfyingVars = append(allSatisfyingVars, varID)
					if prev, exists := g.orGroupPrefs[varID]; !exists || altIdx < prev {
						g.orGroupPrefs[varID] = altIdx
					}
				}
			}
		} else {
			for _, p := range g.packages[alt.Name] {
				if alt.PackageSatisfiesUseDeps(p.UseFlags) {
					key := p.Name + "@" + p.Version
					varID := g.getVarID(key)
					allSatisfyingVars = append(allSatisfyingVars, varID)
					if prev, exists := g.orGroupPrefs[varID]; !exists || altIdx < prev {
						g.orGroupPrefs[varID] = altIdx
					}
				}
			}
		}
	}
	if len(allSatisfyingVars) == 0 {
		reason := fmt.Sprintf("no provider for OR-group (needed by %s)", g.varNames[dependentVarID])
		g.withMeta(ClauseProhibit, reason)
		g.addClause([]int{-dependentVarID})
		g.prohibits[dependentVarID] = reason
		logging.Debug("Prohibiting %d: no packages satisfy OR-group", dependentVarID)
		return nil
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
				g.withMeta(ClauseAtMostOne, fmt.Sprintf("conflict %s vs %s (slot %s:%s)",
					g.varNames[vars[i]], g.varNames[vars[j]], sg.name, sg.slot))
				g.addClause([]int{-vars[i], -vars[j]})
			}
		}
	}
}

// AddBlockerConflict adds conflict clauses for a specific candidate version that
// declares a blocker. Emits (-blockerVarID | -B) for each matching version B
// of the blocked package. Only the declaring version gets the conflict — other
// versions of the same name that don't declare the blocker are unaffected.
func (g *GophersatAdapter) AddBlockerConflict(blockerVarID int, blockedAtom *pkg.Atom) {
	if blockedAtom == nil || blockerVarID == 0 {
		return
	}
	blockedName := blockedAtom.CP()
	blockerKey := g.varNames[blockerVarID]

	blockedVersions := g.packages[blockedName]
	if len(blockedVersions) == 0 {
		logging.Debug("Blocker %s blocks %s but no versions registered — vacuously true",
			blockerKey, blockedAtom.String())
		return
	}

	for _, tv := range blockedVersions {
		if !blockedAtom.Matches(tv) {
			continue
		}
		targetKey := tv.Name + "@" + tv.Version
		targetVarID, exists := g.vars[targetKey]
		if !exists || targetVarID == blockerVarID {
			continue
		}

		reason := fmt.Sprintf("blocker: %s blocks %s (atom %s)", blockerKey, targetKey, blockedAtom.String())
		g.withMeta(ClauseConflict, reason)
		g.addClause([]int{-blockerVarID, -targetVarID})
		logging.Debug("Added blocker conflict: %s vs %s", blockerKey, targetKey)
	}
}

// MarkInstalled marks a SAT variable as representing an installed package (from VDB).
// Used by MAX-SAT (task 014) for preference weights: installed packages get higher
// weight to prefer keeping them over pulling new versions.
func (g *GophersatAdapter) MarkInstalled(varID int) {
	g.installed[varID] = true
}

// IsVarInstalled returns true if the given SAT variable represents an installed package.
func (g *GophersatAdapter) IsVarInstalled(varID int) bool {
	return g.installed[varID]
}

// findSatisfyingVars returns SAT variable IDs for all registered packages
// that satisfy the given constraint, including USE dependency filtering.
// When the constraint has UseRequire or UseBlock, only packages whose
// effective USE flags satisfy those requirements are included.
func (g *GophersatAdapter) findSatisfyingVars(c pkg.Constraint) []int {
	var result []int

	switch c.Type {
	case pkg.ConstraintTypeVersion:
		for _, p := range g.packages[c.Name] {
			if c.Version == nil || c.Version.Satisfies(p.Version) {
				if !c.PackageSatisfiesUseDeps(p.UseFlags) {
					logging.Debug("Package %s-%s excluded by USE deps (%v require, %v block)",
						p.Name, p.Version, c.UseRequire, c.UseBlock)
					continue
				}
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				result = append(result, varID)
			}
		}
	case pkg.ConstraintTypeSlot:
		isOperator := c.Slot == "=" || c.Slot == "*"
		for _, p := range g.packages[c.Name] {
			slotMatch := isOperator || p.Slot.Name == c.Slot
			versionMatch := c.Version == nil || c.Version.Satisfies(p.Version)
			if slotMatch && versionMatch {
				if !c.PackageSatisfiesUseDeps(p.UseFlags) {
					logging.Debug("Package %s-%s excluded by USE deps (slot %s)",
						p.Name, p.Version, c.Slot)
					continue
				}
				key := p.Name + "@" + p.Version
				varID := g.getVarID(key)
				result = append(result, varID)
			}
		}
	}

	return result
}

// atomSatisfiesUseDeps checks whether a package's effective USE flags satisfy
// the USE dependency requirements specified on an atom.
// Returns true if all UseRequire flags are enabled and all UseBlock flags are disabled.
// If the atom has no USE deps, always returns true.
func atomSatisfiesUseDeps(p *pkg.Package, atom *pkg.Atom) bool {
	if atom == nil || !atom.HasUseDeps() {
		return true
	}

	// Build UseDefault map from atom (entries like "ssl(+)", "debug(-)")
	useDefault := make(map[string]bool)
	for _, entry := range atom.UseDefault {
		if strings.HasSuffix(entry, "(+)") {
			useDefault[strings.TrimSuffix(entry, "(+)")] = true
		} else if strings.HasSuffix(entry, "(-)") {
			useDefault[strings.TrimSuffix(entry, "(-)")] = false
		}
	}

	for _, flag := range atom.UseRequire {
		enabled, declared := p.UseFlags[flag]
		if !declared {
			if dflt, hasDefault := useDefault[flag]; hasDefault {
				enabled = dflt
			}
		}
		if !enabled {
			return false
		}
	}
	for _, flag := range atom.UseBlock {
		enabled, declared := p.UseFlags[flag]
		if !declared {
			if dflt, hasDefault := useDefault[flag]; hasDefault {
				enabled = dflt
			}
		}
		if enabled {
			return false
		}
	}
	return true
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

// literalNames maps a slice of var IDs to their name@version strings.
func (g *GophersatAdapter) literalNames(varIDs []int) []string {
	names := make([]string, 0, len(varIDs))
	for _, id := range varIDs {
		absID := id
		if absID < 0 {
			absID = -absID
		}
		if name, ok := g.varNames[absID]; ok {
			if id < 0 {
				names = append(names, "!"+name)
			} else {
				names = append(names, name)
			}
		} else {
			names = append(names, fmt.Sprintf("var%d", id))
		}
	}
	return names
}

// varName returns human-readable name for a var ID.
func (g *GophersatAdapter) varName(id int) string {
	if name, ok := g.varNames[id]; ok {
		return name
	}
	return fmt.Sprintf("var%d", id)
}

// packageNameOf extracts the package name (category/package) from a varID.
func (g *GophersatAdapter) packageNameOf(varID int) string {
	name := g.varName(varID)
	if idx := strings.Index(name, "@"); idx >= 0 {
		return name[:idx]
	}
	return name
}

// ProhibitedPackageNames returns the set of package names (category/package)
// that have at least one version prohibited by the SAT encoding.
// Used by the resolver's retry loop to detect which OR-group alternatives
// were blocked due to unexplored dependencies.
func (g *GophersatAdapter) ProhibitedPackageNames() map[string]bool {
	names := make(map[string]bool, len(g.prohibits))
	for varID := range g.prohibits {
		names[g.packageNameOf(varID)] = true
	}
	return names
}

// ExplainUNSATResult holds the structured explanation output.
type ExplainUNSATResult struct {
	Lines    []string // human-readable lines
	CoreSize int      // number of clauses/nodes in the explanation
}

// edgeRef points from a provider back to an implication edge that contains it.
type edgeRef struct {
	dependent int
	edgeIdx   int
}

// unsatTracer propagates deadness bottom-up from prohibits via worklist,
// then formats the justification chain top-down.
type unsatTracer struct {
	adapter       *GophersatAdapter
	dead          map[int]bool             // varID → is dead
	justification map[int]*implicationEdge // varID → edge that proved it dead (nil = prohibit)
	reason        map[int]string           // varID → one-line summary
	printed       map[int]bool             // dedup for formatting
	chain         []string                 // output lines
}

// propagate marks nodes dead bottom-up: prohibits first, then dependents
// whose implication has all providers dead. Records justification edge.
func (tr *unsatTracer) propagate() {
	g := tr.adapter

	// Build reverse index: provider → edges that reference it
	reverse := make(map[int][]edgeRef)
	for dep, edges := range g.implications {
		for i, edge := range edges {
			for _, prov := range edge.providers {
				reverse[prov] = append(reverse[prov], edgeRef{dependent: dep, edgeIdx: i})
			}
		}
	}

	// Seed worklist with prohibits
	worklist := make([]int, 0, len(g.prohibits))
	for varID, r := range g.prohibits {
		tr.dead[varID] = true
		tr.reason[varID] = r
		worklist = append(worklist, varID)
	}

	for len(worklist) > 0 {
		v := worklist[0]
		worklist = worklist[1:]

		for _, ref := range reverse[v] {
			if tr.dead[ref.dependent] {
				continue
			}
			edge := &g.implications[ref.dependent][ref.edgeIdx]
			allDead := true
			for _, prov := range edge.providers {
				if !tr.dead[prov] {
					allDead = false
					break
				}
			}
			if allDead {
				tr.dead[ref.dependent] = true
				tr.justification[ref.dependent] = edge
				tr.reason[ref.dependent] = tr.summarizeEdge(edge)
				worklist = append(worklist, ref.dependent)
			}
		}
	}
}

// summarizeEdge builds a one-line summary for a dead edge.
func (tr *unsatTracer) summarizeEdge(edge *implicationEdge) string {
	g := tr.adapter
	pkgName := g.packageNameOf(edge.providers[0])

	distinctPkgs := 1
	for i := 1; i < len(edge.providers); i++ {
		if g.packageNameOf(edge.providers[i]) != pkgName {
			distinctPkgs++
		}
	}

	if distinctPkgs == 1 && len(edge.providers) > 1 {
		return fmt.Sprintf("needs %s but all %d versions impossible", pkgName, len(edge.providers))
	}
	if distinctPkgs == 1 {
		return fmt.Sprintf("needs %s but impossible", pkgName)
	}
	return fmt.Sprintf("needs one of %d providers but all impossible", len(edge.providers))
}

// walkTopDown formats the justification chain from root to prohibit leaves.
// Each var printed once in full; subsequent refs emit "(see above)".
func (tr *unsatTracer) walkTopDown(varID int, depth int) {
	if !tr.dead[varID] {
		return
	}
	indent := strings.Repeat("  ", depth)
	name := tr.adapter.varName(varID)

	if tr.printed[varID] {
		tr.chain = append(tr.chain, fmt.Sprintf("%s%s  (see above)", indent, name))
		return
	}
	tr.printed[varID] = true

	tr.chain = append(tr.chain, fmt.Sprintf("%s%s: %s", indent, name, tr.reason[varID]))

	edge := tr.justification[varID]
	if edge == nil {
		return
	}

	// Group providers by package name, walk each
	type provGroup struct {
		pkgName string
		varIDs  []int
	}
	var groups []provGroup
	gIdx := make(map[string]int)
	for _, prov := range edge.providers {
		p := tr.adapter.packageNameOf(prov)
		if idx, ok := gIdx[p]; ok {
			groups[idx].varIDs = append(groups[idx].varIDs, prov)
		} else {
			gIdx[p] = len(groups)
			groups = append(groups, provGroup{pkgName: p, varIDs: []int{prov}})
		}
	}

	for _, grp := range groups {
		if len(grp.varIDs) <= 4 {
			for _, prov := range grp.varIDs {
				tr.walkTopDown(prov, depth+1)
			}
		} else {
			tr.chain = append(tr.chain, fmt.Sprintf("%s  %s: %d versions, all impossible (first 3):",
				indent, grp.pkgName, len(grp.varIDs)))
			for _, prov := range grp.varIDs[:3] {
				tr.walkTopDown(prov, depth+2)
			}
		}
	}
}

// findReachableConflicts collects ClauseConflict clauses where both literals
// are reachable from root candidates via implication edges.
func (tr *unsatTracer) findReachableConflicts() []string {
	g := tr.adapter

	// BFS from roots through implications to find all reachable vars
	reachable := make(map[int]bool)
	queue := make([]int, len(g.rootVars))
	copy(queue, g.rootVars)
	for _, v := range g.rootVars {
		reachable[v] = true
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, edge := range g.implications[v] {
			for _, prov := range edge.providers {
				if !reachable[prov] {
					reachable[prov] = true
					queue = append(queue, prov)
				}
			}
		}
	}

	// Find conflict clauses where both vars are reachable
	var results []string
	for i, clause := range g.clauses {
		if g.clausesMeta[i].Source != ClauseConflict {
			continue
		}
		if len(clause) == 2 {
			a, b := clause[0], clause[1]
			if a < 0 {
				a = -a
			}
			if b < 0 {
				b = -b
			}
			if reachable[a] && reachable[b] {
				results = append(results, fmt.Sprintf("  %s", g.clausesMeta[i].Reason))
			}
		}
	}
	return results
}

// ExplainWhyUNSAT propagates deadness bottom-up from prohibit leaves via worklist,
// then formats the justification chain top-down from roots. O(edges), cycle-safe,
// deterministic. Cycles do NOT make nodes dead (correct: SAT can satisfy cyclic deps).
func (g *GophersatAdapter) ExplainWhyUNSAT() ExplainUNSATResult {
	if len(g.rootVars) == 0 {
		return ExplainUNSATResult{Lines: []string{"no root candidates registered"}}
	}

	tr := &unsatTracer{
		adapter:       g,
		dead:          make(map[int]bool),
		justification: make(map[int]*implicationEdge),
		reason:        make(map[int]string),
		printed:       make(map[int]bool),
	}

	tr.propagate()

	var rootResults []string
	allRootsDead := true
	for _, rootVar := range g.rootVars {
		if !tr.dead[rootVar] {
			allRootsDead = false
		} else {
			rootResults = append(rootResults, fmt.Sprintf("  %s: %s", g.varName(rootVar), tr.reason[rootVar]))
		}
	}

	var lines []string
	if allRootsDead {
		lines = append(lines, fmt.Sprintf("UNSAT: all %d root candidates are impossible", len(g.rootVars)))
	} else {
		lines = append(lines, "UNSAT: conflict between required packages")
	}

	if len(rootResults) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Root candidates:")
		lines = append(lines, rootResults...)
	}

	for _, rootVar := range g.rootVars {
		tr.walkTopDown(rootVar, 0)
	}

	if len(tr.chain) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Dependency chain:")
		lines = append(lines, tr.chain...)
	}

	// If not all roots dead, the UNSAT is caused by binary conflict clauses
	// (blockers or at-most-one). Find ClauseConflict clauses involving
	// candidates reachable from roots and report them.
	if !allRootsDead {
		conflicts := tr.findReachableConflicts()
		if len(conflicts) > 0 {
			lines = append(lines, "")
			lines = append(lines, "Conflicts:")
			lines = append(lines, conflicts...)
		}
	}

	coreSize := len(tr.chain)
	if coreSize == 0 {
		coreSize = 1
	}

	return ExplainUNSATResult{
		Lines:    lines,
		CoreSize: coreSize,
	}
}

// ExplainUNSATGeneric extracts an unsatisfiable subset using gophersat/explain.
// Expensive (extra SAT call) and often returns the full problem on implication-heavy formulas.
// Use only when graph-based ExplainWhyUNSAT cannot isolate the conflict
// (e.g., at-most-one conflicts without prohibit chains).
func (g *GophersatAdapter) ExplainUNSATGeneric() []string {
	pb := &explain.Problem{
		Clauses:   g.clauses,
		NbVars:    len(g.vars),
		NbClauses: len(g.clauses),
	}

	subset, err := pb.UnsatSubset()
	if err != nil {
		return []string{fmt.Sprintf("UNSAT explanation failed: %v", err)}
	}

	// Map subset clauses back to our metadata via literal-set matching.
	type clauseKey string
	makeCK := func(c []int) clauseKey {
		s := make([]int, len(c))
		copy(s, c)
		sort.Ints(s)
		return clauseKey(fmt.Sprintf("%v", s))
	}

	idx := make(map[clauseKey]int, len(g.clauses))
	for i, c := range g.clauses {
		idx[makeCK(c)] = i
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("UNSAT core (generic): %d clauses (of %d total)", subset.NbClauses, len(g.clauses)))

	counts := make(map[ClauseSource]int)
	for _, c := range subset.Clauses {
		if origIdx, ok := idx[makeCK(c)]; ok {
			counts[g.clausesMeta[origIdx].Source]++
		}
	}
	for src, n := range counts {
		lines = append(lines, fmt.Sprintf("  %s: %d clauses", src, n))
	}

	return lines
}
