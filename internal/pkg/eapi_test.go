package pkg

import (
	"errors"
	"sort"
	"testing"
)

func TestGetEAPIFeatures(t *testing.T) {
	tests := []struct {
		name    string
		eapi    string
		wantErr bool
	}{
		{"EAPI 0", "0", false},
		{"EAPI 1", "1", false},
		{"EAPI 2", "2", false},
		{"EAPI 3", "3", false},
		{"EAPI 4", "4", false},
		{"EAPI 5", "5", false},
		{"EAPI 6", "6", false},
		{"EAPI 7", "7", false},
		{"EAPI 8", "8", false},
		{"EAPI 9 unsupported", "9", true},
		{"EAPI 10 unsupported", "10", true},
		{"empty EAPI", "", true},
		{"invalid EAPI string", "invalid", true},
		{"paludis reserved", "paludis-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features, err := GetEAPIFeatures(tt.eapi)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEAPIFeatures(%q) error = %v, wantErr %v", tt.eapi, err, tt.wantErr)
				return
			}
			if !tt.wantErr && features.Version != tt.eapi {
				t.Errorf("GetEAPIFeatures(%q).Version = %q, want %q", tt.eapi, features.Version, tt.eapi)
			}
		})
	}
}

func TestGetEAPIFeaturesError(t *testing.T) {
	_, err := GetEAPIFeatures("unknown")
	if err == nil {
		t.Error("expected error for unknown EAPI")
	}
	if !errors.Is(err, ErrUnsupportedEAPI) {
		t.Errorf("error = %v, want ErrUnsupportedEAPI", err)
	}
}

func TestMustGetEAPIFeatures(t *testing.T) {
	// Test valid EAPI
	features := MustGetEAPIFeatures("8")
	if features.Version != "8" {
		t.Errorf("Version = %q, want %q", features.Version, "8")
	}

	// Test panic on invalid EAPI
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid EAPI")
		}
	}()
	MustGetEAPIFeatures("invalid")
}

func TestIsEAPISupported(t *testing.T) {
	tests := []struct {
		eapi string
		want bool
	}{
		{"0", true},
		{"1", true},
		{"2", true},
		{"3", true},
		{"4", true},
		{"5", true},
		{"6", true},
		{"7", true},
		{"8", true},
		{"9", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.eapi, func(t *testing.T) {
			if got := IsEAPISupported(tt.eapi); got != tt.want {
				t.Errorf("IsEAPISupported(%q) = %v, want %v", tt.eapi, got, tt.want)
			}
		})
	}
}

func TestSupportedEAPIs(t *testing.T) {
	eapis := SupportedEAPIs()

	// Should have all EAPIs 0-8
	if len(eapis) != 9 {
		t.Errorf("len(SupportedEAPIs()) = %d, want 9", len(eapis))
	}

	// Sort for consistent testing
	sort.Strings(eapis)
	expected := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}
	for i, e := range expected {
		if eapis[i] != e {
			t.Errorf("SupportedEAPIs()[%d] = %q, want %q", i, eapis[i], e)
		}
	}
}

func TestLatestEAPI(t *testing.T) {
	if got := LatestEAPI(); got != "8" {
		t.Errorf("LatestEAPI() = %q, want %q", got, "8")
	}
}

func TestDefaultEAPI(t *testing.T) {
	if got := DefaultEAPI(); got != "0" {
		t.Errorf("DefaultEAPI() = %q, want %q", got, "0")
	}
}

// TestEAPI0Features tests EAPI 0 baseline features
func TestEAPI0Features(t *testing.T) {
	f := MustGetEAPIFeatures("0")

	// Bash version
	if f.BashVersion != "3.2" {
		t.Errorf("BashVersion = %q, want %q", f.BashVersion, "3.2")
	}
	if f.BashVersionMajor != 3 || f.BashVersionMinor != 2 {
		t.Errorf("BashVersionMajor/Minor = %d.%d, want 3.2", f.BashVersionMajor, f.BashVersionMinor)
	}

	// EAPI 0 specific behaviors
	if !f.RdependDefaultsToDepend {
		t.Error("RdependDefaultsToDepend should be true for EAPI 0")
	}
	if !f.EmptyGroupsMatch {
		t.Error("EmptyGroupsMatch should be true for EAPI 0")
	}

	// Features NOT available in EAPI 0
	if f.SlotDeps {
		t.Error("SlotDeps should be false for EAPI 0")
	}
	if f.IUSEDefaults {
		t.Error("IUSEDefaults should be false for EAPI 0")
	}
	if f.UseDeps {
		t.Error("UseDeps should be false for EAPI 0")
	}
	if f.BDEPEND {
		t.Error("BDEPEND should be false for EAPI 0")
	}
	if f.IDEPEND {
		t.Error("IDEPEND should be false for EAPI 0")
	}
	if f.RequiredUse {
		t.Error("RequiredUse should be false for EAPI 0")
	}
}

// TestEAPI1Features tests EAPI 1 new features
func TestEAPI1Features(t *testing.T) {
	f := MustGetEAPIFeatures("1")

	// New in EAPI 1
	if !f.SlotDeps {
		t.Error("SlotDeps should be true for EAPI 1")
	}
	if !f.IUSEDefaults {
		t.Error("IUSEDefaults should be true for EAPI 1")
	}

	// Still has RDEPEND default
	if !f.RdependDefaultsToDepend {
		t.Error("RdependDefaultsToDepend should be true for EAPI 1")
	}

	// Not yet available
	if f.UseDeps {
		t.Error("UseDeps should be false for EAPI 1")
	}
	if f.SrcURIArrows {
		t.Error("SrcURIArrows should be false for EAPI 1")
	}
}

// TestEAPI2Features tests EAPI 2 new features
func TestEAPI2Features(t *testing.T) {
	f := MustGetEAPIFeatures("2")

	// New in EAPI 2
	if !f.UseDeps {
		t.Error("UseDeps should be true for EAPI 2")
	}
	if !f.SrcURIArrows {
		t.Error("SrcURIArrows should be true for EAPI 2")
	}
	if !f.HasSrcPrepare {
		t.Error("HasSrcPrepare should be true for EAPI 2")
	}
	if !f.HasSrcConfigure {
		t.Error("HasSrcConfigure should be true for EAPI 2")
	}
	if f.DefaultSrcPrepareFormat != 2 {
		t.Errorf("DefaultSrcPrepareFormat = %d, want 2", f.DefaultSrcPrepareFormat)
	}

	// Not yet available
	if f.UseDepDefaults {
		t.Error("UseDepDefaults should be false for EAPI 2")
	}
	if f.RequiredUse {
		t.Error("RequiredUse should be false for EAPI 2")
	}
}

// TestEAPI4Features tests EAPI 4 new features
func TestEAPI4Features(t *testing.T) {
	f := MustGetEAPIFeatures("4")

	// New in EAPI 4
	if !f.UseDepDefaults {
		t.Error("UseDepDefaults should be true for EAPI 4")
	}
	if !f.RequiredUse {
		t.Error("RequiredUse should be true for EAPI 4")
	}
	if !f.Properties {
		t.Error("Properties should be true for EAPI 4")
	}
	if f.DefaultSrcInstallFormat != 4 {
		t.Errorf("DefaultSrcInstallFormat = %d, want 4", f.DefaultSrcInstallFormat)
	}

	// RDEPEND default removed
	if f.RdependDefaultsToDepend {
		t.Error("RdependDefaultsToDepend should be false for EAPI 4+")
	}

	// Not yet available
	if f.SlotOperators {
		t.Error("SlotOperators should be false for EAPI 4")
	}
	if f.SubSlots {
		t.Error("SubSlots should be false for EAPI 4")
	}
}

// TestEAPI5Features tests EAPI 5 new features
func TestEAPI5Features(t *testing.T) {
	f := MustGetEAPIFeatures("5")

	// New in EAPI 5
	if !f.SlotOperators {
		t.Error("SlotOperators should be true for EAPI 5")
	}
	if !f.SubSlots {
		t.Error("SubSlots should be true for EAPI 5")
	}
	if !f.IUSEEffective {
		t.Error("IUSEEffective should be true for EAPI 5")
	}
	if !f.RequiredUseAtMostOne {
		t.Error("RequiredUseAtMostOne should be true for EAPI 5")
	}
	if !f.Usev {
		t.Error("Usev should be true for EAPI 5")
	}
	if !f.UsexDefaultValues {
		t.Error("UsexDefaultValues should be true for EAPI 5")
	}

	// Still Bash 3.2
	if f.BashVersionMajor != 3 {
		t.Errorf("BashVersionMajor = %d, want 3", f.BashVersionMajor)
	}

	// Not yet available
	if f.Eapply {
		t.Error("Eapply should be false for EAPI 5")
	}
	if f.Failglob {
		t.Error("Failglob should be false for EAPI 5")
	}
}

// TestEAPI6Features tests EAPI 6 new features
func TestEAPI6Features(t *testing.T) {
	f := MustGetEAPIFeatures("6")

	// Bash version bump
	if f.BashVersion != "4.2" {
		t.Errorf("BashVersion = %q, want %q", f.BashVersion, "4.2")
	}
	if f.BashVersionMajor != 4 || f.BashVersionMinor != 2 {
		t.Errorf("BashVersionMajor/Minor = %d.%d, want 4.2", f.BashVersionMajor, f.BashVersionMinor)
	}

	// New in EAPI 6
	if !f.Failglob {
		t.Error("Failglob should be true for EAPI 6")
	}
	if !f.Eapply {
		t.Error("Eapply should be true for EAPI 6")
	}
	if !f.EapplyUser {
		t.Error("EapplyUser should be true for EAPI 6")
	}
	if !f.Einstalldocs {
		t.Error("Einstalldocs should be true for EAPI 6")
	}
	if !f.GetLibdir {
		t.Error("GetLibdir should be true for EAPI 6")
	}
	if f.DefaultSrcPrepareFormat != 6 {
		t.Errorf("DefaultSrcPrepareFormat = %d, want 6", f.DefaultSrcPrepareFormat)
	}
	if f.DefaultSrcInstallFormat != 6 {
		t.Errorf("DefaultSrcInstallFormat = %d, want 6", f.DefaultSrcInstallFormat)
	}

	// Not yet available
	if f.BDEPEND {
		t.Error("BDEPEND should be false for EAPI 6")
	}
	if f.ProfileDirectories {
		t.Error("ProfileDirectories should be false for EAPI 6")
	}
}

// TestEAPI7Features tests EAPI 7 new features
func TestEAPI7Features(t *testing.T) {
	f := MustGetEAPIFeatures("7")

	// New in EAPI 7
	if !f.BDEPEND {
		t.Error("BDEPEND should be true for EAPI 7")
	}
	if !f.Dostrip {
		t.Error("Dostrip should be true for EAPI 7")
	}
	if !f.ProfileDirectories {
		t.Error("ProfileDirectories should be true for EAPI 7")
	}
	if !f.EnvUnset {
		t.Error("EnvUnset should be true for EAPI 7")
	}

	// EmptyGroupsMatch changed
	if f.EmptyGroupsMatch {
		t.Error("EmptyGroupsMatch should be false for EAPI 7+")
	}

	// Still Bash 4.2
	if f.BashVersionMajor != 4 {
		t.Errorf("BashVersionMajor = %d, want 4", f.BashVersionMajor)
	}

	// Not yet available
	if f.IDEPEND {
		t.Error("IDEPEND should be false for EAPI 7")
	}
	if f.DosymRelative {
		t.Error("DosymRelative should be false for EAPI 7")
	}
	if f.SrcURISelectiveRestrictions {
		t.Error("SrcURISelectiveRestrictions should be false for EAPI 7")
	}
}

// TestEAPI8Features tests EAPI 8 new features
func TestEAPI8Features(t *testing.T) {
	f := MustGetEAPIFeatures("8")

	// Bash version bump
	if f.BashVersion != "5.0" {
		t.Errorf("BashVersion = %q, want %q", f.BashVersion, "5.0")
	}
	if f.BashVersionMajor != 5 || f.BashVersionMinor != 0 {
		t.Errorf("BashVersionMajor/Minor = %d.%d, want 5.0", f.BashVersionMajor, f.BashVersionMinor)
	}

	// New in EAPI 8
	if !f.IDEPEND {
		t.Error("IDEPEND should be true for EAPI 8")
	}
	if !f.DosymRelative {
		t.Error("DosymRelative should be true for EAPI 8")
	}
	if !f.SrcURISelectiveRestrictions {
		t.Error("SrcURISelectiveRestrictions should be true for EAPI 8")
	}
	if f.DefaultSrcPrepareFormat != 8 {
		t.Errorf("DefaultSrcPrepareFormat = %d, want 8", f.DefaultSrcPrepareFormat)
	}

	// All previous features should be present
	if !f.BDEPEND {
		t.Error("BDEPEND should be true for EAPI 8")
	}
	if !f.SlotOperators {
		t.Error("SlotOperators should be true for EAPI 8")
	}
	if !f.SubSlots {
		t.Error("SubSlots should be true for EAPI 8")
	}
	if !f.Eapply {
		t.Error("Eapply should be true for EAPI 8")
	}
	if !f.Failglob {
		t.Error("Failglob should be true for EAPI 8")
	}
}

// TestEAPIFeatureMethods tests the feature query methods
func TestEAPIFeatureMethods(t *testing.T) {
	tests := []struct {
		eapi     string
		method   string
		expected bool
	}{
		// SlotOperators: EAPI 5+
		{"4", "SupportsSlotOperators", false},
		{"5", "SupportsSlotOperators", true},
		{"8", "SupportsSlotOperators", true},
		// SubSlots: EAPI 5+
		{"4", "SupportsSubSlots", false},
		{"5", "SupportsSubSlots", true},
		// BDEPEND: EAPI 7+
		{"6", "SupportsBDEPEND", false},
		{"7", "SupportsBDEPEND", true},
		{"8", "SupportsBDEPEND", true},
		// IDEPEND: EAPI 8+
		{"7", "SupportsIDEPEND", false},
		{"8", "SupportsIDEPEND", true},
		// RequiredUse: EAPI 4+
		{"3", "SupportsRequiredUse", false},
		{"4", "SupportsRequiredUse", true},
		// DosymRelative: EAPI 8+
		{"7", "SupportsDosymRelative", false},
		{"8", "SupportsDosymRelative", true},
		// Eapply: EAPI 6+
		{"5", "SupportsEapply", false},
		{"6", "SupportsEapply", true},
		// SrcURIArrows: EAPI 2+
		{"1", "SupportsSrcURIArrows", false},
		{"2", "SupportsSrcURIArrows", true},
		// SrcURISelectiveRestrictions: EAPI 8+
		{"7", "SupportsSrcURISelectiveRestrictions", false},
		{"8", "SupportsSrcURISelectiveRestrictions", true},
	}

	for _, tt := range tests {
		t.Run(tt.eapi+"_"+tt.method, func(t *testing.T) {
			f := MustGetEAPIFeatures(tt.eapi)
			var got bool
			switch tt.method {
			case "SupportsSlotOperators":
				got = f.SupportsSlotOperators()
			case "SupportsSubSlots":
				got = f.SupportsSubSlots()
			case "SupportsBDEPEND":
				got = f.SupportsBDEPEND()
			case "SupportsIDEPEND":
				got = f.SupportsIDEPEND()
			case "SupportsRequiredUse":
				got = f.SupportsRequiredUse()
			case "SupportsDosymRelative":
				got = f.SupportsDosymRelative()
			case "SupportsEapply":
				got = f.SupportsEapply()
			case "SupportsSrcURIArrows":
				got = f.SupportsSrcURIArrows()
			case "SupportsSrcURISelectiveRestrictions":
				got = f.SupportsSrcURISelectiveRestrictions()
			default:
				t.Fatalf("unknown method: %s", tt.method)
			}
			if got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.method, got, tt.expected)
			}
		})
	}
}

// TestRequiresBashVersion tests the bash version checking method
func TestRequiresBashVersion(t *testing.T) {
	tests := []struct {
		eapi     string
		major    int
		minor    int
		expected bool
	}{
		// EAPI 0-5 requires Bash 3.2
		{"0", 3, 2, true},
		{"0", 3, 1, false},
		{"0", 4, 0, true},
		{"5", 3, 2, true},
		{"5", 3, 1, false},
		// EAPI 6-7 requires Bash 4.2
		{"6", 4, 2, true},
		{"6", 4, 1, false},
		{"6", 4, 3, true},
		{"6", 5, 0, true},
		{"7", 4, 2, true},
		{"7", 4, 1, false},
		// EAPI 8 requires Bash 5.0
		{"8", 5, 0, true},
		{"8", 4, 3, false},
		{"8", 5, 1, true},
		{"8", 6, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.eapi, func(t *testing.T) {
			f := MustGetEAPIFeatures(tt.eapi)
			got := f.RequiresBashVersion(tt.major, tt.minor)
			if got != tt.expected {
				t.Errorf("EAPI %s RequiresBashVersion(%d, %d) = %v, want %v",
					tt.eapi, tt.major, tt.minor, got, tt.expected)
			}
		})
	}
}

// TestEAPIFeaturesString tests the String method
func TestEAPIFeaturesString(t *testing.T) {
	for _, eapi := range SupportedEAPIs() {
		f := MustGetEAPIFeatures(eapi)
		if f.String() != eapi {
			t.Errorf("EAPI %s String() = %q, want %q", eapi, f.String(), eapi)
		}
	}
}

// TestEAPIFeaturesIsValid tests the IsValid method
func TestEAPIFeaturesIsValid(t *testing.T) {
	// Valid EAPI
	f := MustGetEAPIFeatures("8")
	if !f.IsValid() {
		t.Error("IsValid() should be true for valid EAPI")
	}

	// Zero value should be invalid
	var empty EAPIFeatures
	if empty.IsValid() {
		t.Error("IsValid() should be false for zero value")
	}
}

// TestEAPIProgressionBDEPEND verifies BDEPEND is only in EAPI 7+
func TestEAPIProgressionBDEPEND(t *testing.T) {
	for _, eapi := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		f := MustGetEAPIFeatures(eapi)
		if f.BDEPEND {
			t.Errorf("EAPI %s should not have BDEPEND", eapi)
		}
	}
	for _, eapi := range []string{"7", "8"} {
		f := MustGetEAPIFeatures(eapi)
		if !f.BDEPEND {
			t.Errorf("EAPI %s should have BDEPEND", eapi)
		}
	}
}

// TestEAPIProgressionIDEPEND verifies IDEPEND is only in EAPI 8+
func TestEAPIProgressionIDEPEND(t *testing.T) {
	for _, eapi := range []string{"0", "1", "2", "3", "4", "5", "6", "7"} {
		f := MustGetEAPIFeatures(eapi)
		if f.IDEPEND {
			t.Errorf("EAPI %s should not have IDEPEND", eapi)
		}
	}
	f := MustGetEAPIFeatures("8")
	if !f.IDEPEND {
		t.Error("EAPI 8 should have IDEPEND")
	}
}

// TestEAPIProgressionRdependDefault verifies RDEPEND default behavior
func TestEAPIProgressionRdependDefault(t *testing.T) {
	// EAPI 0-3: RDEPEND defaults to DEPEND
	for _, eapi := range []string{"0", "1", "2", "3"} {
		f := MustGetEAPIFeatures(eapi)
		if !f.RdependDefaultsToDepend {
			t.Errorf("EAPI %s should have RdependDefaultsToDepend=true", eapi)
		}
	}
	// EAPI 4+: No default
	for _, eapi := range []string{"4", "5", "6", "7", "8"} {
		f := MustGetEAPIFeatures(eapi)
		if f.RdependDefaultsToDepend {
			t.Errorf("EAPI %s should have RdependDefaultsToDepend=false", eapi)
		}
	}
}

// TestEAPIProgressionEmptyGroups verifies empty group matching behavior
func TestEAPIProgressionEmptyGroups(t *testing.T) {
	// EAPI 0-6: Empty groups may match
	for _, eapi := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		f := MustGetEAPIFeatures(eapi)
		if !f.EmptyGroupsMatch {
			t.Errorf("EAPI %s should have EmptyGroupsMatch=true", eapi)
		}
	}
	// EAPI 7+: Empty groups never match
	for _, eapi := range []string{"7", "8"} {
		f := MustGetEAPIFeatures(eapi)
		if f.EmptyGroupsMatch {
			t.Errorf("EAPI %s should have EmptyGroupsMatch=false", eapi)
		}
	}
}

// TestAllEAPIsHaveVersion ensures all EAPIs in the map have correct Version field
func TestAllEAPIsHaveVersion(t *testing.T) {
	for eapi := range supportedEAPIs {
		f := supportedEAPIs[eapi]
		if f.Version != eapi {
			t.Errorf("supportedEAPIs[%q].Version = %q, want %q", eapi, f.Version, eapi)
		}
	}
}

// TestAllEAPIsHaveBashVersion ensures all EAPIs specify a bash version
func TestAllEAPIsHaveBashVersion(t *testing.T) {
	for eapi := range supportedEAPIs {
		f := supportedEAPIs[eapi]
		if f.BashVersion == "" {
			t.Errorf("EAPI %s has empty BashVersion", eapi)
		}
		if f.BashVersionMajor == 0 {
			t.Errorf("EAPI %s has BashVersionMajor=0", eapi)
		}
	}
}
