package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/daemon"
	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/install"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
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
//
//nolint:gocyclo // CLI entry point with many flags and modes — splitting would fragment user-facing logic.
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
	noDeps := fs.Bool("nodeps", false, "Skip dependency resolution, build only specified packages")
	fs.BoolVar(noDeps, "O", false, "Alias for --nodeps")
	showInfo := fs.Bool("info", false, "Show system environment information")

	// Dependency resolution options (Portage-compatible)
	deep := fs.Bool("deep", false, "Traverse dependencies of already-installed packages")
	fs.BoolVar(deep, "D", false, "Alias for --deep")
	withBdeps := fs.Bool("with-bdeps", false, "Include build-time dependencies for installed packages")
	emptyTree := fs.Bool("emptytree", false, "Assume no packages are installed (full dependency tree)")
	fs.BoolVar(emptyTree, "e", false, "Alias for --emptytree")
	varDbPath := fs.String("vardb", "/var/db/pkg", "Path to installed packages database")

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("emerge")) }

	if err := fs.Parse(reorderArgs(args)); err != nil {
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

	// When using --deep, installed packages will be in the solution — implicitly enable replace
	if *deep && !*replace {
		*replace = true
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

	var solution map[string]*pkg.Package

	if *noDeps {
		// --nodeps: skip resolution, just find the best acceptable version
		logging.Action("Skipping dependency resolution (--nodeps)...")
		acceptKeywords := []string{"amd64", "~amd64"}
		if cfg != nil && len(cfg.MakeConf.ACCEPT_KEYWORDS) > 0 {
			acceptKeywords = cfg.MakeConf.ACCEPT_KEYWORDS
		}
		solution = make(map[string]*pkg.Package)
		for _, name := range packages {
			found, loadErr := a.loadBestAcceptableVersion(r, name, acceptKeywords)
			if loadErr != nil || found == nil {
				logging.Warn("Package %s not found: %v", name, loadErr)
				continue
			}
			key := fmt.Sprintf("%s-%s", found.Name, found.Version)
			solution[key] = found
		}
	} else {
		// Resolve dependencies with Portage-compatible filtering
		logging.Action("Calculating dependencies...")
		var resolveErr error
		solution, resolveErr = a.resolvePackageDependenciesWithOptions(r, packages, solver.ResolveOptions{
			Deep:      *deep,
			WithBdeps: *withBdeps,
			EmptyTree: *emptyTree || *useMock, // Mock mode implies emptytree
		}, *varDbPath, *useMock)
		if resolveErr != nil {
			return resolveErr
		}

		// Filter out target packages if --onlydeps is specified
		if *onlyDeps {
			solution = a.filterTargetPackages(solution, packages)
			if len(solution) == 0 {
				logging.Info("No dependencies to build (--onlydeps specified)")
				return nil
			}
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
	for _, p := range solution {
		useStr := FormatUSEFlags(p, cfg)
		fmt.Printf("[ebuild  N    ] %s-%s %s\n", p.Name, p.Version, useStr)
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

	// Resolve effective USE flags for each package before building.
	// This applies make.conf USE, USE_EXPAND variables (PYTHON_TARGETS, etc.),
	// and per-package USE from package.use to each package's UseFlags map.
	// Per Portage: USE_EXPAND vars like PYTHON_TARGETS="python3_12" are converted
	// to USE flags like python_targets_python3_12.
	for _, p := range solution {
		ApplyEffectiveUSE(p, cfg)
	}

	// Create fetcher for downloading sources (with mirrors from config)
	fetcher := a.createFetcherWithConfig(*distDir, cfg)

	// Build options for the build function
	buildOpts := &parallelBuildOptions{
		repoPath:      *repoPath,
		distDir:       *distDir,
		tmpDir:        *tmpDir,
		makeJobs:      *makeJobs,
		keepWork:      *keepWork,
		enableTests:   *enableTests,
		replace:       *replace,
		force:         *force,
		root:          *rootPath,
		fetcher:       fetcher,
		useExpandVars: GetUSEExpandVars(cfg),
		cfg:           cfg,
	}

	// Use parallel scheduler if jobs > 1
	if *parallelBuilds > 1 {
		return a.buildPackagesParallel(solution, *parallelBuilds, *keepGoing, buildOpts)
	}

	// Sequential build (original behavior)
	return a.buildAndInstallPackages(solution, *repoPath, *distDir, *tmpDir, *makeJobs, *keepWork, *enableTests, *replace, *force, *rootPath, *keepGoing, fetcher, buildOpts.useExpandVars, cfg)
}

// parallelBuildOptions holds options for parallel build execution.
type parallelBuildOptions struct {
	repoPath      string
	distDir       string
	tmpDir        string
	makeJobs      int
	keepWork      bool
	enableTests   bool
	replace       bool
	force         bool
	root          string
	fetcher       fetch.Fetcher
	useExpandVars map[string]string // USE_EXPAND variables (e.g., PYTHON_TARGETS="python3_12")
	cfg           *config.Config    // Portage configuration for CFLAGS, etc.
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
	imageDir, err := a.buildPackageFromSource(p, opts.repoPath, opts.distDir, opts.tmpDir, opts.makeJobs, opts.keepWork, opts.enableTests, opts.fetcher, opts.useExpandVars, opts.cfg)
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
func (a *App) buildAndInstallPackages(solution map[string]*pkg.Package, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests, replace, force bool, root string, keepGoing bool, fetcher fetch.Fetcher, useExpandVars map[string]string, cfg *config.Config) error {
	logging.Action("Starting source build...")

	builtCount := 0
	failedCount := 0
	totalPackages := len(solution)
	var failedPkgs []string

	// Get package database (with root prefix)
	db, err := a.getOrCreatePackageDBWithRoot(root)
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Create installer with custom root
	installer := install.NewInstaller(root, db)
	installer.Verbose = a.verbose

	pkgNum := 0
	for name, p := range solution {
		pkgNum++
		logging.Action("(%d/%d) Emerging %s-%s", pkgNum, totalPackages, p.Name, p.Version)

		buildErr := a.buildAndInstallSingle(name, p, installer, repoPath, distDir, tmpDir, jobs, keepWork, enableTests, replace, force, fetcher, useExpandVars, cfg)
		if buildErr != nil {
			if keepGoing {
				logging.Error("failed to emerge %s: %v", name, buildErr)
				failedCount++
				failedPkgs = append(failedPkgs, name)
				continue
			}
			return fmt.Errorf("failed to emerge %s: %w", name, buildErr)
		}

		builtCount++
		logging.Action("%s-%s merged successfully (%d/%d)", p.Name, p.Version, builtCount, totalPackages)
	}

	if failedCount > 0 {
		logging.Error("Emerge completed with %d failure(s) out of %d package(s):", failedCount, totalPackages)
		for _, name := range failedPkgs {
			logging.Error("  - %s", name)
		}
		return fmt.Errorf("%d package(s) failed to build", failedCount)
	}

	logging.Action("Emerge completed successfully: %d package(s) built and installed", builtCount)
	return nil
}

// buildAndInstallSingle builds and installs a single package with panic recovery.
// This prevents interpreter panics (e.g., unsupported bash features) from
// crashing the entire emerge process when --keep-going is used.
func (a *App) buildAndInstallSingle(name string, p *pkg.Package, installer *install.Installer, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests, replace, force bool, fetcher fetch.Fetcher, useExpandVars map[string]string, cfg *config.Config) (buildErr error) {
	defer func() {
		if r := recover(); r != nil {
			buildErr = fmt.Errorf("internal error (panic): %v", r)
		}
	}()

	imageDir, err := a.buildPackageFromSource(p, repoPath, distDir, tmpDir, jobs, keepWork, enableTests, fetcher, useExpandVars, cfg)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if err := a.installFromImageDir(installer, p, imageDir, keepWork, replace, force); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	return nil
}

// buildPackageFromSource builds a package from source using ebuild executor.
//
// Returns the image directory (D) where files are installed.
// If fetcher is provided, source tarballs are downloaded automatically.
//
//nolint:gocyclo // Build orchestrator with many setup steps — splitting would hurt readability.
func (a *App) buildPackageFromSource(p *pkg.Package, repoPath, distDir, tmpDir string, jobs int, keepWork, enableTests bool, fetcher fetch.Fetcher, useExpandVars map[string]string, cfg *config.Config) (string, error) {
	// Find ebuild file
	ebuildPath := a.findEbuildFile(p, repoPath)
	if ebuildPath == "" && a.verbose {
		logging.Warn("No ebuild file found for %s-%s, using defaults", p.Name, p.Version)
	}

	// Create executor options with fetcher for automatic distfile download
	// NOTE: KeepWork is always true here because we need the image directory
	// for installation. Cleanup happens after install in installFromImageDir.
	opts := ebuild.DefaultOptions()
	opts.TmpDir = tmpDir
	opts.PortDir = repoPath
	opts.DistDir = distDir
	opts.EbuildPath = ebuildPath
	opts.EnableSandbox = true
	opts.EnableTests = enableTests
	opts.KeepWork = true     // Must be true - cleanup after install
	opts.DenyNetwork = false // Must be false - fetcher needs network access
	opts.Fetcher = fetcher

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

	// Apply make.conf build variables (CFLAGS, CXXFLAGS, LDFLAGS).
	// os.Getenv in NewEnvironment may return empty since GRPM runs standalone,
	// not through a Portage profile that sources make.conf.
	if cfg != nil && cfg.MakeConf != nil {
		if executor.Env.CFLAGS == "" && cfg.MakeConf.CFLAGS != "" {
			executor.Env.CFLAGS = cfg.MakeConf.CFLAGS
		}
		if executor.Env.CXXFLAGS == "" && cfg.MakeConf.CXXFLAGS != "" {
			executor.Env.CXXFLAGS = cfg.MakeConf.CXXFLAGS
		}
		if executor.Env.LDFLAGS == "" && cfg.MakeConf.LDFLAGS != "" {
			executor.Env.LDFLAGS = cfg.MakeConf.LDFLAGS
		}
	}

	// Set USE_EXPAND variables as separate environment variables.
	// Per Portage: PYTHON_TARGETS, PYTHON_SINGLE_TARGET, etc. are set
	// both as USE flags (python_targets_python3_12 in USE) and as
	// separate variables (PYTHON_TARGETS="python3_12").
	if executor.Env.ExtraVars == nil {
		executor.Env.ExtraVars = make(map[string]string)
	}
	for k, v := range useExpandVars {
		executor.Env.ExtraVars[k] = v
	}

	// Set multilib/ABI variables from profile defaults.
	// These come from profiles/arch/amd64/make.defaults in Portage.
	// Without them, multilib_foreach_abi fails with "no ABIs enabled".
	// Note: MULTILIB_ABIS only includes "amd64" because ABI_X86="64"
	// means only 64-bit is enabled. The x86 (32-bit) ABI requires
	// ABI_X86="64 32" and a 32-bit cross-compiler (i686-pc-linux-gnu-gcc).
	multilibDefaults := map[string]string{
		"DEFAULT_ABI":   "amd64",
		"MULTILIB_ABIS": "amd64",
		"ABI":           "amd64",
		"ABI_X86":       "64",
		"CHOST":         "x86_64-pc-linux-gnu",
		"CBUILD":        "x86_64-pc-linux-gnu",
		"CHOST_amd64":   "x86_64-pc-linux-gnu",
		"LIBDIR_amd64":  "lib64",
	}
	for k, v := range multilibDefaults {
		if _, exists := executor.Env.ExtraVars[k]; !exists {
			executor.Env.ExtraVars[k] = v
		}
	}

	// Pass all make.conf variables to environment (profile variables, etc.)
	if cfg != nil {
		for _, varName := range []string{
			"ARCH", "KERNEL", "USERLAND",
			"ACCEPT_KEYWORDS", "ACCEPT_LICENSE",
			"USE_EXPAND", "USE_EXPAND_HIDDEN",
		} {
			if v := cfg.GetVariable(varName); v != "" {
				if _, exists := executor.Env.ExtraVars[varName]; !exists {
					executor.Env.ExtraVars[varName] = v
				}
			}
		}
	}

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

// loadBestAcceptableVersion loads the best non-masked, keyword-accepted version of a package.
// If the highest version is a live ebuild (9999) or has no acceptable KEYWORDS,
// it falls back to GetAllVersions and picks the best acceptable one.
func (a *App) loadBestAcceptableVersion(r repo.Repository, name string, acceptKeywords []string) (*pkg.Package, error) {
	p, err := r.LoadPackage(name)
	if err != nil {
		return nil, err
	}

	// Check if highest version is acceptable
	if p.IsKeywordAccepted(acceptKeywords) {
		return p, nil
	}

	// Highest version is masked/unkeyworded — try all versions
	versions, err := r.GetAllVersions(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get versions for %s: %w", name, err)
	}

	// Filter to acceptable versions
	var acceptable []*pkg.Package
	for _, v := range versions {
		if v.IsKeywordAccepted(acceptKeywords) {
			acceptable = append(acceptable, v)
		}
	}

	if len(acceptable) == 0 {
		// No acceptable versions; fall back to highest (user may have reasons)
		logging.Warn("No keyword-accepted version found for %s, using %s", name, p.Version)
		return p, nil
	}

	// Sort by version (highest first)
	sort.Slice(acceptable, func(i, j int) bool {
		return pkg.CompareVersions(acceptable[i].Version, acceptable[j].Version) > 0
	})

	if acceptable[0].Version != p.Version {
		logging.Debug("Package %s-%s is unkeyworded, using %s instead",
			name, p.Version, acceptable[0].Version)
	}

	return acceptable[0], nil
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
