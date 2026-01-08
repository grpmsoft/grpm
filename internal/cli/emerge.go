package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/daemon"
	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/install"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// runEmerge handles the 'emerge' command - builds packages from source.
//
// Process:
//  1. Resolve dependencies (SAT solver)
//  2. For each package in topological order:
//     - Download sources (if needed)
//     - Execute ebuild phases (unpack, configure, compile, install)
//     - Merge to system
//     - Update VarDB
//
// Parallel builds:
//   - Use --jobs/-j N to build N packages in parallel (default: 1)
//   - Dependencies are respected: a package only builds after its deps complete
//   - Use --keep-going to continue building on failure
func (a *App) runEmerge(args []string) error {
	// Load Portage configuration (make.conf)
	cfg := a.loadPortageConfig()

	// Parse flags with defaults from configuration
	fs := flag.NewFlagSet("emerge", flag.ContinueOnError)
	repoPath := fs.String("repo", cfg.GetPortDir(), "Path to Portage repository")
	distDir := fs.String("distdir", cfg.GetDistDir(), "Directory for downloaded sources")
	tmpDir := fs.String("tmpdir", "/var/tmp/portage", "Temporary build directory")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")
	pretend := fs.Bool("pretend", false, "Show what would be built (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	ask := fs.Bool("ask", false, "Ask for confirmation before building")
	fs.BoolVar(ask, "a", false, "Alias for --ask")
	makeJobs := fs.Int("make-jobs", a.parseJobsFromMakeOpts(cfg.GetMakeOpts()), "Number of parallel make jobs per package")
	parallelBuilds := fs.Int("jobs", 1, "Number of packages to build in parallel")
	fs.IntVar(parallelBuilds, "j", 1, "Alias for --jobs")
	keepGoing := fs.Bool("keep-going", false, "Continue building on failure")
	fs.BoolVar(keepGoing, "k", false, "Alias for --keep-going")
	keepWork := fs.Bool("keep-work", false, "Keep work directory after build")
	enableTests := fs.Bool("test", false, "Run test phase")

	if err := fs.Parse(args); err != nil {
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Validate parallel builds count
	if *parallelBuilds < 1 {
		*parallelBuilds = 1
	}
	if *parallelBuilds > runtime.NumCPU()*2 {
		log.Printf("Warning: --jobs %d exceeds 2x CPU count (%d), this may cause resource contention",
			*parallelBuilds, runtime.NumCPU())
	}

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Resolve dependencies
	log.Println("Calculating dependencies...")
	solution, err := a.resolvePackageDependencies(r, packages)
	if err != nil {
		return err
	}

	if len(solution) == 0 {
		log.Println("Nothing to build")
		return nil
	}

	// Display build plan
	fmt.Println("\n*** Build plan:")
	fmt.Println("*** These are the packages that would be built from source:")
	if *parallelBuilds > 1 {
		fmt.Printf("*** Parallel builds: %d packages at a time\n", *parallelBuilds)
	}
	fmt.Println()
	for name, p := range solution {
		fmt.Printf("[ebuild  N    ] %s-%s USE=\"...\"\n", name, p.Version)
	}
	fmt.Printf("\nTotal: %d package(s)\n", len(solution))

	if *pretend {
		return nil
	}

	if *ask {
		shouldBuild, err := a.askUserConfirmation()
		if err != nil {
			return err
		}
		if !shouldBuild {
			return nil
		}
	}

	// Create fetcher for downloading sources (with mirrors from config)
	fetcher := a.createFetcherWithConfig(*distDir, cfg)

	// Build options for the build function
	buildOpts := &parallelBuildOptions{
		repoPath:    *repoPath,
		distDir:     *distDir,
		tmpDir:      *tmpDir,
		makeJobs:    *makeJobs,
		keepWork:    *keepWork,
		enableTests: *enableTests,
		fetcher:     fetcher,
	}

	// Use parallel scheduler if jobs > 1
	if *parallelBuilds > 1 {
		return a.buildPackagesParallel(solution, *parallelBuilds, *keepGoing, buildOpts)
	}

	// Sequential build (original behavior)
	return a.buildAndInstallPackages(solution, *repoPath, *distDir, *tmpDir, *makeJobs, *keepWork, *enableTests, fetcher)
}

// parallelBuildOptions holds options for parallel build execution.
type parallelBuildOptions struct {
	repoPath    string
	distDir     string
	tmpDir      string
	makeJobs    int
	keepWork    bool
	enableTests bool
	fetcher     fetch.Fetcher
}

// buildPackagesParallel builds packages using the parallel scheduler.
//
// Dependencies are respected: a package only starts building after all its
// dependencies have completed successfully.
func (a *App) buildPackagesParallel(solution map[string]*pkg.Package, parallelJobs int, keepGoing bool, opts *parallelBuildOptions) error {
	log.Printf("\n>>> Starting parallel build with %d workers...", parallelJobs)

	// Get package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Create installer
	installer := install.NewInstaller("/", db)
	installer.Verbose = a.verbose

	// Configure scheduler
	failureMode := daemon.FailureModeStop
	if keepGoing {
		failureMode = daemon.FailureModeContinue
	}

	config := &daemon.SchedulerConfig{
		MaxWorkers:  parallelJobs,
		FailureMode: failureMode,
		Verbose:     a.verbose,
		ProgressCallback: func(stats *daemon.SchedulerStats) {
			log.Printf(">>> %s", daemon.FormatStats(stats))
		},
	}

	scheduler := daemon.NewBuildScheduler(config)

	// Create tasks for all packages
	taskMap := make(map[string]*daemon.BuildTask)
	for _, p := range solution {
		task := daemon.NewBuildTask(p)
		taskMap[p.Name] = task

		// Capture variables for closure
		pkgCopy := p
		task.BuildFunc = func(ctx context.Context, buildPkg *pkg.Package) error {
			return a.buildAndInstallSinglePackage(ctx, buildPkg, installer, opts)
		}
		_ = pkgCopy // Ensure pkgCopy is used

		if err := scheduler.AddTask(task); err != nil {
			return fmt.Errorf("failed to add task for %s: %w", p.Name, err)
		}
	}

	// Add dependencies
	for _, p := range solution {
		taskID := p.Name + "-" + p.Version
		for _, dep := range p.Deps {
			depPkg, exists := solution[dep.Name]
			if exists {
				depTaskID := depPkg.Name + "-" + depPkg.Version
				if err := scheduler.AddDependency(taskID, depTaskID); err != nil {
					if a.verbose {
						log.Printf("Warning: could not add dependency %s -> %s: %v", taskID, depTaskID, err)
					}
				}
			}
		}
	}

	// Show build order
	if a.verbose {
		buildOrder := scheduler.GetBuildOrder()
		if len(buildOrder) == 0 {
			// Trigger topological sort by starting (will compute order)
			log.Println("Computing build order...")
		}
	}

	// Execute builds
	ctx := context.Background()
	err = scheduler.Start(ctx)

	// Report final stats
	stats := scheduler.GetStats()
	log.Printf("\n>>> Build summary:")
	log.Printf("    Completed: %d/%d packages", stats.CompletedTasks, stats.TotalTasks)
	if stats.FailedTasks > 0 {
		log.Printf("    Failed: %d packages", stats.FailedTasks)
	}
	if stats.CanceledTasks > 0 {
		log.Printf("    Canceled: %d packages", stats.CanceledTasks)
	}
	log.Printf("    Total time: %s", stats.ElapsedTime.Round(1000000000))

	if err != nil {
		failedTask := scheduler.GetFailedTask()
		if failedTask != nil {
			return fmt.Errorf("emerge failed: %s: %s", failedTask.ID, failedTask.GetError())
		}
		return fmt.Errorf("emerge failed: %w", err)
	}

	log.Printf("\n>>> Emerge completed successfully: %d package(s) built and installed", stats.CompletedTasks)
	return nil
}

// buildAndInstallSinglePackage builds and installs a single package.
// Used by both sequential and parallel build paths.
func (a *App) buildAndInstallSinglePackage(ctx context.Context, p *pkg.Package, installer *install.Installer, opts *parallelBuildOptions) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	log.Printf(">>> Building %s-%s", p.Name, p.Version)

	// Build from source
	imageDir, err := a.buildPackageFromSource(p, opts.repoPath, opts.distDir, opts.tmpDir, opts.makeJobs, opts.keepWork, opts.enableTests, opts.fetcher)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Check context cancellation before install
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Install to system
	if err := a.installFromImageDir(installer, p, imageDir, opts.keepWork); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	return nil
}

// loadPortageConfig loads Portage configuration from /etc/portage.
//
// Returns a default configuration if loading fails.
// Errors are logged but not fatal - missing make.conf is common in tests.
func (a *App) loadPortageConfig() *config.Config {
	cfg, err := config.LoadConfig("/etc/portage")
	if err != nil {
		if a.verbose {
			log.Printf("Warning: failed to load Portage config: %v", err)
		}
		// Return config with defaults
		return &config.Config{
			Root:     "/etc/portage",
			MakeConf: config.DefaultMakeConf(),
		}
	}
	return cfg
}

// parseJobsFromMakeOpts extracts the -j value from MAKEOPTS.
//
// Parses MAKEOPTS like "-j4" or "-j8 -l4" and returns the job count.
// Returns 4 as default if parsing fails or MAKEOPTS is empty.
func (a *App) parseJobsFromMakeOpts(makeOpts string) int {
	if makeOpts == "" {
		return 4
	}

	// Parse -jN from MAKEOPTS
	var jobs int
	for i := 0; i < len(makeOpts)-2; i++ {
		if makeOpts[i] == '-' && makeOpts[i+1] == 'j' {
			// Parse the number after -j
			numStart := i + 2
			numEnd := numStart
			for numEnd < len(makeOpts) && makeOpts[numEnd] >= '0' && makeOpts[numEnd] <= '9' {
				numEnd++
			}
			if numEnd > numStart {
				for j := numStart; j < numEnd; j++ {
					jobs = jobs*10 + int(makeOpts[j]-'0')
				}
				if jobs > 0 {
					return jobs
				}
			}
		}
	}

	return 4 // Default
}

// createFetcherWithConfig creates an HTTPDownloader using Portage configuration.
//
// Uses GENTOO_MIRRORS from make.conf if configured, otherwise falls back
// to DefaultMirrors. This is the preferred method for creating fetchers.
func (a *App) createFetcherWithConfig(distDir string, cfg *config.Config) fetch.Fetcher {
	// Get mirrors from config, fall back to defaults
	mirrors := cfg.GetGentooMirrors()
	if len(mirrors) == 0 {
		mirrors = fetch.DefaultMirrors
		if a.verbose {
			log.Printf("Using default Gentoo mirrors (GENTOO_MIRRORS not configured)")
		}
	} else if a.verbose {
		log.Printf("Using %d configured Gentoo mirror(s) from make.conf", len(mirrors))
	}

	fetchConfig := fetch.DefaultConfig().
		WithDistDir(distDir).
		WithMirrors(mirrors)

	downloader := fetch.NewHTTPDownloader(fetchConfig)

	// Set progress callback for verbose output
	if a.verbose {
		downloader.SetProgressCallback(func(filename string, downloaded, total int64) {
			if total > 0 {
				percent := float64(downloaded) / float64(total) * 100
				log.Printf("  %s: %.1f%% (%d/%d bytes)", filename, percent, downloaded, total)
			} else {
				log.Printf("  %s: %d bytes", filename, downloaded)
			}
		})
	}

	return downloader
}

// createFetcher creates an HTTPDownloader for fetching distfiles.
//
// Deprecated: Use createFetcherWithConfig instead.
// This method is kept for backward compatibility.
func (a *App) createFetcher(distDir string) fetch.Fetcher {
	cfg := a.loadPortageConfig()
	return a.createFetcherWithConfig(distDir, cfg)
}

// buildAndInstallPackages builds packages from source and installs them.
func (a *App) buildAndInstallPackages(solution map[string]*pkg.Package, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests bool, fetcher fetch.Fetcher) error {
	log.Println("\n>>> Starting source build...")

	builtCount := 0
	totalPackages := len(solution)

	// Get package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Create installer
	installer := install.NewInstaller("/", db)
	installer.Verbose = a.verbose

	for name, p := range solution {
		log.Printf("\n>>> (%d/%d) Emerging %s-%s", builtCount+1, totalPackages, name, p.Version)

		// Build from source (fetcher will download sources automatically)
		imageDir, err := a.buildPackageFromSource(p, repoPath, distDir, tmpDir, jobs, keepWork, enableTests, fetcher)
		if err != nil {
			return fmt.Errorf("failed to build %s: %w", name, err)
		}

		// Install to system
		if err := a.installFromImageDir(installer, p, imageDir, keepWork); err != nil {
			return fmt.Errorf("failed to install %s: %w", name, err)
		}

		builtCount++
		log.Printf(">>> %s-%s merged successfully (%d/%d)", name, p.Version, builtCount, totalPackages)
	}

	log.Printf("\n>>> Emerge completed successfully: %d package(s) built and installed", builtCount)
	return nil
}

// buildPackageFromSource builds a package from source using ebuild executor.
//
// Returns the image directory (D) where files are installed.
// If fetcher is provided, source tarballs are downloaded automatically.
func (a *App) buildPackageFromSource(p *pkg.Package, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests bool, fetcher fetch.Fetcher) (string, error) {
	// Find ebuild file
	ebuildPath := a.findEbuildFile(p, repoPath)
	if ebuildPath == "" && a.verbose {
		log.Printf("Warning: No ebuild file found for %s-%s, using defaults", p.Name, p.Version)
	}

	// Create executor options with fetcher for automatic distfile download
	opts := ebuild.ExecutorOptions{
		TmpDir:        tmpDir,
		PortDir:       repoPath,
		DistDir:       distDir,
		EbuildPath:    ebuildPath,
		EnableSandbox: true,
		EnableTests:   enableTests,
		KeepWork:      keepWork,
		Fetcher:       fetcher,
	}

	// Create ebuild executor
	executor, err := ebuild.NewExecutor(p, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	// Set progress callback
	executor.OnProgress = func(phase ebuild.Phase, status string) {
		if a.verbose {
			log.Printf("  [%s] %s", phase, status)
		}
	}

	// Set MAKEOPTS for parallel builds
	executor.Env.MAKEOPTS = fmt.Sprintf("-j%d", jobs)

	// Execute build phases (fetch happens automatically before unpack)
	log.Printf(">>> Fetching sources...")
	log.Printf(">>> Unpacking sources...")
	log.Printf(">>> Configuring...")
	log.Printf(">>> Compiling...")
	log.Printf(">>> Installing to temporary directory...")

	phases := ebuild.StandardPhases()
	results, err := executor.ExecutePhases(phases)
	if err != nil {
		// Print build log on failure
		log.Printf("Build failed! Phase results:")
		for _, result := range results {
			if !result.Success {
				log.Printf("  Phase %s failed: %v", result.Phase, result.Error)
				if result.Output != "" {
					log.Printf("  Output: %s", result.Output)
				}
			}
		}
		return "", fmt.Errorf("build failed: %w", err)
	}

	if a.verbose {
		log.Printf("Build successful! Results:")
		for _, result := range results {
			log.Printf("  Phase %s: %dms", result.Phase, result.Duration)
		}
	}

	return executor.GetImageDirectory(), nil
}

// installFromImageDir installs package files from the image directory to the system.
func (a *App) installFromImageDir(installer *install.Installer, p *pkg.Package, imageDir string, keepWork bool) error {
	// Create work directory structure (installer expects workDir with /image subdirectory)
	workDir := filepath.Dir(imageDir)

	// Install using Installer
	opts := install.InstallOptions{
		WorkDir:  workDir,
		Replace:  false,
		Force:    false,
		Pretend:  false,
		KeepWork: keepWork,
	}

	if err := installer.Install(p, opts); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

// findEbuildFile finds the ebuild file for a package.
//
// Searches in: /var/db/repos/gentoo/<category>/<package>/<package>-<version>.ebuild
func (a *App) findEbuildFile(p *pkg.Package, repoPath string) string {
	category := filepath.Dir(p.Name)
	pkgName := filepath.Base(p.Name)

	ebuildName := fmt.Sprintf("%s-%s.ebuild", pkgName, p.Version)
	ebuildPath := filepath.Join(repoPath, category, pkgName, ebuildName)

	// Check if exists
	if _, err := os.Stat(ebuildPath); err == nil {
		return ebuildPath
	}

	return ""
}
