package solver

import "github.com/grpmsoft/grpm/internal/pkg"

// Status represents the resolution status
type Status int

const (
	Indet Status = iota // Indeterminate
	Sat                 // Satisfiable - solution found
	Unsat               // Unsatisfiable - dependency conflict
)

// Solver is the interface for dependency resolution
type Solver interface {
	AddPackage(pkg *pkg.Package)
	AddConstraint(constraint pkg.Constraint) error
	Solve() (Status, map[string]string, error)
}
