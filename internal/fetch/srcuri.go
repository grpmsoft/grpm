// Package fetch implements distfile (source tarball) fetching for GRPM.
//
// This file provides integration between SRC_URI parsing and distfile fetching.
package fetch

import (
	"context"
	"fmt"

	"github.com/grpmsoft/grpm/internal/repo"
)

// SrcURIFetcher provides high-level SRC_URI-based fetching.
//
// It combines SRC_URI parsing with manifest-based checksum verification
// and the actual download process.
type SrcURIFetcher struct {
	downloader Fetcher
	config     Config
}

// NewSrcURIFetcher creates a new SRC_URI fetcher.
//
// The fetcher uses the provided configuration for mirrors and download settings.
func NewSrcURIFetcher(config Config) *SrcURIFetcher {
	return &SrcURIFetcher{
		downloader: NewHTTPDownloader(config),
		config:     config,
	}
}

// FetchSrcURI parses SRC_URI and fetches the required distfiles.
//
// Parameters:
//   - ctx: context for cancellation
//   - srcURI: the SRC_URI content from the ebuild
//   - manifest: parsed Manifest with checksums
//   - activeFlags: enabled USE flags for conditional filtering
//   - vars: package variables for expansion (P, PV, PN, etc.)
//   - destDir: destination directory for downloads
//
// The function:
//  1. Parses SRC_URI to extract download entries
//  2. Filters entries based on active USE flags
//  3. Looks up checksums from the Manifest
//  4. Downloads and verifies each file
func (f *SrcURIFetcher) FetchSrcURI(
	ctx context.Context,
	srcURI string,
	manifest *Manifest,
	activeFlags map[string]bool,
	vars map[string]string,
	destDir string,
) error {
	// Parse SRC_URI
	entries, err := repo.ParseSrcURI(srcURI, activeFlags, vars)
	if err != nil {
		return fmt.Errorf("parsing SRC_URI: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	// Convert entries to distfiles
	distfiles, err := f.entriesToDistfiles(entries, manifest)
	if err != nil {
		return err
	}

	// Fetch all distfiles
	return f.downloader.Fetch(ctx, distfiles, destDir)
}

// entriesToDistfiles converts SrcURIEntry to Distfile with checksums from Manifest.
func (f *SrcURIFetcher) entriesToDistfiles(entries []repo.SrcURIEntry, manifest *Manifest) ([]Distfile, error) {
	distfiles := make([]Distfile, 0, len(entries))
	seen := make(map[string]bool)

	for _, entry := range entries {
		// Skip duplicates (same filename)
		if seen[entry.Filename] {
			continue
		}
		seen[entry.Filename] = true

		// Look up checksum info from manifest
		manifestEntry, ok := manifest.GetEntry(entry.Filename)
		if !ok {
			return nil, fmt.Errorf("distfile %s not found in Manifest", entry.Filename)
		}

		// Create distfile with checksum and URI
		distfile := NewDistfile(entry.Filename, manifestEntry.Size, manifestEntry.Checksums)

		// Add the explicit URI if present
		if entry.URL != "" {
			distfile = distfile.WithURIs([]string{entry.URL})
		}

		distfiles = append(distfiles, distfile)
	}

	return distfiles, nil
}

// ParseAndListDistfiles parses SRC_URI and returns the list of required distfiles.
//
// This is useful for dry-run operations or showing what would be downloaded.
func ParseAndListDistfiles(
	srcURI string,
	activeFlags map[string]bool,
	vars map[string]string,
) ([]repo.SrcURIEntry, error) {
	return repo.ParseSrcURI(srcURI, activeFlags, vars)
}
