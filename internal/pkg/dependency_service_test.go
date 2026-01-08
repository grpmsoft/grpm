package pkg

import (
	"fmt"
	"testing"
)

// TestNewDependencyService tests service creation
func TestNewDependencyService(t *testing.T) {
	service := NewDependencyService()

	if service == nil {
		t.Error("NewDependencyService() returned nil")
	}
}

// TestDependencyService_ResolveDependencyTree tests dependency tree resolution
func TestDependencyService_ResolveDependencyTree(t *testing.T) {
	service := NewDependencyService()

	// Create test packages
	// zlib has no dependencies
	zlib := NewPackage("sys-libs/zlib", "1.2.13", "0")

	// openssl depends on zlib
	openssl := NewPackage("dev-libs/openssl", "1.1.1", "0")
	openssl.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "sys-libs/zlib",
	})

	// hello depends on openssl (which depends on zlib)
	hello := NewPackage("app-misc/hello", "2.10", "0")
	hello.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/openssl",
	})

	// Package loader function (simulates repository)
	packageLoader := func(name string) (*Package, error) {
		switch name {
		case "sys-libs/zlib":
			return zlib, nil
		case "dev-libs/openssl":
			return openssl, nil
		default:
			return nil, fmt.Errorf("package %s not found", name)
		}
	}

	// Resolve dependency tree for hello
	packages, err := service.ResolveDependencyTree(hello, packageLoader)

	if err != nil {
		t.Errorf("ResolveDependencyTree() unexpected error: %v", err)
	}

	// Should contain: hello, openssl, zlib (3 packages total)
	expectedPackages := 3
	if len(packages) != expectedPackages {
		t.Errorf("ResolveDependencyTree() returned %d packages, expected %d", len(packages), expectedPackages)
	}

	// Verify all packages are present
	expectedNames := []string{"app-misc/hello", "dev-libs/openssl", "sys-libs/zlib"}
	for _, name := range expectedNames {
		if _, exists := packages[name]; !exists {
			t.Errorf("ResolveDependencyTree() missing package %s", name)
		}
	}
}

// TestDependencyService_ResolveDependencyTree_CircularDependency tests circular dependency handling
func TestDependencyService_ResolveDependencyTree_CircularDependency(t *testing.T) {
	service := NewDependencyService()

	// Create packages with circular dependency
	// pkgA depends on pkgB
	pkgA := NewPackage("app-test/pkg-a", "1.0", "0")
	pkgA.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "app-test/pkg-b",
	})

	// pkgB depends on pkgA (circular!)
	pkgB := NewPackage("app-test/pkg-b", "1.0", "0")
	pkgB.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "app-test/pkg-a",
	})

	packageLoader := func(name string) (*Package, error) {
		switch name {
		case "app-test/pkg-a":
			return pkgA, nil
		case "app-test/pkg-b":
			return pkgB, nil
		default:
			return nil, fmt.Errorf("package %s not found", name)
		}
	}

	// Resolve - should handle circular dependency gracefully
	packages, err := service.ResolveDependencyTree(pkgA, packageLoader)

	if err != nil {
		t.Errorf("ResolveDependencyTree() with circular dependency error: %v", err)
	}

	// Should contain both packages without infinite loop
	if len(packages) != 2 {
		t.Errorf("ResolveDependencyTree() with circular dependency returned %d packages, expected 2", len(packages))
	}
}

// TestDependencyService_ResolveDependencyTree_MissingDependency tests missing dependency handling
func TestDependencyService_ResolveDependencyTree_MissingDependency(t *testing.T) {
	service := NewDependencyService()

	// Package with missing dependency
	pkg := NewPackage("app-test/test", "1.0", "0")
	pkg.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "non-existent/package",
	})

	packageLoader := func(name string) (*Package, error) {
		return nil, fmt.Errorf("package %s not found", name)
	}

	// Resolve - should handle missing dependency gracefully
	packages, err := service.ResolveDependencyTree(pkg, packageLoader)

	// According to implementation, missing dependencies are logged but not fatal
	if err != nil {
		t.Errorf("ResolveDependencyTree() with missing dependency should not error, got: %v", err)
	}

	// Should contain at least the root package
	if len(packages) < 1 {
		t.Error("ResolveDependencyTree() should contain at least the root package")
	}

	// Root package should be present
	if _, exists := packages["app-test/test"]; !exists {
		t.Error("ResolveDependencyTree() missing root package")
	}
}

// TestDependencyService_FindConflicts tests conflict detection
func TestDependencyService_FindConflicts(t *testing.T) {
	service := NewDependencyService()

	tests := []struct {
		name              string
		packages          []*Package
		expectedConflicts int
	}{
		{
			name: "No conflicts - different packages",
			packages: []*Package{
				NewPackage("sys-libs/zlib", "1.2.13", "0"),
				NewPackage("dev-libs/openssl", "1.1.1", "0"),
			},
			expectedConflicts: 0,
		},
		{
			name: "No conflicts - same package different versions same slot",
			packages: []*Package{
				NewPackage("sys-libs/zlib", "1.2.13", "0"),
				NewPackage("sys-libs/zlib", "1.3.0", "0"),
			},
			expectedConflicts: 0,
		},
		{
			name: "Conflict - different packages same slot different subslots",
			packages: []*Package{
				NewPackage("dev-lang/python", "3.11.5", "3.11/3.11"),
				NewPackage("dev-lang/python-exec", "2.4.10", "3.11/3.10"),
			},
			expectedConflicts: 1,
		},
		{
			name: "Multiple conflicts",
			packages: []*Package{
				NewPackage("app-test/pkg-a", "1.0", "1/1"),
				NewPackage("app-test/pkg-b", "1.0", "1/2"),
				NewPackage("app-test/pkg-c", "1.0", "1/3"),
			},
			expectedConflicts: 3, // a-b, a-c, b-c
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflicts := service.FindConflicts(tt.packages)

			if len(conflicts) != tt.expectedConflicts {
				t.Errorf("FindConflicts() returned %d conflicts, expected %d", len(conflicts), tt.expectedConflicts)
			}
		})
	}
}

// TestDependencyService_FilterByConstraint tests constraint-based filtering
func TestDependencyService_FilterByConstraint(t *testing.T) {
	service := NewDependencyService()

	packages := []*Package{
		NewPackage("sys-libs/zlib", "1.2.13", "0"),
		NewPackage("sys-libs/zlib", "1.3.0", "0"),
		NewPackage("sys-libs/zlib", "1.1.0", "0"),
		NewPackage("dev-libs/openssl", "1.1.1", "0"),
	}

	tests := []struct {
		name          string
		constraint    Constraint
		expectedCount int
		expectedNames []string
	}{
		{
			name: "Filter by package name only",
			constraint: Constraint{
				Type: ConstraintTypeVersion,
				Name: "sys-libs/zlib",
			},
			expectedCount: 3,
		},
		{
			name: "Filter by exact version",
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewExactVersionConstraint("1.2.13"),
			},
			expectedCount: 1,
		},
		{
			name: "Filter by >= constraint",
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewMinVersionConstraint("1.2.0"),
			},
			expectedCount: 2, // 1.2.13 and 1.3.0
		},
		{
			name: "Filter by <= constraint",
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewMaxVersionConstraint("1.2.13"),
			},
			expectedCount: 2, // 1.1.0 and 1.2.13
		},
		{
			name: "Filter non-existent package",
			constraint: Constraint{
				Type: ConstraintTypeVersion,
				Name: "non-existent/package",
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := service.FilterByConstraint(packages, tt.constraint)

			if len(filtered) != tt.expectedCount {
				t.Errorf("FilterByConstraint() returned %d packages, expected %d", len(filtered), tt.expectedCount)
			}

			// Verify all filtered packages satisfy the constraint
			for _, pkg := range filtered {
				if !pkg.SatisfiesConstraint(tt.constraint) {
					t.Errorf("FilterByConstraint() returned package %s that doesn't satisfy constraint", pkg.Name)
				}
			}
		})
	}
}

// TestDependencyService_ValidateDependencyGraph tests dependency graph validation
func TestDependencyService_ValidateDependencyGraph(t *testing.T) {
	service := NewDependencyService()

	tests := []struct {
		name        string
		packages    map[string]*Package
		shouldError bool
	}{
		{
			name: "Valid simple graph",
			packages: map[string]*Package{
				"sys-libs/zlib": NewPackage("sys-libs/zlib", "1.2.13", "0"),
			},
			shouldError: false,
		},
		{
			name: "Valid complex graph",
			packages: map[string]*Package{
				"sys-libs/zlib":    NewPackage("sys-libs/zlib", "1.2.13", "0"),
				"dev-libs/openssl": NewPackage("dev-libs/openssl", "1.1.1", "0"),
				"app-misc/hello":   NewPackage("app-misc/hello", "2.10", "0"),
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateDependencyGraph(tt.packages)

			if tt.shouldError && err == nil {
				t.Error("ValidateDependencyGraph() expected error but got nil")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("ValidateDependencyGraph() unexpected error: %v", err)
			}
		})
	}
}

// TestDependencyService_ResolveDependencyTree_ComplexGraph tests complex dependency resolution
func TestDependencyService_ResolveDependencyTree_ComplexGraph(t *testing.T) {
	service := NewDependencyService()

	// Create a more complex dependency graph:
	// app depends on lib-a and lib-b
	// lib-a depends on lib-common
	// lib-b depends on lib-common
	// lib-common has no dependencies

	libCommon := NewPackage("dev-libs/lib-common", "1.0", "0")

	libA := NewPackage("dev-libs/lib-a", "1.0", "0")
	libA.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/lib-common",
	})

	libB := NewPackage("dev-libs/lib-b", "1.0", "0")
	libB.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/lib-common",
	})

	app := NewPackage("app-test/app", "1.0", "0")
	app.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/lib-a",
	})
	app.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/lib-b",
	})

	packageLoader := func(name string) (*Package, error) {
		switch name {
		case "dev-libs/lib-common":
			return libCommon, nil
		case "dev-libs/lib-a":
			return libA, nil
		case "dev-libs/lib-b":
			return libB, nil
		default:
			return nil, fmt.Errorf("package %s not found", name)
		}
	}

	packages, err := service.ResolveDependencyTree(app, packageLoader)

	if err != nil {
		t.Errorf("ResolveDependencyTree() unexpected error: %v", err)
	}

	// Should contain: app, lib-a, lib-b, lib-common (4 packages total)
	// lib-common should only appear once despite being depended on by both lib-a and lib-b
	if len(packages) != 4 {
		t.Errorf("ResolveDependencyTree() returned %d packages, expected 4", len(packages))
	}

	// Verify all packages are present
	expectedNames := []string{"app-test/app", "dev-libs/lib-a", "dev-libs/lib-b", "dev-libs/lib-common"}
	for _, name := range expectedNames {
		if _, exists := packages[name]; !exists {
			t.Errorf("ResolveDependencyTree() missing package %s", name)
		}
	}
}

// BenchmarkDependencyService_ResolveDependencyTree benchmarks dependency resolution
func BenchmarkDependencyService_ResolveDependencyTree(b *testing.B) {
	service := NewDependencyService()

	zlib := NewPackage("sys-libs/zlib", "1.2.13", "0")
	openssl := NewPackage("dev-libs/openssl", "1.1.1", "0")
	openssl.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "sys-libs/zlib",
	})
	hello := NewPackage("app-misc/hello", "2.10", "0")
	hello.AddDependency(Constraint{
		Type: ConstraintTypeVersion,
		Name: "dev-libs/openssl",
	})

	packageLoader := func(name string) (*Package, error) {
		switch name {
		case "sys-libs/zlib":
			return zlib, nil
		case "dev-libs/openssl":
			return openssl, nil
		default:
			return nil, fmt.Errorf("package %s not found", name)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.ResolveDependencyTree(hello, packageLoader)
	}
}

// BenchmarkDependencyService_FindConflicts benchmarks conflict detection
func BenchmarkDependencyService_FindConflicts(b *testing.B) {
	service := NewDependencyService()

	packages := []*Package{
		NewPackage("dev-lang/python", "3.11.5", "3.11/3.11"),
		NewPackage("dev-lang/python-exec", "2.4.10", "3.11/3.10"),
		NewPackage("sys-libs/zlib", "1.2.13", "0"),
		NewPackage("dev-libs/openssl", "1.1.1", "0"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.FindConflicts(packages)
	}
}

// BenchmarkDependencyService_FilterByConstraint benchmarks constraint filtering
func BenchmarkDependencyService_FilterByConstraint(b *testing.B) {
	service := NewDependencyService()

	packages := []*Package{
		NewPackage("sys-libs/zlib", "1.2.13", "0"),
		NewPackage("sys-libs/zlib", "1.3.0", "0"),
		NewPackage("sys-libs/zlib", "1.1.0", "0"),
		NewPackage("dev-libs/openssl", "1.1.1", "0"),
	}

	constraint := Constraint{
		Type:    ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: NewMinVersionConstraint("1.2.0"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.FilterByConstraint(packages, constraint)
	}
}
