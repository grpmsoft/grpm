package solver

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestNewSolutionOptimizer(t *testing.T) {
	tests := []struct {
		name     string
		strategy OptimizationStrategy
		expected OptimizationWeights
	}{
		{
			name:     "newest versions strategy",
			strategy: StrategyNewestVersions,
			expected: NewestVersionWeights(),
		},
		{
			name:     "minimal packages strategy",
			strategy: StrategyMinimalPackages,
			expected: MinimalPackageWeights(),
		},
		{
			name:     "balanced strategy",
			strategy: StrategyBalanced,
			expected: DefaultWeights(),
		},
		{
			name:     "stable strategy",
			strategy: StrategyStable,
			expected: StableWeights(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizer := NewSolutionOptimizer(tt.strategy)

			if optimizer == nil {
				t.Fatal("NewSolutionOptimizer returned nil")
			}

			if optimizer.strategy != tt.strategy {
				t.Errorf("Expected strategy %v, got %v", tt.strategy, optimizer.strategy)
			}

			// Verify weights are set correctly
			if optimizer.weights.VersionFreshness != tt.expected.VersionFreshness {
				t.Errorf("VersionFreshness mismatch: expected %v, got %v",
					tt.expected.VersionFreshness, optimizer.weights.VersionFreshness)
			}
		})
	}
}

func TestOptimizationStrategy_String(t *testing.T) {
	tests := []struct {
		strategy OptimizationStrategy
		expected string
	}{
		{StrategyNewestVersions, "newest-versions"},
		{StrategyMinimalPackages, "minimal-packages"},
		{StrategyBalanced, "balanced"},
		{StrategyStable, "stable"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.strategy.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSolutionOptimizer_AnalyzeSolution(t *testing.T) {
	g := NewDependencyGraph()

	// Build simple graph: root -> dep1 -> dep2
	root := pkg.NewPackage("root", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	dep2 := pkg.NewPackage("dep2", "1.0", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep1, false)
	g.AddPackage(dep2, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
	_ = g.AddDependency("root", "dep1", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("dep1", "dep2", constraint, EdgeTypeRuntime)

	g.CalculateDepths()

	solution := &Solution{
		Packages: []*pkg.Package{root, dep1, dep2},
		Graph:    g,
	}

	optimizer := NewSolutionOptimizer(StrategyBalanced)
	optimizer.AnalyzeSolution(solution)

	// Check metadata
	if solution.Metadata.TotalPackages != 3 {
		t.Errorf("Expected 3 total packages, got %d", solution.Metadata.TotalPackages)
	}

	if solution.Metadata.RootPackages != 1 {
		t.Errorf("Expected 1 root package, got %d", solution.Metadata.RootPackages)
	}

	if solution.Metadata.Dependencies != 2 {
		t.Errorf("Expected 2 dependencies, got %d", solution.Metadata.Dependencies)
	}

	if solution.Metadata.MaxDepth != 2 {
		t.Errorf("Expected max depth 2, got %d", solution.Metadata.MaxDepth)
	}

	if solution.Metadata.HasCycles {
		t.Error("Solution should not have cycles")
	}

	if solution.Metadata.HasConflicts {
		t.Error("Solution should not have conflicts")
	}

	// Check that score was computed
	if solution.Score == 0 {
		t.Error("Score should have been computed")
	}
}

func TestSolutionOptimizer_ComputeScore(t *testing.T) {
	optimizer := NewSolutionOptimizer(StrategyBalanced)

	tests := []struct {
		name     string
		metadata SolutionMetadata
		minScore float64 // Minimum expected score
		maxScore float64 // Maximum expected score
	}{
		{
			name: "perfect solution",
			metadata: SolutionMetadata{
				TotalPackages:    3,
				RootPackages:     1,
				Dependencies:     2,
				MaxDepth:         2,
				AverageDepth:     1.0,
				HasCycles:        false,
				HasConflicts:     false,
				ConflictCount:    0,
				NewestVersions:   3,
				OutdatedVersions: 0,
			},
			minScore: 90.0,
			maxScore: 105.0,
		},
		{
			name: "solution with conflicts",
			metadata: SolutionMetadata{
				TotalPackages: 5,
				HasConflicts:  true,
				ConflictCount: 2,
			},
			minScore: 0.0,
			maxScore: 60.0, // Should be heavily penalized
		},
		{
			name: "solution with cycles",
			metadata: SolutionMetadata{
				TotalPackages: 4,
				HasCycles:     true,
			},
			minScore: 50.0,
			maxScore: 85.0,
		},
		{
			name: "deep dependency tree",
			metadata: SolutionMetadata{
				TotalPackages:  20,
				MaxDepth:       15,
				NewestVersions: 20,
			},
			minScore: 85.0,  // High score due to newest versions bonus
			maxScore: 105.0, // Even with depth penalty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			solution := &Solution{
				Metadata: tt.metadata,
			}

			score := optimizer.computeScore(solution)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Score %v outside expected range [%v, %v]",
					score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestSolutionOptimizer_RankSolutions(t *testing.T) {
	optimizer := NewSolutionOptimizer(StrategyBalanced)

	// Create three solutions with different qualities
	g1 := NewDependencyGraph()
	root1 := pkg.NewPackage("pkg-a", "1.0", "0")
	g1.AddPackage(root1, true)

	g2 := NewDependencyGraph()
	root2 := pkg.NewPackage("pkg-b", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	g2.AddPackage(root2, true)
	g2.AddPackage(dep1, false)
	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
	_ = g2.AddDependency("pkg-b", "dep1", constraint, EdgeTypeRuntime)

	g3 := NewDependencyGraph()
	root3 := pkg.NewPackage("pkg-c", "1.0", "0")
	dep2 := pkg.NewPackage("dep2", "1.0", "0")
	g3.AddPackage(root3, true)
	g3.AddPackage(dep2, false)
	_ = g3.AddDependency("pkg-c", "dep2", constraint, EdgeTypeRuntime)
	// Add cycle to make it worse
	_ = g3.AddDependency("dep2", "pkg-c", constraint, EdgeTypeRuntime)

	solutions := []*Solution{
		{Packages: []*pkg.Package{root3, dep2}, Graph: g3}, // Worst (has cycle)
		{Packages: []*pkg.Package{root1}, Graph: g1},       // Best (minimal)
		{Packages: []*pkg.Package{root2, dep1}, Graph: g2}, // Middle
	}

	ranked := optimizer.RankSolutions(solutions)

	if len(ranked) != 3 {
		t.Errorf("Expected 3 ranked solutions, got %d", len(ranked))
	}

	// Verify ranking order (best first)
	if ranked[0].Score < ranked[1].Score {
		t.Error("First solution should have highest score")
	}

	if ranked[1].Score < ranked[2].Score {
		t.Error("Second solution should have higher score than third")
	}

	// Verify the one with cycle is ranked worst
	if !ranked[2].Metadata.HasCycles {
		t.Error("Worst ranked solution should be the one with cycle")
	}
}

func TestSolutionOptimizer_GetBestSolution(t *testing.T) {
	optimizer := NewSolutionOptimizer(StrategyBalanced)

	g1 := NewDependencyGraph()
	root1 := pkg.NewPackage("pkg-a", "1.0", "0")
	g1.AddPackage(root1, true)

	g2 := NewDependencyGraph()
	root2 := pkg.NewPackage("pkg-b", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	g2.AddPackage(root2, true)
	g2.AddPackage(dep1, false)

	solutions := []*Solution{
		{Packages: []*pkg.Package{root2, dep1}, Graph: g2},
		{Packages: []*pkg.Package{root1}, Graph: g1}, // Should be best
	}

	best := optimizer.GetBestSolution(solutions)

	if best == nil {
		t.Fatal("GetBestSolution returned nil")
	}

	// Best should be the one with fewer packages (for balanced strategy)
	if len(best.Packages) != 1 {
		t.Errorf("Best solution should have 1 package, got %d", len(best.Packages))
	}
}

func TestFilterValidSolutions(t *testing.T) {
	// Create solutions with different issues
	goodSolution := &Solution{
		Metadata: SolutionMetadata{
			HasConflicts: false,
			HasCycles:    false,
		},
	}

	conflictSolution := &Solution{
		Metadata: SolutionMetadata{
			HasConflicts: true,
			HasCycles:    false,
		},
	}

	cycleSolution := &Solution{
		Metadata: SolutionMetadata{
			HasConflicts: false,
			HasCycles:    true,
		},
	}

	badSolution := &Solution{
		Metadata: SolutionMetadata{
			HasConflicts: true,
			HasCycles:    true,
		},
	}

	solutions := []*Solution{goodSolution, conflictSolution, cycleSolution, badSolution}

	valid := FilterValidSolutions(solutions)

	if len(valid) != 1 {
		t.Errorf("Expected 1 valid solution, got %d", len(valid))
	}

	if valid[0] != goodSolution {
		t.Error("Valid solution should be the one without conflicts or cycles")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool // true if v1 > v2
	}{
		{
			name:     "newer version",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "older version",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: false,
		},
		{
			name:     "equal versions",
			v1:       "1.5.0",
			v2:       "1.5.0",
			expected: false,
		},
		{
			name:     "patch version",
			v1:       "1.2.3",
			v2:       "1.2.2",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSelectNewestVersion(t *testing.T) {
	packages := []*pkg.Package{
		pkg.NewPackage("pkg", "1.0.0", "0"),
		pkg.NewPackage("pkg", "2.5.0", "0"),
		pkg.NewPackage("pkg", "1.8.0", "0"),
		pkg.NewPackage("pkg", "2.0.0", "0"),
	}

	newest := SelectNewestVersion(packages)

	if newest == nil {
		t.Fatal("SelectNewestVersion returned nil")
	}

	if newest.Version != "2.5.0" {
		t.Errorf("Expected version 2.5.0, got %s", newest.Version)
	}
}

func TestSelectNewestVersion_EmptyList(t *testing.T) {
	packages := []*pkg.Package{}

	newest := SelectNewestVersion(packages)

	if newest != nil {
		t.Error("SelectNewestVersion should return nil for empty list")
	}
}

func TestPruneTransitiveDeps(t *testing.T) {
	// Build graph with some unreachable nodes
	g := NewDependencyGraph()

	// Reachable chain: root -> dep1 -> dep2
	root := pkg.NewPackage("root", "1.0", "0")
	dep1 := pkg.NewPackage("dep1", "1.0", "0")
	dep2 := pkg.NewPackage("dep2", "1.0", "0")

	// Unreachable nodes
	orphan1 := pkg.NewPackage("orphan1", "1.0", "0")
	orphan2 := pkg.NewPackage("orphan2", "1.0", "0")

	g.AddPackage(root, true)
	g.AddPackage(dep1, false)
	g.AddPackage(dep2, false)
	g.AddPackage(orphan1, false)
	g.AddPackage(orphan2, false)

	constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}

	_ = g.AddDependency("root", "dep1", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("dep1", "dep2", constraint, EdgeTypeRuntime)
	// orphan1 and orphan2 have no connections to root

	if g.PackageCount() != 5 {
		t.Errorf("Original graph should have 5 packages, got %d", g.PackageCount())
	}

	pruned := PruneTransitiveDeps(g)

	if pruned.PackageCount() != 3 {
		t.Errorf("Pruned graph should have 3 packages, got %d", pruned.PackageCount())
	}

	// Check that reachable packages are present
	if _, ok := pruned.GetNode("root"); !ok {
		t.Error("Pruned graph should contain root")
	}

	if _, ok := pruned.GetNode("dep1"); !ok {
		t.Error("Pruned graph should contain dep1")
	}

	if _, ok := pruned.GetNode("dep2"); !ok {
		t.Error("Pruned graph should contain dep2")
	}

	// Check that orphans are removed
	if _, ok := pruned.GetNode("orphan1"); ok {
		t.Error("Pruned graph should not contain orphan1")
	}

	if _, ok := pruned.GetNode("orphan2"); ok {
		t.Error("Pruned graph should not contain orphan2")
	}
}

func TestPruneTransitiveDeps_DiamondGraph(t *testing.T) {
	// Build diamond graph (all nodes reachable)
	//     root
	//    /    \
	//  dep1  dep2
	//    \    /
	//     dep3

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

	_ = g.AddDependency("root", "dep1", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("root", "dep2", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("dep1", "dep3", constraint, EdgeTypeRuntime)
	_ = g.AddDependency("dep2", "dep3", constraint, EdgeTypeRuntime)

	pruned := PruneTransitiveDeps(g)

	// All nodes should remain (all are reachable)
	if pruned.PackageCount() != 4 {
		t.Errorf("Pruned graph should have 4 packages, got %d", pruned.PackageCount())
	}

	// Check all nodes are present
	if _, ok := pruned.GetNode("root"); !ok {
		t.Error("Pruned graph should contain root")
	}
	if _, ok := pruned.GetNode("dep1"); !ok {
		t.Error("Pruned graph should contain dep1")
	}
	if _, ok := pruned.GetNode("dep2"); !ok {
		t.Error("Pruned graph should contain dep2")
	}
	if _, ok := pruned.GetNode("dep3"); !ok {
		t.Error("Pruned graph should contain dep3")
	}
}

func BenchmarkAnalyzeSolution(b *testing.B) {
	g := NewDependencyGraph()

	// Build large graph
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

	g.CalculateDepths()

	solution := &Solution{
		Graph: g,
	}

	optimizer := NewSolutionOptimizer(StrategyBalanced)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizer.AnalyzeSolution(solution)
	}
}

func BenchmarkRankSolutions(b *testing.B) {
	optimizer := NewSolutionOptimizer(StrategyBalanced)

	// Create 10 solutions
	solutions := make([]*Solution, 10)
	for i := 0; i < 10; i++ {
		g := NewDependencyGraph()
		root := pkg.NewPackage("root-"+string(rune(i)), "1.0", "0")
		g.AddPackage(root, true)

		// Add varying number of deps
		constraint := pkg.Constraint{Type: pkg.ConstraintTypeVersion}
		for j := 0; j < i*5; j++ {
			dep := pkg.NewPackage("dep-"+string(rune(j)), "1.0", "0")
			g.AddPackage(dep, false)
			_ = g.AddDependency("root-"+string(rune(i)), "dep-"+string(rune(j)), constraint, EdgeTypeRuntime)
		}

		solutions[i] = &Solution{Graph: g}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = optimizer.RankSolutions(solutions)
	}
}
