package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/grpmsoft/grpm/internal/state"
)

// Unmerger handles package removal (uninstallation).
type Unmerger struct {
	installer *Installer
	pkg       *state.InstalledPackage
	opts      UninstallOptions

	// removedFiles tracks files removed during unmerge
	removedFiles []string
}

// NewUnmerger creates a new unmerger.
func NewUnmerger(installer *Installer, pkg *state.InstalledPackage, opts UninstallOptions) *Unmerger {
	return &Unmerger{
		installer:    installer,
		pkg:          pkg,
		opts:         opts,
		removedFiles: make([]string, 0),
	}
}

// Unmerge performs package removal.
//
// Steps:
//  1. Run pre-remove hooks
//  2. Remove files in reverse order (to handle dependencies)
//  3. Remove empty directories
//  4. Update package database
//  5. Run post-remove hooks
func (u *Unmerger) Unmerge() error {
	atom := fmt.Sprintf("%s-%s", u.pkg.Package.Name, u.pkg.Package.Version)
	u.installer.progress("Unmerging %s", atom)

	// Step 1: Run pre-remove hooks
	if !u.opts.SkipHooks {
		if err := u.runPreRemoveHooks(); err != nil {
			return fmt.Errorf("pre-remove hooks failed: %w", err)
		}
	}

	// Step 2: Remove files
	if err := u.removeFiles(); err != nil {
		return fmt.Errorf("file removal failed: %w", err)
	}

	// Step 3: Remove empty directories
	if err := u.removeEmptyDirectories(); err != nil {
		// Non-critical error
		u.installer.progress("Warning: failed to remove some directories: %v", err)
	}

	// Step 4: Update database
	if err := u.updateDatabase(); err != nil {
		return fmt.Errorf("database update failed: %w", err)
	}

	// Step 5: Run post-remove hooks
	if !u.opts.SkipHooks {
		if err := u.runPostRemoveHooks(); err != nil {
			// Post-remove hooks are not critical
			u.installer.progress("Warning: post-remove hooks failed: %v", err)
		}
	}

	return nil
}

// removeFiles removes all files owned by the package.
func (u *Unmerger) removeFiles() error {
	u.installer.progress("Removing files (%d total)", len(u.pkg.Files))

	// Sort files by path length (longest first) to remove nested files before parents
	sortedFiles := make([]state.InstalledFile, len(u.pkg.Files))
	copy(sortedFiles, u.pkg.Files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return len(sortedFiles[i].Path) > len(sortedFiles[j].Path)
	})

	for _, file := range sortedFiles {
		fullPath := filepath.Join(u.installer.Root, file.Path)

		// Skip if file doesn't exist
		if _, err := os.Lstat(fullPath); err != nil {
			if os.IsNotExist(err) {
				u.installer.progress("  skip  %s (already removed)", file.Path)
				continue
			}
			return fmt.Errorf("failed to stat %s: %w", file.Path, err)
		}

		// Remove based on file type
		switch file.Type {
		case state.FileTypeRegular:
			if err := u.removeRegularFile(fullPath, file.Path); err != nil {
				return err
			}

		case state.FileTypeSymlink:
			if err := u.removeSymlink(fullPath, file.Path); err != nil {
				return err
			}

		case state.FileTypeDirectory:
			// Directories are handled in removeEmptyDirectories()
			continue

		default:
			u.installer.progress("  skip  %s (unknown type)", file.Path)
		}

		u.removedFiles = append(u.removedFiles, file.Path)
	}

	return nil
}

// removeRegularFile removes a regular file.
func (u *Unmerger) removeRegularFile(fullPath, relPath string) error {
	u.installer.progress("  rm   %s", relPath)

	// Check if file was modified (hash changed)
	if u.pkg.Package != nil {
		// Find file in installed files
		for _, f := range u.pkg.Files {
			if f.Path == relPath && f.Type == state.FileTypeRegular && f.Hash != "" {
				// Calculate current hash
				merger := &Merger{installer: u.installer}
				currentHash, err := merger.calculateHash(fullPath)
				if err == nil && currentHash != f.Hash {
					// File was modified
					u.installer.progress("  warn  %s (modified, backing up)", relPath)
					backupPath := fullPath + ".grpm-backup"
					if err := os.Rename(fullPath, backupPath); err != nil {
						return fmt.Errorf("failed to backup %s: %w", relPath, err)
					}
					return nil
				}
			}
		}
	}

	// Remove file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", relPath, err)
	}

	return nil
}

// removeSymlink removes a symbolic link.
func (u *Unmerger) removeSymlink(fullPath, relPath string) error {
	u.installer.progress("  rm   %s (symlink)", relPath)

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to remove symlink %s: %w", relPath, err)
	}

	return nil
}

// removeEmptyDirectories removes empty directories left by package removal.
func (u *Unmerger) removeEmptyDirectories() error {
	// Collect all directories owned by this package
	directories := make([]string, 0)
	for _, file := range u.pkg.Files {
		if file.Type == state.FileTypeDirectory {
			directories = append(directories, file.Path)
		}
	}

	// Sort by path length (longest first) to remove nested dirs first
	sort.Slice(directories, func(i, j int) bool {
		return len(directories[i]) > len(directories[j])
	})

	for _, dir := range directories {
		fullPath := filepath.Join(u.installer.Root, dir)

		// Check if directory is empty
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Already removed
			}
			continue // Skip on error
		}

		if len(entries) == 0 {
			u.installer.progress("  rmdir %s", dir)
			if err := os.Remove(fullPath); err != nil {
				// Non-critical - directory might be in use
				continue
			}
		}
	}

	return nil
}

// updateDatabase removes package from database.
func (u *Unmerger) updateDatabase() error {
	u.installer.progress("Updating package database")

	atom := fmt.Sprintf("%s-%s", u.pkg.Package.Name, u.pkg.Package.Version)
	if err := u.installer.DB.Remove(atom); err != nil {
		return err
	}

	u.installer.progress("Removed %s from database", atom)

	return nil
}

// runPreRemoveHooks runs pre-remove hooks.
func (u *Unmerger) runPreRemoveHooks() error {
	u.installer.progress("Running pre-remove hooks")

	// TODO: Implement pre-remove hooks
	// - Check reverse dependencies
	// - Backup configuration files
	// - Custom package-specific hooks

	return nil
}

// runPostRemoveHooks runs post-remove hooks.
func (u *Unmerger) runPostRemoveHooks() error {
	u.installer.progress("Running post-remove hooks")

	// Run standard post-remove hooks
	hooks := []Hook{
		&LdconfigHook{},
		// TODO: Add more hooks:
		// - UpdateDesktopDBHook
		// - UpdateMimeDBHook
		// - UpdateCachesHook
	}

	ctx := HookContext{
		Package:   u.pkg.Package,
		Phase:     PhasePostRemove,
		Root:      u.installer.Root,
		Env:       make(map[string]string),
		Installer: u.installer,
	}

	for _, hook := range hooks {
		if err := hook.Run(ctx); err != nil {
			return fmt.Errorf("hook failed: %w", err)
		}
	}

	return nil
}
