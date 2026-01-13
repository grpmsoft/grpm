package integration

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/binpkg"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestBinpkg_GPKGMetadataParsing tests GPKG metadata parsing from tar archives.
func TestBinpkg_GPKGMetadataParsing(t *testing.T) {
	splitTests := []struct {
		input   string
		name    string
		version string
	}{
		{"zlib-1.2.13", "zlib", "1.2.13"},
		{"hello-2.10-r1", "hello", "2.10-r1"},
		{"gtk+-3.24.38", "gtk+", "3.24.38"},
		{"python-exec-2.4.10", "python-exec", "2.4.10"},
		{"gcc-12.3.1_p20230526", "gcc", "12.3.1_p20230526"},
	}

	for _, tc := range splitTests {
		t.Run("splitPkgNameVersion_"+tc.input, func(t *testing.T) {
			tmpDir := t.TempDir()
			gpkgPath := filepath.Join(tmpDir, tc.input+".gpkg.tar")
			if err := createTestGPKG(gpkgPath, tc.input, "", tc.version, "0", nil); err != nil {
				t.Fatalf("failed to create test GPKG: %v", err)
			}
			binPkg, err := binpkg.LoadGPKG(gpkgPath)
			if err != nil {
				t.Fatalf("failed to load GPKG: %v", err)
			}
			if binPkg.Package == nil {
				t.Fatal("loaded package is nil")
			}
			if tc.version != "" && binPkg.Package.Version != tc.version {
				t.Errorf("version mismatch: got %q, expected %q", binPkg.Package.Version, tc.version)
			}
		})
	}

	t.Run("full_metadata_parsing", func(t *testing.T) {
		tmpDir := t.TempDir()
		gpkgPath := filepath.Join(tmpDir, "sys-libs--zlib-1.2.13.gpkg.tar")
		metadata := map[string]string{
			"CATEGORY":   "sys-libs",
			"PF":         "zlib-1.2.13",
			"SLOT":       "0/1.2",
			"USE":        "minizip static-libs",
			"BUILD_TIME": "1700000000",
			"EAPI":       "8",
			"CFLAGS":     "-O2 -pipe",
		}
		if err := createTestGPKG(gpkgPath, "zlib-1.2.13", "sys-libs", "1.2.13", "0/1.2", metadata); err != nil {
			t.Fatalf("failed to create test GPKG: %v", err)
		}
		binPkg, err := binpkg.LoadGPKG(gpkgPath)
		if err != nil {
			t.Fatalf("failed to load GPKG: %v", err)
		}
		if binPkg.Package.Name != "sys-libs/zlib" {
			t.Errorf("package name: got %q, expected %q", binPkg.Package.Name, "sys-libs/zlib")
		}
		if binPkg.Package.Version != "1.2.13" {
			t.Errorf("package version: got %q, expected %q", binPkg.Package.Version, "1.2.13")
		}
		if binPkg.BuildInfo == nil {
			t.Fatal("BuildInfo is nil")
		}
		if binPkg.BuildInfo.EAPI != "8" {
			t.Errorf("EAPI: got %q, expected %q", binPkg.BuildInfo.EAPI, "8")
		}
	})
}

// TestBinpkg_AtomMatching tests binhost package searching with Portage atom specs.
func TestBinpkg_AtomMatching(t *testing.T) {
	tmpDir := t.TempDir()
	binhost, err := binpkg.NewBinhost(tmpDir)
	if err != nil {
		t.Fatalf("failed to create binhost: %v", err)
	}

	binhost.Packages = []*binpkg.BinaryPackage{
		{Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.2.11"}, Format: binpkg.FormatGPKG},
		{Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.2.13"}, Format: binpkg.FormatGPKG},
		{Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.3.0"}, Format: binpkg.FormatGPKG},
		{Package: &pkg.Package{Name: "app-misc/hello", Version: "2.10"}, Format: binpkg.FormatGPKG},
		{Package: &pkg.Package{Name: "dev-libs/openssl", Version: "1.1.1k"}, Format: binpkg.FormatGPKG},
	}

	tests := []struct {
		name          string
		atom          string
		expectedCount int
	}{
		{"all zlib versions", "sys-libs/zlib", 3},
		{"single hello version", "app-misc/hello", 1},
		{"exact version match", "=sys-libs/zlib-1.2.13", 1},
		{"exact version no match", "=sys-libs/zlib-1.2.12", 0},
		{"zlib >= 1.2.13", ">=sys-libs/zlib-1.2.13", 2},
		{"zlib >= 2.0", ">=sys-libs/zlib-2.0", 0},
		{"zlib > 1.2.11", ">sys-libs/zlib-1.2.11", 2},
		{"zlib < 1.3.0", "<sys-libs/zlib-1.3.0", 2},
		{"nonexistent package", "dev-fake/notexist", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matches := binhost.Find(tc.atom)
			if len(matches) != tc.expectedCount {
				t.Errorf("Find(%q): got %d packages, expected %d", tc.atom, len(matches), tc.expectedCount)
			}
		})
	}
}

// createTestGPKG creates a minimal GPKG tar file for testing.
func createTestGPKG(path, pf, category, version, slot string, extraMeta map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	tw := tar.NewWriter(f)
	defer func() { _ = tw.Close() }()

	var metaBuf bytes.Buffer
	metaTW := tar.NewWriter(&metaBuf)

	metaFiles := map[string]string{"PF": pf, "SLOT": slot}
	if category != "" {
		metaFiles["CATEGORY"] = category
	}
	for k, v := range extraMeta {
		metaFiles[k] = v
	}

	for name, content := range metaFiles {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := metaTW.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := metaTW.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := metaTW.Close(); err != nil {
		return err
	}

	metaData := metaBuf.Bytes()
	hdr := &tar.Header{Name: "metadata.tar", Mode: 0644, Size: int64(len(metaData))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(metaData); err != nil {
		return err
	}
	return nil
}
