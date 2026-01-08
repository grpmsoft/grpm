// Package state provides package sets management for GRPM.
//
// This module implements Portage-compatible package sets:
//   - @selected: User-installed packages (/var/lib/portage/world)
//   - @system: Base system packages (profile/packages)
//   - @world: Union of @selected and @system
//
// Example:
//
//	manager := state.NewSetManager("/var/lib/portage", profilePath)
//	world, err := manager.GetWorld()
//	updates, err := manager.CalculateUpdates(world, repo)
package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/profile"
	"github.com/grpmsoft/grpm/internal/repo"
)

// SetName represents a package set identifier.
type SetName string

const (
	// SetWorld represents the @world set (selected + system).
	SetWorld SetName = "@world"

	// SetSelected represents the @selected set (user-installed packages).
	SetSelected SetName = "@selected"

	// SetSystem represents the @system set (base system packages).
	SetSystem SetName = "@system"
)

// PackageSet represents a collection of package atoms.
//
// This is a Value Object - immutable after creation.
type PackageSet struct {
	name    SetName
	atoms   []string
	atomSet map[string]bool
}

// NewPackageSet creates a new package set with the given name and atoms.
func NewPackageSet(name SetName, atoms []string) *PackageSet {
	atomSet := make(map[string]bool, len(atoms))
	uniqueAtoms := make([]string, 0, len(atoms))

	for _, atom := range atoms {
		atom = strings.TrimSpace(atom)
		if atom == "" {
			continue
		}
		if !atomSet[atom] {
			atomSet[atom] = true
			uniqueAtoms = append(uniqueAtoms, atom)
		}
	}

	sort.Strings(uniqueAtoms)

	return &PackageSet{
		name:    name,
		atoms:   uniqueAtoms,
		atomSet: atomSet,
	}
}

// Name returns the set name.
func (s *PackageSet) Name() SetName {
	return s.name
}

// Atoms returns a copy of the package atoms in the set.
func (s *PackageSet) Atoms() []string {
	result := make([]string, len(s.atoms))
	copy(result, s.atoms)
	return result
}

// Contains checks if the set contains the given atom.
func (s *PackageSet) Contains(atom string) bool {
	return s.atomSet[atom]
}

// Len returns the number of atoms in the set.
func (s *PackageSet) Len() int {
	return len(s.atoms)
}

// Union returns a new set containing atoms from both sets.
func (s *PackageSet) Union(other *PackageSet) *PackageSet {
	atoms := make([]string, 0, len(s.atoms)+len(other.atoms))
	atoms = append(atoms, s.atoms...)
	atoms = append(atoms, other.atoms...)
	return NewPackageSet(SetWorld, atoms)
}

// UpdateInfo contains information about a package update.
type UpdateInfo struct {
	// Atom is the package atom (category/name).
	Atom string

	// InstalledVersion is the currently installed version (empty if new).
	InstalledVersion string

	// AvailableVersion is the version available in the repository.
	AvailableVersion string

	// Slot is the package slot.
	Slot string

	// IsNew indicates this is a new package (not installed).
	IsNew bool

	// IsUpgrade indicates this is an upgrade (newer version available).
	IsUpgrade bool

	// IsDowngrade indicates this is a downgrade (older version in repo).
	IsDowngrade bool

	// UseChanged indicates USE flags have changed.
	UseChanged bool

	// OldUseFlags are the USE flags used during installation.
	OldUseFlags []string

	// NewUseFlags are the USE flags that would be used now.
	NewUseFlags []string
}

// String returns a formatted string representation.
func (u *UpdateInfo) String() string {
	var prefix string
	switch {
	case u.IsNew:
		prefix = "N"
	case u.IsUpgrade:
		prefix = "U"
	case u.IsDowngrade:
		prefix = "UD"
	case u.UseChanged:
		prefix = "U"
	default:
		prefix = "R" // Reinstall
	}

	versionInfo := u.AvailableVersion
	if u.InstalledVersion != "" && u.InstalledVersion != u.AvailableVersion {
		versionInfo = fmt.Sprintf("%s [%s]", u.AvailableVersion, u.InstalledVersion)
	}

	return fmt.Sprintf("[ebuild     %s ] %s-%s", prefix, u.Atom, versionInfo)
}

// UpdatePlan contains the complete update plan.
type UpdatePlan struct {
	// Updates contains packages that need updating.
	Updates []*UpdateInfo

	// NewPackages is the count of new packages.
	NewPackages int

	// Upgrades is the count of upgrades.
	Upgrades int

	// Downgrades is the count of downgrades.
	Downgrades int

	// Reinstalls is the count of reinstalls (USE changes).
	Reinstalls int
}

// SetManager manages package sets.
type SetManager struct {
	// portageDir is the Portage state directory (/var/lib/portage).
	portageDir string

	// profilePath is the path to the active profile.
	profilePath string

	// profile is the loaded profile (lazy-loaded).
	profile *profile.Profile

	// db is the installed packages database.
	db *PackageDatabase
}

// NewSetManager creates a new set manager.
//
// Parameters:
//   - portageDir: Path to Portage state directory (default: /var/lib/portage)
//   - profilePath: Path to the active profile (default: /etc/portage/make.profile)
func NewSetManager(portageDir, profilePath string) *SetManager {
	if portageDir == "" {
		portageDir = "/var/lib/portage"
	}
	if profilePath == "" {
		profilePath = "/etc/portage/make.profile"
	}

	return &SetManager{
		portageDir:  portageDir,
		profilePath: profilePath,
	}
}

// SetDatabase sets the package database for installed package lookups.
func (m *SetManager) SetDatabase(db *PackageDatabase) {
	m.db = db
}

// GetSelected returns the @selected set (user-installed packages).
//
// Reads from /var/lib/portage/world file.
func (m *SetManager) GetSelected() (*PackageSet, error) {
	worldPath := filepath.Join(m.portageDir, "world")
	atoms, err := parseWorldFile(worldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read world file: %w", err)
	}
	return NewPackageSet(SetSelected, atoms), nil
}

// GetSystem returns the @system set (base system packages).
//
// Reads from profile/packages file.
func (m *SetManager) GetSystem() (*PackageSet, error) {
	if m.profile == nil {
		p, err := profile.LoadProfile(m.profilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load profile: %w", err)
		}
		if err := p.Resolve(); err != nil {
			return nil, fmt.Errorf("failed to resolve profile: %w", err)
		}
		m.profile = p
	}

	atoms := m.profile.GetSystemPackages()
	return NewPackageSet(SetSystem, atoms), nil
}

// GetWorld returns the @world set (@selected + @system).
func (m *SetManager) GetWorld() (*PackageSet, error) {
	selected, err := m.GetSelected()
	if err != nil {
		return nil, fmt.Errorf("failed to get @selected: %w", err)
	}

	system, err := m.GetSystem()
	if err != nil {
		return nil, fmt.Errorf("failed to get @system: %w", err)
	}

	world := selected.Union(system)
	return &PackageSet{
		name:    SetWorld,
		atoms:   world.atoms,
		atomSet: world.atomSet,
	}, nil
}

// GetSet returns a package set by name.
func (m *SetManager) GetSet(name SetName) (*PackageSet, error) {
	switch name {
	case SetWorld:
		return m.GetWorld()
	case SetSelected:
		return m.GetSelected()
	case SetSystem:
		return m.GetSystem()
	default:
		return nil, fmt.Errorf("unknown set: %s", name)
	}
}

// UpdateOptions contains options for update calculation.
type UpdateOptions struct {
	// Deep includes dependencies in update calculation.
	Deep bool

	// NewUse recalculates USE flags and updates if changed.
	NewUse bool

	// ChangedUse only updates packages with changed USE flags.
	ChangedUse bool

	// Update only updates packages that have newer versions.
	Update bool
}

// DefaultUpdateOptions returns the default update options.
func DefaultUpdateOptions() *UpdateOptions {
	return &UpdateOptions{
		Update: true,
	}
}

// CalculateUpdates calculates available updates for the given package set.
//
// Parameters:
//   - set: The package set to calculate updates for
//   - r: The repository to check for available versions
//   - opts: Update options (deep, newuse, etc.)
//
// Returns an UpdatePlan with all packages that need updating.
func (m *SetManager) CalculateUpdates(set *PackageSet, r repo.Repository, opts *UpdateOptions) (*UpdatePlan, error) {
	if opts == nil {
		opts = DefaultUpdateOptions()
	}

	plan := &UpdatePlan{
		Updates: make([]*UpdateInfo, 0),
	}

	// Process each atom in the set
	for _, atom := range set.Atoms() {
		updateInfo, err := m.checkPackageUpdate(atom, r, opts)
		if err != nil {
			// Log warning but continue with other packages
			continue
		}

		if updateInfo != nil {
			plan.Updates = append(plan.Updates, updateInfo)

			// Update counters
			switch {
			case updateInfo.IsNew:
				plan.NewPackages++
			case updateInfo.IsUpgrade:
				plan.Upgrades++
			case updateInfo.IsDowngrade:
				plan.Downgrades++
			case updateInfo.UseChanged:
				plan.Reinstalls++
			}
		}
	}

	// If deep mode, also check dependencies
	if opts.Deep {
		if err := m.addDeepDependencies(plan, r, opts); err != nil {
			return nil, fmt.Errorf("failed to calculate deep dependencies: %w", err)
		}
	}

	return plan, nil
}

// checkPackageUpdate checks if a package needs updating.
func (m *SetManager) checkPackageUpdate(atom string, r repo.Repository, opts *UpdateOptions) (*UpdateInfo, error) {
	// Load available package from repository
	available, err := r.LoadPackage(atom)
	if err != nil {
		return nil, fmt.Errorf("package not found in repository: %s", atom)
	}

	info := &UpdateInfo{
		Atom:             atom,
		AvailableVersion: available.Version,
		Slot:             available.Slot.Name,
	}

	// Check if package is installed
	if m.db != nil {
		installed := m.findInstalledPackage(atom)
		if installed != nil {
			info.InstalledVersion = installed.Package.Version

			// Compare versions
			cmp := pkg.CompareVersions(available.Version, installed.Package.Version)
			switch {
			case cmp > 0:
				info.IsUpgrade = true
			case cmp < 0:
				info.IsDowngrade = true
			}

			// Check USE flag changes if requested
			if opts.NewUse || opts.ChangedUse {
				info.OldUseFlags = installed.USE
				info.NewUseFlags = getUSEFlags(available)

				if !slicesEqual(info.OldUseFlags, info.NewUseFlags) {
					info.UseChanged = true
				}
			}

			// Determine if update is needed
			if opts.Update && !info.IsUpgrade && !info.UseChanged {
				return nil, nil // No update needed
			}
			if opts.ChangedUse && !info.UseChanged {
				return nil, nil // No USE change
			}
		} else {
			info.IsNew = true
		}
	} else {
		// No database - treat as new
		info.IsNew = true
	}

	return info, nil
}

// findInstalledPackage finds an installed package matching the atom.
func (m *SetManager) findInstalledPackage(atom string) *InstalledPackage {
	if m.db == nil {
		return nil
	}

	// List all packages and find matching ones
	for _, installed := range m.db.List() {
		// Extract category/name from installed package
		pkgAtom := extractAtom(installed.Package.Name)
		if pkgAtom == atom || strings.HasPrefix(pkgAtom, atom) || strings.HasSuffix(atom, "/"+filepath.Base(pkgAtom)) {
			return installed
		}
	}

	return nil
}

// addDeepDependencies adds dependencies to the update plan in deep mode.
func (m *SetManager) addDeepDependencies(plan *UpdatePlan, r repo.Repository, opts *UpdateOptions) error {
	processed := make(map[string]bool)

	// Mark already processed packages
	for _, update := range plan.Updates {
		processed[update.Atom] = true
	}

	// Process dependencies of each package
	for _, update := range plan.Updates {
		if err := m.processDependencies(update.Atom, r, plan, processed, opts); err != nil {
			// Log warning but continue
			continue
		}
	}

	return nil
}

// processDependencies recursively processes package dependencies.
func (m *SetManager) processDependencies(atom string, r repo.Repository, plan *UpdatePlan, processed map[string]bool, opts *UpdateOptions) error {
	if processed[atom] {
		return nil
	}
	processed[atom] = true

	// Load package to get dependencies
	p, err := r.LoadPackage(atom)
	if err != nil {
		return nil
	}

	// Process each dependency
	for _, dep := range p.Deps {
		depAtom := dep.Name
		if processed[depAtom] {
			continue
		}

		updateInfo, err := m.checkPackageUpdate(depAtom, r, opts)
		if err != nil {
			continue
		}

		if updateInfo != nil {
			plan.Updates = append(plan.Updates, updateInfo)
			switch {
			case updateInfo.IsNew:
				plan.NewPackages++
			case updateInfo.IsUpgrade:
				plan.Upgrades++
			case updateInfo.IsDowngrade:
				plan.Downgrades++
			case updateInfo.UseChanged:
				plan.Reinstalls++
			}
		}

		// Recurse into dependencies
		if err := m.processDependencies(depAtom, r, plan, processed, opts); err != nil {
			continue
		}
	}

	return nil
}

// parseWorldFile parses the /var/lib/portage/world file.
//
// Format: One package atom per line (category/package).
// Lines starting with # are comments.
//
// Example:
//
//	app-editors/vim
//	sys-apps/systemd
//	dev-lang/go
func parseWorldFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Empty world file is valid
			return []string{}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	atoms := make([]string, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		atoms = append(atoms, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return atoms, nil
}

// extractAtom extracts the category/name from a full package name.
//
// Example: "sys-libs/zlib-1.2.13" -> "sys-libs/zlib"
func extractAtom(fullName string) string {
	// Find the last hyphen followed by a digit (version start)
	for i := len(fullName) - 1; i >= 0; i-- {
		if fullName[i] == '-' && i+1 < len(fullName) {
			nextChar := fullName[i+1]
			if nextChar >= '0' && nextChar <= '9' {
				return fullName[:i]
			}
		}
	}
	return fullName
}

// getUSEFlags extracts USE flags from a package.
func getUSEFlags(p *pkg.Package) []string {
	if p == nil || p.UseFlags == nil {
		return []string{}
	}

	flags := make([]string, 0, len(p.UseFlags))
	for flag, enabled := range p.UseFlags {
		if enabled {
			flags = append(flags, flag)
		} else {
			flags = append(flags, "-"+flag)
		}
	}
	sort.Strings(flags)
	return flags
}

// slicesEqual checks if two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// WriteWorld writes the @selected set to the world file.
func (m *SetManager) WriteWorld(set *PackageSet) error {
	worldPath := filepath.Join(m.portageDir, "world")

	// Ensure directory exists
	if err := os.MkdirAll(m.portageDir, 0755); err != nil {
		return fmt.Errorf("failed to create portage directory: %w", err)
	}

	// Write atoms to file
	file, err := os.Create(worldPath)
	if err != nil {
		return fmt.Errorf("failed to create world file: %w", err)
	}
	defer func() { _ = file.Close() }()

	for _, atom := range set.Atoms() {
		if _, err := fmt.Fprintln(file, atom); err != nil {
			return fmt.Errorf("failed to write atom: %w", err)
		}
	}

	return nil
}

// AddToWorld adds a package to the world file.
func (m *SetManager) AddToWorld(atom string) error {
	selected, err := m.GetSelected()
	if err != nil {
		// If world file doesn't exist, create empty set
		selected = NewPackageSet(SetSelected, []string{})
	}

	if selected.Contains(atom) {
		return nil // Already in world
	}

	atoms := append(selected.Atoms(), atom)
	newSet := NewPackageSet(SetSelected, atoms)

	return m.WriteWorld(newSet)
}

// RemoveFromWorld removes a package from the world file.
func (m *SetManager) RemoveFromWorld(atom string) error {
	selected, err := m.GetSelected()
	if err != nil {
		return nil // World file doesn't exist, nothing to remove
	}

	if !selected.Contains(atom) {
		return nil // Not in world
	}

	atoms := make([]string, 0, len(selected.atoms)-1)
	for _, a := range selected.Atoms() {
		if a != atom {
			atoms = append(atoms, a)
		}
	}

	newSet := NewPackageSet(SetSelected, atoms)
	return m.WriteWorld(newSet)
}
