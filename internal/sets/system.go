// Package sets implements Portage package sets.
//
// This file implements the @system set (profile system packages).
package sets

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/profile"
)

// ============================================================================
// System Set (@system)
// ============================================================================

// SystemSet represents core system packages from the profile.
// Reads from profile's "packages" file.
type SystemSet struct {
	rootDir string
	profile *profile.Profile
}

// NewSystemSet creates a new system set.
func NewSystemSet(rootDir string, profile *profile.Profile) *SystemSet {
	return &SystemSet{
		rootDir: rootDir,
		profile: profile,
	}
}

// Name returns the set name.
func (s *SystemSet) Name() string {
	return "@system"
}

// Packages returns all system packages from the profile.
func (s *SystemSet) Packages() ([]*pkg.Atom, error) {
	var atoms []*pkg.Atom

	// Read packages from profile
	profilePackages, err := s.readProfilePackages()
	if err != nil {
		return nil, err
	}
	atoms = append(atoms, profilePackages...)

	// Read from /etc/portage/profile/packages (user overrides)
	userPackages, err := s.readUserPackages()
	if err == nil {
		atoms = append(atoms, userPackages...)
	}

	return Deduplicate(atoms), nil
}

// Contains checks if a package is in the system set.
func (s *SystemSet) Contains(atom *pkg.Atom) bool {
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

// readProfilePackages reads packages from the profile's packages file.
func (s *SystemSet) readProfilePackages() ([]*pkg.Atom, error) {
	if s.profile == nil {
		return nil, nil
	}

	// Get profile directory path
	profileDir := s.profile.Path
	if profileDir == "" {
		// Try default location
		profileDir = filepath.Join(s.rootDir, "var", "db", "repos", "gentoo", "profiles", "default", "linux", "amd64")
	}

	// Find all packages files in profile inheritance chain
	var atoms []*pkg.Atom

	// Walk up the profile chain
	currentDir := profileDir
	for {
		packagesFile := filepath.Join(currentDir, "packages")
		if fileAtoms, err := s.parsePackagesFile(packagesFile); err == nil {
			atoms = append(atoms, fileAtoms...)
		}

		// Check for parent profile
		parentFile := filepath.Join(currentDir, "parent")
		parentContent, err := os.ReadFile(parentFile)
		if err != nil {
			break
		}

		// Parse parent reference
		parentPath := strings.TrimSpace(string(parentContent))
		if parentPath == "" {
			break
		}

		// Resolve parent path
		if !filepath.IsAbs(parentPath) {
			currentDir = filepath.Clean(filepath.Join(currentDir, parentPath))
		} else {
			currentDir = parentPath
		}
	}

	return atoms, nil
}

// readUserPackages reads packages from /etc/portage/profile/packages.
func (s *SystemSet) readUserPackages() ([]*pkg.Atom, error) {
	packagesFile := filepath.Join(s.rootDir, "etc", "portage", "profile", "packages")
	return s.parsePackagesFile(packagesFile)
}

// parsePackagesFile parses a packages file.
// Lines starting with * are system packages.
func (s *SystemSet) parsePackagesFile(path string) ([]*pkg.Atom, error) {
	file, err := os.Open(path)
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

		// System packages start with *
		if strings.HasPrefix(line, "*") {
			atomStr := strings.TrimPrefix(line, "*")
			atomStr = strings.TrimSpace(atomStr)

			atom, err := pkg.ParseAtom(atomStr)
			if err != nil {
				// Skip invalid atoms
				continue
			}

			atoms = append(atoms, atom)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return atoms, nil
}
