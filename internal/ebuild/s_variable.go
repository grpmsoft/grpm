package ebuild

import (
	"regexp"
	"strings"
	"unicode"
)

// ParseSVariable extracts and evaluates S variable from ebuild content.
//
// The S variable specifies the source directory within WORKDIR.
// Default is ${WORKDIR}/${P}, but many packages define custom S.
//
// Supported patterns:
//   - ${WORKDIR}/${PN/f/F}-${PV} (substitution)
//   - ${WORKDIR}/${PN^}-${PV} (uppercase first char)
//   - ${WORKDIR}/${MY_P} (custom variable)
//   - ${WORKDIR}/source (hardcoded)
//
// Parameters:
//   - content: ebuild file content
//   - vars: map of variables (P, PN, PV, WORKDIR, etc.)
//
// Returns evaluated S path, or empty string if S is not defined.
func ParseSVariable(content string, vars map[string]string) string {
	// Extract S= assignment
	sValue := extractEbuildVariable(content, "S")
	if sValue == "" {
		return ""
	}

	// First extract any custom variables defined in ebuild
	customVars := extractCustomVariables(content, vars)
	for k, v := range customVars {
		if _, exists := vars[k]; !exists {
			vars[k] = v
		}
	}

	// Expand the S value with bash parameter expansion
	return ExpandBashParameters(sValue, vars)
}

// extractCustomVariables extracts custom variable definitions from ebuild.
//
// Looks for MY_P, MY_PN, MY_PV and similar custom variables.
func extractCustomVariables(content string, vars map[string]string) map[string]string {
	result := make(map[string]string)

	// Common custom variable names
	customVarNames := []string{"MY_P", "MY_PN", "MY_PV", "MY_PF"}

	for _, varName := range customVarNames {
		value := extractEbuildVariable(content, varName)
		if value != "" {
			result[varName] = ExpandBashParameters(value, vars)
		}
	}

	return result
}

// ExpandBashParameters expands bash parameter expansion syntax.
//
// Supports:
//   - ${var} - simple expansion
//   - ${var/pattern/replacement} - single substitution
//   - ${var//pattern/replacement} - global substitution
//   - ${var^} - uppercase first character
//   - ${var^^} - uppercase all
//   - ${var,} - lowercase first character
//   - ${var,,} - lowercase all
//   - ${var%pattern} - remove shortest suffix
//   - ${var%%pattern} - remove longest suffix
//   - ${var#pattern} - remove shortest prefix
//   - ${var##pattern} - remove longest prefix
func ExpandBashParameters(s string, vars map[string]string) string {
	result := s

	// Process complex expansions first (longest patterns)
	result = expandComplexParameters(result, vars)

	// Then simple ${var} expansion
	result = expandSimpleVariables(result, vars)

	// Finally $VAR expansion (without braces)
	result = expandDollarVariables(result, vars)

	return result
}

// expandComplexParameters handles ${var/pattern/replacement} and similar.
func expandComplexParameters(s string, vars map[string]string) string {
	// Pattern for complex expansions: ${varname<operator>...}
	complexPattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(//|/|##|#|%%|%|\^\^|\^|,,|,)([^}]*)\}`)

	result := s
	for {
		match := complexPattern.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		varName := result[match[2]:match[3]]
		operator := result[match[4]:match[5]]
		operand := result[match[6]:match[7]]

		varValue, exists := vars[varName]
		if !exists {
			// Variable not found, leave as-is or empty
			result = result[:match[0]] + result[match[1]:]
			continue
		}

		expanded := applyOperator(varValue, operator, operand)
		result = result[:match[0]] + expanded + result[match[1]:]
	}

	return result
}

// applyOperator applies bash parameter expansion operator.
func applyOperator(value, operator, operand string) string {
	switch operator {
	case "/":
		// ${var/pattern/replacement} - single substitution
		parts := splitSubstitution(operand)
		if len(parts) >= 1 {
			replacement := ""
			if len(parts) >= 2 {
				replacement = parts[1]
			}
			return replaceFirst(value, parts[0], replacement)
		}
		return value

	case "//":
		// ${var//pattern/replacement} - global substitution
		parts := splitSubstitution(operand)
		if len(parts) >= 1 {
			replacement := ""
			if len(parts) >= 2 {
				replacement = parts[1]
			}
			return strings.ReplaceAll(value, parts[0], replacement)
		}
		return value

	case "^":
		// ${var^} - uppercase first character
		if len(value) == 0 {
			return value
		}
		return string(unicode.ToUpper(rune(value[0]))) + value[1:]

	case "^^":
		// ${var^^} - uppercase all
		return strings.ToUpper(value)

	case ",":
		// ${var,} - lowercase first character
		if len(value) == 0 {
			return value
		}
		return string(unicode.ToLower(rune(value[0]))) + value[1:]

	case ",,":
		// ${var,,} - lowercase all
		return strings.ToLower(value)

	case "%":
		// ${var%pattern} - remove shortest suffix match
		return removeSuffix(value, operand, false)

	case "%%":
		// ${var%%pattern} - remove longest suffix match
		return removeSuffix(value, operand, true)

	case "#":
		// ${var#pattern} - remove shortest prefix match
		return removePrefix(value, operand, false)

	case "##":
		// ${var##pattern} - remove longest prefix match
		return removePrefix(value, operand, true)

	default:
		return value
	}
}

// splitSubstitution splits pattern/replacement from operand.
func splitSubstitution(operand string) []string {
	// Find unescaped /
	parts := make([]string, 0, 2)
	current := ""
	escaped := false

	for _, c := range operand {
		if escaped {
			current += string(c)
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if c == '/' {
			parts = append(parts, current)
			current = ""
			continue
		}

		current += string(c)
	}

	parts = append(parts, current)
	return parts
}

// replaceFirst replaces first occurrence of pattern.
func replaceFirst(s, pattern, replacement string) string {
	idx := strings.Index(s, pattern)
	if idx == -1 {
		return s
	}
	return s[:idx] + replacement + s[idx+len(pattern):]
}

// removeSuffix removes suffix matching pattern.
func removeSuffix(s, pattern string, longest bool) string {
	// Convert glob pattern to simple suffix match
	// Support * as wildcard

	if strings.HasSuffix(pattern, "*") {
		// ${var%prefix*} - remove from prefix to end
		prefix := pattern[:len(pattern)-1]
		if longest {
			// Find first occurrence (longest match)
			idx := strings.Index(s, prefix)
			if idx != -1 {
				return s[:idx]
			}
		} else {
			// Find last occurrence (shortest match)
			idx := strings.LastIndex(s, prefix)
			if idx != -1 {
				return s[:idx]
			}
		}
	} else if strings.HasPrefix(pattern, "*") {
		// ${var%*suffix} - remove everything from suffix to end
		// This is less common, typically used as %% for longest
		suffix := pattern[1:]
		if longest {
			idx := strings.Index(s, suffix)
			if idx != -1 {
				return s[:idx]
			}
		} else {
			idx := strings.LastIndex(s, suffix)
			if idx != -1 {
				return s[:idx]
			}
		}
	} else {
		// Simple suffix
		if strings.HasSuffix(s, pattern) {
			return s[:len(s)-len(pattern)]
		}
	}
	return s
}

// removePrefix removes prefix matching pattern.
func removePrefix(s, pattern string, longest bool) string {
	// Convert glob pattern to simple prefix match
	// For now, support * as wildcard
	if strings.HasSuffix(pattern, "*") {
		// ${var##prefix*} - remove everything up to prefix
		prefix := pattern[:len(pattern)-1]
		if longest {
			// Find last occurrence (longest match)
			idx := strings.LastIndex(s, prefix)
			if idx != -1 {
				return s[idx+len(prefix):]
			}
		} else {
			// Find first occurrence (shortest match)
			idx := strings.Index(s, prefix)
			if idx != -1 {
				return s[idx+len(prefix):]
			}
		}
	} else {
		// Simple prefix
		if strings.HasPrefix(s, pattern) {
			return s[len(pattern):]
		}
	}
	return s
}

// expandSimpleVariables expands ${var} syntax without operators.
func expandSimpleVariables(s string, vars map[string]string) string {
	simplePattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

	result := s
	for {
		match := simplePattern.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		varName := result[match[2]:match[3]]
		varValue, exists := vars[varName]
		if !exists {
			varValue = ""
		}

		result = result[:match[0]] + varValue + result[match[1]:]
	}

	return result
}

// expandDollarVariables expands $VAR syntax (without braces).
func expandDollarVariables(s string, vars map[string]string) string {
	// Match $VAR followed by non-alphanumeric or end of string
	dollarPattern := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)(?:[^A-Za-z0-9_]|$)`)

	result := s
	offset := 0

	for {
		match := dollarPattern.FindStringSubmatchIndex(result[offset:])
		if match == nil {
			break
		}

		// Adjust for offset
		start := offset + match[0]
		varNameStart := offset + match[2]
		varNameEnd := offset + match[3]

		varName := result[varNameStart:varNameEnd]
		varValue, exists := vars[varName]
		if !exists {
			varValue = ""
		}

		// Replace only $VAR part, keep the trailing character
		dollarVarLen := 1 + len(varName) // $ + varname
		result = result[:start] + varValue + result[start+dollarVarLen:]

		// Move offset past the replacement
		offset = start + len(varValue)
	}

	return result
}
