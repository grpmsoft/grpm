// Package sets implements Portage package sets.
//
// Package sets allow grouping packages for batch operations:
//   - @world: All user-selected and system packages
//   - @system: Core system packages from profile
//   - @selected: User-explicitly-installed packages
//   - @preserved-rebuild: Packages needing rebuild after library changes
//   - Custom sets: User-defined package groups
//
// Reference: https://wiki.gentoo.org/wiki/Package_sets
package sets

import (
	"fmt"
	"strings"
	"sync"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/profile"
)

// ============================================================================
// Set Interface
// ============================================================================

// Set represents a package set.
type Set interface {
	// Name returns the set name (with @ prefix).
	Name() string

	// Packages returns all packages in this set.
	Packages() ([]*pkg.Atom, error)

	// Contains checks if an atom is in this set.
	Contains(atom *pkg.Atom) bool
}

// MutableSet is a set that can be modified.
type MutableSet interface {
	Set

	// Add adds a package to the set.
	Add(atom *pkg.Atom) error

	// Remove removes a package from the set.
	Remove(atom *pkg.Atom) error
}

// ============================================================================
// Set Registry
// ============================================================================

// Registry manages all known package sets.
type Registry struct {
	sets map[string]Set
	mu   sync.RWMutex

	// Dependencies for building sets
	profile *profile.Profile
	rootDir string
}

// NewRegistry creates a new set registry.
//
// Parameters:
//   - rootDir: System root directory (usually "/")
//   - profile: Loaded profile for @system set
func NewRegistry(rootDir string, profile *profile.Profile) *Registry {
	r := &Registry{
		sets:    make(map[string]Set),
		profile: profile,
		rootDir: rootDir,
	}

	// Register built-in sets
	r.registerBuiltinSets()

	return r
}

// registerBuiltinSets registers the standard Portage sets.
func (r *Registry) registerBuiltinSets() {
	// Create @selected first (needed by @world)
	selected := NewSelectedSet(r.rootDir)
	r.sets["@selected"] = selected

	// Create @system (reads from profile)
	system := NewSystemSet(r.rootDir, r.profile)
	r.sets["@system"] = system

	// Create @world (= @selected + @system)
	world := NewWorldSet(selected, system)
	r.sets["@world"] = world

	// Create @preserved-rebuild (packages needing rebuild after library changes)
	preserved := NewPreservedRebuildSet(r.rootDir)
	r.sets["@preserved-rebuild"] = preserved
}

// GetSet returns a set by name.
func (r *Registry) GetSet(name string) (Set, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Ensure name has @ prefix
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	set, ok := r.sets[name]
	if !ok {
		return nil, fmt.Errorf("unknown set: %s", name)
	}

	return set, nil
}

// RegisterSet adds a custom set to the registry.
func (r *Registry) RegisterSet(name string, set Set) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure name has @ prefix
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	if _, exists := r.sets[name]; exists {
		return fmt.Errorf("set already exists: %s", name)
	}

	r.sets[name] = set
	return nil
}

// ListSets returns all registered set names.
func (r *Registry) ListSets() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.sets))
	for name := range r.sets {
		names = append(names, name)
	}
	return names
}

// ExpandSet recursively expands a set to package atoms.
// Handles nested sets (e.g., @world contains @selected and @system).
func (r *Registry) ExpandSet(name string) ([]*pkg.Atom, error) {
	set, err := r.GetSet(name)
	if err != nil {
		return nil, err
	}

	return set.Packages()
}

// ============================================================================
// Helper Functions
// ============================================================================

// Deduplicate removes duplicate atoms from a slice.
func Deduplicate(atoms []*pkg.Atom) []*pkg.Atom {
	seen := make(map[string]bool)
	result := make([]*pkg.Atom, 0, len(atoms))

	for _, atom := range atoms {
		key := atom.String()
		if !seen[key] {
			seen[key] = true
			result = append(result, atom)
		}
	}

	return result
}

// AtomsEqual checks if two atoms are equal (same category/package).
func AtomsEqual(a, b *pkg.Atom) bool {
	return a.Category == b.Category && a.Package == b.Package
}

// IsSetReference checks if a string is a set reference.
func IsSetReference(s string) bool {
	return strings.HasPrefix(s, "@")
}
