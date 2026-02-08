package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/distfile"
	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
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

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("fetch")) }

	if err := fs.Parse(reorderArgs(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Expand set references (@world, @selected, @system)
	packages, err := a.expandPackageArgs(packages)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		a.log.Info("No packages in specified set(s)")
		return nil
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
// Uses distfile.Service for unified SRC_URI resolution (single source of truth).
func (a *App) fetchPackageDistfiles(atom, repoPath, distDir string, pretend, verifyOnly bool, cfg *config.Config) error {
	logging.Action("Fetching distfiles for %s", atom)

	// Use unified distfile service
	evaluator := ebuild.NewSrcURIEvaluator()
	svc := distfile.NewService(repoPath, evaluator)
	ctx := context.Background()

	distfiles, err := svc.ResolveDistfilesForAtom(ctx, atom)
	if err != nil {
		logging.Warn("could not resolve distfiles: %v", err)
		logging.Info("  Falling back to manifest-only downloads")

		// Fallback: parse manifest directly
		parsedAtom, parseErr := pkg.ParseAtom(atom)
		if parseErr != nil {
			return fmt.Errorf("invalid package atom %q: %w", atom, parseErr)
		}
		manifestPath := fetch.ManifestPath(repoPath, parsedAtom.CP())
		manifest, manifestErr := fetch.ParseManifest(manifestPath)
		if manifestErr != nil {
			return fmt.Errorf("failed to parse Manifest: %w", manifestErr)
		}
		distfiles = manifest.GetDistfiles()
	}

	if len(distfiles) == 0 {
		logging.Info("  No distfiles for %s", atom)
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

// verifyDistfiles checks existing distfiles against their checksums.
func (a *App) verifyDistfiles(distfiles []fetch.Distfile, distDir, atom string) error {
	logging.Info("  Verifying distfiles for %s...", atom)

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

	logging.Info("  All distfiles verified successfully")
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
				logging.Verbose("    %s: %.1f%% (%d/%d bytes)", filename, pct, downloaded, total)
			} else {
				logging.Verbose("    %s: %d bytes", filename, downloaded)
			}
		})
	}

	// Download
	ctx := context.Background()
	if err := fetcher.Fetch(ctx, distfiles, distDir); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	logging.Info("  Downloaded %d distfile(s) to %s", len(distfiles), distDir)
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
