package solver

import (
	"fmt"
	"strings"
)

// CycleError represents a circular dependency error with detailed path information
type CycleError struct {
	Cycle []string // Package names forming the cycle
}

// Error implements the error interface
func (e *CycleError) Error() string {
	if len(e.Cycle) == 0 {
		return "circular dependency detected"
	}

	// Build human-readable cycle path: A -> B -> C -> A
	var sb strings.Builder
	sb.WriteString("circular dependency detected: ")

	for i, pkg := range e.Cycle {
		if i > 0 {
			sb.WriteString(" -> ")
		}
		sb.WriteString(pkg)
	}

	// Close the cycle by repeating the first package
	if len(e.Cycle) > 0 {
		sb.WriteString(" -> ")
		sb.WriteString(e.Cycle[0])
	}

	return sb.String()
}

// DetectCycles checks the dependency graph for circular dependencies using DFS
// Returns a CycleError if a cycle is found, nil otherwise
func (g *DependencyGraph) DetectCycles() error {
	// Reset traversal state
	g.ResetVisited()

	// DFS from all roots (covers disconnected components)
	for _, root := range g.roots {
		if err := g.dfsDetectCycle(root, []string{}); err != nil {
			return err
		}
	}

	// Check any unvisited nodes (orphaned subgraphs)
	for name, node := range g.nodes {
		if !node.Visited {
			if err := g.dfsDetectCycle(name, []string{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// dfsDetectCycle performs DFS traversal with cycle detection
// path tracks the current DFS path for cycle reconstruction
func (g *DependencyGraph) dfsDetectCycle(nodeName string, path []string) error {
	node, exists := g.nodes[nodeName]
	if !exists {
		return nil // Shouldn't happen, but handle gracefully
	}

	// If already fully processed, skip
	if node.State == NodeStateVisited {
		return nil
	}

	// If currently being processed, we found a back edge (cycle)
	if node.State == NodeStateVisiting {
		// Extract cycle from path
		cycle := extractCycle(path, nodeName)
		return &CycleError{Cycle: cycle}
	}

	// Mark as being processed
	node.State = NodeStateVisiting
	node.Visited = true
	node.InStack = true

	// Add current node to path
	currentPath := append(path, nodeName)

	// Recursively visit all dependencies
	for _, edge := range node.Dependencies {
		if err := g.dfsDetectCycle(edge.To, currentPath); err != nil {
			return err
		}
	}

	// Mark as fully processed
	node.State = NodeStateVisited
	node.InStack = false

	return nil
}

// extractCycle extracts the cycle path from the DFS path
// Returns the sequence of packages forming the cycle
func extractCycle(path []string, cycleStart string) []string {
	// Find where the cycle starts in the path
	startIdx := -1
	for i, pkg := range path {
		if pkg == cycleStart {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		// Cycle start not found in path (shouldn't happen)
		// Return just the cycle start
		return []string{cycleStart}
	}

	// Return the portion of the path forming the cycle
	return path[startIdx:]
}

// HasCycles is a convenience method that returns true if cycles exist
func (g *DependencyGraph) HasCycles() bool {
	return g.DetectCycles() != nil
}

// GetAllCycles attempts to find all cycles in the graph (advanced analysis)
// This is more expensive than DetectCycles() but provides complete information
func (g *DependencyGraph) GetAllCycles() []*CycleError {
	cycles := make([]*CycleError, 0)

	// Reset state
	g.ResetVisited()

	// Try DFS from every node to find all cycles
	for name := range g.nodes {
		if err := g.dfsCollectCycles(name, []string{}, &cycles); err != nil {
			// Continue searching even if we found a cycle
			continue
		}
	}

	// Deduplicate cycles (same cycle can be found from different starting points)
	return deduplicateCycles(cycles)
}

// dfsCollectCycles collects all cycles during DFS traversal
func (g *DependencyGraph) dfsCollectCycles(nodeName string, path []string, cycles *[]*CycleError) error {
	node, exists := g.nodes[nodeName]
	if !exists {
		return nil
	}

	if node.State == NodeStateVisited {
		return nil
	}

	if node.State == NodeStateVisiting {
		cycle := extractCycle(path, nodeName)
		*cycles = append(*cycles, &CycleError{Cycle: cycle})
		return fmt.Errorf("cycle found") // Signal to stop this path
	}

	node.State = NodeStateVisiting
	node.InStack = true

	currentPath := append(path, nodeName)

	for _, edge := range node.Dependencies {
		// Ignore errors, keep searching
		_ = g.dfsCollectCycles(edge.To, currentPath, cycles)
	}

	node.State = NodeStateVisited
	node.InStack = false

	return nil
}

// deduplicateCycles removes duplicate cycle representations
func deduplicateCycles(cycles []*CycleError) []*CycleError {
	seen := make(map[string]bool)
	unique := make([]*CycleError, 0)

	for _, cycle := range cycles {
		// Normalize cycle representation (smallest package name first)
		normalized := normalizeCycle(cycle.Cycle)
		key := strings.Join(normalized, "->")

		if !seen[key] {
			seen[key] = true
			unique = append(unique, &CycleError{Cycle: normalized})
		}
	}

	return unique
}

// normalizeCycle creates a canonical representation of a cycle
// (starts from lexicographically smallest package)
func normalizeCycle(cycle []string) []string {
	if len(cycle) == 0 {
		return cycle
	}

	// Find index of smallest package name
	minIdx := 0
	for i := 1; i < len(cycle); i++ {
		if cycle[i] < cycle[minIdx] {
			minIdx = i
		}
	}

	// Rotate cycle to start from smallest package
	normalized := make([]string, len(cycle))
	for i := 0; i < len(cycle); i++ {
		normalized[i] = cycle[(minIdx+i)%len(cycle)]
	}

	return normalized
}
