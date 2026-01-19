package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/daemon"
	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/install"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/tools"
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
	checkTools := fs.Bool("check-tools", false, "Perform optional pre-build tool availability check")
	replace := fs.Bool("replace", false, "Replace existing package (ignore collisions with same package)")
	fs.BoolVar(replace, "R", false, "Alias for --replace")
	force := fs.Bool("force", false, "Force installation (skip collision checks for untracked files)")
	fs.BoolVar(force, "f", false, "Alias for --force")
	rootPath := fs.String("root", "/", "Installation root directory (like $ROOT in Portage)")
	onlyDeps := fs.Bool("onlydeps", false, "Build dependencies only, not the target package(s)")
	fs.BoolVar(onlyDeps, "o", false, "Alias for --onlydeps")
	showInfo := fs.Bool("info", false, "Show system environment information")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // Help was requested, not an error
		}
		return err
	}

	// Handle --info (doesn't require packages)
	if *showInfo {
		return a.runSystemInfo(cfg, *repoPath)
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

	// Validate parallel builds count
	if *parallelBuilds < 1 {
		*parallelBuilds = 1
	}
	if *parallelBuilds > runtime.NumCPU()*2 {
		logging.Warn("--jobs %d exceeds 2x CPU count (%d), this may cause resource contention",
			*parallelBuilds, runtime.NumCPU())
	}

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Resolve dependencies
	logging.Action("Calculating dependencies...")
	solution, err := a.resolvePackageDependencies(r, packages)
	if err != nil {
		return err
	}

	// Filter out target packages if --onlydeps is specified
	if *onlyDeps {
		solution = a.filterTargetPackages(solution, packages)
		if len(solution) == 0 {
			logging.Info("No dependencies to build (--onlydeps specified)")
			return nil
		}
	}

	if len(solution) == 0 {
		logging.Info("Nothing to build")
		return nil
	}

	// Check external tool availability (opt-in)
	// NOTE: Tool dependencies are normally handled via BDEPEND like Portage.
	// This optional check provides early validation before build starts.
	if *checkTools {
		if err := a.checkBuildTools(solution, *repoPath); err != nil {
			return err
		}
	}

	// Display build plan
	fmt.Println("\n*** Build plan:")
	fmt.Println("*** These are the packages that would be built from source:")
	if *parallelBuilds > 1 {
		fmt.Printf("*** Parallel builds: %d packages at a time\n", *parallelBuilds)
	}
	fmt.Println()
	for name, p := range solution {
		useStr := FormatUSEFlags(p, cfg)
		fmt.Printf("[ebuild  N    ] %s-%s %s\n", name, p.Version, useStr)
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
		replace:     *replace,
		force:       *force,
		root:        *rootPath,
		fetcher:     fetcher,
	}

	// Use parallel scheduler if jobs > 1
	if *parallelBuilds > 1 {
		return a.buildPackagesParallel(solution, *parallelBuilds, *keepGoing, buildOpts)
	}

	// Sequential build (original behavior)
	return a.buildAndInstallPackages(solution, *repoPath, *distDir, *tmpDir, *makeJobs, *keepWork, *enableTests, *replace, *force, *rootPath, fetcher)
}

// parallelBuildOptions holds options for parallel build execution.
type parallelBuildOptions struct {
	repoPath    string
	distDir     string
	tmpDir      string
	makeJobs    int
	keepWork    bool
	enableTests bool
	replace     bool
	force       bool
	root        string
	fetcher     fetch.Fetcher
}

// buildPackagesParallel builds packages using the parallel scheduler.
//
// Dependencies are respected: a package only starts building after all its
// dependencies have completed successfully.
func (a *App) buildPackagesParallel(solution map[string]*pkg.Package, parallelJobs int, keepGoing bool, opts *parallelBuildOptions) error {
	logging.Action("Starting parallel build with %d workers...", parallelJobs)

	// Get package database (with root prefix)
	db, err := a.getOrCreatePackageDBWithRoot(opts.root)
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Create installer with custom root
	installer := install.NewInstaller(opts.root, db)
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
			logging.Info("%s", daemon.FormatStats(stats))
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
						logging.Warn("could not add dependency %s -> %s: %v", taskID, depTaskID, err)
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
			logging.Debug("Computing build order...")
		}
	}

	// Execute builds
	ctx := context.Background()
	err = scheduler.Start(ctx)

	// Report final stats
	stats := scheduler.GetStats()
	logging.Info("")
	logging.Action("Build summary:")
	logging.Info("    Completed: %d/%d packages", stats.CompletedTasks, stats.TotalTasks)
	if stats.FailedTasks > 0 {
		logging.Warn("    Failed: %d packages", stats.FailedTasks)
	}
	if stats.CanceledTasks > 0 {
		logging.Warn("    Canceled: %d packages", stats.CanceledTasks)
	}
	logging.Info("    Total time: %s", stats.ElapsedTime.Round(1000000000))

	if err != nil {
		failedTask := scheduler.GetFailedTask()
		if failedTask != nil {
			return fmt.Errorf("emerge failed: %s: %s", failedTask.ID, failedTask.GetError())
		}
		return fmt.Errorf("emerge failed: %w", err)
	}

	logging.Action("Emerge completed successfully: %d package(s) built and installed", stats.CompletedTasks)
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

	logging.Action("Building %s-%s", p.Name, p.Version)

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
	if err := a.installFromImageDir(installer, p, imageDir, opts.keepWork, opts.replace, opts.force); err != nil {
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
			logging.Warn("failed to load Portage config: %v", err)
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
			logging.Debug("Using default Gentoo mirrors (GENTOO_MIRRORS not configured)")
		}
	} else if a.verbose {
		logging.Debug("Using %d configured Gentoo mirror(s) from make.conf", len(mirrors))
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
				logging.Verbose("  %s: %.1f%% (%d/%d bytes)", filename, percent, downloaded, total)
			} else {
				logging.Verbose("  %s: %d bytes", filename, downloaded)
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
func (a *App) buildAndInstallPackages(solution map[string]*pkg.Package, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests, replace, force bool, root string, fetcher fetch.Fetcher) error {
	logging.Action("Starting source build...")

	builtCount := 0
	totalPackages := len(solution)

	// Get package database (with root prefix)
	db, err := a.getOrCreatePackageDBWithRoot(root)
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Create installer with custom root
	installer := install.NewInstaller(root, db)
	installer.Verbose = a.verbose

	for name, p := range solution {
		logging.Action("(%d/%d) Emerging %s-%s", builtCount+1, totalPackages, name, p.Version)

		// Build from source (fetcher will download sources automatically)
		imageDir, err := a.buildPackageFromSource(p, repoPath, distDir, tmpDir, jobs, keepWork, enableTests, fetcher)
		if err != nil {
			return fmt.Errorf("failed to build %s: %w", name, err)
		}

		// Install to system
		if err := a.installFromImageDir(installer, p, imageDir, keepWork, replace, force); err != nil {
			return fmt.Errorf("failed to install %s: %w", name, err)
		}

		builtCount++
		logging.Action("%s-%s merged successfully (%d/%d)", name, p.Version, builtCount, totalPackages)
	}

	logging.Action("Emerge completed successfully: %d package(s) built and installed", builtCount)
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
		logging.Warn("No ebuild file found for %s-%s, using defaults", p.Name, p.Version)
	}

	// Create executor options with fetcher for automatic distfile download
	// NOTE: KeepWork is always true here because we need the image directory
	// for installation. Cleanup happens after install in installFromImageDir.
	opts := ebuild.ExecutorOptions{
		TmpDir:        tmpDir,
		PortDir:       repoPath,
		DistDir:       distDir,
		EbuildPath:    ebuildPath,
		EnableSandbox: true,
		EnableTests:   enableTests,
		KeepWork:      true, // Must be true - cleanup after install
		Fetcher:       fetcher,
	}

	// Create ebuild executor
	executor, err := ebuild.NewExecutor(p, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	// Parse ebuild to detect custom phase functions
	if err := executor.ParseEbuild(); err != nil {
		if a.verbose {
			logging.Warn("failed to parse ebuild: %v", err)
		}
		// Continue anyway - will use default phases
	}

	// Set progress callback
	executor.OnProgress = func(phase ebuild.Phase, status string) {
		if a.verbose {
			logging.Verbose("  [%s] %s", phase, status)
		}
	}

	// Set MAKEOPTS for parallel builds
	executor.Env.MAKEOPTS = fmt.Sprintf("-j%d", jobs)

	// Execute build phases (fetch happens automatically before unpack)
	logging.Action("Fetching sources...")
	logging.Action("Unpacking sources...")
	logging.Action("Configuring...")
	logging.Action("Compiling...")
	logging.Action("Installing to temporary directory...")

	phases := ebuild.StandardPhases()
	results, err := executor.ExecutePhases(phases)
	if err != nil {
		// Print build log on failure
		logging.Error("Build failed! Phase results:")
		for _, result := range results {
			if !result.Success {
				logging.Error("  Phase %s failed: %v", result.Phase, result.Error)
				if result.Output != "" {
					logging.Error("  Output: %s", result.Output)
				}
			}
		}
		return "", fmt.Errorf("build failed: %w", err)
	}

	if a.verbose {
		logging.Debug("Build successful! Results:")
		for _, result := range results {
			logging.Debug("  Phase %s: %dms", result.Phase, result.Duration)
		}
	}

	return executor.GetImageDirectory(), nil
}

// installFromImageDir installs package files from the image directory to the system.
func (a *App) installFromImageDir(installer *install.Installer, p *pkg.Package, imageDir string, keepWork, replace, force bool) error {
	// Create work directory structure (installer expects workDir with /image subdirectory)
	workDir := filepath.Dir(imageDir)

	// Install using Installer
	opts := install.InstallOptions{
		WorkDir:  workDir,
		Replace:  replace,
		Force:    force,
		Pretend:  false,
		KeepWork: keepWork,
	}

	if err := installer.Install(p, opts); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Cleanup work directory after successful install (unless keepWork is set)
	if !keepWork {
		if err := os.RemoveAll(workDir); err != nil {
			// Non-fatal - log warning but don't fail
			logging.Warn("failed to cleanup work directory %s: %v", workDir, err)
		}
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

// filterTargetPackages removes the target packages from the solution,
// leaving only their dependencies. Used for --onlydeps flag.
//
// The packages parameter contains atom strings (e.g., "app-misc/hello", "=sys-devel/gcc-13.4.1").
// Each atom is parsed to extract the package name (category/package), and matching
// packages are removed from the solution.
func (a *App) filterTargetPackages(solution map[string]*pkg.Package, packages []string) map[string]*pkg.Package {
	// Build a set of target package names from atoms
	targetNames := make(map[string]bool)
	for _, atomStr := range packages {
		atom, err := pkg.ParseAtom(atomStr)
		if err != nil {
			// If parsing fails, try using the string directly as package name
			targetNames[atomStr] = true
			continue
		}
		targetNames[atom.CP()] = true
	}

	// Filter out target packages
	filtered := make(map[string]*pkg.Package)
	for name, p := range solution {
		if !targetNames[name] {
			filtered[name] = p
		} else if a.verbose {
			logging.Debug("--onlydeps: excluding target package %s-%s", name, p.Version)
		}
	}

	return filtered
}

// checkBuildTools verifies that all required external tools are available.
//
// Analyzes the ebuilds in the solution to determine which eclasses are inherited,
// then checks if the tools required by those eclasses are present on the system.
//
// This uses per-package tool checking via CheckForEclassesOnly(), which only
// checks tools explicitly mapped to the inherited eclasses. This prevents
// packages like sys-libs/glibc from requiring Rust, Java, or Ruby when they
// don't actually need those tools.
//
// Returns an error if required tools are missing, with suggestions for installation.
func (a *App) checkBuildTools(solution map[string]*pkg.Package, repoPath string) error {
	logging.Debug("Checking external tool availability...")

	checker := tools.NewChecker()

	// Collect all eclasses from all packages
	allEclasses := make(map[string]bool)
	for _, p := range solution {
		ebuildPath := a.findEbuildFile(p, repoPath)
		if ebuildPath == "" {
			continue
		}

		// Read ebuild and extract inherit
		content, err := os.ReadFile(ebuildPath)
		if err != nil {
			if a.verbose {
				logging.Warn("could not read ebuild %s: %v", ebuildPath, err)
			}
			continue
		}

		eclasses := tools.ExtractInherit(string(content))
		for _, eclass := range eclasses {
			allEclasses[eclass] = true
		}
	}

	if len(allEclasses) == 0 {
		if a.verbose {
			logging.Debug("No eclasses detected, skipping tool check")
		}
		return nil
	}

	// Convert to slice
	eclassList := make([]string, 0, len(allEclasses))
	for eclass := range allEclasses {
		eclassList = append(eclassList, eclass)
	}

	// Check tools ONLY for the specific eclasses inherited by these packages.
	// This uses CheckForEclassesOnly() instead of CheckForEclasses() to avoid
	// requiring tools from the global registry (e.g., Rust for cargo eclass)
	// when the packages don't actually inherit those eclasses.
	result := checker.CheckForEclassesOnly(eclassList)

	if a.verbose {
		logging.Debug("Detected eclasses: %v", eclassList)
		logging.Debug("Required tools: %d, available: %d, missing: %d",
			len(result.Required), len(result.Available), len(result.Missing))
	}

	if !result.CanBuild {
		fmt.Println()
		fmt.Println("*** Error: Missing required build tools")
		fmt.Println()
		fmt.Print(tools.FormatMissingTools(result.Missing))
		fmt.Println()
		fmt.Println("Install missing tools via: grpm emerge <package>")
		fmt.Println("Or install system packages as suggested above.")
		return fmt.Errorf("missing %d required build tool(s)", len(result.Missing))
	}

	if a.verbose && len(result.Required) > 0 {
		logging.Debug("All %d required tools are available", len(result.Required))
	}

	return nil
}

// runSystemInfo displays system environment information (emerge --info).
//
// This is the GRPM equivalent of Portage's "emerge --info" command.
// It displays:
//   - GRPM version and Go version
//   - System uname and memory info
//   - Key installed packages (gcc, glibc, binutils, etc.)
//   - Repository information
//   - Configuration variables (CFLAGS, USE, etc.)
func (a *App) runSystemInfo(cfg *config.Config, repoPath string) error {
	info := GatherSystemInfo(a.version, cfg, repoPath)
	fmt.Print(FormatSystemInfo(info))
	return nil
}
