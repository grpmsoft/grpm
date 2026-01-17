package repo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FindSimilarPackages finds packages with names similar to the query.
//
// It uses simple substring matching and Levenshtein-like distance
// to find packages that might be what the user intended.
//
// Parameters:
//   - query: The package atom to search for (e.g., "neofatch" or "app-misc/neofatch")
//   - repoPath: Path to the Portage repository
//   - maxResults: Maximum number of results to return
//
// Returns up to maxResults similar package names.
func FindSimilarPackages(query, repoPath string, maxResults int) []string {
	// Parse query to extract package name
	var category, pkgName string
	parts := strings.Split(query, "/")
	if len(parts) == 2 {
		category = parts[0]
		pkgName = parts[1]
	} else {
		pkgName = query
	}

	// Remove version if present
	pkgName = removeVersionFromName(pkgName)
	pkgNameLower := strings.ToLower(pkgName)

	// Collect candidate packages
	type scored struct {
		cp    string
		score int // Lower is better
	}
	var candidates []scored

	// If category is specified, only search that category
	if category != "" {
		categoryPath := filepath.Join(repoPath, category)
		if entries, err := os.ReadDir(categoryPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				nameLower := strings.ToLower(name)

				// Calculate similarity score
				if score := calculateSimilarity(pkgNameLower, nameLower); score < 5 {
					candidates = append(candidates, scored{
						cp:    category + "/" + name,
						score: score,
					})
				}
			}
		}
	} else {
		// Search all categories
		categories, err := os.ReadDir(repoPath)
		if err != nil {
			return nil
		}

		for _, cat := range categories {
			if !cat.IsDir() {
				continue
			}
			catName := cat.Name()

			// Skip non-category directories
			if !strings.Contains(catName, "-") && catName != "virtual" {
				continue
			}

			categoryPath := filepath.Join(repoPath, catName)
			entries, err := os.ReadDir(categoryPath)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				nameLower := strings.ToLower(name)

				// Calculate similarity score
				if score := calculateSimilarity(pkgNameLower, nameLower); score < 5 {
					candidates = append(candidates, scored{
						cp:    catName + "/" + name,
						score: score,
					})
				}
			}
		}
	}

	// Sort by score (lower is better)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	// Return top results
	result := make([]string, 0, maxResults)
	for i := 0; i < len(candidates) && i < maxResults; i++ {
		result = append(result, candidates[i].cp)
	}

	return result
}

// calculateSimilarity calculates a similarity score between two strings.
//
// Lower scores indicate higher similarity:
//   - 0: Exact match
//   - 1: Case-insensitive match
//   - 2: One is prefix of the other
//   - 3: Contains one string in the other
//   - 4+: Levenshtein-like distance
func calculateSimilarity(query, target string) int {
	// Exact match
	if query == target {
		return 0
	}

	// Prefix match (neofet matches neofetch)
	if strings.HasPrefix(target, query) || strings.HasPrefix(query, target) {
		return 1
	}

	// Contains match
	if strings.Contains(target, query) || strings.Contains(query, target) {
		return 2
	}

	// Calculate simple edit distance
	return simpleEditDistance(query, target)
}

// simpleEditDistance calculates a simplified edit distance.
//
// This is not a full Levenshtein implementation but gives reasonable results
// for package name matching without external dependencies.
func simpleEditDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Quick length check - if lengths differ by more than 3, skip
	lenDiff := len(s1) - len(s2)
	if lenDiff < 0 {
		lenDiff = -lenDiff
	}
	if lenDiff > 3 {
		return lenDiff + 3
	}

	// Count common characters (simplified)
	common := 0
	used := make([]bool, len(s2))
	for _, c1 := range s1 {
		for j, c2 := range s2 {
			if !used[j] && c1 == c2 {
				common++
				used[j] = true
				break
			}
		}
	}

	// Score based on uncommon characters
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	return maxLen - common + 2
}

// removeVersionFromName removes version suffix from package name.
// Example: "gcc-13.4.1" -> "gcc"
func removeVersionFromName(name string) string {
	// Find last hyphen before digit
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '-' && i+1 < len(name) {
			if name[i+1] >= '0' && name[i+1] <= '9' {
				return name[:i]
			}
		}
	}
	return name
}
