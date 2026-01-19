// Package cli provides command-line interface components for GRPM.
package cli

import "sort"

// SuggestCommand finds commands similar to the input using Levenshtein distance.
//
// The threshold parameter controls maximum edit distance for a suggestion.
// A threshold of 2 works well for typical command typos.
//
// Returns a slice of suggested commands sorted by distance (closest first).
// Returns nil if no commands are within the threshold.
//
// Example:
//
//	suggestions := SuggestCommand("emrge", []string{"emerge", "search", "info"}, 2)
//	// Returns: ["emerge"]
func SuggestCommand(input string, commands []string, threshold int) []string {
	if input == "" || len(commands) == 0 || threshold < 0 {
		return nil
	}

	type suggestion struct {
		command  string
		distance int
	}

	var suggestions []suggestion

	for _, cmd := range commands {
		dist := levenshtein(input, cmd)
		if dist <= threshold {
			suggestions = append(suggestions, suggestion{
				command:  cmd,
				distance: dist,
			})
		}
	}

	if len(suggestions) == 0 {
		return nil
	}

	// Sort by distance (closest first), then alphabetically for same distance
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].distance != suggestions[j].distance {
			return suggestions[i].distance < suggestions[j].distance
		}
		return suggestions[i].command < suggestions[j].command
	})

	result := make([]string, len(suggestions))
	for i, s := range suggestions {
		result[i] = s.command
	}

	return result
}

// levenshtein calculates the Levenshtein distance (edit distance) between two strings.
//
// The Levenshtein distance is the minimum number of single-character edits
// (insertions, deletions, or substitutions) required to change one string into another.
//
// This implementation uses the Wagner-Fischer algorithm with O(min(m,n)) space complexity.
//
// Examples:
//
//	levenshtein("emerge", "emerge") == 0  // identical
//	levenshtein("emrge", "emerge")  == 1  // one insertion
//	levenshtein("search", "serach") == 2  // two transpositions (swap a,r)
func levenshtein(a, b string) int {
	// Handle empty strings
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Ensure a is the shorter string for space optimization
	if len(a) > len(b) {
		a, b = b, a
	}

	// Convert to runes for proper Unicode handling
	runesA := []rune(a)
	runesB := []rune(b)

	// Use two rows instead of full matrix (space optimization)
	prevRow := make([]int, len(runesA)+1)
	currRow := make([]int, len(runesA)+1)

	// Initialize first row
	for i := range prevRow {
		prevRow[i] = i
	}

	// Fill the matrix row by row
	for j := 1; j <= len(runesB); j++ {
		currRow[0] = j

		for i := 1; i <= len(runesA); i++ {
			cost := 0
			if runesA[i-1] != runesB[j-1] {
				cost = 1
			}

			// Minimum of: delete, insert, or substitute
			currRow[i] = min3(
				prevRow[i]+1,      // deletion
				currRow[i-1]+1,    // insertion
				prevRow[i-1]+cost, // substitution
			)
		}

		// Swap rows
		prevRow, currRow = currRow, prevRow
	}

	return prevRow[len(runesA)]
}

// min3 returns the minimum of three integers.
func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// GetKnownCommands returns a list of all known GRPM commands.
//
// This function extracts command names from the CommandRegistry
// for use in command suggestions.
func GetKnownCommands() []string {
	registry := NewCommandRegistry()
	commands := registry.All()

	result := make([]string, len(commands))
	for i, cmd := range commands {
		result[i] = cmd.Name
	}

	return result
}
