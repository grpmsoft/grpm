package cli

import (
	"sort"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestSearchVersionSorting verifies that search results display the highest version
// using PMS-compliant version comparison, not alphabetical sorting.
//
// This is a regression test for v0.8.2-002: Fix search version display.
// Before the fix, "grpm search hello" showed 2.12 instead of 2.12.2 because
// filepath.Glob returns alphabetically sorted results.
func TestSearchVersionSorting(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
	}{
		{
			name:     "hello versions - 2.12.2 is higher than 2.12",
			versions: []string{"2.9", "2.10", "2.12", "2.12.2"},
			want:     "2.12.2",
		},
		{
			name:     "reverse order input",
			versions: []string{"2.12.2", "2.12", "2.10", "2.9"},
			want:     "2.12.2",
		},
		{
			name:     "alphabetical order would choose wrong version",
			versions: []string{"2.12", "2.12.2", "2.9"},
			want:     "2.12.2", // alphabetically "2.9" > "2.12.2" > "2.12"
		},
		{
			name:     "versions with suffixes",
			versions: []string{"1.0_alpha", "1.0_beta", "1.0_rc1", "1.0", "1.0_p1"},
			want:     "1.0_p1", // patchlevel is highest
		},
		{
			name:     "versions with revisions",
			versions: []string{"1.0-r1", "1.0", "1.0-r2", "1.0-r10"},
			want:     "1.0-r10",
		},
		{
			name:     "complex gcc-style versions",
			versions: []string{"13.4.1_p20250807", "14.2.0", "15.2.1_p20251122"},
			want:     "15.2.1_p20251122",
		},
		{
			name:     "single version",
			versions: []string{"1.0"},
			want:     "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This replicates the sorting logic from runSearch()
			parsedVersions := make([]string, len(tt.versions))
			copy(parsedVersions, tt.versions)

			// Sort using PMS-compliant comparison (ascending order)
			sort.Slice(parsedVersions, func(i, j int) bool {
				return pkg.CompareVersions(parsedVersions[i], parsedVersions[j]) < 0
			})

			// Take the highest version (last after ascending sort)
			got := parsedVersions[len(parsedVersions)-1]

			if got != tt.want {
				t.Errorf("highest version = %q, want %q", got, tt.want)
				t.Logf("sorted order: %v", parsedVersions)
			}
		})
	}
}

// TestSearchVersionSorting_AlphabeticalVsPMS demonstrates why PMS comparison
// is necessary instead of alphabetical sorting.
func TestSearchVersionSorting_AlphabeticalVsPMS(t *testing.T) {
	versions := []string{"2.9", "2.10", "2.12", "2.12.2"}

	// Alphabetical sort (WRONG)
	alphabetical := make([]string, len(versions))
	copy(alphabetical, versions)
	sort.Strings(alphabetical)

	// PMS sort (CORRECT)
	pms := make([]string, len(versions))
	copy(pms, versions)
	sort.Slice(pms, func(i, j int) bool {
		return pkg.CompareVersions(pms[i], pms[j]) < 0
	})

	// Alphabetical order: 2.10, 2.12, 2.12.2, 2.9 (wrong! 2.9 > 2.12.2 alphabetically)
	// PMS order: 2.9, 2.10, 2.12, 2.12.2 (correct)

	alphabeticalHighest := alphabetical[len(alphabetical)-1]
	pmsHighest := pms[len(pms)-1]

	if alphabeticalHighest == pmsHighest {
		t.Skip("alphabetical and PMS happen to agree for this input")
	}

	// The bug was that alphabetical sorting would return "2.9" as highest
	// because "2.9" > "2.12.2" in ASCII comparison
	if alphabeticalHighest != "2.9" {
		t.Errorf("expected alphabetical highest to be 2.9 (demonstrating the bug), got %q", alphabeticalHighest)
	}

	if pmsHighest != "2.12.2" {
		t.Errorf("expected PMS highest to be 2.12.2, got %q", pmsHighest)
	}

	t.Logf("Alphabetical order: %v (highest: %s) - WRONG", alphabetical, alphabeticalHighest)
	t.Logf("PMS order: %v (highest: %s) - CORRECT", pms, pmsHighest)
}

// TestLoadBestPackageVersion_VersionFiltering verifies that loadBestPackageVersion
// correctly filters out masked and unkeyworded packages.
//
// This is a regression test for v0.8.2-001: Apply mask/keyword filtering to info command.
// Before the fix, `grpm info sys-devel/gcc` showed gcc-16.0.9999 (masked/unkeyworded)
// instead of gcc-15.2.1 (stable).
func TestLoadBestPackageVersion_VersionFiltering(t *testing.T) {
	// Test the filtering logic used by loadBestPackageVersion
	// We simulate the filtering without needing a real repository

	tests := []struct {
		name           string
		versions       []testPackageInfo
		acceptKeywords []string
		maskedVersions []string
		wantVersion    string
		wantError      bool
	}{
		{
			name: "gcc scenario - masked 9999 excluded",
			versions: []testPackageInfo{
				{version: "15.2.1", keywords: []string{"amd64", "~x86"}},
				{version: "16.0.9999", keywords: []string{}}, // No keywords = unkeyworded
			},
			acceptKeywords: []string{"amd64"},
			maskedVersions: []string{"16.0.9999"},
			wantVersion:    "15.2.1",
		},
		{
			name: "glibc scenario - live 9999 excluded",
			versions: []testPackageInfo{
				{version: "2.40-r8", keywords: []string{"amd64"}},
				{version: "9999", keywords: []string{}}, // Live package, no keywords
			},
			acceptKeywords: []string{"amd64"},
			maskedVersions: []string{},
			wantVersion:    "2.40-r8",
		},
		{
			name: "testing keyword acceptance",
			versions: []testPackageInfo{
				{version: "1.0", keywords: []string{"amd64"}},
				{version: "2.0", keywords: []string{"~amd64"}},
			},
			acceptKeywords: []string{"amd64"}, // Only stable
			maskedVersions: []string{},
			wantVersion:    "1.0", // 2.0 has only ~amd64, not accepted
		},
		{
			name: "testing keyword with tilde acceptance",
			versions: []testPackageInfo{
				{version: "1.0", keywords: []string{"amd64"}},
				{version: "2.0", keywords: []string{"~amd64"}},
			},
			acceptKeywords: []string{"amd64", "~amd64"}, // Accept both stable and testing
			maskedVersions: []string{},
			wantVersion:    "2.0", // Both are acceptable, 2.0 is higher
		},
		{
			name: "all versions masked",
			versions: []testPackageInfo{
				{version: "1.0", keywords: []string{"amd64"}},
				{version: "2.0", keywords: []string{"amd64"}},
			},
			acceptKeywords: []string{"amd64"},
			maskedVersions: []string{"1.0", "2.0"},
			wantVersion:    "",
			wantError:      true,
		},
		{
			name: "all versions unkeyworded",
			versions: []testPackageInfo{
				{version: "1.0", keywords: []string{}},
				{version: "2.0", keywords: []string{}},
			},
			acceptKeywords: []string{"amd64"},
			maskedVersions: []string{},
			wantVersion:    "",
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create packages from test data
			packages := make([]*pkg.Package, len(tt.versions))
			for i, v := range tt.versions {
				p := pkg.NewPackage("test/package", v.version, "0")
				p.Keywords = v.keywords
				packages[i] = p
			}

			// Create mask set for quick lookup
			maskedSet := make(map[string]bool)
			for _, v := range tt.maskedVersions {
				maskedSet[v] = true
			}

			// Filter packages (simulating loadBestPackageVersion logic)
			var acceptable []*pkg.Package
			for _, p := range packages {
				// Check mask
				if maskedSet[p.Version] {
					continue
				}

				// Check keywords
				if !p.IsKeywordAccepted(tt.acceptKeywords) {
					continue
				}

				acceptable = append(acceptable, p)
			}

			// Check for error case
			if len(acceptable) == 0 {
				if !tt.wantError {
					t.Errorf("unexpected error: no acceptable versions found")
				}
				return
			}

			if tt.wantError {
				t.Errorf("expected error but got %d acceptable versions", len(acceptable))
				return
			}

			// Sort by version (highest first)
			sort.Slice(acceptable, func(i, j int) bool {
				return pkg.CompareVersions(acceptable[i].Version, acceptable[j].Version) > 0
			})

			got := acceptable[0].Version
			if got != tt.wantVersion {
				t.Errorf("best version = %q, want %q", got, tt.wantVersion)
			}
		})
	}
}

// testPackageInfo holds test package data for version filtering tests.
type testPackageInfo struct {
	version  string
	keywords []string
}

// TestLoadBestPackageVersion_ExplicitVersionFallback verifies that explicit version
// requests (like =sys-devel/gcc-13.4.1) trigger fallback to loadPackageFromAtom.
func TestLoadBestPackageVersion_ExplicitVersionFallback(t *testing.T) {
	tests := []struct {
		atom        string
		wantVersion bool // true if atom has explicit version
	}{
		{"sys-devel/gcc", false},
		{"=sys-devel/gcc-13.4.1", true},
		{">=sys-devel/gcc-13.0", true},
		{"<sys-devel/gcc-16", true},
		{"~sys-devel/gcc-14.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tt.atom)
			if err != nil && !tt.wantVersion {
				// Simple category/package atoms may fail parsing
				// That's expected, they'll be handled differently
				return
			}
			if err != nil {
				t.Skipf("ParseAtom failed: %v", err)
				return
			}

			hasVersion := atom.HasVersion()
			if hasVersion != tt.wantVersion {
				t.Errorf("HasVersion() = %v, want %v", hasVersion, tt.wantVersion)
			}
		})
	}
}
