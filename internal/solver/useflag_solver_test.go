package solver

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestUseFlagSolver_BasicOperations tests basic USE flag operations
func TestUseFlagSolver_BasicOperations(t *testing.T) {
	solver := NewUseFlagSolver()

	// Set global USE flag
	solver.SetGlobalUseFlag("ssl", true)
	solver.SetGlobalUseFlag("mysql", false)

	// Test IsUseFlagEnabled
	if !solver.IsUseFlagEnabled("any-package", "ssl") {
		t.Error("Expected ssl to be enabled globally")
	}

	if solver.IsUseFlagEnabled("any-package", "mysql") {
		t.Error("Expected mysql to be disabled globally")
	}

	// Package-specific override
	solver.SetPackageUseFlags("sys-libs/zlib", []string{"-ssl", "static-libs"})

	if solver.IsUseFlagEnabled("sys-libs/zlib", "ssl") {
		t.Error("Expected ssl to be disabled for zlib (package override)")
	}

	if !solver.IsUseFlagEnabled("sys-libs/zlib", "static-libs") {
		t.Error("Expected static-libs to be enabled for zlib")
	}
}

// TestUseFlagSolver_EvaluateCondition tests USE flag condition evaluation
func TestUseFlagSolver_EvaluateCondition(t *testing.T) {
	solver := NewUseFlagSolver()
	solver.SetGlobalUseFlag("ssl", true)
	solver.SetGlobalUseFlag("mysql", false)

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{"Empty condition", "", true},
		{"Single enabled flag", "ssl", true},
		{"Single disabled flag", "mysql", false},
		{"Negation of disabled", "-mysql", true},
		{"Negation of disabled (! syntax)", "!mysql", true},
		{"Negation of enabled", "-ssl", false},
		{"Multiple enabled", "ssl", true},
		{"Mixed condition", "ssl,-mysql", true},
		{"Failed condition", "ssl,mysql", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := solver.EvaluateUseCondition("test-pkg", tt.condition)
			if result != tt.expected {
				t.Errorf("EvaluateUseCondition(%q) = %v, expected %v", tt.condition, result, tt.expected)
			}
		})
	}
}

// TestUseFlagSolver_FilterDependencies tests dependency filtering by USE flags
func TestUseFlagSolver_FilterDependencies(t *testing.T) {
	solver := NewUseFlagSolver()
	solver.SetGlobalUseFlag("ssl", true)
	solver.SetGlobalUseFlag("mysql", false)

	// Create package with conditional dependencies
	p := pkg.NewPackage("app-test/myapp", "1.0", "0")
	p.AddDependency(pkg.Constraint{
		Type:      pkg.ConstraintTypeVersion,
		Name:      "dev-libs/openssl",
		Condition: "ssl", // Only if ssl USE flag enabled
	})
	p.AddDependency(pkg.Constraint{
		Type:      pkg.ConstraintTypeVersion,
		Name:      "dev-db/mysql",
		Condition: "mysql", // Only if mysql USE flag enabled
	})
	p.AddDependency(pkg.Constraint{
		Type: pkg.ConstraintTypeVersion,
		Name: "sys-libs/zlib", // Unconditional
	})

	// Filter dependencies
	filtered := solver.FilterDependenciesByUseFlags(p)

	// Should include openssl (ssl=true) and zlib (unconditional)
	// Should NOT include mysql (mysql=false)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered dependencies, got %d", len(filtered))
	}

	// Verify correct dependencies included
	foundOpenSSL := false
	foundZlib := false
	for _, dep := range filtered {
		if dep.Name == "dev-libs/openssl" {
			foundOpenSSL = true
		}
		if dep.Name == "sys-libs/zlib" {
			foundZlib = true
		}
		if dep.Name == "dev-db/mysql" {
			t.Error("MySQL should be filtered out (USE flag disabled)")
		}
	}

	if !foundOpenSSL {
		t.Error("Expected openssl dependency (ssl USE flag enabled)")
	}
	if !foundZlib {
		t.Error("Expected zlib dependency (unconditional)")
	}
}

// TestUseFlagSolver_ResolveUseFlagsForPackage tests USE flag resolution
func TestUseFlagSolver_ResolveUseFlagsForPackage(t *testing.T) {
	solver := NewUseFlagSolver()
	solver.SetGlobalUseFlag("ssl", true)
	solver.SetGlobalUseFlag("static", false)

	// Create package with IUSE
	p := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	p.UseFlags["ssl"] = false         // Available but not enabled by default
	p.UseFlags["static-libs"] = false // Available but not enabled by default

	// Override for this specific package
	solver.SetPackageUseFlags("sys-libs/zlib", []string{"static-libs", "-ssl"})

	resolved, err := solver.ResolveUseFlagsForPackage(p)
	if err != nil {
		t.Errorf("ResolveUseFlagsForPackage() error: %v", err)
	}

	// ssl should be disabled (package override)
	if resolved["ssl"] {
		t.Error("Expected ssl to be disabled (package override)")
	}

	// static-libs should be enabled (package override)
	if !resolved["static-libs"] {
		t.Error("Expected static-libs to be enabled (package override)")
	}
}

// TestUseFlagSolver_GetEnabledUseFlags tests enabled USE flag extraction
func TestUseFlagSolver_GetEnabledUseFlags(t *testing.T) {
	solver := NewUseFlagSolver()
	solver.SetGlobalUseFlag("ssl", true)

	p := pkg.NewPackage("test/pkg", "1.0", "0")
	p.UseFlags["ssl"] = false
	p.UseFlags["mysql"] = false

	enabled := solver.GetEnabledUseFlags(p)

	// Only ssl should be enabled (global)
	foundSSL := false
	for _, flag := range enabled {
		if flag == "ssl" {
			foundSSL = true
		}
		if flag == "mysql" {
			t.Error("mysql should not be in enabled flags")
		}
	}

	if !foundSSL {
		t.Error("Expected ssl in enabled flags")
	}
}

// BenchmarkUseFlagSolver_EvaluateCondition benchmarks condition evaluation
func BenchmarkUseFlagSolver_EvaluateCondition(b *testing.B) {
	solver := NewUseFlagSolver()
	solver.SetGlobalUseFlag("ssl", true)
	solver.SetGlobalUseFlag("mysql", false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = solver.EvaluateUseCondition("test-pkg", "ssl,mysql,-bindist")
	}
}
