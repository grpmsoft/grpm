package solver

import (
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestDependencyGraph_DetectConflicts_NoConflicts(t *testing.T) {
	g := NewDependencyGraph()

	// Build simple graph with no conflicts
	pkgA := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkgB := pkg.NewPackage("app-arch/gzip", "1.12", "0")

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
	_ = g.AddDependency("sys-libs/zlib", "app-arch/gzip", constraint, EdgeTypeRuntime)

	// Should not detect any conflicts
	conflicts := g.DetectConflicts()
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	if g.HasConflicts() {
		t.Error("HasConflicts() should return false")
	}
}

func TestDependencyGraph_DetectSlotConflicts(t *testing.T) {
	g := NewDependencyGraph()

	// Add two versions of the same package in the same slot
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.2.11", "0")

	g.AddPackage(pkg1, true)
	// Note: AddPackage has duplicate check, so we need to directly add to nodes
	// For testing purposes, we simulate the conflict scenario
	node2 := &GraphNode{
		Package:      pkg2,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	g.nodes["sys-libs/zlib-1.2.11"] = node2

	// Should detect slot conflict
	slotConflicts := g.GetConflictsByType(ConflictTypeSlot)
	if len(slotConflicts) == 0 {
		t.Fatal("Should have detected slot conflict")
	}

	conflict := slotConflicts[0]
	if conflict.Type != ConflictTypeSlot {
		t.Errorf("Expected ConflictTypeSlot, got %v", conflict.Type)
	}

	if conflict.Severity != SeverityError {
		t.Errorf("Expected SeverityError, got %v", conflict.Severity)
	}

	if len(conflict.Packages) != 2 {
		t.Errorf("Expected 2 conflicting packages, got %d", len(conflict.Packages))
	}

	// Check error message
	errMsg := conflict.Error()
	if !strings.Contains(errMsg, "slot conflict") {
		t.Errorf("Error message should contain 'slot conflict': %s", errMsg)
	}
}

func TestDependencyGraph_DetectVersionConflicts(t *testing.T) {
	g := NewDependencyGraph()

	// Create scenario: A and B both depend on C, but require different versions
	pkgA := pkg.NewPackage("pkg-a", "1.0", "0")
	pkgB := pkg.NewPackage("pkg-b", "1.0", "0")
	pkgC := pkg.NewPackage("pkg-c", "1.5", "0") // Current version

	g.AddPackage(pkgA, true)
	g.AddPackage(pkgB, true)
	g.AddPackage(pkgC, false)

	// A requires C >= 2.0
	constraint1 := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewMinVersionConstraint("2.0"),
	}
	_ = g.AddDependency("pkg-a", "pkg-c", constraint1, EdgeTypeRuntime)

	// B requires C <= 1.0
	constraint2 := pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewMaxVersionConstraint("1.0"),
	}
	_ = g.AddDependency("pkg-b", "pkg-c", constraint2, EdgeTypeRuntime)

	// Detect conflicts
	conflicts := g.GetConflictsByType(ConflictTypeVersion)
	if len(conflicts) == 0 {
		t.Fatal("Should have detected version conflict")
	}

	conflict := conflicts[0]
	if conflict.Type != ConflictTypeVersion {
		t.Errorf("Expected ConflictTypeVersion, got %v", conflict.Type)
	}

	if !strings.Contains(conflict.Details, "does not satisfy") {
		t.Errorf("Error message should describe version mismatch: %s", conflict.Details)
	}
}

func TestConflictType_String(t *testing.T) {
	tests := []struct {
		conflictType ConflictType
		expected     string
	}{
		{ConflictTypeSlot, "slot conflict"},
		{ConflictTypeVersion, "version conflict"},
		{ConflictTypeUseFlag, "USE flag conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.conflictType.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityWarning, "warning"},
		{SeverityError, "error"},
		{SeverityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.severity.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestConflict_Error(t *testing.T) {
	conflict := &ConflictError{
		Type:     ConflictTypeSlot,
		Packages: []string{"pkg-a", "pkg-b"},
		Details:  "multiple versions in slot 0",
		Severity: SeverityError,
	}

	errMsg := conflict.Error()

	// Check that error message contains key components
	expectedSubstrings := []string{
		"slot conflict",
		"error",
		"pkg-a",
		"pkg-b",
		"multiple versions",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(errMsg, substr) {
			t.Errorf("Error message should contain '%s': %s", substr, errMsg)
		}
	}
}

func TestExtractBaseName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard package",
			input:    "sys-libs/zlib-1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "package with revision",
			input:    "app-arch/gzip-1.12-r1",
			expected: "app-arch/gzip",
		},
		{
			name:     "package with dash in name",
			input:    "dev-lang/go-bootstrap-1.22.0",
			expected: "dev-lang/go-bootstrap",
		},
		{
			name:     "package without version",
			input:    "sys-apps/portage",
			expected: "sys-apps/portage",
		},
		{
			name:     "invalid format",
			input:    "invalid-package",
			expected: "invalid-package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSatisfiesVersionConstraint(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint *pkg.VersionConstraint
		expected   bool
	}{
		{
			name:       "exact match",
			version:    "1.2.3",
			constraint: pkg.NewExactVersionConstraint("1.2.3"),
			expected:   true,
		},
		{
			name:       "exact mismatch",
			version:    "1.2.4",
			constraint: pkg.NewExactVersionConstraint("1.2.3"),
			expected:   false,
		},
		{
			name:       "greater than - satisfied",
			version:    "2.0.0",
			constraint: pkg.NewVersionConstraint(pkg.OpGreater, "1.0.0"),
			expected:   true,
		},
		{
			name:       "greater than - not satisfied",
			version:    "0.9.0",
			constraint: pkg.NewVersionConstraint(pkg.OpGreater, "1.0.0"),
			expected:   false,
		},
		{
			name:       "minimum version - satisfied",
			version:    "1.5.0",
			constraint: pkg.NewMinVersionConstraint("1.2.0"),
			expected:   true,
		},
		{
			name:       "minimum version - not satisfied",
			version:    "1.0.0",
			constraint: pkg.NewMinVersionConstraint("1.2.0"),
			expected:   false,
		},
		{
			name:       "maximum version - satisfied",
			version:    "1.0.0",
			constraint: pkg.NewMaxVersionConstraint("2.0.0"),
			expected:   true,
		},
		{
			name:       "maximum version - not satisfied",
			version:    "3.0.0",
			constraint: pkg.NewMaxVersionConstraint("2.0.0"),
			expected:   false,
		},
		{
			name:       "nil constraint",
			version:    "1.0.0",
			constraint: nil,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := satisfiesVersionConstraint(tt.version, tt.constraint)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDependencyGraph_ConflictReport(t *testing.T) {
	g := NewDependencyGraph()

	// Graph with no conflicts
	report := g.ConflictReport()
	if !strings.Contains(report, "No conflicts detected") {
		t.Errorf("Expected 'No conflicts detected', got: %s", report)
	}

	// Add slot conflict
	pkg1 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkg2 := pkg.NewPackage("sys-libs/zlib", "1.2.11", "0")

	g.AddPackage(pkg1, true)
	node2 := &GraphNode{
		Package:      pkg2,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1,
		State:        NodeStateUnvisited,
	}
	g.nodes["sys-libs/zlib-1.2.11"] = node2

	report = g.ConflictReport()

	// Check report contains key information
	expectedSubstrings := []string{
		"Detected",
		"conflicts",
		"ERROR",
		"slot conflict",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(report, substr) {
			t.Errorf("Report should contain '%s': %s", substr, report)
		}
	}
}

func BenchmarkDetectConflicts_NoConflicts(b *testing.B) {
	g := NewDependencyGraph()

	// Build large graph without conflicts
	root := pkg.NewPackage("root", "1.0", "0")
	g.AddPackage(root, true)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
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
		_ = g.DetectConflicts()
	}
}

func BenchmarkDetectConflicts_WithSlotConflicts(b *testing.B) {
	g := NewDependencyGraph()

	// Build graph with slot conflicts
	for i := 0; i < 10; i++ {
		for v := 0; v < 5; v++ {
			pkgName := "category/pkg-" + string(rune(i))
			version := "1." + string(rune('0'+v))
			p := pkg.NewPackage(pkgName, version, "0")

			if i == 0 && v == 0 {
				g.AddPackage(p, true)
			} else {
				// Manually add to create conflicts
				node := &GraphNode{
					Package:      p,
					Dependencies: make([]*GraphEdge, 0),
					Dependents:   make([]*GraphEdge, 0),
					Depth:        -1,
					State:        NodeStateUnvisited,
				}
				g.nodes[pkgName+"-"+version] = node
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.DetectConflicts()
	}
}
