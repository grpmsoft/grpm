package solver

import (
	"sort"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// Solution represents a valid package installation solution
type Solution struct {
	Packages []*pkg.Package   // Packages to install
	Graph    *DependencyGraph // Dependency graph
	Score    float64          // Quality score (higher is better)
	Metadata SolutionMetadata // Additional information
}

// SolutionMetadata contains solution analysis information
type SolutionMetadata struct {
	TotalPackages    int     // Total number of packages
	RootPackages     int     // Number of explicitly requested packages
	Dependencies     int     // Number of transitive dependencies
	MaxDepth         int     // Maximum dependency depth
	AverageDepth     float64 // Average dependency depth
	HasCycles        bool    // Whether solution contains cycles
	HasConflicts     bool    // Whether solution contains conflicts
	ConflictCount    int     // Number of conflicts
	NewestVersions   int     // Count of packages using newest available version
	OutdatedVersions int     // Count of packages using older versions
}

// OptimizationStrategy defines how to rank solutions
type OptimizationStrategy int

const (
	// StrategyNewestVersions prefers solutions with newest package versions
	StrategyNewestVersions OptimizationStrategy = iota
	// StrategyMinimalPackages prefers solutions with fewest packages
	StrategyMinimalPackages
	// StrategyBalanced balances version freshness and package count
	StrategyBalanced
	// StrategyStable prefers tested/stable versions over bleeding edge
	StrategyStable
)

// String returns the name of the optimization strategy
func (s OptimizationStrategy) String() string {
	switch s {
	case StrategyNewestVersions:
		return "newest-versions"
	case StrategyMinimalPackages:
		return "minimal-packages"
	case StrategyBalanced:
		return "balanced"
	case StrategyStable:
		return "stable"
	default:
		return "unknown"
	}
}

// SolutionOptimizer finds and ranks solutions
type SolutionOptimizer struct {
	strategy OptimizationStrategy
	weights  OptimizationWeights
}

// OptimizationWeights controls scoring factors
type OptimizationWeights struct {
	VersionFreshness float64 // Weight for using newest versions (0-1)
	PackageCount     float64 // Weight for minimizing packages (0-1)
	DepthPenalty     float64 // Penalty for deep dependency trees (0-1)
	ConflictPenalty  float64 // Penalty for conflicts (0-1)
	CyclePenalty     float64 // Penalty for cycles (0-1)
}

// DefaultWeights returns balanced optimization weights
func DefaultWeights() OptimizationWeights {
	return OptimizationWeights{
		VersionFreshness: 0.4,
		PackageCount:     0.3,
		DepthPenalty:     0.1,
		ConflictPenalty:  0.5,
		CyclePenalty:     0.5,
	}
}

// NewestVersionWeights returns weights favoring newest versions
func NewestVersionWeights() OptimizationWeights {
	return OptimizationWeights{
		VersionFreshness: 0.8,
		PackageCount:     0.1,
		DepthPenalty:     0.05,
		ConflictPenalty:  0.5,
		CyclePenalty:     0.5,
	}
}

// MinimalPackageWeights returns weights favoring fewer packages
func MinimalPackageWeights() OptimizationWeights {
	return OptimizationWeights{
		VersionFreshness: 0.2,
		PackageCount:     0.6,
		DepthPenalty:     0.15,
		ConflictPenalty:  0.5,
		CyclePenalty:     0.5,
	}
}

// StableWeights returns weights favoring stable/tested versions
func StableWeights() OptimizationWeights {
	return OptimizationWeights{
		VersionFreshness: 0.1, // Lower preference for newest
		PackageCount:     0.4,
		DepthPenalty:     0.2,
		ConflictPenalty:  0.5,
		CyclePenalty:     0.5,
	}
}

// NewSolutionOptimizer creates an optimizer with the given strategy
func NewSolutionOptimizer(strategy OptimizationStrategy) *SolutionOptimizer {
	var weights OptimizationWeights

	switch strategy {
	case StrategyNewestVersions:
		weights = NewestVersionWeights()
	case StrategyMinimalPackages:
		weights = MinimalPackageWeights()
	case StrategyBalanced:
		weights = DefaultWeights()
	case StrategyStable:
		weights = StableWeights()
	default:
		weights = DefaultWeights()
	}

	return &SolutionOptimizer{
		strategy: strategy,
		weights:  weights,
	}
}

// NewSolutionOptimizerWithWeights creates an optimizer with custom weights
func NewSolutionOptimizerWithWeights(weights OptimizationWeights) *SolutionOptimizer {
	return &SolutionOptimizer{
		strategy: StrategyBalanced,
		weights:  weights,
	}
}

// AnalyzeSolution computes metadata and score for a solution
func (so *SolutionOptimizer) AnalyzeSolution(solution *Solution) {
	// Compute metadata
	solution.Metadata = so.computeMetadata(solution.Graph)

	// Compute score based on strategy
	solution.Score = so.computeScore(solution)
}

// computeMetadata analyzes the dependency graph
func (so *SolutionOptimizer) computeMetadata(graph *DependencyGraph) SolutionMetadata {
	metadata := SolutionMetadata{
		TotalPackages: graph.PackageCount(),
		RootPackages:  len(graph.GetRoots()),
	}

	metadata.Dependencies = metadata.TotalPackages - metadata.RootPackages

	// Calculate depth statistics
	byDepth := graph.GetPackagesByDepth()
	var totalDepth int
	var packageCount int

	for depth, packages := range byDepth {
		if depth > metadata.MaxDepth {
			metadata.MaxDepth = depth
		}
		totalDepth += depth * len(packages)
		packageCount += len(packages)
	}

	if packageCount > 0 {
		metadata.AverageDepth = float64(totalDepth) / float64(packageCount)
	}

	// Check for cycles
	metadata.HasCycles = graph.HasCycles()

	// Check for conflicts
	conflicts := graph.DetectConflicts()
	metadata.HasConflicts = len(conflicts) > 0
	metadata.ConflictCount = len(conflicts)

	// TODO: Compute version freshness (requires version comparison with available versions)
	// For now, assume all versions are relatively fresh
	metadata.NewestVersions = metadata.TotalPackages
	metadata.OutdatedVersions = 0

	return metadata
}

// computeScore calculates solution quality score
func (so *SolutionOptimizer) computeScore(solution *Solution) float64 {
	meta := solution.Metadata
	score := 100.0 // Start with perfect score

	// Penalty for conflicts (critical)
	if meta.HasConflicts {
		score -= so.weights.ConflictPenalty * 50.0 * float64(meta.ConflictCount)
	}

	// Penalty for cycles (critical)
	if meta.HasCycles {
		score -= so.weights.CyclePenalty * 30.0
	}

	// Penalty for package count (prefer minimal installations)
	if meta.TotalPackages > 0 {
		// Normalize: 1-10 packages = small penalty, 100+ = large penalty
		packagePenalty := float64(meta.TotalPackages) / 10.0
		if packagePenalty > 10.0 {
			packagePenalty = 10.0
		}
		score -= so.weights.PackageCount * packagePenalty
	}

	// Penalty for deep dependency trees
	if meta.MaxDepth > 5 {
		depthPenalty := float64(meta.MaxDepth-5) * 2.0
		score -= so.weights.DepthPenalty * depthPenalty
	}

	// Bonus for version freshness
	if meta.TotalPackages > 0 {
		freshnessRatio := float64(meta.NewestVersions) / float64(meta.TotalPackages)
		bonus := freshnessRatio * 10.0
		score += so.weights.VersionFreshness * bonus
	}

	// Ensure score doesn't go negative
	if score < 0 {
		score = 0
	}

	return score
}

// RankSolutions sorts solutions by score (best first)
func (so *SolutionOptimizer) RankSolutions(solutions []*Solution) []*Solution {
	// Analyze all solutions first
	for _, solution := range solutions {
		if solution.Score == 0 {
			so.AnalyzeSolution(solution)
		}
	}

	// Sort by score (descending)
	ranked := make([]*Solution, len(solutions))
	copy(ranked, solutions)

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

// GetBestSolution returns the highest-scored solution
func (so *SolutionOptimizer) GetBestSolution(solutions []*Solution) *Solution {
	if len(solutions) == 0 {
		return nil
	}

	ranked := so.RankSolutions(solutions)
	return ranked[0]
}

// FilterValidSolutions returns only solutions without critical issues
func FilterValidSolutions(solutions []*Solution) []*Solution {
	valid := make([]*Solution, 0)

	for _, solution := range solutions {
		// Skip solutions with conflicts or cycles
		if solution.Metadata.HasConflicts || solution.Metadata.HasCycles {
			continue
		}

		valid = append(valid, solution)
	}

	return valid
}

// CompareVersions compares two package versions
// Returns true if v1 is newer than v2
func CompareVersions(v1, v2 string) bool {
	return pkg.CompareVersions(v1, v2) > 0
}

// SelectNewestVersion chooses the newest version from a list of packages
func SelectNewestVersion(packages []*pkg.Package) *pkg.Package {
	if len(packages) == 0 {
		return nil
	}

	newest := packages[0]
	for i := 1; i < len(packages); i++ {
		if CompareVersions(packages[i].Version, newest.Version) {
			newest = packages[i]
		}
	}

	return newest
}

// PruneTransitiveDeps removes unnecessary transitive dependencies
// Keeps only packages that are directly or indirectly required
func PruneTransitiveDeps(graph *DependencyGraph) *DependencyGraph {
	// Create new graph with only reachable packages from roots
	pruned := NewDependencyGraph()

	// BFS from roots to find all reachable packages
	visited := make(map[string]bool)
	queue := make([]string, 0)

	for _, root := range graph.GetRoots() {
		queue = append(queue, root)
		visited[root] = true
	}

	// Collect reachable nodes
	reachable := make(map[string]*GraphNode)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		node, ok := graph.GetNode(current)
		if !ok {
			continue
		}

		reachable[current] = node

		// Add dependencies to queue
		for _, edge := range node.Dependencies {
			if !visited[edge.To] {
				visited[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}

	// Build pruned graph with reachable nodes
	for name, node := range reachable {
		isRoot := false
		for _, root := range graph.GetRoots() {
			if root == name {
				isRoot = true
				break
			}
		}
		pruned.AddPackage(node.Package, isRoot)
	}

	// Add edges for reachable nodes
	for name, node := range reachable {
		for _, edge := range node.Dependencies {
			if _, ok := reachable[edge.To]; ok {
				_ = pruned.AddDependency(name, edge.To, edge.Constraint, edge.Type)
			}
		}
	}

	return pruned
}
