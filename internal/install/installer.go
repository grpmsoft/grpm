// Package install implements package installation engine.
//
// This module handles actual package installation to the filesystem,
// including file merging, collision detection, and database updates.
//
// Example:
//
//	installer := install.NewInstaller("/", db, cfg)
//	err := installer.Install(pkg, install.InstallOptions{})
package install

import (
	"fmt"
	"os"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// Installer manages package installation.
type Installer struct {
	// Root is the installation root (usually "/")
	Root string

	// DB is the package database for tracking installed packages
	DB *state.PackageDatabase

	// DryRun mode - don't actually install, just simulate
	DryRun bool

	// Verbose enables detailed logging
	Verbose bool

	// OnProgress is called during installation with status updates
	OnProgress func(status string)
}

// InstallOptions configures package installation behavior.
type InstallOptions struct {
	// Replace allows replacing an existing package
	Replace bool

	// Force ignores file collisions
	Force bool

	// KeepWork preserves the work directory after installation
	KeepWork bool

	// Pretend performs a dry-run without actual changes
	Pretend bool

	// FetchOnly only downloads, doesn't install
	FetchOnly bool

	// SkipHooks skips pre/post install hooks
	SkipHooks bool

	// WorkDir is the build/work directory (e.g., /var/tmp/portage/...)
	WorkDir string
}

// UninstallOptions configures package removal behavior.
type UninstallOptions struct {
	// Force removes even if other packages depend on this
	Force bool

	// Pretend performs a dry-run
	Pretend bool

	// CleanDepends also removes unused dependencies
	CleanDepends bool

	// SkipHooks skips pre/post remove hooks
	SkipHooks bool
}

// NewInstaller creates a new package installer.
//
// Parameters:
//   - root: Installation root (usually "/")
//   - db: Package database for tracking installed packages
//
// Example:
//
//	installer := NewInstaller("/", db)
func NewInstaller(root string, db *state.PackageDatabase) *Installer {
	return &Installer{
		Root:       root,
		DB:         db,
		DryRun:     false,
		Verbose:    false,
		OnProgress: nil,
	}
}

// Install installs a package to the system.
//
// The installation process:
//  1. Check if package already installed
//  2. Detect file collisions
//  3. Run pre-install hooks
//  4. Install files from WorkDir to Root
//  5. Update package database
//  6. Run post-install hooks
//
// Example:
//
//	opts := InstallOptions{
//	    WorkDir: "/var/tmp/portage/sys-libs/zlib-1.2.13",
//	    Replace: true,
//	}
//	err := installer.Install(pkg, opts)
func (i *Installer) Install(p *pkg.Package, opts InstallOptions) error {
	if p == nil {
		return fmt.Errorf("package is nil")
	}

	i.progress("Installing %s-%s", p.Name, p.Version)

	// Validate options
	if opts.WorkDir == "" {
		return fmt.Errorf("WorkDir is required for installation")
	}

	// Check if work directory exists
	if _, err := os.Stat(opts.WorkDir); err != nil {
		return fmt.Errorf("work directory does not exist: %s", opts.WorkDir)
	}

	// Check if already installed
	atom := fmt.Sprintf("%s-%s", p.Name, p.Version)
	existingAtom := i.findInstalledVersion(p.Name)

	if existingAtom != "" && !opts.Replace {
		return fmt.Errorf("package already installed: %s (use --replace or -R)", existingAtom)
	}

	// Pretend mode - just validate and return
	if opts.Pretend || i.DryRun {
		if existingAtom != "" {
			i.progress("[pretend] Would replace %s with %s", existingAtom, atom)
		} else {
			i.progress("[pretend] Would install %s", atom)
		}
		return nil
	}

	// If replacing, unmerge old version first
	// This ensures clean replacement without file collisions from old package
	if existingAtom != "" && opts.Replace {
		i.progress("Replacing %s with %s", existingAtom, atom)

		// Unmerge old package (keep config files via CONFIG_PROTECT)
		if err := i.Uninstall(existingAtom, UninstallOptions{
			SkipHooks: opts.SkipHooks,
		}); err != nil {
			return fmt.Errorf("failed to remove old package %s: %w", existingAtom, err)
		}
	}

	// Create merger for this package
	merger := NewMerger(i, p, opts)

	// Execute merge process
	if err := merger.Merge(); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	i.progress("Successfully installed %s", atom)

	return nil
}

// Uninstall removes a package from the system.
//
// The removal process:
//  1. Check if package is installed
//  2. Check for reverse dependencies (unless Force)
//  3. Run pre-remove hooks
//  4. Remove files from filesystem
//  5. Update package database
//  6. Run post-remove hooks
//
// Example:
//
//	err := installer.Uninstall("sys-libs/zlib-1.2.13", UninstallOptions{})
func (i *Installer) Uninstall(atom string, opts UninstallOptions) error {
	i.progress("Uninstalling %s", atom)

	// Check if installed
	installedPkg, err := i.DB.Get(atom)
	if err != nil {
		return fmt.Errorf("package not installed: %s", atom)
	}

	// Pretend mode
	if opts.Pretend || i.DryRun {
		i.progress("[pretend] Would uninstall %s", atom)
		return nil
	}

	// Create unmerger
	unmerger := NewUnmerger(i, installedPkg, opts)

	// Execute unmerge process
	if err := unmerger.Unmerge(); err != nil {
		return fmt.Errorf("unmerge failed: %w", err)
	}

	i.progress("Successfully uninstalled %s", atom)

	return nil
}

// Upgrade upgrades a package to a new version.
//
// This is essentially: uninstall old version + install new version,
// but with optimizations to avoid unnecessary file operations.
//
// Example:
//
//	err := installer.Upgrade("sys-libs/zlib-1.2.11", newPkg, InstallOptions{
//	    WorkDir: "/var/tmp/portage/sys-libs/zlib-1.2.13",
//	})
func (i *Installer) Upgrade(oldAtom string, newPkg *pkg.Package, opts InstallOptions) error {
	i.progress("Upgrading %s to %s-%s", oldAtom, newPkg.Name, newPkg.Version)

	// Check old package exists
	_, err := i.DB.Get(oldAtom)
	if err != nil {
		return fmt.Errorf("old package not installed: %s", oldAtom)
	}

	// Pretend mode
	if opts.Pretend || i.DryRun {
		i.progress("[pretend] Would upgrade %s to %s-%s", oldAtom, newPkg.Name, newPkg.Version)
		return nil
	}

	// For now, just uninstall old and install new
	// TODO: Optimize by detecting which files changed
	if err := i.Uninstall(oldAtom, UninstallOptions{}); err != nil {
		return fmt.Errorf("failed to remove old package: %w", err)
	}

	if err := i.Install(newPkg, opts); err != nil {
		// Try to rollback - reinstall old package
		// TODO: Implement proper rollback mechanism
		return fmt.Errorf("failed to install new package: %w", err)
	}

	return nil
}

// Verify verifies installed package integrity.
//
// Checks:
//   - All files are present
//   - File hashes match
//   - File permissions are correct
//
// Returns a list of problems found.
func (i *Installer) Verify(atom string) ([]string, error) {
	installedPkg, err := i.DB.Get(atom)
	if err != nil {
		return nil, fmt.Errorf("package not installed: %s", atom)
	}

	problems := make([]string, 0)

	// Check each file
	for _, file := range installedPkg.Files {
		fullPath := i.Root + file.Path

		// Check if file exists
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				problems = append(problems, fmt.Sprintf("missing: %s", file.Path))
			} else {
				problems = append(problems, fmt.Sprintf("error accessing %s: %v", file.Path, err))
			}
			continue
		}

		// Check file type
		switch file.Type {
		case state.FileTypeRegular:
			if !info.Mode().IsRegular() {
				problems = append(problems, fmt.Sprintf("wrong type: %s (expected regular file)", file.Path))
			}
		case state.FileTypeDirectory:
			if !info.IsDir() {
				problems = append(problems, fmt.Sprintf("wrong type: %s (expected directory)", file.Path))
			}
		case state.FileTypeSymlink:
			if info.Mode()&os.ModeSymlink == 0 {
				problems = append(problems, fmt.Sprintf("wrong type: %s (expected symlink)", file.Path))
			}
		}

		// TODO: Check file hash for regular files
		// TODO: Check file permissions
	}

	return problems, nil
}

// List returns a list of files owned by a package.
func (i *Installer) List(atom string) ([]string, error) {
	installedPkg, err := i.DB.Get(atom)
	if err != nil {
		return nil, fmt.Errorf("package not installed: %s", atom)
	}

	files := make([]string, len(installedPkg.Files))
	for idx, file := range installedPkg.Files {
		files[idx] = file.Path
	}

	return files, nil
}

// progress reports installation progress.
func (i *Installer) progress(format string, args ...interface{}) {
	if i.OnProgress != nil {
		i.OnProgress(fmt.Sprintf(format, args...))
	}

	if i.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}

// findInstalledVersion finds any installed version of a package by name.
//
// Returns the full atom (category/name-version) if found, empty string otherwise.
// This is used to detect if a package needs replacement during installation.
//
// Example:
//
//	atom := installer.findInstalledVersion("app-misc/hello")
//	// Returns "app-misc/hello-2.12" if that version is installed
func (i *Installer) findInstalledVersion(packageName string) string {
	if i.DB == nil {
		return ""
	}

	// List all installed packages and find one with matching name
	packages := i.DB.List()
	for _, pkg := range packages {
		// Extract package name from atom and compare
		// Use same logic as collision.go extractPackageName
		atom := fmt.Sprintf("%s-%s", pkg.Package.Name, pkg.Package.Version)
		if extractPackageNameFromAtom(atom) == packageName {
			return atom
		}
	}

	return ""
}

// extractPackageNameFromAtom extracts package name (category/name) from atom.
//
// This is a copy of the logic in collision.go for use in installer.go.
// We duplicate rather than export to keep collision.go's internal API clean.
func extractPackageNameFromAtom(atom string) string {
	// Find category separator
	slashIdx := -1
	for idx, c := range atom {
		if c == '/' {
			slashIdx = idx
			break
		}
	}
	if slashIdx == -1 {
		return atom
	}

	category := atom[:slashIdx]
	rest := atom[slashIdx+1:]

	// Find version separator (first dash followed by digit)
	for i := 0; i < len(rest)-1; i++ {
		if rest[i] == '-' && i+1 < len(rest) && rest[i+1] >= '0' && rest[i+1] <= '9' {
			return category + "/" + rest[:i]
		}
	}

	// No version found, return as-is
	return atom
}
