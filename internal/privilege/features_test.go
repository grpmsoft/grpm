package privilege

import (
	"reflect"
	"testing"
)

func TestFeaturesFromPortage(t *testing.T) {
	tests := []struct {
		name            string
		portageFeatures []string
		want            Features
	}{
		{
			name:            "empty",
			portageFeatures: []string{},
			want:            Features{},
		},
		{
			name:            "userpriv_only",
			portageFeatures: []string{"userpriv"},
			want:            Features{UserPriv: true},
		},
		{
			name:            "userfetch_only",
			portageFeatures: []string{"userfetch"},
			want:            Features{UserFetch: true},
		},
		{
			name:            "usersandbox_only",
			portageFeatures: []string{"usersandbox"},
			want:            Features{UserSandbox: true},
		},
		{
			name:            "all_privilege_features",
			portageFeatures: []string{"userpriv", "userfetch", "usersandbox"},
			want:            Features{UserPriv: true, UserFetch: true, UserSandbox: true},
		},
		{
			name:            "mixed_with_sandbox",
			portageFeatures: []string{"sandbox", "userpriv", "network-sandbox", "userfetch"},
			want:            Features{UserPriv: true, UserFetch: true},
		},
		{
			name:            "unrelated_features",
			portageFeatures: []string{"sandbox", "network-sandbox", "pid-sandbox"},
			want:            Features{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FeaturesFromPortage(tt.portageFeatures)
			if got != tt.want {
				t.Errorf("FeaturesFromPortage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDefaultFeatures(t *testing.T) {
	f := DefaultFeatures()

	if !f.UserPriv {
		t.Error("DefaultFeatures().UserPriv = false, want true")
	}

	if !f.UserFetch {
		t.Error("DefaultFeatures().UserFetch = false, want true")
	}

	if f.UserSandbox {
		t.Error("DefaultFeatures().UserSandbox = true, want false")
	}
}

func TestStrictFeatures(t *testing.T) {
	f := StrictFeatures()

	if !f.UserPriv {
		t.Error("StrictFeatures().UserPriv = false, want true")
	}

	if !f.UserFetch {
		t.Error("StrictFeatures().UserFetch = false, want true")
	}

	if !f.UserSandbox {
		t.Error("StrictFeatures().UserSandbox = false, want true")
	}
}

func TestNoopFeatures(t *testing.T) {
	f := NoopFeatures()

	if f.UserPriv {
		t.Error("NoopFeatures().UserPriv = true, want false")
	}

	if f.UserFetch {
		t.Error("NoopFeatures().UserFetch = true, want false")
	}

	if f.UserSandbox {
		t.Error("NoopFeatures().UserSandbox = true, want false")
	}
}

func TestHasPrivilegeFeature(t *testing.T) {
	tests := []struct {
		name     string
		features []string
		want     bool
	}{
		{
			name:     "empty",
			features: []string{},
			want:     false,
		},
		{
			name:     "has_userpriv",
			features: []string{"sandbox", "userpriv", "network-sandbox"},
			want:     true,
		},
		{
			name:     "no_userpriv",
			features: []string{"sandbox", "network-sandbox"},
			want:     false,
		},
		{
			name:     "only_userpriv",
			features: []string{"userpriv"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPrivilegeFeature(tt.features); got != tt.want {
				t.Errorf("HasPrivilegeFeature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasFetchFeature(t *testing.T) {
	tests := []struct {
		name     string
		features []string
		want     bool
	}{
		{
			name:     "empty",
			features: []string{},
			want:     false,
		},
		{
			name:     "has_userfetch",
			features: []string{"sandbox", "userfetch", "userpriv"},
			want:     true,
		},
		{
			name:     "no_userfetch",
			features: []string{"sandbox", "userpriv"},
			want:     false,
		},
		{
			name:     "only_userfetch",
			features: []string{"userfetch"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasFetchFeature(tt.features); got != tt.want {
				t.Errorf("HasFetchFeature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllPhases(t *testing.T) {
	phases := AllPhases()

	if len(phases) != 12 {
		t.Errorf("AllPhases() has %d phases, want 12", len(phases))
	}

	// Verify all expected phases are present
	expected := map[string]bool{
		"fetch":     true,
		"unpack":    true,
		"prepare":   true,
		"configure": true,
		"compile":   true,
		"test":      true,
		"install":   true,
		"preinst":   true,
		"postinst":  true,
		"prerm":     true,
		"postrm":    true,
		"qmerge":    true,
	}

	for _, phase := range phases {
		if !expected[phase] {
			t.Errorf("Unexpected phase %q in AllPhases()", phase)
		}
		delete(expected, phase)
	}

	if len(expected) > 0 {
		t.Errorf("Missing phases in AllPhases(): %v", expected)
	}
}

func TestBuildPhases(t *testing.T) {
	phases := BuildPhases()

	expected := []string{"fetch", "unpack", "prepare", "configure", "compile", "test", "install"}

	if !reflect.DeepEqual(phases, expected) {
		t.Errorf("BuildPhases() = %v, want %v", phases, expected)
	}

	// All build phases should NOT require root
	for _, phase := range phases {
		if RequiresRoot(phase) {
			t.Errorf("Build phase %q requires root, but should not", phase)
		}
	}
}

func TestMergePhases(t *testing.T) {
	phases := MergePhases()

	expected := []string{"preinst", "postinst", "prerm", "postrm", "qmerge"}

	if !reflect.DeepEqual(phases, expected) {
		t.Errorf("MergePhases() = %v, want %v", phases, expected)
	}

	// All merge phases SHOULD require root
	for _, phase := range phases {
		if !RequiresRoot(phase) {
			t.Errorf("Merge phase %q does not require root, but should", phase)
		}
	}
}

func TestPhaseCategorization(t *testing.T) {
	// Ensure AllPhases = BuildPhases + MergePhases
	all := AllPhases()
	build := BuildPhases()
	merge := MergePhases()

	if len(all) != len(build)+len(merge) {
		t.Errorf("AllPhases() count (%d) != BuildPhases() (%d) + MergePhases() (%d)",
			len(all), len(build), len(merge))
	}

	// Create a map of all phases
	allMap := make(map[string]bool)
	for _, p := range all {
		allMap[p] = true
	}

	// Verify all build phases are in all
	for _, p := range build {
		if !allMap[p] {
			t.Errorf("Build phase %q not in AllPhases()", p)
		}
	}

	// Verify all merge phases are in all
	for _, p := range merge {
		if !allMap[p] {
			t.Errorf("Merge phase %q not in AllPhases()", p)
		}
	}
}

// TestFeaturesFromPortage_NilSlice tests with nil slice.
func TestFeaturesFromPortage_NilSlice(t *testing.T) {
	var features []string
	f := FeaturesFromPortage(features)

	if f.UserPriv || f.UserFetch || f.UserSandbox {
		t.Error("FeaturesFromPortage(nil) should return all false features")
	}
}

// TestFeaturesFromPortage_DuplicateFeatures tests with duplicate features.
func TestFeaturesFromPortage_DuplicateFeatures(t *testing.T) {
	features := []string{"userpriv", "userpriv", "userpriv"}
	f := FeaturesFromPortage(features)

	if !f.UserPriv {
		t.Error("FeaturesFromPortage should handle duplicates")
	}
}

// TestBuildPhases_Content tests the specific content of BuildPhases.
func TestBuildPhases_Content(t *testing.T) {
	phases := BuildPhases()

	expected := map[string]bool{
		"fetch":     true,
		"unpack":    true,
		"prepare":   true,
		"configure": true,
		"compile":   true,
		"test":      true,
		"install":   true,
	}

	if len(phases) != len(expected) {
		t.Errorf("BuildPhases() has %d phases, want %d", len(phases), len(expected))
	}

	for _, phase := range phases {
		if !expected[phase] {
			t.Errorf("Unexpected phase %q in BuildPhases()", phase)
		}
	}
}

// TestMergePhases_Content tests the specific content of MergePhases.
func TestMergePhases_Content(t *testing.T) {
	phases := MergePhases()

	expected := map[string]bool{
		"preinst":  true,
		"postinst": true,
		"prerm":    true,
		"postrm":   true,
		"qmerge":   true,
	}

	if len(phases) != len(expected) {
		t.Errorf("MergePhases() has %d phases, want %d", len(phases), len(expected))
	}

	for _, phase := range phases {
		if !expected[phase] {
			t.Errorf("Unexpected phase %q in MergePhases()", phase)
		}
	}
}

// TestAllPhases_Uniqueness tests that all phases are unique.
func TestAllPhases_Uniqueness(t *testing.T) {
	phases := AllPhases()
	seen := make(map[string]bool)

	for _, phase := range phases {
		if seen[phase] {
			t.Errorf("Duplicate phase %q in AllPhases()", phase)
		}
		seen[phase] = true
	}
}

// TestBuildPhases_NoOverlap tests that build and merge phases don't overlap.
func TestBuildPhases_NoOverlap(t *testing.T) {
	build := make(map[string]bool)
	for _, p := range BuildPhases() {
		build[p] = true
	}

	for _, p := range MergePhases() {
		if build[p] {
			t.Errorf("Phase %q appears in both BuildPhases() and MergePhases()", p)
		}
	}
}

// TestFeaturesComparison tests Features struct comparison.
func TestFeaturesComparison(t *testing.T) {
	f1 := Features{UserPriv: true, UserFetch: false, UserSandbox: false}
	f2 := Features{UserPriv: true, UserFetch: false, UserSandbox: false}
	f3 := Features{UserPriv: false, UserFetch: true, UserSandbox: false}

	if f1 != f2 {
		t.Error("Equal features should be comparable")
	}

	if f1 == f3 {
		t.Error("Different features should not be equal")
	}
}

// TestDefaultFeatures_Stability tests that DefaultFeatures returns same values.
func TestDefaultFeatures_Stability(t *testing.T) {
	f1 := DefaultFeatures()
	f2 := DefaultFeatures()

	if f1 != f2 {
		t.Error("DefaultFeatures() should return consistent values")
	}
}

// TestStrictFeatures_Stability tests that StrictFeatures returns same values.
func TestStrictFeatures_Stability(t *testing.T) {
	f1 := StrictFeatures()
	f2 := StrictFeatures()

	if f1 != f2 {
		t.Error("StrictFeatures() should return consistent values")
	}
}

// TestNoopFeatures_Stability tests that NoopFeatures returns same values.
func TestNoopFeatures_Stability(t *testing.T) {
	f1 := NoopFeatures()
	f2 := NoopFeatures()

	if f1 != f2 {
		t.Error("NoopFeatures() should return consistent values")
	}
}

// BenchmarkFeaturesFromPortage benchmarks feature parsing.
func BenchmarkFeaturesFromPortage(b *testing.B) {
	features := []string{"sandbox", "userpriv", "userfetch", "network-sandbox", "usersandbox"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FeaturesFromPortage(features)
	}
}

// BenchmarkHasPrivilegeFeature benchmarks privilege feature check.
func BenchmarkHasPrivilegeFeature(b *testing.B) {
	features := []string{"sandbox", "userpriv", "userfetch", "network-sandbox"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HasPrivilegeFeature(features)
	}
}

// BenchmarkHasFetchFeature benchmarks fetch feature check.
func BenchmarkHasFetchFeature(b *testing.B) {
	features := []string{"sandbox", "userpriv", "userfetch", "network-sandbox"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HasFetchFeature(features)
	}
}

// BenchmarkAllPhases benchmarks phase retrieval.
func BenchmarkAllPhases(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AllPhases()
	}
}

// BenchmarkBuildPhases benchmarks build phase retrieval.
func BenchmarkBuildPhases(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = BuildPhases()
	}
}

// BenchmarkMergePhases benchmarks merge phase retrieval.
func BenchmarkMergePhases(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MergePhases()
	}
}
