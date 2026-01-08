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
