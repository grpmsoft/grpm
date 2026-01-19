// Package cli provides command-line interface components for GRPM.
package cli

import (
	"fmt"
	"sort"
	"strings"
)

// CommandMeta holds metadata for generating professional help output.
//
// This structure provides all information needed to format help text
// comparable to Cobra-style CLI tools, while using only the standard
// library flag package.
type CommandMeta struct {
	// Name is the command name (e.g., "emerge", "resolve")
	Name string

	// Short is a brief one-line description
	Short string

	// Long is an extended description (optional, used in detailed help)
	Long string

	// Usage is the usage pattern (e.g., "emerge [flags] <package>...")
	Usage string

	// Examples are example commands with comments
	Examples []string

	// Flags are the flag definitions for this command
	Flags []FlagMeta

	// Aliases are command aliases (e.g., "unmerge" for "remove")
	Aliases []string

	// SeeAlso references related commands (optional)
	SeeAlso []string
}

// FlagMeta holds metadata for a single flag.
type FlagMeta struct {
	// Short is the short flag name without dash (e.g., "p")
	Short string

	// Long is the long flag name without dashes (e.g., "pretend")
	Long string

	// Type is the value type: "bool", "string", "int"
	Type string

	// Default is the default value for display (empty for bool flags)
	Default string

	// Description is the help text for this flag
	Description string

	// Hidden flags are not shown in help output
	Hidden bool
}

// HelpFormatter formats command help in a professional, Cobra-like style.
type HelpFormatter struct {
	// Width is the maximum line width (default 80)
	Width int

	// IndentSize is the indentation for flag descriptions (default 2)
	IndentSize int

	// FlagPadding is the padding between flag and description (default 4)
	FlagPadding int
}

// DefaultHelpFormatter returns a HelpFormatter with sensible defaults.
func DefaultHelpFormatter() *HelpFormatter {
	return &HelpFormatter{
		Width:       80,
		IndentSize:  2,
		FlagPadding: 4,
	}
}

// Format generates the complete help text for a command.
func (h *HelpFormatter) Format(meta CommandMeta) string {
	var sb strings.Builder

	// Header: command name and short description
	sb.WriteString(fmt.Sprintf("grpm %s - %s\n", meta.Name, meta.Short))

	// Long description (if provided)
	if meta.Long != "" {
		sb.WriteString("\n")
		sb.WriteString(h.wrapText(meta.Long, 0))
		sb.WriteString("\n")
	}

	// Usage
	sb.WriteString("\nUsage:\n")
	sb.WriteString(fmt.Sprintf("  grpm %s\n", meta.Usage))

	// Aliases (if any)
	if len(meta.Aliases) > 0 {
		sb.WriteString("\nAliases:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", strings.Join(meta.Aliases, ", ")))
	}

	// Flags
	visibleFlags := h.filterVisibleFlags(meta.Flags)
	if len(visibleFlags) > 0 {
		sb.WriteString("\nFlags:\n")
		sb.WriteString(h.FormatFlags(visibleFlags))
	}

	// Examples
	if len(meta.Examples) > 0 {
		sb.WriteString("\nExamples:\n")
		for _, example := range meta.Examples {
			sb.WriteString(fmt.Sprintf("  %s\n", example))
		}
	}

	// See also
	if len(meta.SeeAlso) > 0 {
		sb.WriteString("\nSee also:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", strings.Join(meta.SeeAlso, ", ")))
	}

	// Footer
	sb.WriteString("\nRun 'grpm --help' for global options.\n")

	return sb.String()
}

// FormatFlags formats a list of flags in aligned columns.
//
// Output format:
//
//	-p, --pretend       Show what would be done (dry-run)
//	-a, --ask           Ask for confirmation
//	    --root string   Installation root (default "/")
func (h *HelpFormatter) FormatFlags(flags []FlagMeta) string {
	if len(flags) == 0 {
		return ""
	}

	// Calculate the maximum flag column width
	maxFlagWidth := 0
	for _, f := range flags {
		width := h.flagColumnWidth(f)
		if width > maxFlagWidth {
			maxFlagWidth = width
		}
	}

	// Build output
	var sb strings.Builder
	for _, f := range flags {
		line := h.formatSingleFlag(f, maxFlagWidth)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// filterVisibleFlags returns only non-hidden flags.
func (h *HelpFormatter) filterVisibleFlags(flags []FlagMeta) []FlagMeta {
	visible := make([]FlagMeta, 0, len(flags))
	for _, f := range flags {
		if !f.Hidden {
			visible = append(visible, f)
		}
	}
	return visible
}

// flagColumnWidth calculates the width needed for the flag column.
func (h *HelpFormatter) flagColumnWidth(f FlagMeta) int {
	width := h.IndentSize

	// Short flag: "-p, " (4 chars) or "    " (4 chars)
	width += 4

	// Long flag: "--pretend" + type suffix
	width += 2 + len(f.Long)

	// Type suffix: " string", " int"
	switch f.Type {
	case "string":
		width += 7 // " string"
	case "int":
		width += 4 // " int"
	}

	return width
}

// formatSingleFlag formats a single flag line.
func (h *HelpFormatter) formatSingleFlag(f FlagMeta, maxWidth int) string {
	var sb strings.Builder

	// Indent
	sb.WriteString(strings.Repeat(" ", h.IndentSize))

	// Short flag
	if f.Short != "" {
		sb.WriteString(fmt.Sprintf("-%s, ", f.Short))
	} else {
		sb.WriteString("    ")
	}

	// Long flag
	sb.WriteString(fmt.Sprintf("--%s", f.Long))

	// Type suffix
	switch f.Type {
	case "string":
		sb.WriteString(" string")
	case "int":
		sb.WriteString(" int")
	}

	// Padding to align descriptions
	currentWidth := sb.Len()
	padding := maxWidth + h.FlagPadding - currentWidth
	if padding < 2 {
		padding = 2
	}
	sb.WriteString(strings.Repeat(" ", padding))

	// Description
	sb.WriteString(f.Description)

	// Default value (for non-bool flags with non-empty defaults)
	if f.Type != "bool" && f.Default != "" {
		sb.WriteString(fmt.Sprintf(" (default %q)", f.Default))
	}

	return sb.String()
}

// wrapText wraps text to the configured width with the given indent.
func (h *HelpFormatter) wrapText(text string, indent int) string {
	if text == "" {
		return ""
	}

	maxLen := h.Width - indent
	if maxLen < 20 {
		maxLen = 20
	}

	indentStr := strings.Repeat(" ", indent)
	var lines []string
	var currentLine strings.Builder

	words := strings.Fields(text)
	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= maxLen {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, indentStr+currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, indentStr+currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// CommandRegistry holds metadata for all commands.
//
// This provides a central location for command metadata, allowing
// consistent help generation across the CLI.
type CommandRegistry struct {
	commands map[string]CommandMeta
}

// NewCommandRegistry creates a new command registry with all GRPM commands.
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		commands: make(map[string]CommandMeta),
	}
	r.registerAllCommands()
	return r
}

// Get returns the CommandMeta for a command name, or nil if not found.
func (r *CommandRegistry) Get(name string) *CommandMeta {
	if meta, ok := r.commands[name]; ok {
		return &meta
	}
	return nil
}

// All returns all registered commands sorted by name.
func (r *CommandRegistry) All() []CommandMeta {
	commands := make([]CommandMeta, 0, len(r.commands))
	for _, meta := range r.commands {
		commands = append(commands, meta)
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})
	return commands
}

// registerAllCommands registers all GRPM CLI commands.
func (r *CommandRegistry) registerAllCommands() {
	// resolve command
	r.commands["resolve"] = CommandMeta{
		Name:  "resolve",
		Short: "Resolve package dependencies using SAT solver",
		Long: "Resolves package dependencies using a Boolean Satisfiability (SAT) solver. " +
			"This command calculates the complete dependency graph for the specified packages " +
			"without installing them.",
		Usage: "resolve [flags] <package>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be done (dry-run)"},
			{Long: "dry-run", Type: "bool", Description: "Alias for --pretend", Hidden: true},
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
			{Long: "autounmask", Type: "bool", Description: "Show USE/keyword changes to resolve conflicts"},
			{Long: "autounmask-write", Type: "bool", Description: "Write autounmask changes to /etc/portage"},
		},
		Examples: []string{
			"grpm resolve app-misc/hello          # Resolve single package",
			"grpm resolve -p sys-libs/zlib        # Pretend mode (show dependencies)",
			"grpm resolve @world                  # Resolve all world packages",
			"grpm resolve --autounmask gcc        # Show USE changes needed",
		},
		SeeAlso: []string{"install", "emerge"},
	}

	// install command
	r.commands["install"] = CommandMeta{
		Name:  "install",
		Short: "Install packages (binary or source)",
		Long: "Installs packages to the system. Supports both binary packages (.gpkg.tar) " +
			"and source builds. Creates filesystem snapshots for safe rollback.",
		Usage: "install [flags] <package>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be installed (dry-run)"},
			{Short: "a", Long: "ask", Type: "bool", Description: "Ask for confirmation before installing"},
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
			{Long: "binpkg", Type: "bool", Description: "Prefer binary packages when available"},
			{Long: "binpkg-dir", Type: "string", Default: "/var/cache/binpkgs", Description: "Directory with binary packages"},
			{Long: "snapshot-dir", Type: "string", Default: "/.snapshots", Description: "Snapshot directory"},
			{Long: "fs-type", Type: "string", Default: "btrfs", Description: "Filesystem type (btrfs or zfs)"},
			{Long: "no-snapshot", Type: "bool", Description: "Skip snapshot creation"},
			{Long: "autounmask", Type: "bool", Description: "Show USE/keyword changes to resolve conflicts"},
			{Long: "autounmask-write", Type: "bool", Description: "Write autounmask changes to /etc/portage"},
		},
		Examples: []string{
			"grpm install app-misc/hello          # Install from source",
			"grpm install -p sys-libs/zlib        # Pretend mode",
			"grpm install --binpkg @world         # Install binaries for world",
			"grpm install -a --binpkg gcc glibc   # Ask before installing binaries",
		},
		SeeAlso: []string{"resolve", "emerge", "remove"},
	}

	// emerge command
	r.commands["emerge"] = CommandMeta{
		Name:  "emerge",
		Short: "Build packages from source (Portage-compatible)",
		Long: "Builds packages from source using the Portage ebuild system. " +
			"Downloads sources, executes ebuild phases (unpack, configure, compile, install), " +
			"and merges to the system. Supports parallel builds with dependency ordering.",
		Usage: "emerge [flags] <package>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be built (dry-run)"},
			{Short: "a", Long: "ask", Type: "bool", Description: "Ask for confirmation before building"},
			{Short: "j", Long: "jobs", Type: "int", Default: "1", Description: "Number of packages to build in parallel"},
			{Short: "k", Long: "keep-going", Type: "bool", Description: "Continue building on failure"},
			{Short: "R", Long: "replace", Type: "bool", Description: "Replace existing package (ignore same-package collisions)"},
			{Short: "f", Long: "force", Type: "bool", Description: "Force installation (skip collision checks)"},
			{Short: "o", Long: "onlydeps", Type: "bool", Description: "Build dependencies only, not the target package(s)"},
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "distdir", Type: "string", Default: "/var/cache/distfiles", Description: "Directory for downloaded sources"},
			{Long: "tmpdir", Type: "string", Default: "/var/tmp/portage", Description: "Temporary build directory"},
			{Long: "root", Type: "string", Default: "/", Description: "Installation root directory (like $ROOT)"},
			{Long: "make-jobs", Type: "int", Default: "4", Description: "Number of parallel make jobs per package"},
			{Long: "keep-work", Type: "bool", Description: "Keep work directory after build"},
			{Long: "test", Type: "bool", Description: "Run test phase"},
			{Long: "check-tools", Type: "bool", Description: "Check external tool availability before build"},
			{Long: "info", Type: "bool", Description: "Show system environment information"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
		},
		Examples: []string{
			"grpm emerge app-misc/hello           # Build single package",
			"grpm emerge -p @world                # Pretend update world",
			"grpm emerge -j4 --keep-going @world  # Parallel build, continue on errors",
			"grpm emerge --root /mnt/gentoo gcc   # Build to alternate root",
			"grpm emerge -o sys-devel/gcc         # Build gcc dependencies only",
			"grpm emerge --info                   # Show system info",
		},
		SeeAlso: []string{"install", "resolve", "fetch"},
	}

	// remove command
	r.commands["remove"] = CommandMeta{
		Name:    "remove",
		Short:   "Remove installed packages",
		Long:    "Removes packages from the system. Use --depclean to also remove orphaned dependencies.",
		Usage:   "remove [flags] <package>...",
		Aliases: []string{"uninstall", "unmerge"},
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be removed (dry-run)"},
			{Short: "c", Long: "depclean", Type: "bool", Description: "Also remove unused dependencies"},
			{Long: "force", Type: "bool", Description: "Force removal even if dependencies exist"},
		},
		Examples: []string{
			"grpm remove app-misc/hello           # Remove single package",
			"grpm remove -p sys-libs/zlib         # Pretend mode",
			"grpm remove -c app-misc/hello        # Remove with orphan cleanup",
		},
		SeeAlso: []string{"depclean", "install"},
	}

	// search command
	r.commands["search"] = CommandMeta{
		Name:  "search",
		Short: "Search for packages in repository",
		Long:  "Searches for packages by name in the Portage repository. Optionally searches package descriptions.",
		Usage: "search [flags] <pattern>",
		Flags: []FlagMeta{
			{Short: "S", Long: "desc", Type: "bool", Description: "Search in descriptions too"},
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
		},
		Examples: []string{
			"grpm search hello                    # Search by name",
			"grpm search -S 'web server'          # Search in descriptions",
			"grpm search --repo /path/overlay vim # Search custom overlay",
		},
		SeeAlso: []string{"info"},
	}

	// info command
	r.commands["info"] = CommandMeta{
		Name:  "info",
		Short: "Show detailed package information",
		Long:  "Displays detailed information about a package including version, slot, USE flags, and dependencies.",
		Usage: "info [flags] <package>",
		Flags: []FlagMeta{
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
		},
		Examples: []string{
			"grpm info app-misc/hello             # Show package info",
			"grpm info sys-devel/gcc              # Show gcc info",
			"grpm info =sys-devel/gcc-13.4.1      # Show specific version",
		},
		SeeAlso: []string{"search"},
	}

	// sync command
	r.commands["sync"] = CommandMeta{
		Name:  "sync",
		Short: "Synchronize Portage repository",
		Long: "Synchronizes the local Portage repository with upstream. " +
			"Supports rsync and git methods with automatic selection. " +
			"Git sync includes GPG signature verification.",
		Usage: "sync [flags]",
		Flags: []FlagMeta{
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Repository path to sync"},
			{Long: "url", Type: "string", Description: "Source repository URL (auto-detected)"},
			{Long: "method", Type: "string", Default: "auto", Description: "Sync method: rsync, git, or auto"},
			{Long: "prefer-git", Type: "bool", Description: "Prefer Git over rsync when using auto method"},
			{Long: "skip-gpg-verify", Type: "bool", Description: "Skip GPG signature verification (NOT RECOMMENDED)"},
		},
		Examples: []string{
			"grpm sync                            # Sync default repo (auto method)",
			"grpm sync --method git               # Force git sync",
			"grpm sync --repo /var/db/repos/guru  # Sync custom overlay",
		},
		SeeAlso: []string{"resolve"},
	}

	// update command
	r.commands["update"] = CommandMeta{
		Name:  "update",
		Short: "Update installed packages",
		Long: "Calculates and applies updates for installed packages. " +
			"Supports package sets (@world, @selected, @system) and individual packages.",
		Usage: "update [flags] [<target>...]",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be updated (dry-run)"},
			{Short: "a", Long: "ask", Type: "bool", Description: "Ask for confirmation before updating"},
			{Short: "D", Long: "deep", Type: "bool", Description: "Include dependencies in update calculation"},
			{Short: "N", Long: "newuse", Type: "bool", Description: "Recalculate USE flags and update if changed"},
			{Short: "U", Long: "changed-use", Type: "bool", Description: "Only update packages with changed USE flags"},
			{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "mock", Type: "bool", Description: "Use mock repository for testing"},
			{Long: "portage-dir", Type: "string", Default: "/var/lib/portage", Description: "Portage state directory"},
			{Long: "profile", Type: "string", Default: "/etc/portage/make.profile", Description: "Path to active profile"},
		},
		Examples: []string{
			"grpm update                          # Update @world (default)",
			"grpm update -p @world                # Pretend update world",
			"grpm update -DN @world               # Deep update with USE recalc",
			"grpm update app-misc/hello           # Update specific package",
		},
		SeeAlso: []string{"emerge", "depclean"},
	}

	// build command
	r.commands["build"] = CommandMeta{
		Name:  "build",
		Short: "Create binary packages from installed packages",
		Long:  "Creates binary packages (.gpkg.tar or .tbz2) from installed packages for redistribution or faster reinstallation.",
		Usage: "build [flags] <package>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be built (dry-run)"},
			{Long: "output", Type: "string", Default: "/var/cache/binpkgs", Description: "Output directory for binary packages"},
			{Long: "format", Type: "string", Default: "gpkg", Description: "Package format: gpkg or tbz2"},
			{Long: "compression", Type: "string", Default: "zstd", Description: "Compression: none, gzip, bzip2, xz, zstd"},
		},
		Examples: []string{
			"grpm build app-misc/hello            # Build binary package",
			"grpm build -p sys-libs/zlib          # Pretend mode",
			"grpm build --format tbz2 @world      # Build legacy format",
		},
		SeeAlso: []string{"install", "emerge"},
	}

	// depclean command
	r.commands["depclean"] = CommandMeta{
		Name:  "depclean",
		Short: "Remove unused dependencies (orphan packages)",
		Long: "Identifies and removes packages that are not in the @world set " +
			"and are not required by any @world package as a dependency. " +
			"This is the GRPM equivalent of Portage's 'emerge --depclean'.",
		Usage: "depclean [flags]",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be removed (dry-run)"},
			{Short: "a", Long: "ask", Type: "bool", Description: "Ask for confirmation before removing"},
			{Long: "exclude", Type: "string", Description: "Exclude package from removal (can be repeated)"},
			{Long: "portage-dir", Type: "string", Default: "/var/lib/portage", Description: "Portage state directory"},
			{Long: "profile", Type: "string", Default: "/etc/portage/make.profile", Description: "Path to active profile"},
		},
		Examples: []string{
			"grpm depclean                        # Remove all orphans",
			"grpm depclean -p                     # Pretend mode",
			"grpm depclean --exclude sys-libs/glibc  # Keep specific package",
		},
		SeeAlso: []string{"remove", "update"},
	}

	// fetch command
	r.commands["fetch"] = CommandMeta{
		Name:  "fetch",
		Short: "Download source files (distfiles) without building",
		Long: "Downloads source tarballs for specified packages without building them. " +
			"Useful for pre-fetching sources before offline builds or verifying source availability.",
		Usage: "fetch [flags] <package>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Show what would be downloaded (dry-run)"},
			{Short: "r", Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Long: "distdir", Type: "string", Default: "/var/cache/distfiles", Description: "Directory for downloaded sources"},
			{Long: "verify", Type: "bool", Description: "Only verify existing files, don't download"},
		},
		Examples: []string{
			"grpm fetch app-misc/hello            # Download sources",
			"grpm fetch -p sys-devel/gcc          # Pretend mode",
			"grpm fetch --verify @world           # Verify all distfiles",
		},
		SeeAlso: []string{"emerge"},
	}

	// analyze command
	r.commands["analyze"] = CommandMeta{
		Name:  "analyze",
		Short: "Analyze Portage repository coverage",
		Long: "Analyzes a Portage repository and reports coverage metrics: " +
			"total packages, packages GRPM can build, and packages blocked by missing features. " +
			"Provides detailed breakdown by category and eclass.",
		Usage: "analyze [flags]",
		Flags: []FlagMeta{
			{Short: "r", Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Path to Portage repository"},
			{Short: "o", Long: "output", Type: "string", Default: "text", Description: "Output format: text, json, markdown"},
			{Short: "c", Long: "category", Type: "string", Description: "Analyze specific category only"},
			{Short: "v", Long: "verbose", Type: "bool", Description: "Show details for each package"},
		},
		Examples: []string{
			"grpm analyze                         # Analyze default repo",
			"grpm analyze -c app-misc             # Single category",
			"grpm analyze -o json > coverage.json # JSON output",
			"grpm analyze -o markdown > REPORT.md # Markdown report",
		},
		SeeAlso: []string{"tools"},
	}

	// tools command
	r.commands["tools"] = CommandMeta{
		Name:  "tools",
		Short: "Check external tool availability",
		Long: "Checks which external build tools are available on the system. " +
			"Provides actionable suggestions for installing missing tools " +
			"and shows tools required by specific eclasses.",
		Usage: "tools [flags]",
		Flags: []FlagMeta{
			{Long: "check", Type: "bool", Description: "Check all tools and show summary"},
			{Long: "missing", Type: "bool", Description: "Show only missing tools"},
			{Long: "available", Type: "bool", Description: "Show only available tools"},
			{Short: "c", Long: "category", Type: "string", Description: "Filter by category (compilers, build-systems, etc.)"},
			{Short: "e", Long: "for-eclass", Type: "string", Description: "Show tools needed for specific eclass"},
			{Long: "paths", Type: "bool", Description: "Show PATH directories used for detection"},
		},
		Examples: []string{
			"grpm tools                           # List all tools",
			"grpm tools --check                   # Show summary",
			"grpm tools --missing                 # Show missing tools",
			"grpm tools --for-eclass cmake        # Tools for cmake.eclass",
			"grpm tools --category compilers      # Show compiler tools",
		},
		SeeAlso: []string{"analyze", "emerge"},
	}

	// completion command
	r.commands["completion"] = CommandMeta{
		Name:  "completion",
		Short: "Generate shell completion scripts",
		Long: "Generates shell completion scripts for bash, zsh, or fish. " +
			"The generated scripts provide command and flag completion for GRPM. " +
			"Run without arguments to see installation instructions.",
		Usage: "completion [bash|zsh|fish]",
		Flags: []FlagMeta{},
		Examples: []string{
			"grpm completion                      # Show installation instructions",
			"grpm completion bash                 # Output bash completion script",
			"grpm completion zsh                  # Output zsh completion script",
			"grpm completion fish                 # Output fish completion script",
			"grpm completion bash > /etc/bash_completion.d/grpm  # Install bash completion",
		},
		SeeAlso: []string{"help"},
	}

	// doc command
	r.commands["doc"] = CommandMeta{
		Name:  "doc",
		Short: "Generate documentation (man pages)",
		Long: "Generates documentation for GRPM in various formats. " +
			"Currently supports man page generation in troff/roff format. " +
			"Man pages can be output to stdout or written to files.",
		Usage: "doc <subcommand> [options]",
		Flags: []FlagMeta{
			{Long: "all", Type: "bool", Description: "Generate all man pages (requires --dir)"},
			{Long: "dir", Type: "string", Description: "Output directory for man pages"},
			{Long: "list", Type: "bool", Description: "List all available man pages"},
		},
		Examples: []string{
			"grpm doc man                         # Output main man page to stdout",
			"grpm doc man emerge                  # Output emerge man page to stdout",
			"grpm doc man --list                  # List all available man pages",
			"grpm doc man --all --dir ./man       # Generate all man pages to directory",
			"grpm doc man | man -l -              # View generated man page directly",
		},
		SeeAlso: []string{"help", "completion"},
	}
}

// FormatMainHelp generates the main help text for the grpm command.
//
// This is used when running 'grpm --help' or 'grpm help'.
func FormatMainHelp(version string, commands []CommandMeta) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("GRPM - Go Resource Package Manager v%s\n", version))
	sb.WriteString("\nUsage: grpm [global-options] <command> [command-options] [arguments...]\n")

	sb.WriteString("\nGlobal Options:\n")
	sb.WriteString("  -V, --version    Show version information\n")
	sb.WriteString("  -v, -vv, -vvv    Verbose output (levels 1-3)\n")

	sb.WriteString("\nCommands:\n")

	// Find max command name width for alignment
	maxNameLen := 0
	for _, cmd := range commands {
		if len(cmd.Name) > maxNameLen {
			maxNameLen = len(cmd.Name)
		}
	}

	// Sort commands alphabetically
	sortedCmds := make([]CommandMeta, len(commands))
	copy(sortedCmds, commands)
	sort.Slice(sortedCmds, func(i, j int) bool {
		return sortedCmds[i].Name < sortedCmds[j].Name
	})

	// Format commands
	for _, cmd := range sortedCmds {
		padding := strings.Repeat(" ", maxNameLen-len(cmd.Name)+2)
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", cmd.Name, padding, cmd.Short))
	}

	sb.WriteString("\nRun 'grpm <command> --help' for command-specific help.\n")
	sb.WriteString("See docs/CLI_REFERENCE.md for detailed documentation.\n")

	return sb.String()
}

// GetCommandHelp returns formatted help text for a specific command.
//
// Returns empty string if command is not found.
func GetCommandHelp(name string) string {
	registry := NewCommandRegistry()
	meta := registry.Get(name)
	if meta == nil {
		return ""
	}

	formatter := DefaultHelpFormatter()
	return formatter.Format(*meta)
}
