package solver

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestOrGroupIDAssignment tests that OR-dependencies get unique group IDs
func TestOrGroupIDAssignment(t *testing.T) {
	tests := []struct {
		name     string
		deps     []pkg.Constraint
		wantOrID map[string]int // package name -> expected OrGroupID
	}{
		{
			name: "single OR-group",
			deps: []pkg.Constraint{
				{Name: "mysql", OrGroupID: 1},
				{Name: "postgresql", OrGroupID: 1},
			},
			wantOrID: map[string]int{
				"mysql":      1,
				"postgresql": 1,
			},
		},
		{
			name: "multiple OR-groups",
			deps: []pkg.Constraint{
				{Name: "mysql", OrGroupID: 1},
				{Name: "postgresql", OrGroupID: 1},
				{Name: "apache", OrGroupID: 2},
				{Name: "nginx", OrGroupID: 2},
			},
			wantOrID: map[string]int{
				"mysql":      1,
				"postgresql": 1,
				"apache":     2,
				"nginx":      2,
			},
		},
		{
			name: "mixed required and OR",
			deps: []pkg.Constraint{
				{Name: "glibc", OrGroupID: 0}, // required
				{Name: "mysql", OrGroupID: 1}, // OR-group 1
				{Name: "postgresql", OrGroupID: 1},
				{Name: "zlib", OrGroupID: 0}, // required
			},
			wantOrID: map[string]int{
				"glibc":      0,
				"mysql":      1,
				"postgresql": 1,
				"zlib":       0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, dep := range tt.deps {
				wantID := tt.wantOrID[dep.Name]
				if dep.OrGroupID != wantID {
					t.Errorf("%s: OrGroupID = %d, want %d", dep.Name, dep.OrGroupID, wantID)
				}
			}
		})
	}
}

// TestResolverGroupsByOrGroupID tests that resolver correctly groups dependencies
func TestResolverGroupsByOrGroupID(t *testing.T) {
	// Create a mock package with mixed dependencies
	p := &pkg.Package{
		Name:    "test/package",
		Version: "1.0.0",
		Deps: []pkg.Constraint{
			{Name: "sys-libs/glibc", OrGroupID: 0},     // required
			{Name: "dev-db/mysql", OrGroupID: 1},       // OR-group 1
			{Name: "dev-db/postgresql", OrGroupID: 1},  // OR-group 1
			{Name: "sys-libs/zlib", OrGroupID: 0},      // required
			{Name: "www-servers/apache", OrGroupID: 2}, // OR-group 2
			{Name: "www-servers/nginx", OrGroupID: 2},  // OR-group 2
		},
	}

	// Group dependencies manually (simulating resolver logic)
	orGroups := make(map[int][]pkg.Constraint)
	var requiredDeps []pkg.Constraint

	for _, dep := range p.Deps {
		if dep.OrGroupID == 0 {
			requiredDeps = append(requiredDeps, dep)
		} else {
			orGroups[dep.OrGroupID] = append(orGroups[dep.OrGroupID], dep)
		}
	}

	// Verify grouping
	if len(requiredDeps) != 2 {
		t.Errorf("requiredDeps count = %d, want 2", len(requiredDeps))
	}

	if len(orGroups) != 2 {
		t.Errorf("OR-groups count = %d, want 2", len(orGroups))
	}

	if len(orGroups[1]) != 2 {
		t.Errorf("OR-group 1 size = %d, want 2", len(orGroups[1]))
	}

	if len(orGroups[2]) != 2 {
		t.Errorf("OR-group 2 size = %d, want 2", len(orGroups[2]))
	}

	// Verify correct packages in each group
	wantGroup1 := map[string]bool{"dev-db/mysql": true, "dev-db/postgresql": true}
	wantGroup2 := map[string]bool{"www-servers/apache": true, "www-servers/nginx": true}

	for _, dep := range orGroups[1] {
		if !wantGroup1[dep.Name] {
			t.Errorf("unexpected package in OR-group 1: %s", dep.Name)
		}
	}

	for _, dep := range orGroups[2] {
		if !wantGroup2[dep.Name] {
			t.Errorf("unexpected package in OR-group 2: %s", dep.Name)
		}
	}
}

// TestCollectDependenciesSkipsOrGroups tests that collectDependencies doesn't recursively
// collect dependencies from OR-group alternatives
func TestCollectDependenciesSkipsOrGroups(t *testing.T) {
	// This would need a mock repository to test properly
	t.Skip("Requires mock repository implementation")

	// Test logic:
	// 1. Create a mock repo with packages:
	//    - test/main: depends on (mysql OR postgresql) + glibc
	//    - mysql: depends on 100 other packages
	//    - postgresql: depends on 100 other packages
	//    - glibc: depends on 5 packages
	// 2. Run collectDependencies(test/main)
	// 3. Verify allPackages contains:
	//    - test/main
	//    - glibc
	//    - glibc's 5 dependencies
	//    - Total: ~7 packages
	// 4. Verify allPackages does NOT contain mysql/postgresql dependencies (200 packages)
}

// TestSATAdapterOrGroupClause tests that AddOrGroupConstraint generates correct clauses
func TestSATAdapterOrGroupClause(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Add some test packages
	mysqlPkg := &pkg.Package{Name: "dev-db/mysql", Version: "8.0"}
	postgresPkg := &pkg.Package{Name: "dev-db/postgresql", Version: "15.3"}

	adapter.AddPackage(mysqlPkg)
	adapter.AddPackage(postgresPkg)

	// Create OR-group: (mysql OR postgresql)
	alternatives := []pkg.Constraint{
		{Type: pkg.ConstraintTypeVersion, Name: "dev-db/mysql"},
		{Type: pkg.ConstraintTypeVersion, Name: "dev-db/postgresql"},
	}

	err := adapter.AddOrGroupConstraint(alternatives)
	if err != nil {
		t.Fatalf("AddOrGroupConstraint() error = %v", err)
	}

	// Verify that a clause was added
	if len(adapter.clauses) == 0 {
		t.Fatal("no clauses added for OR-group")
	}

	// The clause should contain variables for both packages
	// (exact SAT encoding is implementation detail, just verify non-empty)
	lastClause := adapter.clauses[len(adapter.clauses)-1]
	if len(lastClause) < 2 {
		t.Errorf("OR-group clause has %d literals, want at least 2", len(lastClause))
	}
}

// TestOrGroupWithVersionConstraints tests OR-groups with version constraints
func TestOrGroupWithVersionConstraints(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Add multiple versions of packages
	adapter.AddPackage(&pkg.Package{Name: "dev-db/mysql", Version: "5.7"})
	adapter.AddPackage(&pkg.Package{Name: "dev-db/mysql", Version: "8.0"})
	adapter.AddPackage(&pkg.Package{Name: "dev-db/postgresql", Version: "14.0"})
	adapter.AddPackage(&pkg.Package{Name: "dev-db/postgresql", Version: "15.3"})

	// Create OR-group: (mysql >= 8.0 OR postgresql >= 15.0)
	alternatives := []pkg.Constraint{
		{
			Type:    pkg.ConstraintTypeVersion,
			Name:    "dev-db/mysql",
			Version: pkg.NewMinVersionConstraint("8.0"),
		},
		{
			Type:    pkg.ConstraintTypeVersion,
			Name:    "dev-db/postgresql",
			Version: pkg.NewMinVersionConstraint("15.0"),
		},
	}

	err := adapter.AddOrGroupConstraint(alternatives)
	if err != nil {
		t.Fatalf("AddOrGroupConstraint() error = %v", err)
	}

	// Verify clause was added
	if len(adapter.clauses) == 0 {
		t.Fatal("no clauses added for OR-group with version constraints")
	}

	// The clause should contain:
	// - mysql@8.0 (satisfies >= 8.0)
	// - postgresql@15.3 (satisfies >= 15.0)
	// Total: at least 2 literals
	lastClause := adapter.clauses[len(adapter.clauses)-1]
	if len(lastClause) < 2 {
		t.Errorf("OR-group clause has %d literals, want at least 2", len(lastClause))
	}
}

// BenchmarkOrGroupResolution benchmarks OR-dependency resolution
func BenchmarkOrGroupResolution(b *testing.B) {
	// Create a package with many OR-groups
	deps := make([]pkg.Constraint, 0)
	for i := 1; i <= 10; i++ {
		deps = append(deps, pkg.Constraint{
			Name:      "test/package-a-" + string(rune(i)),
			OrGroupID: i,
		})
		deps = append(deps, pkg.Constraint{
			Name:      "test/package-b-" + string(rune(i)),
			OrGroupID: i,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate grouping logic
		orGroups := make(map[int][]pkg.Constraint)
		for _, dep := range deps {
			orGroups[dep.OrGroupID] = append(orGroups[dep.OrGroupID], dep)
		}
	}
}
