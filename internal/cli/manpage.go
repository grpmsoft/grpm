// Package cli provides command-line interface components for GRPM.
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ManPageGenerator generates man pages in troff/roff format for GRPM.
//
// Man pages are the standard documentation format for Unix/Linux command-line
// tools. This generator produces valid troff output that can be viewed with
// the `man` command or converted to other formats.
//
// Usage:
//
//	gen := NewManPageGenerator("0.9.1")
//	main := gen.GenerateMain()       // grpm.1
//	emerge := gen.GenerateCommand("emerge")  // grpm-emerge.1
type ManPageGenerator struct {
	registry *CommandRegistry
	version  string
	date     string
}

// NewManPageGenerator creates a new ManPageGenerator with the given version.
//
// The date is automatically set to the current month and year.
func NewManPageGenerator(version string) *ManPageGenerator {
	return &ManPageGenerator{
		registry: NewCommandRegistry(),
		version:  version,
		date:     time.Now().Format("January 2006"),
	}
}

// NewManPageGeneratorWithDate creates a ManPageGenerator with a specific date.
//
// This is useful for reproducible builds where the date should be fixed.
func NewManPageGeneratorWithDate(version, date string) *ManPageGenerator {
	return &ManPageGenerator{
		registry: NewCommandRegistry(),
		version:  version,
		date:     date,
	}
}

// GenerateMain generates the main man page (grpm.1) for the GRPM command.
//
// This page documents:
//   - Overall description of GRPM
//   - Synopsis with global options
//   - All available commands
//   - Global options
//   - Exit codes
//   - See also references
func (g *ManPageGenerator) GenerateMain() string {
	var sb strings.Builder

	// Title header
	sb.WriteString(g.formatTH("GRPM", "1", "User Commands"))

	// NAME section
	sb.WriteString(".SH NAME\n")
	sb.WriteString("grpm \\- Go Resource Package Manager for Gentoo Linux\n")

	// SYNOPSIS section
	sb.WriteString(".SH SYNOPSIS\n")
	sb.WriteString(".B grpm\n")
	sb.WriteString("[\\fIglobal-options\\fR] <\\fIcommand\\fR> [\\fIcommand-options\\fR] [\\fIarguments\\fR...]\n")

	// DESCRIPTION section
	sb.WriteString(".SH DESCRIPTION\n")
	sb.WriteString(".B grpm\n")
	sb.WriteString("is a next-generation package manager for Gentoo Linux written in Go.\n")
	sb.WriteString("It reimplements Portage with a focus on advanced dependency resolution\n")
	sb.WriteString("using SAT (Boolean Satisfiability) solvers, transactional updates with\n")
	sb.WriteString("filesystem snapshots, and maintaining full compatibility with the\n")
	sb.WriteString("existing Gentoo/Portage ecosystem.\n")
	sb.WriteString(".PP\n")
	sb.WriteString("Key features include:\n")
	sb.WriteString(".IP \\(bu 2\n")
	sb.WriteString("SAT-based dependency resolution for optimal package selection\n")
	sb.WriteString(".IP \\(bu 2\n")
	sb.WriteString("Daemon architecture with gRPC API for parallel operations\n")
	sb.WriteString(".IP \\(bu 2\n")
	sb.WriteString("Native repository sync (rsync and git with GPG verification)\n")
	sb.WriteString(".IP \\(bu 2\n")
	sb.WriteString("Binary package support (GPKG and TBZ2 formats)\n")
	sb.WriteString(".IP \\(bu 2\n")
	sb.WriteString("Filesystem snapshots for safe rollback (Btrfs/ZFS)\n")

	// COMMANDS section
	sb.WriteString(".SH COMMANDS\n")
	commands := g.registry.All()
	for _, cmd := range commands {
		sb.WriteString(".TP\n")
		sb.WriteString(fmt.Sprintf(".B %s\n", cmd.Name))
		sb.WriteString(fmt.Sprintf("%s\n", escapeManPage(cmd.Short)))
	}
	sb.WriteString(".PP\n")
	sb.WriteString("Run\n")
	sb.WriteString(".B grpm <command> --help\n")
	sb.WriteString("for command-specific help, or\n")
	sb.WriteString(".B man grpm-<command>\n")
	sb.WriteString("for the full manual page.\n")

	// GLOBAL OPTIONS section
	sb.WriteString(".SH GLOBAL OPTIONS\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".BR \\-V \", \" \\-\\-version\n")
	sb.WriteString("Show version information and exit.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".BR \\-h \", \" \\-\\-help\n")
	sb.WriteString("Show help information and exit.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".BR \\-v \", \" \\-vv \", \" \\-vvv\n")
	sb.WriteString("Increase verbosity level. Use \\fB-v\\fR for verbose, \\fB-vv\\fR for more\n")
	sb.WriteString("verbose, and \\fB-vvv\\fR for debug output.\n")

	// PACKAGE SETS section
	sb.WriteString(".SH PACKAGE SETS\n")
	sb.WriteString("GRPM supports package sets, which are collections of packages referenced\n")
	sb.WriteString("by a name prefixed with \\fB@\\fR:\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B @world\n")
	sb.WriteString("All packages explicitly installed by the user.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B @selected\n")
	sb.WriteString("Alias for @world.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B @system\n")
	sb.WriteString("Packages required for basic system functionality.\n")

	// EXIT STATUS section
	sb.WriteString(".SH EXIT STATUS\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B 0\n")
	sb.WriteString("Success.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B 1\n")
	sb.WriteString("General error (invalid arguments, command failure, etc.).\n")

	// FILES section
	sb.WriteString(".SH FILES\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".I /etc/portage/make.conf\n")
	sb.WriteString("Main Portage configuration file.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".I /etc/portage/make.profile\n")
	sb.WriteString("Symlink to the active system profile.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".I /var/db/repos/gentoo\n")
	sb.WriteString("Default Portage repository location.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".I /var/cache/binpkgs\n")
	sb.WriteString("Default binary package cache directory.\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".I /var/lib/portage/world\n")
	sb.WriteString("List of explicitly installed packages.\n")

	// SEE ALSO section
	sb.WriteString(".SH SEE ALSO\n")
	sb.WriteString(".BR emerge (1),\n")
	sb.WriteString(".BR portage (5),\n")
	sb.WriteString(".BR make.conf (5),\n")
	sb.WriteString(".BR ebuild (5)\n")

	// AUTHORS section
	sb.WriteString(".SH AUTHORS\n")
	sb.WriteString("GRPM is developed by the GRPM project.\n")
	sb.WriteString(".PP\n")
	sb.WriteString("Project homepage: https://github.com/grpmsoft/grpm\n")

	// BUGS section
	sb.WriteString(".SH BUGS\n")
	sb.WriteString("Report bugs at https://github.com/grpmsoft/grpm/issues\n")

	return sb.String()
}

// GenerateCommand generates a man page for a specific GRPM subcommand.
//
// The generated page name follows the format grpm-<command>.1
// (e.g., grpm-emerge.1, grpm-resolve.1).
//
// Returns empty string if the command is not found.
func (g *ManPageGenerator) GenerateCommand(cmdName string) string {
	meta := g.registry.Get(cmdName)
	if meta == nil {
		return ""
	}

	var sb strings.Builder

	// Title header
	pageName := fmt.Sprintf("GRPM-%s", strings.ToUpper(cmdName))
	sb.WriteString(g.formatTH(pageName, "1", "User Commands"))

	// NAME section
	sb.WriteString(".SH NAME\n")
	sb.WriteString(fmt.Sprintf("grpm-%s \\- %s\n", cmdName, escapeManPage(strings.ToLower(meta.Short))))

	// SYNOPSIS section
	sb.WriteString(".SH SYNOPSIS\n")
	sb.WriteString(".B grpm\n")
	sb.WriteString(fmt.Sprintf("%s\n", formatSynopsis(meta.Usage)))

	// DESCRIPTION section
	sb.WriteString(".SH DESCRIPTION\n")
	if meta.Long != "" {
		sb.WriteString(formatDescription(meta.Long))
	} else {
		sb.WriteString(fmt.Sprintf("%s\n", escapeManPage(meta.Short)))
	}

	// OPTIONS section
	if len(meta.Flags) > 0 {
		sb.WriteString(".SH OPTIONS\n")
		for _, flag := range meta.Flags {
			if flag.Hidden {
				continue
			}
			sb.WriteString(g.formatFlag(flag))
		}
	}

	// ALIASES section
	if len(meta.Aliases) > 0 {
		sb.WriteString(".SH ALIASES\n")
		sb.WriteString("This command can also be invoked as:\n")
		for _, alias := range meta.Aliases {
			sb.WriteString(".TP\n")
			sb.WriteString(fmt.Sprintf(".B grpm %s\n", alias))
		}
	}

	// EXAMPLES section
	if len(meta.Examples) > 0 {
		sb.WriteString(".SH EXAMPLES\n")
		for _, example := range meta.Examples {
			sb.WriteString(formatExample(example))
		}
	}

	// SEE ALSO section
	sb.WriteString(".SH SEE ALSO\n")
	sb.WriteString(".BR grpm (1)")

	// Add related commands from metadata
	if len(meta.SeeAlso) > 0 {
		for _, related := range meta.SeeAlso {
			sb.WriteString(fmt.Sprintf(",\n.BR grpm-%s (1)", related))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// GenerateAll generates all man pages and returns them as a map.
//
// The map keys are the man page filenames (e.g., "grpm.1", "grpm-emerge.1").
func (g *ManPageGenerator) GenerateAll() map[string]string {
	pages := make(map[string]string)

	// Main page
	pages["grpm.1"] = g.GenerateMain()

	// Command pages
	for _, cmd := range g.registry.All() {
		filename := fmt.Sprintf("grpm-%s.1", cmd.Name)
		pages[filename] = g.GenerateCommand(cmd.Name)
	}

	return pages
}

// CommandNames returns a sorted list of all command names.
func (g *ManPageGenerator) CommandNames() []string {
	commands := g.registry.All()
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name)
	}
	sort.Strings(names)
	return names
}

// formatTH formats the .TH (title header) macro.
//
// Format: .TH name section date source manual
func (g *ManPageGenerator) formatTH(name, section, manual string) string {
	return fmt.Sprintf(".TH %s %s \"%s\" \"GRPM v%s\" \"%s\"\n",
		name, section, g.date, g.version, manual)
}

// formatFlag formats a single flag for the OPTIONS section.
func (g *ManPageGenerator) formatFlag(flag FlagMeta) string {
	var sb strings.Builder

	sb.WriteString(".TP\n")

	// Format flag names
	if flag.Short != "" && flag.Long != "" {
		sb.WriteString(fmt.Sprintf(".BR \\-%s \", \" \\-\\-%s", flag.Short, escapeDashes(flag.Long)))
	} else if flag.Long != "" {
		sb.WriteString(fmt.Sprintf(".BR \\-\\-%s", escapeDashes(flag.Long)))
	}

	// Add type indicator for non-bool flags
	if flag.Type != "bool" && flag.Type != "" {
		sb.WriteString(fmt.Sprintf(" \" \" \\fI%s\\fR", flag.Type))
	}
	sb.WriteString("\n")

	// Description
	sb.WriteString(escapeManPage(flag.Description))

	// Default value for non-bool flags
	if flag.Type != "bool" && flag.Default != "" {
		sb.WriteString(fmt.Sprintf(" Default: \\fI%s\\fR.", escapeManPage(flag.Default)))
	}
	sb.WriteString("\n")

	return sb.String()
}

// formatSynopsis formats the synopsis line with proper troff markup.
func formatSynopsis(usage string) string {
	// Remove "grpm " prefix if present (we add it separately in .B grpm)
	result := strings.TrimPrefix(usage, "grpm ")

	// Make [flags] italic
	result = strings.ReplaceAll(result, "[flags]", "[\\fIflags\\fR]")

	// Make <package> style arguments bold-italic
	result = strings.ReplaceAll(result, "<package>", "<\\fIpackage\\fR>")
	result = strings.ReplaceAll(result, "<pattern>", "<\\fIpattern\\fR>")
	result = strings.ReplaceAll(result, "<target>", "<\\fItarget\\fR>")

	// Handle ... (multiple arguments)
	result = strings.ReplaceAll(result, "...", "\\&...")

	return result
}

// formatDescription formats the long description with proper paragraph breaks.
func formatDescription(desc string) string {
	var sb strings.Builder

	// Split into sentences for better formatting
	sentences := strings.Split(desc, ". ")
	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}
		sb.WriteString(escapeManPage(sentence))
		if i < len(sentences)-1 {
			sb.WriteString(".\n")
		} else if !strings.HasSuffix(sentence, ".") {
			sb.WriteString(".\n")
		} else {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatExample formats an example command line.
func formatExample(example string) string {
	var sb strings.Builder

	// Split on # to separate command from comment
	parts := strings.SplitN(example, "#", 2)

	sb.WriteString(".PP\n")
	sb.WriteString(".nf\n") // No-fill mode (preserve formatting)

	// Command part (bold)
	cmd := strings.TrimSpace(parts[0])
	sb.WriteString(fmt.Sprintf("\\fB%s\\fR", escapeManPage(cmd)))

	// Comment part (if present)
	if len(parts) > 1 {
		comment := strings.TrimSpace(parts[1])
		sb.WriteString(fmt.Sprintf("  # %s", escapeManPage(comment)))
	}

	sb.WriteString("\n.fi\n") // End no-fill mode

	return sb.String()
}

// escapeManPage escapes special characters for troff/roff format.
func escapeManPage(s string) string {
	// Escape backslashes first
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// Escape hyphens/dashes (troff uses them for hyphenation)
	s = strings.ReplaceAll(s, "-", "\\-")
	// Escape periods at the start of a line
	s = strings.ReplaceAll(s, "\n.", "\n\\&.")
	return s
}

// escapeDashes escapes dashes in flag names for troff.
func escapeDashes(s string) string {
	return strings.ReplaceAll(s, "-", "\\-")
}

// GetManPageInstallInstructions returns instructions for installing man pages.
func GetManPageInstallInstructions() string {
	return `Man Page Installation Instructions

Generate all man pages:
  mkdir -p man/man1
  grpm doc man --all --dir man/man1

Generate specific man page:
  grpm doc man > grpm.1
  grpm doc man emerge > grpm-emerge.1

Install to system (requires root):
  install -m 644 man/man1/*.1 /usr/share/man/man1/

View generated man page:
  grpm doc man | man -l -
  grpm doc man emerge | man -l -

Or save and view:
  grpm doc man > /tmp/grpm.1 && man /tmp/grpm.1
`
}
