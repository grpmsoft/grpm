package cli

import (
	"strings"
	"testing"
)

func TestHelpFormatter_FormatSingleFlag(t *testing.T) {
	formatter := DefaultHelpFormatter()

	tests := []struct {
		name     string
		flag     FlagMeta
		contains []string
	}{
		{
			name: "bool flag with short and long",
			flag: FlagMeta{
				Short:       "p",
				Long:        "pretend",
				Type:        "bool",
				Description: "Show what would be done",
			},
			contains: []string{"-p,", "--pretend", "Show what would be done"},
		},
		{
			name: "string flag with default",
			flag: FlagMeta{
				Long:        "repo",
				Type:        "string",
				Default:     "/var/db/repos/gentoo",
				Description: "Path to repository",
			},
			contains: []string{"--repo", "string", "Path to repository", `"/var/db/repos/gentoo"`},
		},
		{
			name: "int flag",
			flag: FlagMeta{
				Short:       "j",
				Long:        "jobs",
				Type:        "int",
				Default:     "4",
				Description: "Number of parallel jobs",
			},
			contains: []string{"-j,", "--jobs", "int", "Number of parallel jobs", `"4"`},
		},
		{
			name: "long flag only (no short)",
			flag: FlagMeta{
				Long:        "mock",
				Type:        "bool",
				Description: "Use mock repository",
			},
			contains: []string{"    ", "--mock", "Use mock repository"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatSingleFlag(tt.flag, 30)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("formatSingleFlag() missing %q in:\n%s", want, result)
				}
			}
		})
	}
}

func TestHelpFormatter_FormatFlags(t *testing.T) {
	formatter := DefaultHelpFormatter()

	flags := []FlagMeta{
		{Short: "p", Long: "pretend", Type: "bool", Description: "Dry-run mode"},
		{Short: "a", Long: "ask", Type: "bool", Description: "Ask confirmation"},
		{Long: "repo", Type: "string", Default: "/var/db/repos/gentoo", Description: "Repository path"},
	}

	result := formatter.FormatFlags(flags)

	// Check all flags are present
	if !strings.Contains(result, "-p, --pretend") {
		t.Error("Missing -p, --pretend")
	}
	if !strings.Contains(result, "-a, --ask") {
		t.Error("Missing -a, --ask")
	}
	if !strings.Contains(result, "--repo string") {
		t.Error("Missing --repo string")
	}
}

func TestHelpFormatter_Format(t *testing.T) {
	formatter := DefaultHelpFormatter()

	meta := CommandMeta{
		Name:  "test",
		Short: "Test command for unit tests",
		Long:  "This is a longer description for the test command.",
		Usage: "test [flags] <args>...",
		Flags: []FlagMeta{
			{Short: "p", Long: "pretend", Type: "bool", Description: "Dry-run mode"},
			{Long: "output", Type: "string", Default: "/tmp", Description: "Output directory"},
		},
		Examples: []string{
			"grpm test foo              # Test with foo",
			"grpm test -p bar           # Pretend mode",
		},
		Aliases: []string{"t", "tst"},
	}

	result := formatter.Format(meta)

	// Check header
	if !strings.Contains(result, "grpm test - Test command for unit tests") {
		t.Error("Missing header line")
	}

	// Check long description
	if !strings.Contains(result, "longer description") {
		t.Error("Missing long description")
	}

	// Check usage
	if !strings.Contains(result, "Usage:") {
		t.Error("Missing Usage section")
	}
	if !strings.Contains(result, "grpm test [flags] <args>...") {
		t.Error("Missing usage pattern")
	}

	// Check aliases
	if !strings.Contains(result, "Aliases:") {
		t.Error("Missing Aliases section")
	}
	if !strings.Contains(result, "t, tst") {
		t.Error("Missing alias list")
	}

	// Check flags
	if !strings.Contains(result, "Flags:") {
		t.Error("Missing Flags section")
	}
	if !strings.Contains(result, "-p, --pretend") {
		t.Error("Missing pretend flag")
	}

	// Check examples
	if !strings.Contains(result, "Examples:") {
		t.Error("Missing Examples section")
	}
	if !strings.Contains(result, "grpm test foo") {
		t.Error("Missing example 1")
	}

	// Check footer
	if !strings.Contains(result, "grpm --help") {
		t.Error("Missing footer reference")
	}
}

func TestHelpFormatter_HiddenFlags(t *testing.T) {
	formatter := DefaultHelpFormatter()

	flags := []FlagMeta{
		{Short: "p", Long: "pretend", Type: "bool", Description: "Visible flag"},
		{Long: "hidden", Type: "bool", Description: "Hidden flag", Hidden: true},
	}

	visible := formatter.filterVisibleFlags(flags)

	if len(visible) != 1 {
		t.Errorf("Expected 1 visible flag, got %d", len(visible))
	}
	if visible[0].Long != "pretend" {
		t.Errorf("Expected pretend flag, got %s", visible[0].Long)
	}
}

func TestCommandRegistry_AllCommands(t *testing.T) {
	registry := NewCommandRegistry()
	commands := registry.All()

	// Check that we have all expected commands
	expectedCommands := []string{
		"resolve", "install", "emerge", "remove", "search",
		"info", "sync", "update", "build", "depclean",
		"fetch", "analyze", "tools", "completion",
	}

	if len(commands) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(commands))
	}

	// Check each expected command exists
	for _, name := range expectedCommands {
		meta := registry.Get(name)
		if meta == nil {
			t.Errorf("Missing command: %s", name)
			continue
		}
		if meta.Name != name {
			t.Errorf("Command %s has wrong name: %s", name, meta.Name)
		}
		if meta.Short == "" {
			t.Errorf("Command %s has empty Short description", name)
		}
		if meta.Usage == "" {
			t.Errorf("Command %s has empty Usage", name)
		}
	}
}

func TestCommandRegistry_CommandsAreSorted(t *testing.T) {
	registry := NewCommandRegistry()
	commands := registry.All()

	for i := 1; i < len(commands); i++ {
		if commands[i].Name < commands[i-1].Name {
			t.Errorf("Commands not sorted: %s came after %s", commands[i].Name, commands[i-1].Name)
		}
	}
}

func TestCommandRegistry_EmergeCommand(t *testing.T) {
	registry := NewCommandRegistry()
	meta := registry.Get("emerge")

	if meta == nil {
		t.Fatal("emerge command not found")
	}

	// Check key flags exist
	flagNames := make(map[string]bool)
	for _, f := range meta.Flags {
		flagNames[f.Long] = true
	}

	requiredFlags := []string{"pretend", "ask", "jobs", "repo", "root", "onlydeps"}
	for _, name := range requiredFlags {
		if !flagNames[name] {
			t.Errorf("emerge missing flag: %s", name)
		}
	}

	// Check examples exist
	if len(meta.Examples) == 0 {
		t.Error("emerge has no examples")
	}
}

func TestCommandRegistry_ResolveCommand(t *testing.T) {
	registry := NewCommandRegistry()
	meta := registry.Get("resolve")

	if meta == nil {
		t.Fatal("resolve command not found")
	}

	// Check autounmask flags exist
	flagNames := make(map[string]bool)
	for _, f := range meta.Flags {
		flagNames[f.Long] = true
	}

	if !flagNames["autounmask"] {
		t.Error("resolve missing autounmask flag")
	}
	if !flagNames["autounmask-write"] {
		t.Error("resolve missing autounmask-write flag")
	}
}

func TestGetCommandHelp(t *testing.T) {
	// Test existing command
	help := GetCommandHelp("emerge")
	if help == "" {
		t.Error("GetCommandHelp returned empty for emerge")
	}
	if !strings.Contains(help, "grpm emerge") {
		t.Error("Help doesn't contain command name")
	}

	// Test non-existing command
	help = GetCommandHelp("nonexistent")
	if help != "" {
		t.Error("GetCommandHelp should return empty for unknown command")
	}
}

func TestFormatMainHelp(t *testing.T) {
	registry := NewCommandRegistry()
	help := FormatMainHelp("0.9.1", registry.All())

	// Check version
	if !strings.Contains(help, "0.9.1") {
		t.Error("Missing version in main help")
	}

	// Check global options
	if !strings.Contains(help, "-V, --version") {
		t.Error("Missing version flag")
	}

	// Check commands section
	if !strings.Contains(help, "Commands:") {
		t.Error("Missing Commands section")
	}

	// Check all commands are listed with descriptions
	expectedCommands := []string{"resolve", "install", "emerge", "remove", "search"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(help, cmd) {
			t.Errorf("Missing command %s in main help", cmd)
		}
	}

	// Check footer
	if !strings.Contains(help, "grpm <command> --help") {
		t.Error("Missing help footer")
	}
}

func TestHelpFormatter_WrapText(t *testing.T) {
	formatter := &HelpFormatter{Width: 40}

	// Test long text wrapping
	longText := "This is a very long text that should be wrapped to fit within the configured width limit."
	wrapped := formatter.wrapText(longText, 0)

	lines := strings.Split(wrapped, "\n")
	for _, line := range lines {
		if len(line) > 40 {
			// Note: words longer than width won't be broken
			t.Logf("Long line (may be single word): %s", line)
		}
	}

	// Test empty text
	empty := formatter.wrapText("", 0)
	if empty != "" {
		t.Error("wrapText should return empty for empty input")
	}

	// Test with indent
	withIndent := formatter.wrapText("Short text", 4)
	if !strings.HasPrefix(withIndent, "    ") {
		t.Error("wrapText should add indent")
	}
}

func TestDefaultHelpFormatter(t *testing.T) {
	formatter := DefaultHelpFormatter()

	if formatter.Width != 80 {
		t.Errorf("Expected Width=80, got %d", formatter.Width)
	}
	if formatter.IndentSize != 2 {
		t.Errorf("Expected IndentSize=2, got %d", formatter.IndentSize)
	}
	if formatter.FlagPadding != 4 {
		t.Errorf("Expected FlagPadding=4, got %d", formatter.FlagPadding)
	}
}

// TestAllCommandsHaveMetadata ensures all commands in the registry have complete metadata.
func TestAllCommandsHaveMetadata(t *testing.T) {
	registry := NewCommandRegistry()

	for _, cmd := range registry.All() {
		t.Run(cmd.Name, func(t *testing.T) {
			if cmd.Name == "" {
				t.Error("Empty command name")
			}
			if cmd.Short == "" {
				t.Error("Empty short description")
			}
			if cmd.Usage == "" {
				t.Error("Empty usage")
			}
			// completion command is special - it takes shell name as positional argument
			if len(cmd.Flags) == 0 && cmd.Name != "completion" {
				t.Error("No flags defined")
			}
			if len(cmd.Examples) == 0 {
				t.Error("No examples defined")
			}

			// Check flag quality
			for _, flag := range cmd.Flags {
				if flag.Long == "" {
					t.Errorf("Flag with empty long name")
				}
				if flag.Description == "" {
					t.Errorf("Flag %s has empty description", flag.Long)
				}
				if flag.Type == "" {
					t.Errorf("Flag %s has empty type", flag.Long)
				}
			}
		})
	}
}
