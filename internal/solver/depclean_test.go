package solver

import (
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestReverseDependencyGraph tests the reverse dependency graph functionality.
func TestReverseDependencyGraph(t *testing.T) {
	t.Run("NewReverseDependencyGraph", func(t *testing.T) {
		graph := NewReverseDependencyGraph()
		if graph == nil {
			t.Fatal("expected non-nil graph")
		}
		if graph.PackageCount() != 0 {
			t.Errorf("expected 0 packages, got %d", graph.PackageCount())
		}
	})

	t.Run("AddPackage", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		// Add a simple package without dependencies
		pkg1 := createTestInstalledPackage("sys-libs/zlib", "1.2.13", nil)
		graph.AddPackage(pkg1)

		if graph.PackageCount() != 1 {
			t.Errorf("expected 1 package, got %d", graph.PackageCount())
		}

		// Add a package with dependencies
		pkg2 := createTestInstalledPackage("app-misc/hello", "2.10",
			[]string{"sys-libs/zlib"})
		graph.AddPackage(pkg2)

		if graph.PackageCount() != 2 {
			t.Errorf("expected 2 packages, got %d", graph.PackageCount())
		}
	})

	t.Run("GetDependents", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		// sys-libs/zlib has no dependencies
		zlib := createTestInstalledPackage("sys-libs/zlib", "1.2.13", nil)
		graph.AddPackage(zlib)

		// app-misc/hello depends on sys-libs/zlib
		hello := createTestInstalledPackage("app-misc/hello", "2.10",
			[]string{"sys-libs/zlib"})
		graph.AddPackage(hello)

		// app-editors/vim depends on sys-libs/zlib
		vim := createTestInstalledPackage("app-editors/vim", "9.0",
			[]string{"sys-libs/zlib"})
		graph.AddPackage(vim)

		dependents := graph.GetDependents("sys-libs/zlib")
		if len(dependents) != 2 {
			t.Errorf("expected 2 dependents, got %d", len(dependents))
		}

		// Check that both hello and vim are dependents
		hasHello := false
		hasVim := false
		for _, dep := range dependents {
			if dep == "app-misc/hello" {
				hasHello = true
			}
			if dep == "app-editors/vim" {
				hasVim = true
			}
		}
		if !hasHello || !hasVim {
			t.Errorf("expected hello and vim as dependents, got %v", dependents)
		}
	})

	t.Run("GetDependencies", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		zlib := createTestInstalledPackage("sys-libs/zlib", "1.2.13", nil)
		graph.AddPackage(zlib)

		hello := createTestInstalledPackage("app-misc/hello", "2.10",
			[]string{"sys-libs/zlib", "sys-libs/glibc"})
		graph.AddPackage(hello)

		deps := graph.GetDependencies("app-misc/hello")
		if len(deps) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(deps))
		}
	})

	t.Run("HasDependents", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		zlib := createTestInstalledPackage("sys-libs/zlib", "1.2.13", nil)
		graph.AddPackage(zlib)

		// zlib has no dependents yet
		if graph.HasDependents("sys-libs/zlib") {
			t.Error("expected zlib to have no dependents")
		}

		// Add a package that depends on zlib
		hello := createTestInstalledPackage("app-misc/hello", "2.10",
			[]string{"sys-libs/zlib"})
		graph.AddPackage(hello)

		// Now zlib should have dependents
		if !graph.HasDependents("sys-libs/zlib") {
			t.Error("expected zlib to have dependents")
		}
	})

	t.Run("AllAtoms", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		graph.AddPackage(createTestInstalledPackage("sys-libs/zlib", "1.2.13", nil))
		graph.AddPackage(createTestInstalledPackage("app-misc/hello", "2.10", nil))
		graph.AddPackage(createTestInstalledPackage("dev-lang/go", "1.22", nil))

		atoms := graph.AllAtoms()
		if len(atoms) != 3 {
			t.Errorf("expected 3 atoms, got %d", len(atoms))
		}

		// Check sorting
		if atoms[0] != "app-misc/hello" {
			t.Errorf("expected first atom to be app-misc/hello, got %s", atoms[0])
		}
	})

	t.Run("NilPackage", func(t *testing.T) {
		graph := NewReverseDependencyGraph()

		// Should not panic on nil package
		graph.AddPackage(nil)
		if graph.PackageCount() != 0 {
			t.Errorf("expected 0 packages after adding nil, got %d", graph.PackageCount())
		}
	})
}

// TestExtractBaseAtom tests the atom extraction function.
func TestExtractBaseAtom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple atom",
			input:    "sys-libs/zlib",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with version",
			input:    "sys-libs/zlib-1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with revision",
			input:    "sys-libs/zlib-1.2.13-r1",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with >= operator",
			input:    ">=sys-libs/zlib-1.2",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with <= operator",
			input:    "<=sys-libs/zlib-1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with = operator",
			input:    "=sys-libs/zlib-1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with ~ operator",
			input:    "~sys-libs/zlib-1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with slot",
			input:    "sys-libs/zlib:0",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with slot and subslot",
			input:    "sys-libs/zlib:0/1.2.13",
			expected: "sys-libs/zlib",
		},
		{
			name:     "atom with USE flags",
			input:    "sys-libs/zlib[static-libs]",
			expected: "sys-libs/zlib",
		},
		{
			name:     "complex atom",
			input:    ">=sys-libs/zlib-1.2.13:0/1.2.13[static-libs]",
			expected: "sys-libs/zlib",
		},
		{
			name:     "package with hyphenated name",
			input:    "x11-libs/gtk+-3.24.38",
			expected: "x11-libs/gtk+",
		},
		{
			name:     "virtual package",
			input:    "virtual/libc-1",
			expected: "virtual/libc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractBaseAtom(tc.input)
			if result != tc.expected {
				t.Errorf("extractBaseAtom(%q) = %q, expected %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

// TestDepcleanCalculator tests the depclean calculator using internal methods.
func TestDepcleanCalculator(t *testing.T) {
	t.Run("NoOrphans", func(t *testing.T) {
		// Create database
		db := createTestDB(map[string][]string{
			"app-misc/hello": {"sys-libs/zlib"},
			"sys-libs/zlib":  {},
		})

		// Create calculator
		calc := NewDepcleanCalculator(db, nil)
		graph := calc.buildGraph()

		// Create world set manually
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello", "sys-libs/zlib"})

		// Mark needed packages
		needed := calc.markNeededPackages(worldSet, graph)

		// Find orphans
		result := calc.findOrphans(graph, needed, worldSet)

		if len(result.Orphans) != 0 {
			t.Errorf("expected 0 orphans, got %d", len(result.Orphans))
		}

		if len(result.Protected) != 2 {
			t.Errorf("expected 2 protected, got %d", len(result.Protected))
		}
	})

	t.Run("SimpleOrphan", func(t *testing.T) {
		// World: app-misc/hello
		// Installed: app-misc/hello, sys-libs/zlib, dev-libs/orphan
		// hello depends on zlib
		// orphan has no dependents and is not in world
		db := createTestDB(map[string][]string{
			"app-misc/hello":  {"sys-libs/zlib"},
			"sys-libs/zlib":   {},
			"dev-libs/orphan": {},
		})

		calc := NewDepcleanCalculator(db, nil)
		graph := calc.buildGraph()
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello"})
		needed := calc.markNeededPackages(worldSet, graph)
		result := calc.findOrphans(graph, needed, worldSet)

		if len(result.Orphans) != 1 {
			t.Errorf("expected 1 orphan, got %d", len(result.Orphans))
			for _, o := range result.Orphans {
				t.Logf("  orphan: %s", o.Atom)
			}
		}

		if len(result.Orphans) > 0 && result.Orphans[0].Atom != "dev-libs/orphan" {
			t.Errorf("expected dev-libs/orphan, got %s", result.Orphans[0].Atom)
		}
	})

	t.Run("TransitiveDependency", func(t *testing.T) {
		// World: app-misc/hello
		// hello -> sys-libs/zlib -> virtual/libc
		// All should be protected
		db := createTestDB(map[string][]string{
			"app-misc/hello": {"sys-libs/zlib"},
			"sys-libs/zlib":  {"virtual/libc"},
			"virtual/libc":   {},
		})

		calc := NewDepcleanCalculator(db, nil)
		graph := calc.buildGraph()
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello"})
		needed := calc.markNeededPackages(worldSet, graph)
		result := calc.findOrphans(graph, needed, worldSet)

		if len(result.Orphans) != 0 {
			t.Errorf("expected 0 orphans, got %d", len(result.Orphans))
			for _, o := range result.Orphans {
				t.Logf("  orphan: %s", o.Atom)
			}
		}

		if len(result.Protected) != 3 {
			t.Errorf("expected 3 protected, got %d", len(result.Protected))
		}
	})

	t.Run("MultipleOrphans", func(t *testing.T) {
		// World: app-misc/hello
		// Orphans: dev-libs/orphan1, dev-libs/orphan2
		db := createTestDB(map[string][]string{
			"app-misc/hello":   {},
			"dev-libs/orphan1": {},
			"dev-libs/orphan2": {},
		})

		calc := NewDepcleanCalculator(db, nil)
		graph := calc.buildGraph()
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello"})
		needed := calc.markNeededPackages(worldSet, graph)
		result := calc.findOrphans(graph, needed, worldSet)

		if len(result.Orphans) != 2 {
			t.Errorf("expected 2 orphans, got %d", len(result.Orphans))
		}
	})

	t.Run("WithExclusion", func(t *testing.T) {
		db := createTestDB(map[string][]string{
			"app-misc/hello":   {},
			"dev-libs/orphan1": {},
			"dev-libs/orphan2": {},
		})

		calc := NewDepcleanCalculator(db, nil)
		calc.SetOptions(&DepcleanOptions{
			Exclude:       []string{"dev-libs/orphan1"},
			IncludeSystem: true,
		})

		graph := calc.buildGraph()
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello"})
		needed := calc.markNeededPackages(worldSet, graph)
		result := calc.findOrphans(graph, needed, worldSet)

		// Only orphan2 should be orphaned, orphan1 is excluded
		if len(result.Orphans) != 1 {
			t.Errorf("expected 1 orphan, got %d", len(result.Orphans))
		}

		if len(result.Orphans) > 0 && result.Orphans[0].Atom != "dev-libs/orphan2" {
			t.Errorf("expected dev-libs/orphan2, got %s", result.Orphans[0].Atom)
		}
	})

	t.Run("NilDatabase", func(t *testing.T) {
		calc := NewDepcleanCalculator(nil, nil)
		_, err := calc.Calculate()
		if err == nil {
			t.Error("expected error for nil database")
		}
	})

	t.Run("EmptyWorld", func(t *testing.T) {
		// Empty world - all packages are orphans
		db := state.NewPackageDatabase("/var/db/pkg")
		_ = db.Add(createTestInstalledPackage("dev-libs/orphan1", "1.0", nil))
		_ = db.Add(createTestInstalledPackage("dev-libs/orphan2", "2.0", nil))

		calc := NewDepcleanCalculator(db, nil)
		result, err := calc.Calculate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Orphans) != 2 {
			t.Errorf("expected 2 orphans, got %d", len(result.Orphans))
		}
	})
}

// TestOrphanReason tests the orphan reason type.
func TestOrphanReason(t *testing.T) {
	tests := []struct {
		reason   OrphanReason
		expected string
	}{
		{OrphanReasonNotInWorld, "not in @world"},
		{OrphanReasonNotRequired, "not required by @world"},
		{OrphanReasonUnused, "no longer needed"},
		{OrphanReason(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if tc.reason.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.reason.String())
			}
		})
	}
}

// TestFormatDepcleanResult tests the result formatting function.
func TestFormatDepcleanResult(t *testing.T) {
	t.Run("NoOrphans", func(t *testing.T) {
		result := &DepcleanResult{
			Orphans:   []*OrphanInfo{},
			Protected: []string{"app-misc/hello", "sys-libs/zlib"},
		}

		output := FormatDepcleanResult(result, false)
		if output == "" {
			t.Error("expected non-empty output")
		}

		if !strings.Contains(output, "No orphaned packages found") {
			t.Error("expected 'No orphaned packages found' in output")
		}
	})

	t.Run("WithOrphans", func(t *testing.T) {
		result := &DepcleanResult{
			Orphans: []*OrphanInfo{
				{
					Atom:    "dev-libs/orphan",
					Version: "1.0",
					Reason:  OrphanReasonNotRequired,
					Size:    1024 * 1024, // 1 MB
				},
			},
			TotalSize: 1024 * 1024,
		}

		output := FormatDepcleanResult(result, true)
		if !strings.Contains(output, "pretend") {
			t.Error("expected 'pretend' in output")
		}
		if !strings.Contains(output, "dev-libs/orphan") {
			t.Error("expected orphan package in output")
		}
	})
}

// TestFormatBytes tests the byte formatting function.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := formatBytes(tc.bytes)
			if result != tc.expected {
				t.Errorf("formatBytes(%d) = %q, expected %q",
					tc.bytes, result, tc.expected)
			}
		})
	}
}

// TestDepcleanComplexGraph tests depclean with a more complex dependency graph.
func TestDepcleanComplexGraph(t *testing.T) {
	// Complex scenario:
	// World: [app-misc/hello, dev-lang/python]
	//
	// app-misc/hello -> sys-libs/zlib
	// dev-lang/python -> sys-libs/zlib, dev-libs/libffi
	// sys-libs/zlib -> (none)
	// dev-libs/libffi -> (none)
	// dev-libs/orphan1 -> sys-libs/zlib (orphan depends on needed package)
	// dev-libs/orphan2 -> dev-libs/orphan1 (chain of orphans)

	db := createTestDB(map[string][]string{
		"app-misc/hello":   {"sys-libs/zlib"},
		"dev-lang/python":  {"sys-libs/zlib", "dev-libs/libffi"},
		"sys-libs/zlib":    {},
		"dev-libs/libffi":  {},
		"dev-libs/orphan1": {"sys-libs/zlib"},
		"dev-libs/orphan2": {"dev-libs/orphan1"},
	})

	calc := NewDepcleanCalculator(db, nil)
	graph := calc.buildGraph()
	worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello", "dev-lang/python"})
	needed := calc.markNeededPackages(worldSet, graph)
	result := calc.findOrphans(graph, needed, worldSet)

	// Expected orphans: orphan1, orphan2
	// Expected protected: hello, python, zlib, libffi
	if len(result.Orphans) != 2 {
		t.Errorf("expected 2 orphans, got %d", len(result.Orphans))
		for _, o := range result.Orphans {
			t.Logf("  orphan: %s", o.Atom)
		}
	}

	if len(result.Protected) != 4 {
		t.Errorf("expected 4 protected, got %d", len(result.Protected))
		for _, p := range result.Protected {
			t.Logf("  protected: %s", p)
		}
	}

	// Verify specific orphans
	orphanAtoms := make(map[string]bool)
	for _, o := range result.Orphans {
		orphanAtoms[o.Atom] = true
	}

	if !orphanAtoms["dev-libs/orphan1"] {
		t.Error("expected dev-libs/orphan1 to be orphan")
	}
	if !orphanAtoms["dev-libs/orphan2"] {
		t.Error("expected dev-libs/orphan2 to be orphan")
	}
}

// TestDepcleanDiamondDependency tests depclean with diamond dependency pattern.
func TestDepcleanDiamondDependency(t *testing.T) {
	// Diamond pattern:
	// World: [app-misc/top]
	//
	//     top
	//    /   \
	//  mid1  mid2
	//    \   /
	//    bottom
	//
	// All should be protected

	db := createTestDB(map[string][]string{
		"app-misc/top":    {"dev-libs/mid1", "dev-libs/mid2"},
		"dev-libs/mid1":   {"sys-libs/bottom"},
		"dev-libs/mid2":   {"sys-libs/bottom"},
		"sys-libs/bottom": {},
	})

	calc := NewDepcleanCalculator(db, nil)
	graph := calc.buildGraph()
	worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/top"})
	needed := calc.markNeededPackages(worldSet, graph)
	result := calc.findOrphans(graph, needed, worldSet)

	if len(result.Orphans) != 0 {
		t.Errorf("expected 0 orphans in diamond pattern, got %d", len(result.Orphans))
	}

	if len(result.Protected) != 4 {
		t.Errorf("expected 4 protected in diamond pattern, got %d", len(result.Protected))
	}
}

// Helper functions for tests

func createTestInstalledPackage(name, version string, deps []string) *state.InstalledPackage {
	p := pkg.NewPackage(name, version, "0")

	for _, dep := range deps {
		p.AddDependency(pkg.Constraint{
			Name: dep,
			Type: pkg.ConstraintTypeVersion,
		})
	}

	return &state.InstalledPackage{
		Package:     p,
		InstallTime: time.Now(),
		Size:        1024, // 1 KB default
	}
}

// createTestDB creates a test database with the given packages and their dependencies.
func createTestDB(packages map[string][]string) *state.PackageDatabase {
	db := state.NewPackageDatabase("/var/db/pkg")

	for atom, deps := range packages {
		installedPkg := createTestInstalledPackage(atom, "1.0", deps)
		_ = db.Add(installedPkg)
	}

	return db
}

// TestDepcleanCalculatorWithMockWorld tests using a different approach
func TestDepcleanCalculatorWithMockWorld(t *testing.T) {
	t.Run("DirectCalculation", func(t *testing.T) {
		// Create database
		packages := map[string][]string{
			"app-misc/hello":  {"sys-libs/zlib"},
			"sys-libs/zlib":   {},
			"dev-libs/orphan": {},
		}
		db := createTestDB(packages)

		// Create calculator without set manager
		calc := NewDepcleanCalculator(db, nil)

		// Build graph manually for testing
		graph := calc.buildGraph()
		if graph.PackageCount() != 3 {
			t.Errorf("expected 3 packages in graph, got %d", graph.PackageCount())
		}

		// Test with mock world (manually simulate world set behavior)
		worldSet := state.NewPackageSet(state.SetWorld, []string{"app-misc/hello"})

		// Test markNeededPackages
		needed := calc.markNeededPackages(worldSet, graph)

		// hello and its dependency zlib should be needed
		if !needed["app-misc/hello"] {
			t.Error("expected app-misc/hello to be needed")
		}
		if !needed["sys-libs/zlib"] {
			t.Error("expected sys-libs/zlib to be needed")
		}
		if needed["dev-libs/orphan"] {
			t.Error("expected dev-libs/orphan to NOT be needed")
		}

		// Test findOrphans
		result := calc.findOrphans(graph, needed, worldSet)

		if len(result.Orphans) != 1 {
			t.Errorf("expected 1 orphan, got %d", len(result.Orphans))
		}
		if len(result.Orphans) > 0 && result.Orphans[0].Atom != "dev-libs/orphan" {
			t.Errorf("expected dev-libs/orphan, got %s", result.Orphans[0].Atom)
		}
	})
}
