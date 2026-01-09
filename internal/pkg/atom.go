// Package pkg provides domain models for package management.
package pkg

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Atom represents a package dependency atom per PMS Section 8.3.
// Format: [blocker][operator]category/package[-version][:slot[/subslot]][::repository][use-deps]
// This is a Value Object - immutable after creation.
type Atom struct {
	// Blocker type: "" (none), "!" (weak blocker), "!!" (strong blocker)
	Blocker string

	// Version operator: "", "=", ">=", ">", "<=", "<", "~", "=*"
	// Empty means no version constraint (any version matches)
	Operator string

	// Category is the package category (e.g., "sys-libs", "dev-lang")
	Category string

	// Package is the package name without category (e.g., "glibc", "python")
	Package string

	// Version constraint (empty means any version)
	Version string

	// Slot constraint: "", "0", "2", "*", "="
	// Special values:
	//   "*" - any slot (slot operator)
	//   "=" - slot rebuild operator (subslot changes trigger rebuild)
	Slot string

	// Subslot constraint (e.g., "3" in ":0/3")
	Subslot string

	// Repository constraint (e.g., "gentoo" from "::gentoo")
	Repository string

	// UseRequire is list of required USE flags (e.g., ["ssl", "threads"])
	UseRequire []string

	// UseBlock is list of blocked USE flags (e.g., ["debug"] from [-debug])
	UseBlock []string

	// UseConditional is list of conditional USE deps (e.g., ["ssl?"] meaning "if ssl enabled")
	UseConditional []string

	// UseDefault is list of USE flags with defaults (e.g., ["ssl(+)"] meaning default enabled)
	UseDefault []string
}

// Common atom parsing errors.
var (
	ErrEmptyAtom           = errors.New("atom: empty string")
	ErrNoCategory          = errors.New("atom: missing category (no '/' found)")
	ErrEmptyCategory       = errors.New("atom: empty category")
	ErrEmptyPackage        = errors.New("atom: empty package name")
	ErrInvalidCategory     = errors.New("atom: invalid category format")
	ErrInvalidPackage      = errors.New("atom: invalid package name format")
	ErrInvalidVersion      = errors.New("atom: invalid version format")
	ErrInvalidSlot         = errors.New("atom: invalid slot format")
	ErrInvalidRepository   = errors.New("atom: invalid repository format")
	ErrInvalidUseDeps      = errors.New("atom: invalid USE dependency format")
	ErrVersionWithoutOp    = errors.New("atom: version specified without operator")
	ErrOperatorWithoutVer  = errors.New("atom: operator specified without version")
	ErrUnmatchedBracket    = errors.New("atom: unmatched '[' in USE dependencies")
	ErrGlobWithoutOperator = errors.New("atom: glob '*' requires '=' operator")
)

// ParseAtom parses a package atom string per PMS Section 8.3.
// Examples:
//
//	"sys-libs/glibc" - any version
//	">=sys-libs/glibc-2.38" - version 2.38 or higher
//	"=dev-lang/python-3.12*" - glob match (3.12.x)
//	"sys-libs/glibc:2.38" - slot constraint
//	"sys-libs/glibc:2/2.38" - slot/subslot constraint
//	"sys-libs/glibc::gentoo" - repository constraint
//	"dev-libs/openssl[ssl,-static]" - USE dependencies
//	"!sys-libs/uclibc" - weak blocker
//	"!!sys-libs/uclibc" - strong blocker
func ParseAtom(s string) (*Atom, error) {
	if s == "" {
		return nil, ErrEmptyAtom
	}

	atom := &Atom{
		UseRequire:     make([]string, 0),
		UseBlock:       make([]string, 0),
		UseConditional: make([]string, 0),
		UseDefault:     make([]string, 0),
	}

	pos := 0

	// Step 1: Parse blocker prefix (!, !!)
	pos = parseBlocker(s, pos, atom)

	// Step 2: Parse version operator (>=, >, =, <=, <, ~)
	pos = parseOperator(s, pos, atom)

	// Step 3-4: Parse category/package[-version]
	newPos, err := parseCPV(s, pos, atom)
	if err != nil {
		return nil, err
	}
	pos = newPos

	// Step 5: Parse slot and repository
	newPos, err = parseSlotAndRepo(s, pos, atom)
	if err != nil {
		return nil, err
	}
	pos = newPos

	// Step 6: Parse USE dependencies
	if err := parseUseDeps(s, pos, atom); err != nil {
		return nil, err
	}

	// Validate: if we have version but no operator, that's an error
	if atom.Version != "" && atom.Operator == "" {
		return nil, ErrVersionWithoutOp
	}

	return atom, nil
}

// parseBlocker extracts blocker prefix (!, !!) from the atom string.
func parseBlocker(s string, pos int, atom *Atom) int {
	n := len(s)
	if pos < n && s[pos] == '!' {
		pos++
		if pos < n && s[pos] == '!' {
			atom.Blocker = "!!"
			pos++
		} else {
			atom.Blocker = "!"
		}
	}
	return pos
}

// parseOperator extracts version operator from the atom string.
func parseOperator(s string, pos int, atom *Atom) int {
	n := len(s)
	if pos >= n {
		return pos
	}

	switch {
	case pos+1 < n && s[pos:pos+2] == ">=":
		atom.Operator = ">="
		return pos + 2
	case pos+1 < n && s[pos:pos+2] == "<=":
		atom.Operator = "<="
		return pos + 2
	case s[pos] == '>':
		atom.Operator = ">"
		return pos + 1
	case s[pos] == '<':
		atom.Operator = "<"
		return pos + 1
	case s[pos] == '~':
		atom.Operator = "~"
		return pos + 1
	case s[pos] == '=':
		atom.Operator = "="
		return pos + 1
	}
	return pos
}

// parseCPV extracts category/package[-version] from the atom string.
func parseCPV(s string, pos int, atom *Atom) (int, error) {
	n := len(s)

	// Find the end of cat/pkg[-ver] section
	cpvEnd := n
	for i := pos; i < n; i++ {
		if s[i] == ':' || s[i] == '[' {
			cpvEnd = i
			break
		}
	}

	cpv := s[pos:cpvEnd]
	if cpv == "" {
		return pos, ErrNoCategory
	}

	// Find the category/package split (first '/')
	slashIdx := strings.Index(cpv, "/")
	if slashIdx == -1 {
		return pos, ErrNoCategory
	}

	atom.Category = cpv[:slashIdx]
	if atom.Category == "" {
		return pos, ErrEmptyCategory
	}

	if !isValidCategory(atom.Category) {
		return pos, fmt.Errorf("%w: %q", ErrInvalidCategory, atom.Category)
	}

	pkgVer := cpv[slashIdx+1:]
	if pkgVer == "" {
		return pos, ErrEmptyPackage
	}

	// Split package name from version if operator present
	if atom.Operator != "" {
		pkgName, version := splitPackageVersion(pkgVer)
		if pkgName == "" {
			return pos, ErrEmptyPackage
		}
		if version == "" {
			return pos, ErrOperatorWithoutVer
		}

		atom.Package = pkgName
		atom.Version = version

		// Handle glob operator (=cat/pkg-1.2*)
		if strings.HasSuffix(atom.Version, "*") {
			if atom.Operator != "=" {
				return pos, ErrGlobWithoutOperator
			}
			atom.Operator = "=*"
			atom.Version = strings.TrimSuffix(atom.Version, "*")
		}
	} else {
		atom.Package = pkgVer
	}

	if !isValidPackageName(atom.Package) {
		return pos, fmt.Errorf("%w: %q", ErrInvalidPackage, atom.Package)
	}

	return cpvEnd, nil
}

// parseSlotAndRepo extracts slot, subslot, and repository from the atom string.
func parseSlotAndRepo(s string, pos int, atom *Atom) (int, error) {
	n := len(s)

	if pos >= n || s[pos] != ':' {
		return pos, nil
	}
	pos++ // skip ':'

	// Check for :: (repository without slot)
	if pos < n && s[pos] == ':' {
		return parseRepository(s, pos+1, atom)
	}

	// Parse slot[:subslot]
	slotEnd := pos
	for slotEnd < n && s[slotEnd] != ':' && s[slotEnd] != '[' {
		slotEnd++
	}
	slotStr := s[pos:slotEnd]
	pos = slotEnd

	// Split slot/subslot
	if idx := strings.Index(slotStr, "/"); idx != -1 {
		atom.Slot = slotStr[:idx]
		atom.Subslot = slotStr[idx+1:]
	} else {
		atom.Slot = slotStr
	}

	// Validate slot
	if atom.Slot != "" && atom.Slot != "*" && atom.Slot != "=" {
		if !isValidSlot(atom.Slot) {
			return pos, fmt.Errorf("%w: %q", ErrInvalidSlot, atom.Slot)
		}
	}

	// Check for repository after slot
	if pos+1 < n && s[pos:pos+2] == "::" {
		return parseRepository(s, pos+2, atom)
	}

	return pos, nil
}

// parseRepository extracts repository name from the atom string.
func parseRepository(s string, pos int, atom *Atom) (int, error) {
	n := len(s)
	repoEnd := pos
	for repoEnd < n && s[repoEnd] != '[' {
		repoEnd++
	}

	atom.Repository = s[pos:repoEnd]
	if atom.Repository == "" {
		return pos, ErrInvalidRepository
	}
	if !isValidRepository(atom.Repository) {
		return pos, fmt.Errorf("%w: %q", ErrInvalidRepository, atom.Repository)
	}

	return repoEnd, nil
}

// parseUseDeps extracts USE dependencies from the atom string.
func parseUseDeps(s string, pos int, atom *Atom) error {
	n := len(s)

	if pos >= n || s[pos] != '[' {
		return nil
	}
	pos++ // skip '['

	// Find closing bracket
	bracketEnd := strings.Index(s[pos:], "]")
	if bracketEnd == -1 {
		return ErrUnmatchedBracket
	}

	useStr := s[pos : pos+bracketEnd]
	if useStr == "" {
		return nil
	}

	// Parse comma-separated USE flags
	flags := strings.Split(useStr, ",")
	for _, flag := range flags {
		if err := parseUseFlag(flag, atom); err != nil {
			return err
		}
	}

	return nil
}

// parseUseFlag parses a single USE flag and adds it to the atom.
func parseUseFlag(flag string, atom *Atom) error {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return nil
	}

	// Check for default suffix: flag(+) or flag(-)
	hasDefault := false
	defaultValue := ""
	if strings.HasSuffix(flag, "(+)") {
		hasDefault = true
		defaultValue = "+"
		flag = strings.TrimSuffix(flag, "(+)")
	} else if strings.HasSuffix(flag, "(-)") {
		hasDefault = true
		defaultValue = "-"
		flag = strings.TrimSuffix(flag, "(-)")
	}

	// Check for conditional suffix: flag?
	isConditional := strings.HasSuffix(flag, "?")
	if isConditional {
		flag = strings.TrimSuffix(flag, "?")
	}

	// Check for negation prefix: -flag
	isBlocked := strings.HasPrefix(flag, "-")
	if isBlocked {
		flag = strings.TrimPrefix(flag, "-")
	}

	// Validate flag name
	if !isValidUseFlag(flag) {
		return fmt.Errorf("%w: invalid flag %q", ErrInvalidUseDeps, flag)
	}

	// Categorize the flag
	if hasDefault {
		atom.UseDefault = append(atom.UseDefault, flag+"("+defaultValue+")")
	}
	if isConditional {
		if isBlocked {
			atom.UseConditional = append(atom.UseConditional, "!"+flag+"?")
		} else {
			atom.UseConditional = append(atom.UseConditional, flag+"?")
		}
	} else if isBlocked {
		atom.UseBlock = append(atom.UseBlock, flag)
	} else {
		atom.UseRequire = append(atom.UseRequire, flag)
	}

	return nil
}

// splitPackageVersion splits "pkgname-1.2.3" into ("pkgname", "1.2.3").
// Version starts at the last '-' followed by a digit.
// Returns (fullstring, "") if no version found.
func splitPackageVersion(s string) (pkg, ver string) {
	// Work backwards to find the version separator
	// Version must start with a digit after the '-'
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' && i+1 < len(s) && isDigit(s[i+1]) {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// isValidCategory checks if s is a valid category name per PMS.
// Format: [A-Za-z0-9][A-Za-z0-9+_.-]*
func isValidCategory(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alphanumeric
	if !isAlphaNum(rune(s[0])) {
		return false
	}

	// Rest can be alphanumeric, +, _, ., -
	for _, r := range s[1:] {
		if !isAlphaNum(r) && r != '+' && r != '_' && r != '.' && r != '-' {
			return false
		}
	}

	return true
}

// isValidPackageName checks if s is a valid package name per PMS.
// Format: [A-Za-z0-9][A-Za-z0-9+_-]*
// Note: '.' is NOT allowed in package names (unlike categories)
func isValidPackageName(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alphanumeric
	if !isAlphaNum(rune(s[0])) {
		return false
	}

	// Rest can be alphanumeric, +, _, -
	// Note: '.' is forbidden per PMS
	for _, r := range s[1:] {
		if !isAlphaNum(r) && r != '+' && r != '_' && r != '-' {
			return false
		}
	}

	return true
}

// isValidSlot checks if s is a valid slot name per PMS.
// Format: [A-Za-z0-9][A-Za-z0-9+_.-]*
func isValidSlot(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alphanumeric
	if !isAlphaNum(rune(s[0])) {
		return false
	}

	// Rest can be alphanumeric, +, _, ., -
	for _, r := range s[1:] {
		if !isAlphaNum(r) && r != '+' && r != '_' && r != '.' && r != '-' {
			return false
		}
	}

	return true
}

// isValidRepository checks if s is a valid repository name per PMS.
// Format: [A-Za-z0-9][A-Za-z0-9_-]*
func isValidRepository(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alphanumeric
	if !isAlphaNum(rune(s[0])) {
		return false
	}

	// Rest can be alphanumeric, _, -
	for _, r := range s[1:] {
		if !isAlphaNum(r) && r != '_' && r != '-' {
			return false
		}
	}

	return true
}

// isValidUseFlag checks if s is a valid USE flag name per PMS.
// Format: [A-Za-z0-9][A-Za-z0-9+_@-]*
func isValidUseFlag(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alphanumeric
	if !isAlphaNum(rune(s[0])) {
		return false
	}

	// Rest can be alphanumeric, +, _, @, -
	for _, r := range s[1:] {
		if !isAlphaNum(r) && r != '+' && r != '_' && r != '@' && r != '-' {
			return false
		}
	}

	return true
}

// isAlphaNum returns true if r is ASCII alphanumeric
func isAlphaNum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isDigit returns true if b is an ASCII digit
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// String returns the canonical string representation of the atom.
func (a *Atom) String() string {
	if a == nil {
		return ""
	}

	var sb strings.Builder

	// Blocker
	sb.WriteString(a.Blocker)

	// Operator (but not =* - that's rendered as = with * suffix on version)
	if a.Operator == "=*" {
		sb.WriteString("=")
	} else {
		sb.WriteString(a.Operator)
	}

	// Category/Package
	sb.WriteString(a.Category)
	sb.WriteString("/")
	sb.WriteString(a.Package)

	// Version
	if a.Version != "" {
		sb.WriteString("-")
		sb.WriteString(a.Version)
		if a.Operator == "=*" {
			sb.WriteString("*")
		}
	}

	// Slot
	if a.Slot != "" {
		sb.WriteString(":")
		sb.WriteString(a.Slot)
		if a.Subslot != "" {
			sb.WriteString("/")
			sb.WriteString(a.Subslot)
		}
	}

	// Repository
	if a.Repository != "" {
		sb.WriteString("::")
		sb.WriteString(a.Repository)
	}

	// USE dependencies
	if len(a.UseRequire) > 0 || len(a.UseBlock) > 0 || len(a.UseConditional) > 0 {
		sb.WriteString("[")
		first := true

		for _, flag := range a.UseRequire {
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString(flag)
			first = false
		}

		for _, flag := range a.UseBlock {
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString("-")
			sb.WriteString(flag)
			first = false
		}

		for _, flag := range a.UseConditional {
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString(flag)
			first = false
		}

		sb.WriteString("]")
	}

	return sb.String()
}

// CPV returns the Category/Package-Version string (without operator/blocker/slot/use).
func (a *Atom) CPV() string {
	if a == nil {
		return ""
	}

	if a.Version == "" {
		return a.Category + "/" + a.Package
	}
	return a.Category + "/" + a.Package + "-" + a.Version
}

// CP returns the Category/Package string (without version).
func (a *Atom) CP() string {
	if a == nil {
		return ""
	}
	return a.Category + "/" + a.Package
}

// IsBlocker returns true if this atom is a blocker (weak or strong).
func (a *Atom) IsBlocker() bool {
	return a != nil && a.Blocker != ""
}

// IsStrongBlocker returns true if this atom is a strong blocker (!!).
func (a *Atom) IsStrongBlocker() bool {
	return a != nil && a.Blocker == "!!"
}

// IsWeakBlocker returns true if this atom is a weak blocker (!).
func (a *Atom) IsWeakBlocker() bool {
	return a != nil && a.Blocker == "!"
}

// HasVersion returns true if this atom has a version constraint.
func (a *Atom) HasVersion() bool {
	return a != nil && a.Version != ""
}

// HasSlot returns true if this atom has a slot constraint.
func (a *Atom) HasSlot() bool {
	return a != nil && a.Slot != ""
}

// HasRepository returns true if this atom has a repository constraint.
func (a *Atom) HasRepository() bool {
	return a != nil && a.Repository != ""
}

// HasUseDeps returns true if this atom has USE dependencies.
func (a *Atom) HasUseDeps() bool {
	return a != nil && (len(a.UseRequire) > 0 || len(a.UseBlock) > 0 || len(a.UseConditional) > 0)
}

// Matches checks if this atom matches the given package.
// It checks category, package name, version constraint, and slot.
func (a *Atom) Matches(p *Package) bool {
	if a == nil || p == nil {
		return false
	}

	// Extract category and package from p.Name (format: "category/package")
	parts := strings.SplitN(p.Name, "/", 2)
	if len(parts) != 2 {
		return false
	}
	pCat, pPkg := parts[0], parts[1]

	// Check category and package name
	if a.Category != pCat || a.Package != pPkg {
		return false
	}

	// Check version constraint
	if a.Version != "" {
		if !a.matchesVersion(p.Version) {
			return false
		}
	}

	// Check slot constraint
	if a.Slot != "" && a.Slot != "*" && a.Slot != "=" {
		if p.Slot.Name != a.Slot {
			return false
		}
	}

	// Check subslot constraint
	if a.Subslot != "" {
		if p.Slot.Subslot != a.Subslot {
			return false
		}
	}

	return true
}

// matchesVersion checks if the given version matches this atom's version constraint.
func (a *Atom) matchesVersion(version string) bool {
	if a.Version == "" {
		return true // No version constraint means any version matches
	}

	switch a.Operator {
	case "=":
		return version == a.Version
	case "=*":
		// Glob match: version must start with constraint version
		return strings.HasPrefix(version, a.Version)
	case ">":
		return CompareVersions(version, a.Version) > 0
	case ">=":
		return CompareVersions(version, a.Version) >= 0
	case "<":
		return CompareVersions(version, a.Version) < 0
	case "<=":
		return CompareVersions(version, a.Version) <= 0
	case "~":
		// Revision match: versions must match ignoring revision (-rN)
		return matchesRevision(version, a.Version)
	default:
		return false
	}
}

// matchesRevision checks if two versions match ignoring revision (-rN suffix).
// Used for the ~ operator.
func matchesRevision(v1, v2 string) bool {
	// Strip -rN suffix from both versions
	strip := func(v string) string {
		if idx := strings.LastIndex(v, "-r"); idx != -1 {
			// Verify it's actually a revision (followed by digits)
			suffix := v[idx+2:]
			allDigits := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(suffix) > 0 {
				return v[:idx]
			}
		}
		return v
	}

	return strip(v1) == strip(v2)
}

// ToConstraint converts this Atom to a Constraint for use with the solver.
func (a *Atom) ToConstraint() Constraint {
	c := Constraint{
		Type: ConstraintTypeVersion,
		Name: a.CP(),
	}

	if a.Version != "" {
		var op VersionOperator
		switch a.Operator {
		case "=", "=*":
			op = OpEqual
		case ">":
			op = OpGreater
		case ">=":
			op = OpGreaterEqual
		case "<":
			op = OpLess
		case "<=":
			op = OpLessEqual
		default:
			op = OpEqual
		}
		c.Version = NewVersionConstraint(op, a.Version)
	}

	if a.Slot != "" {
		c.Slot = a.Slot
	}

	return c
}
