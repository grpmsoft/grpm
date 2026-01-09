package ebuild

import (
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestNewEnvironment(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl":   true,
			"debug": false,
		},
	}

	env, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment() failed: %v", err)
	}

	// Check basic variables
	if env.PN != "zlib" {
		t.Errorf("PN = %s, expected zlib", env.PN)
	}

	if env.PV != "1.2.13" {
		t.Errorf("PV = %s, expected 1.2.13", env.PV)
	}

	if env.CATEGORY != "sys-libs" {
		t.Errorf("CATEGORY = %s, expected sys-libs", env.CATEGORY)
	}

	if env.P != "zlib-1.2.13" {
		t.Errorf("P = %s, expected zlib-1.2.13", env.P)
	}

	if env.EAPI != "8" {
		t.Errorf("EAPI = %s, expected 8", env.EAPI)
	}
}

func TestEnvironmentWithRevision(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13-r1",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment() failed: %v", err)
	}

	if env.PV != "1.2.13" {
		t.Errorf("PV = %s, expected 1.2.13", env.PV)
	}

	if env.PR != "r1" {
		t.Errorf("PR = %s, expected r1", env.PR)
	}

	if env.PF != "zlib-1.2.13-r1" {
		t.Errorf("PF = %s, expected zlib-1.2.13-r1", env.PF)
	}
}

func TestEnvironmentToMap(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment() failed: %v", err)
	}

	envMap := env.ToMap()

	if envMap["PN"] != "zlib" {
		t.Errorf("envMap[PN] = %s, expected zlib", envMap["PN"])
	}

	if envMap["CATEGORY"] != "sys-libs" {
		t.Errorf("envMap[CATEGORY] = %s, expected sys-libs", envMap["CATEGORY"])
	}

	if envMap["EAPI"] != "8" {
		t.Errorf("envMap[EAPI] = %s, expected 8", envMap["EAPI"])
	}
}

func TestEnvironmentToSlice(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment() failed: %v", err)
	}

	envSlice := env.ToSlice()

	// Check that it contains expected variables
	hasPN := false
	hasCategory := false

	for _, entry := range envSlice {
		if strings.HasPrefix(entry, "PN=") {
			hasPN = true
		}
		if strings.HasPrefix(entry, "CATEGORY=") {
			hasCategory = true
		}
	}

	if !hasPN {
		t.Error("ToSlice() missing PN variable")
	}

	if !hasCategory {
		t.Error("ToSlice() missing CATEGORY variable")
	}
}

func TestEnvironmentInvalidPackageName(t *testing.T) {
	p := &pkg.Package{
		Name:    "invalid",
		Version: "1.0",
		Slot:    pkg.Slot{Name: "0"},
	}

	_, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err == nil {
		t.Error("NewEnvironment() should fail with invalid package name")
	}
}

func TestEnvironmentNilPackage(t *testing.T) {
	_, err := NewEnvironment(nil, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err == nil {
		t.Error("NewEnvironment() should fail with nil package")
	}
}

func BenchmarkNewEnvironment(b *testing.B) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	}
}

func BenchmarkEnvironmentToSlice(b *testing.B) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, _ := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = env.ToSlice()
	}
}

func TestComputePVR(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		revision string
		expected string
	}{
		{
			name:     "no revision (r0)",
			version:  "1.2.13",
			revision: "r0",
			expected: "1.2.13",
		},
		{
			name:     "empty revision",
			version:  "1.2.13",
			revision: "",
			expected: "1.2.13",
		},
		{
			name:     "with revision r1",
			version:  "1.2.13",
			revision: "r1",
			expected: "1.2.13-r1",
		},
		{
			name:     "with revision r5",
			version:  "2.0",
			revision: "r5",
			expected: "2.0-r5",
		},
		{
			name:     "complex version",
			version:  "1.2.3_alpha4",
			revision: "r2",
			expected: "1.2.3_alpha4-r2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computePVR(tt.version, tt.revision)
			if result != tt.expected {
				t.Errorf("computePVR(%q, %q) = %q, expected %q",
					tt.version, tt.revision, result, tt.expected)
			}
		})
	}
}

func TestEnvironmentPVR(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectedPVR string
		expectedPF  string
	}{
		{
			name:        "no revision",
			version:     "1.2.13",
			expectedPVR: "1.2.13",
			expectedPF:  "zlib-1.2.13",
		},
		{
			name:        "with revision",
			version:     "1.2.13-r1",
			expectedPVR: "1.2.13-r1",
			expectedPF:  "zlib-1.2.13-r1",
		},
		{
			name:        "high revision",
			version:     "2.0-r10",
			expectedPVR: "2.0-r10",
			expectedPF:  "zlib-2.0-r10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &pkg.Package{
				Name:    "sys-libs/zlib",
				Version: tt.version,
				Slot:    pkg.Slot{Name: "0"},
			}

			env, err := NewEnvironment(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
			if err != nil {
				t.Fatalf("NewEnvironment() failed: %v", err)
			}

			if env.PVR != tt.expectedPVR {
				t.Errorf("PVR = %q, expected %q", env.PVR, tt.expectedPVR)
			}
			if env.PF != tt.expectedPF {
				t.Errorf("PF = %q, expected %q", env.PF, tt.expectedPF)
			}
		})
	}
}

func TestEnvironmentWithEAPI(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	tests := []struct {
		name                string
		eapi                string
		expectTrailingSlash bool
		expectOffsetPrefix  bool
		expectSYSROOT       bool
		expectBROOT         bool
	}{
		{
			name:                "EAPI 0",
			eapi:                "0",
			expectTrailingSlash: true,
			expectOffsetPrefix:  false,
			expectSYSROOT:       false,
			expectBROOT:         false,
		},
		{
			name:                "EAPI 3 (offset-prefix)",
			eapi:                "3",
			expectTrailingSlash: true,
			expectOffsetPrefix:  true,
			expectSYSROOT:       false,
			expectBROOT:         false,
		},
		{
			name:                "EAPI 6",
			eapi:                "6",
			expectTrailingSlash: true,
			expectOffsetPrefix:  true,
			expectSYSROOT:       false,
			expectBROOT:         false,
		},
		{
			name:                "EAPI 7 (no trailing slash, cross-compilation)",
			eapi:                "7",
			expectTrailingSlash: false,
			expectOffsetPrefix:  true,
			expectSYSROOT:       true,
			expectBROOT:         true,
		},
		{
			name:                "EAPI 8",
			eapi:                "8",
			expectTrailingSlash: false,
			expectOffsetPrefix:  true,
			expectSYSROOT:       true,
			expectBROOT:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnvironmentWithEAPI(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles", tt.eapi)
			if err != nil {
				t.Fatalf("NewEnvironmentWithEAPI() failed: %v", err)
			}

			if env.EAPI != tt.eapi {
				t.Errorf("EAPI = %q, expected %q", env.EAPI, tt.eapi)
			}

			// Check trailing slash behavior
			hasTrailingSlashD := strings.HasSuffix(env.D, "/")
			hasTrailingSlashROOT := strings.HasSuffix(env.ROOT, "/") || env.ROOT == ""

			if tt.expectTrailingSlash {
				if !hasTrailingSlashD {
					t.Errorf("D = %q, expected trailing slash for EAPI %s", env.D, tt.eapi)
				}
				if !hasTrailingSlashROOT && env.ROOT != "/" {
					t.Errorf("ROOT = %q, expected trailing slash for EAPI %s", env.ROOT, tt.eapi)
				}
			} else {
				if hasTrailingSlashD {
					t.Errorf("D = %q, expected no trailing slash for EAPI %s", env.D, tt.eapi)
				}
				// For EAPI 7+, ROOT should be empty string (no trailing slash)
				if env.ROOT == "/" {
					t.Errorf("ROOT = %q, expected empty string for EAPI %s", env.ROOT, tt.eapi)
				}
			}

			// Check EAPI features
			if env.EAPIFeatures.SupportsOffsetPrefix() != tt.expectOffsetPrefix {
				t.Errorf("SupportsOffsetPrefix() = %v, expected %v for EAPI %s",
					env.EAPIFeatures.SupportsOffsetPrefix(), tt.expectOffsetPrefix, tt.eapi)
			}
			if env.EAPIFeatures.SupportsSYSROOT() != tt.expectSYSROOT {
				t.Errorf("SupportsSYSROOT() = %v, expected %v for EAPI %s",
					env.EAPIFeatures.SupportsSYSROOT(), tt.expectSYSROOT, tt.eapi)
			}
			if env.EAPIFeatures.SupportsBROOT() != tt.expectBROOT {
				t.Errorf("SupportsBROOT() = %v, expected %v for EAPI %s",
					env.EAPIFeatures.SupportsBROOT(), tt.expectBROOT, tt.eapi)
			}
		})
	}
}

func TestEnvironmentToMapEAPIVariables(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	tests := []struct {
		name           string
		eapi           string
		expectPORTDIR  bool
		expectEPREFIX  bool
		expectEROOT    bool
		expectED       bool
		expectSYSROOT  bool
		expectESYSROOT bool
		expectBROOT    bool
	}{
		{
			name:          "EAPI 0 - no offset-prefix vars",
			eapi:          "0",
			expectPORTDIR: true,
			expectEPREFIX: false,
			expectEROOT:   false,
			expectED:      false,
			expectSYSROOT: false,
			expectBROOT:   false,
		},
		{
			name:          "EAPI 3 - offset-prefix vars",
			eapi:          "3",
			expectPORTDIR: true,
			expectEPREFIX: true,
			expectEROOT:   true,
			expectED:      true,
			expectSYSROOT: false,
			expectBROOT:   false,
		},
		{
			name:           "EAPI 7 - all vars",
			eapi:           "7",
			expectPORTDIR:  false, // Removed in EAPI 7
			expectEPREFIX:  true,
			expectEROOT:    true,
			expectED:       true,
			expectSYSROOT:  true,
			expectESYSROOT: true,
			expectBROOT:    true,
		},
		{
			name:           "EAPI 8 - all vars",
			eapi:           "8",
			expectPORTDIR:  false, // Removed in EAPI 7
			expectEPREFIX:  true,
			expectEROOT:    true,
			expectED:       true,
			expectSYSROOT:  true,
			expectESYSROOT: true,
			expectBROOT:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnvironmentWithEAPI(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles", tt.eapi)
			if err != nil {
				t.Fatalf("NewEnvironmentWithEAPI() failed: %v", err)
			}

			envMap := env.ToMap()

			checkVar := func(name string, expected bool) {
				_, exists := envMap[name]
				if exists != expected {
					if expected {
						t.Errorf("ToMap() missing %s for EAPI %s", name, tt.eapi)
					} else {
						t.Errorf("ToMap() should not include %s for EAPI %s", name, tt.eapi)
					}
				}
			}

			checkVar("PORTDIR", tt.expectPORTDIR)
			checkVar("EPREFIX", tt.expectEPREFIX)
			checkVar("EROOT", tt.expectEROOT)
			checkVar("ED", tt.expectED)
			checkVar("SYSROOT", tt.expectSYSROOT)
			checkVar("ESYSROOT", tt.expectESYSROOT)
			checkVar("BROOT", tt.expectBROOT)

			// PVR should always be present
			if _, exists := envMap["PVR"]; !exists {
				t.Errorf("ToMap() missing PVR for EAPI %s", tt.eapi)
			}

			// ROOT should always be present
			if _, exists := envMap["ROOT"]; !exists {
				t.Errorf("ToMap() missing ROOT for EAPI %s", tt.eapi)
			}
		})
	}
}

func TestTrailingSlashHelpers(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		withSlash    string
		withoutSlash string
	}{
		{
			name:         "empty string",
			input:        "",
			withSlash:    "/",
			withoutSlash: "",
		},
		{
			name:         "root",
			input:        "/",
			withSlash:    "/",
			withoutSlash: "",
		},
		{
			name:         "path without slash",
			input:        "/var/tmp/portage",
			withSlash:    "/var/tmp/portage/",
			withoutSlash: "/var/tmp/portage",
		},
		{
			name:         "path with slash",
			input:        "/var/tmp/portage/",
			withSlash:    "/var/tmp/portage/",
			withoutSlash: "/var/tmp/portage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withSlash := ensureTrailingSlash(tt.input)
			if withSlash != tt.withSlash {
				t.Errorf("ensureTrailingSlash(%q) = %q, expected %q", tt.input, withSlash, tt.withSlash)
			}

			withoutSlash := removeTrailingSlash(tt.input)
			if withoutSlash != tt.withoutSlash {
				t.Errorf("removeTrailingSlash(%q) = %q, expected %q", tt.input, withoutSlash, tt.withoutSlash)
			}
		})
	}
}

func TestEnvironmentCrossCompilationVars(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	// Test EAPI 8 (has SYSROOT, ESYSROOT, BROOT)
	env, err := NewEnvironmentWithEAPI(p, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles", "8")
	if err != nil {
		t.Fatalf("NewEnvironmentWithEAPI() failed: %v", err)
	}

	// For native builds, SYSROOT = ROOT
	if env.SYSROOT != env.ROOT {
		t.Errorf("SYSROOT = %q, expected %q (same as ROOT for native builds)", env.SYSROOT, env.ROOT)
	}

	// For native builds, ESYSROOT = EROOT
	if env.ESYSROOT != env.EROOT {
		t.Errorf("ESYSROOT = %q, expected %q (same as EROOT for native builds)", env.ESYSROOT, env.EROOT)
	}

	// BROOT should be empty for EAPI 7+ (root represented as empty string)
	if env.BROOT != "" {
		t.Errorf("BROOT = %q, expected empty string for EAPI 8", env.BROOT)
	}
}
