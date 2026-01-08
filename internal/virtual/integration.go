package virtual

import (
	"github.com/grpmsoft/grpm/internal/pkg"
)

// DependencyRewriter rewrites virtual package dependencies to concrete providers.
//
// This is used during dependency resolution to replace virtual package
// dependencies with their selected providers.
type DependencyRewriter struct {
	resolver *Resolver
}

// NewDependencyRewriter creates a new dependency rewriter.
func NewDependencyRewriter(resolver *Resolver) *DependencyRewriter {
	return &DependencyRewriter{
		resolver: resolver,
	}
}

// RewriteConstraints rewrites virtual dependencies in a list of constraints.
//
// For each constraint referencing a virtual package, the virtual is
// resolved to a concrete provider using the resolver. The constraint
// is then updated to reference the provider instead.
//
// constraints: the original constraints to rewrite
// providerLookup: function to find available providers for a virtual
//
// Returns the rewritten constraints and a map of virtual -> selected provider.
func (r *DependencyRewriter) RewriteConstraints(
	constraints []pkg.Constraint,
	providerLookup func(virtual string) []string,
) ([]pkg.Constraint, map[string]string, error) {
	rewritten := make([]pkg.Constraint, 0, len(constraints))
	virtualMappings := make(map[string]string)

	for _, c := range constraints {
		// Check if this constraint references a virtual package
		if IsVirtual(c.Name) {
			// Get available providers
			providers := providerLookup(c.Name)

			if len(providers) == 0 {
				// No providers - keep original constraint
				// (will fail during resolution with clear error)
				rewritten = append(rewritten, c)
				continue
			}

			// Select best provider
			selected, err := r.resolver.SelectProvider(c.Name, providers)
			if err != nil {
				// Keep original on error
				rewritten = append(rewritten, c)
				continue
			}

			// Record mapping
			virtualMappings[c.Name] = selected

			// Create new constraint pointing to provider
			newConstraint := pkg.Constraint{
				Type:      c.Type,
				Name:      StripSlot(selected), // Use provider name
				Version:   c.Version,
				Slot:      ExtractSlot(selected), // Use provider slot if any
				Flag:      c.Flag,
				Required:  c.Required,
				Condition: c.Condition,
				OrGroupID: c.OrGroupID,
			}
			rewritten = append(rewritten, newConstraint)
		} else {
			// Not a virtual - keep as-is
			rewritten = append(rewritten, c)
		}
	}

	return rewritten, virtualMappings, nil
}

// FindVirtualDeps finds all virtual package dependencies in a constraint list.
func FindVirtualDeps(constraints []pkg.Constraint) []string {
	virtuals := make([]string, 0)
	seen := make(map[string]bool)

	for _, c := range constraints {
		if IsVirtual(c.Name) && !seen[c.Name] {
			virtuals = append(virtuals, c.Name)
			seen[c.Name] = true
		}
	}

	return virtuals
}

// PackageWithProviders represents a package that may provide virtuals.
type PackageWithProviders struct {
	Package  *pkg.Package
	Provides []string // Virtual packages this package provides
}

// BuildProviderIndex creates an index of which packages provide which virtuals.
//
// This scans packages and builds a map:
//
//	virtual/jdk -> [dev-java/openjdk:17, dev-java/oracle-jdk-bin:17]
func BuildProviderIndex(packages []*pkg.Package) map[string][]string {
	index := make(map[string][]string)

	for _, p := range packages {
		// Check if package is in virtual/ category
		if IsVirtual(p.Name) {
			// This IS a virtual package - parse its RDEPEND for providers
			// Note: The Package.Deps field contains OR-group alternatives
			// which are the providers
			for _, dep := range p.Deps {
				if dep.OrGroupID > 0 {
					// This is an OR-group alternative = a provider
					if !IsVirtual(dep.Name) {
						index[p.Name] = append(index[p.Name], formatProviderAtom(dep))
					}
				}
			}
		}

		// Check Provides field (explicit provider declarations)
		for _, provide := range p.Provides {
			if IsVirtual(provide.Name) {
				providerAtom := p.Name
				if p.Slot.Name != "" {
					providerAtom += ":" + p.Slot.String()
				}
				index[provide.Name] = append(index[provide.Name], providerAtom)
			}
		}
	}

	return index
}

// formatProviderAtom formats a constraint as a provider atom.
func formatProviderAtom(c pkg.Constraint) string {
	atom := c.Name
	if c.Slot != "" {
		atom += ":" + c.Slot
	}
	return atom
}

// VirtualResolutionResult contains the result of virtual resolution.
type VirtualResolutionResult struct {
	// SelectedProviders maps virtual names to selected providers
	SelectedProviders map[string]string

	// RewrittenDeps contains dependencies with virtuals replaced by providers
	RewrittenDeps []pkg.Constraint

	// UnresolvedVirtuals lists virtuals that could not be resolved
	UnresolvedVirtuals []string
}

// ResolveVirtualsInPackage resolves all virtual dependencies for a package.
//
// This is a high-level function that:
// 1. Finds all virtual dependencies in the package
// 2. Looks up available providers for each virtual
// 3. Selects the best provider based on configuration
// 4. Returns rewritten dependencies with virtuals replaced
func ResolveVirtualsInPackage(
	p *pkg.Package,
	resolver *Resolver,
	providerIndex map[string][]string,
) (*VirtualResolutionResult, error) {
	result := &VirtualResolutionResult{
		SelectedProviders:  make(map[string]string),
		UnresolvedVirtuals: make([]string, 0),
	}

	rewriter := NewDependencyRewriter(resolver)

	// Create provider lookup function
	providerLookup := func(virtual string) []string {
		if providers, ok := providerIndex[virtual]; ok {
			return providers
		}
		return nil
	}

	// Rewrite dependencies
	rewritten, mappings, err := rewriter.RewriteConstraints(p.Deps, providerLookup)
	if err != nil {
		return nil, err
	}

	result.RewrittenDeps = rewritten
	result.SelectedProviders = mappings

	// Find unresolved virtuals
	for _, c := range rewritten {
		if IsVirtual(c.Name) {
			result.UnresolvedVirtuals = append(result.UnresolvedVirtuals, c.Name)
		}
	}

	return result, nil
}
