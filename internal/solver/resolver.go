package solver

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/mask"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/state"
)

// ResolveResult is the result of dependency resolution: packages keyed by installation slot.
// Two packages with the same SlotKey cannot coexist; different SlotKeys can.
type PackageAction int

const (
	ActionInstall   PackageAction = iota
	ActionUpgrade                 // version > installed, same slot
	ActionDowngrade               // version < installed, same slot (Portage "UD")
	ActionKeep                    // same version and slot, already installed
	ActionRemove                  // installed but deselected (blocker, slot removal)
)

func (a PackageAction) String() string {
	switch a {
	case ActionInstall:
		return "N"
	case ActionUpgrade:
		return "U"
	case ActionDowngrade:
		return "UD"
	case ActionKeep:
		return "K"
	case ActionRemove:
		return "R"
	default:
		return "?"
	}
}

type ResolveEntry struct {
	Package *pkg.Package
	Action  PackageAction
}
type ResolveResult map[pkg.SlotKey]*ResolveEntry

func (s ResolveResult) PackageAt(key pkg.SlotKey) *pkg.Package {
	if e, ok := s[key]; ok && e != nil {
		return e.Package
	}
	return nil
}

// FindByName returns all packages with the given name across all slots.
func (s ResolveResult) FindByName(name string) []*pkg.Package {
	var result []*pkg.Package
	for key, e := range s {
		if key.Name == name && e != nil {
			result = append(result, e.Package)
		}
	}
	return result
}

// FindByDep returns packages matching dep name and slot constraint.
// If dep.Slot is set and is not an operator ("=", "*"), only matching slots are returned.
func (s ResolveResult) FindByDep(dep pkg.Constraint) []*pkg.Package {
	isOperator := dep.Slot == "=" || dep.Slot == "*"
	var result []*pkg.Package
	for key, e := range s {
		if key.Name != dep.Name {
			continue
		}
		if dep.Slot != "" && !isOperator && key.Slot != dep.Slot {
			continue
		}
		if e != nil {
			result = append(result, e.Package)
		}
	}
	return result
}

// ResolveOptions configures dependency resolution behavior.
type ResolveOptions struct {
	// WithBdeps includes build-time dependencies (BDEPEND) even for installed packages.
	// By default, BDEPEND is skipped for packages already installed.
	// Equivalent to Portage's --with-bdeps=y
	WithBdeps bool

	// Deep traverses dependencies of installed packages.
	// Without this, only dependencies of packages to be installed are followed.
	// Equivalent to Portage's --deep
	Deep bool

	// NewUse reinstalls packages if USE flags have changed.
	// Equivalent to Portage's --newuse
	NewUse bool

	// Update prefers newer versions over installed ones.
	// Without this, installed versions that satisfy constraints are preferred.
	// Equivalent to Portage's --update/-u
	Update bool

	// EmptyTree assumes no packages are installed.
	// Resolves the complete dependency tree from scratch.
	// Equivalent to Portage's --emptytree
	EmptyTree bool
}

// PortageResolver resolves package dependencies using SAT solving.
// It supports package masking and keyword filtering.
type PortageResolver struct {
	repo           repo.Repository
	maskManager    *mask.MaskManager
	acceptKeywords []string // ACCEPT_KEYWORDS from make.conf (e.g., ["amd64", "~amd64"])

	// installedDB is the database of installed packages.
	// When set, resolver skips dependencies that are already satisfied.
	installedDB *state.PackageDatabase

	// options configures resolution behavior.
	options ResolveOptions

	// PostPassAdded counts packages added by post-SAT safety net.
	// Should be 0 if SAT encoding is complete. Non-zero signals a gap.
	PostPassAdded int

	// PackagesExplored counts unique package names loaded during collection.
	// Measures exploration cost, not result size.
	PackagesExplored int

	// orExpansionLevel tracks how many OR-group alternatives to expand per group.
	// Key: "pkgname:groupID". Value: number of alternatives to fully explore (default 1).
	// On UNSAT retry, the level is incremented for groups whose preferred alternative
	// was prohibited, allowing the next alternative's deps to be explored.
	orExpansionLevel map[string]int
}

// NewResolver creates a new resolver without mask/keyword support.
// For full filtering, use NewResolverWithFilters.
func NewResolver(r repo.Repository) *PortageResolver {
	return &PortageResolver{repo: r}
}

// NewResolverWithMasks creates a new resolver with mask filtering support.
// Masked packages will be excluded from solver consideration.
// Deprecated: Use NewResolverWithFilters for both mask and keyword filtering.
func NewResolverWithMasks(r repo.Repository, maskMgr *mask.MaskManager) *PortageResolver {
	return &PortageResolver{
		repo:        r,
		maskManager: maskMgr,
	}
}

// NewResolverWithFilters creates a new resolver with both mask and keyword filtering.
// - maskMgr: filters packages from package.mask
// - acceptKeywords: filters packages by KEYWORDS (e.g., ["amd64", "~amd64"])
func NewResolverWithFilters(r repo.Repository, maskMgr *mask.MaskManager, acceptKeywords []string) *PortageResolver {
	return &PortageResolver{
		repo:           r,
		maskManager:    maskMgr,
		acceptKeywords: acceptKeywords,
	}
}

// SetInstalledDB sets the installed packages database.
//
// When set, the resolver will skip dependencies that are already satisfied
// by installed packages. This matches Portage's behavior of only showing
// packages that need to be installed.
//
// Without this, the resolver returns the complete transitive dependency tree.
func (r *PortageResolver) SetInstalledDB(db *state.PackageDatabase) {
	r.installedDB = db
}

// SetOptions sets the resolution options.
func (r *PortageResolver) SetOptions(opts ResolveOptions) {
	r.options = opts
}

// isInstalled checks if a package is already installed.
// Returns false if no installed database is set or EmptyTree is enabled.
func (r *PortageResolver) isInstalled(name string) bool {
	if r.installedDB == nil || r.options.EmptyTree {
		return false
	}
	return r.installedDB.IsInstalled(name)
}

// sortAlternativesByInstalled reorders OR-group alternatives to put
// already-installed packages first. This creates a preference for keeping
// installed packages when the SAT solver picks from OR alternatives.
func (r *PortageResolver) sortAlternativesByInstalled(alternatives []pkg.Constraint) []pkg.Constraint {
	sorted := make([]pkg.Constraint, 0, len(alternatives))
	var notInstalled []pkg.Constraint

	for _, alt := range alternatives {
		if r.isInstalled(alt.Name) {
			sorted = append(sorted, alt)
		} else {
			notInstalled = append(notInstalled, alt)
		}
	}

	return append(sorted, notInstalled...)
}

// groupDependenciesByOrGroupID groups dependencies by their OrGroupID
// Returns required dependencies (OrGroupID=0) and OR-groups (OrGroupID>0)
// isBuildTimeDep returns true for DEPEND and BDEPEND.
func isBuildTimeDep(depType pkg.DepType) bool {
	return depType == pkg.DepTypeBuild || depType == pkg.DepTypeBuildHost
}

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

// collectDependencies is best-effort: it explores the search space for SAT.
// Missing deps are logged, not fatal — SAT encoding determines satisfiability.
//
//nolint:gocyclo // Complexity inherent to Portage-compatible dependency resolution algorithm
func (r *PortageResolver) collectDependencies(p *pkg.Package, allPackages map[string]*pkg.Package, allCandidates map[string][]*pkg.Package) {
	versionKey := p.Name + "@" + p.Version
	if _, exists := allPackages[versionKey]; exists {
		return // Already processed
	}

	// Check if this package is masked
	if r.isMasked(p) {
		logging.Debug("Skipping masked package: %s-%s", p.Name, p.Version)
		return
	}

	// Store a copy of the package
	copyPkg := *p
	allPackages[versionKey] = &copyPkg

	// Load all candidate versions for this package into allCandidates
	r.addCandidateVersions(p.Name, allCandidates)

	// Register blocker targets as candidates so they have SAT variables.
	// Without this, packages referenced only by !atom (not by deps) never
	// enter the graph → no conflict clause → ActionRemove won't fire.
	for _, blocker := range p.Blockers {
		if blocker.Atom != nil {
			r.addCandidateVersions(blocker.Atom.CP(), allCandidates)
		}
	}

	// Group dependencies by OrGroupID
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	// Process REQUIRED dependencies only.
	// Note: BDEPEND skip for installed packages removed for multi-version SAT.
	// SAT needs ALL deps explored to build correct implications. Without BDEPEND
	// providers in the adapter, implication clauses emit prohibit on ALL candidates.
	// Portage's BDEPEND optimization (skip for installed) belongs in the result
	// filtering phase, not in SAT exploration.
	for _, dep := range requiredDeps {
		// SAT must see all providers to build correct implication clauses.

		// Load ALL candidate versions and recursively explore each one's deps.
		// SAT needs transitive deps of ALL candidates, not just the highest.
		r.addCandidateVersions(dep.Name, allCandidates)

		if candidates, ok := allCandidates[dep.Name]; ok {
			for _, candidate := range candidates {
				r.collectDependencies(candidate, allPackages, allCandidates)
			}
		}
	}

	// Lazy OR expansion: expand only the first N preferred alternatives,
	// where N = orExpansionLevel[key] (default 1). Others get candidates
	// registered (for SAT variable creation) but deps NOT explored. An
	// unexpanded alternative without dep exploration will get prohibit
	// clauses in addPackageConstraints (no providers for its deps).
	// If SAT is UNSAT, Resolve() retries with expanded level incremented.
	for groupID, alternatives := range orGroups {
		logging.Debug("OR-group %d for %s: %d alternatives", groupID, p.Name, len(alternatives))
		sorted := r.sortAlternativesByInstalled(alternatives)

		orKey := fmt.Sprintf("%s:%d", p.Name, groupID)
		expandLevel := 1
		if r.orExpansionLevel != nil {
			if lvl, ok := r.orExpansionLevel[orKey]; ok && lvl > expandLevel {
				expandLevel = lvl
			}
		}

		for i, alt := range sorted {
			// Register candidates for SAT variable creation
			r.addCandidateVersions(alt.Name, allCandidates)

			// Explore deps of alternatives up to the expansion level
			if i < expandLevel {
				if candidates, ok := allCandidates[alt.Name]; ok {
					for _, candidate := range candidates {
						r.collectDependencies(candidate, allPackages, allCandidates)
					}
				}
			}
			// Non-expanded alternatives: candidates registered but deps not explored.
			// addPackageConstraints will emit prohibit for their unresolvable deps,
			// making SAT prefer the expanded alternative.
		}
	}
}

// addCandidateVersions loads all versions of a package from the repository
// and stores them in allCandidates. Does nothing if the package has already
// been loaded. Masked/unkeyworded versions are filtered out.
func (r *PortageResolver) addCandidateVersions(name string, allCandidates map[string][]*pkg.Package) {
	if _, exists := allCandidates[name]; exists {
		return // Already loaded
	}
	r.PackagesExplored++

	versions, err := r.repo.GetAllVersions(name)
	if err != nil {
		logging.Debug("Warning: failed to get versions for %s: %v", name, err)
		// Still mark as visited so installed candidates can be injected later
		allCandidates[name] = nil
		return
	}

	filtered := r.filterMaskedPackages(versions)
	if len(filtered) == 0 {
		logging.Debug("Warning: all versions of %s are masked/unkeyworded", name)
		// Still mark the name as visited so addInstalledCandidates can inject
		// installed versions that may have been removed from the repo.
		allCandidates[name] = nil
		return
	}

	// Sort by version descending (newest first) for consistent SAT variable ordering
	sort.Slice(filtered, func(i, j int) bool {
		return pkg.CompareVersions(filtered[i].Version, filtered[j].Version) > 0
	})

	allCandidates[name] = filtered
	logging.Debug("Loaded %d candidate versions for %s", len(filtered), name)
}

// addInstalledCandidates injects installed packages from VDB as SAT candidates.
// For each installed package whose name appears in allCandidates (i.e., is part of the
// dependency graph), the installed version is added as an additional candidate if it
// isn't already present. This allows the SAT solver to choose to keep installed
// versions — prerequisite for MAX-SAT optimization (task 014).
//
// Installed versions that no longer exist in the repo are still added as candidates,
// since the solver needs them to model "keep current installation" scenarios.
//
// Returns a list of SAT variable keys ("name@version") for all installed candidates,
// so the caller can mark them via adapter.MarkInstalled().
func (r *PortageResolver) addInstalledCandidates(allCandidates map[string][]*pkg.Package, allPackages map[string]*pkg.Package) []string {
	if r.installedDB == nil || r.options.EmptyTree {
		return nil
	}

	var installedKeys []string

	installedPkgs := r.installedDB.List()
	for _, ip := range installedPkgs {
		if ip.Package == nil {
			continue
		}

		name := ip.Package.Name
		version := ip.Package.Version

		// Only add installed packages that are part of the dependency graph
		// (i.e., their name is referenced in allCandidates).
		candidates, inGraph := allCandidates[name]
		if !inGraph {
			continue
		}

		// Check if this version is already a candidate
		alreadyPresent := false
		for _, c := range candidates {
			if c.Version == version {
				alreadyPresent = true
				break
			}
		}

		versionKey := name + "@" + version
		if !alreadyPresent {
			// Create a Package from the installed metadata
			installedPkg := ip.Package
			allCandidates[name] = append(allCandidates[name], installedPkg)
			allPackages[versionKey] = installedPkg
			logging.Debug("Added installed package as SAT candidate: %s-%s", name, version)
		}

		installedKeys = append(installedKeys, versionKey)
	}

	if len(installedKeys) > 0 {
		logging.Debug("Injected %d installed packages as SAT candidates", len(installedKeys))
	}

	return installedKeys
}

// addPackageConstraints adds all constraints for a single package version to the SAT solver.
// Uses implication clauses: if this version is selected, its deps must be satisfied.
// Root packages get an at-least-one clause (unit clause for single version).
func (r *PortageResolver) addPackageConstraints(adapter *GophersatAdapter, p *pkg.Package, rootPackages []string) {
	pkgKey := p.Name + "@" + p.Version
	pkgVarID := adapter.GetVarID(pkgKey)

	// Root packages: at-least-one of their versions must be selected.
	// This is handled separately in addRootConstraints, not here.
	// But if this specific version IS a root package and the only version,
	// we still add it here for backward compat with single-version repos.
	if contains(rootPackages, p.Name) && pkgVarID == 0 {
		// Package not registered in SAT — fallback to old behavior
		logging.Debug("Warning: root package %s not registered in SAT adapter", p.Name)
		return
	}

	// If this package has no SAT variable (shouldn't happen), skip
	if pkgVarID == 0 {
		return
	}

	// Group dependencies by OrGroupID
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	// Skip BDEPEND implications for installed (Keep) candidates.
	// Installed packages are already built — they don't need build deps.
	// Tree candidates still need BDEPEND (they will be built).
	// Skip BDEPEND only if installed AND not rebuilding (--newuse forces rebuild)
	skipBDEPEND := adapter.IsVarInstalled(pkgVarID) && !r.options.NewUse

	// Add REQUIRED dependencies as implications: (-P@V | B1 | B2 | ...)
	for _, dep := range requiredDeps {
		if skipBDEPEND && isBuildTimeDep(dep.DepType) {
			continue
		}
		versionStr := "any"
		if dep.Version != nil {
			versionStr = dep.Version.String()
		}
		logging.Debug("Adding implication: %s => %s %s", pkgKey, dep.Name, versionStr)

		switch dep.Type {
		case pkg.ConstraintTypeSlot:
			if err := adapter.AddImplicationSlotConstraint(pkgVarID, dep); err != nil {
				logging.Debug("Warning: failed to add slot implication: %v", err)
			}
		default:
			if err := adapter.AddImplicationConstraint(pkgVarID, dep); err != nil {
				logging.Debug("Warning: failed to add implication: %v", err)
			}
		}
	}

	// Add OR-group constraints as implications.
	for groupID, alternatives := range orGroups {
		// Skip BDEPEND OR-groups for installed candidates
		if skipBDEPEND && len(alternatives) > 0 && isBuildTimeDep(alternatives[0].DepType) {
			continue
		}
		logging.Debug("Adding OR-group %d implication from %s with %d alternatives",
			groupID, pkgKey, len(alternatives))
		sorted := r.sortAlternativesByInstalled(alternatives)
		if err := adapter.AddImplicationOrGroup(pkgVarID, sorted); err != nil {
			logging.Debug("Warning: failed to add OR-group implication: %v", err)
		}
	}

	// Add blocker conflict clauses: if this package blocks another,
	// they cannot coexist in the solution.
	// Strong blockers ("!!") always emit hard conflict (-A|-B).
	// Weak blockers ("!") only conflict when an installed side is present.
	for _, blocker := range p.Blockers {
		if blocker.Atom == nil {
			continue
		}
		// Both ! and !! emit (-A|-B) in SAT. PMS 8.2.6.6: weak blockers allow
		// temporary coexistence during merge transaction, but the final state
		// prohibits both. IsStrong is an annotation for the merge planner, not
		// the solver. The installed flag is used for MAX-SAT preference weights.
		logging.Debug("Adding blocker from %s: blocks %s (strong=%v)", pkgKey, blocker.Atom.String(), blocker.IsStrong)
		adapter.AddBlockerConflict(pkgVarID, blocker.Atom)
	}
}

// addRootConstraints adds at-least-one constraints for root packages.
// Filters candidates by the user's atom (e.g., =foo-1.0 only matches 1.0).
// Allows SAT to backtrack among matching versions if one is unsatisfiable.
func (r *PortageResolver) addRootConstraints(adapter *GophersatAdapter, rootPackages map[string]*pkg.Package, rootAtoms map[string]string) {
	for name, p := range rootPackages {
		var candidateVars []int
		atomStr := rootAtoms[name]
		atom, parseErr := pkg.ParseAtom(atomStr)

		if versions, ok := adapter.packages[p.Name]; ok {
			for _, v := range versions {
				// Filter by atom if user specified version or slot constraint
				if parseErr == nil && (atom.HasVersion() || atom.Slot != "") {
					if !atom.Matches(v) {
						continue
					}
				}
				key := v.Name + "@" + v.Version
				if varID := adapter.GetVarID(key); varID != 0 {
					candidateVars = append(candidateVars, varID)
				}
			}
		}
		if len(candidateVars) == 0 {
			if varID := adapter.GetVarID(p.Name + "@" + p.Version); varID != 0 {
				candidateVars = []int{varID}
			}
		}
		if len(candidateVars) > 0 {
			adapter.withMeta(ClauseRoot, fmt.Sprintf("root: at-least-one of %s (atom=%s)", p.Name, atomStr))
			adapter.addClause(candidateVars)
			adapter.addRootVars(candidateVars)
			logging.Debug("Added root at-least-one for %s (%d candidates, atom=%s)", p.Name, len(candidateVars), atomStr)
		}
	}
}

func (r *PortageResolver) determineAction(p *pkg.Package) PackageAction {
	if r.installedDB == nil {
		return ActionInstall
	}
	// Find installed package in the SAME slot.
	// GetInstalledVersion returns first by name regardless of slot, so we scan
	// the full list to find a match in the same slot.
	installed := r.findInstalledInSlot(p.Name, p.Slot.Name)
	if installed == nil {
		return ActionInstall // new slot, nothing installed there
	}
	if installed.Version == p.Version {
		if r.options.NewUse {
			return ActionUpgrade // same version, rebuild for USE changes
		}
		return ActionKeep
	}
	cmp := pkg.CompareVersions(p.Version, installed.Version)
	if cmp > 0 {
		return ActionUpgrade
	}
	return ActionDowngrade
}

// findInstalledInSlot returns the installed package matching name AND slot.
// Returns nil if no match. This is slot-aware unlike GetInstalledVersion.
func (r *PortageResolver) findInstalledInSlot(name, slot string) *pkg.Package {
	if r.installedDB == nil {
		return nil
	}
	for _, ip := range r.installedDB.List() {
		if ip.Package == nil {
			continue
		}
		if ip.Package.Name == name && ip.Package.Slot.Name == slot {
			return ip.Package
		}
	}
	return nil
}

// buildResultFromSolution builds the final result map from the SAT solution.
// The solution map contains package names as keys and selected versions as values.
// allCandidates provides a fallback for installed packages that may not be in the repo.
func (r *PortageResolver) buildResultFromSolution(solution map[string]string, allCandidates map[string][]*pkg.Package) (ResolveResult, error) {
	result := make(ResolveResult)
	for key, version := range solution {
		// Parse package name from SAT variable key (name@version)
		name := key
		if idx := strings.Index(key, "@"); idx >= 0 {
			name = key[:idx]
		}
		// Load the specific version that was selected by the SAT solver
		p, err := r.repo.LoadPackageVersion(name, version)
		if err != nil {
			// Fallback to LoadPackage if LoadPackageVersion fails
			p, err = r.repo.LoadPackage(name)
			if err != nil {
				// Final fallback: check allCandidates (covers installed-only packages
				// that were removed from the repo but are still in VDB)
				if candidates, ok := allCandidates[name]; ok {
					for _, c := range candidates {
						if c.Version == version {
							p = c
							break
						}
					}
				}
				if p == nil {
					logging.Debug("Warning: package %s not found: %v", name, err)
					continue
				}
			}
		}
		result[pkg.SlotKeyOf(p)] = &ResolveEntry{Package: p, Action: r.determineAction(p)}
	}
	return result, nil
}

// loadPackageFromAtom parses a package atom string and loads the matching package.
// Supports PMS-compliant atoms like "=sys-devel/gcc-13.4.1_p20250807" or ">=dev-libs/openssl-3.0".
// If no version operator is specified, loads the highest available unmasked version.
// Masked packages are filtered out unless explicitly requested by exact version.
func (r *PortageResolver) loadPackageFromAtom(atomStr string) (*pkg.Package, error) {
	// Try to parse as a full atom first
	atom, err := pkg.ParseAtom(atomStr)
	if err != nil {
		// If parsing fails, it might be a simple "category/package" string
		// Use loadUnmaskedPackage to filter masked versions
		return r.loadUnmaskedPackage(atomStr)
	}

	// If atom has a version or slot constraint, use FindByAtom to get matching packages
	if atom.HasVersion() || atom.Slot != "" {
		matches, err := r.repo.FindByAtom(atom)
		if err != nil {
			return nil, fmt.Errorf("failed to find packages matching %s: %w", atomStr, err)
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("no packages match atom %s", atomStr)
		}

		// Filter out masked packages (unless this is an exact version request)
		// For exact matches (=), we respect the user's explicit request
		if atom.Operator != "=" {
			matches = r.filterMaskedPackages(matches)
			if len(matches) == 0 {
				return nil, fmt.Errorf("all matching packages for %s are masked", atomStr)
			}
		} else {
			// For exact match, warn if masked but still return it
			if len(matches) == 1 && r.isMasked(matches[0]) {
				logging.Debug("Warning: explicitly requested package %s is masked", atomStr)
			}
		}

		// Sort matches by version (highest first) and return the best match
		sort.Slice(matches, func(i, j int) bool {
			return pkg.CompareVersions(matches[i].Version, matches[j].Version) > 0
		})

		// For exact match (=), return the only match
		// For range operators (>=, >, <=, <), return the highest matching version
		// For ~ (revision match), return the highest matching revision
		logging.Debug("Atom %s matched %d packages, selected %s-%s",
			atomStr, len(matches), matches[0].Name, matches[0].Version)

		return matches[0], nil
	}

	// No version constraint - load best unmasked version
	return r.loadUnmaskedPackage(atom.CP())
}

// maxOrRetries bounds the lazy OR expansion retry loop.
// Each retry expands one more alternative in at least one OR-group.
// 10 is generous — most real OR-groups have 2-3 alternatives.
const maxOrRetries = 10

//nolint:gocyclo // Complexity inherent to multi-pass Portage-compatible resolution with OR-group support
func (r *PortageResolver) Resolve(packages []string) (ResolveResult, error) {
	r.PostPassAdded = 0
	r.PackagesExplored = 0

	// Track root packages: name -> the specific version selected by loadPackageFromAtom.
	rootPackageNames := make([]string, 0, len(packages))
	rootPackagesMap := make(map[string]*pkg.Package)
	rootAtoms := make(map[string]string) // name -> original atom string

	// Load root packages (does not change across retries).
	for _, pkgName := range packages {
		p, err := r.loadPackageFromAtom(pkgName)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", pkgName, err)
		}
		rootPackageNames = append(rootPackageNames, p.Name)
		rootPackagesMap[p.Name] = p
		rootAtoms[p.Name] = pkgName
		logging.Debug("Resolving package: %s-%s with %d dependencies",
			p.Name, p.Version, len(p.Deps))
	}

	// Initialize lazy OR expansion levels (all start at 1 = first alternative only).
	if r.orExpansionLevel == nil {
		r.orExpansionLevel = make(map[string]int)
	}

	// Retry loop: collect deps + build SAT + solve.
	// On UNSAT caused by lazy OR (unexpanded alternative prohibited),
	// increment expansion level and rebuild from scratch.
	var lastAdapter *GophersatAdapter

	for attempt := 0; attempt <= maxOrRetries; attempt++ {
		if attempt > 0 {
			logging.Info("OR-expansion retry %d: expanding additional alternatives", attempt)
		}

		adapter := NewGophersatAdapter()
		allPackages := make(map[string]*pkg.Package)
		allCandidates := make(map[string][]*pkg.Package)

		// Reset exploration counter for this attempt
		r.PackagesExplored = 0

		// Collect dependencies with current orExpansionLevel
		for _, p := range rootPackagesMap {
			r.addCandidateVersions(p.Name, allCandidates)
			if candidates, ok := allCandidates[p.Name]; ok {
				for _, candidate := range candidates {
					r.collectDependencies(candidate, allPackages, allCandidates)
				}
			}
		}

		logging.Debug("Total packages in dependency graph: %d", len(allPackages))
		logging.Debug("Total candidate packages: %d names", len(allCandidates))

		// Inject installed packages (VDB) as SAT candidates.
		installedKeys := r.addInstalledCandidates(allCandidates, allPackages)

		// Register ALL candidate versions with the SAT adapter.
		for _, candidates := range allCandidates {
			for _, c := range candidates {
				adapter.AddPackage(c)
			}
		}
		for _, p := range allPackages {
			adapter.AddPackage(p)
		}

		// Mark installed versions in the adapter.
		for _, key := range installedKeys {
			if varID := adapter.GetVarID(key); varID != 0 {
				adapter.MarkInstalled(varID)
			}
		}

		// Add root constraints
		r.addRootConstraints(adapter, rootPackagesMap, rootAtoms)

		// Add implication constraints for each candidate version.
		for _, candidates := range allCandidates {
			for _, c := range candidates {
				r.addPackageConstraints(adapter, c, rootPackageNames)
			}
		}
		for _, p := range allPackages {
			if _, hasCandidates := allCandidates[p.Name]; !hasCandidates {
				r.addPackageConstraints(adapter, p, rootPackageNames)
			}
		}

		// Add at-most-one-per-slot pairwise exclusion clauses
		adapter.AddAtMostOnePerSlot()

		logging.Debug("Total clauses in SAT problem: %d", len(adapter.clauses))

		// Solve
		const maxsatTimeout = 5 * time.Second
		updateMode := r.options.Update
		status, solution, err := adapter.SolveOptimal(maxsatTimeout, updateMode)
		if err != nil {
			return nil, err
		}

		if status == pkg.StatusSat {
			result, buildErr := r.buildResultFromSolution(solution, allCandidates)
			if buildErr != nil {
				return nil, buildErr
			}
			return r.postSATSafetyNet(result, adapter)
		}

		// UNSAT — check if lazy OR expansion can help.
		lastAdapter = adapter
		expanded := r.tryExpandOrGroups(adapter, allPackages)
		if !expanded {
			break
		}
	}

	// Genuine UNSAT — explain and return error
	if lastAdapter != nil {
		explanation := lastAdapter.ExplainWhyUNSAT()
		for _, line := range explanation.Lines {
			logging.Info("%s", line)
		}
	}
	return nil, fmt.Errorf("no solution found (UNSAT, see explanation above)")
}

// tryExpandOrGroups checks if any OR-group has an unexpanded alternative
// that was prohibited due to missing dep exploration. If found, increments
// the expansion level for that group and returns true (caller should retry).
func (r *PortageResolver) tryExpandOrGroups(adapter *GophersatAdapter, allPackages map[string]*pkg.Package) bool {
	prohibitedNames := adapter.ProhibitedPackageNames()
	if len(prohibitedNames) == 0 {
		return false
	}

	expanded := false
	for _, p := range allPackages {
		_, orGroups := groupDependenciesByOrGroupID(p.Deps)
		for groupID, alternatives := range orGroups {
			orKey := fmt.Sprintf("%s:%d", p.Name, groupID)
			currentLevel := 1
			if lvl, ok := r.orExpansionLevel[orKey]; ok {
				currentLevel = lvl
			}
			if currentLevel >= len(alternatives) {
				continue
			}
			sorted := r.sortAlternativesByInstalled(alternatives)
			for i := currentLevel; i < len(sorted); i++ {
				if prohibitedNames[sorted[i].Name] {
					r.orExpansionLevel[orKey] = currentLevel + 1
					logging.Info("OR-group %s: expanding alternative %d (%s) after UNSAT",
						orKey, currentLevel, sorted[i].Name)
					expanded = true
					break
				}
			}
		}
	}

	// Fallback: all expanded alternatives are prohibited — try next one.
	if !expanded {
		for _, p := range allPackages {
			_, orGroups := groupDependenciesByOrGroupID(p.Deps)
			for groupID, alternatives := range orGroups {
				orKey := fmt.Sprintf("%s:%d", p.Name, groupID)
				currentLevel := 1
				if lvl, ok := r.orExpansionLevel[orKey]; ok {
					currentLevel = lvl
				}
				if currentLevel >= len(alternatives) {
					continue
				}
				sorted := r.sortAlternativesByInstalled(alternatives)
				allExpandedProhibited := true
				for i := 0; i < currentLevel && i < len(sorted); i++ {
					if !prohibitedNames[sorted[i].Name] {
						allExpandedProhibited = false
						break
					}
				}
				if allExpandedProhibited {
					r.orExpansionLevel[orKey] = currentLevel + 1
					nextAlt := "?"
					if currentLevel < len(sorted) {
						nextAlt = sorted[currentLevel].Name
					}
					logging.Info("OR-group %s: all expanded alternatives prohibited, trying %s",
						orKey, nextAlt)
					expanded = true
				}
			}
		}
	}

	return expanded
}

// postSATSafetyNet runs the post-SAT fixup passes.
// Should add ZERO packages if SAT encoding is complete.
func (r *PortageResolver) postSATSafetyNet(result ResolveResult, adapter *GophersatAdapter) (ResolveResult, error) {
	for pass := 1; pass <= 10; pass++ {
		added := 0
		currentEntries := make([]*ResolveEntry, 0, len(result))
		for _, ent := range result {
			currentEntries = append(currentEntries, ent)
		}
		for _, ent := range currentEntries {
			added += r.fillMissingDeps(ent, result)
		}
		r.PostPassAdded += added
		if added > 0 {
			logging.Info("WARNING: Post-SAT pass %d added %d packages — SAT encoding incomplete", pass, added)
		}
		if added == 0 {
			break
		}
	}

	r.addRemovedPackages(result, adapter)

	logging.Info("Resolved packages:")
	for key, entry := range result {
		logging.Debug("- %s-%s [slot:%s action:%s]", entry.Package.Name, entry.Package.Version, key.Slot, entry.Action)
	}

	return result, nil
}

// fillMissingDeps adds missing deps for one result entry. Returns count added.
// Skips BDEPEND for Keep (installed) packages — they're already built.
func (r *PortageResolver) fillMissingDeps(ent *ResolveEntry, result ResolveResult) int {
	added := 0
	p := ent.Package
	isKeep := ent.Action == ActionKeep
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	for _, dep := range requiredDeps {
		if isKeep && isBuildTimeDep(dep.DepType) {
			continue
		}
		depPkg, err := r.loadUnmaskedPackage(dep.Name)
		if err != nil {
			continue
		}
		depKey := pkg.SlotKeyOf(depPkg)
		if _, inResult := result[depKey]; inResult {
			continue
		}
		result[depKey] = &ResolveEntry{Package: depPkg, Action: r.determineAction(depPkg)}
		added++
	}

	for _, alternatives := range orGroups {
		if isKeep && len(alternatives) > 0 && isBuildTimeDep(alternatives[0].DepType) {
			continue
		}
		satisfied := false
		for _, alt := range alternatives {
			altPkg, altErr := r.loadUnmaskedPackage(alt.Name)
			if altErr != nil {
				continue
			}
			if _, inResult := result[pkg.SlotKeyOf(altPkg)]; inResult {
				satisfied = true
				break
			}
		}
		if satisfied {
			continue
		}
		for _, alt := range alternatives {
			altPkg, err := r.loadUnmaskedPackage(alt.Name)
			if err != nil {
				continue
			}
			result[pkg.SlotKeyOf(altPkg)] = &ResolveEntry{Package: altPkg, Action: r.determineAction(altPkg)}
			added++
			break
		}
	}

	return added
}

// addRemovedPackages marks installed packages for removal ONLY when they are
// deselected by a blocker conflict clause. "Not selected" alone does not mean
// "remove" — python:3.12 installed while python:3.13 is requested should NOT
// remove 3.12 (that's a different slot, Portage shows NS for the new one).
//
// Rule: installed && !selected && ∃ conflict(selected, installed) → Remove.
func (r *PortageResolver) addRemovedPackages(result ResolveResult, adapter *GophersatAdapter) {
	if r.installedDB == nil || r.options.EmptyTree || adapter == nil {
		return
	}
	for _, ip := range r.installedDB.List() {
		if ip.Package == nil {
			continue
		}
		key := pkg.SlotKeyOf(ip.Package)
		if _, inResult := result[key]; inResult {
			continue // already in result — not removed
		}
		// Check if this installed package has a conflict clause with any selected package
		installedKey := ip.Package.Name + "@" + ip.Package.Version
		installedVarID := adapter.GetVarID(installedKey)
		if installedVarID == 0 {
			continue // not a SAT variable — not in the dependency graph
		}
		if adapter.HasConflictWith(installedVarID, result) {
			result[key] = &ResolveEntry{Package: ip.Package, Action: ActionRemove}
			logging.Debug("Added ActionRemove for installed %s-%s (blocker conflict)",
				ip.Package.Name, ip.Package.Version)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isMasked checks if a package is masked using the mask manager.
// Returns false if no mask manager is configured.
func (r *PortageResolver) isMasked(p *pkg.Package) bool {
	if r.maskManager == nil || p == nil {
		return false
	}
	return r.maskManager.IsPackageMasked(p)
}

// filterMaskedPackages filters out masked packages from the list.
// If mask manager is not configured, returns the original list.
func (r *PortageResolver) filterMaskedPackages(packages []*pkg.Package) []*pkg.Package {
	var filtered []*pkg.Package

	for _, p := range packages {
		// Check package.mask filtering
		if r.maskManager != nil && r.maskManager.IsPackageMasked(p) {
			logging.Debug("Filtered masked package: %s-%s", p.Name, p.Version)
			continue
		}

		// Check KEYWORDS filtering
		if r.isKeywordMasked(p) {
			logging.Debug("Filtered unkeyworded package: %s-%s (KEYWORDS: %v)", p.Name, p.Version, p.Keywords)
			continue
		}

		filtered = append(filtered, p)
	}

	return filtered
}

// isKeywordMasked returns true if the package is masked due to KEYWORDS.
// A package is keyword-masked if:
// - It has no KEYWORDS (unkeyworded/live package)
// - Its KEYWORDS don't match ACCEPT_KEYWORDS
func (r *PortageResolver) isKeywordMasked(p *pkg.Package) bool {
	// No keyword filtering configured - accept all
	if len(r.acceptKeywords) == 0 {
		return false
	}

	// Use package's built-in method for keyword checking
	return !p.IsKeywordAccepted(r.acceptKeywords)
}

// loadUnmaskedPackage loads a package, filtering out masked and unkeyworded versions.
// If the highest version is masked/unkeyworded, it tries to find an acceptable version.
func (r *PortageResolver) loadUnmaskedPackage(name string) (*pkg.Package, error) {
	// First try loading the package normally
	p, err := r.repo.LoadPackage(name)
	if err != nil {
		return nil, err
	}

	// Check if the highest version is acceptable (not masked, keywords accepted)
	if r.isPackageAcceptable(p) {
		return p, nil
	}

	// The highest version is masked/unkeyworded - try to find an acceptable version
	versions, err := r.repo.GetAllVersions(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get versions for %s: %w", name, err)
	}

	// Filter out masked and unkeyworded versions
	acceptableVersions := r.filterMaskedPackages(versions)
	if len(acceptableVersions) == 0 {
		// All versions are masked or unkeyworded
		return nil, r.buildMaskError(p)
	}

	// Sort by version (highest first) and return the best acceptable version
	sort.Slice(acceptableVersions, func(i, j int) bool {
		return pkg.CompareVersions(acceptableVersions[i].Version, acceptableVersions[j].Version) > 0
	})

	logging.Debug("Package %s-%s is masked/unkeyworded, using %s-%s instead",
		name, p.Version, name, acceptableVersions[0].Version)

	return acceptableVersions[0], nil
}

// isPackageAcceptable returns true if the package passes all filters (mask and keywords).
func (r *PortageResolver) isPackageAcceptable(p *pkg.Package) bool {
	// Check package.mask
	if r.maskManager != nil && r.maskManager.IsPackageMasked(p) {
		return false
	}

	// Check KEYWORDS
	if r.isKeywordMasked(p) {
		return false
	}

	return true
}

// buildMaskError creates a descriptive error for why a package is masked.
func (r *PortageResolver) buildMaskError(p *pkg.Package) error {
	// Check if it's masked by package.mask
	if r.maskManager != nil && r.maskManager.IsPackageMasked(p) {
		atom, source := r.maskManager.GetMaskReason(
			extractCategory(p.Name),
			extractPackageName(p.Name),
			p.Version,
			p.Slot.Name,
		)
		return fmt.Errorf("all versions of %s are masked (by %s: %s)", p.Name, source, atom)
	}

	// Check if it's masked by keywords
	if r.isKeywordMasked(p) {
		if len(p.Keywords) == 0 {
			return fmt.Errorf("all versions of %s are unkeyworded (missing KEYWORDS)", p.Name)
		}
		return fmt.Errorf("all versions of %s are keyword-masked (KEYWORDS=%v, ACCEPT_KEYWORDS=%v)",
			p.Name, p.Keywords, r.acceptKeywords)
	}

	return fmt.Errorf("all versions of %s are masked", p.Name)
}

// extractCategory extracts category from "category/package" format.
func extractCategory(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// extractPackageName extracts package name from "category/package" format.
func extractPackageName(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return name
}
