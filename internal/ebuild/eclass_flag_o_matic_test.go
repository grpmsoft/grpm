// Package ebuild implements ebuild execution engine.
//
// This file contains tests for flag-o-matic.eclass functions.
package ebuild

import (
	"bytes"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// FlagSet Tests
// ============================================================================

func TestNewFlagSet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil, // strings.Fields returns nil for empty
		},
		{
			name:     "single flag",
			input:    "-O2",
			expected: []string{"-O2"},
		},
		{
			name:     "multiple flags",
			input:    "-O2 -march=native -fPIC",
			expected: []string{"-O2", "-march=native", "-fPIC"},
		},
		{
			name:     "flags with extra whitespace",
			input:    "  -O2   -fPIC  ",
			expected: []string{"-O2", "-fPIC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFlagSet(tt.input)
			flags := fs.Flags()

			if len(flags) != len(tt.expected) {
				t.Errorf("expected %d flags, got %d", len(tt.expected), len(flags))
				return
			}

			for i, expected := range tt.expected {
				if flags[i] != expected {
					t.Errorf("flag[%d]: expected %q, got %q", i, expected, flags[i])
				}
			}
		})
	}
}

func TestFlagSetAppend(t *testing.T) {
	tests := []struct {
		name       string
		initial    string
		append     []string
		expected   string
		immutCheck bool // verify original is unchanged
	}{
		{
			name:       "append to empty",
			initial:    "",
			append:     []string{"-O2", "-fPIC"},
			expected:   "-O2 -fPIC",
			immutCheck: true,
		},
		{
			name:       "append to existing",
			initial:    "-Wall",
			append:     []string{"-Wextra"},
			expected:   "-Wall -Wextra",
			immutCheck: true,
		},
		{
			name:     "append nothing",
			initial:  "-O2",
			append:   []string{},
			expected: "-O2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewFlagSet(tt.initial)
			result := original.Append(tt.append...)

			if result.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.String())
			}

			// Verify immutability
			if tt.immutCheck && original.String() != strings.TrimSpace(tt.initial) {
				t.Errorf("original was modified: expected %q, got %q", tt.initial, original.String())
			}
		})
	}
}

func TestFlagSetFilter(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		patterns []string
		expected string
	}{
		{
			name:     "filter exact match",
			initial:  "-O2 -fPIC -Wall",
			patterns: []string{"-fPIC"},
			expected: "-O2 -Wall",
		},
		{
			name:     "filter glob prefix",
			initial:  "-O2 -O3 -Os -fPIC",
			patterns: []string{"-O*"},
			expected: "-fPIC",
		},
		{
			name:     "filter glob suffix",
			initial:  "-march=native -march=x86-64 -fPIC",
			patterns: []string{"-march=*"},
			expected: "-fPIC",
		},
		{
			name:     "filter multiple patterns",
			initial:  "-O2 -march=native -Wall -Wextra",
			patterns: []string{"-O*", "-W*"},
			expected: "-march=native",
		},
		{
			name:     "filter nothing matches",
			initial:  "-O2 -fPIC",
			patterns: []string{"-Wall"},
			expected: "-O2 -fPIC",
		},
		{
			name:     "filter removes all",
			initial:  "-O2 -O3",
			patterns: []string{"-O*"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFlagSet(tt.initial)
			result := fs.Filter(tt.patterns...)

			if result.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.String())
			}
		})
	}
}

func TestFlagSetReplace(t *testing.T) {
	tests := []struct {
		name       string
		initial    string
		oldPattern string
		newFlag    string
		expected   string
	}{
		{
			name:       "replace exact match",
			initial:    "-O2 -fPIC",
			oldPattern: "-O2",
			newFlag:    "-O3",
			expected:   "-O3 -fPIC",
		},
		{
			name:       "replace with glob",
			initial:    "-O2 -march=native -fPIC",
			oldPattern: "-O*",
			newFlag:    "-Os",
			expected:   "-Os -march=native -fPIC",
		},
		{
			name:       "replace with empty (removal)",
			initial:    "-O2 -fPIC",
			oldPattern: "-O2",
			newFlag:    "",
			expected:   "-fPIC",
		},
		{
			name:       "replace no match",
			initial:    "-O2 -fPIC",
			oldPattern: "-Wall",
			newFlag:    "-Wextra",
			expected:   "-O2 -fPIC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFlagSet(tt.initial)
			result := fs.Replace(tt.oldPattern, tt.newFlag)

			if result.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.String())
			}
		})
	}
}

func TestFlagSetContains(t *testing.T) {
	fs := NewFlagSet("-O2 -march=native -fPIC")

	tests := []struct {
		name     string
		flag     string
		expected bool
	}{
		{"exact match", "-O2", true},
		{"not present", "-Wall", false},
		{"partial match should fail", "-O", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if fs.Contains(tt.flag) != tt.expected {
				t.Errorf("Contains(%q): expected %v", tt.flag, tt.expected)
			}
		})
	}
}

func TestFlagSetContainsPattern(t *testing.T) {
	fs := NewFlagSet("-O2 -march=native -fPIC")

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"exact match", "-O2", true},
		{"glob prefix", "-O*", true},
		{"glob suffix", "-march=*", true},
		{"no match", "-W*", false},
		{"question mark", "-O?", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if fs.ContainsPattern(tt.pattern) != tt.expected {
				t.Errorf("ContainsPattern(%q): expected %v", tt.pattern, tt.expected)
			}
		})
	}
}

func TestFlagSetGetFlag(t *testing.T) {
	fs := NewFlagSet("-O2 -march=native -fPIC")

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{"exact match", "-O2", "-O2"},
		{"glob match", "-march=*", "-march=native"},
		{"no match", "-Wall", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fs.GetFlag(tt.pattern); got != tt.expected {
				t.Errorf("GetFlag(%q): expected %q, got %q", tt.pattern, tt.expected, got)
			}
		})
	}
}

func TestFlagSetStripToSafe(t *testing.T) {
	tests := []struct {
		name         string
		initial      string
		safePatterns []string
		expected     string
	}{
		{
			name:         "keep safe optimization",
			initial:      "-O2 -funknown-option -fPIC",
			safePatterns: []string{"-O*", "-fPIC"},
			expected:     "-O2 -fPIC",
		},
		{
			name:         "keep nothing",
			initial:      "-funknown1 -funknown2",
			safePatterns: []string{"-O*"},
			expected:     "",
		},
		{
			name:         "keep all",
			initial:      "-O2 -O3",
			safePatterns: []string{"-O*"},
			expected:     "-O2 -O3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewFlagSet(tt.initial)
			result := fs.StripToSafe(tt.safePatterns)

			if result.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.String())
			}
		})
	}
}

// ============================================================================
// Pattern Matching Tests
// ============================================================================

func TestMatchGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		pattern  string
		expected bool
	}{
		// Exact matches
		{"exact match", "-O2", "-O2", true},
		{"exact no match", "-O2", "-O3", false},

		// Wildcard *
		{"star prefix", "-march=native", "-march=*", true},
		{"star suffix", "-O2", "*2", true},
		{"star middle", "-march=native", "-*=*", true},
		{"star only", "anything", "*", true},
		{"star no match", "-O2", "-W*", false},

		// Wildcard ?
		{"question single char", "-O2", "-O?", true},
		{"question no match", "-O12", "-O?", false},
		{"question multiple", "-O12", "-O??", true},

		// Combined
		{"star and question", "-O2x", "-O?*", true},

		// Special regex characters should be escaped
		{"escape dot", "-O.2", "-O.2", true},
		{"escape brackets", "[-O2]", "[-O2]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if matchGlobPattern(tt.s, tt.pattern) != tt.expected {
				t.Errorf("matchGlobPattern(%q, %q): expected %v", tt.s, tt.pattern, tt.expected)
			}
		})
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func createFlagOMaticTestHelpers(t *testing.T, cflags, cxxflags, ldflags string) *Helpers {
	t.Helper()

	p := &pkg.Package{
		Name:    "test-cat/test-pkg",
		Version: "1.0.0",
	}

	env, err := NewEnvironment(p, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	env.CFLAGS = cflags
	env.CXXFLAGS = cxxflags
	env.LDFLAGS = ldflags

	var stdout, stderr bytes.Buffer
	return NewHelpers(env, &stdout, &stderr)
}

func TestAppendFlags(t *testing.T) {
	tests := []struct {
		name            string
		initialCFlags   string
		initialCXXFlags string
		appendFlags     []string
		expectedCFlags  string
		expectedCXX     string
	}{
		{
			name:            "append to empty",
			initialCFlags:   "",
			initialCXXFlags: "",
			appendFlags:     []string{"-O2", "-fPIC"},
			expectedCFlags:  "-O2 -fPIC",
			expectedCXX:     "-O2 -fPIC",
		},
		{
			name:            "append to existing",
			initialCFlags:   "-Wall",
			initialCXXFlags: "-Wall",
			appendFlags:     []string{"-Wextra"},
			expectedCFlags:  "-Wall -Wextra",
			expectedCXX:     "-Wall -Wextra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := createFlagOMaticTestHelpers(t, tt.initialCFlags, tt.initialCXXFlags, "")

			if err := h.AppendFlags(tt.appendFlags); err != nil {
				t.Fatalf("AppendFlags failed: %v", err)
			}

			if h.env.CFLAGS != tt.expectedCFlags {
				t.Errorf("CFLAGS: expected %q, got %q", tt.expectedCFlags, h.env.CFLAGS)
			}
			if h.env.CXXFLAGS != tt.expectedCXX {
				t.Errorf("CXXFLAGS: expected %q, got %q", tt.expectedCXX, h.env.CXXFLAGS)
			}
		})
	}
}

func TestAppendCflags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "-Wall", "-Wall", "")

	if err := h.AppendCflags([]string{"-O2"}); err != nil {
		t.Fatalf("AppendCflags failed: %v", err)
	}

	// CFLAGS should be updated
	if h.env.CFLAGS != "-Wall -O2" {
		t.Errorf("CFLAGS: expected %q, got %q", "-Wall -O2", h.env.CFLAGS)
	}

	// CXXFLAGS should be unchanged
	if h.env.CXXFLAGS != "-Wall" {
		t.Errorf("CXXFLAGS should be unchanged: expected %q, got %q", "-Wall", h.env.CXXFLAGS)
	}
}

func TestAppendCxxflags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "-Wall", "-Wall", "")

	if err := h.AppendCxxflags([]string{"-std=c++17"}); err != nil {
		t.Fatalf("AppendCxxflags failed: %v", err)
	}

	// CFLAGS should be unchanged
	if h.env.CFLAGS != "-Wall" {
		t.Errorf("CFLAGS should be unchanged: expected %q, got %q", "-Wall", h.env.CFLAGS)
	}

	// CXXFLAGS should be updated
	if h.env.CXXFLAGS != "-Wall -std=c++17" {
		t.Errorf("CXXFLAGS: expected %q, got %q", "-Wall -std=c++17", h.env.CXXFLAGS)
	}
}

func TestAppendLdflags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "-Wl,-O1")

	if err := h.AppendLdflags([]string{"-Wl,--as-needed"}); err != nil {
		t.Fatalf("AppendLdflags failed: %v", err)
	}

	if h.env.LDFLAGS != "-Wl,-O1 -Wl,--as-needed" {
		t.Errorf("LDFLAGS: expected %q, got %q", "-Wl,-O1 -Wl,--as-needed", h.env.LDFLAGS)
	}
}

func TestFilterFlags(t *testing.T) {
	tests := []struct {
		name           string
		initialCFlags  string
		patterns       []string
		expectedCFlags string
	}{
		{
			name:           "filter exact",
			initialCFlags:  "-O2 -fPIC -Wall",
			patterns:       []string{"-O2"},
			expectedCFlags: "-fPIC -Wall",
		},
		{
			name:           "filter glob",
			initialCFlags:  "-O2 -O3 -Os -fPIC",
			patterns:       []string{"-O*"},
			expectedCFlags: "-fPIC",
		},
		{
			name:           "filter multiple patterns",
			initialCFlags:  "-O2 -march=native -Wall -Wextra",
			patterns:       []string{"-march=*", "-W*"},
			expectedCFlags: "-O2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := createFlagOMaticTestHelpers(t, tt.initialCFlags, tt.initialCFlags, "")

			if err := h.FilterFlags(tt.patterns); err != nil {
				t.Fatalf("FilterFlags failed: %v", err)
			}

			if h.env.CFLAGS != tt.expectedCFlags {
				t.Errorf("CFLAGS: expected %q, got %q", tt.expectedCFlags, h.env.CFLAGS)
			}
		})
	}
}

func TestFilterLdflags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "-Wl,-O1 -Wl,--as-needed -Wl,-rpath,/usr/lib")

	if err := h.FilterLdflags([]string{"-Wl,-rpath*"}); err != nil {
		t.Fatalf("FilterLdflags failed: %v", err)
	}

	if h.env.LDFLAGS != "-Wl,-O1 -Wl,--as-needed" {
		t.Errorf("LDFLAGS: expected %q, got %q", "-Wl,-O1 -Wl,--as-needed", h.env.LDFLAGS)
	}
}

func TestStripFlags(t *testing.T) {
	// StripFlags removes optimization and CPU flags (per eclass_helpers.go implementation)
	h := createFlagOMaticTestHelpers(t, "-O2 -Wall -march=native -fPIC", "", "")

	if err := h.StripFlags(nil); err != nil {
		t.Fatalf("StripFlags failed: %v", err)
	}

	// Should remove -O2 and -march=native but keep -Wall and -fPIC
	cflags := h.env.CFLAGS
	if strings.Contains(cflags, "-O2") {
		t.Errorf("CFLAGS should not contain -O2 (optimization flag): %q", cflags)
	}
	if strings.Contains(cflags, "-march=native") {
		t.Errorf("CFLAGS should not contain -march=native (CPU flag): %q", cflags)
	}
	if !strings.Contains(cflags, "-Wall") {
		t.Errorf("CFLAGS should contain -Wall: %q", cflags)
	}
	if !strings.Contains(cflags, "-fPIC") {
		t.Errorf("CFLAGS should contain -fPIC: %q", cflags)
	}
}

func TestReplaceCpuFlags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "-O2 -march=i686 -fPIC", "-O2 -march=i686", "")

	if err := h.ReplaceCpuFlags([]string{"i686", "pentium4"}); err != nil {
		t.Fatalf("ReplaceCpuFlags failed: %v", err)
	}

	if !strings.Contains(h.env.CFLAGS, "-march=pentium4") {
		t.Errorf("CFLAGS should contain -march=pentium4: %q", h.env.CFLAGS)
	}
	if strings.Contains(h.env.CFLAGS, "-march=i686") {
		t.Errorf("CFLAGS should not contain -march=i686: %q", h.env.CFLAGS)
	}
}

func TestAppendLfsFlags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "")

	if err := h.AppendLfsFlags(nil); err != nil {
		t.Fatalf("AppendLfsFlags failed: %v", err)
	}

	cppflags := h.env.GetVar("CPPFLAGS")
	expectedFlags := []string{"-D_LARGEFILE_SOURCE", "-D_LARGEFILE64_SOURCE", "-D_FILE_OFFSET_BITS=64"}

	for _, flag := range expectedFlags {
		if !strings.Contains(cppflags, flag) {
			t.Errorf("CPPFLAGS should contain %s: %q", flag, cppflags)
		}
	}
}

func TestGetFlag(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "-O2 -march=native -fPIC", "", "")
	var stdout bytes.Buffer
	h.stdout = &stdout

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{"get march value", "march", "native"},
		{"get O level", "-O*", "-O2"},
		{"not found", "mtune", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			if err := h.GetFlag([]string{tt.pattern}); err != nil {
				t.Fatalf("GetFlag failed: %v", err)
			}

			got := stdout.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestIsFlag(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "-O2 -march=native -fPIC", "", "")

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"exact match found", "-O2", true},
		{"glob match found", "-O*", true},
		{"not found", "-Wall", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.IsFlag([]string{tt.pattern})
			found := err == nil

			if found != tt.expected {
				t.Errorf("IsFlag(%q): expected %v", tt.pattern, tt.expected)
			}
		})
	}
}

func TestIsLdflag(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "-Wl,--as-needed -Wl,-O1")

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"exact match", "-Wl,--as-needed", true},
		{"glob match", "-Wl,-O*", true},
		{"not found", "-Wl,-rpath*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.IsLdflag([]string{tt.pattern})
			found := err == nil

			if found != tt.expected {
				t.Errorf("IsLdflag(%q): expected %v", tt.pattern, tt.expected)
			}
		})
	}
}

func TestNoAsNeeded(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "")

	if err := h.NoAsNeeded(nil); err != nil {
		t.Fatalf("NoAsNeeded failed: %v", err)
	}

	if !strings.Contains(h.env.LDFLAGS, "-Wl,--no-as-needed") {
		t.Errorf("LDFLAGS should contain -Wl,--no-as-needed: %q", h.env.LDFLAGS)
	}
}

func TestRawLdflags(t *testing.T) {
	h := createFlagOMaticTestHelpers(t, "", "", "-Wl,-O1 -Wl,--as-needed")
	var stdout bytes.Buffer
	h.stdout = &stdout

	if err := h.RawLdflags(nil); err != nil {
		t.Fatalf("RawLdflags failed: %v", err)
	}

	expected := "-Wl,-O1 -Wl,--as-needed"
	if stdout.String() != expected {
		t.Errorf("expected %q, got %q", expected, stdout.String())
	}
}

// ============================================================================
// Thread Safety Tests
// ============================================================================

func TestFlagSetConcurrentAccess(t *testing.T) {
	fs := NewFlagSet("-O2 -march=native -fPIC")
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = fs.String()
				_ = fs.Flags()
				_ = fs.Contains("-O2")
				_ = fs.ContainsPattern("-O*")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestEmptyPatterns(t *testing.T) {
	fs := NewFlagSet("-O2 -fPIC")

	// Filter with no patterns should keep all flags
	result := fs.Filter()
	if result.String() != "-O2 -fPIC" {
		t.Errorf("expected unchanged flags, got %q", result.String())
	}

	// Append with no flags should keep all flags
	result = fs.Append()
	if result.String() != "-O2 -fPIC" {
		t.Errorf("expected unchanged flags, got %q", result.String())
	}
}

func TestUnicodeInFlags(t *testing.T) {
	// While unusual, flags with unicode should not crash
	fs := NewFlagSet("-DNAME=\"\u4e2d\u6587\"")

	if fs.Len() != 1 {
		t.Errorf("expected 1 flag, got %d", fs.Len())
	}

	if !fs.Contains("-DNAME=\"\u4e2d\u6587\"") {
		t.Errorf("flag not found")
	}
}

func TestSpecialCharactersInFlags(t *testing.T) {
	// Flags can contain special characters
	fs := NewFlagSet("-DPATH=/usr/lib -DQUOTE='test'")

	if fs.Len() != 2 {
		t.Errorf("expected 2 flags, got %d", fs.Len())
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkFlagSetFilter(b *testing.B) {
	// Create a realistic flag set
	flags := "-O2 -march=native -fPIC -Wall -Wextra -Werror -std=c11 " +
		"-I/usr/include -D_GNU_SOURCE -fstack-protector-strong"
	fs := NewFlagSet(flags)
	patterns := []string{"-W*", "-I*"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fs.Filter(patterns...)
	}
}

func BenchmarkFlagSetAppend(b *testing.B) {
	fs := NewFlagSet("-O2 -fPIC")
	flags := []string{"-Wall", "-Wextra"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fs.Append(flags...)
	}
}

func BenchmarkMatchGlobPattern(b *testing.B) {
	testCases := []struct {
		s       string
		pattern string
	}{
		{"-march=native", "-march=*"},
		{"-O2", "-O?"},
		{"-fstack-protector-strong", "-fstack-protector*"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := testCases[i%len(testCases)]
		_ = matchGlobPattern(tc.s, tc.pattern)
	}
}
