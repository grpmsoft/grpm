package solver

import (
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestNewDependencyGraph(t *testing.T) {
	g := NewDependencyGraph()

	if g == nil {
		t.Fatal("NewDependencyGraph returned nil")
	}

	if g.PackageCount() != 0 {
		t.Errorf("New graph should have 0 packages, got %d", g.PackageCount())
	}

	if g.EdgeCount() != 0 {
		t.Errorf("New graph should have 0 edges, got %d", g.EdgeCount())
	}
}

func TestDependencyGraph_AddPackage(t *testing.T) {
	g := NewDependencyGraph()

	// Add root package
	rootPkg := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	g.AddPackage(rootPkg, true)

	if g.PackageCount() != 1 {
		t.Errorf("Expected 1 package, got %d", g.PackageCount())
	}

	node, ok := g.GetNode("sys-libs/zlib")
	if !ok {
		t.Fatal("Package not found in graph")
	}

	if node.Package.Name != "sys-libs/zlib" {
		t.Errorf("Expected package name 'sys-libs/zlib', got '%s'", node.Package.Name)
	}

	if node.Depth != 0 {
		t.Errorf("Root package should have depth 0, got %d", node.Depth)
	}

	// Add dependency package
	depPkg := pkg.NewPackage("app-arch/gzip", "1.12", "0")
	g.AddPackage(depPkg, false)

	if g.PackageCount() != 2 {
		t.Errorf("Expected 2 packages, got %d", g.PackageCount())
	}

	depNode, ok := g.GetNode("app-arch/gzip")
	if !ok {
		t.Fatal("Dependency package not found in graph")
	}

	if depNode.Depth != -1 {
		t.Errorf("Non-root package should have depth -1 initially, got %d", depNode.Depth)
	}
}

func TestDependencyGraph_AddPackage_Duplicate(t *testing.T) {
	g := NewDependencyGraph()

	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	g.AddPackage(pkg1, true)

	// Try to add same package again
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.2.14", "0")
	g.AddPackage(pkg2, false)

	if g.PackageCount() != 1 {
		t.Errorf("Duplicate package should not be added, expected 1 package, got %d", g.PackageCount())
	}

	// Should keep the first version
	node, _ := g.GetNode("sys-libs/zlib")
	if node.Package.Version != "1.2.13" {
		t.Errorf("Expected first version '1.2.13', got '%s'", node.Package.Version)
	}
}

func TestDependencyGraph_AddDependency(t *testing.T) {
	g := NewDependencyGraph()

	// Add packages
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("app-arch/gzip", "1.12", "0")
	g.AddPackage(pkg1, true)
	g.AddPackage(pkg2, false)

	// Add dependency: zlib depends on gzip
	constraint := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Name:    "app-arch/gzip",
		Version: pkg.NewMinVersionConstraint("1.12"),
	}

	err := g.AddDependency("sys-libs/zlib", "app-arch/gzip", constraint, EdgeTypeRuntime)
	if err != nil {
		t.Fatalf("Failed to add dependency: %v", err)
	}

	if g.EdgeCount() != 1 {
		t.Errorf("Expected 1 edge, got %d", g.EdgeCount())
	}

	// Check outgoing edges (dependencies)
	zlibNode, _ := g.GetNode("sys-libs/zlib")
	if len(zlibNode.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency for zlib, got %d", len(zlibNode.Dependencies))
	}

	// Check incoming edges (dependents)
	gzipNode, _ := g.GetNode("app-arch/gzip")
	if len(gzipNode.Dependents) != 1 {
		t.Errorf("Expected 1 dependent for gzip, got %d", len(gzipNode.Dependents))
	}

	// Verify edge details
	edge := zlibNode.Dependencies[0]
	if edge.From != "sys-libs/zlib" {
		t.Errorf("Expected edge from 'sys-libs/zlib', got '%s'", edge.From)
	}
	if edge.To != "app-arch/gzip" {
		t.Errorf("Expected edge to 'app-arch/gzip', got '%s'", edge.To)
	}
	if edge.Type != EdgeTypeRuntime {
		t.Errorf("Expected EdgeTypeRuntime, got %v", edge.Type)
	}
}

func TestDependencyGraph_AddDependency_MissingPackage(t *testing.T) {
	g := NewDependencyGraph()

	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	g.AddPackage(pkg1, true)

	constraint := pkg.Constraint{
		Type: pkg.ConstraintTypeVersion,
		Name: "app-arch/gzip",
	}

	// Try to add dependency to non-existent package
	err := g.AddDependency("sys-libs/zlib", "app-arch/gzip", constraint, EdgeTypeRuntime)
	if err == nil {
		t.Error("Expected error when adding dependency to non-existent package")
	}
}

func TestDependencyGraph_CalculateDepths(t *testing.T) {
	g := NewDependencyGraph()

	// Build graph:
	// root (depth 0)
	//   -> dep1 (depth 1)
	//      -> dep2 (depth 2)
	//   -> dep2 (depth 1, shorter path)

	root := pkg.NewPackage("root", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	dep2 := pkg.NewPackage("dep2", "1.0", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep1, false)
	g.AddPackage(dep2, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	g.AddDependency("root", "dep1", constraint, EdgeTypeRuntime)
	g.AddDependency("dep1", "dep2", constraint, EdgeTypeRuntime)
	g.AddDependency("root", "dep2", constraint, EdgeTypeRuntime)

	// Calculate depths
	g.CalculateDepths()

	// Check depths
	rootNode, _ := g.GetNode("root")
	dep1Node, _ := g.GetNode("dep1")
	dep2Node, _ := g.GetNode("dep2")

	if rootNode.Depth != 0 {
		t.Errorf("Root depth should be 0, got %d", rootNode.Depth)
	}

	if dep1Node.Depth != 1 {
		t.Errorf("dep1 depth should be 1, got %d", dep1Node.Depth)
	}

	if dep2Node.Depth != 1 {
		t.Errorf("dep2 depth should be 1 (shorter path), got %d", dep2Node.Depth)
	}
}

func TestDependencyGraph_GetPackagesByDepth(t *testing.T) {
	g := NewDependencyGraph()

	root := pkg.NewPackage("root", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	dep2 := pkg.NewPackage("dep2", "1.0", "0")
	dep3 := pkg.NewPackage("dep3", "1.0", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep1, false)
	g.AddPackage(dep2, false)
	g.AddPackage(dep3, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	g.AddDependency("root", "dep1", constraint, EdgeTypeRuntime)
	g.AddDependency("root", "dep2", constraint, EdgeTypeRuntime)
	g.AddDependency("dep1", "dep3", constraint, EdgeTypeRuntime)

	g.CalculateDepths()

	byDepth := g.GetPackagesByDepth()

	// Check depth 0 (root)
	if len(byDepth[0]) != 1 {
		t.Errorf("Expected 1 package at depth 0, got %d", len(byDepth[0]))
	}

	// Check depth 1 (dep1, dep2)
	if len(byDepth[1]) != 2 {
		t.Errorf("Expected 2 packages at depth 1, got %d", len(byDepth[1]))
	}

	// Check depth 2 (dep3)
	if len(byDepth[2]) != 1 {
		t.Errorf("Expected 1 package at depth 2, got %d", len(byDepth[2]))
	}
}

func TestDependencyGraph_String(t *testing.T) {
	g := NewDependencyGraph()

	root := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	dep := pkg.NewPackage("app-arch/gzip", "1.12", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
	g.AddDependency("sys-libs/zlib", "app-arch/gzip", constraint, EdgeTypeRuntime)

	g.CalculateDepths()

	str := g.String()

	// Check that output contains key information
	if !strings.Contains(str, "2 packages") {
		t.Error("String output should contain package count")
	}

	if !strings.Contains(str, "1 edges") {
		t.Error("String output should contain edge count")
	}

	if !strings.Contains(str, "sys-libs/zlib") {
		t.Error("String output should contain root package name")
	}
}

func TestDependencyGraph_ToDOT(t *testing.T) {
	g := NewDependencyGraph()

	root := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	dep := pkg.NewPackage("app-arch/gzip", "1.12", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep, false)

	constraint := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewMinVersionConstraint("1.12"),
	}
	g.AddDependency("sys-libs/zlib", "app-arch/gzip", constraint, EdgeTypeRuntime)

	dot := g.ToDOT()

	// Check DOT format
	if !strings.HasPrefix(dot, "digraph DependencyGraph") {
		t.Error("DOT output should start with 'digraph DependencyGraph'")
	}

	if !strings.Contains(dot, "sys-libs/zlib") {
		t.Error("DOT output should contain package names")
	}

	if !strings.Contains(dot, "->") {
		t.Error("DOT output should contain edge arrow")
	}

	if !strings.Contains(dot, "lightblue") {
		t.Error("DOT output should mark root packages with lightblue color")
	}
}

func TestDependencyGraph_ResetVisited(t *testing.T) {
	g := NewDependencyGraph()

	pkg1 := pkg.NewPackage("pkg1", "1.0", "0")
	pkg2 := pkg.NewPackage("pkg2", "1.0", "0")

	g.AddPackage(pkg1, true)
	g.AddPackage(pkg2, false)

	// Set visited flags
	node1, _ := g.GetNode("pkg1")
	node2, _ := g.GetNode("pkg2")
	node1.Visited = true
	node1.InStack = true
	node1.State = NodeStateVisited
	node2.Visited = true

	// Reset
	g.ResetVisited()

	// Check flags are reset
	if node1.Visited || node1.InStack || node1.State != NodeStateUnvisited {
		t.Error("ResetVisited should reset all traversal flags for node1")
	}

	if node2.Visited || node2.State != NodeStateUnvisited {
		t.Error("ResetVisited should reset all traversal flags for node2")
	}
}

func BenchmarkDependencyGraph_AddPackage(b *testing.B) {
	g := NewDependencyGraph()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pkgName := "pkg-" + string(rune(i))
		p := pkg.NewPackage(pkgName, "1.0", "0")
		g.AddPackage(p, false)
	}
}

func BenchmarkDependencyGraph_AddDependency(b *testing.B) {
	g := NewDependencyGraph()

	// Pre-create packages
	for i := 0; i < 1000; i++ {
		pkgName := "pkg-" + string(rune(i))
		p := pkg.NewPackage(pkgName, "1.0", "0")
		g.AddPackage(p, false)
	}

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	b.ResetTimer()
	for i := 0; i < b.N && i < 999; i++ {
		from := "pkg-" + string(rune(i))
		to := "pkg-" + string(rune(i+1))
		g.AddDependency(from, to, constraint, EdgeTypeRuntime)
	}
}

func BenchmarkDependencyGraph_CalculateDepths(b *testing.B) {
	g := NewDependencyGraph()

	// Build a chain graph
	root := pkg.NewPackage("root", "1.0", "0")
	g.AddPackage(root, true)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	prev := "root"
	for i := 0; i < 100; i++ {
		pkgName := "pkg-" + string(rune(i))
		p := pkg.NewPackage(pkgName, "1.0", "0")
		g.AddPackage(p, false)
		g.AddDependency(prev, pkgName, constraint, EdgeTypeRuntime)
		prev = pkgName
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.CalculateDepths()
	}
}
