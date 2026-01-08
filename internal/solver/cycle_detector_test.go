package solver

import (
	"errors"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestDependencyGraph_DetectCycles_NoCycle(t *testing.T) {
	g := NewDependencyGraph()

	// Build linear dependency chain: A -> B -> C
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)
	g.AddPackage(pkgC, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-c", constraint, EdgeTypeRuntime)

	// Should not detect any cycles
	err := g.DetectCycles()
	if err != nil {
		t.Errorf("DetectCycles() returned error for acyclic graph: %v", err)
	}

	if g.HasCycles() {
		t.Error("HasCycles() returned true for acyclic graph")
	}
}

func TestDependencyGraph_DetectCycles_SimpleCycle(t *testing.T) {
	g := NewDependencyGraph()

	// Build cycle: A -> B -> A
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-a", constraint, EdgeTypeRuntime)

	// Should detect cycle
	err := g.DetectCycles()
	if err == nil {
		t.Fatal("DetectCycles() should have detected a cycle")
	}

	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("Expected *CycleError, got %T", err)
	}

	// Check cycle contains both packages
	if len(cycleErr.Cycle) != 2 {
		t.Errorf("Expected cycle length 2, got %d", len(cycleErr.Cycle))
	}

	// Check error message format
	errMsg := cycleErr.Error()
	if !strings.Contains(errMsg, "circular dependency detected") {
		t.Errorf("Error message missing 'circular dependency detected': %s", errMsg)
	}

	if !strings.Contains(errMsg, "pkg-a") || !strings.Contains(errMsg, "pkg-b") {
		t.Errorf("Error message missing package names: %s", errMsg)
	}

	if !g.HasCycles() {
		t.Error("HasCycles() should return true")
	}
}

func TestDependencyGraph_DetectCycles_SelfCycle(t *testing.T) {
	g := NewDependencyGraph()

	// Build self-cycle: A -> A
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")

	g.AddPackage(pkgA, true)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
	_ = g.AddDependency("pkg-a", "pkg-a", constraint, EdgeTypeRuntime)

	// Should detect self-cycle
	err := g.DetectCycles()
	if err == nil {
		t.Fatal("DetectCycles() should have detected a self-cycle")
	}

	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("Expected *CycleError, got %T", err)
	}

	if len(cycleErr.Cycle) != 1 {
		t.Errorf("Expected self-cycle length 1, got %d: %v", len(cycleErr.Cycle), cycleErr.Cycle)
	}

	if cycleErr.Cycle[0] != "pkg-a" {
		t.Errorf("Expected cycle to contain 'pkg-a', got %v", cycleErr.Cycle)
	}
}

func TestDependencyGraph_DetectCycles_ComplexCycle(t *testing.T) {
	g := NewDependencyGraph()

	// Build complex cycle: A -> B -> C -> D -> B (cycle starts at B)
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.0", "0")
	pkgD := pkg.NewPackage("pkg-d", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)
	g.AddPackage(pkgC, false)
	g.AddPackage(pkgD, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-c", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-c", "pkg-d", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-d", "pkg-b", constraint, EdgeTypeRuntime) // Back edge

	// Should detect cycle
	err := g.DetectCycles()
	if err == nil {
		t.Fatal("DetectCycles() should have detected a cycle")
	}

	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("Expected *CycleError, got %T", err)
	}

	// Cycle should be B -> C -> D -> B (length 3)
	if len(cycleErr.Cycle) != 3 {
		t.Errorf("Expected cycle length 3, got %d: %v", len(cycleErr.Cycle), cycleErr.Cycle)
	}

	// Check that cycle contains the right packages
	cycleSet := make(map[string]bool)
	for _, pkg := range cycleErr.Cycle {
		cycleSet[pkg] = true
	}

	expectedInCycle := []string{"pkg-b", "pkg-c", "pkg-d"}
	for _, pkg := range expectedInCycle {
		if !cycleSet[pkg] {
			t.Errorf("Expected package '%s' in cycle, got %v", pkg, cycleErr.Cycle)
		}
	}

	// pkg-a should NOT be in the cycle
	if cycleSet["pkg-a"] {
		t.Errorf("Package 'pkg-a' should not be in cycle, got %v", cycleErr.Cycle)
	}
}

func TestDependencyGraph_DetectCycles_DiamondNoCycle(t *testing.T) {
	g := NewDependencyGraph()

	// Build diamond graph (no cycle):
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D

	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.0", "0")
	pkgD := pkg.NewPackage("pkg-d", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)
	g.AddPackage(pkgC, false)
	g.AddPackage(pkgD, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-a", "pkg-c", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-d", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-c", "pkg-d", constraint, EdgeTypeRuntime)

	// Should NOT detect cycle (diamond is acyclic)
	err := g.DetectCycles()
	if err != nil {
		t.Errorf("DetectCycles() returned error for diamond graph: %v", err)
	}
}

func TestDependencyGraph_DetectCycles_MultipleCycles(t *testing.T) {
	g := NewDependencyGraph()

	// Build graph with two separate cycles:
	// Cycle 1: A -> B -> A
	// Cycle 2: C -> D -> C

	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.0", "0")
	pkgD := pkg.NewPackage("pkg-d", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)
	g.AddPackage(pkgC, true)
	g.AddPackage(pkgD, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-a", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-c", "pkg-d", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-d", "pkg-c", constraint, EdgeTypeRuntime)

	// Should detect at least one cycle
	err := g.DetectCycles()
	if err == nil {
		t.Fatal("DetectCycles() should have detected a cycle")
	}

	// GetAllCycles() should find both cycles
	cycles := g.GetAllCycles()
	if len(cycles) < 1 {
		t.Errorf("Expected at least 1 cycle, got %d", len(cycles))
	}
}

func TestDependencyGraph_GetAllCycles(t *testing.T) {
	g := NewDependencyGraph()

	// Build graph with known cycle: A -> B -> C -> A
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.0", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)
	g.AddPackage(pkgC, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("pkg-a", "pkg-b", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-b", "pkg-c", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("pkg-c", "pkg-a", constraint, EdgeTypeRuntime)

	// Get all cycles
	cycles := g.GetAllCycles()
	if len(cycles) == 0 {
		t.Fatal("GetAllCycles() should have found at least one cycle")
	}

	// Check first cycle contains all three packages
	cycle := cycles[0]
	if len(cycle.Cycle) != 3 {
		t.Errorf("Expected cycle length 3, got %d", len(cycle.Cycle))
	}

	// Verify all packages are in the cycle
	cycleSet := make(map[string]bool)
	for _, pkg := range cycle.Cycle {
		cycleSet[pkg] = true
	}

	if !cycleSet["pkg-a"] || !cycleSet["pkg-b"] || !cycleSet["pkg-c"] {
		t.Errorf("Cycle should contain all three packages, got %v", cycle.Cycle)
	}
}

func TestCycleError_Error(t *testing.T) {
	tests := []struct {
		name     string
		cycle    []string
		expected []string // Substrings that should appear in error message
	}{
		{
			name:     "simple cycle",
			cycle:    []string{"pkg-a", "pkg-b"},
			expected: []string{"circular dependency detected", "pkg-a", "pkg-b", "->"},
		},
		{
			name:     "complex cycle",
			cycle:    []string{"sys-libs/zlib", "app-arch/gzip", "dev-libs/openssl"},
			expected: []string{"circular dependency", "sys-libs/zlib", "app-arch/gzip", "dev-libs/openssl"},
		},
		{
			name:     "self cycle",
			cycle:    []string{"pkg-a"},
			expected: []string{"circular dependency", "pkg-a"},
		},
		{
			name:     "empty cycle",
			cycle:    []string{},
			expected: []string{"circular dependency"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CycleError{Cycle: tt.cycle}
			errMsg := err.Error()

			for _, expected := range tt.expected {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Error message should contain '%s', got: %s", expected, errMsg)
				}
			}
		})
	}
}

func TestExtractCycle(t *testing.T) {
	tests := []struct {
		name        string
		path        []string
		cycleStart  string
		expected    []string
		description string
	}{
		{
			name:        "cycle at end",
			path:        []string{"A", "B", "C"},
			cycleStart:  "C",
			expected:    []string{"C"},
			description: "Cycle starts at last element",
		},
		{
			name:        "cycle in middle",
			path:        []string{"A", "B", "C", "D"},
			cycleStart:  "B",
			expected:    []string{"B", "C", "D"},
			description: "Cycle starts in middle of path",
		},
		{
			name:        "cycle at start",
			path:        []string{"A", "B", "C"},
			cycleStart:  "A",
			expected:    []string{"A", "B", "C"},
			description: "Cycle starts at beginning",
		},
		{
			name:        "cycle not in path",
			path:        []string{"A", "B"},
			cycleStart:  "C",
			expected:    []string{"C"},
			description: "Cycle start not found in path (edge case)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCycle(tt.path, tt.cycleStart)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected cycle length %d, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("At index %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestNormalizeCycle(t *testing.T) {
	tests := []struct {
		name     string
		cycle    []string
		expected []string
	}{
		{
			name:     "already normalized",
			cycle:    []string{"A", "B", "C"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "needs rotation",
			cycle:    []string{"C", "A", "B"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "rotation from middle",
			cycle:    []string{"B", "C", "A"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "single element",
			cycle:    []string{"A"},
			expected: []string{"A"},
		},
		{
			name:     "empty cycle",
			cycle:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeCycle(tt.cycle)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("At index %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func BenchmarkDetectCycles_NoCycle(b *testing.B) {
	g := NewDependencyGraph()

	// Build large linear chain
	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	root := pkg.NewPackage("root", "1.0", "0")
	g.AddPackage(root, true)

	prev := "root"
	for i := 0; i < 100; i++ {
		pkgName := "pkg-" + string(rune(i))
		p := pkg.NewPackage(pkgName, "1.0", "0")
		g.AddPackage(p, false)
		_ = g.AddDependency(prev, pkgName, constraint, EdgeTypeRuntime)
		prev = pkgName
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.DetectCycles()
	}
}

func BenchmarkDetectCycles_WithCycle(b *testing.B) {
	g := NewDependencyGraph()

	// Build chain with cycle at end
	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	root := pkg.NewPackage("root", "1.0", "0")
	g.AddPackage(root, true)

	prev := "root"
	for i := 0; i < 50; i++ {
		pkgName := "pkg-" + string(rune(i))
		p := pkg.NewPackage(pkgName, "1.0", "0")
		g.AddPackage(p, false)
		_ = g.AddDependency(prev, pkgName, constraint, EdgeTypeRuntime)
		prev = pkgName
	}

	// Add cycle: last -> first
	_ = g.AddDependency(prev, "root", constraint, EdgeTypeRuntime)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.DetectCycles()
	}
}
