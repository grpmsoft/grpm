package binpkg

import (
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestSelectionStrategy_String(t *testing.T) {
	tests := []struct {
		strategy SelectionStrategy
		expected string
	}{
		{StrategyPreferBinary, "prefer-binary"},
		{StrategyPreferSource, "prefer-source"},
		{StrategyBinaryOnly, "binary-only"},
		{StrategySourceOnly, "source-only"},
		{StrategyAuto, "auto"},
		{SelectionStrategy(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.strategy.String()
		if result != tt.expected {
			t.Errorf("SelectionStrategy(%d).String() = %q, want %q", tt.strategy, result, tt.expected)
		}
	}
}

func TestDefaultSelectorOptions(t *testing.T) {
	opts := DefaultSelectorOptions()

	if opts.Strategy != StrategyPreferBinary {
		t.Errorf("Strategy = %v, want %v", opts.Strategy, StrategyPreferBinary)
	}

	expectedMaxAge := 30 * 24 * time.Hour
	if opts.MaxAge != expectedMaxAge {
		t.Errorf("MaxAge = %v, want %v", opts.MaxAge, expectedMaxAge)
	}

	if opts.MinCoverage != 0.9 {
		t.Errorf("MinCoverage = %v, want 0.9", opts.MinCoverage)
	}

	if !opts.AllowUntrusted {
		t.Error("AllowUntrusted should be true by default")
	}

	if !opts.PreferLocal {
		t.Error("PreferLocal should be true by default")
	}
}

func TestNewSelector(t *testing.T) {
	opts := DefaultSelectorOptions()
	selector := NewSelector(opts)

	if selector == nil {
		t.Fatal("NewSelector() returned nil")
	}

	if selector.Options.Strategy != opts.Strategy {
		t.Errorf("Options.Strategy = %v, want %v", selector.Options.Strategy, opts.Strategy)
	}

	if len(selector.Binhosts) != 0 {
		t.Errorf("Binhosts length = %d, want 0", len(selector.Binhosts))
	}
}

func TestSelector_AddBinhost(t *testing.T) {
	selector := NewSelector(DefaultSelectorOptions())

	binhost := &Binhost{
		URI:      "file:///var/cache/binpkgs",
		Type:     BinhostLocal,
		Packages: []*BinaryPackage{},
	}

	selector.AddBinhost(binhost)

	if len(selector.Binhosts) != 1 {
		t.Errorf("Binhosts length = %d, want 1", len(selector.Binhosts))
	}

	if selector.Binhosts[0] != binhost {
		t.Error("Binhost not added correctly")
	}
}

func TestSelector_Select_SourceOnly(t *testing.T) {
	opts := SelectorOptions{
		Strategy: StrategySourceOnly,
	}
	selector := NewSelector(opts)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	result, err := selector.Select(p)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.UseBinary {
		t.Error("UseBinary should be false for source-only strategy")
	}

	if result.Reason != "strategy is source-only" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestSelector_Select_BinaryOnly_NotFound(t *testing.T) {
	opts := SelectorOptions{
		Strategy: StrategyBinaryOnly,
	}
	selector := NewSelector(opts)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	_, err := selector.Select(p)
	if err == nil {
		t.Error("Select() should fail when no binary found with binary-only strategy")
	}
}

func TestSelector_Select_PreferBinary_NoBinhost(t *testing.T) {
	opts := SelectorOptions{
		Strategy: StrategyPreferBinary,
	}
	selector := NewSelector(opts)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	result, err := selector.Select(p)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.UseBinary {
		t.Error("UseBinary should be false when no binhosts available")
	}

	if result.Reason != "no compatible binary found" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestSelector_Select_PreferBinary_WithMatch(t *testing.T) {
	opts := SelectorOptions{
		Strategy:       StrategyPreferBinary,
		MinCoverage:    0.5,
		AllowUntrusted: true,
		MaxAge:         30 * 24 * time.Hour,
	}
	selector := NewSelector(opts)

	// Add binhost with matching package
	binhost := &Binhost{
		URI:  "file:///var/cache/binpkgs",
		Type: BinhostLocal,
		Packages: []*BinaryPackage{
			{
				Package: &pkg.Package{
					Name:    "sys-libs/zlib",
					Version: "1.2.13",
				},
				Format: FormatGPKG,
				BuildInfo: &BuildMetadata{
					USE:       []string{"ssl"},
					BuildDate: time.Now().Add(-1 * time.Hour),
				},
			},
		},
	}
	selector.AddBinhost(binhost)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	result, err := selector.Select(p)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if !result.UseBinary {
		t.Error("UseBinary should be true when compatible binary found")
	}

	if result.BinaryPackage == nil {
		t.Error("BinaryPackage should not be nil")
	}

	if result.Binhost != binhost {
		t.Error("Binhost should be set")
	}
}

func TestSelector_Select_PreferSource_LowScore(t *testing.T) {
	opts := SelectorOptions{
		Strategy:       StrategyPreferSource,
		MinCoverage:    0.9,
		AllowUntrusted: true,
		RequiredUSE:    []string{"ssl", "python", "test"},
	}
	selector := NewSelector(opts)

	// Add binhost with low-score package (missing required USE flags)
	binhost := &Binhost{
		URI:  "file:///var/cache/binpkgs",
		Type: BinhostLocal,
		Packages: []*BinaryPackage{
			{
				Package: &pkg.Package{
					Name:    "sys-libs/zlib",
					Version: "1.2.13",
				},
				Format: FormatGPKG,
				BuildInfo: &BuildMetadata{
					USE:       []string{"ssl"}, // Only 1 of 3 required flags
					BuildDate: time.Now(),
				},
			},
		},
	}
	selector.AddBinhost(binhost)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	result, err := selector.Select(p)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Should prefer source because binary score is low
	if result.UseBinary {
		t.Error("UseBinary should be false for low-score binary with prefer-source strategy")
	}
}

func TestSelector_Select_Auto(t *testing.T) {
	opts := SelectorOptions{
		Strategy:       StrategyAuto,
		MinCoverage:    0.9,
		AllowUntrusted: true,
		MaxAge:         30 * 24 * time.Hour,
	}
	selector := NewSelector(opts)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	result, err := selector.Select(p)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Should fall back to source when no binary available
	if result.UseBinary {
		t.Error("UseBinary should be false when no binary available")
	}
}

func TestSelector_scorePackage(t *testing.T) {
	selector := NewSelector(SelectorOptions{
		RequiredUSE: []string{"ssl", "python"},
		MaxAge:      30 * 24 * time.Hour,
	})

	tests := []struct {
		name      string
		binPkg    *BinaryPackage
		wantScore float64
		wantRange [2]float64 // min, max
	}{
		{
			name:      "nil_buildinfo",
			binPkg:    &BinaryPackage{BuildInfo: nil},
			wantScore: 0.0,
		},
		{
			name: "incompatible_use",
			binPkg: &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE: []string{}, // Missing required flags
				},
			},
			wantScore: 0.0,
		},
		{
			name: "full_match",
			binPkg: &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE:       []string{"ssl", "python"},
					BuildDate: time.Now(),
				},
			},
			wantRange: [2]float64{0.9, 1.0}, // Full match with slight age adjustment
		},
		{
			name: "partial_match",
			binPkg: &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE:       []string{"ssl"}, // Only 1 of 2
					BuildDate: time.Now(),
				},
			},
			// IsCompatible returns false when any required flag is missing,
			// so scorePackage returns 0.0
			wantScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := selector.scorePackage(tt.binPkg, &pkg.Package{Name: "test"})

			if tt.wantScore != 0 {
				if score != tt.wantScore {
					t.Errorf("scorePackage() = %v, want %v", score, tt.wantScore)
				}
			}

			if tt.wantRange != [2]float64{} {
				if score < tt.wantRange[0] || score > tt.wantRange[1] {
					t.Errorf("scorePackage() = %v, want in range [%v, %v]", score, tt.wantRange[0], tt.wantRange[1])
				}
			}
		})
	}
}

func TestSelector_scorePackage_NoRequiredUSE(t *testing.T) {
	selector := NewSelector(SelectorOptions{
		RequiredUSE: []string{}, // No required flags
		MaxAge:      30 * 24 * time.Hour,
	})

	binPkg := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE:       []string{"ssl"},
			BuildDate: time.Now(),
		},
	}

	score := selector.scorePackage(binPkg, &pkg.Package{Name: "test"})
	if score != 1.0 {
		t.Errorf("scorePackage() = %v, want 1.0 (no requirements)", score)
	}
}

func TestSelector_scorePackage_AgeEffect(t *testing.T) {
	// Age penalty is only applied when there are RequiredUSE flags
	// because when RequiredUSE is empty, the function returns 1.0 immediately
	selector := NewSelector(SelectorOptions{
		RequiredUSE: []string{"ssl"},     // Need at least one flag for age to apply
		MaxAge:      10 * 24 * time.Hour, // 10 days max age
	})

	// Fresh package with required flag
	freshPkg := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE:       []string{"ssl"},
			BuildDate: time.Now(),
		},
	}

	// Old package (at max age) with required flag
	oldPkg := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE:       []string{"ssl"},
			BuildDate: time.Now().Add(-10 * 24 * time.Hour),
		},
	}

	freshScore := selector.scorePackage(freshPkg, &pkg.Package{Name: "test"})
	oldScore := selector.scorePackage(oldPkg, &pkg.Package{Name: "test"})

	if freshScore <= oldScore {
		t.Errorf("Fresh package score (%v) should be > old package score (%v)", freshScore, oldScore)
	}
}

func TestSelector_GetStats(t *testing.T) {
	selector := NewSelector(DefaultSelectorOptions())

	// Empty selector
	stats := selector.GetStats()
	if stats.TotalBinhosts != 0 {
		t.Errorf("TotalBinhosts = %d, want 0", stats.TotalBinhosts)
	}

	// Add binhosts with packages
	buildTime := time.Now().Add(-24 * time.Hour)
	selector.AddBinhost(&Binhost{
		URI:  "file:///local",
		Type: BinhostLocal,
		Packages: []*BinaryPackage{
			{
				Package:   &pkg.Package{Name: "pkg1"},
				BuildInfo: &BuildMetadata{BuildDate: buildTime},
			},
		},
	})

	selector.AddBinhost(&Binhost{
		URI:  "https://remote",
		Type: BinhostHTTP,
		Packages: []*BinaryPackage{
			{
				Package:   &pkg.Package{Name: "pkg2"},
				BuildInfo: &BuildMetadata{BuildDate: time.Now()},
			},
		},
	})

	stats = selector.GetStats()
	if stats.TotalBinhosts != 2 {
		t.Errorf("TotalBinhosts = %d, want 2", stats.TotalBinhosts)
	}

	if stats.TotalPackages != 2 {
		t.Errorf("TotalPackages = %d, want 2", stats.TotalPackages)
	}

	if stats.LocalPackages != 1 {
		t.Errorf("LocalPackages = %d, want 1", stats.LocalPackages)
	}

	if stats.RemotePackages != 1 {
		t.Errorf("RemotePackages = %d, want 1", stats.RemotePackages)
	}
}

func TestSelector_Select_UnknownStrategy(t *testing.T) {
	opts := SelectorOptions{
		Strategy: SelectionStrategy(99), // Unknown strategy
	}
	selector := NewSelector(opts)

	p := &pkg.Package{Name: "test"}
	_, err := selector.Select(p)
	if err == nil {
		t.Error("Select() should fail for unknown strategy")
	}
}

func TestSelector_findBestBinary_RequiresSignature(t *testing.T) {
	opts := SelectorOptions{
		Strategy:       StrategyPreferBinary,
		AllowUntrusted: false, // Require signature
		MinCoverage:    0.5,
	}
	selector := NewSelector(opts)

	// Add package without signature
	selector.AddBinhost(&Binhost{
		URI:  "file:///local",
		Type: BinhostLocal,
		Packages: []*BinaryPackage{
			{
				Package:   &pkg.Package{Name: "test"},
				Signature: nil, // No signature
				BuildInfo: &BuildMetadata{USE: []string{}, BuildDate: time.Now()},
			},
		},
	})

	result, err := selector.findBestBinary(&pkg.Package{Name: "test"})
	if err != nil {
		t.Fatalf("findBestBinary() error = %v", err)
	}

	// Should not find any match because signature is required
	if result != nil {
		t.Error("findBestBinary() should return nil when signature required but not present")
	}
}

func TestSelector_findBestBinary_TooOld(t *testing.T) {
	opts := SelectorOptions{
		Strategy:       StrategyPreferBinary,
		AllowUntrusted: true,
		MaxAge:         24 * time.Hour, // 1 day max
		MinCoverage:    0.5,
	}
	selector := NewSelector(opts)

	// Add old package
	selector.AddBinhost(&Binhost{
		URI:  "file:///local",
		Type: BinhostLocal,
		Packages: []*BinaryPackage{
			{
				Package: &pkg.Package{Name: "test"},
				BuildInfo: &BuildMetadata{
					USE:       []string{},
					BuildDate: time.Now().Add(-48 * time.Hour), // 2 days old
				},
			},
		},
	})

	result, err := selector.findBestBinary(&pkg.Package{Name: "test"})
	if err != nil {
		t.Fatalf("findBestBinary() error = %v", err)
	}

	// Should not find match because package is too old
	if result != nil {
		t.Error("findBestBinary() should return nil when package is too old")
	}
}

func BenchmarkSelector_Select(b *testing.B) {
	selector := NewSelector(DefaultSelectorOptions())

	// Add binhost with packages
	packages := make([]*BinaryPackage, 100)
	for i := 0; i < 100; i++ {
		packages[i] = &BinaryPackage{
			Package: &pkg.Package{Name: "pkg" + string(rune('0'+i%10))},
			BuildInfo: &BuildMetadata{
				USE:       []string{"ssl", "python"},
				BuildDate: time.Now(),
			},
		}
	}

	selector.AddBinhost(&Binhost{
		URI:      "file:///local",
		Type:     BinhostLocal,
		Packages: packages,
	})

	p := &pkg.Package{Name: "pkg5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = selector.Select(p)
	}
}

func BenchmarkSelector_scorePackage(b *testing.B) {
	selector := NewSelector(SelectorOptions{
		RequiredUSE: []string{"ssl", "python", "xml", "unicode"},
		MaxAge:      30 * 24 * time.Hour,
	})

	binPkg := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE:       []string{"ssl", "python", "xml"},
			BuildDate: time.Now().Add(-24 * time.Hour),
		},
	}

	p := &pkg.Package{Name: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = selector.scorePackage(binPkg, p)
	}
}
