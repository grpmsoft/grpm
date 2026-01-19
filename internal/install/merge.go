package install

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// Merger handles the merge process (installation of files).
type Merger struct {
	installer *Installer
	pkg       *pkg.Package
	opts      InstallOptions

	// imageDir is the directory containing files to install (D in Portage)
	imageDir string

	// installedFiles tracks files installed during this merge
	installedFiles []state.InstalledFile

	// protect handles CONFIG_PROTECT logic for configuration files
	protect *ConfigProtect

	// protectedFiles tracks files that were protected during this merge
	protectedFiles []ProtectedFile
}

// NewMerger creates a new merger.
func NewMerger(installer *Installer, p *pkg.Package, opts InstallOptions) *Merger {
	// Image directory is usually WorkDir/image
	imageDir := filepath.Join(opts.WorkDir, "image")

	return &Merger{
		installer:      installer,
		pkg:            p,
		opts:           opts,
		imageDir:       imageDir,
		installedFiles: make([]state.InstalledFile, 0),
		protect:        NewConfigProtect(),
		protectedFiles: make([]ProtectedFile, 0),
	}
}

// NewMergerWithProtect creates a new merger with custom ConfigProtect settings.
//
// This allows callers to configure protected paths from make.conf or other sources.
func NewMergerWithProtect(installer *Installer, p *pkg.Package, opts InstallOptions, protect *ConfigProtect) *Merger {
	imageDir := filepath.Join(opts.WorkDir, "image")

	return &Merger{
		installer:      installer,
		pkg:            p,
		opts:           opts,
		imageDir:       imageDir,
		installedFiles: make([]state.InstalledFile, 0),
		protect:        protect,
		protectedFiles: make([]ProtectedFile, 0),
	}
}

// Merge performs the merge operation.
//
// Steps:
//  1. Pre-install checks (collisions, disk space)
//  2. Run pre-install hooks
//  3. Install files from image directory to root
//  4. Update package database
//  5. Run post-install hooks
func (m *Merger) Merge() error {
	m.installer.progress("Merging files from %s", m.imageDir)

	// Check if image directory exists
	if _, err := os.Stat(m.imageDir); err != nil {
		return fmt.Errorf("image directory does not exist: %s", m.imageDir)
	}

	// Step 1: Check for collisions
	if !m.opts.Force {
		if err := m.checkCollisions(); err != nil {
			return fmt.Errorf("collision check failed: %w", err)
		}
	}

	// Step 2: Run pre-install hooks
	if !m.opts.SkipHooks {
		if err := m.runPreInstallHooks(); err != nil {
			return fmt.Errorf("pre-install hooks failed: %w", err)
		}
	}

	// Step 3: Install files
	if err := m.installFiles(); err != nil {
		return fmt.Errorf("file installation failed: %w", err)
	}

	// Step 4: Update database
	if err := m.updateDatabase(); err != nil {
		return fmt.Errorf("database update failed: %w", err)
	}

	// Step 5: Run post-install hooks
	if !m.opts.SkipHooks {
		if err := m.runPostInstallHooks(); err != nil {
			// Post-install hooks are not critical
			m.installer.progress("Warning: post-install hooks failed: %v", err)
		}
	}

	// Step 6: Report protected files
	if m.HasProtectedFiles() {
		m.reportProtectedFiles()
	}

	return nil
}

// reportProtectedFiles logs information about protected configuration files.
//
// This informs the user that some configuration files need manual merging
// using tools like etc-update or dispatch-conf.
func (m *Merger) reportProtectedFiles() {
	m.installer.progress("")
	m.installer.progress(">>> %d configuration file(s) need updating.", len(m.protectedFiles))
	m.installer.progress(">>> Use etc-update or dispatch-conf to merge changes.")
	m.installer.progress("")

	for _, pf := range m.protectedFiles {
		m.installer.progress("    %s", pf.Original)
	}
}

// checkCollisions checks for file collisions.
//
// This implements Portage's "protect-owned" behavior (default):
//   - Orphan files (exist but not tracked): warning only, allow overwrite
//   - Files owned by another package: FATAL, block installation
//   - Protected system files: FATAL, block installation
func (m *Merger) checkCollisions() error {
	m.installer.progress("Checking for file collisions")

	detector := NewCollisionDetector(m.installer.DB)

	// Find all files in image directory
	filesToInstall := make([]string, 0)

	err := filepath.Walk(m.imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories - only check files for collisions
		// Directories like /usr, /usr/bin are expected to exist
		if info.IsDir() {
			return nil
		}

		// Get relative path from image directory
		relPath, err := filepath.Rel(m.imageDir, path)
		if err != nil {
			return err
		}

		// Convert to absolute path in target system
		targetPath := filepath.Join(m.installer.Root, relPath)
		filesToInstall = append(filesToInstall, targetPath)

		return nil
	})

	if err != nil {
		return err
	}

	// Detect collisions
	collisions, err := detector.Detect(filesToInstall, m.pkg.Name)
	if err != nil {
		return err
	}

	// Separate fatal collisions from warnings (Portage "protect-owned" behavior)
	var fatalCollisions []Collision
	var warningCollisions []Collision

	for _, c := range collisions {
		switch c.Type {
		case CollisionOwnedByOther, CollisionProtected:
			// These are fatal - file belongs to another package or is protected
			fatalCollisions = append(fatalCollisions, c)
		case CollisionFileExists:
			// Orphan files - just warn, allow overwrite (Portage default)
			warningCollisions = append(warningCollisions, c)
		}
	}

	// Log warnings for orphan files
	if len(warningCollisions) > 0 {
		m.installer.progress(">>> %d orphan file(s) will be overwritten:", len(warningCollisions))
		for _, c := range warningCollisions {
			m.installer.progress("    %s", c.Path)
		}
	}

	// Report and fail on fatal collisions
	if len(fatalCollisions) > 0 {
		for _, c := range fatalCollisions {
			m.installer.progress("!!! %s", c.String())
		}
		return fmt.Errorf("found %d file collision(s)", len(fatalCollisions))
	}

	return nil
}

// installFiles installs all files from image directory to root.
func (m *Merger) installFiles() error {
	m.installer.progress("Installing files")

	// Walk through image directory
	return filepath.Walk(m.imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from image directory
		relPath, err := filepath.Rel(m.imageDir, path)
		if err != nil {
			return err
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Target path in root filesystem
		targetPath := filepath.Join(m.installer.Root, relPath)

		// Install based on file type
		if info.IsDir() {
			return m.installDirectory(relPath, targetPath, info)
		} else if info.Mode()&os.ModeSymlink != 0 {
			return m.installSymlink(path, relPath, targetPath)
		} else {
			return m.installRegularFile(path, relPath, targetPath, info)
		}
	})
}

// installDirectory creates a directory.
func (m *Merger) installDirectory(relPath, targetPath string, info os.FileInfo) error {
	m.installer.progress("  dir  %s", relPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
	}

	// Track installed directory
	m.installedFiles = append(m.installedFiles, state.InstalledFile{
		Path:  "/" + filepath.ToSlash(relPath),
		Type:  state.FileTypeDirectory,
		Mode:  uint32(info.Mode()),
		MTime: info.ModTime().Unix(),
	})

	return nil
}

// installSymlink creates a symbolic link.
func (m *Merger) installSymlink(sourcePath, relPath, targetPath string) error {
	// Read link target
	linkTarget, err := os.Readlink(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", sourcePath, err)
	}

	m.installer.progress("  sym  %s -> %s", relPath, linkTarget)

	// Remove existing file if present
	_ = os.Remove(targetPath)

	// Create symlink
	if err := os.Symlink(linkTarget, targetPath); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
	}

	// Track installed symlink
	m.installedFiles = append(m.installedFiles, state.InstalledFile{
		Path:   "/" + filepath.ToSlash(relPath),
		Type:   state.FileTypeSymlink,
		Target: linkTarget,
		MTime:  time.Now().Unix(),
	})

	return nil
}

// installRegularFile copies a regular file.
//
// If the file is protected by CONFIG_PROTECT and the existing file differs,
// the new file is installed with a ._cfg*_ prefix instead of overwriting.
func (m *Merger) installRegularFile(sourcePath, relPath, targetPath string, info os.FileInfo) error {
	// Track the actual destination (may change for protected files)
	actualDest := targetPath
	isProtected := false

	// Check if this file should be protected
	if m.protect != nil && m.protect.ShouldProtect(targetPath) {
		// Check if the target file already exists
		if _, err := os.Stat(targetPath); err == nil {
			// File exists - check if contents differ
			filesEqual, err := CompareFiles(sourcePath, targetPath)
			if err != nil {
				// Cannot compare - treat as different (safer)
				filesEqual = false
			}

			if !filesEqual {
				// Files differ - generate protected name
				protectedDest := m.protect.GenerateProtectedName(targetPath)
				if protectedDest == "" {
					return fmt.Errorf("too many protected config files for %s", targetPath)
				}

				actualDest = protectedDest
				isProtected = true

				m.installer.progress("  cfg  %s -> %s", relPath, filepath.Base(protectedDest))

				// Track this protected file for reporting
				atom := fmt.Sprintf("%s-%s", m.pkg.Name, m.pkg.Version)
				m.protectedFiles = append(m.protectedFiles, ProtectedFile{
					Original:  targetPath,
					Protected: protectedDest,
					Package:   atom,
				})
			} else {
				m.installer.progress("  obj  %s (unchanged)", relPath)
			}
		} else {
			// File doesn't exist - install normally
			m.installer.progress("  obj  %s", relPath)
		}
	} else {
		m.installer.progress("  obj  %s", relPath)
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(actualDest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Copy file to actual destination (may be protected name)
	if err := m.copyFile(sourcePath, actualDest); err != nil {
		return fmt.Errorf("failed to copy file %s: %w", relPath, err)
	}

	// Set file permissions
	if err := os.Chmod(actualDest, info.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", actualDest, err)
	}

	// Calculate hash
	hash, err := m.calculateHash(actualDest)
	if err != nil {
		// Hash calculation failure is not critical
		hash = ""
	}

	// Track installed file
	// For protected files, we track the original path (where file should live)
	// The protected version is tracked separately in protectedFiles
	installedPath := "/" + filepath.ToSlash(relPath)
	if isProtected {
		// For protected files, track the protected path
		protRelPath, _ := filepath.Rel(m.installer.Root, actualDest)
		installedPath = "/" + filepath.ToSlash(protRelPath)
	}

	m.installedFiles = append(m.installedFiles, state.InstalledFile{
		Path:  installedPath,
		Type:  state.FileTypeRegular,
		Size:  info.Size(),
		Mode:  uint32(info.Mode()),
		Hash:  hash,
		MTime: info.ModTime().Unix(),
	})

	return nil
}

// copyFile copies a file from source to destination.
func (m *Merger) copyFile(source, dest string) error {
	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	// Create destination file
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	// Copy data
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return nil
}

// calculateHash calculates SHA256 hash of a file.
func (m *Merger) calculateHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// updateDatabase updates the package database with installed package info.
func (m *Merger) updateDatabase() error {
	m.installer.progress("Updating package database")

	// Create installed package entry
	installedPkg := &state.InstalledPackage{
		Package:     m.pkg,
		InstallTime: time.Now(),
		Files:       m.installedFiles,
		USE:         make([]string, 0), // TODO: Get actual USE flags
		CFLAGS:      "",                // TODO: Get from build environment
		Size:        m.calculateTotalSize(),
		BuildInfo: state.BuildInfo{
			Host:      "grpm",
			BuildDate: time.Now(),
			EAPI:      "8", // TODO: Get actual EAPI
		},
	}

	// Add to in-memory database
	atom := fmt.Sprintf("%s-%s", m.pkg.Name, m.pkg.Version)
	if err := m.installer.DB.Add(installedPkg); err != nil {
		return err
	}

	// Persist to VarDB on disk
	vardbRoot := filepath.Join(m.installer.Root, "var", "db", "pkg")
	writer := state.NewVarDBWriter(vardbRoot)
	if err := writer.Write(installedPkg); err != nil {
		return fmt.Errorf("failed to persist package to VarDB: %w", err)
	}

	m.installer.progress("Added %s to database (%d files)", atom, len(m.installedFiles))

	return nil
}

// calculateTotalSize calculates total size of installed files.
func (m *Merger) calculateTotalSize() int64 {
	var total int64
	for _, file := range m.installedFiles {
		total += file.Size
	}
	return total
}

// runPreInstallHooks runs pre-install hooks.
func (m *Merger) runPreInstallHooks() error {
	m.installer.progress("Running pre-install hooks")

	// TODO: Implement pre-install hooks
	// - Check dependencies
	// - Check disk space
	// - Custom package-specific hooks

	return nil
}

// runPostInstallHooks runs post-install hooks.
func (m *Merger) runPostInstallHooks() error {
	m.installer.progress("Running post-install hooks")

	// Run standard post-install hooks
	hooks := []Hook{
		&LdconfigHook{},
		// TODO: Add more hooks:
		// - UpdateDesktopDBHook
		// - UpdateMimeDBHook
		// - UpdateCachesHook
	}

	ctx := HookContext{
		Package:   m.pkg,
		Phase:     PhasePostInstall,
		Root:      m.installer.Root,
		Env:       make(map[string]string),
		Installer: m.installer,
	}

	for _, hook := range hooks {
		if err := hook.Run(ctx); err != nil {
			return fmt.Errorf("hook failed: %w", err)
		}
	}

	return nil
}

// GetProtectedFiles returns a list of files that were protected during merge.
//
// These files need manual resolution using a tool like etc-update or dispatch-conf.
// The returned slice contains information about the original path, the protected
// path (._cfg*_ file), and the package that installed the file.
func (m *Merger) GetProtectedFiles() []ProtectedFile {
	return m.protectedFiles
}

// HasProtectedFiles returns true if any files were protected during merge.
func (m *Merger) HasProtectedFiles() bool {
	return len(m.protectedFiles) > 0
}

// GetProtectedCount returns the number of protected files.
func (m *Merger) GetProtectedCount() int {
	return len(m.protectedFiles)
}

// SetConfigProtect sets a custom ConfigProtect for this merger.
//
// This should be called before Merge() to use custom protection settings.
func (m *Merger) SetConfigProtect(protect *ConfigProtect) {
	m.protect = protect
}
