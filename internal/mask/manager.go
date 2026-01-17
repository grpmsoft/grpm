// Package mask implements package mask management for GRPM.
//
// Masks prevent packages from being selected by the solver. Portage supports
// multiple mask sources with different priorities:
//
//  1. Repository profiles/package.mask - Repository-wide masks
//  2. Profile cascade package.mask - Profile-specific masks
//  3. User /etc/portage/package.mask - User-defined masks
//  4. User /etc/portage/package.unmask - User-defined unmasks (overrides all masks)
//
// The MaskManager loads masks from all sources and provides IsMasked() to check
// if a specific package version should be excluded from solver consideration.
//
// Example:
//
//	manager, err := mask.NewMaskManager(cfg, "/var/db/repos/gentoo", profilePath)
//	if err != nil {
//	    return err
//	}
//	if manager.IsMasked("sys-devel", "gcc", "16.0.9999", "16") {
//	    // Skip this version in solver
//	}
package mask

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// MaskEntry represents a single mask or unmask entry with source tracking.
type MaskEntry struct {
	// Atom is the parsed package atom with version constraints
	Atom *config.PackageAtom

	// Source indicates where this mask came from (for debugging)
	Source string

	// SourceFile is the exact file path where this mask was defined
	SourceFile string

	// IsNegation indicates if this entry removes a mask (starts with -)
	IsNegation bool
}

// MaskSource represents the source type of a mask for priority ordering.
type MaskSource int

const (
	// MaskSourceRepo is the lowest priority - repository profiles/package.mask
	MaskSourceRepo MaskSource = iota
	// MaskSourceProfile is medium priority - profile cascade package.mask
	MaskSourceProfile
	// MaskSourceUser is highest priority - /etc/portage/package.mask
	MaskSourceUser
)

// MaskManager manages package masks from multiple sources with proper priority.
// It implements Portage-compatible mask resolution where:
// - User unmasks override all masks
// - User masks override profile and repo masks
// - Profile masks override repo masks
// - Repository masks have lowest priority
type MaskManager struct {
	// repoMasks contains masks from repository profiles/package.mask
	repoMasks []*MaskEntry

	// profileMasks contains masks from profile cascade package.mask
	profileMasks []*MaskEntry

	// userMasks contains masks from /etc/portage/package.mask
	userMasks []*MaskEntry

	// userUnmasks contains unmasks from /etc/portage/package.unmask
	userUnmasks []*MaskEntry

	// maskIndex maps category/package to list of applicable mask entries
	// for fast lookup during package resolution
	maskIndex map[string][]*MaskEntry

	// unmaskIndex maps category/package to list of applicable unmask entries
	unmaskIndex map[string][]*MaskEntry
}

// NewMaskManager creates a new MaskManager by loading masks from all sources.
//
// Parameters:
//   - cfg: Portage configuration (for user config path, typically /etc/portage)
//   - repoPath: Repository path (e.g., /var/db/repos/gentoo)
//   - profilePath: Profile path (e.g., /etc/portage/make.profile)
//
// The manager loads masks in priority order and builds an index for fast lookup.
func NewMaskManager(cfg *config.Config, repoPath, profilePath string) (*MaskManager, error) {
	m := &MaskManager{
		repoMasks:    make([]*MaskEntry, 0),
		profileMasks: make([]*MaskEntry, 0),
		userMasks:    make([]*MaskEntry, 0),
		userUnmasks:  make([]*MaskEntry, 0),
		maskIndex:    make(map[string][]*MaskEntry),
		unmaskIndex:  make(map[string][]*MaskEntry),
	}

	// 1. Load repository masks (lowest priority)
	if repoPath != "" {
		if err := m.loadRepoMasks(repoPath); err != nil {
			logging.Debug("Warning: failed to load repo masks: %v", err)
			// Non-fatal: continue without repo masks
		}
	}

	// 2. Load profile cascade masks (medium priority)
	if profilePath != "" {
		if err := m.loadProfileMasks(profilePath); err != nil {
			logging.Debug("Warning: failed to load profile masks: %v", err)
			// Non-fatal: continue without profile masks
		}
	}

	// 3. Load user masks and unmasks (highest priority)
	if cfg != nil && cfg.Root != "" {
		if err := m.loadUserMasks(cfg.Root); err != nil {
			logging.Debug("Warning: failed to load user masks: %v", err)
		}
		if err := m.loadUserUnmasks(cfg.Root); err != nil {
			logging.Debug("Warning: failed to load user unmasks: %v", err)
		}
	}

	// Build index for fast lookup
	m.buildIndex()

	logging.Debug("MaskManager loaded: %d repo masks, %d profile masks, %d user masks, %d user unmasks",
		len(m.repoMasks), len(m.profileMasks), len(m.userMasks), len(m.userUnmasks))

	return m, nil
}

// IsMasked checks if a specific package version is masked.
//
// Returns true if the package is masked and NOT unmasked by user configuration.
// User unmasks always take precedence over all masks.
//
// Parameters:
//   - category: Package category (e.g., "sys-devel")
//   - name: Package name (e.g., "gcc")
//   - version: Package version (e.g., "16.0.9999")
//   - slot: Package slot (e.g., "16", can be empty)
func (m *MaskManager) IsMasked(category, name, version, slot string) bool {
	// Check if unmasked first (user unmasks override everything)
	if m.isUnmasked(category, name, version, slot) {
		return false
	}

	// Check masks in priority order (user > profile > repo)
	cp := category + "/" + name

	// Check user masks first (highest priority)
	if entries, ok := m.maskIndex[cp]; ok {
		for _, entry := range entries {
			if entry.Atom.Matches(category, name, version, slot) {
				return true
			}
		}
	}

	return false
}

// isUnmasked checks if the package is explicitly unmasked.
func (m *MaskManager) isUnmasked(category, name, version, slot string) bool {
	cp := category + "/" + name

	if entries, ok := m.unmaskIndex[cp]; ok {
		for _, entry := range entries {
			if entry.Atom.Matches(category, name, version, slot) {
				return true
			}
		}
	}

	return false
}

// GetMaskAtom returns the mask atom that matched this package, or nil if not masked.
// Useful for displaying mask reasons to users.
func (m *MaskManager) GetMaskAtom(category, name, version, slot string) *config.PackageAtom {
	if m.isUnmasked(category, name, version, slot) {
		return nil
	}

	cp := category + "/" + name

	if entries, ok := m.maskIndex[cp]; ok {
		for _, entry := range entries {
			if entry.Atom.Matches(category, name, version, slot) {
				return entry.Atom
			}
		}
	}

	return nil
}

// GetMaskReason returns why a package is masked and from which source.
// Returns empty strings if the package is not masked.
func (m *MaskManager) GetMaskReason(category, name, version, slot string) (atom string, source string) {
	if m.isUnmasked(category, name, version, slot) {
		return "", ""
	}

	cp := category + "/" + name

	if entries, ok := m.maskIndex[cp]; ok {
		for _, entry := range entries {
			if entry.Atom.Matches(category, name, version, slot) {
				return entry.Atom.Raw, entry.Source
			}
		}
	}

	return "", ""
}

// IsPackageMasked checks if a Package struct is masked.
// Convenience wrapper around IsMasked.
func (m *MaskManager) IsPackageMasked(p *pkg.Package) bool {
	if p == nil {
		return false
	}

	// Extract category and name from p.Name (format: "category/package")
	parts := strings.SplitN(p.Name, "/", 2)
	if len(parts) != 2 {
		return false
	}

	return m.IsMasked(parts[0], parts[1], p.Version, p.Slot.Name)
}

// buildIndex builds fast lookup indexes from all mask sources.
func (m *MaskManager) buildIndex() {
	// Process all masks (repo, profile, user) into mask index
	// User masks are added last, so they have implicit priority
	for _, entry := range m.repoMasks {
		m.addToMaskIndex(entry)
	}
	for _, entry := range m.profileMasks {
		m.addToMaskIndex(entry)
	}
	for _, entry := range m.userMasks {
		m.addToMaskIndex(entry)
	}

	// Process user unmasks into unmask index
	for _, entry := range m.userUnmasks {
		m.addToUnmaskIndex(entry)
	}
}

// addToMaskIndex adds a mask entry to the mask index.
func (m *MaskManager) addToMaskIndex(entry *MaskEntry) {
	if entry.Atom == nil {
		return
	}

	cp := entry.Atom.Category + "/" + entry.Atom.Name

	// Handle negation entries (remove previous masks)
	// Negation entries act like unmasks for specific atoms
	if entry.IsNegation {
		// Instead of removing from mask index, add to unmask index
		// This allows the negated atom to override the broader mask
		m.unmaskIndex[cp] = append(m.unmaskIndex[cp], entry)
		return
	}

	m.maskIndex[cp] = append(m.maskIndex[cp], entry)
}

// addToUnmaskIndex adds an unmask entry to the unmask index.
func (m *MaskManager) addToUnmaskIndex(entry *MaskEntry) {
	if entry.Atom == nil {
		return
	}

	cp := entry.Atom.Category + "/" + entry.Atom.Name
	m.unmaskIndex[cp] = append(m.unmaskIndex[cp], entry)
}

// loadRepoMasks loads masks from repository profiles/package.mask.
func (m *MaskManager) loadRepoMasks(repoPath string) error {
	maskPath := filepath.Join(repoPath, "profiles", "package.mask")

	entries, err := m.loadMaskPath(maskPath, "repository")
	if err != nil {
		return err
	}

	m.repoMasks = entries
	return nil
}

// loadProfileMasks loads masks from the profile cascade.
// Traverses the profile parent chain and collects all package.mask entries.
func (m *MaskManager) loadProfileMasks(profilePath string) error {
	// Load masks from the profile and its parents
	entries, err := m.loadProfileMasksCascade(profilePath)
	if err != nil {
		return err
	}

	m.profileMasks = entries
	return nil
}

// loadProfileMasksCascade recursively loads masks from profile and parents.
func (m *MaskManager) loadProfileMasksCascade(profilePath string) ([]*MaskEntry, error) {
	var allEntries []*MaskEntry

	// Load parent profiles first (lower priority)
	parentFile := filepath.Join(profilePath, "parent")
	if _, err := os.Stat(parentFile); err == nil {
		parents, err := m.readParentFile(parentFile, profilePath)
		if err == nil {
			for _, parentPath := range parents {
				parentEntries, err := m.loadProfileMasksCascade(parentPath)
				if err == nil {
					allEntries = append(allEntries, parentEntries...)
				}
			}
		}
	}

	// Load this profile's masks (higher priority)
	maskPath := filepath.Join(profilePath, "package.mask")
	entries, err := m.loadMaskPath(maskPath, "profile:"+extractProfileName(profilePath))
	if err == nil {
		allEntries = append(allEntries, entries...)
	}

	// Load this profile's unmasks (for profile-level negation)
	unmaskPath := filepath.Join(profilePath, "package.unmask")
	unmaskEntries, err := m.loadMaskPath(unmaskPath, "profile:"+extractProfileName(profilePath))
	if err == nil {
		// Profile unmasks act as negations to parent masks
		for _, entry := range unmaskEntries {
			entry.IsNegation = true
			allEntries = append(allEntries, entry)
		}
	}

	return allEntries, nil
}

// readParentFile reads parent profile paths from a profile's parent file.
func (m *MaskManager) readParentFile(parentFile, currentProfile string) ([]string, error) {
	file, err := os.Open(parentFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var parents []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Resolve parent path
		parentPath := m.resolveParentPath(line, currentProfile)
		if parentPath != "" {
			parents = append(parents, parentPath)
		}
	}

	return parents, scanner.Err()
}

// resolveParentPath resolves a parent reference to an absolute path.
func (m *MaskManager) resolveParentPath(parent, currentProfile string) string {
	// Handle relative paths
	if !strings.Contains(parent, ":") {
		absPath := filepath.Join(currentProfile, parent)
		absPath = filepath.Clean(absPath)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
		return ""
	}

	// Handle repo:path format (e.g., gentoo:default/linux/amd64/23.0)
	parts := strings.SplitN(parent, ":", 2)
	if len(parts) != 2 {
		return ""
	}

	repoName := parts[0]
	profilePath := parts[1]

	// Handle :path format (same repo)
	if repoName == "" {
		repoRoot := findRepoRoot(currentProfile)
		if repoRoot != "" {
			return filepath.Join(repoRoot, "profiles", profilePath)
		}
		return ""
	}

	// Try common repository locations
	commonPaths := []string{
		filepath.Join("/var/db/repos", repoName, "profiles", profilePath),
		filepath.Join("/usr/portage", "profiles", profilePath),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadUserMasks loads masks from /etc/portage/package.mask.
func (m *MaskManager) loadUserMasks(configRoot string) error {
	maskPath := filepath.Join(configRoot, "package.mask")

	entries, err := m.loadMaskPath(maskPath, "user")
	if err != nil {
		return err
	}

	m.userMasks = entries
	return nil
}

// loadUserUnmasks loads unmasks from /etc/portage/package.unmask.
func (m *MaskManager) loadUserUnmasks(configRoot string) error {
	unmaskPath := filepath.Join(configRoot, "package.unmask")

	entries, err := m.loadMaskPath(unmaskPath, "user")
	if err != nil {
		return err
	}

	m.userUnmasks = entries
	return nil
}

// loadMaskPath loads mask entries from a path (file or directory).
func (m *MaskManager) loadMaskPath(path, source string) ([]*MaskEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Optional file - not an error
		}
		return nil, err
	}

	if info.IsDir() {
		return m.loadMaskDir(path, source)
	}
	return m.loadMaskFile(path, source)
}

// loadMaskDir loads mask entries from all files in a directory.
func (m *MaskManager) loadMaskDir(dirPath, source string) ([]*MaskEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// Collect and sort file names (POSIX locale order)
	var fileNames []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
			continue
		}
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	var allEntries []*MaskEntry
	for _, name := range fileNames {
		filePath := filepath.Join(dirPath, name)
		fileEntries, err := m.loadMaskFile(filePath, source)
		if err != nil {
			logging.Debug("Warning: failed to load mask file %s: %v", filePath, err)
			continue
		}
		allEntries = append(allEntries, fileEntries...)
	}

	return allEntries, nil
}

// loadMaskFile loads mask entries from a single file.
func (m *MaskManager) loadMaskFile(filePath, source string) ([]*MaskEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var entries []*MaskEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for negation (unmask within mask file)
		isNegation := strings.HasPrefix(line, "-")
		if isNegation {
			line = line[1:]
		}

		// Parse the atom
		atom := config.ParseAtom(line)
		if atom.Category == "" || atom.Name == "" {
			logging.Debug("Warning: invalid mask atom %q in %s", line, filePath)
			continue
		}

		entries = append(entries, &MaskEntry{
			Atom:       atom,
			Source:     source,
			SourceFile: filePath,
			IsNegation: isNegation,
		})
	}

	return entries, scanner.Err()
}

// extractProfileName extracts a readable profile name from its path.
func extractProfileName(path string) string {
	// Try to extract relative to "profiles" directory
	if idx := strings.Index(path, "profiles/"); idx >= 0 {
		return path[idx+len("profiles/"):]
	}
	return filepath.Base(path)
}

// findRepoRoot finds the repository root by looking for "profiles" directory.
func findRepoRoot(profilePath string) string {
	current := profilePath

	for {
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}

		// Check if parent is a repository root
		profilesDir := filepath.Join(parent, "profiles")
		if info, err := os.Stat(profilesDir); err == nil && info.IsDir() {
			return parent
		}

		current = parent
	}
}

// MaskStats returns statistics about loaded masks.
type MaskStats struct {
	RepoMaskCount    int
	ProfileMaskCount int
	UserMaskCount    int
	UserUnmaskCount  int
	TotalMaskedCP    int // Unique category/package combinations masked
	TotalUnmaskedCP  int // Unique category/package combinations with unmasks
}

// GetStats returns statistics about loaded masks.
func (m *MaskManager) GetStats() MaskStats {
	return MaskStats{
		RepoMaskCount:    len(m.repoMasks),
		ProfileMaskCount: len(m.profileMasks),
		UserMaskCount:    len(m.userMasks),
		UserUnmaskCount:  len(m.userUnmasks),
		TotalMaskedCP:    len(m.maskIndex),
		TotalUnmaskedCP:  len(m.unmaskIndex),
	}
}

// GetAllMaskedPackages returns a list of all category/package combinations that have masks.
// Useful for debugging and reporting.
func (m *MaskManager) GetAllMaskedPackages() []string {
	packages := make([]string, 0, len(m.maskIndex))
	for cp := range m.maskIndex {
		packages = append(packages, cp)
	}
	sort.Strings(packages)
	return packages
}
