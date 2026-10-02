package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/mask"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/state"
)

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


//nolint:gocyclo // Complexity inherent to Portage-compatible dependency resolution algorithm
// collectDependencies is best-effort: it explores the search space for SAT.
// Missing deps are logged, not fatal — SAT encoding determines satisfiability.
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

	// For OR-groups: add alternative packages to allPackages (so SAT solver knows
	// about them), but DON'T recursively collect their dependencies yet.
	// After SAT solving, we'll collect deps for chosen alternatives in a second pass.
	for groupID, alternatives := range orGroups {
		logging.Debug("OR-group %d for %s: %d alternatives", groupID, p.Name, len(alternatives))
		for _, alt := range alternatives {
			// Note: BDEPEND and installed-alternative skips removed for multi-version SAT.
			// SAT must see all alternatives to build correct OR-group implications.

			// Load ALL candidate versions and explore deps (same as required deps).
			// With implication clauses, SAT handles "only if chosen" — we must
			// explore so SAT has the transitive dep vars to build implications.
			r.addCandidateVersions(alt.Name, allCandidates)
			if candidates, ok := allCandidates[alt.Name]; ok {
				for _, candidate := range candidates {
					r.collectDependencies(candidate, allPackages, allCandidates)
				}
			}
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

	versions, err := r.repo.GetAllVersions(name)
	if err != nil {
		logging.Debug("Warning: failed to get versions for %s: %v", name, err)
		return
	}

	filtered := r.filterMaskedPackages(versions)
	if len(filtered) == 0 {
		logging.Debug("Warning: all versions of %s are masked/unkeyworded", name)
		return
	}

	// Sort by version descending (newest first) for consistent SAT variable ordering
	sort.Slice(filtered, func(i, j int) bool {
		return pkg.CompareVersions(filtered[i].Version, filtered[j].Version) > 0
	})

	allCandidates[name] = filtered
	logging.Debug("Loaded %d candidate versions for %s", len(filtered), name)
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

	// Add REQUIRED dependencies as implications: (-P@V | B1 | B2 | ...)
	for _, dep := range requiredDeps {
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
		logging.Debug("Adding OR-group %d implication from %s with %d alternatives",
			groupID, pkgKey, len(alternatives))
		sorted := r.sortAlternativesByInstalled(alternatives)
		if err := adapter.AddImplicationOrGroup(pkgVarID, sorted); err != nil {
			logging.Debug("Warning: failed to add OR-group implication: %v", err)
		}
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
			logging.Debug("Added root at-least-one for %s (%d candidates, atom=%s)", p.Name, len(candidateVars), atomStr)
		}
	}
}

// buildResultFromSolution builds the final result map from the SAT solution.
// The solution map contains package names as keys and selected versions as values.
func (r *PortageResolver) buildResultFromSolution(solution map[string]string) (map[string]*pkg.Package, error) {
	result := make(map[string]*pkg.Package)
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
				logging.Debug("Warning: package %s not found: %v", name, err)
				continue
			}
		}
		result[packageSlotKey(p)] = p
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

//nolint:gocyclo // Complexity inherent to multi-pass Portage-compatible resolution with OR-group support
func (r *PortageResolver) Resolve(packages []string) (map[string]*pkg.Package, error) {
	r.PostPassAdded = 0
	adapter := NewGophersatAdapter()
	allPackages := make(map[string]*pkg.Package)
	allCandidates := make(map[string][]*pkg.Package) // name -> all versions

	// Track root packages: name -> the specific version selected by loadPackageFromAtom.
	// This determines the unit clause for each root package.
	rootPackageNames := make([]string, 0, len(packages))
	rootPackagesMap := make(map[string]*pkg.Package)
	rootAtoms := make(map[string]string) // name → original atom string

	// Load and collect all dependencies
	for _, pkgName := range packages {
		p, err := r.loadPackageFromAtom(pkgName)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", pkgName, err)
		}

		// Store the actual package name and original atom for root constraints
		rootPackageNames = append(rootPackageNames, p.Name)
		rootPackagesMap[p.Name] = p
		rootAtoms[p.Name] = pkgName

		logging.Debug("Resolving package: %s-%s with %d dependencies",
			p.Name, p.Version, len(p.Deps))

		// Best-effort collection: SAT determines satisfiability, not this phase.
		// Load ALL candidate versions and explore each one's deps.
		r.addCandidateVersions(p.Name, allCandidates)
		if candidates, ok := allCandidates[p.Name]; ok {
			for _, candidate := range candidates {
				r.collectDependencies(candidate, allPackages, allCandidates)
			}
		}
	}

	logging.Debug("Total packages in dependency graph: %d", len(allPackages))
	logging.Debug("Total candidate packages: %d names", len(allCandidates))

	// Register ALL candidate versions with the SAT adapter.
	// This is the key change: instead of registering one version per package,
	// we register all unmasked versions so the SAT solver can choose.
	for _, candidates := range allCandidates {
		for _, c := range candidates {
			adapter.AddPackage(c)
		}
	}

	// Also register any packages from allPackages that might not have candidates
	// (e.g., packages from repos that don't support GetAllVersions well)
	for _, p := range allPackages {
		adapter.AddPackage(p)
	}

	// Add root requirements: unit clause for each root package's selected version
	r.addRootConstraints(adapter, rootPackagesMap, rootAtoms)

	// Add implication constraints for each candidate version.
	// For each version of each package, its deps become implications:
	// "if this version is selected, then its deps must be satisfied"
	for _, candidates := range allCandidates {
		for _, c := range candidates {
			r.addPackageConstraints(adapter, c, rootPackageNames)
		}
	}

	// Also process packages that are in allPackages but not in allCandidates
	// (single-version packages loaded directly)
	for _, p := range allPackages {
		if _, hasCandidates := allCandidates[p.Name]; !hasCandidates {
			r.addPackageConstraints(adapter, p, rootPackageNames)
		}
	}

	// Add at-most-one-per-slot pairwise exclusion clauses
	adapter.AddAtMostOnePerSlot()

	logging.Debug("Total clauses in SAT problem: %d", len(adapter.clauses))

	// Solve
	status, solution, err := adapter.Solve()
	if err != nil {
		return nil, err
	}

	if status != pkg.StatusSat {
		explanation := adapter.ExplainUNSAT()
		for _, line := range explanation {
			logging.Info("%s", line)
		}
		return nil, fmt.Errorf("no solution found (UNSAT, see explanation above)")
	}

	// Build result
	result, err := r.buildResultFromSolution(solution)
	if err != nil {
		return nil, err
	}

	// Post-SAT safety net: should add ZERO packages if SAT encoding is complete.
	// Any addition signals a gap in implication clauses. Logged as warning.
	for pass := 1; pass <= 10; pass++ {
		added := 0
		// Snapshot current result keys to avoid modifying map while iterating
		currentPkgs := make([]*pkg.Package, 0, len(result))
		for _, p := range result {
			currentPkgs = append(currentPkgs, p)
		}

		for _, p := range currentPkgs {
			// Group deps by OR-group
			requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

			// Required deps
			for _, dep := range requiredDeps {
				depPkg, err := r.loadUnmaskedPackage(dep.Name)
				if err != nil {
					continue
				}
				depKey := packageSlotKey(depPkg)
				if _, inResult := result[depKey]; inResult {
					continue
				}
				result[depKey] = depPkg
				added++
			}

			// OR-groups: pick first available alternative (Portage default behavior)
			for _, alternatives := range orGroups {
				// Check if any alternative is already in result
				satisfied := false
				for _, alt := range alternatives {
					altPkg, altErr := r.loadUnmaskedPackage(alt.Name)
					if altErr != nil {
						continue
					}
					if _, inResult := result[packageSlotKey(altPkg)]; inResult {
						satisfied = true
						break
					}
				}
				if satisfied {
					continue
				}
				// Pick first available alternative
				for _, alt := range alternatives {
					altPkg, err := r.loadUnmaskedPackage(alt.Name)
					if err != nil {
						continue
					}
					result[packageSlotKey(altPkg)] = altPkg
					added++
					break // Take first available
				}
			}
		}
		r.PostPassAdded += added
		if added > 0 {
			logging.Info("WARNING: Post-SAT pass %d added %d packages — SAT encoding incomplete", pass, added)
		}
		if added == 0 {
			break
		}
	}

	// Output formatted package list
	logging.Info("Resolved packages:")
	for key, p := range result {
		logging.Debug("- %s-%s [slot:%s key:%s]", p.Name, p.Version, p.Slot.Name, key)
	}

	return result, nil
}

// packageSlotKey returns a string key that distinguishes packages by slot.
// Packages in different slots can coexist (e.g., python:3.12 and python:3.13).
func packageSlotKey(p *pkg.Package) string {
	if p.Slot.Name != "" && p.Slot.Name != "0" {
		return p.Name + ":" + p.Slot.Name
	}
	return p.Name
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
