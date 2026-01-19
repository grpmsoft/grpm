package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompletionGenerator_GenerateBash(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateBash()

	// Test required elements are present
	tests := []struct {
		name     string
		contains string
	}{
		{"header", "# GRPM bash completion script"},
		{"function definition", "_grpm()"},
		{"completion registration", "complete -F _grpm grpm"},
		{"emerge command", "emerge"},
		{"resolve command", "resolve"},
		{"install command", "install"},
		{"pretend flag", "--pretend"},
		{"global help flag", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(script, tt.contains) {
				t.Errorf("bash completion script missing %s: expected to contain %q", tt.name, tt.contains)
			}
		})
	}
}

func TestCompletionGenerator_GenerateZsh(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateZsh()

	// Test required elements are present
	tests := []struct {
		name     string
		contains string
	}{
		{"compdef header", "#compdef grpm"},
		{"main function", "_grpm()"},
		{"commands function", "_grpm_commands()"},
		{"emerge function", "_grpm_emerge()"},
		{"resolve function", "_grpm_resolve()"},
		{"help flag", "--help"},
		{"version flag", "--version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(script, tt.contains) {
				t.Errorf("zsh completion script missing %s: expected to contain %q", tt.name, tt.contains)
			}
		})
	}
}

func TestCompletionGenerator_GenerateFish(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateFish()

	// Test required elements are present
	tests := []struct {
		name     string
		contains string
	}{
		{"header", "# GRPM fish completion script"},
		{"disable file completion", "complete -c grpm -f"},
		{"help flag", "-l help"},
		{"version flag", "-l version"},
		{"emerge command", "-a 'emerge'"},
		{"resolve command", "-a 'resolve'"},
		{"install command", "-a 'install'"},
		{"subcommand condition", "__fish_use_subcommand"},
		{"seen subcommand condition", "__fish_seen_subcommand_from"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(script, tt.contains) {
				t.Errorf("fish completion script missing %s: expected to contain %q", tt.name, tt.contains)
			}
		})
	}
}

func TestCompletionGenerator_AllCommandsIncluded(t *testing.T) {
	gen := NewCompletionGenerator()
	registry := NewCommandRegistry()
	commands := registry.All()

	// Test bash includes all commands
	t.Run("bash", func(t *testing.T) {
		script := gen.GenerateBash()
		for _, cmd := range commands {
			if !strings.Contains(script, cmd.Name) {
				t.Errorf("bash completion missing command: %s", cmd.Name)
			}
		}
	})

	// Test zsh includes all commands
	t.Run("zsh", func(t *testing.T) {
		script := gen.GenerateZsh()
		for _, cmd := range commands {
			if !strings.Contains(script, cmd.Name) {
				t.Errorf("zsh completion missing command: %s", cmd.Name)
			}
		}
	})

	// Test fish includes all commands
	t.Run("fish", func(t *testing.T) {
		script := gen.GenerateFish()
		for _, cmd := range commands {
			if !strings.Contains(script, cmd.Name) {
				t.Errorf("fish completion missing command: %s", cmd.Name)
			}
		}
	})
}

func TestCompletionGenerator_CommandFlagsIncluded(t *testing.T) {
	gen := NewCompletionGenerator()
	registry := NewCommandRegistry()

	// Test that emerge command flags are included
	emergeMeta := registry.Get("emerge")
	if emergeMeta == nil {
		t.Fatal("emerge command not found in registry")
	}

	bashScript := gen.GenerateBash()
	zshScript := gen.GenerateZsh()
	fishScript := gen.GenerateFish()

	for _, f := range emergeMeta.Flags {
		if f.Hidden {
			continue
		}

		t.Run("bash/"+f.Long, func(t *testing.T) {
			if !strings.Contains(bashScript, "--"+f.Long) {
				t.Errorf("bash completion missing flag --%s for emerge", f.Long)
			}
		})

		t.Run("zsh/"+f.Long, func(t *testing.T) {
			if !strings.Contains(zshScript, f.Long) {
				t.Errorf("zsh completion missing flag --%s for emerge", f.Long)
			}
		})

		t.Run("fish/"+f.Long, func(t *testing.T) {
			if !strings.Contains(fishScript, "-l "+f.Long) {
				t.Errorf("fish completion missing flag --%s for emerge", f.Long)
			}
		})
	}
}

func TestCompletionGenerator_AliasesIncluded(t *testing.T) {
	gen := NewCompletionGenerator()

	// The remove command has aliases: uninstall, unmerge
	bashScript := gen.GenerateBash()
	zshScript := gen.GenerateZsh()
	fishScript := gen.GenerateFish()

	aliases := []string{"uninstall", "unmerge"}

	for _, alias := range aliases {
		t.Run("bash/"+alias, func(t *testing.T) {
			if !strings.Contains(bashScript, alias) {
				t.Errorf("bash completion missing alias: %s", alias)
			}
		})

		t.Run("zsh/"+alias, func(t *testing.T) {
			if !strings.Contains(zshScript, alias) {
				t.Errorf("zsh completion missing alias: %s", alias)
			}
		})

		t.Run("fish/"+alias, func(t *testing.T) {
			if !strings.Contains(fishScript, alias) {
				t.Errorf("fish completion missing alias: %s", alias)
			}
		})
	}
}

func TestCompletionGenerator_HiddenFlagsExcluded(t *testing.T) {
	gen := NewCompletionGenerator()
	registry := NewCommandRegistry()

	// The resolve command has a hidden --dry-run flag
	resolveMeta := registry.Get("resolve")
	if resolveMeta == nil {
		t.Fatal("resolve command not found in registry")
	}

	// Find a hidden flag
	var hiddenFlag *FlagMeta
	for _, f := range resolveMeta.Flags {
		if f.Hidden {
			hiddenFlag = &f
			break
		}
	}

	if hiddenFlag == nil {
		t.Skip("no hidden flags found in resolve command")
	}

	fishScript := gen.GenerateFish()

	// Hidden flag should not appear in fish completion with -l prefix
	// (it may appear in case statements for bash/zsh but not as completable)
	if strings.Contains(fishScript, fmt.Sprintf("-l %s -d", hiddenFlag.Long)) {
		t.Errorf("fish completion includes hidden flag --%s", hiddenFlag.Long)
	}
}

func TestGetInstallInstructions(t *testing.T) {
	instructions := GetInstallInstructions()

	// Test all shells are covered
	shells := []string{"Bash:", "Zsh:", "Fish:"}
	for _, shell := range shells {
		if !strings.Contains(instructions, shell) {
			t.Errorf("install instructions missing %s", shell)
		}
	}

	// Test paths are included
	paths := []string{
		"/etc/bash_completion.d/grpm",
		"~/.zsh/completions/_grpm",
		"~/.config/fish/completions/grpm.fish",
	}
	for _, path := range paths {
		if !strings.Contains(instructions, path) {
			t.Errorf("install instructions missing path: %s", path)
		}
	}
}

func TestBashSafeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"emerge", "emerge"},
		{"dep-clean", "dep_clean"},
		{"some-long-name", "some_long_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := bashSafeName(tt.input)
			if result != tt.expected {
				t.Errorf("bashSafeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestZshSafeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"emerge", "emerge"},
		{"dep-clean", "dep_clean"},
		{"some-long-name", "some_long_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := zshSafeName(tt.input)
			if result != tt.expected {
				t.Errorf("zshSafeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBashCompletionValid performs basic validation of bash completion syntax.
func TestBashCompletionValid(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateBash()

	// Check balanced braces (structural check)
	openBraces := strings.Count(script, "{")
	closeBraces := strings.Count(script, "}")
	if openBraces != closeBraces {
		t.Errorf("unbalanced braces in bash completion: open=%d, close=%d", openBraces, closeBraces)
	}

	// Note: Parentheses check removed - bash completion syntax includes
	// unbalanced parentheses in case patterns like "emerge|resolve)"

	// Check required bash completion elements
	requiredElements := []string{
		"_init_completion",
		"COMPREPLY",
		"compgen -W",
		"complete -F",
	}
	for _, elem := range requiredElements {
		if !strings.Contains(script, elem) {
			t.Errorf("bash completion missing required element: %s", elem)
		}
	}
}

// TestZshCompletionValid performs basic validation of zsh completion syntax.
func TestZshCompletionValid(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateZsh()

	// Check balanced braces (structural check)
	openBraces := strings.Count(script, "{")
	closeBraces := strings.Count(script, "}")
	if openBraces != closeBraces {
		t.Errorf("unbalanced braces in zsh completion: open=%d, close=%d", openBraces, closeBraces)
	}

	// Note: Parentheses check removed - zsh completion syntax includes
	// unbalanced parentheses in constructs like '(-p --pretend)'{-p,--pretend}'

	// Check required zsh completion elements
	requiredElements := []string{
		"#compdef",
		"_arguments",
		"_describe",
		"typeset -A",
	}
	for _, elem := range requiredElements {
		if !strings.Contains(script, elem) {
			t.Errorf("zsh completion missing required element: %s", elem)
		}
	}
}

// TestFishCompletionValid performs basic validation of fish completion syntax.
func TestFishCompletionValid(t *testing.T) {
	gen := NewCompletionGenerator()
	script := gen.GenerateFish()

	// All lines starting with "complete" should be valid fish syntax
	lines := strings.Split(script, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "complete") {
			// Check that -c grpm is present
			if !strings.Contains(line, "-c grpm") {
				t.Errorf("line %d: fish completion line missing '-c grpm': %s", i+1, line)
			}
		}
	}

	// Check for common fish completion patterns
	requiredPatterns := []string{
		"complete -c grpm",
		"-d '", // description flag
		"__fish_use_subcommand",
	}
	for _, pattern := range requiredPatterns {
		if !strings.Contains(script, pattern) {
			t.Errorf("fish completion missing required pattern: %s", pattern)
		}
	}
}
