package virtual

import (
	"strings"

	"github.com/coregx/coregex"
)

// Precompiled regex patterns for virtual package parsing (compiled once at package init).
var (
	// packageAtomRegex validates if a token looks like a package atom.
	// Valid examples:
	// - category/package
	// - >=category/package-1.0
	// - category/package:slot
	// - category/package[use]
	// - !category/package (blocker)
	packageAtomRegex = coregex.MustCompile(
		`^!?!?[<>=~]*[a-zA-Z0-9_][-a-zA-Z0-9_]*/[a-zA-Z0-9_][-a-zA-Z0-9_.+]*`,
	)

	// virtualVersionRe matches version pattern for stripping version from atoms.
	// Uses greedy match with end anchor to find the LAST dash-digit pattern.
	// Example: "dev-java/openjdk-17" -> group 1 = "dev-java/openjdk"
	virtualVersionRe = coregex.MustCompile(`^(.+)-(\d.*)$`)
)

// Parser extracts provider information from RDEPEND strings.
type Parser struct {
	// Reserved for future extension
}

// NewParser creates a new dependency parser.
func NewParser() *Parser {
	return &Parser{}
}

// GetProviders parses an RDEPEND string and extracts providers from || blocks.
//
// Gentoo RDEPEND format example:
//
//	RDEPEND="
//	    || (
//	        dev-java/openjdk:17
//	        dev-java/openjdk-bin:17
//	        dev-java/oracle-jdk-bin:17
//	    )
//	"
//
// Returns all providers found in any || block.
func (p *Parser) GetProviders(rdepend string) []string {
	providers := make([]string, 0)

	// Find all || ( ... ) blocks
	blocks := p.extractOrBlocks(rdepend)

	for _, block := range blocks {
		// Extract package atoms from the block
		atoms := p.extractAtoms(block)
		providers = append(providers, atoms...)
	}

	return providers
}

// GetProvidersMap parses RDEPEND and returns providers grouped by OR-block.
//
// Each OR-block is represented by its index (starting from 0).
// This allows distinguishing between multiple independent OR groups.
//
// Example:
//
//	map[0][]string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"}
//	map[1][]string{"app-editors/vim", "app-editors/emacs"}
func (p *Parser) GetProvidersMap(rdepend string) map[int][]string {
	result := make(map[int][]string)

	blocks := p.extractOrBlocks(rdepend)

	for i, block := range blocks {
		atoms := p.extractAtoms(block)
		if len(atoms) > 0 {
			result[i] = atoms
		}
	}

	return result
}

// extractOrBlocks finds all || ( ... ) blocks in a dependency string.
//
// Handles nested parentheses correctly.
func (p *Parser) extractOrBlocks(s string) []string {
	var blocks []string
	i := 0

	for i < len(s) {
		// Look for || pattern
		idx := strings.Index(s[i:], "||")
		if idx == -1 {
			break
		}

		startPos := i + idx + 2 // Position after "||"

		// Skip whitespace to find opening paren
		for startPos < len(s) && (s[startPos] == ' ' || s[startPos] == '\t' || s[startPos] == '\n') {
			startPos++
		}

		if startPos >= len(s) || s[startPos] != '(' {
			i = startPos
			continue
		}

		// Find matching closing paren
		block, endPos := p.extractBlock(s, startPos)
		if block != "" {
			blocks = append(blocks, block)
		}

		i = endPos
	}

	return blocks
}

// extractBlock extracts content between matching parentheses.
//
// Returns the content (without outer parens) and the position after closing paren.
func (p *Parser) extractBlock(s string, start int) (string, int) {
	if start >= len(s) || s[start] != '(' {
		return "", start
	}

	depth := 0
	contentStart := start + 1

	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				// Found matching closing paren
				content := strings.TrimSpace(s[contentStart:i])
				return content, i + 1
			}
		}
	}

	// Unbalanced parens - return what we have
	return strings.TrimSpace(s[contentStart:]), len(s)
}

// extractAtoms extracts package atoms from a block content.
//
// Filters out:
// - USE flag conditions (word?)
// - Nested groups
// - Empty entries
//
// Handles:
// - Simple atoms: dev-java/openjdk
// - Versioned atoms: >=dev-java/openjdk-17
// - Slotted atoms: dev-java/openjdk:17
// - Atoms with USE deps: dev-java/openjdk[ssl]
func (p *Parser) extractAtoms(block string) []string {
	var atoms []string

	// Tokenize by whitespace
	tokens := tokenize(block)

	for _, token := range tokens {
		// Skip operators and control tokens
		if token == "||" || token == "(" || token == ")" {
			continue
		}

		// Skip USE flag conditions (ends with ?)
		if strings.HasSuffix(token, "?") {
			continue
		}

		// Validate as package atom
		if isPackageAtom(token) {
			atoms = append(atoms, token)
		}
	}

	return atoms
}

// tokenize splits a string into tokens, handling nested parens.
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			if current.Len() > 0 {
				tokens = append(tokens, strings.TrimSpace(current.String()))
				current.Reset()
			}
			// Skip nested groups entirely
		case ')':
			depth--
			if depth < 0 {
				depth = 0
			}
		case ' ', '\t', '\n':
			if depth == 0 && current.Len() > 0 {
				tokens = append(tokens, strings.TrimSpace(current.String()))
				current.Reset()
			}
		default:
			if depth == 0 {
				current.WriteRune(ch)
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, strings.TrimSpace(current.String()))
	}

	return tokens
}

func isPackageAtom(token string) bool {
	if token == "" {
		return false
	}

	return packageAtomRegex.MatchString(token)
}

// ParseVirtualProviders parses RDEPEND of a virtual package ebuild
// and returns the list of providers.
//
// This is a convenience function that:
// 1. Creates a parser
// 2. Extracts providers from the RDEPEND
// 3. Normalizes provider names (strips version operators)
//
// Example:
//
//	providers := ParseVirtualProviders(rdepend)
//	// Returns: ["dev-java/openjdk", "dev-java/oracle-jdk-bin"]
func ParseVirtualProviders(rdepend string) []string {
	parser := NewParser()
	rawProviders := parser.GetProviders(rdepend)

	// Normalize providers (remove version operators)
	providers := make([]string, 0, len(rawProviders))
	for _, p := range rawProviders {
		normalized := normalizeProvider(p)
		if normalized != "" {
			providers = append(providers, normalized)
		}
	}

	return providers
}

// normalizeProvider strips version operators and USE deps from an atom.
//
// Examples:
//
//	">=dev-java/openjdk-17:17[ssl]" -> "dev-java/openjdk:17"
func normalizeProvider(atom string) string {
	// Strip leading blockers
	atom = strings.TrimPrefix(atom, "!!")
	atom = strings.TrimPrefix(atom, "!")

	// Strip leading version operators
	atom = strings.TrimPrefix(atom, ">=")
	atom = strings.TrimPrefix(atom, "<=")
	atom = strings.TrimPrefix(atom, ">")
	atom = strings.TrimPrefix(atom, "<")
	atom = strings.TrimPrefix(atom, "~")
	atom = strings.TrimPrefix(atom, "=")

	// Strip USE deps in brackets
	if idx := strings.Index(atom, "["); idx != -1 {
		// Keep slot if present
		slotIdx := strings.Index(atom, ":")
		if slotIdx != -1 && slotIdx < idx {
			atom = atom[:idx]
		} else if slotIdx > idx {
			// Slot after USE deps
			useEnd := strings.Index(atom, "]")
			if useEnd != -1 && slotIdx > useEnd {
				atom = atom[:idx] + atom[slotIdx:]
			} else {
				atom = atom[:idx]
			}
		} else {
			atom = atom[:idx]
		}
	}

	// Extract base package name (strip version) using precompiled regex
	// Pattern has 3 groups: [0]=full match, [1]=package name, [2]=version
	matches := virtualVersionRe.FindStringSubmatch(atom)
	if len(matches) == 3 {
		// Keep the slot if present in original atom
		slotIdx := strings.Index(atom, ":")
		if slotIdx != -1 {
			return matches[1] + atom[slotIdx:]
		}
		return matches[1]
	}

	return atom
}

// ExtractVirtualDependencies identifies virtual package dependencies in RDEPEND.
//
// Returns a map of virtual package names to their available providers.
//
// Example output:
//
//	map["virtual/jdk"][]string{"dev-java/openjdk", "dev-java/oracle-jdk-bin"}
func ExtractVirtualDependencies(rdepend string) map[string][]string {
	result := make(map[string][]string)

	// Note: This function is a placeholder for future implementation.
	// Full extraction requires context about which virtual package
	// the RDEPEND belongs to.
	_ = rdepend // Suppress unused warning until implementation complete

	return result
}
