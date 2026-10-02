package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/crillab/gophersat/explain"
	"github.com/crillab/gophersat/solver"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// ClauseSource identifies why a SAT clause was added.
type ClauseSource int

const (
	ClauseRoot       ClauseSource = iota // root at-least-one
	ClauseImplication                     // if A then B1|B2|...
	ClauseAtMostOne                       // pairwise exclusion (-vi|-vj)
	ClauseProhibit                        // single negative literal: candidate impossible
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
}

func NewGophersatAdapter() *GophersatAdapter {
	return &GophersatAdapter{
		vars:         make(map[string]int),
		varNames:     make(map[int]string),
		packages:     make(map[string][]*pkg.Package),
		addedClauses: make(map[string]struct{}),
		implications: make(map[int][]implicationEdge),
		prohibits:    make(map[int]string),
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

	g.withMeta(ClauseRoot, fmt.Sprintf("atom %s", atom.String()))
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
func (g *GophersatAdapter) AddImplicationSlotConstraint(dependentVarID int, c pkg.Constraint) error {
	isOperator := c.Slot == "=" || c.Slot == "*"
	var slotVars []int
	for _, p := range g.packages[c.Name] {
		slotMatch := isOperator || p.Slot.Name == c.Slot
		versionMatch := c.Version == nil || c.Version.Satisfies(p.Version)
		if slotMatch && versionMatch {
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
		isOperator := c.Slot == "=" || c.Slot == "*"
		for _, p := range g.packages[c.Name] {
			slotMatch := isOperator || p.Slot.Name == c.Slot
			versionMatch := c.Version == nil || c.Version.Satisfies(p.Version)
			if slotMatch && versionMatch {
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

// ExplainUNSATResult holds the structured explanation output.
type ExplainUNSATResult struct {
	Lines    []string // human-readable lines
	CoreSize int      // number of clauses/nodes in the explanation
}

// unsatTracer holds state for the graph-based UNSAT explanation traversal.
type unsatTracer struct {
	adapter *GophersatAdapter
	cache   map[int]string // varID → reason ("ok" if satisfiable)
	chain   []string       // accumulated explanation lines
}

func (tr *unsatTracer) whyImpossible(varID int, depth int) string {
	if r, ok := tr.cache[varID]; ok {
		return r
	}
	tr.cache[varID] = "ok" // cycle guard

	if reason, prohibited := tr.adapter.prohibits[varID]; prohibited {
		tr.cache[varID] = reason
		return reason
	}

	for _, edge := range tr.adapter.implications[varID] {
		allDead := true
		for _, prov := range edge.providers {
			if tr.whyImpossible(prov, depth+1) == "ok" {
				allDead = false
				break
			}
		}
		if allDead && len(edge.providers) > 0 {
			result := tr.formatDeadEdge(varID, edge, depth)
			tr.cache[varID] = result
			return result
		}
	}

	tr.cache[varID] = "ok"
	return "ok"
}

func (tr *unsatTracer) formatDeadEdge(varID int, edge implicationEdge, depth int) string {
	g := tr.adapter

	// Group providers by package name
	pkgGroups := make(map[string][]string)
	var pkgOrder []string
	for _, prov := range edge.providers {
		pkgName := g.packageNameOf(prov)
		if _, seen := pkgGroups[pkgName]; !seen {
			pkgOrder = append(pkgOrder, pkgName)
		}
		ver := g.varName(prov)
		if idx := strings.Index(ver, "@"); idx >= 0 {
			ver = ver[idx+1:]
		}
		pkgGroups[pkgName] = append(pkgGroups[pkgName], fmt.Sprintf("%s: %s", ver, tr.cache[prov]))
	}

	summary := tr.buildSummary(edge, pkgOrder)
	result := fmt.Sprintf("needs [%s] but %s", g.packageNameOf(edge.providers[0]), summary)

	indent := strings.Repeat("  ", depth)
	tr.chain = append(tr.chain, fmt.Sprintf("%s%s: %s", indent, g.varName(varID), result))

	if depth < 8 {
		for _, pkgName := range pkgOrder {
			versions := pkgGroups[pkgName]
			if len(versions) <= 4 {
				for _, v := range versions {
					tr.chain = append(tr.chain, fmt.Sprintf("%s  %s/%s", indent, pkgName, v))
				}
			} else {
				tr.chain = append(tr.chain, fmt.Sprintf("%s  %s: %d versions, all impossible (showing first 3)", indent, pkgName, len(versions)))
				for _, v := range versions[:3] {
					tr.chain = append(tr.chain, fmt.Sprintf("%s    %s", indent, v))
				}
			}
		}
	}
	return result
}

func (tr *unsatTracer) buildSummary(edge implicationEdge, pkgOrder []string) string {
	g := tr.adapter
	if len(pkgOrder) == 1 && len(edge.providers) > 1 {
		return fmt.Sprintf("all %d versions of %s are impossible", len(edge.providers), pkgOrder[0])
	}
	if len(edge.providers) <= 3 {
		parts := make([]string, 0, len(edge.providers))
		for _, prov := range edge.providers {
			parts = append(parts, fmt.Sprintf("%s: %s", g.varName(prov), tr.cache[prov]))
		}
		return strings.Join(parts, "; ")
	}
	return fmt.Sprintf("%d providers all impossible", len(edge.providers))
}

// ExplainWhyUNSAT traverses the implication graph from root candidates to find
// why the problem is unsatisfiable. Returns the dependency chain(s) from root
// to the deepest prohibit clause. O(edges) — no SAT solver call.
func (g *GophersatAdapter) ExplainWhyUNSAT() ExplainUNSATResult {
	if len(g.rootVars) == 0 {
		return ExplainUNSATResult{Lines: []string{"no root candidates registered"}}
	}

	tr := &unsatTracer{adapter: g, cache: make(map[int]string)}

	var rootResults []string
	allRootsDead := true
	for _, rootVar := range g.rootVars {
		r := tr.whyImpossible(rootVar, 0)
		if r == "ok" {
			allRootsDead = false
		} else {
			rootResults = append(rootResults, fmt.Sprintf("  %s: %s", g.varName(rootVar), r))
		}
	}

	var lines []string
	if allRootsDead {
		lines = append(lines, fmt.Sprintf("UNSAT: all %d root candidates are impossible", len(g.rootVars)))
	} else {
		lines = append(lines, "UNSAT: some root candidates impossible (SAT conflict through at-most-one)")
	}

	if len(rootResults) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Root candidates:")
		lines = append(lines, rootResults...)
	}

	if len(tr.chain) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Dependency chain:")
		lines = append(lines, tr.chain...)
	}

	return ExplainUNSATResult{
		Lines:    lines,
		CoreSize: len(tr.chain),
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
