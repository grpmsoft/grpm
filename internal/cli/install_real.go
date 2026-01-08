package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/binpkg"
	"github.com/grpmsoft/grpm/internal/install"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// installPackageReal performs actual package installation using Installer.
//
// Process:
//  1. Create package database (if not exists)
//  2. Prepare workdir (extract from binpkg or create mock)
//  3. Create Installer
//  4. Install package
//  5. Update database
func (a *App) installPackageReal(p *pkg.Package, binpkgPath string) error {
	if a.verbose {
		log.Printf("Installing package: %s-%s", p.Name, p.Version)
	}

	// Create package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Prepare workdir
	workDir, err := a.prepareWorkDir(p, binpkgPath)
	if err != nil {
		return fmt.Errorf("failed to prepare work directory: %w", err)
	}
	defer func() {
		// Cleanup workdir after installation
		if err := os.RemoveAll(workDir); err != nil {
			log.Printf("Warning: failed to clean up work directory: %v", err)
		}
	}()

	// Create installer
	installer := install.NewInstaller("/", db)
	installer.Verbose = a.verbose
	installer.OnProgress = func(status string) {
		if a.verbose {
			log.Printf("[installer] %s", status)
		}
	}

	// Install package
	opts := install.InstallOptions{
		WorkDir:  workDir,
		Replace:  false,
		Force:    false, // Force mode disabled - collision detection enabled
		Pretend:  false,
		KeepWork: false,
	}

	if err := installer.Install(p, opts); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Write package metadata to VarDB
	atom := fmt.Sprintf("%s-%s", p.Name, p.Version)
	installedPkg, err := db.Get(atom)
	if err != nil {
		return fmt.Errorf("package not found in database after installation: %w", err)
	}

	vardbWriter := state.NewVarDBWriter("/var/db/pkg")
	if err := vardbWriter.Write(installedPkg); err != nil {
		log.Printf("Warning: failed to write VarDB entry: %v", err)
		// Don't fail installation if VarDB write fails
	}

	if a.verbose {
		log.Printf("Successfully installed %s-%s", p.Name, p.Version)
	}

	return nil
}

// uninstallPackageReal performs actual package removal using Uninstaller.
func (a *App) uninstallPackageReal(atom string, force bool) error {
	if a.verbose {
		log.Printf("Uninstalling package: %s", atom)
	}

	// Get package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Check if package is installed
	if !db.Has(atom) {
		return fmt.Errorf("package not installed: %s", atom)
	}

	// Create installer (handles both install and uninstall)
	installer := install.NewInstaller("/", db)
	installer.Verbose = a.verbose
	installer.OnProgress = func(status string) {
		if a.verbose {
			log.Printf("[uninstaller] %s", status)
		}
	}

	// Uninstall package
	opts := install.UninstallOptions{
		Force:   force,
		Pretend: false,
	}

	if err := installer.Uninstall(atom, opts); err != nil {
		return fmt.Errorf("uninstallation failed: %w", err)
	}

	// Remove VarDB entry after successful uninstallation
	// Parse category and package name from atom (e.g., "app-misc/hello-2.10")
	parts := strings.Split(atom, "/")
	if len(parts) >= 2 {
		category := parts[0]
		pkgVerName := parts[1] // e.g., "hello-2.10"
		vardbPath := filepath.Join("/var/db/pkg", category, pkgVerName)
		if err := os.RemoveAll(vardbPath); err != nil {
			log.Printf("Warning: failed to remove VarDB entry: %v", err)
			// Don't fail uninstallation if VarDB removal fails
		}
	}

	if a.verbose {
		log.Printf("Successfully uninstalled %s", atom)
	}

	return nil
}

// prepareWorkDir prepares work directory for installation.
//
// If binpkgPath is provided, extracts binary package to workdir.
// Otherwise, creates mock workdir with test files.
func (a *App) prepareWorkDir(p *pkg.Package, binpkgPath string) (string, error) {
	// Create temporary work directory
	// Use only package name without category (e.g., "hello" instead of "app-misc/hello")
	pkgBaseName := filepath.Base(p.Name)
	workDir, err := os.MkdirTemp("/var/tmp/portage", fmt.Sprintf("%s-%s-*", pkgBaseName, p.Version))
	if err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// Create image directory (where files will be staged)
	imageDir := filepath.Join(workDir, "image")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		_ = os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to create image directory: %w", err)
	}

	if binpkgPath != "" {
		// Extract binary package
		if a.verbose {
			log.Printf("Extracting binary package: %s", binpkgPath)
		}

		if err := binpkg.ExtractGPKG(binpkgPath, imageDir); err != nil {
			_ = os.RemoveAll(workDir)
			return "", fmt.Errorf("failed to extract binary package: %w", err)
		}
	} else {
		// Create mock files for testing
		if a.verbose {
			log.Printf("Creating mock installation files")
		}

		if err := a.createMockFiles(imageDir, p); err != nil {
			_ = os.RemoveAll(workDir)
			return "", fmt.Errorf("failed to create mock files: %w", err)
		}
	}

	// Return workDir (not imageDir) - installer expects workDir and will append /image
	return workDir, nil
}

// createMockFiles creates mock files for testing installation.
//
// Creates a simple directory structure with test files:
//   - /usr/share/doc/<pkg>/README
//   - /usr/bin/<pkg>
func (a *App) createMockFiles(imageDir string, p *pkg.Package) error {
	// Use only package name without category
	pkgBaseName := filepath.Base(p.Name)

	// Create doc directory
	docDir := filepath.Join(imageDir, "usr", "share", "doc", pkgBaseName+"-"+p.Version)
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return err
	}

	// Create README file
	readmePath := filepath.Join(docDir, "README")
	readmeContent := fmt.Sprintf("Mock package: %s-%s\n\nThis is a test installation.\n", p.Name, p.Version)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return err
	}

	// Create bin directory
	binDir := filepath.Join(imageDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	// Create mock executable
	binPath := filepath.Join(binDir, pkgBaseName)
	binContent := "#!/bin/sh\necho 'Mock executable for " + p.Name + "'\n"
	if err := os.WriteFile(binPath, []byte(binContent), 0755); err != nil {
		return err
	}

	if a.verbose {
		log.Printf("Created mock files: %s, %s", readmePath, binPath)
	}

	return nil
}

// getOrCreatePackageDB returns the package database, creating it if needed.
func (a *App) getOrCreatePackageDB() (*state.PackageDatabase, error) {
	// Check if VarDB exists
	vardbPath := "/var/db/pkg"
	if _, err := os.Stat(vardbPath); os.IsNotExist(err) {
		// Create VarDB directory structure
		if err := os.MkdirAll(vardbPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create VarDB directory: %w", err)
		}
	}

	// Create package database with VarDB path
	db := state.NewPackageDatabase(vardbPath)

	// Load existing packages from VarDB
	loader := state.NewVarDBLoader(vardbPath)
	if err := loader.LoadInto(db); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to load existing packages: %v", err)
		// Continue with empty database
	}

	if a.verbose {
		log.Printf("Package database initialized with %d packages", db.Count())
	}

	return db, nil
}

// buildBinaryPackage builds a binary package from an installed package.
func (a *App) buildBinaryPackage(installedPkg *state.InstalledPackage, outputDir, format, compression string) (string, error) {
	// Import binpkg package for binary package building
	// (we'll need to add this import at the top of the file)

	// Create temporary work directory
	workDir, err := os.MkdirTemp("/var/tmp/portage", "binpkg-*")
	if err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}
	defer func() {
		// Cleanup work directory after building
		if err := os.RemoveAll(workDir); err != nil && a.verbose {
			log.Printf("Warning: failed to clean up work directory: %v", err)
		}
	}()

	// Create binary package builder
	builder, err := binpkg.NewBinaryPackageBuilder(installedPkg, workDir)
	if err != nil {
		return "", fmt.Errorf("failed to create builder: %w", err)
	}

	// Set output directory
	if err := builder.SetOutputDir(outputDir); err != nil {
		return "", fmt.Errorf("failed to set output directory: %w", err)
	}

	// Set format
	switch format {
	case "gpkg":
		builder.SetFormat(binpkg.FormatGPKG)
	case "tbz2":
		builder.SetFormat(binpkg.FormatTBZ2)
	default:
		return "", fmt.Errorf("unsupported format: %s (use 'gpkg' or 'tbz2')", format)
	}

	// Set compression
	var compressionType binpkg.CompressionType
	switch compression {
	case "none":
		compressionType = binpkg.CompressionNone
	case "gzip":
		compressionType = binpkg.CompressionGzip
	case "bzip2":
		compressionType = binpkg.CompressionBzip2
	case "xz":
		compressionType = binpkg.CompressionXZ
	case "zstd":
		compressionType = binpkg.CompressionZstd
	default:
		return "", fmt.Errorf("unsupported compression: %s", compression)
	}
	builder.SetCompression(compressionType)

	// Enable verbose if needed
	builder.Verbose = a.verbose

	// Build the package
	pkg, err := builder.Build()
	if err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	return pkg.Path, nil
}
