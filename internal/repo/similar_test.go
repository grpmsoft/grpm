package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		query  string
		target string
		want   int // Lower is better
	}{
		// Exact match
		{"neofetch", "neofetch", 0},

		// Prefix match
		{"neofet", "neofetch", 1},
		{"neofetch", "neofet", 1},

		// Contains match
		{"fetch", "neofetch", 2},

		// No match (high score)
		{"zzz", "neofetch", 8}, // Will be calculated by edit distance
	}

	for _, tc := range tests {
		t.Run(tc.query+"_vs_"+tc.target, func(t *testing.T) {
			got := calculateSimilarity(tc.query, tc.target)
			if got != tc.want {
				t.Errorf("calculateSimilarity(%q, %q) = %d, want %d",
					tc.query, tc.target, got, tc.want)
			}
		})
	}
}

func TestSimpleEditDistance(t *testing.T) {
	tests := []struct {
		s1, s2 string
		maxOk  int // Maximum acceptable distance
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"hello", "hello", 2},       // Same string, small distance expected
		{"hello", "helo", 3},        // One char missing
		{"neofetch", "neofatch", 4}, // Typo
	}

	for _, tc := range tests {
		got := simpleEditDistance(tc.s1, tc.s2)
		if got > tc.maxOk {
			t.Errorf("simpleEditDistance(%q, %q) = %d, want <= %d",
				tc.s1, tc.s2, got, tc.maxOk)
		}
	}
}

func TestRemoveVersionFromName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"gcc-13.4.1", "gcc"},
		{"python-3.12", "python"},
		{"neofetch", "neofetch"},
		{"hello-2.12.2", "hello"},
		{"lib-foo-1.0", "lib-foo"},
	}

	for _, tc := range tests {
		got := removeVersionFromName(tc.name)
		if got != tc.want {
			t.Errorf("removeVersionFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestFindSimilarPackages_WithTempRepo(t *testing.T) {
	// Create temporary repository structure
	tmpDir := t.TempDir()

	// Create some package directories
	packages := []string{
		"app-misc/neofetch",
		"app-misc/screenfetch",
		"sys-apps/systemd",
		"dev-lang/python",
		"dev-lang/ruby",
	}

	for _, pkg := range packages {
		dir := filepath.Join(tmpDir, pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	tests := []struct {
		query    string
		wantLen  int
		contains string
	}{
		{"neofatch", 1, "app-misc/neofetch"},       // Typo -> finds neofetch
		{"screenfetch", 1, "app-misc/screenfetch"}, // Exact match
		{"app-misc/fetch", 2, ""},                  // Partial match in category
		{"python", 1, "dev-lang/python"},           // No category
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results := FindSimilarPackages(tc.query, tmpDir, 3)

			if tc.wantLen > 0 && len(results) < tc.wantLen {
				t.Errorf("FindSimilarPackages(%q) returned %d results, want at least %d",
					tc.query, len(results), tc.wantLen)
			}

			if tc.contains != "" {
				found := false
				for _, r := range results {
					if r == tc.contains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindSimilarPackages(%q) = %v, should contain %q",
						tc.query, results, tc.contains)
				}
			}
		})
	}
}

func TestFindSimilarPackages_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	results := FindSimilarPackages("anything", tmpDir, 3)
	if len(results) != 0 {
		t.Errorf("Empty repo should return no results, got %v", results)
	}
}

func TestFindSimilarPackages_NonexistentRepo(t *testing.T) {
	results := FindSimilarPackages("anything", "/nonexistent/path", 3)
	if len(results) != 0 {
		t.Errorf("Nonexistent repo should return empty/nil, got %v", results)
	}
}
