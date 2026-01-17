package repo

import (
	"fmt"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// MockRepository is an in-memory repository for testing.
// Implements both Repository and NamedRepository interfaces.
type MockRepository struct {
	packages map[string]*pkg.Package
	name     string
	priority int
	location string
}

// NewMockRepository creates a new mock repository with sample packages
func NewMockRepository() *MockRepository {
	packages := make(map[string]*pkg.Package)

	// Create hello package
	hello := pkg.NewPackage("app-misc/hello", "2.10", "0")
	hello.AddDependency(pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: pkg.NewVersionConstraint(pkg.OpGreaterEqual, "1.2.13"),
	})

	// Create zlib package
	zlib := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")

	packages["app-misc/hello"] = hello
	packages["sys-libs/zlib"] = zlib

	// Add conflicting package for testing
	conflict := pkg.NewPackage("conflict/example", "1.0", "0")
	conflict.AddDependency(pkg.Constraint{
		Type:    pkg.ConstraintTypeVersion,
		Name:    "sys-libs/zlib",
		Version: pkg.NewVersionConstraint(pkg.OpLess, "1.2.0"), // Conflicting version
	})
	packages["conflict/example"] = conflict

	return &MockRepository{
		packages: packages,
		name:     "mock",
		priority: 0,
		location: "",
	}
}

// NewNamedMockRepository creates a new mock repository with specified name and priority.
func NewNamedMockRepository(name string, priority int) *MockRepository {
	m := NewMockRepository()
	m.name = name
	m.priority = priority
	return m
}

// NewEmptyMockRepository creates an empty mock repository.
func NewEmptyMockRepository() *MockRepository {
	return &MockRepository{
		packages: make(map[string]*pkg.Package),
		name:     "mock",
		priority: 0,
		location: "",
	}
}

// LoadPackages loads multiple packages by name
func (m *MockRepository) LoadPackages(names []string) ([]*pkg.Package, error) {
	result := make([]*pkg.Package, 0, len(names))
	for _, name := range names {
		if pkg, exists := m.packages[name]; exists {
			// Create a copy of the package
			copyPkg := *pkg
			result = append(result, &copyPkg)
		} else {
			return nil, fmt.Errorf("package %s not found", name)
		}
	}
	return result, nil
}

// LoadPackage loads a single package by name
func (m *MockRepository) LoadPackage(name string) (*pkg.Package, error) {
	if pkg, exists := m.packages[name]; exists {
		// Create a copy of the package
		copyPkg := *pkg
		return &copyPkg, nil
	}
	return nil, fmt.Errorf("package %s not found", name)
}

// LoadPackageVersion loads a specific version of a package.
// For MockRepository, this checks if the stored package matches the requested version.
func (m *MockRepository) LoadPackageVersion(name, version string) (*pkg.Package, error) {
	if p, exists := m.packages[name]; exists {
		if p.Version == version {
			copyPkg := *p
			return &copyPkg, nil
		}
		return nil, fmt.Errorf("version %s not found for %s (have %s)", version, name, p.Version)
	}
	return nil, fmt.Errorf("package %s not found", name)
}

// FindBySpecification finds packages matching the specification
func (m *MockRepository) FindBySpecification(spec Specification) ([]*pkg.Package, error) {
	var result []*pkg.Package
	for _, p := range m.packages {
		if spec.IsSatisfiedBy(p) {
			copyPkg := *p
			result = append(result, &copyPkg)
		}
	}
	return result, nil
}

// FindByAtom finds all packages matching a PMS-compliant atom.
// Uses Atom.Matches() for version, slot, and subslot matching.
func (m *MockRepository) FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error) {
	if atom == nil {
		return nil, fmt.Errorf("atom is nil")
	}

	var result []*pkg.Package
	for _, p := range m.packages {
		if atom.Matches(p) {
			copyPkg := *p
			result = append(result, &copyPkg)
		}
	}
	return result, nil
}

// GetAllVersions returns all versions of a package
// Note: MockRepository stores only one version per package
func (m *MockRepository) GetAllVersions(packageName string) ([]*pkg.Package, error) {
	if p, exists := m.packages[packageName]; exists {
		copyPkg := *p
		return []*pkg.Package{&copyPkg}, nil
	}
	return []*pkg.Package{}, nil
}

// Exists checks if a package exists
func (m *MockRepository) Exists(name string) bool {
	_, exists := m.packages[name]
	return exists
}

// Count returns the number of packages
func (m *MockRepository) Count() (int, error) {
	return len(m.packages), nil
}

// Add adds a package to the repository (WritableRepository)
func (m *MockRepository) Add(p *pkg.Package) error {
	// Create a copy before saving
	copyPkg := *p
	m.packages[p.Name] = &copyPkg
	return nil
}

// Update updates an existing package (WritableRepository)
func (m *MockRepository) Update(p *pkg.Package) error {
	if !m.Exists(p.Name) {
		return fmt.Errorf("package %s does not exist", p.Name)
	}
	return m.Add(p)
}

// Remove removes a package from the repository (WritableRepository)
func (m *MockRepository) Remove(name string) error {
	if !m.Exists(name) {
		return fmt.Errorf("package %s does not exist", name)
	}
	delete(m.packages, name)
	return nil
}

// Name returns the repository name (NamedRepository interface).
func (m *MockRepository) Name() string {
	return m.name
}

// Priority returns the repository priority (NamedRepository interface).
func (m *MockRepository) Priority() int {
	return m.priority
}

// SetPriority sets the repository priority (NamedRepository interface).
func (m *MockRepository) SetPriority(priority int) {
	m.priority = priority
}

// Location returns the filesystem path of the repository (NamedRepository interface).
func (m *MockRepository) Location() string {
	return m.location
}

// SetName sets the repository name.
func (m *MockRepository) SetName(name string) {
	m.name = name
}

// SetLocation sets the repository location.
func (m *MockRepository) SetLocation(location string) {
	m.location = location
}
