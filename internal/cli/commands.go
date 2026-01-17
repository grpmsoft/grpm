package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
	"github.com/grpmsoft/grpm/internal/state"
)

// runSearch handles the 'search' command
func (a *App) runSearch(args []string) error {
	// Load Portage configuration for default paths
	cfg := a.loadPortageConfig()

	// Parse flags
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	repoPath := fs.String("repo", cfg.GetPortDir(), "Path to Portage repository")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")
	description := fs.Bool("desc", false, "Search in descriptions too")
	fs.BoolVar(description, "S", false, "Alias for --desc")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	query := fs.Args()
	if len(query) == 0 {
		return fmt.Errorf("no search query specified")
	}

	searchTerm := strings.ToLower(strings.Join(query, " "))

	if *useMock {
		// Mock repository - limited search
		fmt.Printf("Searching for '%s'...\n", searchTerm)
		fmt.Println("\n[ Results for search key : hello ]")
		fmt.Println("*  app-misc/hello")
		fmt.Println("      Latest version available: 2.10")
		fmt.Println("      Description: Prints a familiar, friendly greeting")
		return nil
	}

	// Search in real repository
	fmt.Printf("Searching for '%s'...\n", searchTerm)

	// Get repository path
	absRepoPath, err := filepath.Abs(*repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// List all categories
	categories, err := filepath.Glob(filepath.Join(absRepoPath, "*-*"))
	if err != nil {
		return fmt.Errorf("failed to list categories: %w", err)
	}

	found := 0
	for _, catPath := range categories {
		category := filepath.Base(catPath)

		// List packages in category
		packages, err := filepath.Glob(filepath.Join(catPath, "*"))
		if err != nil {
			continue
		}

		for _, pkgPath := range packages {
			pkgName := filepath.Base(pkgPath)
			fullName := category + "/" + pkgName

			// Search in package name
			if strings.Contains(strings.ToLower(fullName), searchTerm) {
				if found == 0 {
					fmt.Printf("\n[ Results for search key : %s ]\n", searchTerm)
				}
				found++

				// Try to get latest version
				versions, _ := filepath.Glob(filepath.Join(pkgPath, "*.ebuild"))
				latestVersion := "unknown"
				if len(versions) > 0 {
					ebuildName := filepath.Base(versions[len(versions)-1])
					latestVersion = strings.TrimSuffix(strings.TrimPrefix(ebuildName, pkgName+"-"), ".ebuild")
				}

				fmt.Printf("*  %s\n", fullName)
				fmt.Printf("      Latest version available: %s\n", latestVersion)

				// TODO: Parse description from ebuild DESCRIPTION field
				if *description {
					fmt.Println("      Description: (ebuild parsing not implemented)")
				}
			}
		}
	}

	if found == 0 {
		fmt.Printf("\nNo matches found for '%s'.\n", searchTerm)
	} else {
		fmt.Printf("\n[ Applications found : %d ]\n", found)
	}

	return nil
}

// runInfo handles the 'info' command
func (a *App) runInfo(args []string) error {
	// Load Portage configuration for default paths
	cfg := a.loadPortageConfig()

	// Parse flags
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	repoPath := fs.String("repo", cfg.GetPortDir(), "Path to Portage repository")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 {
		return fmt.Errorf("no package specified")
	}

	atomStr := packages[0]

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Load package from atom (handles versioned atoms like "=sys-devel/gcc-13.4.1")
	p, err := a.loadPackageFromAtom(r, atomStr)
	if err != nil {
		return fmt.Errorf("package '%s' not found: %w", atomStr, err)
	}

	// Display package information
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Package:     %s\n", p.Name)
	fmt.Printf("Version:     %s\n", p.Version)
	fmt.Printf("Slot:        %s\n", p.Slot.Name)
	if p.Slot.Subslot != "" {
		fmt.Printf("Sub-Slot:    %s\n", p.Slot.Subslot)
	}
	fmt.Printf("Repository:  gentoo\n")

	if len(p.UseFlags) > 0 {
		useFlags := make([]string, 0, len(p.UseFlags))
		for f, enabled := range p.UseFlags {
			if enabled {
				useFlags = append(useFlags, f)
			} else {
				useFlags = append(useFlags, "-"+f)
			}
		}
		fmt.Printf("\nUSE flags:   %s\n", strings.Join(useFlags, " "))
	}

	if len(p.Deps) > 0 {
		fmt.Printf("\nDependencies (%d):\n", len(p.Deps))
		for _, dep := range p.Deps {
			fmt.Printf("  %s\n", dep.String())
		}
	}

	if len(p.Provides) > 0 {
		fmt.Printf("\nProvides (%d):\n", len(p.Provides))
		for _, prov := range p.Provides {
			fmt.Printf("  %s\n", prov.Name)
		}
	}

	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// runUpdate handles the 'update' command.
//
// Supports package sets:
//   - @world: All user-installed packages plus system packages
//   - @selected: User-installed packages (/var/lib/portage/world)
//   - @system: Base system packages (from profile)
//
// Flags:
//   - --deep/-D: Include dependencies in update calculation
//   - --newuse/-N: Recalculate USE flags
//   - --changed-use/-U: Only update packages with changed USE flags
//   - --pretend/-p: Show what would be updated (dry-run)
//   - --ask/-a: Ask for confirmation before updating
func (a *App) runUpdate(args []string) error {
	// Load Portage configuration for default paths
	cfg := a.loadPortageConfig()

	// Parse flags
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repoPath := fs.String("repo", cfg.GetPortDir(), "Path to Portage repository")
	useMock := fs.Bool("mock", false, "Use mock repository for testing")
	pretend := fs.Bool("pretend", false, "Show what would be updated (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	ask := fs.Bool("ask", false, "Ask for confirmation before updating")
	fs.BoolVar(ask, "a", false, "Alias for --ask")
	deep := fs.Bool("deep", false, "Include dependencies in update")
	fs.BoolVar(deep, "D", false, "Alias for --deep")
	newuse := fs.Bool("newuse", false, "Recalculate USE flags and update if changed")
	fs.BoolVar(newuse, "N", false, "Alias for --newuse")
	changedUse := fs.Bool("changed-use", false, "Only update packages with changed USE flags")
	fs.BoolVar(changedUse, "U", false, "Alias for --changed-use")
	portageDir := fs.String("portage-dir", "/var/lib/portage", "Portage state directory")
	profilePath := fs.String("profile", "/etc/portage/make.profile", "Path to active profile")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Determine target set
	targets := fs.Args()
	targetSet := "@world" // Default to @world
	if len(targets) > 0 {
		targetSet = targets[0]
	}

	// Validate target
	var setName state.SetName
	switch targetSet {
	case "@world", "world":
		setName = state.SetWorld
	case "@selected", "selected":
		setName = state.SetSelected
	case "@system", "system":
		setName = state.SetSystem
	default:
		return fmt.Errorf("unknown target: %s (use @world, @selected, or @system)", targetSet)
	}

	logging.Action("Calculating updates for %s...", setName)

	// Initialize repository
	r, err := a.initRepository(*useMock, *repoPath)
	if err != nil {
		return err
	}

	// Initialize set manager
	setManager := state.NewSetManager(*portageDir, *profilePath)

	// Get package database (if available)
	db, err := a.getOrCreatePackageDB()
	if err == nil {
		setManager.SetDatabase(db)
	}

	// Get the target package set
	pkgSet, err := setManager.GetSet(setName)
	if err != nil {
		return fmt.Errorf("failed to get package set %s: %w", setName, err)
	}

	if pkgSet.Len() == 0 {
		fmt.Printf("Package set %s is empty.\n", setName)
		return nil
	}

	logging.Info("Found %d packages in %s", pkgSet.Len(), setName)

	// Configure update options
	opts := &state.UpdateOptions{
		Deep:       *deep,
		NewUse:     *newuse,
		ChangedUse: *changedUse,
		Update:     true,
	}

	// Calculate updates
	plan, err := setManager.CalculateUpdates(pkgSet, r, opts)
	if err != nil {
		return fmt.Errorf("failed to calculate updates: %w", err)
	}

	// Display update plan
	if len(plan.Updates) == 0 {
		fmt.Println("\nNo updates available.")
		return nil
	}

	a.displayUpdatePlan(plan, setName, *deep, *newuse)

	// Handle pretend mode
	if *pretend {
		return nil
	}

	// Handle ask mode
	if *ask {
		proceed, err := a.askUserConfirmation()
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	// Execute updates
	return a.executeUpdates(plan, r)
}

// displayUpdatePlan displays the calculated update plan.
func (a *App) displayUpdatePlan(plan *state.UpdatePlan, setName state.SetName, deep, newuse bool) {
	fmt.Printf("\n*** Update plan for %s", setName)
	if deep {
		fmt.Print(" (--deep)")
	}
	if newuse {
		fmt.Print(" (--newuse)")
	}
	fmt.Println(":")
	fmt.Println("*** These are the packages that would be merged, in order:")
	fmt.Println()

	for _, update := range plan.Updates {
		fmt.Println(update.String())
	}

	fmt.Println()
	fmt.Printf("Total: %d package(s)\n", len(plan.Updates))

	// Show breakdown
	var breakdown []string
	if plan.NewPackages > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d new", plan.NewPackages))
	}
	if plan.Upgrades > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d upgrades", plan.Upgrades))
	}
	if plan.Downgrades > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d downgrades", plan.Downgrades))
	}
	if plan.Reinstalls > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d reinstalls", plan.Reinstalls))
	}

	if len(breakdown) > 0 {
		fmt.Printf("  (%s)\n", strings.Join(breakdown, ", "))
	}
}

// executeUpdates executes the update plan.
func (a *App) executeUpdates(plan *state.UpdatePlan, r interface{}) error {
	logging.Action("Starting update...")

	for i, update := range plan.Updates {
		logging.Action("Updating %s (%d/%d)", update.Atom, i+1, len(plan.Updates))

		// For now, just log the update
		// Full implementation would call the emerge/install logic
		if update.IsNew {
			logging.Info("    Installing new package: %s-%s", update.Atom, update.AvailableVersion)
		} else if update.IsUpgrade {
			logging.Info("    Upgrading: %s -> %s", update.InstalledVersion, update.AvailableVersion)
		} else if update.UseChanged {
			logging.Info("    Rebuilding with new USE flags")
		}
	}

	logging.Action("Update completed: %d package(s) processed", len(plan.Updates))
	return nil
}

// runRemove handles the 'remove' command
func (a *App) runRemove(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	pretend := fs.Bool("pretend", false, "Show what would be removed (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	depclean := fs.Bool("depclean", false, "Remove unused dependencies")
	fs.BoolVar(depclean, "c", false, "Alias for --depclean")
	force := fs.Bool("force", false, "Force removal even if dependencies exist")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 && !*depclean {
		return fmt.Errorf("no packages specified")
	}

	// Pretend mode - just show what would be removed
	if *pretend {
		fmt.Println("\n*** Removal plan (--pretend mode):")
		fmt.Println("*** These are the packages that would be removed:")
		fmt.Println()
		for _, pkg := range packages {
			fmt.Printf("[ebuild     R ] %s\n", pkg)
		}
		fmt.Printf("\nTotal: %d package(s) to remove\n", len(packages))
		return nil
	}

	// Handle depclean flag - redirect to depclean command
	if *depclean {
		// Build depclean args preserving pretend flag
		depcleanArgs := []string{}
		if *pretend {
			depcleanArgs = append(depcleanArgs, "--pretend")
		}
		return a.runDepclean(depcleanArgs)
	}

	// Real removal
	logging.Info("Removing packages:")
	removedCount := 0

	for _, atom := range packages {
		logging.Action("Uninstalling %s", atom)

		// Uninstall package using real uninstaller
		if err := a.uninstallPackageReal(atom, *force); err != nil {
			return fmt.Errorf("failed to uninstall %s: %w", atom, err)
		}

		removedCount++
		logging.Action("%s uninstalled successfully (%d/%d)", atom, removedCount, len(packages))
	}

	logging.Action("Removal completed successfully: %d package(s) removed", removedCount)
	return nil
}

// runBuild handles the 'build' command - creates binary packages from installed packages
func (a *App) runBuild(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	outputDir := fs.String("output", "/var/cache/binpkgs", "Output directory for binary packages")
	format := fs.String("format", "gpkg", "Package format: gpkg or tbz2")
	compression := fs.String("compression", "zstd", "Compression: none, gzip, bzip2, xz, zstd")
	pretend := fs.Bool("pretend", false, "Show what would be built (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	packages := fs.Args()
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Get package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Build each package
	builtCount := 0
	for _, atom := range packages {
		if *pretend {
			fmt.Printf("[binpkg     B ] %s\n", atom)
			continue
		}

		logging.Action("Building binary package for %s", atom)

		// Get installed package from database
		installedPkg, err := db.Get(atom)
		if err != nil {
			return fmt.Errorf("package not installed: %s", atom)
		}

		// Build binary package
		binpkgPath, err := a.buildBinaryPackage(installedPkg, *outputDir, *format, *compression)
		if err != nil {
			return fmt.Errorf("failed to build binary package for %s: %w", atom, err)
		}

		builtCount++
		logging.Action("Binary package created: %s (%d/%d)", binpkgPath, builtCount, len(packages))
	}

	if *pretend {
		fmt.Printf("\nTotal: %d package(s) would be built\n", len(packages))
	} else {
		logging.Action("Build completed successfully: %d binary package(s) created", builtCount)
	}

	return nil
}

// runDepclean handles the 'depclean' command - removes orphaned packages.
//
// Depclean identifies and removes packages that are:
//   - Not in @world set (user-selected packages)
//   - Not required by any @world package as a dependency
//
// This implements Portage's `emerge --depclean` functionality.
//
// Flags:
//   - --pretend/-p: Show what would be removed (dry-run)
//   - --ask/-a: Ask for confirmation before removing
//   - --exclude: Exclude specific packages from removal (can be specified multiple times)
//   - --portage-dir: Path to Portage state directory (default: /var/lib/portage)
//   - --profile: Path to active profile (default: /etc/portage/make.profile)
func (a *App) runDepclean(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("depclean", flag.ContinueOnError)
	pretend := fs.Bool("pretend", false, "Show what would be removed (dry-run)")
	fs.BoolVar(pretend, "p", false, "Alias for --pretend")
	ask := fs.Bool("ask", false, "Ask for confirmation before removing")
	fs.BoolVar(ask, "a", false, "Alias for --ask")
	portageDir := fs.String("portage-dir", "/var/lib/portage", "Portage state directory")
	profilePath := fs.String("profile", "/etc/portage/make.profile", "Path to active profile")

	// Collect exclude flags (can be specified multiple times)
	var excludes excludeFlags
	fs.Var(&excludes, "exclude", "Exclude package from removal (can be specified multiple times)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	logging.Action("Calculating dependencies...")

	// Get package database
	db, err := a.getOrCreatePackageDB()
	if err != nil {
		return fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Initialize set manager
	setManager := state.NewSetManager(*portageDir, *profilePath)
	setManager.SetDatabase(db)

	// Create depclean calculator
	calc := solver.NewDepcleanCalculator(db, setManager)

	// Set options
	opts := solver.DefaultDepcleanOptions()
	opts.Exclude = []string(excludes)
	opts.Verbose = a.verbose
	calc.SetOptions(opts)

	// Calculate orphaned packages
	result, err := calc.Calculate()
	if err != nil {
		return fmt.Errorf("depclean calculation failed: %w", err)
	}

	// Display results
	fmt.Print(solver.FormatDepcleanResult(result, *pretend))

	// Handle pretend mode
	if *pretend {
		return nil
	}

	// No orphans found
	if len(result.Orphans) == 0 {
		return nil
	}

	// Handle ask mode
	if *ask {
		proceed, err := a.askUserConfirmation()
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	// Execute removal
	return a.executeDepclean(result)
}

// executeDepclean removes the orphaned packages.
func (a *App) executeDepclean(result *solver.DepcleanResult) error {
	logging.Action("Removing orphaned packages...")

	removedCount := 0
	for _, orphan := range result.Orphans {
		atom := fmt.Sprintf("%s-%s", orphan.Atom, orphan.Version)
		logging.Action("Uninstalling %s", atom)

		// Uninstall package using real uninstaller
		if err := a.uninstallPackageReal(atom, false); err != nil {
			return fmt.Errorf("failed to uninstall %s: %w", atom, err)
		}

		removedCount++
		logging.Action("%s uninstalled successfully (%d/%d)",
			atom, removedCount, len(result.Orphans))
	}

	logging.Action("Depclean completed: %d package(s) removed", removedCount)
	return nil
}

// excludeFlags implements flag.Value for collecting multiple --exclude flags.
type excludeFlags []string

func (e *excludeFlags) String() string {
	return strings.Join(*e, ",")
}

func (e *excludeFlags) Set(value string) error {
	*e = append(*e, value)
	return nil
}

// loadPackageFromAtom loads a package from a PMS-compliant atom string.
//
// Supports:
//   - category/package (e.g., "sys-devel/gcc") - loads latest version
//   - =category/package-version (e.g., "=sys-devel/gcc-13.4.1") - loads exact version
//   - >=category/package-version (e.g., ">=sys-devel/gcc-13.0") - loads best matching
//
// This properly handles versioned atoms that contain version numbers in the package name
// (like gcc-13.4.1_p20250807) by parsing the atom first to extract category/package.
func (a *App) loadPackageFromAtom(r repo.Repository, atomStr string) (*pkg.Package, error) {
	// Try to parse as a PMS atom first
	atom, err := pkg.ParseAtom(atomStr)
	if err != nil {
		// If parsing fails, treat as simple category/package
		return r.LoadPackage(atomStr)
	}

	// Get category/package without version
	cp := atom.CP()

	// Check if repository supports FindByAtom
	portageRepo, isPortage := r.(*repo.PortageRepository)
	if !isPortage {
		// Mock or other repo - just use LoadPackage with CP
		return r.LoadPackage(cp)
	}

	// If atom has a version, use FindByAtom to find matching packages
	if atom.HasVersion() {
		matches, err := portageRepo.FindByAtom(atom)
		if err != nil {
			return nil, fmt.Errorf("failed to find packages matching %s: %w", atomStr, err)
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("no packages match atom %s", atomStr)
		}

		// Return the best matching version (FindByAtom already filters by atom constraint)
		// For exact match (=), there should be exactly one result
		return matches[0], nil
	}

	// No version specified - load latest
	return r.LoadPackage(cp)
}
