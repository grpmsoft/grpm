// Package sets implements Portage package sets.
//
// This file implements the @world set.
package sets

import (
	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// World Set (@world)
// ============================================================================

// WorldSet represents all packages that should be kept installed.
// @world = @selected + @system
type WorldSet struct {
	selected *SelectedSet
	system   *SystemSet
}

// NewWorldSet creates a new world set.
func NewWorldSet(selected *SelectedSet, system *SystemSet) *WorldSet {
	return &WorldSet{
		selected: selected,
		system:   system,
	}
}

// Name returns the set name.
func (w *WorldSet) Name() string {
	return "@world"
}

// Packages returns all packages in @world (= @selected + @system).
func (w *WorldSet) Packages() ([]*pkg.Atom, error) {
	var result []*pkg.Atom

	// Add @system packages first
	if w.system != nil {
		system, err := w.system.Packages()
		if err != nil {
			return nil, err
		}
		result = append(result, system...)
	}

	// Add @selected packages
	if w.selected != nil {
		selected, err := w.selected.Packages()
		if err != nil {
			return nil, err
		}
		result = append(result, selected...)
	}

	return Deduplicate(result), nil
}

// Contains checks if a package is in @world.
func (w *WorldSet) Contains(atom *pkg.Atom) bool {
	if w.system != nil && w.system.Contains(atom) {
		return true
	}
	if w.selected != nil && w.selected.Contains(atom) {
		return true
	}
	return false
}
