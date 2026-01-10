// Package ebuild implements ebuild execution engine.
//
// This file provides flag-o-matic.eclass support for safe CFLAGS/LDFLAGS manipulation.
// The flag-o-matic eclass is used by ~40% of packages for compiler flag management.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/flag-o-matic.eclass/
// PMS Reference: https://projects.gentoo.org/pms/latest/pms.html#x1-11500011
package ebuild

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Flag-o-matic Eclass Registration
// ============================================================================

// FlagOMaticEclass represents the flag-o-matic.eclass implementation.
//
// This eclass provides:
//   - append-flags, append-cflags, append-cxxflags, append-cppflags, append-ldflags
//   - filter-flags, filter-ldflags, filter-lfs-flags
//   - replace-flags, replace-cpu-flags
//   - strip-flags, strip-unsupported-flags
//   - test-flags-CC, test-flags-CXX, test-flags-F77, test-flags-FC, test-flags
//   - get-flag, is-flag, is-ldflag
//   - no-as-needed, raw-ldflags
//   - append-lfs-flags
type FlagOMaticEclass struct{}

// Name returns the eclass name.
func (e *FlagOMaticEclass) Name() string {
	return "flag-o-matic"
}

// ExportedFunctions returns empty slice (no phase functions exported).
func (e *FlagOMaticEclass) ExportedFunctions() []string {
	return nil
}

// Variables returns default variables (none for flag-o-matic).
func (e *FlagOMaticEclass) Variables() map[string]string {
	return nil
}

// ============================================================================
// FlagSet - Immutable Value Object for Flag Collections
// ============================================================================

// FlagSet represents an immutable collection of compiler flags.
// Thread-safe for concurrent access.
type FlagSet struct {
	flags []string
	mu    sync.RWMutex
}

// NewFlagSet creates a new FlagSet from a space-separated string.
func NewFlagSet(flagStr string) *FlagSet {
	flags := strings.Fields(flagStr)
	return &FlagSet{flags: flags}
}

// NewFlagSetFromSlice creates a new FlagSet from a slice of flags.
func NewFlagSetFromSlice(flags []string) *FlagSet {
	// Copy to avoid external modification
	copied := make([]string, len(flags))
	copy(copied, flags)
	return &FlagSet{flags: copied}
}

// String returns the flags as a space-separated string.
func (f *FlagSet) String() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return strings.Join(f.flags, " ")
}

// Flags returns a copy of the flag slice.
func (f *FlagSet) Flags() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]string, len(f.flags))
	copy(result, f.flags)
	return result
}

// Len returns the number of flags.
func (f *FlagSet) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.flags)
}

// IsEmpty returns true if the flag set is empty.
func (f *FlagSet) IsEmpty() bool {
	return f.Len() == 0
}

// Contains checks if a flag exists in the set.
func (f *FlagSet) Contains(flag string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, existing := range f.flags {
		if existing == flag {
			return true
		}
	}
	return false
}

// ContainsPattern checks if any flag matches the pattern.
// Supports glob patterns: * (any chars), ? (single char).
func (f *FlagSet) ContainsPattern(pattern string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, flag := range f.flags {
		if matchGlobPattern(flag, pattern) {
			return true
		}
	}
	return false
}

// Append returns a new FlagSet with additional flags appended.
// This is an immutable operation - the original FlagSet is not modified.
func (f *FlagSet) Append(flags ...string) *FlagSet {
	f.mu.RLock()
	defer f.mu.RUnlock()
	newFlags := make([]string, len(f.flags)+len(flags))
	copy(newFlags, f.flags)
	copy(newFlags[len(f.flags):], flags)
	return &FlagSet{flags: newFlags}
}

// Filter returns a new FlagSet with matching flags removed.
// Patterns support glob: * (any chars), ? (single char).
func (f *FlagSet) Filter(patterns ...string) *FlagSet {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]string, 0, len(f.flags))
	for _, flag := range f.flags {
		matched := false
		for _, pattern := range patterns {
			if matchGlobPattern(flag, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			result = append(result, flag)
		}
	}
	return &FlagSet{flags: result}
}

// Replace returns a new FlagSet with old flags replaced by new.
// Pattern supports glob: * (any chars), ? (single char).
func (f *FlagSet) Replace(oldPattern, newFlag string) *FlagSet {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]string, 0, len(f.flags))
	for _, flag := range f.flags {
		if matchGlobPattern(flag, oldPattern) {
			if newFlag != "" {
				result = append(result, newFlag)
			}
		} else {
			result = append(result, flag)
		}
	}
	return &FlagSet{flags: result}
}

// GetFlag returns the value of a flag (e.g., "-O2" for pattern "-O*").
// Returns empty string if not found.
func (f *FlagSet) GetFlag(pattern string) string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, flag := range f.flags {
		if matchGlobPattern(flag, pattern) {
			return flag
		}
	}
	return ""
}

// StripToSafe returns a new FlagSet containing only safe flags.
// Safe flags are those matching the provided safe patterns.
func (f *FlagSet) StripToSafe(safePatterns []string) *FlagSet {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]string, 0, len(f.flags))
	for _, flag := range f.flags {
		for _, pattern := range safePatterns {
			if matchGlobPattern(flag, pattern) {
				result = append(result, flag)
				break
			}
		}
	}
	return &FlagSet{flags: result}
}

// ============================================================================
// Pattern Matching
// ============================================================================

// matchGlobPattern checks if a string matches a glob pattern.
// Supports * (matches any characters) and ? (matches single character).
func matchGlobPattern(s, pattern string) bool {
	// Convert glob to regex
	regexPattern := globToRegex(pattern)
	re, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		// Fall back to exact match on invalid pattern
		return s == pattern
	}
	return re.MatchString(s)
}

// globToRegex converts a glob pattern to a regex pattern.
func globToRegex(glob string) string {
	var result strings.Builder
	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			result.WriteByte('\\')
			result.WriteByte(glob[i])
		default:
			result.WriteByte(glob[i])
		}
	}
	return result.String()
}

// ============================================================================
// Safe Flag Patterns
// ============================================================================

// safeCFlagsPatterns defines flags considered safe for CFLAGS/CXXFLAGS.
// These are the flags that strip-flags will keep.
var safeCFlagsPatterns = []string{
	// Optimization levels
	"-O", "-O0", "-O1", "-O2", "-O3", "-Os", "-Oz", "-Og", "-Ofast",
	// Pipe
	"-pipe",
	// Debug info
	"-g", "-g0", "-g1", "-g2", "-g3", "-ggdb", "-ggdb0", "-ggdb1", "-ggdb2", "-ggdb3",
	// Architecture
	"-march=*", "-mtune=*", "-mcpu=*",
	// Position independent code
	"-fPIC", "-fPIE", "-fpic", "-fpie",
	// Stack protection
	"-fstack-protector", "-fstack-protector-strong", "-fstack-protector-all",
	// Fortify source
	"-D_FORTIFY_SOURCE=*",
	// Common standard flags
	"-std=*",
	// Visibility
	"-fvisibility=*", "-fvisibility-inlines-hidden",
	// Common warning flags
	"-Wall", "-Wextra", "-Werror", "-Wno-*", "-W*",
	// Include paths
	"-I*", "-isystem*",
	// Define macros
	"-D*",
	// Undefine macros
	"-U*",
}

// safeLDFlagsPatterns defines flags considered safe for LDFLAGS.
// Currently unused but reserved for future strip-ldflags implementation.
var _ = []string{
	// Linker flags
	"-Wl,*",
	// Library paths
	"-L*",
	// Libraries
	"-l*",
	// Rpath
	"-rpath*",
	// Position independent
	"-pie", "-shared", "-static",
	// Symbol related
	"-Bsymbolic", "-Bsymbolic-functions",
	// As-needed
	"--as-needed", "--no-as-needed", "-Wl,--as-needed", "-Wl,--no-as-needed",
	// Hash style
	"--hash-style=*", "-Wl,--hash-style=*",
	// Relro
	"-z,relro", "-z,now", "-Wl,-z,relro", "-Wl,-z,now",
}

// lfsFlags contains Large File Support flags.
var lfsFlags = []string{
	"-D_LARGEFILE_SOURCE",
	"-D_LARGEFILE64_SOURCE",
	"-D_FILE_OFFSET_BITS=64",
}

// ============================================================================
// Flag Append Functions
// ============================================================================

// AppendFlagsNew appends to both CFLAGS and CXXFLAGS (new implementation).
// This is the comprehensive implementation for flag-o-matic.eclass.
//
// Usage: append-flags -O2 -march=native
func (h *Helpers) AppendFlagsNew(args []string) error {
	if err := h.appendToEnvVar("CFLAGS", args); err != nil {
		return err
	}
	return h.appendToEnvVar("CXXFLAGS", args)
}

// AppendCppflags appends to CPPFLAGS (C preprocessor flags).
//
// Usage: append-cppflags -DFOO -DBAR=1
func (h *Helpers) AppendCppflags(args []string) error {
	return h.appendToEnvVar("CPPFLAGS", args)
}

// AppendLfsFlags appends Large File Support flags.
//
// Usage: append-lfs-flags
//
// Adds: -D_LARGEFILE_SOURCE -D_LARGEFILE64_SOURCE -D_FILE_OFFSET_BITS=64
func (h *Helpers) AppendLfsFlags(args []string) error {
	return h.AppendCppflags(lfsFlags)
}

// appendToEnvVar appends flags to an environment variable.
func (h *Helpers) appendToEnvVar(varName string, flags []string) error {
	if len(flags) == 0 {
		return nil
	}

	flagStr := strings.Join(flags, " ")

	// Get current value
	var current string
	switch varName {
	case "CFLAGS":
		current = h.getCFLAGS()
	case "CXXFLAGS":
		current = h.getCXXFLAGS()
	case "LDFLAGS":
		current = h.getLDFLAGS()
	case "CPPFLAGS":
		current = h.getCPPFLAGS()
	case "FFLAGS":
		current = h.getFFLAGS()
	case "FCFLAGS":
		current = h.getFCFLAGS()
	default:
		current = h.getEnvVar(varName)
	}

	// Append new flags
	var newValue string
	if current != "" {
		newValue = current + " " + flagStr
	} else {
		newValue = flagStr
	}

	// Set the new value
	h.setFlagVar(varName, newValue)

	return nil
}

// ============================================================================
// Flag Filter Functions
// ============================================================================

// FilterLfsFlags removes Large File Support flags.
//
// Usage: filter-lfs-flags
func (h *Helpers) FilterLfsFlags(args []string) error {
	// Filter from CFLAGS, CXXFLAGS, and CPPFLAGS
	if err := h.FilterFlags(lfsFlags); err != nil {
		return err
	}
	return h.filterFromEnvVar("CPPFLAGS", lfsFlags)
}

// filterFromEnvVar filters flags from a specific environment variable.
func (h *Helpers) filterFromEnvVar(varName string, patterns []string) error {
	var current string
	switch varName {
	case "CFLAGS":
		current = h.getCFLAGS()
	case "CXXFLAGS":
		current = h.getCXXFLAGS()
	case "LDFLAGS":
		current = h.getLDFLAGS()
	case "CPPFLAGS":
		current = h.getCPPFLAGS()
	default:
		current = h.getEnvVar(varName)
	}

	flagSet := NewFlagSet(current)
	filtered := flagSet.Filter(patterns...)
	h.setFlagVar(varName, filtered.String())

	return nil
}

// ============================================================================
// Flag Replacement Functions
// ============================================================================

// ReplaceFlagsImpl replaces old flags with new in CFLAGS/CXXFLAGS.
//
// Usage: replace-flags -O2 -O3
func (h *Helpers) ReplaceFlagsImpl(args []string) error {
	if len(args) < 2 {
		return nil
	}

	oldPattern := args[0]
	newFlag := args[1]

	// Replace in CFLAGS
	cflagsSet := NewFlagSet(h.getCFLAGS())
	h.setFlagVar("CFLAGS", cflagsSet.Replace(oldPattern, newFlag).String())

	// Replace in CXXFLAGS
	cxxflagsSet := NewFlagSet(h.getCXXFLAGS())
	h.setFlagVar("CXXFLAGS", cxxflagsSet.Replace(oldPattern, newFlag).String())

	return nil
}

// ReplaceCpuFlagsImpl replaces CPU-specific flags.
//
// Usage: replace-cpu-flags i686 pentium4
// Usage: replace-cpu-flags march=i686 march=pentium4
func (h *Helpers) ReplaceCpuFlagsImpl(args []string) error {
	if len(args) < 2 {
		return nil
	}

	oldCpu := args[0]
	newCpu := args[1]

	// Handle both "i686" and "march=i686" forms
	var oldPattern, newFlag string
	if strings.HasPrefix(oldCpu, "march=") {
		oldPattern = "-" + oldCpu
		newFlag = "-" + newCpu
	} else {
		// Replace in all CPU-related flags
		patterns := []string{"-march=" + oldCpu, "-mcpu=" + oldCpu, "-mtune=" + oldCpu}
		newFlags := []string{"-march=" + newCpu, "-mcpu=" + newCpu, "-mtune=" + newCpu}

		for i, pattern := range patterns {
			if err := h.ReplaceFlagsImpl([]string{pattern, newFlags[i]}); err != nil {
				return err
			}
		}
		return nil
	}

	return h.ReplaceFlagsImpl([]string{oldPattern, newFlag})
}

// ============================================================================
// Flag Stripping Functions
// ============================================================================

// StripFlagsImpl removes all non-safe flags from CFLAGS/CXXFLAGS.
//
// Usage: strip-flags
func (h *Helpers) StripFlagsImpl(args []string) error {
	// Strip CFLAGS
	cflagsSet := NewFlagSet(h.getCFLAGS())
	h.setFlagVar("CFLAGS", cflagsSet.StripToSafe(safeCFlagsPatterns).String())

	// Strip CXXFLAGS
	cxxflagsSet := NewFlagSet(h.getCXXFLAGS())
	h.setFlagVar("CXXFLAGS", cxxflagsSet.StripToSafe(safeCFlagsPatterns).String())

	return nil
}

// StripUnsupportedFlags removes flags not supported by the compiler.
//
// Usage: strip-unsupported-flags
func (h *Helpers) StripUnsupportedFlags(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Strip unsupported CFLAGS
	cflags := strings.Fields(h.getCFLAGS())
	supportedCFlags := make([]string, 0, len(cflags))
	for _, flag := range cflags {
		if h.testCompilerFlag(ctx, "CC", flag) {
			supportedCFlags = append(supportedCFlags, flag)
		}
	}
	h.setFlagVar("CFLAGS", strings.Join(supportedCFlags, " "))

	// Strip unsupported CXXFLAGS
	cxxflags := strings.Fields(h.getCXXFLAGS())
	supportedCXXFlags := make([]string, 0, len(cxxflags))
	for _, flag := range cxxflags {
		if h.testCompilerFlag(ctx, "CXX", flag) {
			supportedCXXFlags = append(supportedCXXFlags, flag)
		}
	}
	h.setFlagVar("CXXFLAGS", strings.Join(supportedCXXFlags, " "))

	return nil
}

// ============================================================================
// Compiler Flag Testing Functions
// ============================================================================

// TestFlagsCC tests if CC accepts the given flags.
//
// Usage: test-flags-CC -fstack-protector-strong && echo "supported"
//
// Returns exit code 0 if all flags are supported, 1 otherwise.
// Outputs the supported flags to stdout.
func (h *Helpers) TestFlagsCC(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	supported := h.testCompilerFlags(ctx, "CC", args)
	if len(supported) == 0 {
		return exitFalse()
	}

	h.writeStdout(strings.Join(supported, " "))
	return nil
}

// TestFlagsCXX tests if CXX accepts the given flags.
//
// Usage: test-flags-CXX -std=c++17 && echo "supported"
func (h *Helpers) TestFlagsCXX(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	supported := h.testCompilerFlags(ctx, "CXX", args)
	if len(supported) == 0 {
		return exitFalse()
	}

	h.writeStdout(strings.Join(supported, " "))
	return nil
}

// TestFlagsF77 tests if F77 (Fortran 77 compiler) accepts the given flags.
//
// Usage: test-flags-F77 -O2 && echo "supported"
func (h *Helpers) TestFlagsF77(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	supported := h.testCompilerFlags(ctx, "F77", args)
	if len(supported) == 0 {
		return exitFalse()
	}

	h.writeStdout(strings.Join(supported, " "))
	return nil
}

// TestFlagsFC tests if FC (Fortran compiler) accepts the given flags.
//
// Usage: test-flags-FC -O2 && echo "supported"
func (h *Helpers) TestFlagsFC(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	supported := h.testCompilerFlags(ctx, "FC", args)
	if len(supported) == 0 {
		return exitFalse()
	}

	h.writeStdout(strings.Join(supported, " "))
	return nil
}

// TestFlagsAll tests if all compilers (CC, CXX) accept the given flags.
//
// Usage: test-flags -O2 -fPIC && echo "all supported"
//
// Only outputs flags that are supported by all compilers.
func (h *Helpers) TestFlagsAll(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Test with CC
	ccSupported := h.testCompilerFlags(ctx, "CC", args)

	// Test with CXX
	cxxSupported := h.testCompilerFlags(ctx, "CXX", args)

	// Find intersection
	supported := intersectStrings(ccSupported, cxxSupported)
	if len(supported) == 0 {
		return exitFalse()
	}

	h.writeStdout(strings.Join(supported, " "))
	return nil
}

// testCompilerFlags tests multiple flags with a compiler.
func (h *Helpers) testCompilerFlags(ctx context.Context, compilerVar string, flags []string) []string {
	supported := make([]string, 0, len(flags))
	for _, flag := range flags {
		if h.testCompilerFlag(ctx, compilerVar, flag) {
			supported = append(supported, flag)
		}
	}
	return supported
}

// testCompilerFlag tests if a single flag is supported by a compiler.
func (h *Helpers) testCompilerFlag(ctx context.Context, compilerVar, flag string) bool {
	// Get compiler command
	var compiler string
	switch compilerVar {
	case "CC":
		compiler = h.getEnvOrDefault("CC", "gcc")
	case "CXX":
		compiler = h.getEnvOrDefault("CXX", "g++")
	case "F77":
		compiler = h.getEnvOrDefault("F77", "gfortran")
	case "FC":
		compiler = h.getEnvOrDefault("FC", "gfortran")
	default:
		compiler = h.getEnvVar(compilerVar)
		if compiler == "" {
			return false
		}
	}

	// Create temp file based on compiler type
	var testFile, ext string
	switch compilerVar {
	case "CXX":
		ext = ".cpp"
		testFile = "int main(){return 0;}"
	case "F77", "FC":
		ext = ".f90"
		testFile = "program test\nend program test"
	default:
		ext = ".c"
		testFile = "int main(){return 0;}"
	}

	// Create temp directory
	tmpDir := os.TempDir()
	testPath := filepath.Join(tmpDir, "grpm_flag_test"+ext)
	outPath := filepath.Join(tmpDir, "grpm_flag_test.o")

	// Write test file
	if err := os.WriteFile(testPath, []byte(testFile), 0644); err != nil {
		return false
	}
	defer func() {
		_ = os.Remove(testPath)
		_ = os.Remove(outPath)
	}()

	// Run compiler with flag
	cmd := exec.CommandContext(ctx, compiler, flag, "-c", testPath, "-o", outPath)
	cmd.Stderr = nil // Suppress error output
	cmd.Stdout = nil

	return cmd.Run() == nil
}

// intersectStrings returns the intersection of two string slices.
func intersectStrings(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}

	result := make([]string, 0)
	for _, s := range b {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}

// ============================================================================
// Flag Query Functions
// ============================================================================

// GetFlag outputs the current value of a flag matching the pattern.
//
// Usage: march=$(get-flag march)
// Usage: opt=$(get-flag -O*)
//
// Outputs the first matching flag from CFLAGS.
func (h *Helpers) GetFlag(args []string) error {
	if len(args) < 1 {
		return nil
	}

	pattern := args[0]

	// Handle "march" -> "-march=*" conversion
	if !strings.HasPrefix(pattern, "-") {
		pattern = "-" + pattern + "=*"
	}

	// Check CFLAGS
	cflagsSet := NewFlagSet(h.getCFLAGS())
	if flag := cflagsSet.GetFlag(pattern); flag != "" {
		// Extract value after = if present
		if idx := strings.Index(flag, "="); idx != -1 {
			h.writeStdout(flag[idx+1:])
		} else {
			h.writeStdout(flag)
		}
		return nil
	}

	// Not found - output nothing
	return nil
}

// IsFlag checks if a flag is currently set in CFLAGS/CXXFLAGS.
//
// Usage: is-flag -O2 && echo "optimization enabled"
//
// Returns exit code 0 if found, 1 otherwise.
func (h *Helpers) IsFlag(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	pattern := args[0]

	// Check CFLAGS
	cflagsSet := NewFlagSet(h.getCFLAGS())
	if cflagsSet.ContainsPattern(pattern) {
		return nil
	}

	// Check CXXFLAGS
	cxxflagsSet := NewFlagSet(h.getCXXFLAGS())
	if cxxflagsSet.ContainsPattern(pattern) {
		return nil
	}

	return exitFalse()
}

// IsLdflag checks if a flag is currently set in LDFLAGS.
//
// Usage: is-ldflag -Wl,--as-needed && echo "as-needed set"
//
// Returns exit code 0 if found, 1 otherwise.
func (h *Helpers) IsLdflag(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	pattern := args[0]
	ldflagsSet := NewFlagSet(h.getLDFLAGS())
	if ldflagsSet.ContainsPattern(pattern) {
		return nil
	}

	return exitFalse()
}

// ============================================================================
// Utility Functions
// ============================================================================

// NoAsNeeded adds --no-as-needed to LDFLAGS.
//
// Usage: no-as-needed
//
// Adds -Wl,--no-as-needed to LDFLAGS to disable as-needed linking.
func (h *Helpers) NoAsNeeded(args []string) error {
	return h.AppendLdflags([]string{"-Wl,--no-as-needed"})
}

// RawLdflags outputs LDFLAGS without Wl, prefix conversion.
//
// Usage: ldflags=$(raw-ldflags)
//
// Returns LDFLAGS as-is, useful for non-gcc linkers.
func (h *Helpers) RawLdflags(args []string) error {
	h.writeStdout(h.getLDFLAGS())
	return nil
}

// ============================================================================
// Environment Variable Helpers
// ============================================================================

// getCFLAGS returns the current CFLAGS value.
func (h *Helpers) getCFLAGS() string {
	if h.env != nil && h.env.CFLAGS != "" {
		return h.env.CFLAGS
	}
	return os.Getenv("CFLAGS")
}

// getCXXFLAGS returns the current CXXFLAGS value.
func (h *Helpers) getCXXFLAGS() string {
	if h.env != nil && h.env.CXXFLAGS != "" {
		return h.env.CXXFLAGS
	}
	return os.Getenv("CXXFLAGS")
}

// getLDFLAGS returns the current LDFLAGS value.
func (h *Helpers) getLDFLAGS() string {
	if h.env != nil && h.env.LDFLAGS != "" {
		return h.env.LDFLAGS
	}
	return os.Getenv("LDFLAGS")
}

// getCPPFLAGS returns the current CPPFLAGS value.
func (h *Helpers) getCPPFLAGS() string {
	if h.env != nil {
		if val := h.env.GetVar("CPPFLAGS"); val != "" {
			return val
		}
	}
	return os.Getenv("CPPFLAGS")
}

// getFFLAGS returns the current FFLAGS value.
func (h *Helpers) getFFLAGS() string {
	if h.env != nil {
		if val := h.env.GetVar("FFLAGS"); val != "" {
			return val
		}
	}
	return os.Getenv("FFLAGS")
}

// getFCFLAGS returns the current FCFLAGS value.
func (h *Helpers) getFCFLAGS() string {
	if h.env != nil {
		if val := h.env.GetVar("FCFLAGS"); val != "" {
			return val
		}
	}
	return os.Getenv("FCFLAGS")
}

// setFlagVar sets a flag environment variable.
func (h *Helpers) setFlagVar(varName, value string) {
	if h.env == nil {
		return
	}

	switch varName {
	case "CFLAGS":
		h.env.CFLAGS = value
	case "CXXFLAGS":
		h.env.CXXFLAGS = value
	case "LDFLAGS":
		h.env.LDFLAGS = value
	default:
		h.env.SetVar(varName, value)
	}
}

// ============================================================================
// Eclass Setup
// ============================================================================

// SetupFlagOMaticEclass configures the flag-o-matic eclass.
//
// Called when 'inherit flag-o-matic' is executed.
func (h *Helpers) SetupFlagOMaticEclass() error {
	// flag-o-matic doesn't set any default variables
	// It only provides functions for flag manipulation
	return nil
}
