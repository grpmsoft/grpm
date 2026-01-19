package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/mask"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/sets"
	"github.com/grpmsoft/grpm/internal/solver"
	"github.com/grpmsoft/grpm/internal/state"
	"github.com/grpmsoft/grpm/internal/sync"
)

// App represents the CLI application
type App struct {
	client       *Client
	version      string
	verbose      bool
	verboseLevel int // 0=off, 1=-v, 2=-vv, 3=-vvv
	log          *logging.Logger
}

// AppConfig holds application configuration
type AppConfig struct {
	Version      string
	Verbose      bool
	VerboseLevel int
	SocketPath   string
}

// NewApp creates a new CLI application
func NewApp(config *AppConfig) *App {
	clientConfig := &ClientConfig{
		SocketPath: config.SocketPath,
	}

	if clientConfig.SocketPath == "" {
		clientConfig.SocketPath = DefaultClientConfig().SocketPath
	}

	logger := logging.New()

	// Set logging level based on verbosity
	// Set on both the app logger and the global default logger
	var level logging.Level
	switch config.VerboseLevel {
	case 0:
		level = logging.LevelNormal
	case 1:
		level = logging.LevelVerbose
	default:
		level = logging.LevelDebug
	}
	logger.SetLevel(level)
	logging.SetLevel(level) // Also set global default

	app := &App{
		client:       NewClient(clientConfig),
		version:      config.Version,
		verbose:      config.Verbose,
		verboseLevel: config.VerboseLevel,
		log:          logger,
	}

	return app
}

// Run runs the CLI application
func (a *App) Run(args []string) error {
	// Check daemon availability
	if a.client.IsDaemonAvailable() {
		a.log.Verbose("Connected to daemon at %s", a.client.GetSocketPath())
	} else {
		a.log.Verbose("Daemon not available, running in standalone mode")
	}

	// Parse command
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	command := args[0]
	cmdArgs := args[1:]

	// Route to command handler
	switch command {
	case "help", "--help", "-h":
		a.PrintUsage()
		return nil
	case "resolve":
		return a.runResolve(cmdArgs)
	case "install":
		return a.runInstall(cmdArgs)
	case "sync":
		return a.runSync(cmdArgs)
	case "search":
		return a.runSearch(cmdArgs)
	case "info":
		return a.runInfo(cmdArgs)
	case "update":
		return a.runUpdate(cmdArgs)
	case "remove", "uninstall", "unmerge":
		return a.runRemove(cmdArgs)
	case "build":
		return a.runBuild(cmdArgs)
	case "emerge":
		return a.runEmerge(cmdArgs)
	case "depclean":
		return a.runDepclean(cmdArgs)
	case "fetch":
		return a.runFetch(cmdArgs)
	case "analyze":
		return a.runAnalyze(cmdArgs)
	case "tools":
		return a.runTools(cmdArgs)
	default:
		return fmt.Errorf("unknown command: %s\nRun 'grpm help' for usage", command)
	}
}

// Close cleans up application resources
func (a *App) Close() error {
	return a.client.Close()
}

// PrintVersion prints version information
func (a *App) PrintVersion() {
	fmt.Printf("grpm version %s\n", a.version)
	if a.client.IsDaemonAvailable() {
		fmt.Println("Daemon: connected")
	} else {
		fmt.Println("Daemon: not available")
	}
}

// PrintUsage prints usage information using the professional help formatter.
func (a *App) PrintUsage() {
	registry := NewCommandRegistry()
	fmt.Print(FormatMainHelp(a.version, registry.All()))
}

// IsDaemonMode returns true if running via daemon
func (a *App) IsDaemonMode() bool {
	return a.client.IsDaemonAvailable()
}

// ExecuteViaDaemon executes command via daemon gRPC API.
// This is a placeholder for future daemon-mode command routing.
// Currently all commands are handled by specific run* methods.
func (a *App) ExecuteViaDaemon(command string, args []string) error {
	if !a.client.IsDaemonAvailable() {
		return fmt.Errorf("daemon not available")
	}

	a.log.Verbose("Executing via daemon: %s %v", command, args)

	// TODO: Implement gRPC call when daemon command routing is added
	return fmt.Errorf("daemon command routing not implemented")
}

// ExecuteStandalone executes command in standalone mode (without daemon).
// This is a placeholder for future generic command routing.
// Currently all commands are handled by specific run* methods.
func (a *App) ExecuteStandalone(command string, args []string) error {
	a.log.Verbose("Executing in standalone mode: %s %v", command, args)

	// TODO: Implement when generic command routing is needed
	return fmt.Errorf("generic command routing not implemented")
}

// GetClient returns the underlying client
func (a *App) GetClient() *Client {
	return a.client
}

// SetVerbose enables/disables verbose output
func (a *App) SetVerbose(verbose bool) {
	a.verbose = verbose
	if verbose {
		a.log.SetLevel(logging.LevelVerbose)
	}
}

// runResolve handles the 'resolve' command
func (a *App) runResolve(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	repoPath := fs.String("repo", "/var/db/repos/gentoo", "Path to Portage repository")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")
	pretend := fs.Bool("pretend", false, "Show what would be done (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	dryRun := fs.Bool("dry-run", false, "Alias for --pretend")
	autounmask := fs.Bool("autounmask", false, "Show USE/keyword changes to resolve conflicts")
	autounmaskWrite := fs.Bool("autounmask-write", false, "Write autounmask changes to /etc/portage")

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("resolve")) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Normalize flags: --dry-run → --pretend
	if *dryRun {
		*pretend = true
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

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Resolve dependencies (with mask filtering for real repositories)
	pkgResolver := a.createResolverWithMasks(r)
	solution, err := pkgResolver.Resolve(packages)
	if err != nil {
		// Wrap with user-friendly error
		if len(packages) == 1 {
			// Try to find similar packages for single package queries
			similar := repo.FindSimilarPackages(packages[0], *repoPath, 3)
			return WrapPackageNotFound(packages[0], similar, err)
		}
		return WrapResolutionError(packages[0], err)
	}

	if len(solution) == 0 {
		a.log.Info("No packages found in solution")
		return nil
	}

	// Check for slot collisions if autounmask is enabled
	if *autounmask || *autounmaskWrite {
		hasCollisions, err := a.handleResolveAutounmask(solution, *autounmaskWrite)
		if err != nil {
			return err
		}
		if hasCollisions {
			return nil // Collisions were handled/reported
		}
	}

	// Display solution
	a.displayResolveSolution(solution, *pretend)
	return nil
}

// handleResolveAutounmask handles autounmask logic for resolve command.
// Returns true if collisions were found and handled.
func (a *App) handleResolveAutounmask(solution map[string]*pkg.Package, writeChanges bool) (bool, error) {
	graph := solver.NewDependencyGraph()
	for _, p := range solution {
		graph.AddPackage(p, false)
	}

	detector := solver.NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		return false, nil
	}

	// Show collision report
	fmt.Print(solver.GenerateConflictReport(collisions))

	// Generate and show/write autounmask suggestions
	collisionResolver := solver.NewCollisionResolver(detector, nil)
	config := solver.DefaultAutounmaskConfig()
	config.Write = writeChanges

	ia := solver.NewInteractiveAutounmask(collisionResolver, config)
	if err := ia.ProcessCollisions(collisions); err != nil {
		return true, fmt.Errorf("failed to process collisions: %w", err)
	}

	if ia.GetWriter().HasEntries() {
		if writeChanges {
			if err := ia.ApplyAutounmask(); err != nil {
				return true, fmt.Errorf("failed to write autounmask changes: %w", err)
			}
			fmt.Println("\nAutounmask changes written to /etc/portage/")
			fmt.Println("Please re-run the command to apply changes.")
		} else {
			fmt.Print(ia.GetWriter().FormatPreview())
			fmt.Println("\nUse --autounmask-write to apply these changes automatically.")
		}
	}

	return true, nil
}

// displayResolveSolution displays the dependency resolution solution.
func (a *App) displayResolveSolution(solution map[string]*pkg.Package, pretend bool) {
	if pretend {
		fmt.Println("\n*** Dependency resolution (--pretend mode):")
		fmt.Println("*** The following packages would be used:")
	} else {
		fmt.Println("Dependency solution:")
	}

	for name, p := range solution {
		if pretend {
			fmt.Printf("[ebuild  N    ] %s-%s [%s]\n", name, p.Version, p.Slot.Name)
		} else {
			fmt.Printf("- %s-%s [slot:%s]\n", name, p.Version, p.Slot.Name)
		}
	}

	if pretend {
		fmt.Printf("\nTotal: %d package(s)\n", len(solution))
	}
}

// runInstall handles the 'install' command
func (a *App) runInstall(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	repoPath := fs.String("repo", "/var/db/repos/gentoo", "Path to Portage repository")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")
	binpkgDir := fs.String("binpkg-dir", "/var/cache/binpkgs", "Directory with binary packages (.gpkg.tar)")
	useBinpkg := fs.Bool("binpkg", false, "Prefer binary packages when available")
	snapshotDir := fs.String("snapshot-dir", "/.snapshots", "Snapshot directory")
	fsType := fs.String("fs-type", "btrfs", "Filesystem type (btrfs or zfs)")
	noSnapshot := fs.Bool("no-snapshot", false, "Skip snapshot creation (for testing)")
	pretend := fs.Bool("pretend", false, "Show what would be installed (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	dryRun := fs.Bool("dry-run", false, "Alias for --pretend")
	ask := fs.Bool("ask", false, "Show plan and ask for confirmation")
	fs.BoolVar(ask, "a", false, "Alias for --ask")
	autounmask := fs.Bool("autounmask", false, "Show USE/keyword changes to resolve conflicts")
	autounmaskWrite := fs.Bool("autounmask-write", false, "Write autounmask changes to /etc/portage")

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("install")) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Normalize flags: --dry-run → --pretend
	if *dryRun {
		*pretend = true
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

	// Create initial snapshot (unless pretend, ask, or no-snapshot mode)
	snapshotID, err := a.createInitialSnapshot(*pretend, *ask, *noSnapshot, *snapshotDir, *fsType)
	if err != nil {
		return err
	}

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Resolve dependencies
	solution, err := a.resolvePackageDependencies(r, packages)
	if err != nil {
		return err
	}

	// Check for slot collisions if autounmask is enabled
	if *autounmask || *autounmaskWrite {
		collisionErr := a.handleSlotCollisions(solution, *autounmaskWrite)
		if collisionErr != nil {
			return collisionErr
		}
	}

	// Display plan and ask for confirmation if needed
	shouldInstall, err := a.displayPlanAndAsk(solution, *pretend, *ask)
	if err != nil {
		return err
	}
	if !shouldInstall {
		return nil
	}

	// Create snapshot after ask confirmation
	if *ask && snapshotID == "" {
		_, err = a.createSnapshot(*snapshotDir, *fsType)
		if err != nil {
			return err
		}
	}

	// Execute installation
	return a.executeInstallation(solution, *useBinpkg, *binpkgDir)
}

// createInitialSnapshot creates a snapshot before installation (unless in pretend/ask/no-snapshot mode)
func (a *App) createInitialSnapshot(pretend, ask, noSnapshot bool, snapshotDir, fsType string) (string, error) {
	if pretend || ask || noSnapshot {
		return "", nil
	}

	sm := state.NewSnapshotManager(snapshotDir, fsType)
	snapshotID, err := sm.CreateSnapshot("/")
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}
	a.log.Info("Created system snapshot: %s", snapshotID)
	return snapshotID, nil
}

// createSnapshot creates a snapshot (used after ask confirmation)
func (a *App) createSnapshot(snapshotDir, fsType string) (string, error) {
	sm := state.NewSnapshotManager(snapshotDir, fsType)
	snapshotID, err := sm.CreateSnapshot("/")
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}
	a.log.Info("Created system snapshot: %s", snapshotID)
	return snapshotID, nil
}

// initRepository initializes the package repository (mock or real)
func (a *App) initRepository(useMock bool, repoPath string) (repo.Repository, error) {
	if useMock {
		a.log.Verbose("Using mock repository")
		return repo.NewMockRepository(), nil
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	a.log.Verbose("Using repository: %s", absRepoPath)

	r, err := repo.NewPortageRepository(absRepoPath)
	if err != nil {
		return nil, fmt.Errorf("repository error: %w", err)
	}
	return r, nil
}

// resolvePackageDependencies resolves package dependencies using SAT solver.
// For real Portage repositories, masked packages are automatically filtered.
func (a *App) resolvePackageDependencies(r repo.Repository, packages []string) (map[string]*pkg.Package, error) {
	// Create resolver with mask support if using real Portage repository
	resolver := a.createResolverWithMasks(r)

	solution, err := resolver.Resolve(packages)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	if len(solution) == 0 {
		a.log.Info("No packages to install")
		return nil, nil
	}

	return solution, nil
}

// createResolverWithMasks creates a resolver with package.mask and KEYWORDS filtering.
// For mock repositories, returns a resolver without filtering.
func (a *App) createResolverWithMasks(r repo.Repository) *solver.PortageResolver {
	// Check if this is a real Portage repository
	portageRepo, isPortage := r.(*repo.PortageRepository)
	if !isPortage {
		// Mock repository - no mask filtering needed
		a.log.Verbose("Using resolver without masks (mock repository)")
		return solver.NewResolver(r)
	}

	// Load Portage configuration
	cfg, err := config.LoadConfig("/etc/portage")
	if err != nil {
		a.log.Verbose("Could not load Portage config, using resolver without masks: %v", err)
		return solver.NewResolver(r)
	}

	// Determine profile path
	profilePath := a.detectProfilePath()

	// Create mask manager
	maskMgr, err := mask.NewMaskManager(cfg, portageRepo.Path, profilePath)
	if err != nil {
		a.log.Verbose("Could not initialize mask manager: %v", err)
		// Continue without mask manager - keyword filtering may still work
		maskMgr = nil
	}

	// Get ACCEPT_KEYWORDS from config, with sensible defaults
	acceptKeywords := a.getAcceptKeywords(cfg)

	a.log.Verbose("Package filtering enabled (mask=%v, keywords=%v)",
		maskMgr != nil, acceptKeywords)

	return solver.NewResolverWithFilters(r, maskMgr, acceptKeywords)
}

// getAcceptKeywords returns ACCEPT_KEYWORDS from config or detects defaults.
// Default is the current architecture (e.g., "amd64" on x86_64 Linux).
func (a *App) getAcceptKeywords(cfg *config.Config) []string {
	// First check if ACCEPT_KEYWORDS is set in make.conf
	if len(cfg.MakeConf.ACCEPT_KEYWORDS) > 0 {
		a.log.Verbose("Using ACCEPT_KEYWORDS from make.conf: %v", cfg.MakeConf.ACCEPT_KEYWORDS)
		return cfg.MakeConf.ACCEPT_KEYWORDS
	}

	// Default to stable architecture (detect from system)
	arch := a.detectArchitecture()
	if arch != "" {
		a.log.Verbose("Using default ACCEPT_KEYWORDS: [%s]", arch)
		return []string{arch}
	}

	// Fallback to amd64 (most common)
	a.log.Verbose("Using fallback ACCEPT_KEYWORDS: [amd64]")
	return []string{"amd64"}
}

// detectArchitecture returns the Gentoo architecture keyword for the current system.
// Maps Go GOARCH to Gentoo KEYWORDS (e.g., amd64, x86, arm64, arm).
func (a *App) detectArchitecture() string {
	// Map Go GOARCH to Gentoo KEYWORDS
	archMap := map[string]string{
		"amd64":   "amd64",
		"386":     "x86",
		"arm64":   "arm64",
		"arm":     "arm",
		"ppc64":   "ppc64",
		"ppc64le": "ppc64",
		"riscv64": "riscv",
		"s390x":   "s390",
		"mips64":  "mips",
		"loong64": "loong",
	}

	// Use runtime.GOARCH to get the current architecture
	if arch, ok := archMap[runtime.GOARCH]; ok {
		return arch
	}

	// Fallback for unknown architectures
	return ""
}

// detectProfilePath returns the active Portage profile path.
// Returns empty string if profile cannot be determined.
func (a *App) detectProfilePath() string {
	// Standard profile symlink location
	profileLink := "/etc/portage/make.profile"

	// Resolve symlink to get actual profile path
	target, err := filepath.EvalSymlinks(profileLink)
	if err != nil {
		a.log.Verbose("Could not resolve profile symlink: %v", err)
		return ""
	}

	return target
}

// displayPlanAndAsk displays installation plan and asks for confirmation if needed
func (a *App) displayPlanAndAsk(solution map[string]*pkg.Package, pretend, ask bool) (bool, error) {
	if !pretend && !ask {
		return true, nil // Normal install mode - proceed
	}

	// Display installation plan
	fmt.Println("\n*** Installation plan:")
	fmt.Println("*** These are the packages that would be merged, in order:")
	fmt.Println()
	for name, pkg := range solution {
		fmt.Printf("[ebuild  N    ] %s-%s to / USE=\"...\"\n", name, pkg.Version)
	}
	fmt.Printf("\nTotal: %d package(s)\n", len(solution))

	if ask {
		return a.askUserConfirmation()
	}

	// Pretend mode - don't install
	return false, nil
}

// askUserConfirmation prompts user for installation confirmation
func (a *App) askUserConfirmation() (bool, error) {
	fmt.Print("\nWould you like to merge these packages? [Yes/No] ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	if response == "Yes" || response == "yes" || response == "y" || response == "Y" {
		fmt.Println()
		return true, nil
	}

	fmt.Println("Installation canceled.")
	return false, nil
}

// executeInstallation performs the actual package installation
func (a *App) executeInstallation(solution map[string]*pkg.Package, useBinpkg bool, binpkgDir string) error {
	if solution == nil {
		return nil
	}

	a.log.Action("Installing packages")
	installedCount := 0

	for name, p := range solution {
		a.log.Installing(installedCount+1, len(solution), fmt.Sprintf("%s-%s (slot: %s)", name, p.Version, p.Slot))

		// Search for binary package if requested
		binpkgPath := ""
		if useBinpkg {
			binpkgPath = a.findBinaryPackage(p, binpkgDir)
			if binpkgPath != "" {
				a.log.Verbose("Found binary package: %s", binpkgPath)
			}
		}

		// Install package using real installer
		if err := a.installPackageReal(p, binpkgPath); err != nil {
			return fmt.Errorf("failed to install %s: %w", name, err)
		}

		installedCount++
		a.log.Success("%s-%s installed successfully (%d/%d)", name, p.Version, installedCount, len(solution))
	}

	a.log.Success("Installation completed: %d package(s) installed", installedCount)
	return nil
}

// handleSlotCollisions checks for and handles slot collisions in the solution.
// Returns an error if collisions are found and cannot be resolved.
func (a *App) handleSlotCollisions(solution map[string]*pkg.Package, writeChanges bool) error {
	if solution == nil {
		return nil
	}

	graph := solver.NewDependencyGraph()
	for _, p := range solution {
		graph.AddPackage(p, false)
	}

	detector := solver.NewSlotCollisionDetector(graph, nil)
	collisions := detector.DetectCollisions()

	if len(collisions) == 0 {
		return nil // No collisions, proceed with installation
	}

	// Show collision report
	fmt.Print(solver.GenerateConflictReport(collisions))

	// Generate and show/write autounmask suggestions
	collisionResolver := solver.NewCollisionResolver(detector, nil)
	config := solver.DefaultAutounmaskConfig()
	config.Write = writeChanges

	ia := solver.NewInteractiveAutounmask(collisionResolver, config)
	if err := ia.ProcessCollisions(collisions); err != nil {
		return fmt.Errorf("failed to process collisions: %w", err)
	}

	if ia.GetWriter().HasEntries() {
		if writeChanges {
			if err := ia.ApplyAutounmask(); err != nil {
				return fmt.Errorf("failed to write autounmask changes: %w", err)
			}
			fmt.Println("\nAutounmask changes written to /etc/portage/")
			fmt.Println("Please re-run the command to apply changes.")
		} else {
			fmt.Print(ia.GetWriter().FormatPreview())
			fmt.Println("\nUse --autounmask-write to apply these changes automatically.")
		}
		return fmt.Errorf("slot collisions detected, resolve conflicts before continuing")
	}

	// Collisions exist but no autounmask suggestions available (version conflicts)
	return fmt.Errorf("unresolvable slot collisions detected")
}

// runSync handles the 'sync' command using pluggable sync methods
func (a *App) runSync(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	repoPath := fs.String("repo", "/var/db/repos/gentoo", "Repository path to sync")
	sourceURL := fs.String("url", "", "Source repository URL (auto-detected based on method)")
	method := fs.String("method", "auto", "Sync method: rsync, git, or auto")
	skipGPG := fs.Bool("skip-gpg-verify", false, "Skip GPG signature verification (NOT RECOMMENDED)")
	preferGit := fs.Bool("prefer-git", false, "Prefer Git over rsync when using auto method")

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("sync")) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Create context with signal handling for graceful interruption
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create sync configuration
	syncConfig := &sync.SyncConfig{
		Method:    sync.SyncMethod(*method),
		RepoPath:  *repoPath,
		SourceURL: *sourceURL,
		VerifyGPG: !*skipGPG,
		Verbose:   a.verbose,
		PreferGit: *preferGit,
	}

	// Create syncer
	syncer, err := sync.NewSyncer(syncConfig.Method)
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	// Execute sync (syncer handles its own logging)
	result, err := syncer.Sync(ctx, syncConfig)
	if err != nil {
		a.log.Error("Sync failed: %v", err)
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display results
	a.log.Success("Sync completed successfully")
	a.log.Info("Method: %s", result.Method)
	a.log.Info("Duration: %s", result.Duration)
	if syncConfig.VerifyGPG {
		if result.GPGVerified {
			a.log.Success("GPG: Verified")
		} else {
			a.log.Warn("GPG: Not verified")
		}
	}

	return nil
}

// findBinaryPackage searches for a binary package (.gpkg.tar) in the specified directory.
//
// Searches for:
//   - category/packagename-version.gpkg.tar
//   - packagename-version.gpkg.tar
//
// Returns empty string if not found.
func (a *App) findBinaryPackage(p *pkg.Package, binpkgDir string) string {
	pkgBaseName := filepath.Base(p.Name)
	category := filepath.Dir(p.Name)

	candidates := []string{
		filepath.Join(binpkgDir, category, fmt.Sprintf("%s-%s.gpkg.tar", pkgBaseName, p.Version)),
		filepath.Join(binpkgDir, fmt.Sprintf("%s-%s.gpkg.tar", pkgBaseName, p.Version)),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	a.log.Verbose("Binary package not found for %s-%s in %s", p.Name, p.Version, binpkgDir)

	return ""
}

// expandPackageArgs expands package set references (@world, @selected, @system)
// to their constituent package atoms.
//
// This provides unified set expansion for all CLI commands. Set references like
// "@world" are expanded to the list of package atoms they represent.
//
// Arguments can be:
//   - Package atoms (e.g., "app-misc/hello", ">=sys-libs/zlib-1.2")
//   - Set references (e.g., "@world", "@selected", "@system")
//
// Example:
//
//	packages := []string{"@world", "app-misc/hello"}
//	expanded, err := a.expandPackageArgs(packages)
//	// Returns: ["sys-apps/portage", "dev-lang/go", ..., "app-misc/hello"]
func (a *App) expandPackageArgs(args []string) ([]string, error) {
	// Check if any argument is a set reference
	hasSet := false
	for _, arg := range args {
		if sets.IsSetReference(arg) {
			hasSet = true
			break
		}
	}

	// Fast path: no sets to expand
	if !hasSet {
		return args, nil
	}

	// Create set expander
	expander := sets.NewExpander("/", nil, nil)

	// Expand sets
	expanded, err := expander.Expand(args)
	if err != nil {
		return nil, fmt.Errorf("set expansion failed: %w", err)
	}

	// Log expansion if verbose
	if a.verbose && len(expanded) != len(args) {
		a.log.Verbose("Expanded %d set reference(s) to %d package(s)", len(args), len(expanded))
	}

	return expanded, nil
}
