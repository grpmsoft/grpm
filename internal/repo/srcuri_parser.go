// Package repo provides repository abstractions for package management.
//
// This file implements SRC_URI parsing per PMS Section 7.2.9.
// It handles arrow syntax for filename renaming and USE flag conditionals.
package repo

import (
	"fmt"
	"path"
	"strings"
)

// SrcURIEntry represents a single source file entry from SRC_URI.
//
// SrcURIEntry is a Value Object - immutable after creation.
// It contains all information needed to download a source file.
type SrcURIEntry struct {
	// URL is the download URL
	URL string

	// Filename is the local filename (from arrow syntax, or basename of URL)
	Filename string

	// UseFlag is the conditional USE flag (empty if unconditional)
	// For "ssl? ( url )" this would be "ssl"
	UseFlag string

	// Negate is true for negated conditions (!useflag?)
	// For "!minimal? ( url )" this would be true with UseFlag="minimal"
	Negate bool
}

// SrcURIParser handles parsing of SRC_URI with arrow syntax and conditionals.
//
// The parser supports:
//   - Arrow syntax: url -> filename
//   - USE conditionals: use? ( ... )
//   - Negated conditionals: !use? ( ... )
//   - Nested conditionals
//   - Variable expansion: ${P}, ${PV}, etc.
type SrcURIParser struct {
	// vars holds variable values for expansion (P, PV, PN, etc.)
	vars map[string]string

	// activeFlags indicates which USE flags are enabled
	activeFlags map[string]bool
}

// NewSrcURIParser creates a new SRC_URI parser with variable mappings.
//
// The vars parameter should contain Portage variables like:
//   - P: package-version (e.g., "hello-1.0")
//   - PV: version (e.g., "1.0")
//   - PN: package name (e.g., "hello")
//
// The activeFlags parameter indicates which USE flags are enabled.
func NewSrcURIParser(vars map[string]string, activeFlags map[string]bool) *SrcURIParser {
	p := &SrcURIParser{
		vars:        make(map[string]string),
		activeFlags: make(map[string]bool),
	}

	// Copy vars to avoid external modification
	for k, v := range vars {
		p.vars[k] = v
	}

	// Copy activeFlags
	for k, v := range activeFlags {
		p.activeFlags[k] = v
	}

	return p
}

// Parse parses SRC_URI content and returns entries for active USE flags.
//
// The parser handles:
//   - Simple URLs: https://example.com/foo.tar.gz
//   - Arrow syntax: https://example.com/foo.tar.gz -> bar.tar.gz
//   - USE conditionals: ssl? ( https://example.com/ssl.tar.gz )
//   - Negated conditionals: !minimal? ( https://example.com/extras.tar.gz )
//   - Nested conditionals: ssl? ( gnutls? ( url1 ) openssl? ( url2 ) )
//   - Variable expansion in filenames: -> ${P}.tar.gz
func (p *SrcURIParser) Parse(srcURI string) ([]SrcURIEntry, error) {
	tokens := tokenizeSrcURI(srcURI)
	if len(tokens) == 0 {
		return nil, nil
	}

	entries, _, err := p.parseTokens(tokens, 0, "", false)
	if err != nil {
		return nil, fmt.Errorf("parsing SRC_URI: %w", err)
	}

	return entries, nil
}

// parseTokens recursively parses tokens starting at index.
// Returns entries, next index to process, and any error.
func (p *SrcURIParser) parseTokens(tokens []string, start int, useFlag string, negate bool) ([]SrcURIEntry, int, error) {
	var entries []SrcURIEntry
	i := start

	for i < len(tokens) {
		token := tokens[i]

		// Handle closing parenthesis - end of group
		if token == ")" {
			return entries, i + 1, nil
		}

		// Handle USE flag conditional: flag? or !flag?
		if isUseCondition(token) {
			flag, neg := parseUseCondition(token)

			// Next token must be "("
			if i+1 >= len(tokens) || tokens[i+1] != "(" {
				return nil, i, fmt.Errorf("expected '(' after %s at position %d", token, i)
			}

			// Parse the conditional block
			blockEntries, nextIdx, err := p.parseTokens(tokens, i+2, flag, neg)
			if err != nil {
				return nil, i, err
			}

			// Only include entries if condition is met
			if p.conditionMet(flag, neg) {
				entries = append(entries, blockEntries...)
			}

			i = nextIdx
			continue
		}

		// Handle opening parenthesis - anonymous group
		if token == "(" {
			groupEntries, nextIdx, err := p.parseTokens(tokens, i+1, useFlag, negate)
			if err != nil {
				return nil, i, err
			}
			entries = append(entries, groupEntries...)
			i = nextIdx
			continue
		}

		// Handle URL
		if isURL(token) {
			entry, advance := p.parseURLEntry(tokens, i, useFlag, negate)
			entries = append(entries, entry)
			i += advance
			continue
		}

		// Skip unknown tokens
		i++
	}

	return entries, i, nil
}

// parseURLEntry parses a URL entry, handling optional arrow syntax.
// Returns the entry and number of tokens consumed.
func (p *SrcURIParser) parseURLEntry(tokens []string, i int, useFlag string, negate bool) (SrcURIEntry, int) {
	entry := SrcURIEntry{
		URL:     tokens[i],
		UseFlag: useFlag,
		Negate:  negate,
	}

	// Check for arrow syntax: URL -> filename
	if i+2 < len(tokens) && tokens[i+1] == "->" {
		// Expand variables in the filename
		entry.Filename = p.expandVariables(tokens[i+2])
		return entry, 3 // consumed: URL, ->, filename
	}

	// No arrow - use basename of URL
	entry.Filename = extractFilename(entry.URL)
	return entry, 1 // consumed: URL only
}

// conditionMet checks if a USE flag condition is satisfied.
func (p *SrcURIParser) conditionMet(flag string, negate bool) bool {
	active := p.activeFlags[flag]
	if negate {
		return !active
	}
	return active
}

// expandVariables expands ${VAR} references in a string.
//
// Supports:
//   - ${VAR} -> value of VAR (empty string if not set)
//   - ${VAR:-default} -> value of VAR, or "default" if unset
func (p *SrcURIParser) expandVariables(s string) string {
	result := s

	// Simple variable expansion: ${VAR}
	for k, v := range p.vars {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}

	// Handle ${VAR:-default} pattern and remove unexpanded variables
	result = expandDefaultSyntax(result, p.vars)

	// Remove any remaining unexpanded ${VAR} patterns (undefined variables)
	result = removeUnexpandedVariables(result)

	return result
}

// removeUnexpandedVariables removes any remaining ${VAR} patterns.
// This handles undefined variables by removing them entirely.
func removeUnexpandedVariables(s string) string {
	result := s

	for {
		// Find ${
		idx := strings.Index(result, "${")
		if idx == -1 {
			break
		}

		// Find matching }
		endIdx := strings.Index(result[idx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += idx

		// Remove the entire ${...} pattern
		result = result[:idx] + result[endIdx+1:]
	}

	return result
}

// expandDefaultSyntax handles ${VAR:-default} patterns.
func expandDefaultSyntax(s string, vars map[string]string) string {
	result := s
	start := 0

	for {
		// Find ${
		idx := strings.Index(result[start:], "${")
		if idx == -1 {
			break
		}
		idx += start

		// Find matching }
		endIdx := strings.Index(result[idx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += idx

		// Extract content: VAR:-default
		content := result[idx+2 : endIdx]

		// Check for :- syntax
		if colonIdx := strings.Index(content, ":-"); colonIdx != -1 {
			varName := content[:colonIdx]
			defaultVal := content[colonIdx+2:]

			var replacement string
			if val, ok := vars[varName]; ok && val != "" {
				replacement = val
			} else {
				replacement = defaultVal
			}

			result = result[:idx] + replacement + result[endIdx+1:]
			// Don't advance start - the replacement might contain more variables
			continue
		}

		// Move past this ${} block
		start = endIdx + 1
	}

	return result
}

// tokenizeSrcURI splits SRC_URI content into tokens.
//
// Tokens are:
//   - URLs (starting with http://, https://, ftp://, mirror://)
//   - USE conditions (ending with ?)
//   - Arrow operator (->)
//   - Parentheses ( )
//   - Filenames (after ->)
func tokenizeSrcURI(srcURI string) []string {
	var tokens []string
	var current strings.Builder

	// Normalize whitespace
	content := strings.TrimSpace(srcURI)
	if content == "" {
		return nil
	}

	flushCurrent := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	i := 0
	for i < len(content) {
		ch := content[i]

		switch ch {
		case '(':
			flushCurrent()
			tokens = append(tokens, "(")
			i++

		case ')':
			flushCurrent()
			tokens = append(tokens, ")")
			i++

		case ' ', '\t', '\n', '\r':
			flushCurrent()
			// Skip consecutive whitespace
			for i < len(content) && isWhitespace(content[i]) {
				i++
			}

		case '-':
			// Check for arrow operator
			if i+1 < len(content) && content[i+1] == '>' {
				flushCurrent()
				tokens = append(tokens, "->")
				i += 2
			} else {
				current.WriteByte(ch)
				i++
			}

		default:
			current.WriteByte(ch)
			i++
		}
	}

	flushCurrent()
	return tokens
}

// isWhitespace checks if a byte is whitespace.
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// isUseCondition checks if a token is a USE flag condition.
// Matches: flag? or !flag?
func isUseCondition(token string) bool {
	if !strings.HasSuffix(token, "?") {
		return false
	}

	// Remove the ? and optional ! prefix
	flag := strings.TrimSuffix(token, "?")
	flag = strings.TrimPrefix(flag, "!")

	// Must have a valid flag name remaining
	return len(flag) > 0 && isValidUseFlagName(flag)
}

// isValidUseFlagName checks if a string is a valid USE flag name.
// USE flag names: [A-Za-z0-9][A-Za-z0-9+_@-]*
func isValidUseFlagName(name string) bool {
	if len(name) == 0 {
		return false
	}

	// First character must be alphanumeric
	first := name[0]
	if !isAlphaNum(first) {
		return false
	}

	// Rest can include +, _, @, -
	for i := 1; i < len(name); i++ {
		ch := name[i]
		if !isAlphaNum(ch) && ch != '+' && ch != '_' && ch != '@' && ch != '-' {
			return false
		}
	}

	return true
}

// isAlphaNum checks if a byte is alphanumeric.
func isAlphaNum(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// parseUseCondition extracts the USE flag name and negation from a condition.
// Input: "flag?" returns ("flag", false)
// Input: "!flag?" returns ("flag", true)
func parseUseCondition(token string) (flag string, negate bool) {
	// Remove trailing ?
	flag = strings.TrimSuffix(token, "?")

	// Check for negation
	if strings.HasPrefix(flag, "!") {
		negate = true
		flag = strings.TrimPrefix(flag, "!")
	}

	return flag, negate
}

// isURL checks if a token looks like a URL.
func isURL(token string) bool {
	// Check for common URL schemes
	schemes := []string{
		"http://",
		"https://",
		"ftp://",
		"mirror://",
	}

	lower := strings.ToLower(token)
	for _, scheme := range schemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}

	return false
}

// extractFilename extracts the filename from a URL.
//
// For most URLs, this is the basename.
// Special handling for mirror:// URLs and query strings.
func extractFilename(url string) string {
	// Handle mirror:// URLs: mirror://sourceforge/project/file.tar.gz
	if strings.HasPrefix(strings.ToLower(url), "mirror://") {
		// Extract path after mirror://name/
		parts := strings.SplitN(url, "/", 4) // mirror:, empty, name, path
		if len(parts) >= 4 {
			return path.Base(parts[3])
		}
	}

	// Remove query string and fragment
	clean := url
	if idx := strings.Index(clean, "?"); idx != -1 {
		clean = clean[:idx]
	}
	if idx := strings.Index(clean, "#"); idx != -1 {
		clean = clean[:idx]
	}

	return path.Base(clean)
}

// ParseSrcURI is a convenience function for parsing SRC_URI.
//
// It creates a parser with the given variables and flags, then parses the content.
func ParseSrcURI(content string, activeFlags map[string]bool, vars map[string]string) ([]SrcURIEntry, error) {
	parser := NewSrcURIParser(vars, activeFlags)
	return parser.Parse(content)
}

// ExpandFilename expands variables in a filename string.
//
// This is a convenience function for expanding filenames outside of parsing.
func ExpandFilename(filename string, vars map[string]string) string {
	parser := NewSrcURIParser(vars, nil)
	return parser.expandVariables(filename)
}
