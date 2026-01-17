// Package config implements Portage configuration management.
//
// Loads and parses configuration from /etc/portage/:
//   - make.conf - global Portage settings
//   - package.use - per-package USE flags
//   - package.mask - masked packages
//   - package.accept_keywords - keyword overrides
//   - package.license - license acceptances
//
// Example:
//
//	cfg, err := config.LoadConfig("/etc/portage")
//	useFlags := cfg.GetPackageUSE("sys-libs/zlib")
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config represents Portage configuration.
type Config struct {
	// Root is the Portage configuration directory (usually /etc/portage)
	Root string

	// MakeConf contains settings from make.conf
	MakeConf *MakeConf

	// PackageUSE contains per-package USE flags (raw atom -> flags mapping)
	PackageUSE map[string][]string

	// packageUSEEntries contains parsed package.use entries for pattern matching.
	// Each entry has the parsed atom and expanded USE flags.
	packageUSEEntries []packageUSEEntry

	// PackageMask contains masked packages
	PackageMask []string

	// PackageUnmask contains unmasked packages (overrides masks)
	PackageUnmask []string

	// PackageAcceptKeywords contains keyword overrides
	PackageAcceptKeywords map[string][]string

	// PackageLicense contains license acceptances
	PackageLicense map[string][]string
}

// packageUSEEntry represents a single parsed entry from package.use.
type packageUSEEntry struct {
	// Atom is the parsed package atom with version constraints
	Atom *PackageAtom

	// Flags contains the USE flags for this entry (after USE_EXPAND expansion)
	Flags []string
}

// MakeConf represents settings from make.conf.
type MakeConf struct {
	// CFLAGS = C compiler flags
	CFLAGS string

	// CXXFLAGS = C++ compiler flags
	CXXFLAGS string

	// LDFLAGS = linker flags
	LDFLAGS string

	// MAKEOPTS = make options (e.g., "-j4")
	MAKEOPTS string

	// USE = global USE flags
	USE []string

	// ACCEPT_KEYWORDS = accepted keywords (e.g., "~amd64")
	ACCEPT_KEYWORDS []string

	// ACCEPT_LICENSE = accepted licenses
	ACCEPT_LICENSE []string

	// FEATURES = Portage features
	FEATURES []string

	// PORTDIR = Portage tree directory
	PORTDIR string

	// DISTDIR = distfiles directory
	DISTDIR string

	// PKGDIR = binary packages directory
	PKGDIR string

	// PORT_LOGDIR = build log directory
	PORT_LOGDIR string

	// PORTAGE_TMPDIR = temporary build directory
	PORTAGE_TMPDIR string

	// GENTOO_MIRRORS = list of Gentoo mirrors
	GENTOO_MIRRORS []string

	// Variables stores all variables dynamically (including custom ones).
	// This enables ${VAR} expansion and access to arbitrary variables.
	Variables map[string]string
}

// DefaultMakeConf returns default make.conf settings.
func DefaultMakeConf() *MakeConf {
	mc := &MakeConf{
		CFLAGS:          "-O2 -pipe",
		CXXFLAGS:        "${CFLAGS}",
		LDFLAGS:         "",
		MAKEOPTS:        "-j1",
		USE:             []string{},
		ACCEPT_KEYWORDS: []string{},
		ACCEPT_LICENSE:  []string{"*"},
		FEATURES:        []string{"sandbox", "usersandbox", "userpriv"},
		PORTDIR:         "/var/db/repos/gentoo",
		DISTDIR:         "/var/cache/distfiles",
		PKGDIR:          "/var/cache/binpkgs",
		PORT_LOGDIR:     "/var/log/portage",
		PORTAGE_TMPDIR:  "/var/tmp/portage",
		GENTOO_MIRRORS:  []string{},
		Variables:       make(map[string]string),
	}
	// Initialize Variables map with defaults
	mc.Variables["CFLAGS"] = mc.CFLAGS
	mc.Variables["CXXFLAGS"] = mc.CXXFLAGS
	mc.Variables["LDFLAGS"] = mc.LDFLAGS
	mc.Variables["MAKEOPTS"] = mc.MAKEOPTS
	mc.Variables["PORTDIR"] = mc.PORTDIR
	mc.Variables["DISTDIR"] = mc.DISTDIR
	mc.Variables["PKGDIR"] = mc.PKGDIR
	mc.Variables["PORT_LOGDIR"] = mc.PORT_LOGDIR
	mc.Variables["PORTAGE_TMPDIR"] = mc.PORTAGE_TMPDIR
	return mc
}

// LoadConfig loads Portage configuration from the specified directory.
func LoadConfig(root string) (*Config, error) {
	cfg := &Config{
		Root:                  root,
		MakeConf:              DefaultMakeConf(),
		PackageUSE:            make(map[string][]string),
		packageUSEEntries:     make([]packageUSEEntry, 0),
		PackageMask:           make([]string, 0),
		PackageUnmask:         make([]string, 0),
		PackageAcceptKeywords: make(map[string][]string),
		PackageLicense:        make(map[string][]string),
	}

	// Load make.conf
	if err := cfg.loadMakeConf(); err != nil && !os.IsNotExist(err) {
		// Non-critical error - use defaults
		// Don't warn if file simply doesn't exist (common in tests)
		fmt.Printf("Warning: failed to load make.conf: %v\n", err)
	}

	// Load package.use (with pattern matching support)
	if err := cfg.loadPackageUSE(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.use: %v\n", err)
	}

	// Load package.mask
	if err := cfg.loadPackageMask(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.mask: %v\n", err)
	}

	// Load package.unmask
	if err := cfg.loadPackageUnmask(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.unmask: %v\n", err)
	}

	// Load package.accept_keywords
	if err := cfg.loadPackageFile("package.accept_keywords", cfg.PackageAcceptKeywords); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.accept_keywords: %v\n", err)
	}

	// Load package.license
	if err := cfg.loadPackageFile("package.license", cfg.PackageLicense); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.license: %v\n", err)
	}

	return cfg, nil
}

// loadMakeConf loads settings from make.conf.
func (c *Config) loadMakeConf() error {
	path := filepath.Join(c.Root, "make.conf")
	return c.loadMakeConfFile(path, make(map[string]bool))
}

// loadMakeConfFile loads a make.conf file with source directive support.
// The visited map prevents infinite loops from circular source references.
func (c *Config) loadMakeConfFile(path string, visited map[string]bool) error {
	// Prevent circular includes
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if visited[absPath] {
		return nil // Already processed, skip to prevent infinite loop
	}
	visited[absPath] = true

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle source directive: source /path/to/file or . /path/to/file
		if strings.HasPrefix(line, "source ") || strings.HasPrefix(line, ". ") {
			sourcePath := strings.TrimPrefix(line, "source ")
			sourcePath = strings.TrimPrefix(sourcePath, ". ")
			sourcePath = strings.TrimSpace(sourcePath)
			// Expand variables in the source path
			sourcePath = c.varexpand(sourcePath)
			// Handle relative paths
			if !filepath.IsAbs(sourcePath) {
				sourcePath = filepath.Join(filepath.Dir(path), sourcePath)
			}
			// Recursively load the sourced file
			if err := c.loadMakeConfFile(sourcePath, visited); err != nil {
				// Non-critical: log warning but continue
				if !os.IsNotExist(err) {
					fmt.Printf("Warning: failed to source %s: %v\n", sourcePath, err)
				}
			}
			continue
		}

		// Parse variable assignments: VAR="value"
		if strings.Contains(line, "=") {
			c.parseMakeConfLine(line)
		}
	}

	return scanner.Err()
}

// varexpand expands ${VAR} and $VAR references in a string.
// This matches Portage's varexpand() behavior from lib/portage/util/__init__.py.
// Note: Pattern substitution (${VAR/a/b}) is NOT supported - Portage doesn't support it either.
func (c *Config) varexpand(value string) string {
	if c.MakeConf == nil || c.MakeConf.Variables == nil {
		return value
	}

	result := value
	// Handle ${VAR} syntax
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start

		varName := result[start+2 : end]
		varValue := c.MakeConf.Variables[varName]
		result = result[:start] + varValue + result[end+1:]
	}

	// Handle $VAR syntax (without braces)
	// Must be done after ${VAR} to avoid conflicts
	for {
		start := strings.Index(result, "$")
		if start == -1 {
			break
		}
		// Skip if it's ${ which was already handled
		if start+1 < len(result) && result[start+1] == '{' {
			// This shouldn't happen after the loop above, but be safe
			break
		}
		// Find end of variable name (alphanumeric and underscore)
		end := start + 1
		for end < len(result) && (isAlphaNumeric(result[end]) || result[end] == '_') {
			end++
		}
		if end == start+1 {
			// No valid variable name after $
			break
		}
		varName := result[start+1 : end]
		varValue := c.MakeConf.Variables[varName]
		result = result[:start] + varValue + result[end:]
	}

	return result
}

// isAlphaNumeric returns true if the byte is a letter, digit, or underscore.
func isAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// parseMakeConfLine parses a single line from make.conf.
// It expands ${VAR} references and stores all variables in the Variables map.
func (c *Config) parseMakeConfLine(line string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}

	varName := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes
	value = strings.Trim(value, `"'`)

	// Expand ${VAR} references using previously defined variables
	value = c.varexpand(value)

	// Store in Variables map (for all variables, including custom ones)
	if c.MakeConf.Variables == nil {
		c.MakeConf.Variables = make(map[string]string)
	}
	c.MakeConf.Variables[varName] = value

	// Also update typed fields for known variables
	switch varName {
	case "CFLAGS":
		c.MakeConf.CFLAGS = value
	case "CXXFLAGS":
		c.MakeConf.CXXFLAGS = value
	case "LDFLAGS":
		c.MakeConf.LDFLAGS = value
	case "MAKEOPTS":
		c.MakeConf.MAKEOPTS = value
	case "USE":
		c.MakeConf.USE = strings.Fields(value)
	case "ACCEPT_KEYWORDS":
		c.MakeConf.ACCEPT_KEYWORDS = strings.Fields(value)
	case "ACCEPT_LICENSE":
		c.MakeConf.ACCEPT_LICENSE = strings.Fields(value)
	case "FEATURES":
		c.MakeConf.FEATURES = strings.Fields(value)
	case "PORTDIR":
		c.MakeConf.PORTDIR = value
	case "DISTDIR":
		c.MakeConf.DISTDIR = value
	case "PKGDIR":
		c.MakeConf.PKGDIR = value
	case "PORT_LOGDIR":
		c.MakeConf.PORT_LOGDIR = value
	case "PORTAGE_TMPDIR":
		c.MakeConf.PORTAGE_TMPDIR = value
	case "GENTOO_MIRRORS":
		c.MakeConf.GENTOO_MIRRORS = strings.Fields(value)
	}
}

// loadPackageFile loads a package.* file (package.use, package.accept_keywords, etc).
//
// Format: <atom> <values>
// Example: sys-libs/zlib ssl -debug
func (c *Config) loadPackageFile(filename string, target map[string][]string) error {
	return c.loadPackageFileWithParsing(filename, target, false)
}

// loadPackageUSE loads package.use with pattern parsing support.
func (c *Config) loadPackageUSE() error {
	return c.loadPackageFileWithParsing("package.use", c.PackageUSE, true)
}

// loadPackageFileWithParsing loads a package.* file with optional atom parsing.
// When parseAtoms is true, entries are also stored in packageUSEEntries for pattern matching.
func (c *Config) loadPackageFileWithParsing(filename string, target map[string][]string, parseAtoms bool) error {
	path := filepath.Join(c.Root, filename)

	// Check if it's a file or directory
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error - file is optional
		}
		return err
	}

	if info.IsDir() {
		// Directory - load all files inside
		return c.loadPackageDirectory(path, target, parseAtoms)
	}

	// Single file
	return c.parsePackageFile(path, target, parseAtoms)
}

// loadPackageDirectory loads all files from a package.* directory.
func (c *Config) loadPackageDirectory(dirPath string, target map[string][]string, parseAtoms bool) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		if err := c.parsePackageFile(filePath, target, parseAtoms); err != nil {
			// Log error but continue with other files
			fmt.Printf("Warning: failed to parse %s: %v\n", filePath, err)
		}
	}

	return nil
}

// parsePackageFile parses a package.* file.
func (c *Config) parsePackageFile(path string, target map[string][]string, parseAtoms bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse: <atom> <values>
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		atomStr := fields[0]
		values := fields[1:]

		// Expand USE_EXPAND syntax (e.g., "CPU_FLAGS_X86: avx2 sse4_2")
		expandedValues := expandUSEExpand(values)

		// Append values (don't replace - allow multiple entries)
		target[atomStr] = append(target[atomStr], expandedValues...)

		// For package.use, also store parsed entry for pattern matching
		if parseAtoms {
			parsedAtom := ParseAtom(atomStr)
			c.packageUSEEntries = append(c.packageUSEEntries, packageUSEEntry{
				Atom:  parsedAtom,
				Flags: expandedValues,
			})
		}
	}

	return scanner.Err()
}

// loadPackageMask loads package.mask file or directory.
// In EAPI 7+, package.mask can be a directory containing multiple files.
func (c *Config) loadPackageMask() error {
	path := filepath.Join(c.Root, "package.mask")

	lines, err := readPortageConfigPath(path)
	if err != nil {
		return err
	}

	if lines == nil {
		lines = []string{} // Ensure non-nil slice
	}
	c.PackageMask = lines
	return nil
}

// loadPackageUnmask loads package.unmask file or directory.
// In EAPI 7+, package.unmask can be a directory containing multiple files.
// Unmasks override masks - if a package is unmasked, it will not be filtered.
func (c *Config) loadPackageUnmask() error {
	path := filepath.Join(c.Root, "package.unmask")

	lines, err := readPortageConfigPath(path)
	if err != nil {
		return err
	}

	if lines == nil {
		lines = []string{} // Ensure non-nil slice
	}
	c.PackageUnmask = lines
	return nil
}

// readPortageConfigPath reads lines from a Portage config path.
// The path can be either a regular file or a directory (EAPI 7+).
// For directories, all files are read recursively (excluding dotfiles and backups).
// Files are sorted by name in POSIX locale order before concatenation.
func readPortageConfigPath(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error - path is optional
		}
		return nil, err
	}

	if info.IsDir() {
		return readPortageConfigDir(path)
	}
	return readPortageConfigFile(path)
}

// readPortageConfigDir reads all files in a Portage config directory.
// Files starting with "." or ending with "~" are skipped.
// Subdirectories are ignored (not recursed into).
func readPortageConfigDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Collect and sort file names
	var fileNames []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip dotfiles, backup files, and directories
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || entry.IsDir() {
			continue
		}
		fileNames = append(fileNames, name)
	}

	// Sort by filename (POSIX locale = lexicographic)
	sort.Strings(fileNames)

	// Read all files in order
	var allLines []string
	for _, name := range fileNames {
		filePath := filepath.Join(dir, name)
		lines, err := readPortageConfigFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filePath, err)
		}
		allLines = append(allLines, lines...)
	}

	return allLines, nil
}

// readPortageConfigFile reads lines from a single Portage config file.
func readPortageConfigFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lines = append(lines, line)
	}

	return lines, scanner.Err()
}

// GetPackageUSE returns USE flags for a specific package atom (exact match only).
// For version-aware matching with pattern support, use GetPackageUSEForPackage.
func (c *Config) GetPackageUSE(atom string) []string {
	// Check exact match
	if flags, ok := c.PackageUSE[atom]; ok {
		return flags
	}

	return nil
}

// GetPackageUSEForPackage returns USE flags for a package with full pattern matching.
// It matches against all package.use entries and returns flags from the most specific
// matching patterns, with more specific patterns overriding less specific ones.
//
// Parameters:
//   - category: package category (e.g., "app-misc", "sys-libs")
//   - name: package name (e.g., "hello", "zlib")
//   - version: package version (e.g., "2.10", "1.2.3_alpha1-r2")
//   - slot: package slot (e.g., "0", "0/1.22")
//
// Pattern priority (highest to lowest):
//   - Exact version match (=category/package-version)
//   - Any revision (~category/package-version)
//   - Version prefix (=category/package-version*)
//   - Slot-specific (category/package:slot)
//   - Version range (>=/<=/>/< category/package-version)
//   - Package only (category/package)
//   - Wildcard with slot (*/*:slot, category/*:slot)
//   - Wildcard without slot (*/*:, category/*)
//
// Example:
//
//	flags := cfg.GetPackageUSEForPackage("app-misc", "hello", "2.10", "0")
func (c *Config) GetPackageUSEForPackage(category, name, version, slot string) []string {
	if len(c.packageUSEEntries) == 0 {
		return nil
	}

	// Collect all matching entries with their specificities
	type matchedEntry struct {
		specificity AtomSpecificity
		flags       []string
		index       int // Original order for stable sorting
	}

	var matches []matchedEntry

	for i, entry := range c.packageUSEEntries {
		if entry.Atom.Matches(category, name, version, slot) {
			matches = append(matches, matchedEntry{
				specificity: entry.Atom.GetSpecificity(),
				flags:       entry.Flags,
				index:       i,
			})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort matches by specificity (descending), then by original order (ascending)
	// This ensures more specific patterns override less specific ones,
	// and later entries of same specificity override earlier ones.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].specificity != matches[j].specificity {
			return matches[i].specificity > matches[j].specificity
		}
		return matches[i].index < matches[j].index
	})

	// Build result: accumulate flags from least specific to most specific
	// This way, more specific patterns override less specific ones
	flagSet := make(map[string]bool)     // Track enabled flags
	negativeSet := make(map[string]bool) // Track explicitly disabled flags

	// Process from least specific to most specific (reverse order)
	for i := len(matches) - 1; i >= 0; i-- {
		for _, flag := range matches[i].flags {
			if strings.HasPrefix(flag, "-") {
				baseFlag := flag[1:]
				negativeSet[baseFlag] = true
				delete(flagSet, baseFlag)
			} else {
				flagSet[flag] = true
				delete(negativeSet, flag)
			}
		}
	}

	// Build final result preserving order from most specific matches
	var result []string
	seen := make(map[string]bool)

	// Add flags from most specific to least specific, respecting final state
	for _, m := range matches {
		for _, flag := range m.flags {
			baseFlag := flag
			if strings.HasPrefix(flag, "-") {
				baseFlag = flag[1:]
			}
			if seen[baseFlag] {
				continue
			}
			seen[baseFlag] = true

			// Use the final state
			if flagSet[baseFlag] {
				result = append(result, baseFlag)
			} else if negativeSet[baseFlag] {
				result = append(result, "-"+baseFlag)
			}
		}
	}

	return result
}

// IsMasked returns true if the package is masked.
func (c *Config) IsMasked(atom string) bool {
	for _, masked := range c.PackageMask {
		if masked == atom {
			return true
		}
		// TODO: Support version ranges and wildcards
	}
	return false
}

// GetAcceptKeywords returns accepted keywords for a package.
func (c *Config) GetAcceptKeywords(atom string) []string {
	if keywords, ok := c.PackageAcceptKeywords[atom]; ok {
		return keywords
	}
	return nil
}

// GetGlobalUSE returns global USE flags from make.conf.
func (c *Config) GetGlobalUSE() []string {
	return c.MakeConf.USE
}

// GetGentooMirrors returns configured Gentoo mirrors from make.conf.
//
// If GENTOO_MIRRORS is not set or empty, returns nil.
// The caller should fall back to default mirrors if the result is empty.
func (c *Config) GetGentooMirrors() []string {
	if c.MakeConf == nil || len(c.MakeConf.GENTOO_MIRRORS) == 0 {
		return nil
	}
	// Return a copy to prevent external mutation
	mirrors := make([]string, len(c.MakeConf.GENTOO_MIRRORS))
	copy(mirrors, c.MakeConf.GENTOO_MIRRORS)
	return mirrors
}

// GetDistDir returns the configured distfiles directory.
//
// If DISTDIR is not set, returns the default "/var/cache/distfiles".
func (c *Config) GetDistDir() string {
	if c.MakeConf == nil || c.MakeConf.DISTDIR == "" {
		return "/var/cache/distfiles"
	}
	return c.MakeConf.DISTDIR
}

// GetPortDir returns the configured Portage repository directory.
//
// If PORTDIR is not set, returns the default "/var/db/repos/gentoo".
func (c *Config) GetPortDir() string {
	if c.MakeConf == nil || c.MakeConf.PORTDIR == "" {
		return "/var/db/repos/gentoo"
	}
	return c.MakeConf.PORTDIR
}

// GetMakeOpts returns the configured make options.
//
// If MAKEOPTS is not set, returns the default "-j1".
func (c *Config) GetMakeOpts() string {
	if c.MakeConf == nil || c.MakeConf.MAKEOPTS == "" {
		return "-j1"
	}
	return c.MakeConf.MAKEOPTS
}

// GetVariable returns any variable from make.conf by name.
// This provides access to both standard Portage variables and custom user variables.
// Returns empty string if variable is not set.
//
// Example:
//
//	cfg.GetVariable("CFLAGS")           // "-O2 -pipe -march=native"
//	cfg.GetVariable("MY_CUSTOM_VAR")    // Custom user variable
func (c *Config) GetVariable(name string) string {
	if c.MakeConf == nil || c.MakeConf.Variables == nil {
		return ""
	}
	return c.MakeConf.Variables[name]
}

// GetMainRepoLocation returns the main repository location using Portage's fallback chain:
//  1. repos.conf -> [DEFAULT] main-repo -> [repo_name] location
//  2. PORTDIR from make.conf
//  3. Auto-detect: /var/db/repos/gentoo or /usr/portage
//
// This method provides full Portage compatibility for repository location detection.
func (c *Config) GetMainRepoLocation() string {
	// 1. Try repos.conf
	reposConfPath := filepath.Join(c.Root, "repos.conf")
	info, err := os.Stat(reposConfPath)
	if err == nil {
		location := c.loadReposConfMainLocation(reposConfPath, info.IsDir())
		if location != "" {
			return location
		}
	}

	// 2. Try PORTDIR from make.conf
	if portdir := c.GetPortDir(); portdir != "" && portdir != "/var/db/repos/gentoo" {
		// Only use PORTDIR if it was explicitly set (not the default)
		if c.MakeConf != nil && c.MakeConf.Variables != nil {
			if _, exists := c.MakeConf.Variables["PORTDIR"]; exists {
				return portdir
			}
		}
	}

	// 3. Auto-detect
	modernPath := "/var/db/repos/gentoo"
	if _, err := os.Stat(modernPath); err == nil {
		return modernPath
	}

	legacyPath := "/usr/portage"
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}

	return modernPath
}

// reposConfState holds parsed state from repos.conf files.
type reposConfState struct {
	mainRepo string
	repos    map[string]string // repo name -> location
}

// loadReposConfMainLocation parses repos.conf to find the main repo location.
func (c *Config) loadReposConfMainLocation(path string, isDir bool) string {
	state := &reposConfState{repos: make(map[string]string)}

	if isDir {
		c.loadReposConfDir(path, state)
	} else {
		c.parseReposConfFile(path, state)
	}

	// Find main repo location
	mainRepo := state.mainRepo
	if mainRepo == "" {
		mainRepo = "gentoo"
	}
	if loc, ok := state.repos[mainRepo]; ok {
		return loc
	}

	return ""
}

// loadReposConfDir loads all .conf files from a repos.conf directory.
func (c *Config) loadReposConfDir(dirPath string, state *reposConfState) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasSuffix(name, ".conf") || !strings.Contains(name, ".") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, name := range files {
		c.parseReposConfFile(filepath.Join(dirPath, name), state)
	}
}

// parseReposConfFile parses a single repos.conf file.
func (c *Config) parseReposConfFile(filePath string, state *reposConfState) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			continue
		}

		c.parseReposConfKeyValue(line, currentSection, state)
	}
}

// parseReposConfKeyValue parses a key=value line from repos.conf.
func (c *Config) parseReposConfKeyValue(line, section string, state *reposConfState) {
	if !strings.Contains(line, "=") {
		return
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if section == "DEFAULT" && key == "main-repo" {
		state.mainRepo = value
	} else if section != "" && section != "DEFAULT" && key == "location" {
		state.repos[section] = value
	}
}
