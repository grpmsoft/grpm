package ebuild

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Environment GetVar/SetVar Tests
// ============================================================================

func TestEnvironment_GetVar_BuiltIn(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	tests := []struct {
		name string
		want string
	}{
		{"PN", "zlib"},
		{"PV", "1.2.13"},
		{"CATEGORY", "sys-libs"},
		{"EAPI", "8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := env.GetVar(tt.name)
			if got != tt.want {
				t.Errorf("GetVar(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestEnvironment_GetVar_Extra(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Set extra variable
	env.SetVar("DOCS", "README.md CHANGELOG")
	env.SetVar("HTML_DOCS", "doc/html")

	// Check retrieval
	if got := env.GetVar("DOCS"); got != "README.md CHANGELOG" {
		t.Errorf("GetVar(DOCS) = %q, want %q", got, "README.md CHANGELOG")
	}
	if got := env.GetVar("HTML_DOCS"); got != "doc/html" {
		t.Errorf("GetVar(HTML_DOCS) = %q, want %q", got, "doc/html")
	}
}

func TestEnvironment_GetVar_NotFound(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	if got := env.GetVar("NONEXISTENT"); got != "" {
		t.Errorf("GetVar(NONEXISTENT) = %q, want empty string", got)
	}
}
