package solver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/mask"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// PortageResolver resolves package dependencies using SAT solving.
// It supports package masking and keyword filtering.
type PortageResolver struct {
	repo           repo.Repository
	maskManager    *mask.MaskManager
	acceptKeywords []string // ACCEPT_KEYWORDS from make.conf (e.g., ["amd64", "~amd64"])
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

	// Check if this package is masked
	if r.isMasked(p) {
		logging.Debug("Skipping masked package: %s-%s", p.Name, p.Version)
		return nil
	}

	// Store a copy of the package
	copyPkg := *p
	allPackages[p.Name] = &copyPkg

	// Group dependencies by OrGroupID
	requiredDeps, orGroups := groupDependenciesByOrGroupID(p.Deps)

	// Process REQUIRED dependencies only
	for _, dep := range requiredDeps {
		// Use loadUnmaskedPackage to get the best unmasked version
		depPkg, err := r.loadUnmaskedPackage(dep.Name)
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
			// Use loadUnmaskedPackage to filter masked alternatives
			if _, err := r.loadUnmaskedPackage(alt.Name); err != nil {
				logging.Debug("Warning: OR-alternative %s not found or masked: %v", alt.Name, err)
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

// buildResultFromSolution builds the final result map from the SAT solution.
// The solution map contains package names as keys and selected versions as values.
func (r *PortageResolver) buildResultFromSolution(solution map[string]string) (map[string]*pkg.Package, error) {
	result := make(map[string]*pkg.Package)
	for name, version := range solution {
		// Load the specific version that was selected by the SAT solver
		p, err := r.repo.LoadPackageVersion(name, version)
		if err != nil {
			// Fallback to LoadPackage if LoadPackageVersion fails
			// This handles cases where version might be empty
			p, err = r.repo.LoadPackage(name)
			if err != nil {
				logging.Debug("Warning: package %s not found: %v", name, err)
				continue
			}
		}
		result[name] = p
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

	// If atom has a version constraint, use FindByAtom to get matching packages
	if atom.HasVersion() {
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

func (r *PortageResolver) Resolve(packages []string) (map[string]*pkg.Package, error) {
	adapter := NewGophersatAdapter()
	allPackages := make(map[string]*pkg.Package)

	// Track root package names (not atom strings) for constraint generation
	rootPackageNames := make([]string, 0, len(packages))

	// Load and collect all dependencies
	for _, pkgName := range packages {
		p, err := r.loadPackageFromAtom(pkgName)
		if err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", pkgName, err)
		}

		// Store the actual package name (category/package) for constraints
		rootPackageNames = append(rootPackageNames, p.Name)

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

	// Then add constraints for each package using actual package names
	for _, p := range allPackages {
		r.addPackageConstraints(adapter, p, rootPackageNames)
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
