package cli

import (
	"errors"
	"fmt"
	"strings"
)

// UserError wraps technical errors with user-friendly messages.
//
// It provides:
//   - A human-readable message describing the problem
//   - Suggestions for resolution (similar packages, actions to take)
//   - The original technical error for debugging
type UserError struct {
	// Message is the user-friendly error message
	Message string

	// Suggestions are actionable suggestions (similar packages, commands, etc.)
	Suggestions []string

	// Technical is the original technical error (for debugging)
	Technical error
}

// Error implements the error interface.
func (e *UserError) Error() string {
	var sb strings.Builder

	sb.WriteString("Error: ")
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	if len(e.Suggestions) > 0 {
		sb.WriteString("\n")
		// Check if first suggestion looks like a package name
		if strings.Contains(e.Suggestions[0], "/") {
			sb.WriteString("Did you mean one of these?\n")
			for _, s := range e.Suggestions {
				sb.WriteString("  - ")
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		} else {
			// Action suggestions
			for _, s := range e.Suggestions {
				sb.WriteString("  ")
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
	}

	if e.Technical != nil {
		sb.WriteString("\n  Technical: ")
		sb.WriteString(e.Technical.Error())
		sb.WriteString("\n")
	}

	return sb.String()
}

// Unwrap returns the underlying technical error.
func (e *UserError) Unwrap() error {
	return e.Technical
}

// WrapPackageNotFound creates a user-friendly error for missing packages.
//
// It searches for similar package names and provides suggestions.
func WrapPackageNotFound(atom string, similar []string, technicalErr error) error {
	msg := fmt.Sprintf("Package '%s' not found in repository.", atom)

	suggestions := similar
	if len(suggestions) == 0 {
		suggestions = []string{
			"Use 'grpm search <term>' to find packages.",
		}
	}

	return &UserError{
		Message:     msg,
		Suggestions: suggestions,
		Technical:   technicalErr,
	}
}

// WrapVersionNotFound creates a user-friendly error for missing versions.
func WrapVersionNotFound(atom, requestedVersion string, availableVersions []string, technicalErr error) error {
	msg := fmt.Sprintf("Version '%s-%s' not found.", atom, requestedVersion)

	var suggestions []string
	if len(availableVersions) > 0 {
		// Show up to 5 available versions
		limit := 5
		if len(availableVersions) < limit {
			limit = len(availableVersions)
		}
		versions := strings.Join(availableVersions[:limit], ", ")
		suggestions = append(suggestions, fmt.Sprintf("Available versions: %s", versions))
	}
	suggestions = append(suggestions, fmt.Sprintf("Use 'grpm info %s' to see all versions.", atom))

	return &UserError{
		Message:     msg,
		Suggestions: suggestions,
		Technical:   technicalErr,
	}
}

// WrapMaskedPackage creates a user-friendly error for masked packages.
func WrapMaskedPackage(atom, reason string, technicalErr error) error {
	msg := fmt.Sprintf("Package '%s' is masked.", atom)
	if reason != "" {
		msg = fmt.Sprintf("Package '%s' is masked: %s", atom, reason)
	}

	return &UserError{
		Message: msg,
		Suggestions: []string{
			"Check /etc/portage/package.mask for mask reason.",
			fmt.Sprintf("Use 'grpm info %s' for package information.", atom),
		},
		Technical: technicalErr,
	}
}

// WrapMissingDependency creates a user-friendly error for unresolvable dependencies.
func WrapMissingDependency(pkg, missingDep string, technicalErr error) error {
	msg := fmt.Sprintf("Cannot resolve '%s': requires '%s' which is not available.", pkg, missingDep)

	return &UserError{
		Message: msg,
		Suggestions: []string{
			fmt.Sprintf("Use 'grpm search %s' to find the dependency.", extractPackageName(missingDep)),
			"The dependency may be masked or unkeyworded.",
		},
		Technical: technicalErr,
	}
}

// WrapNetworkError creates a user-friendly error for network issues.
func WrapNetworkError(operation string, technicalErr error) error {
	msg := fmt.Sprintf("Network error during %s.", operation)

	return &UserError{
		Message: msg,
		Suggestions: []string{
			"Check your internet connection.",
			"Verify proxy settings if applicable.",
			"Try again later if the server may be temporarily unavailable.",
		},
		Technical: technicalErr,
	}
}

// WrapResolutionError creates a user-friendly error for dependency resolution failures.
func WrapResolutionError(pkg string, technicalErr error) error {
	// Extract useful info from the technical error
	msg := fmt.Sprintf("Failed to resolve dependencies for '%s'.", pkg)

	// Check for common patterns in the technical error
	techMsg := ""
	if technicalErr != nil {
		techMsg = technicalErr.Error()
	}

	var suggestions []string

	if strings.Contains(techMsg, "no such file or directory") {
		suggestions = append(suggestions, "The package may not exist. Use 'grpm search' to find it.")
	} else if strings.Contains(techMsg, "masked") {
		suggestions = append(suggestions, "Some dependencies may be masked. Check package.mask.")
	} else if strings.Contains(techMsg, "keyword") {
		suggestions = append(suggestions, "Some dependencies may be unkeyworded for your architecture.")
	} else {
		suggestions = append(suggestions, "Use 'grpm resolve --verbose' for more details.")
	}

	return &UserError{
		Message:     msg,
		Suggestions: suggestions,
		Technical:   technicalErr,
	}
}

// IsUserError checks if an error is a UserError.
// Uses errors.As to properly handle wrapped errors.
func IsUserError(err error) bool {
	var userErr *UserError
	return errors.As(err, &userErr)
}

// extractPackageName extracts the package name from an atom.
// Example: ">=sys-libs/glibc-2.0" -> "glibc"
func extractPackageName(atom string) string {
	// Remove version operator prefix
	atom = strings.TrimLeft(atom, ">=<~!")

	// Split by /
	parts := strings.Split(atom, "/")
	if len(parts) != 2 {
		return atom
	}

	pkgPart := parts[1]

	// Remove version suffix (find last hyphen before digit)
	for i := len(pkgPart) - 1; i >= 0; i-- {
		if pkgPart[i] == '-' && i+1 < len(pkgPart) {
			if pkgPart[i+1] >= '0' && pkgPart[i+1] <= '9' {
				return pkgPart[:i]
			}
		}
	}

	return pkgPart
}
