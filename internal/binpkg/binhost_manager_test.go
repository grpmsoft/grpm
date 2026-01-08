package binpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestBinhostManager_Add tests adding packages to binhost
func TestBinhostManager_Add(t *testing.T) {
	// Create temporary binhost directory
	binhostRoot := t.TempDir()

	// Create test package file
	pkgDir := t.TempDir()
	pkgPath := filepath.Join(pkgDir, "test-1.0.0.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test package"), 0644); err != nil {
		t.Fatalf("failed to create test package: %v", err)
	}

	// Create signature file
	sigPath := pkgPath + ".sig"
	if err := os.WriteFile(sigPath, []byte("test signature"), 0644); err != nil {
		t.Fatalf("failed to create signature: %v", err)
	}

	// Create test package
	testPkg := &BinaryPackage{
		Package:   pkg.NewPackage("app-test/testpkg", "1.0.0", "0"),
		Format:    FormatGPKG,
		Path:      pkgPath,
		Size:      12,
		Signature: &Signature{Type: SignatureGPG},
	}

	// Create manager
	manager := NewBinhostManager(binhostRoot)
	manager.Verbose = false

	// Execute
	err := manager.Add(testPkg)

	// Verify
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	// Check package file copied to binhost
	expectedPath := filepath.Join(binhostRoot, "app-test", "testpkg-1.0.0.gpkg.tar")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("package file not copied to binhost: %s", expectedPath)
	}

	// Check signature file copied
	expectedSigPath := expectedPath + ".sig"
	if _, err := os.Stat(expectedSigPath); os.IsNotExist(err) {
		t.Errorf("signature file not copied to binhost: %s", expectedSigPath)
	}

	// Check package added to list
	if len(manager.Packages) != 1 {
		t.Errorf("expected 1 package in list, got %d", len(manager.Packages))
	}

	t.Logf("Package added successfully to binhost at %s", expectedPath)
}

// TestBinhostManager_Remove tests removing packages from binhost
func TestBinhostManager_Remove(t *testing.T) {
	t.Skip("Integration test - requires real binary packages (run in WSL2 Gentoo)")

	// Create temporary binhost with test package
	binhostRoot := t.TempDir()

	// Add package directly to filesystem
	pkgDir := filepath.Join(binhostRoot, "sys-apps")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package directory: %v", err)
	}

	pkgPath := filepath.Join(pkgDir, "testapp-1.0.0.tbz2")
	if err := os.WriteFile(pkgPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test package: %v", err)
	}

	// Create manager and load
	manager := NewBinhostManager(binhostRoot)
	if err := manager.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	// Execute
	err := manager.Remove("sys-apps", "testapp", "1.0.0")

	// Verify
	if err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Check file deleted
	if _, err := os.Stat(pkgPath); !os.IsNotExist(err) {
		t.Errorf("package file still exists after removal: %s", pkgPath)
	}

	// Check removed from list
	if len(manager.Packages) != 0 {
		t.Errorf("expected 0 packages in list, got %d", len(manager.Packages))
	}

	t.Logf("Package removed successfully from binhost")
}

// TestBinhostManager_List tests listing packages
func TestBinhostManager_List(t *testing.T) {
	// Create temporary binhost
	binhostRoot := t.TempDir()

	// Create test packages
	packages := []struct {
		name    string
		version string
		format  BinaryFormat
	}{
		{"app-misc/hello", "1.0.0", FormatGPKG},
		{"sys-apps/test", "2.1.0", FormatTBZ2},
		{"dev-lang/go", "1.22.0", FormatGPKG},
	}

	manager := NewBinhostManager(binhostRoot)

	for _, p := range packages {
		// Create package file
		pkgDir := t.TempDir()
		ext := ".gpkg.tar"
		if p.format == FormatTBZ2 {
			ext = ".tbz2"
		}

		pkgName := strings.SplitN(p.name, "/", 2)[1]
		pkgPath := filepath.Join(pkgDir, pkgName+"-"+p.version+ext)
		if err := os.WriteFile(pkgPath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test package: %v", err)
		}

		testPkg := &BinaryPackage{
			Package: pkg.NewPackage(p.name, p.version, "0"),
			Format:  p.format,
			Path:    pkgPath,
			Size:    4,
		}

		if err := manager.Add(testPkg); err != nil {
			t.Fatalf("failed to add package: %v", err)
		}
	}

	// Execute
	pkgList := manager.List()

	// Verify
	if len(pkgList) != len(packages) {
		t.Errorf("expected %d packages, got %d", len(packages), len(pkgList))
	}

	// Verify package details
	for i, pkg := range pkgList {
		if pkg.Package.Name != packages[i].name {
			t.Errorf("package %d: expected name %s, got %s", i, packages[i].name, pkg.Package.Name)
		}
		if pkg.Package.Version != packages[i].version {
			t.Errorf("package %d: expected version %s, got %s", i, packages[i].version, pkg.Package.Version)
		}
	}

	t.Logf("Listed %d packages successfully", len(pkgList))
}

// TestBinhostManager_Scan tests scanning binhost directory
func TestBinhostManager_Scan(t *testing.T) {
	t.Skip("Integration test - requires real binary packages (run in WSL2 Gentoo)")

	// Create temporary binhost with packages
	binhostRoot := t.TempDir()

	// Create package files
	packages := []string{
		"app-misc/hello-1.0.0.gpkg.tar",
		"sys-apps/test-2.1.0.tbz2",
		"dev-lang/python-3.11.0.gpkg.tar",
	}

	for _, pkg := range packages {
		parts := strings.Split(pkg, "/")
		category := parts[0]
		filename := parts[1]

		// Create category directory
		categoryDir := filepath.Join(binhostRoot, category)
		if err := os.MkdirAll(categoryDir, 0755); err != nil {
			t.Fatalf("failed to create category directory: %v", err)
		}

		// Create package file
		pkgPath := filepath.Join(categoryDir, filename)
		if err := os.WriteFile(pkgPath, []byte("test package"), 0644); err != nil {
			t.Fatalf("failed to create package file: %v", err)
		}
	}

	// Create manager
	manager := NewBinhostManager(binhostRoot)

	// Execute
	err := manager.Scan()

	// Verify
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	if len(manager.Packages) != len(packages) {
		t.Errorf("expected %d packages, got %d", len(packages), len(manager.Packages))
	}

	// Verify packages loaded correctly
	for _, pkg := range manager.Packages {
		if pkg.Package.Name == "" {
			t.Errorf("package has empty name")
		}
		if pkg.Package.Version == "" {
			t.Errorf("package %s has empty version", pkg.Package.Name)
		}
	}

	t.Logf("Scanned binhost and found %d packages", len(manager.Packages))
}

// TestBinhostManager_GenerateIndex tests Packages index generation
func TestBinhostManager_GenerateIndex(t *testing.T) {
	// Create temporary binhost
	binhostRoot := t.TempDir()

	// Add test packages
	manager := NewBinhostManager(binhostRoot)

	buildTime := time.Date(2025, 10, 9, 12, 0, 0, 0, time.UTC)
	testPkg := &BinaryPackage{
		Package:  pkg.NewPackage("app-test/sample", "1.2.3", "0"),
		Format:   FormatGPKG,
		Path:     filepath.Join(binhostRoot, "app-test", "sample-1.2.3.gpkg.tar"),
		Size:     1024,
		Checksum: "abc123def456",
		BuildInfo: &BuildMetadata{
			BuildDate:  buildTime,
			Repository: "gentoo",
			USE:        []string{"ssl", "ipv6"},
		},
	}

	// Set USE flags in package
	testPkg.Package.UseFlags = map[string]bool{
		"ssl":  true,
		"ipv6": true,
	}

	// Create package file
	if err := os.MkdirAll(filepath.Dir(testPkg.Path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(testPkg.Path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create package file: %v", err)
	}

	manager.Packages = append(manager.Packages, testPkg)

	// Execute
	err := manager.GenerateIndex()

	// Verify
	if err != nil {
		t.Fatalf("GenerateIndex() failed: %v", err)
	}

	// Check Packages file created
	indexPath := filepath.Join(binhostRoot, "Packages")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatalf("Packages index not created")
	}

	// Read and verify index content
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index: %v", err)
	}

	indexStr := string(content)

	// Verify required fields present
	if !strings.Contains(indexStr, "CPV: app-test/sample-1.2.3") {
		t.Errorf("index missing CPV field")
	}
	if !strings.Contains(indexStr, "SIZE: 1024") {
		t.Errorf("index missing SIZE field")
	}
	if !strings.Contains(indexStr, "SHA256: abc123def456") {
		t.Errorf("index missing SHA256 field")
	}

	t.Logf("Packages index generated successfully (%d bytes)", len(content))
}

// TestBinhostManager_Clean tests orphaned file cleanup
func TestBinhostManager_Clean(t *testing.T) {
	t.Skip("Integration test - requires real binary packages (run in WSL2 Gentoo)")

	// Create temporary binhost
	binhostRoot := t.TempDir()

	// Create valid package
	validDir := filepath.Join(binhostRoot, "app-misc")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	validPkg := filepath.Join(validDir, "hello-1.0.0.gpkg.tar")
	if err := os.WriteFile(validPkg, []byte("valid"), 0644); err != nil {
		t.Fatalf("failed to create valid package: %v", err)
	}

	// Create orphaned file (not a package)
	orphanedFile := filepath.Join(validDir, "random.txt")
	if err := os.WriteFile(orphanedFile, []byte("orphan"), 0644); err != nil {
		t.Fatalf("failed to create orphaned file: %v", err)
	}

	// Create empty directory
	emptyDir := filepath.Join(binhostRoot, "dev-empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("failed to create empty directory: %v", err)
	}

	// Scan and clean
	manager := NewBinhostManager(binhostRoot)
	if err := manager.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	err := manager.Clean()

	// Verify
	if err != nil {
		t.Fatalf("Clean() failed: %v", err)
	}

	// Check valid package still exists
	if _, err := os.Stat(validPkg); os.IsNotExist(err) {
		t.Errorf("valid package was removed: %s", validPkg)
	}

	// Check orphaned file removed
	if _, err := os.Stat(orphanedFile); !os.IsNotExist(err) {
		t.Errorf("orphaned file still exists: %s", orphanedFile)
	}

	// Check empty directory removed
	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Errorf("empty directory still exists: %s", emptyDir)
	}

	t.Logf("Binhost cleaned successfully")
}

// TestBinhostManager_ErrorHandling tests error cases
func TestBinhostManager_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(t *testing.T) (*BinhostManager, *BinaryPackage)
		operation  string
		wantErrMsg string
	}{
		{
			name: "add package with invalid name",
			setupFunc: func(t *testing.T) (*BinhostManager, *BinaryPackage) {
				manager := NewBinhostManager(t.TempDir())
				pkg := &BinaryPackage{
					Package: pkg.NewPackage("invalid-name-no-category", "1.0.0", "0"),
				}
				return manager, pkg
			},
			operation:  "add",
			wantErrMsg: "invalid package name format",
		},
		{
			name: "add package with missing file",
			setupFunc: func(t *testing.T) (*BinhostManager, *BinaryPackage) {
				manager := NewBinhostManager(t.TempDir())
				pkg := &BinaryPackage{
					Package: pkg.NewPackage("app-test/pkg", "1.0.0", "0"),
					Path:    "/nonexistent/path/pkg-1.0.0.gpkg.tar",
				}
				return manager, pkg
			},
			operation:  "add",
			wantErrMsg: "failed to copy package file",
		},
		{
			name: "remove non-existent package",
			setupFunc: func(t *testing.T) (*BinhostManager, *BinaryPackage) {
				manager := NewBinhostManager(t.TempDir())
				return manager, nil
			},
			operation:  "remove",
			wantErrMsg: "package not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, pkg := tt.setupFunc(t)

			var err error
			switch tt.operation {
			case "add":
				err = manager.Add(pkg)
			case "remove":
				err = manager.Remove("app-test", "nonexistent", "1.0.0")
			}

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
			}
		})
	}
}

// BenchmarkBinhostManager_Add benchmarks adding packages
func BenchmarkBinhostManager_Add(b *testing.B) {
	// Setup
	binhostRoot := b.TempDir()
	manager := NewBinhostManager(binhostRoot)

	// Create test package file
	pkgDir := b.TempDir()
	pkgPath := filepath.Join(pkgDir, "test-1.0.0.gpkg.tar")
	if err := os.WriteFile(pkgPath, []byte("test package content"), 0644); err != nil {
		b.Fatalf("failed to create test package: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pkg := &BinaryPackage{
			Package: pkg.NewPackage("app-bench/test", "1.0.0", "0"),
			Format:  FormatGPKG,
			Path:    pkgPath,
			Size:    20,
		}

		if err := manager.Add(pkg); err != nil {
			b.Fatalf("Add() failed: %v", err)
		}

		// Clean up for next iteration
		manager.Packages = nil
	}
}

// BenchmarkBinhostManager_GenerateIndex benchmarks index generation
func BenchmarkBinhostManager_GenerateIndex(b *testing.B) {
	// Setup binhost with multiple packages
	binhostRoot := b.TempDir()
	manager := NewBinhostManager(binhostRoot)

	// Add 100 test packages
	for i := 0; i < 100; i++ {
		pkg := &BinaryPackage{
			Package:   pkg.NewPackage("app-bench/pkg", "1.0.0", "0"),
			Format:    FormatGPKG,
			Path:      filepath.Join(binhostRoot, "app-bench", "pkg-1.0.0.gpkg.tar"),
			Size:      1024,
			Checksum:  "abc123",
			BuildInfo: &BuildMetadata{BuildDate: time.Now(), Repository: "gentoo"},
		}
		manager.Packages = append(manager.Packages, pkg)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := manager.GenerateIndex(); err != nil {
			b.Fatalf("GenerateIndex() failed: %v", err)
		}
	}
}
