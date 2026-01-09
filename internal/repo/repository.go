package repo

import (
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Repository provides access to package storage following DDD Repository pattern
// It acts as a collection-like interface to packages, abstracting persistence details
type Repository interface {
	// LoadPackage loads a single package by exact name
	LoadPackage(name string) (*pkg.Package, error)

	// LoadPackages loads multiple packages by exact names
	LoadPackages(names []string) ([]*pkg.Package, error)

	// FindBySpecification finds all packages matching the specification
	FindBySpecification(spec Specification) ([]*pkg.Package, error)

	// FindByAtom finds all packages matching a PMS-compliant atom.
	// Supports version constraints, slots, and USE flags.
	// Example atoms: ">=sys-libs/glibc-2.38", "dev-lang/python:3.12"
	FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error)

	// GetAllVersions returns all versions of a given package
	GetAllVersions(packageName string) ([]*pkg.Package, error)

	// Exists checks if a package exists in the repository
	Exists(name string) bool

	// Count returns the total number of packages in the repository
	Count() (int, error)
}

// WritableRepository extends Repository with write operations
// Separated to follow Interface Segregation Principle
type WritableRepository interface {
	Repository

	// Add adds a package to the repository
	Add(p *pkg.Package) error

	// Update updates an existing package
	Update(p *pkg.Package) error

	// Remove removes a package from the repository
	Remove(name string) error
}
