package pkg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coregx/coregex"
)

// Precompiled regex patterns for performance (compiled once at package init).
var (
	// versionMainRe extracts the main version components before any suffix.
	// Matches: numeric.numeric.numeric[letter]
	// Example: "1.2.3a" from "1.2.3a_alpha1-r2"
	versionMainRe = coregex.MustCompile(`^(\d+(?:\.\d+)*)([a-z])?`)

	// versionSuffixRe extracts suffixes like _alpha1, _beta2, _pre3, _rc4, _p5
	versionSuffixRe = coregex.MustCompile(`_(alpha|beta|pre|rc|p)(\d*)`)

	// versionRevisionRe extracts the revision number -rN
	versionRevisionRe = coregex.MustCompile(`-r(\d+)$`)

	// versionDigitRe validates that a version contains at least one digit.
	versionDigitRe = coregex.MustCompile(`^\d`)
)

// ErrEmptyVersion is returned when an empty version string is provided.
var ErrEmptyVersion = errors.New("version cannot be empty")

// errVersionInvalid is a local error for version parsing failures.
// Note: ErrInvalidVersion is defined in atom.go with package-level scope.
var errVersionInvalid = errors.New("invalid version format")

// suffixPriority defines the ordering of version suffixes per PMS Algorithm 3.5-3.6.
// _alpha < _beta < _pre < _rc < (no suffix) < _p
var suffixPriority = map[string]int{
	"alpha": -4,
	"beta":  -3,
	"pre":   -2,
	"rc":    -1,
	"":      0, // no suffix = release
	"p":     1, // patchlevel is AFTER release
}

// VersionComponent represents a single numeric component of a version.
// Per PMS Algorithm 3.3, components with leading zeros are compared specially.
type VersionComponent struct {
	Value          int    // Numeric value (for non-leading-zero comparison)
	HasLeadingZero bool   // True if component started with 0 and has multiple digits
	Original       string // Original string for leading zero comparison
}

// VersionSuffix represents a version suffix like _alpha1 or _beta2.
type VersionSuffix struct {
	Type   string // "alpha", "beta", "pre", "rc", "p"
	Number int    // suffix number (1 in _alpha1, 0 if no number)
}

// Version is a Value Object representing a Gentoo package version.
// It implements PMS Chapter 3.2-3.3 version comparison algorithm.
// The struct is immutable and self-validating.
type Version struct {
	raw          string             // Original version string
	components   []VersionComponent // Numeric parts (1.2.3 -> [1, 2, 3])
	letterSuffix string             // Single letter suffix "a"-"z" or ""
	suffixes     []VersionSuffix    // _alpha1, _beta2, etc.
	revision     int                // -r1, -r2, etc. (0 means no revision)
}

// NewVersion creates a new Version instance with validation.
// Parses version according to PMS Chapter 3.2 format:
// [0-9]+(\.[0-9]+)*[a-z]?(_suffix[0-9]*)*(-r[0-9]+)?
func NewVersion(versionStr string) (Version, error) {
	if versionStr == "" {
		return Version{}, ErrEmptyVersion
	}

	// Parse the version string
	v, err := parseVersion(versionStr)
	if err != nil {
		return Version{}, fmt.Errorf("%w: %s", errVersionInvalid, versionStr)
	}

	return v, nil
}

// MustNewVersion creates a Version or panics if invalid (use in tests/constants).
func MustNewVersion(versionStr string) Version {
	v, err := NewVersion(versionStr)
	if err != nil {
		panic(fmt.Sprintf("invalid version: %v", err))
	}
	return v
}

// String returns the string representation of the version.
func (v Version) String() string {
	return v.raw
}

// Equals checks if two versions are equal (Value Object equality).
func (v Version) Equals(other Version) bool {
	return v.raw == other.raw
}

// CompareTo compares this version with another per PMS Algorithm 3.2.
// Returns: <0 if v<other, 0 if v==other, >0 if v>other
func (v Version) CompareTo(other Version) int {
	return compareVersions(v, other)
}

// IsGreaterThan checks if this version is greater than another.
func (v Version) IsGreaterThan(other Version) bool {
	return v.CompareTo(other) > 0
}

// IsLessThan checks if this version is less than another.
func (v Version) IsLessThan(other Version) bool {
	return v.CompareTo(other) < 0
}

// IsGreaterThanOrEqual checks if this version is >= another.
func (v Version) IsGreaterThanOrEqual(other Version) bool {
	return v.CompareTo(other) >= 0
}

// IsLessThanOrEqual checks if this version is <= another.
func (v Version) IsLessThanOrEqual(other Version) bool {
	return v.CompareTo(other) <= 0
}

// parseVersion parses a Gentoo version string into structured components.
func parseVersion(s string) (Version, error) {
	if s == "" {
		return Version{}, errors.New("empty version")
	}

	// Must start with a digit
	if !versionDigitRe.MatchString(s) {
		return Version{}, errors.New("version must start with digit")
	}

	v := Version{raw: s}
	remaining := s

	// Step 1: Extract revision (-rN) from the end
	if revMatch := versionRevisionRe.FindStringSubmatch(remaining); revMatch != nil {
		revNum, err := strconv.Atoi(revMatch[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid revision: %s", revMatch[1])
		}
		v.revision = revNum
		remaining = remaining[:len(remaining)-len(revMatch[0])]
	}

	// Step 2: Extract suffixes (_alpha1, _beta2, etc.)
	suffixMatches := versionSuffixRe.FindAllStringSubmatchIndex(remaining, -1)
	if len(suffixMatches) > 0 {
		// Find where suffixes start
		suffixStart := suffixMatches[0][0]
		suffixPart := remaining[suffixStart:]
		remaining = remaining[:suffixStart]

		// Parse each suffix
		for _, match := range versionSuffixRe.FindAllStringSubmatch(suffixPart, -1) {
			suffix := VersionSuffix{Type: match[1]}
			if match[2] != "" {
				num, err := strconv.Atoi(match[2])
				if err != nil {
					return Version{}, fmt.Errorf("invalid suffix number: %s", match[2])
				}
				suffix.Number = num
			}
			v.suffixes = append(v.suffixes, suffix)
		}
	}

	// Step 3: Extract main version and letter suffix
	mainMatch := versionMainRe.FindStringSubmatch(remaining)
	if mainMatch == nil {
		return Version{}, errors.New("invalid main version format")
	}

	// Parse numeric components
	numericPart := mainMatch[1]
	parts := strings.Split(numericPart, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}
		comp := VersionComponent{
			Original:       part,
			HasLeadingZero: len(part) > 1 && part[0] == '0',
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("invalid numeric component: %s", part)
		}
		comp.Value = num
		v.components = append(v.components, comp)
	}

	// Extract letter suffix if present
	if len(mainMatch) > 2 && mainMatch[2] != "" {
		v.letterSuffix = mainMatch[2]
	}

	if len(v.components) == 0 {
		return Version{}, errors.New("no numeric components found")
	}

	return v, nil
}

// compareVersions implements PMS Algorithm 3.2 for version comparison.
func compareVersions(v1, v2 Version) int {
	// Step 1: Compare numeric components
	maxLen := len(v1.components)
	if len(v2.components) > maxLen {
		maxLen = len(v2.components)
	}

	for i := 0; i < maxLen; i++ {
		var c1, c2 VersionComponent

		if i < len(v1.components) {
			c1 = v1.components[i]
		} else {
			// Missing component treated as 0
			c1 = VersionComponent{Value: 0, Original: "0"}
		}

		if i < len(v2.components) {
			c2 = v2.components[i]
		} else {
			// Missing component treated as 0
			c2 = VersionComponent{Value: 0, Original: "0"}
		}

		cmp := compareComponents(c1, c2)
		if cmp != 0 {
			return cmp
		}
	}

	// Step 2: Compare letter suffix (PMS Algorithm 3.4)
	// No letter > letter (e.g., 1.0 > 1.0z > 1.0a)
	cmp := compareLetterSuffix(v1.letterSuffix, v2.letterSuffix)
	if cmp != 0 {
		return cmp
	}

	// Step 3: Compare suffixes (_alpha, _beta, _pre, _rc, _p)
	cmp = compareSuffixes(v1.suffixes, v2.suffixes)
	if cmp != 0 {
		return cmp
	}

	// Step 4: Compare revisions
	return v1.revision - v2.revision
}

// compareComponents compares two version components per PMS Algorithm 3.3.
func compareComponents(c1, c2 VersionComponent) int {
	// If either has leading zeros, use special comparison
	if c1.HasLeadingZero || c2.HasLeadingZero {
		return compareWithLeadingZeros(c1.Original, c2.Original)
	}

	// Standard numeric comparison
	return c1.Value - c2.Value
}

// compareWithLeadingZeros implements PMS Algorithm 3.3 for leading zero comparison.
// 1. Strip trailing zeros from both strings
// 2. Compare lexicographically (ASCII)
func compareWithLeadingZeros(s1, s2 string) int {
	// Strip trailing zeros
	s1 = strings.TrimRight(s1, "0")
	s2 = strings.TrimRight(s2, "0")

	// Handle empty results (all zeros)
	if s1 == "" {
		s1 = "0"
	}
	if s2 == "" {
		s2 = "0"
	}

	// Lexicographic comparison
	return strings.Compare(s1, s2)
}

// compareLetterSuffix compares letter suffixes per PMS Algorithm 3.4.
// No letter > any letter, and letters compare alphabetically.
func compareLetterSuffix(l1, l2 string) int {
	// Both have no letter - equal
	if l1 == "" && l2 == "" {
		return 0
	}
	// No letter > letter
	if l1 == "" && l2 != "" {
		return 1
	}
	if l1 != "" && l2 == "" {
		return -1
	}
	// Both have letters - compare alphabetically
	return strings.Compare(l1, l2)
}

// compareSuffixes compares version suffixes per PMS Algorithm 3.5-3.6.
// Order: _alpha < _beta < _pre < _rc < (no suffix) < _p
func compareSuffixes(s1, s2 []VersionSuffix) int {
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	// If no suffixes on either side, they're equal
	if maxLen == 0 {
		return 0
	}

	for i := 0; i < maxLen; i++ {
		var suf1, suf2 VersionSuffix

		if i < len(s1) {
			suf1 = s1[i]
		} else {
			// No more suffixes means release version (priority 0)
			suf1 = VersionSuffix{Type: "", Number: 0}
		}

		if i < len(s2) {
			suf2 = s2[i]
		} else {
			// No more suffixes means release version (priority 0)
			suf2 = VersionSuffix{Type: "", Number: 0}
		}

		// Compare by suffix type priority
		p1 := suffixPriority[suf1.Type]
		p2 := suffixPriority[suf2.Type]

		if p1 != p2 {
			return p1 - p2
		}

		// Same type - compare by number
		if suf1.Number != suf2.Number {
			return suf1.Number - suf2.Number
		}
	}

	return 0
}

// legacyParseComponents provides fallback parsing for backward compatibility.
func legacyParseComponents(v string) []interface{} {
	// Use simple regex to extract components
	re := coregex.MustCompile(`(\d+)|([a-zA-Z]+)`)
	parts := re.FindAllString(v, -1)

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
