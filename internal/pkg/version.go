package pkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coregx/coregex"
)

// Precompiled regex patterns for performance (compiled once at package init).
var (
	// versionComponentsRe extracts numeric and alphabetic components from version strings.
	// Example: "1.2.3_alpha4" -> ["1", "2", "3", "alpha", "4"]
	versionComponentsRe = coregex.MustCompile(`(\d+)|([a-zA-Z]+)`)

	// versionDigitRe validates that a version contains at least one digit.
	versionDigitRe = coregex.MustCompile(`\d`)
)

// Version is a Value Object representing a Gentoo package version
// It is immutable and self-validating
type Version struct {
	raw        string
	components []interface{} // Parsed version components
}

// NewVersion creates a new Version instance with validation
func NewVersion(versionStr string) (Version, error) {
	if versionStr == "" {
		return Version{}, fmt.Errorf("version cannot be empty")
	}

	// Validate version format (basic check)
	if !isValidVersionFormat(versionStr) {
		return Version{}, fmt.Errorf("invalid version format: %s", versionStr)
	}

	components := parseVersionComponents(versionStr)

	return Version{
		raw:        versionStr,
		components: components,
	}, nil
}

// MustNewVersion creates a Version or panics if invalid (use in tests/constants)
func MustNewVersion(versionStr string) Version {
	v, err := NewVersion(versionStr)
	if err != nil {
		panic(fmt.Sprintf("invalid version: %v", err))
	}
	return v
}

// String returns the string representation of the version
func (v Version) String() string {
	return v.raw
}

// Equals checks if two versions are equal (Value Object equality)
func (v Version) Equals(other Version) bool {
	return v.raw == other.raw
}

// CompareTo compares this version with another
// Returns: <0 if v<other, 0 if v==other, >0 if v>other
func (v Version) CompareTo(other Version) int {
	return compareVersionComponents(v.components, other.components)
}

// IsGreaterThan checks if this version is greater than another
func (v Version) IsGreaterThan(other Version) bool {
	return v.CompareTo(other) > 0
}

// IsLessThan checks if this version is less than another
func (v Version) IsLessThan(other Version) bool {
	return v.CompareTo(other) < 0
}

// IsGreaterThanOrEqual checks if this version is >= another
func (v Version) IsGreaterThanOrEqual(other Version) bool {
	return v.CompareTo(other) >= 0
}

// IsLessThanOrEqual checks if this version is <= another
func (v Version) IsLessThanOrEqual(other Version) bool {
	return v.CompareTo(other) <= 0
}

// parseVersionComponents splits version into components: 1.2.3_alpha4-r5 -> [1, 2, 3, "alpha", 4, 5]
func parseVersionComponents(v string) []interface{} {
	// Separate digits and letters using precompiled regex
	parts := versionComponentsRe.FindAllString(v, -1)

	var result []interface{}
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			result = append(result, num)
		} else {
			result = append(result, part)
		}
	}
	return result
}

// compareVersionComponents compares two parsed version component slices
func compareVersionComponents(v1, v2 []interface{}) int {
	// Compare components
	for i := 0; i < len(v1) && i < len(v2); i++ {
		switch a := v1[i].(type) {
		case int:
			if b, ok := v2[i].(int); ok {
				if a != b {
					return a - b
				}
			} else {
				// Numbers are always greater than strings
				return 1
			}
		case string:
			if b, ok := v2[i].(string); ok {
				if cmp := strings.Compare(a, b); cmp != 0 {
					return cmp
				}
			} else {
				// Strings are always less than numbers
				return -1
			}
		}
	}

	// If all components are equal, the longer version is considered greater
	return len(v1) - len(v2)
}

// isValidVersionFormat performs basic validation of Gentoo version format
func isValidVersionFormat(v string) bool {
	// Basic validation: must contain at least one digit
	return versionDigitRe.MatchString(v)
}
