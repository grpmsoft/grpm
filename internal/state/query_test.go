package state

import (
	"reflect"
	"testing"
	"time"
)

func TestQuery(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	// Add test packages
	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", []InstalledFile{
		{Path: "/usr/lib/libz.so", Type: FileTypeRegular},
	})
	pkg1.InstallTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	pkg1.USE = []string{"ssl", "unicode"}
	pkg1.Size = 100 * 1024 // 100KB

	pkg2 := createTestPackage("app-editors/vim", "9.0", []InstalledFile{
		{Path: "/usr/bin/vim", Type: FileTypeRegular},
	})
	pkg2.InstallTime = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	pkg2.USE = []string{"python", "ruby"}
	pkg2.Size = 5 * 1024 * 1024 // 5MB

	pkg3 := createTestPackage("sys-libs/glibc", "2.38", []InstalledFile{
		{Path: "/lib64/libc.so.6", Type: FileTypeRegular},
	})
	pkg3.InstallTime = time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	pkg3.USE = []string{"ssl"}
	pkg3.Size = 10 * 1024 * 1024 // 10MB

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)
	_ = db.Add(pkg3)

	tests := []struct {
		name     string
		spec     QuerySpec
		wantLen  int
		wantName string
	}{
		{
			name:    "query by category",
			spec:    QuerySpec{Category: "sys-libs"},
			wantLen: 2,
		},
		{
			name:    "query by name pattern (prefix)",
			spec:    QuerySpec{NamePattern: "sys-libs/*"},
			wantLen: 2,
		},
		{
			name:     "query by name pattern (contains)",
			spec:     QuerySpec{NamePattern: "*vim*"},
			wantLen:  1,
			wantName: "app-editors/vim",
		},
		{
			name:    "query by USE flag",
			spec:    QuerySpec{HasUSEFlag: "ssl"},
			wantLen: 2,
		},
		{
			name:    "query by install time (after)",
			spec:    QuerySpec{InstalledAfter: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			wantLen: 1,
		},
		{
			name:    "query by install time (before)",
			spec:    QuerySpec{InstalledBefore: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			wantLen: 2,
		},
		{
			name:    "query by size (min)",
			spec:    QuerySpec{MinSize: 1 * 1024 * 1024},
			wantLen: 2, // vim and glibc
		},
		{
			name:    "query by size (max)",
			spec:    QuerySpec{MaxSize: 1 * 1024 * 1024},
			wantLen: 1, // zlib
		},
		{
			name:    "query by file ownership",
			spec:    QuerySpec{OwnsFile: "/usr/bin/vim"},
			wantLen: 1,
		},
		{
			name:    "query with limit",
			spec:    QuerySpec{Limit: 2},
			wantLen: 2,
		},
		{
			name:    "empty query (all packages)",
			spec:    QuerySpec{},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := db.Query(tt.spec)
			if len(results) != tt.wantLen {
				t.Errorf("Query() returned %d results, want %d", len(results), tt.wantLen)
			}

			if tt.wantName != "" && len(results) > 0 {
				if results[0].Package.Name != tt.wantName {
					t.Errorf("Query() returned %s, want %s", results[0].Package.Name, tt.wantName)
				}
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		{
			name:    "exact match",
			s:       "zlib",
			pattern: "zlib",
			want:    true,
		},
		{
			name:    "prefix match",
			s:       "zlib-1.2.13",
			pattern: "zlib*",
			want:    true,
		},
		{
			name:    "suffix match",
			s:       "sys-libs/zlib",
			pattern: "*zlib",
			want:    true,
		},
		{
			name:    "contains match",
			s:       "sys-libs/zlib",
			pattern: "*zlib*",
			want:    true,
		},
		{
			name:    "no match",
			s:       "vim",
			pattern: "zlib*",
			want:    false,
		},
		{
			name:    "empty pattern matches all",
			s:       "anything",
			pattern: "",
			want:    true,
		},
		{
			name:    "simple contains pattern",
			s:       "sys-libs/zlib",
			pattern: "*lib*",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestHasUSEFlag(t *testing.T) {
	tests := []struct {
		name     string
		useFlags []string
		flag     string
		want     bool
	}{
		{
			name:     "flag present",
			useFlags: []string{"ssl", "unicode", "python"},
			flag:     "ssl",
			want:     true,
		},
		{
			name:     "flag not present",
			useFlags: []string{"ssl", "unicode"},
			flag:     "python",
			want:     false,
		},
		{
			name:     "empty flags",
			useFlags: []string{},
			flag:     "ssl",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasUSEFlag(tt.useFlags, tt.flag)
			if got != tt.want {
				t.Errorf("hasUSEFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOwnsFile(t *testing.T) {
	pkg := &InstalledPackage{
		Files: []InstalledFile{
			{Path: "/usr/bin/test"},
			{Path: "/usr/lib/test.so"},
		},
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "owns file",
			path: "/usr/bin/test",
			want: true,
		},
		{
			name: "does not own file",
			path: "/usr/bin/other",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ownsFile(pkg, tt.path)
			if got != tt.want {
				t.Errorf("ownsFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindByCategory(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg2 := createTestPackage("sys-libs/glibc", "2.38", nil)
	pkg3 := createTestPackage("app-editors/vim", "9.0", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)
	_ = db.Add(pkg3)

	results := db.FindByCategory("sys-libs")

	if len(results) != 2 {
		t.Errorf("FindByCategory() returned %d results, want 2", len(results))
	}
}

func TestFindByPattern(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg2 := createTestPackage("sys-libs/glibc", "2.38", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results := db.FindByPattern("*zlib*")

	if len(results) != 1 {
		t.Errorf("FindByPattern() returned %d results, want 1", len(results))
	}

	if len(results) > 0 && results[0].Package.Name != "sys-libs/zlib" {
		t.Errorf("FindByPattern() returned wrong package: %s", results[0].Package.Name)
	}
}

func TestFindInstalledAfter(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg1.InstallTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)
	pkg2.InstallTime = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results := db.FindInstalledAfter(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC))

	if len(results) != 1 {
		t.Errorf("FindInstalledAfter() returned %d results, want 1", len(results))
	}
}

func TestFindInstalledBefore(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg1.InstallTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)
	pkg2.InstallTime = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results := db.FindInstalledBefore(time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC))

	if len(results) != 1 {
		t.Errorf("FindInstalledBefore() returned %d results, want 1", len(results))
	}
}

func TestFindWithUSEFlag(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg1.USE = []string{"ssl", "unicode"}

	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)
	pkg2.USE = []string{"python", "ruby"}

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results := db.FindWithUSEFlag("ssl")

	if len(results) != 1 {
		t.Errorf("FindWithUSEFlag() returned %d results, want 1", len(results))
	}
}

func TestFindLargePackages(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg1.Size = 100 * 1024 // 100KB

	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)
	pkg2.Size = 10 * 1024 * 1024 // 10MB

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results := db.FindLargePackages(1 * 1024 * 1024) // > 1MB

	if len(results) != 1 {
		t.Errorf("FindLargePackages() returned %d results, want 1", len(results))
	}

	if len(results) > 0 && results[0].Package.Name != "app-editors/vim" {
		t.Errorf("FindLargePackages() returned wrong package: %s", results[0].Package.Name)
	}
}

func TestMatchesQuery_CategoryExtraction(t *testing.T) {
	testPkg := createTestPackage("sys-libs/zlib", "1.2.13", nil)

	// Test category matching
	if !matchesQuery(testPkg, QuerySpec{Category: "sys-libs"}) {
		t.Error("Expected category to match")
	}

	if matchesQuery(testPkg, QuerySpec{Category: "app-editors"}) {
		t.Error("Expected category not to match")
	}

	// Test invalid package name (no category)
	pkgInvalid := createTestPackage("invalid-no-slash", "1.0.0", nil)

	if matchesQuery(pkgInvalid, QuerySpec{Category: "sys-libs"}) {
		t.Error("Expected invalid package name not to match category")
	}
}

func TestQuerySpec_Empty(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	// Empty query should return all packages
	results := db.Query(QuerySpec{})

	if len(results) != 2 {
		t.Errorf("Empty Query() returned %d results, want 2", len(results))
	}
}

func TestQuerySpec_CombinedFilters(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg1.InstallTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	pkg1.USE = []string{"ssl"}
	pkg1.Size = 100 * 1024

	pkg2 := createTestPackage("sys-libs/glibc", "2.38", nil)
	pkg2.InstallTime = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	pkg2.USE = []string{"ssl", "unicode"}
	pkg2.Size = 10 * 1024 * 1024

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	// Combined filters: category + USE flag + min size
	results := db.Query(QuerySpec{
		Category:   "sys-libs",
		HasUSEFlag: "ssl",
		MinSize:    1 * 1024 * 1024,
	})

	if len(results) != 1 {
		t.Errorf("Combined Query() returned %d results, want 1", len(results))
	}

	if len(results) > 0 && results[0].Package.Name != "sys-libs/glibc" {
		t.Errorf("Combined Query() returned wrong package: %s", results[0].Package.Name)
	}
}

func TestMatchPattern_EdgeCases(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		want    bool
	}{
		// Multiple wildcards
		// TODO: Improve matchPattern for complex patterns with multiple wildcards
		// Current implementation only handles simple cases
		{"abc", "a*b*c", true},
		{"ad", "a*c", false},

		// Edge cases
		{"", "", true},
		{"test", "*", true},
		{"test", "**", true},
		{"test", "***", true},
	}

	for _, tt := range tests {
		got := matchPattern(tt.s, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
		}
	}
}

func BenchmarkQuery(b *testing.B) {
	db := NewPackageDatabase("/var/db/pkg")

	// Add 1000 packages
	for i := 0; i < 1000; i++ {
		pkg := createTestPackage("sys-libs/package-"+string(rune(i)), "1.0.0", nil)
		pkg.USE = []string{"ssl", "unicode"}
		pkg.Size = int64(i) * 1024
		_ = db.Add(pkg)
	}

	spec := QuerySpec{
		Category:   "sys-libs",
		HasUSEFlag: "ssl",
		MinSize:    500 * 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.Query(spec)
	}
}

func BenchmarkMatchPattern(b *testing.B) {
	s := "sys-libs/zlib"
	pattern := "*zlib*"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = matchPattern(s, pattern)
	}
}

func TestQuerySpec_ZeroValues(t *testing.T) {
	// Test that zero values don't affect query
	pkg := createTestPackage("sys-libs/zlib", "1.2.13", nil)

	spec := QuerySpec{
		InstalledAfter:  time.Time{}, // Zero value
		InstalledBefore: time.Time{}, // Zero value
		MinSize:         0,           // Zero value
		MaxSize:         0,           // Zero value
	}

	if !matchesQuery(pkg, spec) {
		t.Error("Zero-value QuerySpec should match all packages")
	}
}

func TestQueryResults_Ordering(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	// Results order should be consistent (map iteration may vary)
	pkg1 := createTestPackage("sys-libs/zlib", "1.2.13", nil)
	pkg2 := createTestPackage("app-editors/vim", "9.0", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	results1 := db.Query(QuerySpec{})
	results2 := db.Query(QuerySpec{})

	// Both queries should return same packages (order may vary)
	if len(results1) != len(results2) {
		t.Error("Query() should return consistent number of results")
	}

	// Verify same packages are in results
	names1 := make(map[string]bool)
	names2 := make(map[string]bool)

	for _, pkg := range results1 {
		names1[pkg.Package.Name] = true
	}
	for _, pkg := range results2 {
		names2[pkg.Package.Name] = true
	}

	if !reflect.DeepEqual(names1, names2) {
		t.Error("Query() should return same packages")
	}
}
