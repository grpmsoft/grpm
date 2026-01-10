// Package sets implements Portage package sets.
//
// This file implements the @selected set (world file).
package sets

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Selected Set (@selected)
// ============================================================================

// SelectedSet represents explicitly installed packages.
// Stored in /var/lib/portage/world.
type SelectedSet struct {
	worldFile string
	mu        sync.RWMutex
}

// NewSelectedSet creates a new selected set.
func NewSelectedSet(rootDir string) *SelectedSet {
	return &SelectedSet{
		worldFile: filepath.Join(rootDir, "var", "lib", "portage", "world"),
	}
}

// Name returns the set name.
func (s *SelectedSet) Name() string {
	return "@selected"
}

// Packages returns all packages in the world file.
func (s *SelectedSet) Packages() ([]*pkg.Atom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readWorldFile()
}

// Contains checks if a package is in the world file.
func (s *SelectedSet) Contains(atom *pkg.Atom) bool {
	atoms, err := s.Packages()
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

// Add adds a package to the world file.
func (s *SelectedSet) Add(atom *pkg.Atom) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read existing packages
	atoms, err := s.readWorldFile()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read world file: %w", err)
	}

	// Check if already exists
	for _, a := range atoms {
		if AtomsEqual(a, atom) {
			return nil // Already in world
		}
	}

	// Add new package
	atoms = append(atoms, atom)

	return s.writeWorldFile(atoms)
}

// Remove removes a package from the world file.
func (s *SelectedSet) Remove(atom *pkg.Atom) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read existing packages
	atoms, err := s.readWorldFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("read world file: %w", err)
	}

	// Filter out the package
	filtered := make([]*pkg.Atom, 0, len(atoms))
	for _, a := range atoms {
		if !AtomsEqual(a, atom) {
			filtered = append(filtered, a)
		}
	}

	// Only write if something changed
	if len(filtered) == len(atoms) {
		return nil // Package wasn't in world
	}

	return s.writeWorldFile(filtered)
}

// readWorldFile reads packages from the world file.
func (s *SelectedSet) readWorldFile() ([]*pkg.Atom, error) {
	file, err := os.Open(s.worldFile)
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

		// Parse atom (world file uses category/package format)
		atom, err := pkg.ParseAtom(line)
		if err != nil {
			// Log warning but continue
			continue
		}

		atoms = append(atoms, atom)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan world file: %w", err)
	}

	return atoms, nil
}

// writeWorldFile writes packages to the world file.
func (s *SelectedSet) writeWorldFile(atoms []*pkg.Atom) error {
	// Ensure directory exists
	dir := filepath.Dir(s.worldFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write to temp file first
	tmpFile := s.worldFile + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	writer := bufio.NewWriter(file)
	for _, atom := range atoms {
		// Write category/package format (no version)
		line := fmt.Sprintf("%s/%s\n", atom.Category, atom.Package)
		if _, err := writer.WriteString(line); err != nil {
			_ = file.Close()
			_ = os.Remove(tmpFile)
			return fmt.Errorf("write atom: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("flush: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("close: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, s.worldFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// WorldFile returns the path to the world file.
func (s *SelectedSet) WorldFile() string {
	return s.worldFile
}
