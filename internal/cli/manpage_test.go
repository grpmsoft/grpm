package cli

import (
	"strings"
	"testing"
)

func TestNewManPageGenerator(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")

	if gen == nil {
		t.Fatal("NewManPageGenerator returned nil")
	}
	if gen.version != "0.9.1" {
		t.Errorf("Expected version 0.9.1, got %s", gen.version)
	}
	if gen.date == "" {
		t.Error("Date should not be empty")
	}
	if gen.registry == nil {
		t.Error("Registry should not be nil")
	}
}

func TestNewManPageGeneratorWithDate(t *testing.T) {
	gen := NewManPageGeneratorWithDate("0.9.1", "January 2026")

	if gen.version != "0.9.1" {
		t.Errorf("Expected version 0.9.1, got %s", gen.version)
	}
	if gen.date != "January 2026" {
		t.Errorf("Expected date 'January 2026', got %s", gen.date)
	}
}

func TestManPageGenerator_GenerateMain(t *testing.T) {
	gen := NewManPageGeneratorWithDate("0.9.1", "January 2026")
	page := gen.GenerateMain()

	// Check TH header
	if !strings.Contains(page, ".TH GRPM 1") {
		t.Error("Missing .TH header")
	}
	if !strings.Contains(page, "GRPM v0.9.1") {
		t.Error("Missing version in header")
	}
	if !strings.Contains(page, "January 2026") {
		t.Error("Missing date in header")
	}

	// Check required sections
	requiredSections := []string{
		".SH NAME",
		".SH SYNOPSIS",
		".SH DESCRIPTION",
		".SH COMMANDS",
		".SH GLOBAL OPTIONS",
		".SH PACKAGE SETS",
		".SH EXIT STATUS",
		".SH FILES",
		".SH SEE ALSO",
		".SH AUTHORS",
		".SH BUGS",
	}
	for _, section := range requiredSections {
		if !strings.Contains(page, section) {
			t.Errorf("Missing section: %s", section)
		}
	}

	// Check content
	if !strings.Contains(page, "Go Resource Package Manager") {
		t.Error("Missing description text")
	}
	if !strings.Contains(page, "emerge") {
		t.Error("Missing emerge command in COMMANDS section")
	}
	if !strings.Contains(page, "resolve") {
		t.Error("Missing resolve command in COMMANDS section")
	}
	if !strings.Contains(page, "@world") {
		t.Error("Missing @world in PACKAGE SETS section")
	}
}

func TestManPageGenerator_GenerateCommand(t *testing.T) {
	gen := NewManPageGeneratorWithDate("0.9.1", "January 2026")

	tests := []struct {
		name           string
		cmdName        string
		wantNonEmpty   bool
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "emerge command",
			cmdName:      "emerge",
			wantNonEmpty: true,
			wantContains: []string{
				".TH GRPM-EMERGE 1", // Header uses literal dash
				".SH NAME",
				".SH SYNOPSIS",
				".SH DESCRIPTION",
				".SH OPTIONS",
				".SH EXAMPLES",
				".SH SEE ALSO",
				"\\-\\-pretend",
				"\\-\\-repo",
				"build packages from source", // lowercase in NAME section
			},
		},
		{
			name:         "resolve command",
			cmdName:      "resolve",
			wantNonEmpty: true,
			wantContains: []string{
				".TH GRPM-RESOLVE 1",
				"resolve package dependencies", // lowercase in NAME section
				"\\-\\-autounmask",
			},
		},
		{
			name:         "remove command with aliases",
			cmdName:      "remove",
			wantNonEmpty: true,
			wantContains: []string{
				".SH ALIASES",
				"unmerge",
				"uninstall",
			},
		},
		{
			name:         "unknown command",
			cmdName:      "nonexistent",
			wantNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := gen.GenerateCommand(tt.cmdName)

			if tt.wantNonEmpty && page == "" {
				t.Error("Expected non-empty page")
				return
			}
			if !tt.wantNonEmpty && page != "" {
				t.Error("Expected empty page for unknown command")
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(page, want) {
					t.Errorf("Missing expected content: %s", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(page, notWant) {
					t.Errorf("Found unexpected content: %s", notWant)
				}
			}
		})
	}
}

func TestManPageGenerator_GenerateAll(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")
	pages := gen.GenerateAll()

	// Should have main page
	if _, ok := pages["grpm.1"]; !ok {
		t.Error("Missing grpm.1 (main page)")
	}

	// Should have command pages
	expectedCommands := []string{
		"emerge", "resolve", "install", "remove", "search",
		"info", "sync", "update", "build", "depclean",
		"fetch", "analyze", "tools", "completion", "doc",
	}

	for _, cmd := range expectedCommands {
		filename := "grpm-" + cmd + ".1"
		if _, ok := pages[filename]; !ok {
			t.Errorf("Missing %s", filename)
		}
	}

	// All pages should be non-empty
	for filename, content := range pages {
		if content == "" {
			t.Errorf("Page %s is empty", filename)
		}
	}
}

func TestManPageGenerator_CommandNames(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")
	names := gen.CommandNames()

	if len(names) == 0 {
		t.Error("CommandNames returned empty list")
	}

	// Check sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("Commands not sorted: %s came after %s", names[i], names[i-1])
		}
	}

	// Check expected commands exist
	expectedCommands := map[string]bool{
		"emerge":  false,
		"resolve": false,
		"install": false,
	}

	for _, name := range names {
		if _, ok := expectedCommands[name]; ok {
			expectedCommands[name] = true
		}
	}

	for cmd, found := range expectedCommands {
		if !found {
			t.Errorf("Expected command %s not found in list", cmd)
		}
	}
}

func TestEscapeManPage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with\\-dash"},
		{"with\\backslash", "with\\\\backslash"},
		{"multi-dashed-text", "multi\\-dashed\\-text"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeManPage(tt.input)
			if result != tt.expected {
				t.Errorf("escapeManPage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeDashes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pretend", "pretend"},
		{"dry-run", "dry\\-run"},
		{"autounmask-write", "autounmask\\-write"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeDashes(tt.input)
			if result != tt.expected {
				t.Errorf("escapeDashes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatSynopsis(t *testing.T) {
	tests := []struct {
		input    string
		contains []string
	}{
		{
			input:    "emerge [flags] <package>...",
			contains: []string{"[\\fIflags\\fR]", "<\\fIpackage\\fR>", "\\&..."},
		},
		{
			input:    "search [flags] <pattern>",
			contains: []string{"<\\fIpattern\\fR>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatSynopsis(tt.input)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("formatSynopsis(%q) missing %q, got %q", tt.input, want, result)
				}
			}
		})
	}
}

func TestManPageGenerator_FormatFlag(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")

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
			contains: []string{".TP", "\\-p", "\\-\\-pretend", "Show what would be done"},
		},
		{
			name: "string flag with default",
			flag: FlagMeta{
				Long:        "repo",
				Type:        "string",
				Default:     "/var/db/repos/gentoo",
				Description: "Path to repository",
			},
			contains: []string{"\\-\\-repo", "\\fIstring\\fR", "Default:", "/var/db/repos/gentoo"},
		},
		{
			name: "int flag",
			flag: FlagMeta{
				Short:       "j",
				Long:        "jobs",
				Type:        "int",
				Default:     "4",
				Description: "Parallel jobs",
			},
			contains: []string{"\\-j", "\\-\\-jobs", "\\fIint\\fR", "Default: \\fI4\\fR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.formatFlag(tt.flag)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("formatFlag() missing %q in:\n%s", want, result)
				}
			}
		})
	}
}

func TestManPageGenerator_ValidTroffOutput(t *testing.T) {
	gen := NewManPageGeneratorWithDate("0.9.1", "January 2026")
	page := gen.GenerateMain()

	// Check that the output starts with a valid .TH macro
	if !strings.HasPrefix(page, ".TH ") {
		t.Error("Man page should start with .TH macro")
	}

	// Check that sections are properly formatted
	lines := strings.Split(page, "\n")
	for i, line := range lines {
		// .SH should be followed by section name on same line
		if strings.HasPrefix(line, ".SH") && len(line) < 5 {
			t.Errorf("Line %d: .SH macro should have section name on same line", i+1)
		}
		// .TP should not have content on the same line
		if strings.HasPrefix(line, ".TP ") && len(line) > 4 {
			t.Errorf("Line %d: .TP macro should not have content on same line", i+1)
		}
	}
}

func TestGetManPageInstallInstructions(t *testing.T) {
	instructions := GetManPageInstallInstructions()

	if instructions == "" {
		t.Error("Install instructions should not be empty")
	}

	expectedPhrases := []string{
		"Generate all man pages",
		"Generate specific man page",
		"Install to system",
		"man -l -",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(instructions, phrase) {
			t.Errorf("Missing phrase in instructions: %s", phrase)
		}
	}
}

// TestManPageGenerator_HiddenFlags ensures hidden flags are not shown as separate entries in man pages.
func TestManPageGenerator_HiddenFlags(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")

	// The resolve command has a hidden --dry-run flag
	page := gen.GenerateCommand("resolve")

	// Should contain visible flags
	if !strings.Contains(page, "\\-\\-pretend") {
		t.Error("Missing visible --pretend flag")
	}

	// The hidden --dry-run alias should NOT appear as its own .TP entry in OPTIONS
	// (Note: "dry-run" may appear in descriptions like "Show what would be done (dry-run)")
	// Check the OPTIONS section specifically
	optionsStart := strings.Index(page, ".SH OPTIONS")
	if optionsStart == -1 {
		t.Fatal("Missing OPTIONS section")
	}

	// Find next section after OPTIONS
	nextSection := strings.Index(page[optionsStart+1:], ".SH ")
	var optionsSection string
	if nextSection == -1 {
		optionsSection = page[optionsStart:]
	} else {
		optionsSection = page[optionsStart : optionsStart+1+nextSection]
	}

	// The hidden --dry-run flag should not appear as a .BR flag definition
	// Pattern for a flag definition: ".BR \-\-dry\-run"
	if strings.Contains(optionsSection, ".BR \\-\\-dry\\-run") {
		t.Error("Hidden --dry-run flag should not appear as .BR entry in OPTIONS section")
	}

	// Also check that it doesn't appear as a standalone "\\-\\-dry\\-run" flag
	// (which would indicate a flag entry, not a description mention)
	// In troff, flag entries start with .TP then .BR
	lines := strings.Split(optionsSection, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, ".BR") && strings.Contains(line, "dry\\-run") {
			t.Errorf("Line %d: Hidden --dry-run appears as flag entry: %s", i, line)
		}
	}
}

// TestAllCommandsHaveManPages ensures every registered command can generate a man page.
func TestAllCommandsHaveManPages(t *testing.T) {
	gen := NewManPageGenerator("0.9.1")
	registry := NewCommandRegistry()

	for _, cmd := range registry.All() {
		t.Run(cmd.Name, func(t *testing.T) {
			page := gen.GenerateCommand(cmd.Name)
			if page == "" {
				t.Errorf("Command %s has no man page", cmd.Name)
				return
			}

			// Check basic structure
			if !strings.Contains(page, ".SH NAME") {
				t.Errorf("Command %s man page missing NAME section", cmd.Name)
			}
			if !strings.Contains(page, ".SH SYNOPSIS") {
				t.Errorf("Command %s man page missing SYNOPSIS section", cmd.Name)
			}
			if !strings.Contains(page, ".SH DESCRIPTION") {
				t.Errorf("Command %s man page missing DESCRIPTION section", cmd.Name)
			}
		})
	}
}
