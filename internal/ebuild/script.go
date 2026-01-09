// Package ebuild implements ebuild execution engine.
//
// This file provides ebuild script parsing to extract defined functions.
// Used for phase dispatch to determine whether custom or default phases are used.
package ebuild

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// EbuildScript represents a parsed ebuild file.
//
// Contains metadata extracted from the ebuild including defined functions,
// inherited eclasses, and EAPI version.
type EbuildScript struct {
	// Path to the ebuild file
	Path string

	// DefinedFunctions contains all function names defined in the ebuild
	DefinedFunctions map[string]bool

	// InheritedEclasses lists eclasses inherited by this ebuild
	InheritedEclasses []string

	// EAPI version extracted from the ebuild
	EAPI string

	// AST is the parsed syntax tree (optional, for advanced use)
	AST *syntax.File
}

// ParseEbuildScript parses an ebuild file and extracts defined functions.
//
// This is a lightweight parser that identifies function definitions without
// fully executing the ebuild. Used for phase dispatch decisions.
//
// Returns EbuildScript with function information or error.
func ParseEbuildScript(path string) (*EbuildScript, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening ebuild %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	script := &EbuildScript{
		Path:              path,
		DefinedFunctions:  make(map[string]bool),
		InheritedEclasses: make([]string, 0),
		EAPI:              "0", // Default EAPI
	}

	// Parse using mvdan.cc/sh parser for accurate bash parsing
	parser := syntax.NewParser(syntax.KeepComments(false))
	ast, err := parser.Parse(file, path)
	if err != nil {
		return nil, fmt.Errorf("parsing ebuild %s: %w", path, err)
	}

	script.AST = ast

	// Extract functions and metadata from AST
	syntax.Walk(ast, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.FuncDecl:
			// Found a function declaration
			script.DefinedFunctions[n.Name.Value] = true
		case *syntax.CallExpr:
			// Look for inherit calls and EAPI assignments
			if len(n.Args) > 0 {
				if lit := getFirstWordLit(n.Args[0]); lit != "" {
					if lit == "inherit" && len(n.Args) > 1 {
						// Collect inherited eclasses
						for _, arg := range n.Args[1:] {
							if eclassName := getFirstWordLit(arg); eclassName != "" {
								script.InheritedEclasses = append(script.InheritedEclasses, eclassName)
							}
						}
					}
				}
			}
		case *syntax.Assign:
			// Look for EAPI assignment
			if n.Name != nil && n.Name.Value == "EAPI" {
				if n.Value != nil {
					if word := getWordValue(n.Value); word != "" {
						script.EAPI = strings.Trim(word, "\"'")
					}
				}
			}
		}
		return true
	})

	return script, nil
}

// ParseEbuildScriptFromString parses ebuild content from a string.
//
// Useful for testing and when ebuild content is already in memory.
func ParseEbuildScriptFromString(content string) (*EbuildScript, error) {
	script := &EbuildScript{
		Path:              "(string)",
		DefinedFunctions:  make(map[string]bool),
		InheritedEclasses: make([]string, 0),
		EAPI:              "0",
	}

	// Parse using mvdan.cc/sh parser
	parser := syntax.NewParser(syntax.KeepComments(false))
	ast, err := parser.Parse(strings.NewReader(content), "ebuild")
	if err != nil {
		return nil, fmt.Errorf("parsing ebuild content: %w", err)
	}

	script.AST = ast

	// Extract functions and metadata from AST
	syntax.Walk(ast, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.FuncDecl:
			script.DefinedFunctions[n.Name.Value] = true
		case *syntax.CallExpr:
			if len(n.Args) > 0 {
				if lit := getFirstWordLit(n.Args[0]); lit != "" {
					if lit == "inherit" && len(n.Args) > 1 {
						for _, arg := range n.Args[1:] {
							if eclassName := getFirstWordLit(arg); eclassName != "" {
								script.InheritedEclasses = append(script.InheritedEclasses, eclassName)
							}
						}
					}
				}
			}
		case *syntax.Assign:
			if n.Name != nil && n.Name.Value == "EAPI" {
				if n.Value != nil {
					if word := getWordValue(n.Value); word != "" {
						script.EAPI = strings.Trim(word, "\"'")
					}
				}
			}
		}
		return true
	})

	return script, nil
}

// HasFunction checks if a function is defined in the ebuild.
func (s *EbuildScript) HasFunction(name string) bool {
	return s.DefinedFunctions[name]
}

// HasPhaseFunction checks if a phase function is defined.
//
// Phase functions follow the naming convention: src_unpack, src_prepare, etc.
func (s *EbuildScript) HasPhaseFunction(phase Phase) bool {
	funcName := phaseFunctionName(phase)
	return s.DefinedFunctions[funcName]
}

// phaseFunctionName returns the bash function name for a phase.
// Per PMS Chapter 9, ebuild phase function names follow these conventions:
// - src_* phases: operate on sources (unpack, prepare, configure, compile, test, install)
// - pkg_* phases: operate on packages (setup, preinst, postinst, prerm, postrm, config, info, nofetch, pretend)
func phaseFunctionName(phase Phase) string {
	switch phase {
	case PhasePretend:
		return "pkg_pretend"
	case PhaseSetup:
		return "pkg_setup"
	case PhaseUnpack:
		return "src_unpack"
	case PhasePrepare:
		return "src_prepare"
	case PhaseConfigure:
		return "src_configure"
	case PhaseCompile:
		return "src_compile"
	case PhaseTest:
		return "src_test"
	case PhaseInstall:
		return "src_install"
	case PhasePreinst:
		return "pkg_preinst"
	case PhasePostinst:
		return "pkg_postinst"
	case PhasePrerem:
		return "pkg_prerm"
	case PhasePostrm:
		return "pkg_postrm"
	case PhaseConfig:
		return "pkg_config"
	case PhaseInfo:
		return "pkg_info"
	case PhaseNofetch:
		return "pkg_nofetch"
	default:
		return ""
	}
}

// getFirstWordLit extracts the first literal from a Word node.
func getFirstWordLit(word *syntax.Word) string {
	if word == nil || len(word.Parts) == 0 {
		return ""
	}
	if lit, ok := word.Parts[0].(*syntax.Lit); ok {
		return lit.Value
	}
	return ""
}

// getWordValue extracts the full value from a Word node.
func getWordValue(word *syntax.Word) string {
	if word == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, dpart := range p.Parts {
				if lit, ok := dpart.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		}
	}
	return sb.String()
}

// FindDefinedPhases returns a list of phases that have custom functions defined.
//
// This is similar to Portage's DEFINED_PHASES metadata.
// Per PMS Chapter 9: Includes all phase functions that an ebuild may define.
func (s *EbuildScript) FindDefinedPhases() []Phase {
	defined := make([]Phase, 0)
	// All possible phases per PMS Chapter 9
	phases := []Phase{
		PhasePretend, // EAPI 4+
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
		PhasePreinst,
		PhasePostinst,
		PhasePrerem,
		PhasePostrm,
		PhaseConfig,
		PhaseInfo,
		PhaseNofetch,
	}

	for _, phase := range phases {
		if s.HasPhaseFunction(phase) {
			defined = append(defined, phase)
		}
	}

	return defined
}

// QuickParseFunctions does a fast regex-based scan for function definitions.
//
// This is faster than full AST parsing but less accurate.
// Use when only function names are needed without full parsing.
func QuickParseFunctions(path string) (map[string]bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	functions := make(map[string]bool)

	// Regex patterns for function definitions
	// Matches: func_name() { or function func_name {
	funcPattern := regexp.MustCompile(`^\s*(?:function\s+)?([a-zA-Z_][a-zA-Z0-9_-]*)\s*\(\s*\)\s*\{?`)
	funcKeywordPattern := regexp.MustCompile(`^\s*function\s+([a-zA-Z_][a-zA-Z0-9_-]*)\s*(?:\(\s*\))?\s*\{?`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for function() { pattern
		if matches := funcPattern.FindStringSubmatch(line); len(matches) > 1 {
			functions[matches[1]] = true
			continue
		}

		// Check for function keyword pattern
		if matches := funcKeywordPattern.FindStringSubmatch(line); len(matches) > 1 {
			functions[matches[1]] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file %s: %w", path, err)
	}

	return functions, nil
}
