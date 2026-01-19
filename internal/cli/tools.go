package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/grpmsoft/grpm/internal/tools"
)

// runTools handles the 'tools' command - external tool detection and management.
//
// This command allows users to check which external tools are available on the
// system and which are missing. It provides actionable suggestions for installing
// missing tools.
//
// Subcommands/Flags:
//   - (default): List all known tools with availability status
//   - --check: Check all tools and show summary
//   - --missing: Show only missing tools
//   - --available: Show only available tools
//   - --category CAT: Filter by category (compilers, build-systems, etc.)
//   - --for-eclass ECLASS: Show tools needed for specific eclass
//
// Usage:
//
//	grpm tools                          # List all tools with status
//	grpm tools --check                  # Check and summarize
//	grpm tools --missing                # Show only missing tools
//	grpm tools --category compilers     # Show only compiler tools
//	grpm tools --for-eclass cmake       # Tools needed for cmake.eclass
func (a *App) runTools(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
	check := fs.Bool("check", false, "Check all tools and show summary")
	missing := fs.Bool("missing", false, "Show only missing tools")
	available := fs.Bool("available", false, "Show only available tools")
	category := fs.String("category", "", "Filter by category")
	fs.StringVar(category, "c", "", "Alias for --category")
	forEclass := fs.String("for-eclass", "", "Show tools needed for specific eclass")
	fs.StringVar(forEclass, "e", "", "Alias for --for-eclass")
	showPaths := fs.Bool("paths", false, "Show PATH directories")

	// Set custom help handler
	fs.Usage = func() { fmt.Print(GetCommandHelp("tools")) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Create registry and detector
	registry := tools.NewDefaultRegistry()
	detector := tools.NewDetector(registry)

	// Handle --paths flag
	if *showPaths {
		return a.showToolPaths()
	}

	// Handle --for-eclass flag
	if *forEclass != "" {
		return a.showToolsForEclass(registry, detector, *forEclass)
	}

	// Handle --check flag
	if *check {
		return a.showToolSummary(detector)
	}

	// Handle category filter
	if *category != "" {
		cat := tools.ToolCategory(*category)
		return a.showToolsByCategory(registry, detector, cat)
	}

	// Handle --missing or --available flags
	if *missing {
		return a.showMissingTools(detector)
	}
	if *available {
		return a.showAvailableTools(detector)
	}

	// Default: show all tools
	return a.showAllTools(registry, detector)
}

// showAllTools displays all registered tools with their availability status.
func (a *App) showAllTools(registry *tools.Registry, detector *tools.Detector) error {
	allTools := registry.All()
	if len(allTools) == 0 {
		fmt.Println("No tools registered.")
		return nil
	}

	// Sort by name
	sort.Slice(allTools, func(i, j int) bool {
		return allTools[i].Name < allTools[j].Name
	})

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOOL\tSTATUS\tPACKAGE\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "----\t------\t-------\t-----------")

	for _, tool := range allTools {
		status := "[ ]"
		if detector.IsAvailable(tool.Name) {
			status = "[x]"
		}

		desc := tool.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tool.Name, status, tool.Package, desc)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	// Show summary
	summary := detector.Summary()
	fmt.Printf("\nTotal: %d tools (%d available, %d missing)\n",
		summary.Total, summary.Available, summary.Missing)

	return nil
}

// showMissingTools displays only the missing tools with installation hints.
func (a *App) showMissingTools(detector *tools.Detector) error {
	missingTools := detector.Missing()

	if len(missingTools) == 0 {
		fmt.Println("All tools are available!")
		return nil
	}

	fmt.Printf("Missing tools (%d):\n\n", len(missingTools))

	// Sort by name
	sort.Slice(missingTools, func(i, j int) bool {
		return missingTools[i].Name < missingTools[j].Name
	})

	for _, tool := range missingTools {
		fmt.Printf("  %s - %s\n", tool.Name, tool.Description)
		fmt.Printf("    Install: grpm install %s\n\n", tool.Package)
	}

	// Show quick install command for all
	if len(missingTools) > 0 {
		packages := make([]string, len(missingTools))
		for i, tool := range missingTools {
			packages[i] = tool.Package
		}
		fmt.Printf("Install all: grpm install %s\n", strings.Join(packages, " "))
	}

	return nil
}

// showAvailableTools displays only the available tools.
func (a *App) showAvailableTools(detector *tools.Detector) error {
	availableTools := detector.Available()

	if len(availableTools) == 0 {
		fmt.Println("No tools available.")
		return nil
	}

	fmt.Printf("Available tools (%d):\n\n", len(availableTools))

	// Sort by name
	sort.Slice(availableTools, func(i, j int) bool {
		return availableTools[i].Name < availableTools[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOOL\tPATH")
	_, _ = fmt.Fprintln(w, "----\t----")

	for _, tool := range availableTools {
		path, _ := detector.FindBinary(tool.Name)
		_, _ = fmt.Fprintf(w, "%s\t%s\n", tool.Name, path)
	}

	return w.Flush()
}

// showToolsByCategory displays tools filtered by category.
func (a *App) showToolsByCategory(registry *tools.Registry, detector *tools.Detector, cat tools.ToolCategory) error {
	categoryTools := registry.ByCategory(cat)

	if len(categoryTools) == 0 {
		// List available categories
		fmt.Printf("Unknown category: %s\n\n", cat)
		fmt.Println("Available categories:")
		for _, c := range tools.AllCategories() {
			count := len(registry.ByCategory(c))
			if count > 0 {
				fmt.Printf("  - %s (%d tools)\n", c, count)
			}
		}
		return nil
	}

	fmt.Printf("Tools in category '%s' (%d):\n\n", cat, len(categoryTools))

	// Sort by name
	sort.Slice(categoryTools, func(i, j int) bool {
		return categoryTools[i].Name < categoryTools[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOOL\tSTATUS\tPACKAGE\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "----\t------\t-------\t-----------")

	availCount := 0
	for _, tool := range categoryTools {
		status := "[ ]"
		if detector.IsAvailable(tool.Name) {
			status = "[x]"
			availCount++
		}

		desc := tool.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tool.Name, status, tool.Package, desc)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("\nSummary: %d/%d available\n", availCount, len(categoryTools))

	return nil
}

// showToolsForEclass displays tools required by a specific eclass.
func (a *App) showToolsForEclass(registry *tools.Registry, detector *tools.Detector, eclass string) error {
	// Normalize eclass name
	eclass = strings.TrimSuffix(eclass, ".eclass")

	// Get tools from registry
	eclassTools := registry.ByEclass(eclass)

	// Also check the static map for eclasses not yet in registry
	if toolNames, ok := tools.EclassToolMap()[eclass]; ok && len(eclassTools) == 0 {
		for _, name := range toolNames {
			if tool := registry.Get(name); tool != nil {
				eclassTools = append(eclassTools, tool)
			}
		}
	}

	if len(eclassTools) == 0 {
		fmt.Printf("No tools registered for eclass: %s\n\n", eclass)
		fmt.Println("Known eclasses with tool requirements:")
		for _, e := range registry.Eclasses() {
			fmt.Printf("  - %s\n", e)
		}
		return nil
	}

	fmt.Printf("Tools required for %s.eclass (%d):\n\n", eclass, len(eclassTools))

	// Check for missing tools
	missing := detector.MissingForEclass(eclass)
	hasMissing := len(missing) > 0

	// Sort by name
	sort.Slice(eclassTools, func(i, j int) bool {
		return eclassTools[i].Name < eclassTools[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOOL\tSTATUS\tPACKAGE")
	_, _ = fmt.Fprintln(w, "----\t------\t-------")

	for _, tool := range eclassTools {
		status := "[x]"
		if !detector.IsAvailable(tool.Name) {
			if tool.Optional {
				status = "[-]"
			} else {
				status = "[ ]"
			}
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", tool.Name, status, tool.Package)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	// Show install hint if missing tools
	if hasMissing {
		fmt.Printf("\nMissing required tools: %d\n", len(missing))
		packages := make([]string, len(missing))
		for i, tool := range missing {
			packages[i] = tool.Package
		}
		fmt.Printf("Install: grpm install %s\n", strings.Join(packages, " "))
	} else {
		fmt.Println("\nAll required tools are available.")
	}

	return nil
}

// showToolSummary displays a summary of tool availability.
func (a *App) showToolSummary(detector *tools.Detector) error {
	summary := detector.Summary()

	fmt.Println("Tool Detection Summary")
	fmt.Println("======================")
	fmt.Printf("\nTotal tools:     %d\n", summary.Total)
	fmt.Printf("Available:       %d (%.1f%%)\n", summary.Available, summary.AvailabilityPercent())
	fmt.Printf("Missing:         %d\n", summary.Missing)

	// Show by category
	fmt.Println("\nBy Category:")
	for _, cat := range tools.AllCategories() {
		cs := summary.ByCategory[cat]
		if cs.Total > 0 {
			fmt.Printf("  %-15s %d/%d available\n", cat+":", cs.Available, cs.Total)
		}
	}

	// Show missing tools if any
	if len(summary.MissingTools) > 0 && len(summary.MissingTools) <= 10 {
		fmt.Println("\nMissing tools:")
		for _, tool := range summary.MissingTools {
			fmt.Printf("  - %s (%s)\n", tool.Name, tool.Package)
		}
	} else if len(summary.MissingTools) > 10 {
		fmt.Printf("\nMissing tools: %d (run 'grpm tools --missing' for details)\n", len(summary.MissingTools))
	}

	return nil
}

// showToolPaths displays the PATH directories used for tool detection.
func (a *App) showToolPaths() error {
	paths := tools.LookupPaths()

	fmt.Println("PATH directories used for tool detection:")
	fmt.Println()

	for i, path := range paths {
		fmt.Printf("  %d. %s\n", i+1, path)
	}

	if len(paths) == 0 {
		fmt.Println("  (no valid directories in PATH)")
	}

	return nil
}
