package solver

import (
	"fmt"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// DependencyGraph represents the complete dependency graph for package resolution
type DependencyGraph struct {
	nodes map[string]*GraphNode // Package name -> Node
	edges []*GraphEdge          // All dependency edges
	roots []string              // Root packages (user-requested)
}

// GraphNode represents a package in the dependency graph
type GraphNode struct {
	Package      *pkg.Package // Package metadata
	Dependencies []*GraphEdge // Outgoing edges (this package depends on...)
	Dependents   []*GraphEdge // Incoming edges (...packages that depend on this)
	Depth        int          // Distance from root packages (BFS depth)
	Visited      bool         // For graph traversal algorithms
	InStack      bool         // For cycle detection (DFS stack)
	State        NodeState    // Processing state
}

// GraphEdge represents a dependency relationship
type GraphEdge struct {
	From       string         // Source package name
	To         string         // Target package name
	Constraint pkg.Constraint // Version/slot/USE constraint
	Type       EdgeType       // Dependency type
}

// NodeState represents the processing state of a node
type NodeState int

const (
	NodeStateUnvisited NodeState = iota // Not yet processed
	NodeStateVisiting                   // Currently being processed (DFS)
	NodeStateVisited                    // Fully processed
)

// EdgeType categorizes dependency edges
type EdgeType int

const (
	EdgeTypeRuntime   EdgeType = iota // RDEPEND
	EdgeTypeBuildtime                 // DEPEND
	EdgeTypeBuild                     // BDEPEND
)

// NewDependencyGraph creates a new empty dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*GraphNode),
		edges: make([]*GraphEdge, 0),
		roots: make([]string, 0),
	}
}

// AddPackage adds a package node to the graph
func (g *DependencyGraph) AddPackage(p *pkg.Package, isRoot bool) {
	if _, exists := g.nodes[p.Name]; exists {
		return // Already added
	}

	node := &GraphNode{
		Package:      p,
		Dependencies: make([]*GraphEdge, 0),
		Dependents:   make([]*GraphEdge, 0),
		Depth:        -1, // Will be calculated later
		State:        NodeStateUnvisited,
	}

	g.nodes[p.Name] = node

	if isRoot {
		g.roots = append(g.roots, p.Name)
		node.Depth = 0
	}
}

// AddDependency adds a dependency edge between two packages
func (g *DependencyGraph) AddDependency(from, to string, constraint pkg.Constraint, edgeType EdgeType) error {
	// Validate nodes exist
	fromNode, ok := g.nodes[from]
	if !ok {
		return fmt.Errorf("source package not found: %s", from)
	}

	toNode, ok := g.nodes[to]
	if !ok {
		return fmt.Errorf("target package not found: %s", to)
	}

	// Create edge
	edge := &GraphEdge{
		From:       from,
		To:         to,
		Constraint: constraint,
		Type:       edgeType,
	}

	// Add to graph structures
	g.edges = append(g.edges, edge)
	fromNode.Dependencies = append(fromNode.Dependencies, edge)
	toNode.Dependents = append(toNode.Dependents, edge)

	return nil
}

// GetNode retrieves a node by package name
func (g *DependencyGraph) GetNode(name string) (*GraphNode, bool) {
	node, ok := g.nodes[name]
	return node, ok
}

// GetRoots returns the root packages
func (g *DependencyGraph) GetRoots() []string {
	return g.roots
}

// PackageCount returns the total number of packages in the graph
func (g *DependencyGraph) PackageCount() int {
	return len(g.nodes)
}

// EdgeCount returns the total number of dependency edges
func (g *DependencyGraph) EdgeCount() int {
	return len(g.edges)
}

// CalculateDepths performs BFS to calculate depths from root nodes
func (g *DependencyGraph) CalculateDepths() {
	// Reset depths
	for _, node := range g.nodes {
		if node.Depth != 0 { // Don't reset roots
			node.Depth = -1
		}
	}

	// BFS from all roots
	queue := make([]string, 0, len(g.roots))
	queue = append(queue, g.roots...)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		node := g.nodes[current]

		// Process dependencies
		for _, edge := range node.Dependencies {
			depNode := g.nodes[edge.To]

			// Update depth if this is a shorter path
			newDepth := node.Depth + 1
			if depNode.Depth == -1 || newDepth < depNode.Depth {
				depNode.Depth = newDepth
				queue = append(queue, edge.To)
			}
		}
	}
}

// GetPackagesByDepth returns packages grouped by depth level
func (g *DependencyGraph) GetPackagesByDepth() map[int][]string {
	result := make(map[int][]string)

	for name, node := range g.nodes {
		if node.Depth >= 0 {
			result[node.Depth] = append(result[node.Depth], name)
		}
	}

	return result
}

// ResetVisited resets visited flags for all nodes (for graph algorithms)
func (g *DependencyGraph) ResetVisited() {
	for _, node := range g.nodes {
		node.Visited = false
		node.InStack = false
		node.State = NodeStateUnvisited
	}
}

// String returns a human-readable representation of the graph
func (g *DependencyGraph) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Dependency Graph: %d packages, %d edges\n", len(g.nodes), len(g.edges)))
	sb.WriteString(fmt.Sprintf("Root packages: %v\n", g.roots))

	// Group by depth
	byDepth := g.GetPackagesByDepth()
	for depth := 0; depth <= 10; depth++ { // Max depth 10 for display
		if packages, ok := byDepth[depth]; ok {
			sb.WriteString(fmt.Sprintf("\nDepth %d: %d packages\n", depth, len(packages)))
			for _, name := range packages {
				node := g.nodes[name]
				sb.WriteString(fmt.Sprintf("  - %s-%s (deps: %d, dependents: %d)\n",
					name, node.Package.Version, len(node.Dependencies), len(node.Dependents)))
			}
		}
	}

	return sb.String()
}

// ToDOT exports the graph in Graphviz DOT format for visualization
func (g *DependencyGraph) ToDOT() string {
	var sb strings.Builder

	sb.WriteString("digraph DependencyGraph {\n")
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// Nodes
	for name, node := range g.nodes {
		color := "lightgray"
		if node.Depth == 0 {
			color = "lightblue" // Root packages
		}

		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n%s\", style=filled, fillcolor=%s];\n",
			name, name, node.Package.Version, color))
	}

	sb.WriteString("\n")

	// Edges
	for _, edge := range g.edges {
		style := "solid"
		switch edge.Type {
		case EdgeTypeBuildtime:
			style = "dashed"
		case EdgeTypeBuild:
			style = "dotted"
		}

		label := ""
		if edge.Constraint.Version != nil {
			label = edge.Constraint.Version.String()
		}

		sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\", style=%s];\n",
			edge.From, edge.To, label, style))
	}

	sb.WriteString("}\n")

	return sb.String()
}
