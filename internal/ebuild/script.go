// Package ebuild implements ebuild execution engine.
//
// This file provides ebuild script parsing to extract defined functions.
// Used for phase dispatch to determine whether custom or default phases are used.
package ebuild

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
// The vars parameter provides known variables (PV, PN, P, etc.) for evaluating
// simple conditionals. When a conditional like [[ ${PV} == 9999 ]] can be
// statically evaluated, branches that are definitely false are skipped.
// Pass nil to collect all inherit calls unconditionally (conservative mode).
//
// Returns EbuildScript with function information or error.
func ParseEbuildScript(path string, vars map[string]string) (*EbuildScript, error) {
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
	// CRITICAL: Must use LangBash variant for ebuild compatibility
	// Ebuilds use bash-specific syntax (local -a, arrays, etc.)
	parser := syntax.NewParser(
		syntax.KeepComments(false),
		syntax.Variant(syntax.LangBash),
	)
	ast, err := parser.Parse(file, path)
	if err != nil {
		return nil, fmt.Errorf("parsing ebuild %s: %w", path, err)
	}

	script.AST = ast

	// If no vars provided, try to infer PV from filename.
	// Ebuild filenames follow the pattern: <name>-<version>.ebuild
	if vars == nil {
		vars = make(map[string]string)
	}
	if _, ok := vars["PV"]; !ok {
		if pv := pvFromPath(path); pv != "" {
			vars["PV"] = pv
		}
	}

	// Extract functions and metadata from AST using condition-aware walker
	walkStmts(ast.Stmts, script, vars)

	return script, nil
}

// ParseEbuildScriptFromString parses ebuild content from a string.
//
// Useful for testing and when ebuild content is already in memory.
// Pass vars to enable conditional evaluation, or nil for conservative mode.
func ParseEbuildScriptFromString(content string, vars map[string]string) (*EbuildScript, error) {
	script := &EbuildScript{
		Path:              "(string)",
		DefinedFunctions:  make(map[string]bool),
		InheritedEclasses: make([]string, 0),
		EAPI:              "0",
	}

	// Parse using mvdan.cc/sh parser with bash variant
	parser := syntax.NewParser(
		syntax.KeepComments(false),
		syntax.Variant(syntax.LangBash),
	)
	ast, err := parser.Parse(strings.NewReader(content), "ebuild")
	if err != nil {
		return nil, fmt.Errorf("parsing ebuild content: %w", err)
	}

	script.AST = ast

	if vars == nil {
		vars = make(map[string]string)
	}

	// Extract functions and metadata from AST using condition-aware walker
	walkStmts(ast.Stmts, script, vars)

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

// walkStmts walks a list of statements, extracting metadata into script.
// It evaluates simple if-conditions using vars to skip dead branches.
func walkStmts(stmts []*syntax.Stmt, script *EbuildScript, vars map[string]string) {
	for _, stmt := range stmts {
		walkNode(stmt, script, vars)
	}
}

// walkNode processes a single AST node, recursing into children as needed.
func walkNode(node syntax.Node, script *EbuildScript, vars map[string]string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *syntax.Stmt:
		walkNode(n.Cmd, script, vars)
	case *syntax.FuncDecl:
		script.DefinedFunctions[n.Name.Value] = true
		// Don't recurse into function bodies for inherit extraction.
		// Inherit calls inside functions are runtime, not load-time.
	case *syntax.CallExpr:
		walkCallExpr(n, script)
	case *syntax.IfClause:
		walkIfClause(n, script, vars)
	case *syntax.Block:
		walkStmts(n.Stmts, script, vars)
	case *syntax.Subshell:
		walkStmts(n.Stmts, script, vars)
	case *syntax.BinaryCmd:
		walkNode(n.X, script, vars)
		walkNode(n.Y, script, vars)
	case *syntax.WhileClause:
		walkStmts(n.Cond, script, vars)
		walkStmts(n.Do, script, vars)
	case *syntax.ForClause:
		walkStmts(n.Do, script, vars)
	case *syntax.CaseClause:
		for _, ci := range n.Items {
			walkStmts(ci.Stmts, script, vars)
		}
	}
}

// walkCallExpr extracts metadata from a call expression (command or assignment).
func walkCallExpr(n *syntax.CallExpr, script *EbuildScript) {
	// Check assigns (e.g., EAPI=8 as a bare assignment)
	for _, a := range n.Assigns {
		if a.Name != nil && a.Name.Value == "EAPI" {
			if a.Value != nil {
				if word := getWordValue(a.Value); word != "" {
					script.EAPI = strings.Trim(word, "\"'")
				}
			}
		}
	}
	// Check command (e.g., inherit cmake)
	if len(n.Args) > 0 {
		if lit := getFirstWordLit(n.Args[0]); lit == "inherit" && len(n.Args) > 1 {
			for _, arg := range n.Args[1:] {
				if eclassName := getFirstWordLit(arg); eclassName != "" {
					script.InheritedEclasses = append(script.InheritedEclasses, eclassName)
				}
			}
		}
	}
}

// walkIfClause processes an if/elif/else chain with conditional evaluation.
//
// For simple conditions like [[ ${PV} == 9999 ]], if the variable is known
// and doesn't match, the branch body is skipped entirely. This prevents
// loading eclasses that are behind version-specific conditionals.
func walkIfClause(ic *syntax.IfClause, script *EbuildScript, vars map[string]string) {
	if ic == nil {
		return
	}

	// Try to evaluate the condition statically.
	result := evalCondition(ic.Cond, vars)

	switch result {
	case condTrue:
		// Condition is definitely true — walk this branch only.
		walkStmts(ic.Then, script, vars)
		// Skip else/elif entirely.
	case condFalse:
		// Condition is definitely false — skip this branch, try else/elif.
		if ic.Else != nil {
			if ic.Else.Cond != nil {
				// This is an "elif" — evaluate its condition.
				walkIfClause(ic.Else, script, vars)
			} else {
				// This is an "else" — walk its body.
				walkStmts(ic.Else.Then, script, vars)
			}
		}
	default:
		// Can't evaluate — conservatively walk ALL branches.
		walkStmts(ic.Then, script, vars)
		if ic.Else != nil {
			walkIfClause(ic.Else, script, vars)
		}
	}
}

// condResult represents the result of static condition evaluation.
type condResult int

const (
	condUnknown condResult = iota
	condTrue
	condFalse
)

// evalCondition tries to statically evaluate an if-clause condition.
//
// Supports:
//   - [[ ${VAR} == literal ]] and [[ ${VAR} != literal ]]
//   - Glob patterns in the literal (e.g., [[ ${PV} == *_p* ]])
//   - Simple string equality (e.g., [[ ${PV} == 9999 ]])
func evalCondition(cond []*syntax.Stmt, vars map[string]string) condResult {
	if len(vars) == 0 || len(cond) != 1 {
		return condUnknown
	}
	stmt := cond[0]
	if stmt.Cmd == nil {
		return condUnknown
	}

	tc, ok := stmt.Cmd.(*syntax.TestClause)
	if !ok {
		return condUnknown
	}

	bt, ok := tc.X.(*syntax.BinaryTest)
	if !ok {
		return condUnknown
	}

	// Only handle == and !=
	var negate bool
	switch bt.Op {
	case syntax.TsMatch, syntax.TsMatchShort:
		negate = false
	case syntax.TsNoMatch:
		negate = true
	default:
		return condUnknown
	}

	// Left side: expect ${VAR} (a Word containing a single ParamExp)
	varName := extractSimpleVar(bt.X)
	if varName == "" {
		return condUnknown
	}
	val, ok := vars[varName]
	if !ok {
		return condUnknown
	}

	// Right side: expect a literal or simple glob pattern
	pat := extractTestLiteral(bt.Y)
	if pat == "" {
		return condUnknown
	}

	// Match: in [[ ]], == does glob matching
	matched := matchGlob(pat, val)

	if negate {
		matched = !matched
	}

	if matched {
		return condTrue
	}
	return condFalse
}

// extractSimpleVar extracts the variable name from a simple ${VAR} expression.
// Returns "" if the expression is not a simple parameter expansion.
func extractSimpleVar(expr syntax.TestExpr) string {
	word, ok := expr.(*syntax.Word)
	if !ok || len(word.Parts) != 1 {
		return ""
	}
	pe, ok := word.Parts[0].(*syntax.ParamExp)
	if !ok || pe.Param == nil {
		return ""
	}
	// Only simple ${VAR}, not ${VAR:-default} or ${VAR%pattern} etc.
	if pe.Exp != nil || pe.Slice != nil || pe.Repl != nil || pe.Excl || pe.Length {
		return ""
	}
	return pe.Param.Value
}

// extractTestLiteral extracts a literal string from a test expression.
// Handles both unquoted literals and double-quoted strings containing only literals.
func extractTestLiteral(expr syntax.TestExpr) string {
	word, ok := expr.(*syntax.Word)
	if !ok || len(word.Parts) == 0 {
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
			for _, dp := range p.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				} else {
					return "" // contains variable expansion
				}
			}
		default:
			return "" // contains non-literal parts
		}
	}
	return sb.String()
}

// matchGlob performs simple glob matching (supports * and ? wildcards).
func matchGlob(pattern, value string) bool {
	// filepath.Match handles *, ?, and [...] patterns
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return pattern == value // fall back to exact match on invalid pattern
	}
	return matched
}

// pvFromPath extracts the package version from an ebuild filename.
//
// Ebuild filenames follow: <name>-<version>.ebuild
// For example: make-4.4.1-r102.ebuild → PV=4.4.1, PVR=4.4.1-r102
func pvFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".ebuild")
	if base == "" {
		return ""
	}
	// Find the last hyphen followed by a digit — that's where the version starts
	for i := len(base) - 1; i > 0; i-- {
		if base[i-1] == '-' && i < len(base) && base[i] >= '0' && base[i] <= '9' {
			vr := base[i:]
			// Strip -rN revision suffix for PV
			if idx := strings.LastIndex(vr, "-r"); idx >= 0 {
				// Verify it's actually a revision (digits after -r)
				rev := vr[idx+2:]
				allDigits := true
				for _, c := range rev {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits && len(rev) > 0 {
					return vr[:idx]
				}
			}
			return vr
		}
	}
	return ""
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
