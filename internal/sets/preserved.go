// Package sets implements Portage package sets.
//
// This file implements the @preserved-rebuild set.
package sets

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Preserved Rebuild Set (@preserved-rebuild)
// ============================================================================

// PreservedRebuildSet represents packages needing rebuild after library changes.
// Reads from /var/lib/portage/preserved_libs_registry.
type PreservedRebuildSet struct {
	rootDir string
}

// NewPreservedRebuildSet creates a new preserved rebuild set.
func NewPreservedRebuildSet(rootDir string) *PreservedRebuildSet {
	return &PreservedRebuildSet{
		rootDir: rootDir,
	}
}

// Name returns the set name.
func (p *PreservedRebuildSet) Name() string {
	return "@preserved-rebuild"
}

// Packages returns packages that need rebuilding due to preserved libraries.
func (p *PreservedRebuildSet) Packages() ([]*pkg.Atom, error) {
	// Read from preserved libs registry
	registryFile := filepath.Join(p.rootDir, "var", "lib", "portage", "preserved_libs_registry")

	file, err := os.Open(registryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No preserved libs
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	atomMap := make(map[string]*pkg.Atom)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: category/package-version: [lib1, lib2, ...]
		// We just need the category/package part
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			continue
		}

		atomStr := strings.TrimSpace(parts[0])

		// The format is "category/package-version", but ParseAtom expects
		// a version operator. Try parsing as-is first, then with "=" prefix.
		atom, err := pkg.ParseAtom(atomStr)
		if err != nil {
			// Try with "=" prefix for exact version match
			atom, err = pkg.ParseAtom("=" + atomStr)
			if err != nil {
				continue
			}
		}

		// Use category/package as key to avoid duplicates
		key := atom.Category + "/" + atom.Package
		atomMap[key] = atom
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice
	result := make([]*pkg.Atom, 0, len(atomMap))
	for _, atom := range atomMap {
		result = append(result, atom)
	}

	return result, nil
}

// Contains checks if a package is in the preserved rebuild set.
func (p *PreservedRebuildSet) Contains(atom *pkg.Atom) bool {
	atoms, err := p.Packages()
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
