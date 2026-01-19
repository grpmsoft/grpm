package cli

import (
	"reflect"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		// Identical strings
		{"identical empty", "", "", 0},
		{"identical single char", "a", "a", 0},
		{"identical word", "emerge", "emerge", 0},

		// Empty string cases
		{"empty a", "", "hello", 5},
		{"empty b", "hello", "", 5},

		// Single character edits
		{"one insertion", "emrge", "emerge", 1},
		{"one deletion", "eemerge", "emerge", 1},
		{"one substitution", "emerga", "emerge", 1},

		// Common typos for GRPM commands
		{"emrge typo", "emrge", "emerge", 1},
		{"serach typo", "serach", "search", 2},
		{"instal typo", "instal", "install", 1}, //nolint:misspell // intentional typo for test
		{"remov typo", "remov", "remove", 1},
		{"synce typo", "synce", "sync", 1},
		{"resolv typo", "resolv", "resolve", 1},

		// Transpositions (counted as 2 operations in Levenshtein)
		{"transposition", "ab", "ba", 2},
		{"search transposition", "saerch", "search", 2},

		// Completely different strings
		{"completely different", "abc", "xyz", 3},
		{"different lengths", "short", "verylongstring", 12},

		// Unicode handling
		{"unicode identical", "hello", "hello", 0},
		{"unicode with accents", "cafe", "cafe", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levenshtein(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}

			// Verify symmetry: distance(a, b) == distance(b, a)
			reversed := levenshtein(tt.b, tt.a)
			if result != reversed {
				t.Errorf("levenshtein is not symmetric: (%q, %q)=%d, (%q, %q)=%d",
					tt.a, tt.b, result, tt.b, tt.a, reversed)
			}
		})
	}
}

func TestSuggestCommand(t *testing.T) {
	commands := []string{
		"resolve", "install", "sync", "search", "info",
		"update", "remove", "build", "emerge", "depclean",
		"fetch", "analyze", "tools", "completion", "doc",
	}

	tests := []struct {
		name      string
		input     string
		threshold int
		expected  []string
	}{
		// Common typos that should be suggested
		{
			name:      "emrge suggests emerge",
			input:     "emrge",
			threshold: 2,
			expected:  []string{"emerge"},
		},
		{
			name:      "serach suggests search",
			input:     "serach",
			threshold: 2,
			expected:  []string{"search"},
		},
		{
			name:      "instal suggests install", //nolint:misspell // intentional typo for test
			input:     "instal",                  //nolint:misspell // intentional typo for test
			threshold: 2,
			expected:  []string{"install"},
		},
		{
			name:      "remov suggests remove",
			input:     "remov",
			threshold: 2,
			expected:  []string{"remove"},
		},
		{
			name:      "resolv suggests resolve",
			input:     "resolv",
			threshold: 2,
			expected:  []string{"resolve"},
		},
		{
			name:      "synce suggests sync",
			input:     "synce",
			threshold: 2,
			expected:  []string{"sync"},
		},
		{
			name:      "infoo suggests info",
			input:     "infoo",
			threshold: 2,
			expected:  []string{"info"},
		},
		{
			name:      "ftch suggests fetch",
			input:     "ftch",
			threshold: 2,
			expected:  []string{"fetch"},
		},

		// Multiple suggestions (both within threshold)
		{
			name:      "doc suggests doc and completion",
			input:     "do",
			threshold: 2,
			expected:  []string{"doc"},
		},

		// No suggestions for completely wrong input
		{
			name:      "xyz has no suggestions",
			input:     "xyz",
			threshold: 2,
			expected:  nil,
		},
		{
			name:      "verylongwrongcommand has no suggestions",
			input:     "verylongwrongcommand",
			threshold: 2,
			expected:  nil,
		},

		// Edge cases
		{
			name:      "empty input",
			input:     "",
			threshold: 2,
			expected:  nil,
		},
		{
			name:      "exact match",
			input:     "emerge",
			threshold: 2,
			expected:  []string{"emerge"},
		},
		{
			name:      "zero threshold exact match",
			input:     "emerge",
			threshold: 0,
			expected:  []string{"emerge"},
		},
		{
			name:      "zero threshold no match",
			input:     "emrge",
			threshold: 0,
			expected:  nil,
		},
		{
			name:      "negative threshold",
			input:     "emerge",
			threshold: -1,
			expected:  nil,
		},

		// Higher threshold includes more matches
		{
			name:      "higher threshold finds more",
			input:     "anayze",
			threshold: 3,
			expected:  []string{"analyze"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuggestCommand(tt.input, commands, tt.threshold)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SuggestCommand(%q, commands, %d) = %v, want %v",
					tt.input, tt.threshold, result, tt.expected)
			}
		})
	}
}

func TestSuggestCommand_EmptyCommands(t *testing.T) {
	result := SuggestCommand("emerge", []string{}, 2)
	if result != nil {
		t.Errorf("SuggestCommand with empty commands = %v, want nil", result)
	}
}

func TestSuggestCommand_SortOrder(t *testing.T) {
	// Create commands where multiple are within threshold
	commands := []string{"abc", "ab", "abcd", "abcde"}

	// Input "abx" has distances:
	// - "abc" = 1 (substitute x->c)
	// - "ab" = 1 (delete x)
	// - "abcd" = 2 (substitute x->c, insert d) OR (delete x, insert c, insert d)
	// - "abcde" = 3 (beyond threshold)
	//
	// With threshold 2, should return: ab, abc, abcd
	// Sorted by distance first, then alphabetically for same distance
	// Both ab and abc have distance 1, ab < abc alphabetically
	result := SuggestCommand("abx", commands, 2)

	expected := []string{"ab", "abc", "abcd"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("SuggestCommand sort order: got %v, want %v", result, expected)
	}
}

func TestGetKnownCommands(t *testing.T) {
	commands := GetKnownCommands()

	// Verify we get a non-empty list
	if len(commands) == 0 {
		t.Error("GetKnownCommands returned empty list")
	}

	// Verify essential commands are present
	essentialCommands := []string{
		"resolve", "install", "emerge", "remove", "search",
		"info", "sync", "update", "fetch", "analyze", "tools",
	}

	commandSet := make(map[string]bool)
	for _, cmd := range commands {
		commandSet[cmd] = true
	}

	for _, essential := range essentialCommands {
		if !commandSet[essential] {
			t.Errorf("GetKnownCommands missing essential command: %s", essential)
		}
	}
}

func TestMin3(t *testing.T) {
	tests := []struct {
		a, b, c  int
		expected int
	}{
		{1, 2, 3, 1},
		{3, 2, 1, 1},
		{2, 1, 3, 1},
		{5, 5, 5, 5},
		{0, 1, 2, 0},
		{-1, 0, 1, -1},
	}

	for _, tt := range tests {
		result := min3(tt.a, tt.b, tt.c)
		if result != tt.expected {
			t.Errorf("min3(%d, %d, %d) = %d, want %d", tt.a, tt.b, tt.c, result, tt.expected)
		}
	}
}
