// Package pkg provides domain models for package management.
package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path traversal error types for security validation.
var (
	ErrInvalidCategoryFormat = fmt.Errorf("invalid category format")
	ErrInvalidPackageFormat  = fmt.Errorf("invalid package name format")
	ErrPathTraversal         = fmt.Errorf("path traversal detected")
)

// ValidateCategoryPackageName validates both category and package name
// to prevent path traversal attacks.
//
// Returns nil if both are valid, or an error describing the issue.
// This is the primary security gate for user-provided package names.
//
// Example:
//
//	if err := pkg.ValidateCategoryPackageName("sys-libs", "zlib"); err != nil {
//	    return err  // Invalid input, don't construct path
//	}
//
// See: https://github.com/grpmsoft/grpm/issues/36
func ValidateCategoryPackageName(category, pkgName string) error {
	if !IsValidCategory(category) {
		return fmt.Errorf("%w: category %q does not match PMS format", ErrInvalidCategoryFormat, category)
	}

	if !IsValidPackageName(pkgName) {
		return fmt.Errorf("%w: package %q does not match PMS format", ErrInvalidPackageFormat, pkgName)
	}

	return nil
}

// ValidatePathContainment ensures that resolvedPath stays within basePath.
// This is a defense-in-depth measure that catches path traversal even if
// input validation is bypassed.
//
// Returns nil if the resolved path is contained within the base path.
// Returns ErrPathTraversal if the path escapes the base directory.
//
// Example:
//
//	pkgDir := filepath.Join(repoRoot, category, pkgName)
//	if err := pkg.ValidatePathContainment(repoRoot, pkgDir); err != nil {
//	    return err  // Path escapes repository root
//	}
//
// See: https://github.com/grpmsoft/grpm/issues/36
func ValidatePathContainment(basePath, resolvedPath string) error {
	cleanBase := filepath.Clean(basePath)
	cleanResolved := filepath.Clean(resolvedPath)

	// Ensure the resolved path starts with the base path
	// We add os.PathSeparator to prevent prefix matching of similar names
	// e.g., /var/db/pkg should not match /var/db/pkgother
	if !strings.HasPrefix(cleanResolved, cleanBase+string(os.PathSeparator)) &&
		cleanResolved != cleanBase {
		return fmt.Errorf("%w: path %q escapes base directory %q", ErrPathTraversal, resolvedPath, basePath)
	}

	return nil
}

// SafeJoinPath joins paths safely with path traversal protection.
// Returns the joined path and nil if safe, or empty string and error if unsafe.
//
// This combines filepath.Join with containment validation for convenience.
//
// Example:
//
//	pkgDir, err := pkg.SafeJoinPath(repoRoot, category, pkgName)
//	if err != nil {
//	    return err  // Path traversal attempt detected
//	}
//
// See: https://github.com/grpmsoft/grpm/issues/36
func SafeJoinPath(basePath string, elems ...string) (string, error) {
	joined := filepath.Join(append([]string{basePath}, elems...)...)

	if err := ValidatePathContainment(basePath, joined); err != nil {
		return "", err
	}

	return joined, nil
}
