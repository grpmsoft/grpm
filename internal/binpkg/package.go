// Package binpkg implements binary package format handling.
//
// Supports both modern GPKG (.gpkg.tar) and legacy TBZ2 (.tbz2) formats.
//
// Example:
//
//	binPkg, err := binpkg.LoadBinaryPackage("/path/to/zlib-1.2.13.gpkg.tar")
//	if err != nil {
//	    return err
//	}
//
//	// Check compatibility
//	if !binPkg.IsCompatible(desiredUSE) {
//	    return errors.New("incompatible USE flags")
//	}
//
//	// Extract to directory
//	if err := binPkg.Extract("/var/tmp/portage/sys-libs/zlib-1.2.13/image"); err != nil {
//	    return err
//	}
package binpkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// BinaryFormat represents binary package format.
type BinaryFormat int

const (
	// FormatGPKG is the modern GPKG format (.gpkg.tar, GLEP 78)
	// - Zstd compression
	// - Metadata in tar header
	// - Signature support
	// - Portage 3.0.30+
	FormatGPKG BinaryFormat = iota

	// FormatTBZ2 is the legacy TBZ2 format (.tbz2)
	// - XPAK metadata
	// - Bzip2 compression
	// - Backward compatibility
	FormatTBZ2

	// FormatUnknown indicates unknown or unsupported format
	FormatUnknown
)

// String returns string representation of binary format.
func (bf BinaryFormat) String() string {
	switch bf {
	case FormatGPKG:
		return "gpkg"
	case FormatTBZ2:
		return "tbz2"
	default:
		return "unknown"
	}
}

// Extension returns file extension for format.
func (bf BinaryFormat) Extension() string {
	switch bf {
	case FormatGPKG:
		return ".gpkg.tar"
	case FormatTBZ2:
		return ".tbz2"
	default:
		return ""
	}
}

// DetectFormat detects binary package format from file path.
func DetectFormat(path string) BinaryFormat {
	if strings.HasSuffix(path, ".gpkg.tar") {
		return FormatGPKG
	}
	if strings.HasSuffix(path, ".tbz2") || strings.HasSuffix(path, ".tar.bz2") {
		return FormatTBZ2
	}
	return FormatUnknown
}

// BinaryPackage represents a binary package (pre-compiled).
type BinaryPackage struct {
	// Package is the package metadata
	Package *pkg.Package

	// Format is the binary package format (GPKG or TBZ2)
	Format BinaryFormat

	// Path is the file path to the binary package
	Path string

	// Size is the package file size in bytes
	Size int64

	// Checksum is the SHA256 checksum
	Checksum string

	// Signature is the package signature (optional)
	Signature *Signature

	// BuildInfo contains build-time metadata
	BuildInfo *BuildMetadata
}

// BuildMetadata contains information about how the package was built.
type BuildMetadata struct {
	// BuildDate is when the package was built
	BuildDate time.Time

	// BuildHost is the hostname where it was built
	BuildHost string

	// CFLAGS are the C compiler flags used
	CFLAGS string

	// CXXFLAGS are the C++ compiler flags used
	CXXFLAGS string

	// LDFLAGS are the linker flags used
	LDFLAGS string

	// USE contains the USE flags used during build
	USE []string

	// Features contains Portage features enabled during build
	Features []string

	// EAPI is the EAPI version
	EAPI string

	// Repository is the repository (e.g., "gentoo")
	Repository string

	// Size is the installed size in bytes
	Size int64
}

// Signature represents a package signature for verification.
type Signature struct {
	// Type is the signature type (GPG, SSH, etc)
	Type SignatureType

	// KeyID is the signing key identifier
	KeyID string

	// Signature is the actual signature data
	Data []byte

	// Created is when the signature was created
	Created time.Time
}

// SignatureType represents the type of signature.
type SignatureType int

const (
	// SignatureGPG is a GPG/PGP signature
	SignatureGPG SignatureType = iota

	// SignatureSSH is an SSH signature
	SignatureSSH

	// SignatureRSA is an RSA signature
	SignatureRSA

	// SignatureNone means no signature
	SignatureNone
)

// String returns string representation of signature type.
func (st SignatureType) String() string {
	switch st {
	case SignatureGPG:
		return "gpg"
	case SignatureSSH:
		return "ssh"
	case SignatureRSA:
		return "rsa"
	default:
		return "none"
	}
}

// LoadBinaryPackage loads a binary package from file.
//
// Automatically detects format (GPKG or TBZ2) and uses appropriate reader.
func LoadBinaryPackage(path string) (*BinaryPackage, error) {
	format := DetectFormat(path)

	switch format {
	case FormatGPKG:
		return LoadGPKG(path)
	case FormatTBZ2:
		return LoadTBZ2(path)
	default:
		return nil, fmt.Errorf("unknown binary package format: %s", path)
	}
}

// IsCompatible checks if this binary package is compatible with desired USE flags.
//
// A binary package is compatible if:
//   - All required USE flags are present
//   - No conflicting USE flags are present
func (bp *BinaryPackage) IsCompatible(desiredUSE []string) bool {
	if bp.BuildInfo == nil {
		return false
	}

	// Build map of actual USE flags
	actualUSE := make(map[string]bool)
	for _, flag := range bp.BuildInfo.USE {
		actualUSE[flag] = true
	}

	// Check all desired USE flags
	for _, flag := range desiredUSE {
		enabled := true
		checkFlag := flag

		// Handle negative flags (-debug)
		if strings.HasPrefix(flag, "-") {
			enabled = false
			checkFlag = flag[1:]
		}

		// Check if flag matches desired state
		if _, present := actualUSE[checkFlag]; present != enabled {
			return false
		}
	}

	return true
}

// IsFresh checks if the binary package is newer than the given maximum age.
func (bp *BinaryPackage) IsFresh(maxAge time.Duration) bool {
	if bp.BuildInfo == nil {
		return false
	}

	return time.Since(bp.BuildInfo.BuildDate) < maxAge
}

// Verify verifies the package signature and checksum.
func (bp *BinaryPackage) Verify() error {
	// TODO: Implement signature verification
	// - Verify GPG/SSH signature if present
	// - Verify SHA256 checksum
	return nil
}

// Extract extracts the binary package contents to destination directory.
//
// This is the installation image directory (D in Portage).
func (bp *BinaryPackage) Extract(destDir string) error {
	switch bp.Format {
	case FormatGPKG:
		return ExtractGPKG(bp.Path, destDir)
	case FormatTBZ2:
		return ExtractTBZ2(bp.Path, destDir)
	default:
		return fmt.Errorf("unsupported format: %s", bp.Format)
	}
}

// GetMetadata extracts metadata without extracting full package.
//
// This is useful for quickly checking package information without extraction.
func (bp *BinaryPackage) GetMetadata() (*BuildMetadata, error) {
	if bp.BuildInfo != nil {
		return bp.BuildInfo, nil
	}

	// Load metadata based on format
	switch bp.Format {
	case FormatGPKG:
		return GetGPKGMetadata(bp.Path)
	case FormatTBZ2:
		return GetTBZ2Metadata(bp.Path)
	default:
		return nil, fmt.Errorf("unsupported format: %s", bp.Format)
	}
}

// String returns string representation of binary package.
func (bp *BinaryPackage) String() string {
	if bp.Package == nil {
		return "BinaryPackage{unknown}"
	}

	return fmt.Sprintf("BinaryPackage{%s-%s, %s, %d bytes}",
		bp.Package.Name,
		bp.Package.Version,
		bp.Format,
		bp.Size)
}
