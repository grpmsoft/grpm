package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// runFetch handles the 'fetch' command - downloads source tarballs for packages.
//
// This command downloads distfiles (source tarballs) for specified packages
// without building them. Useful for:
//   - Pre-fetching sources before offline builds
//   - Verifying source availability and checksums
//   - Populating local distfiles cache
//
// Process:
//  1. Parse package atoms
//  2. Locate ebuild and Manifest files
//  3. Parse SRC_URI and Manifest checksums
//  4. Download from GENTOO_MIRRORS or explicit URIs
//  5. Verify checksums
//
// Flags:
//   - --repo/-r: Path to Portage repository
//   - --distdir: Directory for downloaded sources
//   - --pretend/-p: Show what would be downloaded (dry-run)
//   - --verify: Only verify existing files, don't download
func (a *App) runFetch(args []string) error {
	// Load Portage configuration (make.conf)
	cfg := a.loadPortageConfig()

	// Parse flags with defaults from configuration
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	repoPath := fs.String("repo", cfg.GetPortDir(), "Path to Portage repository")
	fs.StringVar(repoPath, "r", cfg.GetPortDir(), "Alias for --repo")
	distDir := fs.String("distdir", cfg.GetDistDir(), "Directory for downloaded sources")
	pretend := fs.Bool("pretend", false, "Show what would be downloaded (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	verifyOnly := fs.Bool("verify", false, "Only verify existing files, don't download")

	if err := fs.Parse(args); err != nil {
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Process each package
	for _, atom := range packages {
		if err := a.fetchPackageDistfiles(atom, *repoPath, *distDir, *pretend, *verifyOnly, cfg); err != nil {
			return err
		}
	}

	return nil
}

// fetchPackageDistfiles fetches distfiles for a single package.
//
// This function properly handles SRC_URI parsing to get explicit URLs
// for distfiles that are not available on Gentoo mirrors (e.g., .asc signature files).
func (a *App) fetchPackageDistfiles(atom, repoPath, distDir string, pretend, verifyOnly bool, cfg *config.Config) error {
	log.Printf(">>> Fetching distfiles for %s", atom)

	// Parse atom to get category/package
	catPkg, err := a.parsePackageAtom(atom)
	if err != nil {
		return fmt.Errorf("invalid package atom %q: %w", atom, err)
	}

	// Find and parse Manifest
	manifestPath := fetch.ManifestPath(repoPath, catPkg)
	manifest, err := fetch.ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse Manifest for %s: %w", catPkg, err)
	}

	// Parse SRC_URI from ebuild to get explicit URLs for distfiles
	distfiles, err := a.getDistfilesWithURIs(catPkg, repoPath, manifest)
	if err != nil {
		// Fallback to manifest-only distfiles (without explicit URIs)
		log.Printf("  Warning: could not parse SRC_URI: %v", err)
		log.Printf("  Falling back to mirror-only downloads")
		distfiles = manifest.GetDistfiles()
	}

	if len(distfiles) == 0 {
		log.Printf("  No distfiles for %s", catPkg)
		return nil
	}

	// Display mode
	if pretend {
		fmt.Printf("\nDistfiles for %s:\n", atom)
		for _, df := range distfiles {
			algo, _ := df.Checksums.Preferred()
			if len(df.URIs) > 0 {
				fmt.Printf("  %s (%d bytes, %s)\n", df.Filename, df.Size, algo)
				for _, uri := range df.URIs {
					fmt.Printf("    -> %s\n", uri)
				}
			} else {
				fmt.Printf("  %s (%d bytes, %s) [mirrors]\n", df.Filename, df.Size, algo)
			}
		}
		return nil
	}

	// Verify-only mode
	if verifyOnly {
		return a.verifyDistfiles(distfiles, distDir, atom)
	}

	// Download mode
	return a.downloadDistfiles(distfiles, distDir, cfg)
}

// getDistfilesWithURIs parses SRC_URI from the best ebuild version
// and returns distfiles with explicit URLs where available.
//
// This is essential for files like .asc signatures that have explicit
// upstream URLs and are NOT available on Gentoo mirrors.
func (a *App) getDistfilesWithURIs(catPkg, repoPath string, manifest *fetch.Manifest) ([]fetch.Distfile, error) {
	// Split category/package
	parts := strings.SplitN(catPkg, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package format: %s", catPkg)
	}
	category, pkgName := parts[0], parts[1]

	// Find best ebuild version
	pkgDir := filepath.Join(repoPath, catPkg)
	ebuildPath, version, err := a.findBestEbuild(pkgDir, pkgName)
	if err != nil {
		return nil, fmt.Errorf("finding ebuild: %w", err)
	}

	// Read ebuild content
	content, err := os.ReadFile(ebuildPath)
	if err != nil {
		return nil, fmt.Errorf("reading ebuild: %w", err)
	}

	// Create parser with package metadata for variable expansion
	meta := repo.NewPackageMetadata(category, pkgName, version)
	parser := repo.NewEbuildParserWithMetadata(string(content), meta)

	// Extract SRC_URI
	srcURI := parser.ExtractVariable("SRC_URI")
	if srcURI == "" {
		// No SRC_URI - return manifest distfiles without URIs
		return manifest.GetDistfiles(), nil
	}

	// Build variable map for SRC_URI parsing
	vars := map[string]string{
		"P":        meta.P,
		"PN":       meta.PN,
		"PV":       meta.PV,
		"PR":       meta.PR,
		"PVR":      meta.PVR,
		"PF":       meta.PF,
		"CATEGORY": meta.Category,
	}

	// Parse SRC_URI entries (no USE flag filtering for fetch - get all files)
	entries, err := repo.ParseSrcURI(srcURI, nil, vars)
	if err != nil {
		return nil, fmt.Errorf("parsing SRC_URI: %w", err)
	}

	// Build map of filename -> URIs from SRC_URI entries
	uriMap := make(map[string][]string)
	for _, entry := range entries {
		if entry.URL != "" {
			uriMap[entry.Filename] = append(uriMap[entry.Filename], entry.URL)
		}
	}

	// Create distfiles with checksums from manifest and URIs from SRC_URI
	var distfiles []fetch.Distfile
	for _, entry := range manifest.DistFiles {
		df := fetch.NewDistfile(entry.Filename, entry.Size, entry.Checksums)

		// Add explicit URIs if available
		if uris, ok := uriMap[entry.Filename]; ok && len(uris) > 0 {
			df = df.WithURIs(uris)
		}

		distfiles = append(distfiles, df)
	}

	return distfiles, nil
}

// findBestEbuild finds the best (highest version) ebuild in the package directory.
//
// Returns the ebuild path and extracted version string.
func (a *App) findBestEbuild(pkgDir, pkgName string) (string, string, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return "", "", fmt.Errorf("reading package directory: %w", err)
	}

	// Collect all ebuild versions
	type ebuildInfo struct {
		path    string
		version string
	}
	var ebuilds []ebuildInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
			continue
		}

		// Extract version from filename: pkgName-version.ebuild
		name := strings.TrimSuffix(entry.Name(), ".ebuild")
		if !strings.HasPrefix(name, pkgName+"-") {
			continue
		}

		version := strings.TrimPrefix(name, pkgName+"-")
		ebuilds = append(ebuilds, ebuildInfo{
			path:    filepath.Join(pkgDir, entry.Name()),
			version: version,
		})
	}

	if len(ebuilds) == 0 {
		return "", "", fmt.Errorf("no ebuilds found in %s", pkgDir)
	}

	// Sort by version (highest first) using Portage version comparison
	sort.Slice(ebuilds, func(i, j int) bool {
		cmp := pkg.CompareVersions(ebuilds[i].version, ebuilds[j].version)
		return cmp > 0 // Descending order
	})

	best := ebuilds[0]
	log.Printf("  Best ebuild: %s (version %s)", filepath.Base(best.path), best.version)

	return best.path, best.version, nil
}

// parsePackageAtom parses a package atom and returns category/package.
//
// Accepts formats:
//   - category/package (e.g., "app-misc/hello")
//   - category/package-version (e.g., "app-misc/hello-2.10")
//   - =category/package-version (e.g., "=app-misc/hello-2.10")
func (a *App) parsePackageAtom(atom string) (string, error) {
	// Strip leading operators
	clean := atom
	for _, prefix := range []string{">=", "<=", ">", "<", "=", "~"} {
		if len(clean) > len(prefix) && clean[:len(prefix)] == prefix {
			clean = clean[len(prefix):]
			break
		}
	}

	// Must contain category/package
	if !filepath.IsAbs(clean) && filepath.Dir(clean) == "." {
		return "", fmt.Errorf("missing category in atom: %s", atom)
	}

	// Extract category/package (strip version if present)
	parts := filepath.SplitList(clean)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid atom format: %s", atom)
	}

	// For now, just return the clean atom without version parsing
	// A full implementation would parse versions like Portage does
	return clean, nil
}

// verifyDistfiles checks existing distfiles against their checksums.
func (a *App) verifyDistfiles(distfiles []fetch.Distfile, distDir, atom string) error {
	log.Printf("  Verifying distfiles for %s...", atom)

	allValid := true
	for _, df := range distfiles {
		destPath := filepath.Join(distDir, df.Filename)

		if !fileExistsPath(destPath) {
			fmt.Printf("  [MISSING] %s\n", df.Filename)
			allValid = false
			continue
		}

		if err := fetch.Verify(destPath, df.Checksums); err != nil {
			fmt.Printf("  [FAILED]  %s: %v\n", df.Filename, err)
			allValid = false
			continue
		}

		fmt.Printf("  [OK]      %s\n", df.Filename)
	}

	if !allValid {
		return fmt.Errorf("some distfiles failed verification")
	}

	log.Printf("  All distfiles verified successfully")
	return nil
}

// downloadDistfiles downloads distfiles using the configured fetcher.
func (a *App) downloadDistfiles(distfiles []fetch.Distfile, distDir string, cfg *config.Config) error {
	// Create fetcher with config
	fetcher := a.createFetcherWithConfig(distDir, cfg)

	// Set progress callback for verbose output
	if downloader, ok := fetcher.(*fetch.HTTPDownloader); ok && a.verbose {
		downloader.SetProgressCallback(func(filename string, downloaded, total int64) {
			if total > 0 {
				pct := float64(downloaded) / float64(total) * 100
				log.Printf("    %s: %.1f%% (%d/%d bytes)", filename, pct, downloaded, total)
			} else {
				log.Printf("    %s: %d bytes", filename, downloaded)
			}
		})
	}

	// Download
	ctx := context.Background()
	if err := fetcher.Fetch(ctx, distfiles, distDir); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	log.Printf("  Downloaded %d distfile(s) to %s", len(distfiles), distDir)
	return nil
}

// fileExistsPath checks if a file exists at the given path.
func fileExistsPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
