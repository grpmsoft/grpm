package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestNewVarDBLoader(t *testing.T) {
	loader := NewVarDBLoader("/var/db/pkg")

	if loader == nil {
		t.Fatal("NewVarDBLoader() returned nil")
	}

	if loader.root != "/var/db/pkg" {
		t.Errorf("root = %q, want %q", loader.root, "/var/db/pkg")
	}
}

func TestParsePkgNameVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"hello-2.10", "hello", "2.10"},
		{"zlib-1.2.13", "zlib", "1.2.13"},
		{"gcc-12.3.0-r1", "gcc", "12.3.0-r1"},
		{"vim-core-9.0.1000", "vim-core", "9.0.1000"},
		{"python-exec-2", "python-exec", "2"},
		{"noversion", "noversion", ""},
		{"name-with-dashes-1.0", "name-with-dashes", "1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotName, gotVersion := parsePkgNameVersion(tt.input)
			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("version = %q, want %q", gotVersion, tt.wantVersion)
			}
		})
	}
}

func TestVarDBLoader_LoadInto(t *testing.T) {
	tmpDir := t.TempDir()
	vardbRoot := filepath.Join(tmpDir, "var", "db", "pkg")

	// Create a mock VarDB structure
	pkgDir := filepath.Join(vardbRoot, "sys-libs", "zlib-1.2.13")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create CONTENTS file
	contentsData := `obj /usr/lib/libz.so.1.2.13 abc123 0644 1234567890
sym /usr/lib/libz.so -> libz.so.1.2.13 1234567890
dir /usr/lib
`
	if err := os.WriteFile(filepath.Join(pkgDir, "CONTENTS"), []byte(contentsData), 0644); err != nil {
		t.Fatal(err)
	}

	// Create other metadata files
	if err := os.WriteFile(filepath.Join(pkgDir, "USE"), []byte("static-libs minizip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "CFLAGS"), []byte("-O2 -pipe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "EAPI"), []byte("8"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "BUILD_TIME"), []byte("1704067200"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "SIZE"), []byte("1234567"), 0644); err != nil {
		t.Fatal(err)
	}

	// Load into database
	db := NewPackageDatabase(tmpDir)
	loader := NewVarDBLoader(vardbRoot)

	err := loader.LoadInto(db)
	if err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	// Verify package was loaded
	if !db.Has("sys-libs/zlib-1.2.13") {
		t.Error("Package sys-libs/zlib-1.2.13 should be in database")
	}

	// Get and verify package details
	installedPkg, err := db.Get("sys-libs/zlib-1.2.13")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if installedPkg == nil {
		t.Fatal("Get() returned nil")
	}

	if len(installedPkg.Files) != 3 {
		t.Errorf("Files count = %d, want 3", len(installedPkg.Files))
	}

	if len(installedPkg.USE) != 2 {
		t.Errorf("USE flags count = %d, want 2", len(installedPkg.USE))
	}

	if installedPkg.CFLAGS != "-O2 -pipe" {
		t.Errorf("CFLAGS = %q, want %q", installedPkg.CFLAGS, "-O2 -pipe")
	}
}

func TestVarDBLoader_LoadInto_NonExistent(t *testing.T) {
	db := NewPackageDatabase(t.TempDir())
	loader := NewVarDBLoader("/nonexistent/path")

	err := loader.LoadInto(db)
	if err == nil {
		t.Error("LoadInto() should fail for non-existent path")
	}
}

func TestVarDBLoader_LoadInto_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	vardbRoot := filepath.Join(tmpDir, "var", "db", "pkg")
	if err := os.MkdirAll(vardbRoot, 0755); err != nil {
		t.Fatal(err)
	}

	db := NewPackageDatabase(tmpDir)
	loader := NewVarDBLoader(vardbRoot)

	err := loader.LoadInto(db)
	if err != nil {
		t.Fatalf("LoadInto() error = %v (should succeed for empty dir)", err)
	}

	if db.Count() != 0 {
		t.Errorf("Count() = %d, want 0", db.Count())
	}
}

func TestParseContentsLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantType FileType
		wantPath string
		wantErr  bool
	}{
		{
			name:     "regular_file",
			line:     "obj /usr/bin/hello abc123 0755 1234567890",
			wantType: FileTypeRegular,
			wantPath: "/usr/bin/hello",
		},
		{
			name:     "directory",
			line:     "dir /usr/share/doc",
			wantType: FileTypeDirectory,
			wantPath: "/usr/share/doc",
		},
		{
			name:     "symlink",
			line:     "sym /usr/lib/libtest.so -> libtest.so.1 1234567890",
			wantType: FileTypeSymlink,
			wantPath: "/usr/lib/libtest.so",
		},
		{
			name:    "invalid_short",
			line:    "obj",
			wantErr: true,
		},
		{
			name:    "invalid_type",
			line:    "unknown /path 123",
			wantErr: true,
		},
		{
			name:    "obj_missing_fields",
			line:    "obj /path",
			wantErr: true,
		},
		{
			name:    "sym_missing_fields",
			line:    "sym /path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := parseContentsLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseContentsLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if file.Type != tt.wantType {
					t.Errorf("Type = %v, want %v", file.Type, tt.wantType)
				}
				if file.Path != tt.wantPath {
					t.Errorf("Path = %q, want %q", file.Path, tt.wantPath)
				}
			}
		})
	}
}

func TestNewVarDBWriter(t *testing.T) {
	writer := NewVarDBWriter("/var/db/pkg")

	if writer == nil {
		t.Fatal("NewVarDBWriter() returned nil")
	}

	if writer.root != "/var/db/pkg" {
		t.Errorf("root = %q, want %q", writer.root, "/var/db/pkg")
	}
}

func TestVarDBWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()
	vardbRoot := filepath.Join(tmpDir, "var", "db", "pkg")

	writer := NewVarDBWriter(vardbRoot)

	installedPkg := &InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
		},
		InstallTime: time.Unix(1704067200, 0),
		Files: []InstalledFile{
			{Path: "/usr/lib/libz.so.1.2.13", Type: FileTypeRegular, Hash: "abc123", Mode: 0644, MTime: 1234567890},
			{Path: "/usr/lib/libz.so", Type: FileTypeSymlink, Target: "libz.so.1.2.13", MTime: 1234567890},
			{Path: "/usr/lib", Type: FileTypeDirectory},
		},
		USE:    []string{"static-libs", "minizip"},
		CFLAGS: "-O2 -pipe",
		Size:   1234567,
		BuildInfo: BuildInfo{
			EAPI: "8",
		},
	}

	err := writer.Write(installedPkg)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Verify directory was created
	pkgDir := filepath.Join(vardbRoot, "sys-libs", "zlib-1.2.13")
	if _, err := os.Stat(pkgDir); err != nil {
		t.Errorf("Package directory not created: %v", err)
	}

	// Verify CONTENTS file
	contentsPath := filepath.Join(pkgDir, "CONTENTS")
	if _, err := os.Stat(contentsPath); err != nil {
		t.Errorf("CONTENTS file not created: %v", err)
	}

	// Verify USE file
	usePath := filepath.Join(pkgDir, "USE")
	useContent, err := os.ReadFile(usePath)
	if err != nil {
		t.Errorf("Failed to read USE file: %v", err)
	}
	if string(useContent) != "static-libs minizip" {
		t.Errorf("USE content = %q, want %q", useContent, "static-libs minizip")
	}

	// Verify EAPI file
	eapiPath := filepath.Join(pkgDir, "EAPI")
	eapiContent, err := os.ReadFile(eapiPath)
	if err != nil {
		t.Errorf("Failed to read EAPI file: %v", err)
	}
	if string(eapiContent) != "8" {
		t.Errorf("EAPI content = %q, want %q", eapiContent, "8")
	}
}

func TestVarDBWriter_Write_NilPackage(t *testing.T) {
	tmpDir := t.TempDir()
	writer := NewVarDBWriter(tmpDir)

	err := writer.Write(nil)
	if err == nil {
		t.Error("Write() should fail for nil package")
	}

	err = writer.Write(&InstalledPackage{Package: nil})
	if err == nil {
		t.Error("Write() should fail for nil Package field")
	}
}

func TestVarDBWriter_Write_InvalidPackageName(t *testing.T) {
	tmpDir := t.TempDir()
	writer := NewVarDBWriter(tmpDir)

	installedPkg := &InstalledPackage{
		Package: &pkg.Package{
			Name:    "invalid", // No category
			Version: "1.0",
		},
	}

	err := writer.Write(installedPkg)
	if err == nil {
		t.Error("Write() should fail for invalid package name format")
	}
}

func TestFormatContentsLine(t *testing.T) {
	tests := []struct {
		name     string
		file     InstalledFile
		expected string
	}{
		{
			name: "regular_file",
			file: InstalledFile{
				Path:  "/usr/bin/hello",
				Type:  FileTypeRegular,
				Hash:  "abc123",
				Mode:  0755,
				MTime: 1234567890,
			},
			expected: "obj /usr/bin/hello abc123 0755 1234567890",
		},
		{
			name: "directory",
			file: InstalledFile{
				Path: "/usr/share/doc",
				Type: FileTypeDirectory,
			},
			expected: "dir /usr/share/doc",
		},
		{
			name: "symlink",
			file: InstalledFile{
				Path:   "/usr/lib/libtest.so",
				Type:   FileTypeSymlink,
				Target: "libtest.so.1",
				MTime:  1234567890,
			},
			expected: "sym /usr/lib/libtest.so -> libtest.so.1 1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatContentsLine(tt.file)
			if result != tt.expected {
				t.Errorf("formatContentsLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestVarDBRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	vardbRoot := filepath.Join(tmpDir, "var", "db", "pkg")

	// Create original package
	originalPkg := &InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
		},
		InstallTime: time.Unix(1704067200, 0),
		Files: []InstalledFile{
			{Path: "/usr/bin/hello", Type: FileTypeRegular, Hash: "abc", Mode: 0755, MTime: 1234567890},
			{Path: "/usr/share/doc/hello", Type: FileTypeDirectory},
			{Path: "/usr/share/doc/hello/README", Type: FileTypeRegular, Hash: "def", Mode: 0644, MTime: 1234567890},
		},
		USE:    []string{"nls"},
		CFLAGS: "-O2",
		Size:   12345,
		BuildInfo: BuildInfo{
			EAPI: "8",
		},
	}

	// Write to VarDB
	writer := NewVarDBWriter(vardbRoot)
	if err := writer.Write(originalPkg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Load from VarDB
	db := NewPackageDatabase(tmpDir)
	loader := NewVarDBLoader(vardbRoot)
	if err := loader.LoadInto(db); err != nil {
		t.Fatalf("LoadInto() error = %v", err)
	}

	// Verify package was loaded correctly
	loadedPkg, err := db.Get("app-misc/hello-2.10")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loadedPkg == nil {
		t.Fatal("Package not found after round-trip")
	}

	// Verify basic fields
	if loadedPkg.Package.Name != originalPkg.Package.Name {
		t.Errorf("Name = %q, want %q", loadedPkg.Package.Name, originalPkg.Package.Name)
	}

	if len(loadedPkg.Files) != len(originalPkg.Files) {
		t.Errorf("Files count = %d, want %d", len(loadedPkg.Files), len(originalPkg.Files))
	}

	if len(loadedPkg.USE) != len(originalPkg.USE) {
		t.Errorf("USE count = %d, want %d", len(loadedPkg.USE), len(originalPkg.USE))
	}
}

func BenchmarkVarDBLoader_LoadInto(b *testing.B) {
	tmpDir := b.TempDir()
	vardbRoot := filepath.Join(tmpDir, "var", "db", "pkg")

	// Create 100 mock packages
	for i := 0; i < 100; i++ {
		pkgDir := filepath.Join(vardbRoot, "category", "pkg"+string(rune('0'+i%10))+"-1.0")
		_ = os.MkdirAll(pkgDir, 0755)

		contentsData := "obj /usr/bin/test abc123 0755 1234567890\ndir /usr/bin\n"
		_ = os.WriteFile(filepath.Join(pkgDir, "CONTENTS"), []byte(contentsData), 0644)
		_ = os.WriteFile(filepath.Join(pkgDir, "USE"), []byte("test"), 0644)
		_ = os.WriteFile(filepath.Join(pkgDir, "EAPI"), []byte("8"), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db := NewPackageDatabase(tmpDir)
		loader := NewVarDBLoader(vardbRoot)
		_ = loader.LoadInto(db)
	}
}

func BenchmarkVarDBWriter_Write(b *testing.B) {
	tmpDir := b.TempDir()

	files := make([]InstalledFile, 50)
	for i := 0; i < 50; i++ {
		files[i] = InstalledFile{
			Path:  "/usr/share/doc/file" + string(rune('0'+i%10)),
			Type:  FileTypeRegular,
			Hash:  "abc123",
			Mode:  0644,
			MTime: 1234567890,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer := NewVarDBWriter(filepath.Join(tmpDir, string(rune('0'+i%10))))
		installedPkg := &InstalledPackage{
			Package:   &pkg.Package{Name: "test/pkg", Version: "1.0"},
			Files:     files,
			USE:       []string{"test"},
			BuildInfo: BuildInfo{EAPI: "8"},
		}
		_ = writer.Write(installedPkg)
	}
}
