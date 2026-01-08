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
	"strings"
)

// Config represents Portage configuration.
type Config struct {
	// Root is the Portage configuration directory (usually /etc/portage)
	Root string

	// MakeConf contains settings from make.conf
	MakeConf *MakeConf

	// PackageUSE contains per-package USE flags
	PackageUSE map[string][]string

	// PackageMask contains masked packages
	PackageMask []string

	// PackageAcceptKeywords contains keyword overrides
	PackageAcceptKeywords map[string][]string

	// PackageLicense contains license acceptances
	PackageLicense map[string][]string
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
}

// DefaultMakeConf returns default make.conf settings.
func DefaultMakeConf() *MakeConf {
	return &MakeConf{
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
	}
}

// LoadConfig loads Portage configuration from the specified directory.
func LoadConfig(root string) (*Config, error) {
	cfg := &Config{
		Root:                  root,
		MakeConf:              DefaultMakeConf(),
		PackageUSE:            make(map[string][]string),
		PackageMask:           make([]string, 0),
		PackageAcceptKeywords: make(map[string][]string),
		PackageLicense:        make(map[string][]string),
	}

	// Load make.conf
	if err := cfg.loadMakeConf(); err != nil && !os.IsNotExist(err) {
		// Non-critical error - use defaults
		// Don't warn if file simply doesn't exist (common in tests)
		fmt.Printf("Warning: failed to load make.conf: %v\n", err)
	}

	// Load package.use
	if err := cfg.loadPackageFile("package.use", cfg.PackageUSE); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.use: %v\n", err)
	}

	// Load package.mask
	if err := cfg.loadPackageMask(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load package.mask: %v\n", err)
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

		// Parse variable assignments: VAR="value"
		if strings.Contains(line, "=") {
			c.parseMakeConfLine(line)
		}
	}

	return scanner.Err()
}

// parseMakeConfLine parses a single line from make.conf.
func (c *Config) parseMakeConfLine(line string) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}

	varName := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes
	value = strings.Trim(value, `"'`)

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
		return c.loadPackageDirectory(path, target)
	}

	// Single file
	return c.parsePackageFile(path, target)
}

// loadPackageDirectory loads all files from a package.* directory.
func (c *Config) loadPackageDirectory(dirPath string, target map[string][]string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		if err := c.parsePackageFile(filePath, target); err != nil {
			// Log error but continue with other files
			fmt.Printf("Warning: failed to parse %s: %v\n", filePath, err)
		}
	}

	return nil
}

// parsePackageFile parses a package.* file.
func (c *Config) parsePackageFile(path string, target map[string][]string) error {
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

		atom := fields[0]
		values := fields[1:]

		// Append values (don't replace - allow multiple entries)
		target[atom] = append(target[atom], values...)
	}

	return scanner.Err()
}

// loadPackageMask loads package.mask file.
func (c *Config) loadPackageMask() error {
	path := filepath.Join(c.Root, "package.mask")

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error - file is optional
		}
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

		c.PackageMask = append(c.PackageMask, line)
	}

	return scanner.Err()
}

// GetPackageUSE returns USE flags for a specific package.
func (c *Config) GetPackageUSE(atom string) []string {
	// Check exact match
	if flags, ok := c.PackageUSE[atom]; ok {
		return flags
	}

	// TODO: Check wildcard patterns (e.g., "sys-libs/*")

	return nil
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
