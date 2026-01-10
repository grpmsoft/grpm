// Package sets implements Portage package sets.
//
// This file implements file-based custom sets from /etc/portage/sets/.
package sets

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// File-Based Custom Sets
// ============================================================================

// FileSet represents a custom package set defined in a file.
// Located in /etc/portage/sets/<name>.
type FileSet struct {
	name     string
	filePath string
}

// NewFileSet creates a new file-based set.
func NewFileSet(name, filePath string) *FileSet {
	return &FileSet{
		name:     name,
		filePath: filePath,
	}
}

// Name returns the set name.
func (f *FileSet) Name() string {
	return f.name
}

// Packages returns all packages defined in the set file.
func (f *FileSet) Packages() ([]*pkg.Atom, error) {
	file, err := os.Open(f.filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var atoms []*pkg.Atom
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle nested set references
		if strings.HasPrefix(line, "@") {
			// Nested sets are handled by the registry during expansion
			// For now, skip them
			continue
		}

		atom, err := pkg.ParseAtom(line)
		if err != nil {
			// Skip invalid atoms
			continue
		}

		atoms = append(atoms, atom)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return atoms, nil
}

// Contains checks if a package is in the set.
func (f *FileSet) Contains(atom *pkg.Atom) bool {
	atoms, err := f.Packages()
	if err != nil {
		return false
	}

	for _, a := range atoms {
		if AtomsEqual(a, atom) {
			return true
		}
	}
	return false
}

// ============================================================================
// Custom Sets Loader
// ============================================================================

// LoadCustomSets loads custom sets from /etc/portage/sets/.
func LoadCustomSets(rootDir string) ([]*FileSet, error) {
	setsDir := filepath.Join(rootDir, "etc", "portage", "sets")

	entries, err := os.ReadDir(setsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No custom sets
		}
		return nil, err
	}

	var sets []*FileSet
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := "@" + entry.Name()
		filePath := filepath.Join(setsDir, entry.Name())
		sets = append(sets, NewFileSet(name, filePath))
	}

	return sets, nil
}

// RegisterCustomSets loads and registers custom sets with the registry.
func RegisterCustomSets(registry *Registry, rootDir string) error {
	customSets, err := LoadCustomSets(rootDir)
	if err != nil {
		return err
	}

	for _, set := range customSets {
		// Ignore errors for already existing sets
		_ = registry.RegisterSet(set.Name(), set)
	}

	return nil
}
