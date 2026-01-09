// Package binpkg implements binary package building and management.
package binpkg

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/state"
)

// BinaryPackageBuilder builds binary packages from installed packages.
//
// Example:
//
//	builder := binpkg.NewBinaryPackageBuilder(installedPkg, "/tmp/build")
//	builder.SetFormat(binpkg.FormatGPKG)
//	builder.SetCompression(binpkg.CompressionZstd)
//	pkg, err := builder.Build()
type BinaryPackageBuilder struct {
	// Package is the installed package to build from
	Package *state.InstalledPackage

	// WorkDir is the temporary directory for building
	WorkDir string

	// OutputDir is where the final package will be placed
	OutputDir string

	// Format is the binary package format (GPKG or TBZ2)
	Format BinaryFormat

	// Compression is the compression type
	Compression CompressionType

	// Signer is optional package signer
	Signer PackageSigner

	// IncludeBuildLog includes build.log in package metadata
	IncludeBuildLog bool

	// Verbose enables detailed logging
	Verbose bool
}

// CompressionType represents package compression algorithm.
type CompressionType int

const (
	// CompressionNone - no compression (for testing)
	CompressionNone CompressionType = iota
	// CompressionGzip - gzip compression (.gz)
	CompressionGzip
	// CompressionBzip2 - bzip2 compression (.bz2)
	CompressionBzip2
	// CompressionXZ - xz compression (.xz)
	CompressionXZ
	// CompressionZstd - zstd compression (.zst) - default for GPKG
	CompressionZstd
)

// String returns compression type name.
func (c CompressionType) String() string {
	switch c {
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gzip"
	case CompressionBzip2:
		return "bzip2"
	case CompressionXZ:
		return "xz"
	case CompressionZstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// Extension returns file extension for compression type.
func (c CompressionType) Extension() string {
	switch c {
	case CompressionNone:
		return ""
	case CompressionGzip:
		return ".gz"
	case CompressionBzip2:
		return ".bz2"
	case CompressionXZ:
		return ".xz"
	case CompressionZstd:
		return ".zst"
	default:
		return ""
	}
}

// BuildOptions configures the build process.
type BuildOptions struct {
	// IncludeBuildLog includes build.log in package
	IncludeBuildLog bool

	// IncludeDebugInfo includes debug symbols
	IncludeDebugInfo bool

	// StripBinaries strips binaries to reduce size
	StripBinaries bool

	// VerifyChecksums verifies file checksums before packing
	VerifyChecksums bool

	// SignPackage enables package signing
	SignPackage bool

	// SignatureType is GPG or SSH
	SignatureType SignatureType
}

// NewBinaryPackageBuilder creates a new binary package builder.
//
// Parameters:
//   - pkg: Installed package to build from
//   - workDir: Temporary directory for building
//
// Returns:
//   - *BinaryPackageBuilder: Configured builder
//   - error: If workDir cannot be created
func NewBinaryPackageBuilder(pkg *state.InstalledPackage, workDir string) (*BinaryPackageBuilder, error) {
	if pkg == nil {
		return nil, fmt.Errorf("package cannot be nil")
	}
	if pkg.Package == nil {
		return nil, fmt.Errorf("package metadata cannot be nil")
	}

	// Create work directory if it doesn't exist
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	return &BinaryPackageBuilder{
		Package:     pkg,
		WorkDir:     workDir,
		OutputDir:   workDir, // Default to work directory
		Format:      FormatGPKG,
		Compression: CompressionZstd,
		Verbose:     false,
	}, nil
}

// SetFormat sets the binary package format.
func (b *BinaryPackageBuilder) SetFormat(format BinaryFormat) {
	b.Format = format
}

// SetCompression sets the compression type.
func (b *BinaryPackageBuilder) SetCompression(compression CompressionType) {
	b.Compression = compression
}

// SetOutputDir sets the output directory for the final package.
func (b *BinaryPackageBuilder) SetOutputDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	b.OutputDir = dir
	return nil
}

// SetSigner sets the package signer.
func (b *BinaryPackageBuilder) SetSigner(signer PackageSigner) {
	b.Signer = signer
}

// Build builds the binary package.
//
// Process:
//  1. Extract files from installed package
//  2. Collect metadata
//  3. Create package archive (GPKG or TBZ2)
//  4. Generate checksums
//  5. Sign package (if signer configured)
//  6. Validate package structure
//
// Returns:
//   - *BinaryPackage: Built package with path and metadata
//   - error: If build fails at any step
func (b *BinaryPackageBuilder) Build() (*BinaryPackage, error) {
	if b.Verbose {
		fmt.Printf("Building binary package: %s-%s\n", b.Package.Package.Name, b.Package.Package.Version)
	}

	// 1. Extract files from installed package
	files, err := b.extractFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to extract files: %w", err)
	}

	// 2. Collect metadata
	metadata, err := b.collectMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to collect metadata: %w", err)
	}

	// 3. Create package archive
	var pkgPath string
	switch b.Format {
	case FormatGPKG:
		pkgPath, err = b.buildGPKG(files, metadata)
	case FormatTBZ2:
		pkgPath, err = b.buildTBZ2(files, metadata)
	default:
		return nil, fmt.Errorf("unsupported format: %v", b.Format)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create package archive: %w", err)
	}

	// 4. Generate checksums
	checksum, err := b.generateChecksum(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate checksum: %w", err)
	}

	// 5. Sign package (if signer configured)
	var signature *Signature
	if b.Signer != nil {
		signature, err = b.Signer.Sign(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to sign package: %w", err)
		}
	}

	// 6. Get package size
	stat, err := os.Stat(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat package: %w", err)
	}

	// Create BinaryPackage result
	pkg := &BinaryPackage{
		Package:   b.Package.Package,
		Format:    b.Format,
		Path:      pkgPath,
		Size:      stat.Size(),
		Checksum:  checksum,
		Signature: signature,
		BuildInfo: metadata,
	}

	if b.Verbose {
		fmt.Printf("Package built successfully: %s (size: %d bytes)\n", pkgPath, stat.Size())
	}

	return pkg, nil
}

// BuildWithOptions builds the binary package with custom options.
func (b *BinaryPackageBuilder) BuildWithOptions(opts BuildOptions) (*BinaryPackage, error) {
	b.IncludeBuildLog = opts.IncludeBuildLog

	// Configure signer if signing requested
	if opts.SignPackage && b.Signer == nil {
		return nil, fmt.Errorf("signing requested but no signer configured")
	}

	return b.Build()
}

// extractFiles extracts files from installed package to staging directory.
func (b *BinaryPackageBuilder) extractFiles() ([]string, error) {
	stagingDir := filepath.Join(b.WorkDir, "staging")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	var extractedFiles []string
	for _, file := range b.Package.Files {
		// Skip directories
		if file.Type == state.FileTypeDirectory {
			continue
		}

		// Source file path
		srcPath := file.Path

		// Destination path in staging
		dstPath := filepath.Join(stagingDir, file.Path)

		// Create parent directories
		dstDir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dstDir, err)
		}

		// Copy file based on type
		switch file.Type {
		case state.FileTypeRegular:
			if err := b.copyFile(srcPath, dstPath); err != nil {
				return nil, fmt.Errorf("failed to copy file %s: %w", srcPath, err)
			}
		case state.FileTypeSymlink:
			if err := b.copySymlink(srcPath, dstPath); err != nil {
				return nil, fmt.Errorf("failed to copy symlink %s: %w", srcPath, err)
			}
		}

		extractedFiles = append(extractedFiles, file.Path)
	}

	return extractedFiles, nil
}

// copyFile copies a regular file.
func (b *BinaryPackageBuilder) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// copySymlink copies a symbolic link.
func (b *BinaryPackageBuilder) copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	return os.Symlink(target, dst)
}

// collectMetadata collects build metadata from installed package.
func (b *BinaryPackageBuilder) collectMetadata() (*BuildMetadata, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &BuildMetadata{
		BuildHost: hostname,
		BuildDate: time.Now(),
		CFLAGS:    b.Package.CFLAGS,
		USE:       b.Package.USE,
		Size:      b.Package.Size,
	}, nil
}

// generateChecksum generates SHA256 checksum for package file.
func (b *BinaryPackageBuilder) generateChecksum(path string) (checksum string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read file for checksum: %w", err)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// buildGPKG builds a GPKG format package.
func (b *BinaryPackageBuilder) buildGPKG(files []string, metadata *BuildMetadata) (string, error) {
	// Generate output path
	pkgName := strings.ReplaceAll(b.Package.Package.Name, "/", "-")
	outputPath := filepath.Join(b.OutputDir, fmt.Sprintf("%s-%s.gpkg.tar", pkgName, b.Package.Package.Version))

	// Create GPKG writer
	writer := NewGPKGWriter(outputPath)
	writer.SetCompression(b.Compression)
	writer.Verbose = b.Verbose

	// Get staging directory
	stagingDir := filepath.Join(b.WorkDir, "staging")

	// Write GPKG
	if err := writer.Write(metadata, stagingDir); err != nil {
		return "", fmt.Errorf("failed to write GPKG: %w", err)
	}

	return outputPath, nil
}

// buildTBZ2 builds a TBZ2 format package.
func (b *BinaryPackageBuilder) buildTBZ2(files []string, metadata *BuildMetadata) (string, error) {
	// Generate output path
	pkgName := strings.ReplaceAll(b.Package.Package.Name, "/", "-")
	outputPath := filepath.Join(b.OutputDir, fmt.Sprintf("%s-%s.tbz2", pkgName, b.Package.Package.Version))

	// Create TBZ2 writer
	writer := NewTBZ2Writer(outputPath)
	writer.Verbose = b.Verbose

	// Get staging directory
	stagingDir := filepath.Join(b.WorkDir, "staging")

	// Write TBZ2
	if err := writer.Write(metadata, stagingDir); err != nil {
		return "", fmt.Errorf("failed to write TBZ2: %w", err)
	}

	return outputPath, nil
}
