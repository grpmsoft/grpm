package pkg

import (
	"fmt"
	"strings"
)

// RequiredUseError represents an error in REQUIRED_USE validation.
type RequiredUseError struct {
	Expression string
	Reason     string
}

func (e *RequiredUseError) Error() string {
	return fmt.Sprintf("REQUIRED_USE violation: %s (expression: %s)", e.Reason, e.Expression)
}

// RequiredUseValidator validates REQUIRED_USE expressions against enabled USE flags.
// Implements PMS Section 7.2.6 REQUIRED_USE syntax:
//   - flag: flag must be enabled
//   - !flag: flag must be disabled
//   - flag1? ( expr ): if flag1 is enabled, expr must be satisfied
//   - !flag1? ( expr ): if flag1 is disabled, expr must be satisfied
//   - || ( expr1 expr2 ... ): at least one of the expressions must be satisfied
//   - ^^ ( expr1 expr2 ... ): exactly one of the expressions must be satisfied
//   - ?? ( expr1 expr2 ... ): at most one of the expressions may be satisfied
type RequiredUseValidator struct {
	activeFlags map[string]bool
}

// NewRequiredUseValidator creates a new REQUIRED_USE validator.
func NewRequiredUseValidator() *RequiredUseValidator {
	return &RequiredUseValidator{
		activeFlags: make(map[string]bool),
	}
}

// Validate checks if the enabled USE flags satisfy the REQUIRED_USE expression.
// Returns nil if validation passes, or an error describing the violation.
func (v *RequiredUseValidator) Validate(expr string, enabledFlags []string) error {
	if expr == "" {
		return nil
	}

	// Build active flags map
	v.activeFlags = make(map[string]bool)
	for _, flag := range enabledFlags {
		v.activeFlags[flag] = true
	}

	// Tokenize and parse
	tokens := v.tokenize(expr)
	satisfied, _, err := v.parseAndEvaluate(tokens, 0)
	if err != nil {
		return err
	}

	if !satisfied {
		return &RequiredUseError{
			Expression: expr,
			Reason:     "USE flag constraints not satisfied",
		}
	}

	return nil
}

// isEnabled checks if a flag is enabled.
func (v *RequiredUseValidator) isEnabled(flag string) bool {
	return v.activeFlags[flag]
}

// tokenize splits REQUIRED_USE expression into tokens.
func (v *RequiredUseValidator) tokenize(expr string) []string {
	var tokens []string
	var current strings.Builder

	for _, ch := range expr {
		switch ch {
		case '(', ')':
			if current.Len() > 0 {
				tokens = append(tokens, strings.TrimSpace(current.String()))
				current.Reset()
			}
			tokens = append(tokens, string(ch))
		case ' ', '\t', '\n':
			if current.Len() > 0 {
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

// parseAndEvaluate recursively parses and evaluates REQUIRED_USE tokens.
// Returns (satisfied, nextIndex, error).
func (v *RequiredUseValidator) parseAndEvaluate(tokens []string, start int) (bool, int, error) {
	allSatisfied := true
	i := start

	for i < len(tokens) {
		token := tokens[i]

		if token == ")" {
			return allSatisfied, i + 1, nil
		}

		satisfied, nextIdx, err := v.evaluateToken(tokens, i)
		if err != nil {
			return false, 0, err
		}
		allSatisfied = allSatisfied && satisfied
		i = nextIdx
	}

	return allSatisfied, i, nil
}

// evaluateToken evaluates a single token and returns (satisfied, nextIndex, error).
func (v *RequiredUseValidator) evaluateToken(tokens []string, i int) (bool, int, error) {
	token := tokens[i]

	// Handle nested parenthesis
	if token == "(" {
		return v.parseAndEvaluate(tokens, i+1)
	}

	// Handle operators ||, ^^, ??
	if satisfied, nextIdx, handled, err := v.handleOperator(tokens, i); handled {
		return satisfied, nextIdx, err
	}

	// Handle conditional: flag? ( ... ) or !flag? ( ... )
	if strings.HasSuffix(token, "?") {
		return v.handleConditionalInParse(tokens, i)
	}

	// Handle simple flag or !flag
	return v.evaluateSimpleFlag(token), i + 1, nil
}

// handleOperator handles ||, ^^, ?? operators.
// Returns (satisfied, nextIndex, handled, error).
func (v *RequiredUseValidator) handleOperator(tokens []string, i int) (bool, int, bool, error) {
	token := tokens[i]

	type operatorHandler func([]string, int) (bool, int, error)
	operators := map[string]operatorHandler{
		"||": v.evaluateAnyOf,
		"^^": v.evaluateExactlyOne,
		"??": v.evaluateAtMostOne,
	}

	handler, isOperator := operators[token]
	if !isOperator {
		return false, 0, false, nil
	}

	if i+1 >= len(tokens) || tokens[i+1] != "(" {
		return false, 0, true, fmt.Errorf("%s operator must be followed by (", token)
	}

	satisfied, nextIdx, err := handler(tokens, i+2)
	return satisfied, nextIdx, true, err
}

// handleConditionalInParse handles flag? ( ... ) or !flag? ( ... ) in parseAndEvaluate.
func (v *RequiredUseValidator) handleConditionalInParse(tokens []string, i int) (bool, int, error) {
	token := tokens[i]
	flagExpr := strings.TrimSuffix(token, "?")
	negated := strings.HasPrefix(flagExpr, "!")
	if negated {
		flagExpr = strings.TrimPrefix(flagExpr, "!")
	}

	conditionMet := v.isEnabled(flagExpr)
	if negated {
		conditionMet = !conditionMet
	}

	if i+1 >= len(tokens) || tokens[i+1] != "(" {
		return false, 0, fmt.Errorf("conditional %s must be followed by (", token)
	}

	if conditionMet {
		return v.parseAndEvaluate(tokens, i+2)
	}

	// Skip the inner expression - condition not met
	nextIdx, err := v.skipGroup(tokens, i+2)
	return true, nextIdx, err
}

// evaluateAnyOf evaluates || ( ... ) - at least one must be satisfied.
func (v *RequiredUseValidator) evaluateAnyOf(tokens []string, start int) (bool, int, error) {
	satisfiedCount := 0
	i := start

	for i < len(tokens) {
		token := tokens[i]

		if token == ")" {
			return satisfiedCount > 0, i + 1, nil
		}

		if token == "(" {
			// Nested group
			satisfied, nextIdx, err := v.parseAndEvaluate(tokens, i+1)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Handle conditional in OR group
		if strings.HasSuffix(token, "?") {
			satisfied, nextIdx, err := v.evaluateConditional(tokens, i)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Simple flag
		if v.evaluateSimpleFlag(token) {
			satisfiedCount++
		}
		i++
	}

	return satisfiedCount > 0, i, nil
}

// evaluateExactlyOne evaluates ^^ ( ... ) - exactly one must be satisfied.
func (v *RequiredUseValidator) evaluateExactlyOne(tokens []string, start int) (bool, int, error) {
	satisfiedCount := 0
	i := start

	for i < len(tokens) {
		token := tokens[i]

		if token == ")" {
			return satisfiedCount == 1, i + 1, nil
		}

		if token == "(" {
			// Nested group
			satisfied, nextIdx, err := v.parseAndEvaluate(tokens, i+1)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Handle conditional in XOR group
		if strings.HasSuffix(token, "?") {
			satisfied, nextIdx, err := v.evaluateConditional(tokens, i)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Simple flag
		if v.evaluateSimpleFlag(token) {
			satisfiedCount++
		}
		i++
	}

	return satisfiedCount == 1, i, nil
}

// evaluateAtMostOne evaluates ?? ( ... ) - at most one may be satisfied.
func (v *RequiredUseValidator) evaluateAtMostOne(tokens []string, start int) (bool, int, error) {
	satisfiedCount := 0
	i := start

	for i < len(tokens) {
		token := tokens[i]

		if token == ")" {
			return satisfiedCount <= 1, i + 1, nil
		}

		if token == "(" {
			// Nested group
			satisfied, nextIdx, err := v.parseAndEvaluate(tokens, i+1)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Handle conditional in at-most-one group
		if strings.HasSuffix(token, "?") {
			satisfied, nextIdx, err := v.evaluateConditional(tokens, i)
			if err != nil {
				return false, 0, err
			}
			if satisfied {
				satisfiedCount++
			}
			i = nextIdx
			continue
		}

		// Simple flag
		if v.evaluateSimpleFlag(token) {
			satisfiedCount++
		}
		i++
	}

	return satisfiedCount <= 1, i, nil
}

// evaluateConditional evaluates a conditional expression: flag? ( ... ) or !flag? ( ... )
func (v *RequiredUseValidator) evaluateConditional(tokens []string, start int) (bool, int, error) {
	token := tokens[start]
	flagExpr := strings.TrimSuffix(token, "?")
	negated := strings.HasPrefix(flagExpr, "!")
	if negated {
		flagExpr = strings.TrimPrefix(flagExpr, "!")
	}

	// Check if condition applies
	conditionMet := v.isEnabled(flagExpr)
	if negated {
		conditionMet = !conditionMet
	}

	// Must be followed by ( ... )
	if start+1 >= len(tokens) || tokens[start+1] != "(" {
		return false, 0, fmt.Errorf("conditional %s must be followed by (", token)
	}

	if conditionMet {
		// Evaluate the inner expression
		return v.parseAndEvaluate(tokens, start+2)
	}

	// Skip the inner expression - condition not met, so this branch is satisfied
	nextIdx, err := v.skipGroup(tokens, start+2)
	if err != nil {
		return false, 0, err
	}
	return true, nextIdx, nil
}

// evaluateSimpleFlag evaluates a simple flag or !flag expression.
func (v *RequiredUseValidator) evaluateSimpleFlag(token string) bool {
	if strings.HasPrefix(token, "!") {
		// !flag: satisfied if flag is disabled
		flag := strings.TrimPrefix(token, "!")
		return !v.isEnabled(flag)
	}
	// flag: satisfied if flag is enabled
	return v.isEnabled(token)
}

// skipGroup skips over a parenthesized group without evaluation.
func (v *RequiredUseValidator) skipGroup(tokens []string, start int) (int, error) {
	depth := 1
	i := start

	for i < len(tokens) && depth > 0 {
		switch tokens[i] {
		case "(":
			depth++
		case ")":
			depth--
		}
		i++
	}

	if depth != 0 {
		return 0, fmt.Errorf("unbalanced parentheses in REQUIRED_USE")
	}

	return i, nil
}

// ValidateRequiredUse is a convenience function for validating REQUIRED_USE expressions.
// It creates a validator and validates the expression against the given flags.
func ValidateRequiredUse(expr string, enabledFlags []string) error {
	validator := NewRequiredUseValidator()
	return validator.Validate(expr, enabledFlags)
}
