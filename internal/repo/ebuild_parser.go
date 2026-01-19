package repo

import (
	"fmt"
	"strings"

	"github.com/coregx/coregex"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Precompiled regex patterns for ebuild parsing (compiled once at package init).
var (
	// ebuildVarRe matches VAR="value" patterns in ebuild files.
	// Pattern: ^VARNAME="value" (single line)
	ebuildVarRe = coregex.MustCompile(`(?m)^([A-Z_][A-Z0-9_]*)="([^"]*(?:\\"[^"]*)*)"`)

	// ebuildMultiLineVarRe matches multi-line variable assignments.
	// Pattern: VARNAME="line1\nline2\nline3"
	ebuildMultiLineVarRe = coregex.MustCompile(`(?s)^([A-Z_][A-Z0-9_]*)="(.*?)"`)

	// ebuildVarRefRe matches variable references: ${VAR} or ${VAR:-default}
	ebuildVarRefRe = coregex.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)(?::-([^}]*))?\}`)

	// ebuildAtomVersionRe splits "category/package-version" into name and version.
	// Example: "sys-libs/zlib-1.2.13" -> ("sys-libs/zlib", "1.2.13")
	ebuildAtomVersionRe = coregex.MustCompile(`^(.+?)-(\d.*)$`)
)

// DependencyType defines the type of dependency
type DependencyType int

const (
	DepTypeRuntime   DependencyType = iota // RDEPEND
	DepTypeBuild                           // DEPEND
	DepTypeBuildtime                       // BDEPEND
	DepTypeInstall                         // IDEPEND (EAPI 8)
	DepTypePostMerge                       // PDEPEND
)

// ParsedDependency represents a parsed dependency with all metadata
type ParsedDependency struct {
	Atom        *pkg.Atom      // PMS-compliant atom (preferred)
	Constraint  pkg.Constraint // Legacy constraint (for backwards compatibility)
	UseFlag     string         // USE flag condition (e.g., "ssl" for "ssl? ( ... )")
	IsBlocker   bool           // true if starts with !
	IsHardBlock bool           // true if starts with !!
	DepType     DependencyType
	OrGroupID   int // OR-group ID (0 = not in OR-group, >0 = OR alternative)
}

// EbuildVariables maps variable names to their values
type EbuildVariables map[string]string

// PackageMetadata contains package information for variable expansion.
// These correspond to Portage's package variables (PMS 11.1).
type PackageMetadata struct {
	Category string // CATEGORY (e.g., "app-misc")
	PN       string // Package name (e.g., "hello")
	PV       string // Package version (e.g., "1.0")
	PR       string // Package revision (e.g., "r0")
	PVR      string // Version with revision (e.g., "1.0-r0")
	PF       string // Full package-version-revision (e.g., "hello-1.0-r0")
	P        string // Package-version (e.g., "hello-1.0")
	A        string // All source files (SRC_URI filenames)
}

// NewPackageMetadata creates PackageMetadata from category, name, and version.
// Automatically derives P, PF, PVR from base values.
func NewPackageMetadata(category, name, version string) PackageMetadata {
	// Parse revision from version if present (e.g., "1.0-r1" -> "1.0", "r1")
	pv := version
	pr := "r0"
	if idx := strings.LastIndex(version, "-r"); idx != -1 {
		pv = version[:idx]
		pr = version[idx+1:]
	}

	pvr := pv
	if pr != "r0" {
		pvr = pv + "-" + pr
	}

	return PackageMetadata{
		Category: category,
		PN:       name,
		PV:       pv,
		PR:       pr,
		PVR:      pvr,
		PF:       name + "-" + pvr,
		P:        name + "-" + pv,
	}
}

// EbuildParser handles parsing of ebuild files
type EbuildParser struct {
	content       string
	variables     EbuildVariables // Cached extracted variables
	nextOrGroupID int             // Counter for OR-group IDs
	metadata      *PackageMetadata
}

// NewEbuildParser creates a new ebuild parser
func NewEbuildParser(content string) *EbuildParser {
	ep := &EbuildParser{
		content:       content,
		variables:     make(EbuildVariables),
		nextOrGroupID: 1, // Start from 1 (0 means not in OR-group)
	}
	// Extract all variables on initialization
	ep.extractAllVariables()
	return ep
}

// NewEbuildParserWithMetadata creates parser with package metadata for variable expansion.
// Package variables (P, PN, PV, etc.) will be available for ${VAR} expansion.
func NewEbuildParserWithMetadata(content string, meta PackageMetadata) *EbuildParser {
	ep := &EbuildParser{
		content:       content,
		variables:     make(EbuildVariables),
		nextOrGroupID: 1,
		metadata:      &meta,
	}
	// Pre-populate package variables
	ep.variables["CATEGORY"] = meta.Category
	ep.variables["PN"] = meta.PN
	ep.variables["PV"] = meta.PV
	ep.variables["PR"] = meta.PR
	ep.variables["PVR"] = meta.PVR
	ep.variables["PF"] = meta.PF
	ep.variables["P"] = meta.P
	if meta.A != "" {
		ep.variables["A"] = meta.A
	}

	// Extract all variables from content (may override package vars if redefined)
	ep.extractAllVariables()
	return ep
}

// ParseDependencies parses all dependency types from ebuild
func (ep *EbuildParser) ParseDependencies() ([]ParsedDependency, error) {
	var allDeps []ParsedDependency

	// Parse RDEPEND (runtime dependencies)
	rdepend := ep.ExtractVariable("RDEPEND")
	if rdepend != "" {
		deps, err := ep.parseDependencyString(rdepend, DepTypeRuntime)
		if err != nil {
			return nil, fmt.Errorf("parsing RDEPEND: %w", err)
		}
		allDeps = append(allDeps, deps...)
	}

	// Parse DEPEND (build dependencies)
	depend := ep.ExtractVariable("DEPEND")
	if depend != "" {
		deps, err := ep.parseDependencyString(depend, DepTypeBuild)
		if err != nil {
			return nil, fmt.Errorf("parsing DEPEND: %w", err)
		}
		allDeps = append(allDeps, deps...)
	}

	// Parse BDEPEND (build-time dependencies)
	bdepend := ep.ExtractVariable("BDEPEND")
	if bdepend != "" {
		deps, err := ep.parseDependencyString(bdepend, DepTypeBuildtime)
		if err != nil {
			return nil, fmt.Errorf("parsing BDEPEND: %w", err)
		}
		allDeps = append(allDeps, deps...)
	}

	// Parse IDEPEND (EAPI 8: install-time dependencies)
	// These are dependencies needed by pkg_* phases at install time
	idepend := ep.ExtractVariable("IDEPEND")
	if idepend != "" {
		deps, err := ep.parseDependencyString(idepend, DepTypeInstall)
		if err != nil {
			return nil, fmt.Errorf("parsing IDEPEND: %w", err)
		}
		allDeps = append(allDeps, deps...)
	}

	// Parse PDEPEND (post-merge dependencies)
	pdepend := ep.ExtractVariable("PDEPEND")
	if pdepend != "" {
		deps, err := ep.parseDependencyString(pdepend, DepTypePostMerge)
		if err != nil {
			return nil, fmt.Errorf("parsing PDEPEND: %w", err)
		}
		allDeps = append(allDeps, deps...)
	}

	return allDeps, nil
}

// extractAllVariables extracts all variables from ebuild content
// Populates ep.variables map with variable name -> raw value mappings
func (ep *EbuildParser) extractAllVariables() {
	// Find all variable assignments using precompiled regex
	matches := ebuildVarRe.FindAllStringSubmatch(ep.content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			varName := match[1]
			varValue := match[2]
			// Store raw value (will be expanded later)
			ep.variables[varName] = varValue
		}
	}

	// Handle multi-line variables with proper bash syntax
	// VAR="line1
	//      line2
	//      line3"
	multiMatches := ebuildMultiLineVarRe.FindAllStringSubmatch(ep.content, -1)
	for _, match := range multiMatches {
		if len(match) >= 3 {
			varName := match[1]
			// Only set if not already set (prefer first match)
			if _, exists := ep.variables[varName]; !exists {
				ep.variables[varName] = strings.TrimSpace(match[2])
			}
		}
	}
}

// expandVariables recursively expands ${VAR} references in a string
// Supports:
//   - ${VAR} → value of VAR
//   - ${VAR:-default} → value of VAR, or "default" if unset
//   - Recursive expansion: ${DEPEND} can contain ${COMMON_DEPEND}
//
// Protects against infinite recursion with depth limit
func (ep *EbuildParser) expandVariables(value string, depth int) string {
	const maxDepth = 10 // Prevent infinite recursion

	if depth > maxDepth {
		return value // Stop recursion
	}

	// Use precompiled regex for variable references
	expanded := ebuildVarRefRe.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name and default value
		submatches := ebuildVarRefRe.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match // Invalid syntax, return as-is
		}

		varName := submatches[1]
		defaultValue := ""
		if len(submatches) > 2 {
			defaultValue = submatches[2]
		}

		// Look up variable value
		if varValue, exists := ep.variables[varName]; exists {
			// Recursively expand the variable's value
			return ep.expandVariables(varValue, depth+1)
		}

		// Variable not found - use default if provided
		if defaultValue != "" {
			return ep.expandVariables(defaultValue, depth+1)
		}

		// No value and no default - return empty string
		return ""
	})

	return expanded
}

// ExtractVariable extracts a variable value from ebuild content with expansion.
// Uses the pre-extracted variables map and performs ${VAR} expansion.
func (ep *EbuildParser) ExtractVariable(varName string) string {
	// Get raw value from pre-extracted variables
	rawValue, exists := ep.variables[varName]
	if !exists {
		return ""
	}

	// Expand any ${VAR} references recursively
	expanded := ep.expandVariables(rawValue, 0)

	return strings.TrimSpace(expanded)
}

// ExtractProperties extracts and parses the PROPERTIES variable from ebuild.
// PROPERTIES values affect package manager behavior (PMS Section 7.2.7).
// Common values: interactive, live, test_network
func (ep *EbuildParser) ExtractProperties() []string {
	props := ep.ExtractVariable("PROPERTIES")
	if props == "" {
		return nil
	}
	return parseSpaceSeparatedList(props)
}

// ExtractRestrict extracts and parses the RESTRICT variable from ebuild.
// RESTRICT values limit package manager actions (PMS Section 7.2.8).
// Common values: mirror, fetch, strip, test, userpriv, network-sandbox
func (ep *EbuildParser) ExtractRestrict() []string {
	restrict := ep.ExtractVariable("RESTRICT")
	if restrict == "" {
		return nil
	}
	return parseSpaceSeparatedList(restrict)
}

// ExtractRequiredUse extracts the REQUIRED_USE variable from ebuild.
// REQUIRED_USE defines constraints on USE flag combinations (PMS Section 7.2.6).
// Example: "|| ( ssl gnutls )" means at least one of ssl or gnutls must be enabled.
func (ep *EbuildParser) ExtractRequiredUse() string {
	return ep.ExtractVariable("REQUIRED_USE")
}

// parseSpaceSeparatedList splits a space-separated string into a slice.
// Handles multiple whitespace and trims each element.
func parseSpaceSeparatedList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Split on whitespace
	fields := strings.Fields(s)
	result := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// parseDependencyString parses a dependency string (e.g., RDEPEND value)
func (ep *EbuildParser) parseDependencyString(depStr string, depType DependencyType) ([]ParsedDependency, error) {
	var deps []ParsedDependency

	// Tokenize the dependency string
	tokens := tokenizeDependencies(depStr)

	// Parse tokens (orGroupID = 0 means not in OR-group)
	parsedDeps, _, err := ep.parseTokens(tokens, 0, depType, "", 0)
	if err != nil {
		return nil, err
	}

	deps = append(deps, parsedDeps...)
	return deps, nil
}

// tokenizeDependencies splits dependency string into tokens.
// Handles USE dependency brackets [flag(+)] correctly - parentheses inside
// brackets are NOT treated as group delimiters.
//
// Example: "sys-fs/e2fsprogs[tools(+)]" stays as single token,
// while "gpm? ( sys-libs/gpm )" splits into ["gpm?", "(", "sys-libs/gpm", ")"]
func tokenizeDependencies(depStr string) []string {
	var tokens []string
	var current strings.Builder
	inBracket := false // Track if we're inside [...] USE dependency brackets

	for _, ch := range depStr {
		switch ch {
		case '[':
			inBracket = true
			current.WriteRune(ch)
		case ']':
			inBracket = false
			current.WriteRune(ch)
		case '(', ')':
			// Only treat as group delimiter if NOT inside [...]
			if inBracket {
				current.WriteRune(ch)
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, strings.TrimSpace(current.String()))
					current.Reset()
				}
				tokens = append(tokens, string(ch))
			}
		case ' ', '\t', '\n':
			// Only tokenize on whitespace if NOT inside [...]
			if inBracket {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				tokens = append(tokens, strings.TrimSpace(current.String()))
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, strings.TrimSpace(current.String()))
	}

	return tokens
}

// parseTokens recursively parses dependency tokens
func (ep *EbuildParser) parseTokens(tokens []string, start int, depType DependencyType, useFlag string, orGroupID int) ([]ParsedDependency, int, error) {
	var deps []ParsedDependency
	i := start

	for i < len(tokens) {
		token := tokens[i]

		// Handle closing parenthesis - end of group
		if token == ")" {
			return deps, i + 1, nil
		}

		// Handle opening parenthesis - start of group
		if token == "(" {
			groupDeps, nextIdx, err := ep.parseGroup(tokens, i, depType, useFlag, orGroupID)
			if err != nil {
				return nil, i, err
			}
			deps = append(deps, groupDeps...)
			i = nextIdx
			continue
		}

		// Skip || operator and USE flag conditions (handled in parseGroup)
		if token == "||" || strings.HasSuffix(token, "?") {
			i++
			continue
		}

		// Parse package atom
		if ep.isPackageAtom(token) {
			dep, err := ep.parsePackageAtom(token, depType, useFlag, orGroupID)
			if err != nil {
				// Log warning but continue (non-critical)
				i++
				continue
			}
			deps = append(deps, dep)
		}

		i++
	}

	return deps, i, nil
}

// parseGroup handles parsing of grouped dependencies ( ... )
func (ep *EbuildParser) parseGroup(tokens []string, i int, depType DependencyType, useFlag string, orGroupID int) ([]ParsedDependency, int, error) {
	// Check previous token for special operators
	if i > 0 {
		prevToken := tokens[i-1]

		// Handle USE flag condition (e.g., "ssl? ( ... )")
		if strings.HasSuffix(prevToken, "?") {
			flag := strings.TrimSuffix(prevToken, "?")
			// Inherit orGroupID from parent
			return ep.parseTokens(tokens, i+1, depType, flag, orGroupID)
		}

		// Handle || (any-of) operator
		if prevToken == "||" {
			// Assign new OR-group ID and increment counter
			groupID := ep.nextOrGroupID
			ep.nextOrGroupID++
			return ep.parseTokens(tokens, i+1, depType, useFlag, groupID)
		}
	}

	// Regular group - inherit orGroupID from parent
	return ep.parseTokens(tokens, i+1, depType, useFlag, orGroupID)
}

// isPackageAtom checks if token is a valid package atom (not operator, parenthesis,
// or bash syntax fragment).
//
// Valid atoms must contain category/package format with "/".
// Filters out common bash syntax fragments that appear in complex ebuilds:
// - $(cmd) command substitution fragments: $, ), cmd_name
// - '...' string fragments: ', string_content
// - ${VAR} unexpanded variables: ${, VAR}, MULTILIB_USEDEP, etc.
// - Operators and keywords: ||, ^^, ??, if, then, etc.
func (ep *EbuildParser) isPackageAtom(token string) bool {
	if token == "" || token == "(" || token == ")" {
		return false
	}

	// Must contain "/" for category/package format
	// This filters out most bash fragments
	if !strings.Contains(token, "/") {
		return false
	}

	// Filter out tokens starting with bash syntax characters
	if strings.HasPrefix(token, "$") ||
		strings.HasPrefix(token, "'") ||
		strings.HasPrefix(token, "\"") ||
		strings.HasPrefix(token, "`") {
		return false
	}

	// Filter out tokens containing unexpanded ${VAR} patterns
	if strings.Contains(token, "${") {
		return false
	}

	return true
}

// parsePackageAtom parses a single package atom using pkg.ParseAtom (PMS Section 8.3).
// Example: ">=sys-libs/zlib-1.2.13:0/1[static-libs]"
func (ep *EbuildParser) parsePackageAtom(atomStr string, depType DependencyType, useFlag string, orGroupID int) (ParsedDependency, error) {
	dep := ParsedDependency{
		DepType:   depType,
		UseFlag:   useFlag,
		OrGroupID: orGroupID,
	}

	// Use the PMS-compliant atom parser
	atom, err := pkg.ParseAtom(atomStr)
	if err != nil {
		// Fallback to legacy parsing for atoms that fail PMS parsing
		// This handles edge cases like virtual packages or unusual patterns
		return ep.parsePackageAtomLegacy(atomStr, depType, useFlag, orGroupID)
	}

	// Store the parsed atom
	dep.Atom = atom

	// Set blocker flags
	dep.IsBlocker = atom.IsBlocker()
	dep.IsHardBlock = atom.IsStrongBlocker()

	// Convert atom to constraint for backwards compatibility
	dep.Constraint = atom.ToConstraint()
	dep.Constraint.OrGroupID = orGroupID

	// Set slot from atom if present
	if atom.HasSlot() {
		dep.Constraint.Slot = atom.Slot
		if atom.Subslot != "" {
			dep.Constraint.Slot = atom.Slot + "/" + atom.Subslot
		}
		dep.Constraint.Type = pkg.ConstraintTypeSlot
	}

	// Store USE flags in Condition field
	if atom.HasUseDeps() {
		var useFlags []string
		useFlags = append(useFlags, atom.UseRequire...)
		for _, flag := range atom.UseBlock {
			useFlags = append(useFlags, "-"+flag)
		}
		useFlags = append(useFlags, atom.UseConditional...)
		dep.Constraint.Condition = strings.Join(useFlags, " ")
	}

	return dep, nil
}

// parsePackageAtomLegacy is the fallback parser for atoms that fail PMS parsing.
// This handles edge cases and provides backward compatibility.
func (ep *EbuildParser) parsePackageAtomLegacy(atom string, depType DependencyType, useFlag string, orGroupID int) (ParsedDependency, error) {
	dep := ParsedDependency{
		DepType:   depType,
		UseFlag:   useFlag,
		OrGroupID: orGroupID,
	}

	// Check for blockers
	if strings.HasPrefix(atom, "!!") {
		dep.IsBlocker = true
		dep.IsHardBlock = true
		atom = strings.TrimPrefix(atom, "!!")
	} else if strings.HasPrefix(atom, "!") {
		dep.IsBlocker = true
		atom = strings.TrimPrefix(atom, "!")
	}

	// Extract USE flags in brackets [flag1,flag2,-flag3]
	if idx := strings.Index(atom, "["); idx != -1 {
		endIdx := strings.Index(atom, "]")
		if endIdx > idx {
			useFlagsStr := atom[idx+1 : endIdx]
			// Parse USE flags (comma or space separated)
			useFlagsStr = strings.ReplaceAll(useFlagsStr, ",", " ")
			// Store in Condition field for now
			dep.Constraint.Condition = useFlagsStr
			atom = atom[:idx] + atom[endIdx+1:]
		}
	}

	// Extract slot :slot/subslot or :=
	slotParts := strings.Split(atom, ":")
	if len(slotParts) > 1 {
		atom = slotParts[0]
		slotStr := slotParts[1]
		dep.Constraint.Slot = slotStr
		dep.Constraint.Type = pkg.ConstraintTypeSlot
	}

	// Parse version operator and package name
	constraint, err := parseAtomVersion(atom)
	if err != nil {
		return dep, err
	}

	dep.Constraint.Name = constraint.Name
	dep.Constraint.Version = constraint.Version
	if dep.Constraint.Type == 0 {
		dep.Constraint.Type = pkg.ConstraintTypeVersion
	}

	return dep, nil
}

// splitAtomNameVersion splits "category/package-version" into name and version
// Handles: "sys-libs/zlib-1.2.13" -> ("sys-libs/zlib", "1.2.13")
func splitAtomNameVersion(atom string) (string, string) {
	// Find last occurrence of - followed by digit (start of version)
	matches := ebuildAtomVersionRe.FindStringSubmatch(atom)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	// No version found
	return atom, ""
}

// versionOperator defines operator and its handler
type versionOperator struct {
	prefix  string
	handler func(version string) *pkg.VersionConstraint
}

// parseAtomVersion parses version operator from atom
// Supports: =, >=, <=, <, >, ~, =*
func parseAtomVersion(atom string) (pkg.Constraint, error) {
	constraint := pkg.Constraint{
		Type: pkg.ConstraintTypeVersion,
	}

	// Special case: =* operator (glob pattern)
	if strings.HasPrefix(atom, "=") && strings.Contains(atom, "*") {
		atom = strings.TrimPrefix(atom, "=")
		name, version := splitAtomNameVersion(atom)
		versionPattern := strings.TrimSuffix(version, "*")
		constraint.Name = name
		constraint.Version = pkg.NewMinVersionConstraint(versionPattern)
		return constraint, nil
	}

	// Define operators in order (longest first to avoid partial matches)
	operators := []versionOperator{
		{">=", pkg.NewMinVersionConstraint},
		{"<=", pkg.NewMaxVersionConstraint},
		{">", func(v string) *pkg.VersionConstraint { return pkg.NewVersionConstraint(pkg.OpGreater, v) }},
		{"<", func(v string) *pkg.VersionConstraint { return pkg.NewVersionConstraint(pkg.OpLess, v) }},
		{"~", pkg.NewMinVersionConstraint},
		{"=", pkg.NewExactVersionConstraint},
	}

	// Try each operator
	for _, op := range operators {
		if strings.HasPrefix(atom, op.prefix) {
			atom = strings.TrimPrefix(atom, op.prefix)
			name, version := splitAtomNameVersion(atom)

			// Version required for all operators except package name only
			if version == "" {
				return constraint, fmt.Errorf("invalid atom format: missing version after %s", op.prefix)
			}

			constraint.Name = name
			constraint.Version = op.handler(version)
			return constraint, nil
		}
	}

	// No operator - just package name
	constraint.Name = atom
	return constraint, nil
}
