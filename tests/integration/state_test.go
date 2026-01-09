package integration

import (
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/application"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestState_VarDBQueries tests PackageService queries against installed packages.
func TestState_VarDBQueries(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	db := state.NewPackageDatabase(t.TempDir())

	zlibInstalled := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Size:        102400,
	}
	if err := db.Add(zlibInstalled); err != nil {
		t.Fatalf("failed to add installed package: %v", err)
	}

	service := application.NewPackageService(mockRepo, db)

	tests := []struct {
		name            string
		packageName     string
		expectInstalled bool
		expectError     bool
	}{
		{"installed package zlib", "sys-libs/zlib", true, false},
		{"not installed package hello", "app-misc/hello", false, false},
		{"nonexistent package", "dev-fake/notexist", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info, err := service.GetPackageInfo(tc.packageName)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tc.packageName)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPackageInfo(%s) error: %v", tc.packageName, err)
			}
			if info.Installed != tc.expectInstalled {
				t.Errorf("Installed = %v, expected %v", info.Installed, tc.expectInstalled)
			}
		})
	}

	t.Run("nil_database", func(t *testing.T) {
		serviceNoDB := application.NewPackageService(mockRepo, nil)
		info, err := serviceNoDB.GetPackageInfo("sys-libs/zlib")
		if err != nil {
			t.Fatalf("GetPackageInfo failed: %v", err)
		}
		if info.Installed {
			t.Error("expected Installed=false with nil database")
		}
	})
}

// TestState_DatabaseConsistency tests PackageDatabase consistency.
func TestState_DatabaseConsistency(t *testing.T) {
	db := state.NewPackageDatabase(t.TempDir())

	testPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{Path: "/usr/lib/libz.so", Type: state.FileTypeSymlink},
			{Path: "/usr/lib/libz.so.1", Type: state.FileTypeSymlink},
			{Path: "/usr/lib/libz.so.1.2.13", Type: state.FileTypeRegular},
		},
	}

	t.Run("add_and_verify", func(t *testing.T) {
		if err := db.Add(testPkg); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if db.Count() != 1 {
			t.Errorf("Count() = %d, expected 1", db.Count())
		}
		if !db.Has("sys-libs/zlib-1.2.13") {
			t.Error("Has() returned false for installed package")
		}
		pkg, err := db.Get("sys-libs/zlib-1.2.13")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		if pkg.Package.Version != "1.2.13" {
			t.Errorf("Get().Version = %s, expected 1.2.13", pkg.Package.Version)
		}
	})

	t.Run("file_ownership", func(t *testing.T) {
		owner, err := db.WhoOwns("/usr/lib/libz.so.1.2.13")
		if err != nil {
			t.Fatalf("WhoOwns() error: %v", err)
		}
		if owner != "sys-libs/zlib-1.2.13" {
			t.Errorf("WhoOwns() = %s, expected sys-libs/zlib-1.2.13", owner)
		}
		if _, err := db.WhoOwns("/usr/lib/libfoo.so"); err == nil {
			t.Error("WhoOwns() should return error for non-owned file")
		}
	})

	t.Run("remove_and_verify", func(t *testing.T) {
		if err := db.Remove("sys-libs/zlib-1.2.13"); err != nil {
			t.Fatalf("Remove() error: %v", err)
		}
		if db.Has("sys-libs/zlib-1.2.13") {
			t.Error("Has() returned true after removal")
		}
		if db.Count() != 0 {
			t.Errorf("Count() = %d after removal, expected 0", db.Count())
		}
		if _, err := db.WhoOwns("/usr/lib/libz.so.1.2.13"); err == nil {
			t.Error("WhoOwns() should return error after package removal")
		}
	})
}
