// Package fetch implements distfile (source tarball) fetching for GRPM.
//
// This package provides functionality for downloading source tarballs from
// mirrors, verifying checksums, and managing the distfiles cache.
//
// The fetch package is part of the Infrastructure layer in DDD terms,
// handling external I/O operations for source package retrieval.
//
// Example:
//
//	fetcher := fetch.NewFetcher(fetch.Config{
//	    DistDir: "/var/cache/distfiles",
//	    Mirrors: []string{"https://gentoo.osuosl.org/"},
//	})
//
//	distfiles, err := fetch.ParseManifest("/var/db/repos/gentoo/app-misc/hello/Manifest")
//	if err != nil {
//	    return err
//	}
//
//	if err := fetcher.Fetch(ctx, distfiles, "/var/cache/distfiles"); err != nil {
//	    return err
//	}
package fetch

import (
	"context"
	"errors"
)

// ErrChecksumMismatch indicates the downloaded file checksum does not match expected.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ErrNoMirrors indicates no mirrors are available for download.
var ErrNoMirrors = errors.New("no mirrors available")

// ErrDownloadFailed indicates all download attempts failed.
var ErrDownloadFailed = errors.New("all download attempts failed")

// ErrFileNotFound indicates the distfile was not found on any mirror.
var ErrFileNotFound = errors.New("distfile not found")

// Fetcher defines the interface for downloading distfiles.
//
// Implementations handle mirror selection, download resumption,
// checksum verification, and retry logic.
type Fetcher interface {
	// Fetch downloads multiple distfiles to the destination directory.
	// It uses configured mirrors with automatic failover.
	// Returns error if any distfile fails to download or verify.
	Fetch(ctx context.Context, distfiles []Distfile, destDir string) error

	// FetchOne downloads a single distfile to the destination directory.
	// Uses configured mirrors with automatic failover and checksum verification.
	FetchOne(ctx context.Context, distfile Distfile, destDir string) error
}

// Distfile represents a source tarball to be downloaded.
//
// Distfile is a Value Object - immutable after creation, compared by value.
// It contains all information needed to download and verify a source file.
type Distfile struct {
	// Filename is the name of the file (e.g., "hello-2.10.tar.gz")
	Filename string

	// Size is the expected file size in bytes
	Size int64

	// URIs are the download URLs from SRC_URI (can be empty for mirror-only)
	// When empty, the file is fetched from GENTOO_MIRRORS
	URIs []string

	// Checksums contains the expected hash values for verification
	Checksums Checksums
}

// NewDistfile creates a new Distfile with the given parameters.
//
// This constructor validates that filename is not empty.
func NewDistfile(filename string, size int64, checksums Checksums) Distfile {
	return Distfile{
		Filename:  filename,
		Size:      size,
		Checksums: checksums,
		URIs:      make([]string, 0),
	}
}

// WithURIs returns a copy of the Distfile with the given URIs added.
//
// This follows the immutable Value Object pattern.
func (d Distfile) WithURIs(uris []string) Distfile {
	newURIs := make([]string, len(uris))
	copy(newURIs, uris)
	return Distfile{
		Filename:  d.Filename,
		Size:      d.Size,
		URIs:      newURIs,
		Checksums: d.Checksums,
	}
}

// IsValid checks if the Distfile has minimum required data.
func (d Distfile) IsValid() bool {
	return d.Filename != "" && d.Checksums.HasAny()
}

// Checksums holds cryptographic hash values for file verification.
//
// Checksums is a Value Object - immutable after creation.
// Portage supports multiple algorithms; BLAKE2B is preferred (strongest),
// followed by SHA512, then SHA256.
type Checksums struct {
	// SHA256 is the SHA-256 hash (64 hex characters)
	SHA256 string

	// SHA512 is the SHA-512 hash (128 hex characters)
	SHA512 string

	// BLAKE2B is the BLAKE2b hash (128 hex characters)
	// This is the preferred algorithm in modern Portage
	BLAKE2B string
}

// NewChecksums creates a new Checksums Value Object.
func NewChecksums(sha256, sha512, blake2b string) Checksums {
	return Checksums{
		SHA256:  sha256,
		SHA512:  sha512,
		BLAKE2B: blake2b,
	}
}

// HasAny returns true if at least one checksum is set.
func (c Checksums) HasAny() bool {
	return c.SHA256 != "" || c.SHA512 != "" || c.BLAKE2B != ""
}

// Preferred returns the strongest available checksum algorithm and value.
//
// Priority: BLAKE2B > SHA512 > SHA256
// Returns algorithm name and hash value, or empty strings if none set.
func (c Checksums) Preferred() (algorithm, hash string) {
	if c.BLAKE2B != "" {
		return "BLAKE2B", c.BLAKE2B
	}
	if c.SHA512 != "" {
		return "SHA512", c.SHA512
	}
	if c.SHA256 != "" {
		return "SHA256", c.SHA256
	}
	return "", ""
}

// Equals checks if two Checksums are equal (Value Object equality).
func (c Checksums) Equals(other Checksums) bool {
	return c.SHA256 == other.SHA256 &&
		c.SHA512 == other.SHA512 &&
		c.BLAKE2B == other.BLAKE2B
}

// Config holds configuration for the Fetcher.
type Config struct {
	// DistDir is the directory where distfiles are stored
	// Default: /var/cache/distfiles
	DistDir string

	// Mirrors is the list of Gentoo mirrors to use
	// Mirrors are tried in order until download succeeds
	Mirrors []string

	// MaxRetries is the maximum number of retry attempts per mirror
	// Default: 3
	MaxRetries int

	// Timeout is the download timeout per file in seconds
	// Default: 300 (5 minutes)
	Timeout int

	// Resume enables download resume support
	// Default: true
	Resume bool

	// Parallel is the number of parallel downloads
	// Default: 1 (sequential)
	Parallel int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DistDir:    "/var/cache/distfiles",
		Mirrors:    []string{},
		MaxRetries: 3,
		Timeout:    300,
		Resume:     true,
		Parallel:   1,
	}
}

// WithDistDir returns a copy of Config with the given DistDir.
func (c Config) WithDistDir(dir string) Config {
	c.DistDir = dir
	return c
}

// WithMirrors returns a copy of Config with the given mirrors.
func (c Config) WithMirrors(mirrors []string) Config {
	c.Mirrors = make([]string, len(mirrors))
	copy(c.Mirrors, mirrors)
	return c
}

// NewFetcher creates a new Fetcher with the given configuration.
//
// This is the preferred way to create a Fetcher. It returns an HTTPDownloader
// configured with the provided settings.
//
// If config.Mirrors is empty, DefaultMirrors will be used.
//
// Example:
//
//	fetcher := fetch.NewFetcher(fetch.Config{
//	    DistDir: "/var/cache/distfiles",
//	    Mirrors: []string{"https://gentoo.osuosl.org/"},
//	})
func NewFetcher(config Config) Fetcher {
	return NewHTTPDownloader(config)
}
