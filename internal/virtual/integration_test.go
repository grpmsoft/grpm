package virtual

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestDependencyRewriterRewriteConstraints tests constraint rewriting.
func TestDependencyRewriterRewriteConstraints(t *testing.T) {
	resolver := NewResolver()
	resolver.SetDefault("virtual/jdk", "dev-java/openjdk")

	rewriter := NewDependencyRewriter(resolver)

	constraints := []pkg.Constraint{
		{Name: "virtual/jdk", Type: pkg.ConstraintTypeVersion},
		{Name: "sys-libs/zlib", Type: pkg.ConstraintTypeVersion},
	}

	providerLookup := func(virtual string) []string {
		if virtual == "virtual/jdk" {
			return []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"}
		}
		return nil
	}

	rewritten, mappings, err := rewriter.RewriteConstraints(constraints, providerLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check rewritten constraints
	if len(rewritten) != 2 {
		t.Errorf("expected 2 constraints, got %d", len(rewritten))
	}

	// First constraint should be rewritten to provider
	if rewritten[0].Name != "dev-java/openjdk" {
		t.Errorf("expected first constraint to be dev-java/openjdk, got %s", rewritten[0].Name)
	}

	// Second constraint should be unchanged
	if rewritten[1].Name != "sys-libs/zlib" {
		t.Errorf("expected second constraint to be sys-libs/zlib, got %s", rewritten[1].Name)
	}

	// Check mappings
	if mappings["virtual/jdk"] != "dev-java/openjdk:17" {
		t.Errorf("expected virtual/jdk mapping to be dev-java/openjdk:17, got %s", mappings["virtual/jdk"])
	}
}

// TestDependencyRewriterNoProviders tests handling of virtuals without providers.
func TestDependencyRewriterNoProviders(t *testing.T) {
	resolver := NewResolver()
	rewriter := NewDependencyRewriter(resolver)

	constraints := []pkg.Constraint{
		{Name: "virtual/unknown", Type: pkg.ConstraintTypeVersion},
	}

	providerLookup := func(virtual string) []string {
		return nil // No providers available
	}

	rewritten, _, err := rewriter.RewriteConstraints(constraints, providerLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should keep original constraint when no providers
	if len(rewritten) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(rewritten))
	}
	if rewritten[0].Name != "virtual/unknown" {
		t.Errorf("expected constraint to remain virtual/unknown, got %s", rewritten[0].Name)
	}
}

// TestFindVirtualDeps tests finding virtual dependencies.
func TestFindVirtualDeps(t *testing.T) {
	constraints := []pkg.Constraint{
		{Name: "virtual/jdk", Type: pkg.ConstraintTypeVersion},
		{Name: "sys-libs/zlib", Type: pkg.ConstraintTypeVersion},
		{Name: "virtual/editor", Type: pkg.ConstraintTypeVersion},
		{Name: "virtual/jdk", Type: pkg.ConstraintTypeVersion}, // Duplicate
	}

	virtuals := FindVirtualDeps(constraints)

	if len(virtuals) != 2 {
		t.Errorf("expected 2 unique virtuals, got %d: %v", len(virtuals), virtuals)
	}

	// Check that both virtuals are found
	found := make(map[string]bool)
	for _, v := range virtuals {
		found[v] = true
	}

	if !found["virtual/jdk"] {
		t.Error("expected to find virtual/jdk")
	}
	if !found["virtual/editor"] {
		t.Error("expected to find virtual/editor")
	}
}

// TestBuildProviderIndex tests building the provider index.
func TestBuildProviderIndex(t *testing.T) {
	// Create a virtual package with OR-group providers
	virtualJdk := pkg.NewPackage("virtual/jdk", "17", "0")
	virtualJdk.AddDependency(pkg.Constraint{
		Name:      "dev-java/openjdk",
		Slot:      "17",
		OrGroupID: 1,
	})
	virtualJdk.AddDependency(pkg.Constraint{
		Name:      "dev-java/oracle-jdk-bin",
		Slot:      "17",
		OrGroupID: 1,
	})

	// Create a concrete package that provides a virtual
	openjdk := pkg.NewPackage("dev-java/openjdk", "17.0.2", "17")
	openjdk.Provides = []pkg.Constraint{
		{Name: "virtual/jdk"},
	}

	packages := []*pkg.Package{virtualJdk, openjdk}
	index := BuildProviderIndex(packages)

	// Check virtual/jdk providers
	providers, ok := index["virtual/jdk"]
	if !ok {
		t.Fatal("expected virtual/jdk in index")
	}

	// Should have providers from both virtual RDEPEND and package Provides
	if len(providers) < 2 {
		t.Errorf("expected at least 2 providers, got %d: %v", len(providers), providers)
	}
}

// TestResolveVirtualsInPackage tests the high-level resolution function.
func TestResolveVirtualsInPackage(t *testing.T) {
	// Create a package with virtual dependencies
	myApp := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	myApp.AddDependency(pkg.Constraint{
		Name: "virtual/jdk",
		Type: pkg.ConstraintTypeVersion,
	})
	myApp.AddDependency(pkg.Constraint{
		Name:    "sys-libs/zlib",
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewMinVersionConstraint("1.2.13"),
	})

	// Create resolver with default
	resolver := NewResolver()
	resolver.SetDefault("virtual/jdk", "dev-java/openjdk")

	// Create provider index
	providerIndex := map[string][]string{
		"virtual/jdk": {"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
	}

	result, err := ResolveVirtualsInPackage(myApp, resolver, providerIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check selected providers
	if result.SelectedProviders["virtual/jdk"] != "dev-java/openjdk:17" {
		t.Errorf("expected virtual/jdk -> dev-java/openjdk:17, got %s",
			result.SelectedProviders["virtual/jdk"])
	}

	// Check no unresolved virtuals
	if len(result.UnresolvedVirtuals) != 0 {
		t.Errorf("expected no unresolved virtuals, got %v", result.UnresolvedVirtuals)
	}

	// Check rewritten deps
	if len(result.RewrittenDeps) != 2 {
		t.Errorf("expected 2 rewritten deps, got %d", len(result.RewrittenDeps))
	}
}

// TestResolveVirtualsInPackageUnresolved tests handling unresolved virtuals.
func TestResolveVirtualsInPackageUnresolved(t *testing.T) {
	// Create a package with unknown virtual dependency
	myApp := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	myApp.AddDependency(pkg.Constraint{
		Name: "virtual/unknown",
		Type: pkg.ConstraintTypeVersion,
	})

	resolver := NewResolver()
	providerIndex := map[string][]string{} // Empty - no providers

	result, err := ResolveVirtualsInPackage(myApp, resolver, providerIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that unresolved virtual is reported
	if len(result.UnresolvedVirtuals) != 1 {
		t.Errorf("expected 1 unresolved virtual, got %d", len(result.UnresolvedVirtuals))
	}
	if len(result.UnresolvedVirtuals) > 0 && result.UnresolvedVirtuals[0] != "virtual/unknown" {
		t.Errorf("expected virtual/unknown as unresolved, got %s", result.UnresolvedVirtuals[0])
	}
}

// TestSlotPreservation tests that slot information is preserved during rewrite.
func TestSlotPreservation(t *testing.T) {
	resolver := NewResolver()
	rewriter := NewDependencyRewriter(resolver)

	constraints := []pkg.Constraint{
		{Name: "virtual/jdk", Type: pkg.ConstraintTypeVersion},
	}

	providerLookup := func(virtual string) []string {
		return []string{"dev-java/openjdk:17/17"} // Provider with slot and subslot
	}

	rewritten, _, err := rewriter.RewriteConstraints(constraints, providerLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rewritten) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(rewritten))
	}

	// Check slot was extracted
	if rewritten[0].Slot != "17/17" {
		t.Errorf("expected slot to be 17/17, got %s", rewritten[0].Slot)
	}
}
