package solver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestSlotCollisionType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       SlotCollisionType
		expected string
	}{
		{
			name:     "version conflict",
			ct:       CollisionTypeVersion,
			expected: "version conflict",
		},
		{
			name:     "unspecific conflict",
			ct:       CollisionTypeUnspecific,
			expected: "unspecific conflict",
		},
		{
			name:     "USE flag conflict",
			ct:       CollisionTypeSpecific,
			expected: "USE flag conflict",
		},
		{
			name:     "subslot conflict",
			ct:       CollisionTypeSubslot,
			expected: "subslot conflict",
		},
		{
			name:     "unknown conflict type",
			ct:       SlotCollisionType(99),
			expected: "unknown conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ct.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestUseChangeSuggestion_String(t *testing.T) {
	tests := []struct {
		name       string
		suggestion *UseChangeSuggestion
		contains   []string
	}{
		{
			name: "enable single flag",
			suggestion: &UseChangeSuggestion{
				Package:     pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"),
				FlagChanges: map[string]bool{"ssl": true},
			},
			contains: []string{"sys-libs/zlib", "+ssl"},
		},
		{
			name: "disable single flag",
			suggestion: &UseChangeSuggestion{
				Package:     pkg.NewPackage("dev-libs/openssl", "1.1.1", "0"),
				FlagChanges: map[string]bool{"static-libs": false},
			},
			contains: []string{"dev-libs/openssl", "-static-libs"},
		},
		{
			name: "multiple flags",
			suggestion: &UseChangeSuggestion{
				Package: pkg.NewPackage("app-misc/hello", "2.10", "0"),
				FlagChanges: map[string]bool{
					"nls":  true,
					"test": false,
				},
			},
			contains: []string{"app-misc/hello", "+nls", "-test"},
		},
		{
			name: "no changes",
			suggestion: &UseChangeSuggestion{
				Package:     pkg.NewPackage("sys-apps/portage", "3.0", "0"),
				FlagChanges: map[string]bool{},
			},
			contains: []string{"sys-apps/portage", "no changes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.suggestion.String()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestSlotCollisionDetector_NoCollisions(t *testing.T) {
	graph := NewDependencyGraph()

	// Add packages with different slots - no collision
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("app-misc/hello", "2.10", "0")

	graph.AddPackage(pkg1, true)
	graph.AddPackage(pkg2, false)

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) != 0 {
		t.Errorf("Expected no collisions, got %d", len(collisions))
	}
}

func TestSlotCollisionDetector_SameSlotDifferentVersions(t *testing.T) {
	graph := NewDependencyGraph()

	// Add two versions of the same package in the same slot
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	graph.AddPackage(pkg1, true)

	// Manually add second version to simulate collision
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.2.11", "0")
	node2 := &GraphNode{
		Package:      pkg2,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	graph.nodes["sys-libs/zlib-1.2.11"] = node2

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		t.Fatal("Expected at least one collision")
	}

	collision := collisions[0]
	if collision.SlotAtom != "sys-libs/zlib:0" {
		t.Errorf("Expected slot atom 'sys-libs/zlib:0', got %q", collision.SlotAtom)
	}

	if len(collision.Packages) != 2 {
		t.Errorf("Expected 2 packages in collision, got %d", len(collision.Packages))
	}
}

func TestSlotCollisionDetector_DifferentSlots_NoCollision(t *testing.T) {
	graph := NewDependencyGraph()

	// Add two versions of the same package in different slots
	pkg1 := pkg.NewPackage("dev-lang/python", "3.11.0", "3.11")
	pkg2 := pkg.NewPackage("dev-lang/python", "3.12.0", "3.12")

	graph.AddPackage(pkg1, true)
	// Manually add second with different slot
	node2 := &GraphNode{
		Package:      pkg2,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	graph.nodes["dev-lang/python-3.12.0"] = node2

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	// Should be no collision since they're in different slots
	if len(collisions) != 0 {
		t.Errorf("Expected no collisions for different slots, got %d", len(collisions))
	}
}

func TestSlotCollisionDetector_SubslotConflict(t *testing.T) {
	graph := NewDependencyGraph()

	// Add packages with same slot but different subslots
	pkg1 := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0", Subslot: "1.2.13"},
	}
	pkg2 := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.11",
		Slot:    pkg.Slot{Name: "0", Subslot: "1.2.11"},
	}

	graph.AddPackage(pkg1, true)
	node2 := &GraphNode{
		Package:      pkg2,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	graph.nodes["sys-libs/zlib-1.2.11"] = node2

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		t.Fatal("Expected collision for subslot mismatch")
	}

	// Check collision type is subslot
	foundSubslot := false
	for _, c := range collisions {
		if c.CollisionType == CollisionTypeSubslot {
			foundSubslot = true
			break
		}
	}

	if !foundSubslot {
		t.Error("Expected at least one subslot collision type")
	}
}

func TestSlotCollisionDetector_VersionConflict(t *testing.T) {
	graph := NewDependencyGraph()

	// Setup: pkgA requires zlib>=1.2.13, pkgB requires zlib<=1.2.11
	pkgA := pkg.NewPackage("app-a/foo", "1.0", "0")
	pkgB := pkg.NewPackage("app-b/bar", "1.0", "0")
	pkgZlib13 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkgZlib11 := pkg.NewPackage("sys-libs/zlib", "1.2.11", "0")

	graph.AddPackage(pkgA, true)
	graph.AddPackage(pkgB, true)
	graph.AddPackage(pkgZlib13, false)

	// Add second zlib version
	node11 := &GraphNode{
		Package:      pkgZlib11,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	graph.nodes["sys-libs/zlib-1.2.11"] = node11

	// Add dependency edges with version constraints
	constraintA := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: pkg.NewMinVersionConstraint("1.2.13"),
	}
	_ = graph.AddDependency("app-a/foo", "sys-libs/zlib", constraintA, EdgeTypeRuntime)

	constraintB := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: pkg.NewMaxVersionConstraint("1.2.11"),
	}
	// Add edge to the 1.2.11 version
	edge := &GraphEdge{
		From:       "app-b/bar",
		To:         "sys-libs/zlib-1.2.11",
		Constraint: constraintB,
		Type:       EdgeTypeRuntime,
	}
	graph.edges = append(graph.edges, edge)
	node11.Dependents = append(node11.Dependents, edge)

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		t.Fatal("Expected at least one collision")
	}

	// Find the version conflict
	hasVersionConflict := false
	for _, c := range collisions {
		if c.IsVersionConflict {
			hasVersionConflict = true
			break
		}
	}

	if !hasVersionConflict {
		t.Error("Expected version conflict to be detected")
	}
}

func TestCollisionResolver_ResolveVersionConflict(t *testing.T) {
	graph := NewDependencyGraph()

	// Version conflicts cannot be resolved with USE changes
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	graph.AddPackage(pkg1, true)

	node2 := &GraphNode{
		Package:      pkg.NewPackage("sys-libs/zlib", "1.2.11", "0"),
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	graph.nodes["sys-libs/zlib-1.2.11"] = node2

	detector := NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		t.Fatal("Expected collision for test setup")
	}

	// Mark as version conflict
	collisions[0].IsVersionConflict = true

	resolver := NewCollisionResolver(detector, nil)
	solutions := resolver.ResolveCollision(collisions[0])

	// Version conflicts should have no USE-based solutions
	if len(solutions) != 0 {
		t.Errorf("Expected no solutions for version conflict, got %d", len(solutions))
	}
}

func TestCollisionResolver_GenerateConfigurations(t *testing.T) {
	graph := NewDependencyGraph()

	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.2.11", "0")

	collision := &SlotCollision{
		SlotAtom:          "sys-libs/zlib:0",
		Packages:          []*pkg.Package{pkg1, pkg2},
		Parents:           make(map[string][]ParentAtom),
		CollisionType:     CollisionTypeUnspecific,
		IsVersionConflict: false,
	}

	detector := NewSlotCollisionDetector(graph, nil)
	resolver := NewCollisionResolver(detector, nil)

	configs := resolver.generateConfigurations(collision)

	// Should generate one config per package
	if len(configs) != 2 {
		t.Errorf("Expected 2 configurations, got %d", len(configs))
	}
}

func TestGenerateConflictReport(t *testing.T) {
	tests := []struct {
		name       string
		collisions []*SlotCollision
		contains   []string
		notContain []string
	}{
		{
			name:       "no collisions",
			collisions: nil,
			contains:   []string{"No slot conflicts detected"},
		},
		{
			name: "single collision",
			collisions: []*SlotCollision{
				{
					SlotAtom: "sys-libs/zlib:0",
					Packages: []*pkg.Package{
						pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"),
						pkg.NewPackage("sys-libs/zlib", "1.2.11", "0"),
					},
					Parents: map[string][]ParentAtom{
						"sys-libs/zlib-1.2.13": {},
						"sys-libs/zlib-1.2.11": {},
					},
				},
			},
			contains: []string{
				"sys-libs/zlib:0",
				"1.2.13",
				"1.2.11",
				"slot conflict",
			},
		},
		{
			name: "collision with USE flags",
			collisions: []*SlotCollision{
				{
					SlotAtom: "dev-libs/openssl:0",
					Packages: []*pkg.Package{
						{
							Name:     "dev-libs/openssl",
							Version:  "1.1.1",
							Slot:     pkg.Slot{Name: "0"},
							UseFlags: map[string]bool{"asm": true, "static-libs": false},
						},
					},
					Parents: make(map[string][]ParentAtom),
				},
			},
			contains: []string{
				"dev-libs/openssl:0",
				"USE=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := GenerateConflictReport(tt.collisions)

			for _, substr := range tt.contains {
				if !strings.Contains(report, substr) {
					t.Errorf("Expected report to contain %q, got:\n%s", substr, report)
				}
			}

			for _, substr := range tt.notContain {
				if strings.Contains(report, substr) {
					t.Errorf("Expected report NOT to contain %q, got:\n%s", substr, report)
				}
			}
		})
	}
}

func TestGenerateSolutionReport(t *testing.T) {
	tests := []struct {
		name      string
		solutions []*CollisionSolution
		contains  []string
	}{
		{
			name:      "no solutions",
			solutions: nil,
			contains:  []string{"No solutions found"},
		},
		{
			name: "single solution with USE changes",
			solutions: []*CollisionSolution{
				{
					UseChanges: []*UseChangeSuggestion{
						{
							Package: pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"),
							FlagChanges: map[string]bool{
								"static-libs": true,
							},
						},
					},
					Description: "Use zlib-1.2.13 with USE changes",
				},
			},
			contains: []string{
				"possible to solve",
				"+static-libs",
				"sys-libs/zlib",
			},
		},
		{
			name: "multiple solutions",
			solutions: []*CollisionSolution{
				{Description: "Solution 1"},
				{Description: "Solution 2"},
			},
			contains: []string{
				"Solution 1",
				"Solution 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := GenerateSolutionReport(tt.solutions)

			for _, substr := range tt.contains {
				if !strings.Contains(report, substr) {
					t.Errorf("Expected report to contain %q, got:\n%s", substr, report)
				}
			}
		})
	}
}

func TestCollisionSolution_Empty(t *testing.T) {
	solution := &CollisionSolution{
		UseChanges:       make([]*UseChangeSuggestion, 0),
		SelectedPackages: make(map[string]*pkg.Package),
		Description:      "Empty solution",
	}

	report := GenerateSolutionReport([]*CollisionSolution{solution})

	if !strings.Contains(report, "No USE changes required") {
		t.Errorf("Expected 'No USE changes required' for empty solution, got:\n%s", report)
	}
}

func TestParentAtom(t *testing.T) {
	parent := pkg.NewPackage("app-misc/hello", "2.10", "0")

	pa := ParentAtom{
		Parent: parent,
		Atom:   ">=sys-libs/zlib-1.2",
		Constraint: pkg.Constraint{
			Type:    pkg.ConstraintTypeVersion,
			Name:    "sys-libs/zlib",
			Version: pkg.NewMinVersionConstraint("1.2"),
		},
		IsCommandLine: false,
	}

	if pa.Parent.Name != "app-misc/hello" {
		t.Errorf("Expected parent name 'app-misc/hello', got %q", pa.Parent.Name)
	}

	if pa.IsCommandLine {
		t.Error("Expected IsCommandLine to be false")
	}

	// Test command-line atom
	cmdAtom := ParentAtom{
		Atom:          "sys-libs/zlib",
		IsCommandLine: true,
	}

	if cmdAtom.Parent != nil {
		t.Error("Expected nil parent for command-line atom")
	}

	if !cmdAtom.IsCommandLine {
		t.Error("Expected IsCommandLine to be true")
	}
}

func BenchmarkSlotCollisionDetector_LargeGraph(b *testing.B) {
	// Create a large graph with many packages
	graph := NewDependencyGraph()

	// Add 100 unique packages
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("category-%d/package-%d", i%10, i)
		p := pkg.NewPackage(name, "1.0", "0")
		graph.AddPackage(p, i == 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector := NewSlotCollisionDetector(graph, nil)
		_ = detector.DetectCollisions()
	}
}

func BenchmarkSlotCollisionDetector_WithCollisions(b *testing.B) {
	// Create graph with multiple collisions
	graph := NewDependencyGraph()

	// Add 10 packages, each with 5 versions in same slot
	for i := 0; i < 10; i++ {
		baseName := fmt.Sprintf("category/package-%d", i)
		for v := 0; v < 5; v++ {
			version := fmt.Sprintf("%d.0", v)
			p := pkg.NewPackage(baseName, version, "0")

			if i == 0 && v == 0 {
				graph.AddPackage(p, true)
			} else {
				node := &GraphNode{
					Package:      p,
					Dependencies: make([]*GraphEdge, 0),
					Dependents:   make([]*GraphEdge, 0),
					Depth:        -1,
					State:        NodeStateUnvisited,
				}
				key := fmt.Sprintf("%s-%s", baseName, version)
				graph.nodes[key] = node
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector := NewSlotCollisionDetector(graph, nil)
		_ = detector.DetectCollisions()
	}
}
