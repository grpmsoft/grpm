package profile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadEAPI loads the EAPI version from the "eapi" file.
func (p *Profile) loadEAPI() error {
	eapiPath := filepath.Join(p.Path, "eapi")
	content, err := os.ReadFile(eapiPath)
	if err != nil {
		return err
	}

	eapi := strings.TrimSpace(string(content))
	if eapi == "" {
		return fmt.Errorf("empty EAPI in %s", eapiPath)
	}

	p.EAPI = eapi
	return nil
}

// loadMakeDefaults loads variables from the "make.defaults" file.
//
// Format:
//
//	KEY="value"
//	USE="ssl unicode"
//	CFLAGS="-O2 -pipe"
func (p *Profile) loadMakeDefaults() error {
	makeDefaultsPath := filepath.Join(p.Path, "make.defaults")
	file, err := os.Open(makeDefaultsPath)
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

		// Parse KEY="value" or KEY=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes
		value = strings.Trim(value, `"'`)

		p.MakeDefaults[key] = value
	}

	return scanner.Err()
}

// loadUSEMask loads masked USE flags from the "use.mask" file.
//
// Format (one USE flag per line):
//
//	# Comment
//	debug
//	test
func (p *Profile) loadUSEMask() error {
	useMaskPath := filepath.Join(p.Path, "use.mask")
	flags, err := parseListFile(useMaskPath)
	if err != nil {
		return err
	}

	p.USEMask = append(p.USEMask, flags...)
	return nil
}

// loadUSEForce loads forced USE flags from the "use.force" file.
func (p *Profile) loadUSEForce() error {
	useForcePath := filepath.Join(p.Path, "use.force")
	flags, err := parseListFile(useForcePath)
	if err != nil {
		return err
	}

	p.USEForce = append(p.USEForce, flags...)
	return nil
}

// loadPackages loads system packages from the "packages" file.
//
// Format:
//
//	# Comment
//	*sys-apps/baselayout
//	*virtual/libc
//	-sys-apps/removed-package
//
// Lines starting with "*" are system packages.
// Lines starting with "-" are removed packages (not implemented yet).
func (p *Profile) loadPackages() error {
	packagesPath := filepath.Join(p.Path, "packages")
	file, err := os.Open(packagesPath)
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

		// System packages start with "*"
		if strings.HasPrefix(line, "*") {
			pkg := strings.TrimPrefix(line, "*")
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				p.Packages = append(p.Packages, pkg)
			}
		}
		// TODO: Handle "-" for removed packages
	}

	return scanner.Err()
}

// loadPackageMask loads masked packages from the "package.mask" file.
//
// Format:
//
//	# Comment
//	>=sys-libs/zlib-1.3.0
//	dev-lang/python:2.7
func (p *Profile) loadPackageMask() error {
	packageMaskPath := filepath.Join(p.Path, "package.mask")
	masks, err := parseListFile(packageMaskPath)
	if err != nil {
		return err
	}

	p.PackageMask = append(p.PackageMask, masks...)
	return nil
}

// loadPackageUnmask loads unmasked packages from the "package.unmask" file.
func (p *Profile) loadPackageUnmask() error {
	packageUnmaskPath := filepath.Join(p.Path, "package.unmask")
	unmasks, err := parseListFile(packageUnmaskPath)
	if err != nil {
		return err
	}

	p.PackageUnmask = append(p.PackageUnmask, unmasks...)
	return nil
}

// loadPackageUse loads per-package USE flags from the "package.use" file.
//
// Format:
//
//	# Comment
//	sys-libs/zlib minizip
//	app-editors/vim -python perl
func (p *Profile) loadPackageUse() error {
	packageUsePath := filepath.Join(p.Path, "package.use")
	file, err := os.Open(packageUsePath)
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

		// Parse: package flag1 flag2 ...
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		atom := fields[0]
		flags := fields[1:]

		p.PackageUse[atom] = append(p.PackageUse[atom], flags...)
	}

	return scanner.Err()
}

// loadKeywords loads package keywords from the "package.keywords" file.
//
// Format:
//
//	# Comment
//	sys-libs/zlib ~amd64
//	app-editors/vim **
func (p *Profile) loadKeywords() error {
	packageKeywordsPath := filepath.Join(p.Path, "package.keywords")
	file, err := os.Open(packageKeywordsPath)
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

		// Parse: package keyword
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		atom := fields[0]
		keyword := fields[1]

		p.Keywords[atom] = keyword
	}

	return scanner.Err()
}

// loadParents loads and parses parent profiles from the "parent" file.
//
// Format (one parent per line):
//
//	# Comment
//	gentoo:default/linux/amd64
//	:base
//	../../../path/to/profile
//
// Supported formats:
//   - Absolute repo paths: gentoo:default/linux/amd64/23.0
//   - Relative paths: ../../../base
//   - Repo-local: :base
func (p *Profile) loadParents() ([]*Profile, error) {
	parentPath := filepath.Join(p.Path, "parent")
	file, err := os.Open(parentPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	parents := make([]*Profile, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Resolve parent path
		parentProfilePath, err := p.resolveParentPath(line)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parent %s: %w", line, err)
		}

		// Load parent profile
		parentProfile, err := LoadProfile(parentProfilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load parent profile %s: %w", parentProfilePath, err)
		}

		parents = append(parents, parentProfile)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return parents, nil
}

// resolveParentPath resolves a parent profile path.
//
// Supported formats:
//   - Relative path: ../../../base -> resolved relative to current profile
//   - Repo-local: :base -> TODO: requires repository access
//   - Repo path: gentoo:default/linux -> TODO: requires repository access
func (p *Profile) resolveParentPath(parent string) (string, error) {
	// Handle relative paths
	if !strings.Contains(parent, ":") {
		absPath := filepath.Join(p.Path, parent)
		absPath = filepath.Clean(absPath)
		return absPath, nil
	}

	// Handle repo:path format (e.g., gentoo:default/linux/amd64/23.0)
	if strings.Contains(parent, ":") {
		parts := strings.SplitN(parent, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid parent format: %s", parent)
		}

		repoName := parts[0]
		profilePath := parts[1]

		// Try to find repo path
		// TODO: This requires repository manager access
		// For now, try common locations

		// If repoName is empty (e.g., ":base"), use current repo
		if repoName == "" {
			// Find repo root by looking for "profiles" directory
			repoRoot := findRepoRoot(p.Path)
			if repoRoot == "" {
				return "", fmt.Errorf("cannot find repository root for profile %s", p.Path)
			}
			return filepath.Join(repoRoot, "profiles", profilePath), nil
		}

		// Try /var/db/repos/{repo}/profiles/{profilePath}
		repoPath := filepath.Join("/var/db/repos", repoName, "profiles", profilePath)
		if _, err := os.Stat(repoPath); err == nil {
			return repoPath, nil
		}

		// Try /usr/portage/profiles/{profilePath} (old Gentoo location)
		repoPath = filepath.Join("/usr/portage", "profiles", profilePath)
		if _, err := os.Stat(repoPath); err == nil {
			return repoPath, nil
		}

		return "", fmt.Errorf("cannot find repository %s for parent %s", repoName, parent)
	}

	return "", fmt.Errorf("unsupported parent format: %s", parent)
}

// findRepoRoot finds the repository root by looking for "profiles" directory.
//
// Example:
//
//	/var/db/repos/gentoo/profiles/default/linux/amd64/23.0
//	-> /var/db/repos/gentoo
func findRepoRoot(profilePath string) string {
	current := profilePath

	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return ""
		}

		// Check if parent is a repository root
		// (contains "profiles" directory)
		profilesDir := filepath.Join(parent, "profiles")
		if info, err := os.Stat(profilesDir); err == nil && info.IsDir() {
			return parent
		}

		current = parent
	}
}

// parseListFile parses a simple list file or directory (one item per line).
// In EAPI 7+, these config paths can be directories containing multiple files.
//
// Used for: use.mask, use.force, package.mask, package.unmask
func parseListFile(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Optional file
		}
		return nil, err
	}

	if info.IsDir() {
		return parseListDir(path)
	}
	return parseListFileOnly(path)
}

// parseListDir reads all files in a directory and concatenates their contents.
// Files are sorted by name (POSIX locale order). Dotfiles and backup files are skipped.
func parseListDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Collect valid file names
	var fileNames []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip dotfiles, backup files (~), and subdirectories
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || entry.IsDir() {
			continue
		}
		fileNames = append(fileNames, name)
	}

	// Sort by filename (POSIX locale = lexicographic)
	sort.Strings(fileNames)

	// Read all files in sorted order
	var allItems []string
	for _, name := range fileNames {
		filePath := filepath.Join(dir, name)
		items, err := parseListFileOnly(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filePath, err)
		}
		allItems = append(allItems, items...)
	}

	return allItems, nil
}

// parseListFileOnly parses a single list file (not a directory).
func parseListFileOnly(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var items []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		items = append(items, line)
	}

	return items, scanner.Err()
}
