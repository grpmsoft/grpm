package ebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ============================================================================
// EAPI 8 Helper Tests: dosym -r, dostrip, einstalldocs
// ============================================================================

// TestCalculateRelativePath tests the relative path calculation for dosym -r
func TestCalculateRelativePath(t *testing.T) {
	tests := []struct {
		name       string
		linkPath   string
		targetPath string
		want       string
	}{
		{
			name:       "same directory - library version symlink",
			linkPath:   "/usr/lib/libfoo.so",
			targetPath: "/usr/lib/libfoo.so.1",
			want:       "libfoo.so.1",
		},
		{
			name:       "same directory - python symlink",
			linkPath:   "/usr/bin/python",
			targetPath: "/usr/bin/python3.11",
			want:       "python3.11",
		},
		{
			name:       "cross directory - lib to lib64",
			linkPath:   "/usr/lib/libfoo.so",
			targetPath: "/usr/lib64/libfoo.so.1",
			want:       "../lib64/libfoo.so.1",
		},
		{
			name:       "multiple levels up",
			linkPath:   "/usr/lib/foo/bar/libfoo.so",
			targetPath: "/usr/lib64/libfoo.so.1",
			want:       "../../../lib64/libfoo.so.1", // Fixed: 3 levels up from /usr/lib/foo/bar
		},
		{
			name:       "deep target path",
			linkPath:   "/usr/bin/app",
			targetPath: "/opt/app/v1.0/bin/app",
			want:       "../../opt/app/v1.0/bin/app",
		},
		{
			name:       "same file name different dir",
			linkPath:   "/etc/alternatives/java",
			targetPath: "/usr/lib/jvm/java-11/bin/java",
			want:       "../../usr/lib/jvm/java-11/bin/java", // Fixed: 2 levels up from /etc/alternatives
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateRelativePath(tt.linkPath, tt.targetPath)
			// On Windows, paths may use backslash - normalize for comparison
			got = filepath.ToSlash(got)
			if got != tt.want {
				t.Errorf("calculateRelativePath(%q, %q) = %q, want %q",
					tt.linkPath, tt.targetPath, got, tt.want)
			}
		})
	}
}

func TestHelpers_Dosym_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires admin privileges)")
	}

	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"/usr/lib/libfoo.so.1", "/usr/lib/libfoo.so"})
	if err != nil {
		t.Fatalf("Dosym failed: %v", err)
	}

	// Check symlink was created
	linkPath := filepath.Join(helpers.env.D, "usr", "lib", "libfoo.so")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("failed to stat symlink: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink to be created")
	}

	// Check target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to readlink: %v", err)
	}
	if target != "/usr/lib/libfoo.so.1" {
		t.Errorf("symlink target = %q, want /usr/lib/libfoo.so.1", target)
	}
}

func TestHelpers_Dosym_Relative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires admin privileges)")
	}

	helpers, _ := createInstallTestHelpers(t)

	// Use -r flag for automatic relative path calculation
	err := helpers.Dosym([]string{"-r", "/usr/lib/libfoo.so.1", "/usr/lib/libfoo.so"})
	if err != nil {
		t.Fatalf("Dosym -r failed: %v", err)
	}

	// Check symlink was created
	linkPath := filepath.Join(helpers.env.D, "usr", "lib", "libfoo.so")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("failed to stat symlink: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink to be created")
	}

	// Check that target is relative (not absolute)
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to readlink: %v", err)
	}
	// Should be "libfoo.so.1", not "/usr/lib/libfoo.so.1"
	if target != "libfoo.so.1" {
		t.Errorf("relative symlink target = %q, want libfoo.so.1", target)
	}
}

func TestHelpers_Dosym_RelativeCrossDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires admin privileges)")
	}

	helpers, _ := createInstallTestHelpers(t)

	// Cross-directory relative symlink
	err := helpers.Dosym([]string{"-r", "/usr/lib64/libfoo.so.1", "/usr/lib/libfoo.so"})
	if err != nil {
		t.Fatalf("Dosym -r cross-dir failed: %v", err)
	}

	linkPath := filepath.Join(helpers.env.D, "usr", "lib", "libfoo.so")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to readlink: %v", err)
	}

	// Should be relative path like "../lib64/libfoo.so.1"
	expected := "../lib64/libfoo.so.1"
	target = filepath.ToSlash(target) // Normalize for Windows
	if target != expected {
		t.Errorf("cross-dir symlink target = %q, want %q", target, expected)
	}
}

func TestHelpers_DosymRelative_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_DosymRelative_OnlyTarget(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"/target"})
	if err == nil {
		t.Error("expected error with only target")
	}
}

func TestHelpers_DosymRelative_OnlyFlag(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	// -r with no other args
	err := helpers.Dosym([]string{"-r"})
	if err == nil {
		t.Error("expected error with -r flag only")
	}

	// -r with only target
	err = helpers.Dosym([]string{"-r", "/target"})
	if err == nil {
		t.Error("expected error with -r and only target")
	}
}

// ============================================================================
// Dostrip Tests
// ============================================================================

func TestHelpers_Dostrip_Include(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Dostrip([]string{"/usr/bin", "/usr/lib"})
	if err != nil {
		t.Fatalf("Dostrip failed: %v", err)
	}

	include := helpers.GetStripInclude()
	if len(include) != 2 {
		t.Errorf("expected 2 include paths, got %d", len(include))
	}

	expected := []string{"/usr/bin", "/usr/lib"}
	for i, exp := range expected {
		if i >= len(include) || include[i] != exp {
			t.Errorf("include[%d] = %q, want %q", i, include[i], exp)
		}
	}
}

func TestHelpers_Dostrip_Exclude(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Dostrip([]string{"-x", "/usr/lib/debug"})
	if err != nil {
		t.Fatalf("Dostrip -x failed: %v", err)
	}

	exclude := helpers.GetStripExclude()
	if len(exclude) != 1 {
		t.Errorf("expected 1 exclude path, got %d", len(exclude))
	}
	if len(exclude) > 0 && exclude[0] != "/usr/lib/debug" {
		t.Errorf("exclude[0] = %q, want /usr/lib/debug", exclude[0])
	}
}

func TestHelpers_Dostrip_Mixed(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// First add includes
	err := helpers.Dostrip([]string{"/usr/bin", "/usr/lib"})
	if err != nil {
		t.Fatalf("Dostrip include failed: %v", err)
	}

	// Then add excludes
	err = helpers.Dostrip([]string{"-x", "/usr/lib/debug", "/usr/lib/modules"})
	if err != nil {
		t.Fatalf("Dostrip exclude failed: %v", err)
	}

	include := helpers.GetStripInclude()
	exclude := helpers.GetStripExclude()

	if len(include) != 2 {
		t.Errorf("expected 2 include paths, got %d", len(include))
	}
	if len(exclude) != 2 {
		t.Errorf("expected 2 exclude paths, got %d", len(exclude))
	}
}

func TestHelpers_Dostrip_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Dostrip([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Dostrip_ExcludeNoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Dostrip([]string{"-x"})
	if err == nil {
		t.Error("expected error with -x but no paths")
	}
}

func TestHelpers_Dostrip_NormalizePath(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Path without leading slash should be normalized
	err := helpers.Dostrip([]string{"usr/bin"})
	if err != nil {
		t.Fatalf("Dostrip failed: %v", err)
	}

	include := helpers.GetStripInclude()
	if len(include) != 1 {
		t.Fatalf("expected 1 include path, got %d", len(include))
	}
	if include[0] != "/usr/bin" {
		t.Errorf("include[0] = %q, want /usr/bin (normalized)", include[0])
	}
}

// ============================================================================
// ShouldStrip Tests
// ============================================================================

func TestHelpers_ShouldStrip_Default(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// By default (no include/exclude set), everything should be stripped
	if !helpers.ShouldStrip("/usr/bin/foo") {
		t.Error("expected /usr/bin/foo to be stripped by default")
	}
	if !helpers.ShouldStrip("/usr/lib/libfoo.so") {
		t.Error("expected /usr/lib/libfoo.so to be stripped by default")
	}
}

func TestHelpers_ShouldStrip_WithExclude(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Exclude debug directory
	_ = helpers.Dostrip([]string{"-x", "/usr/lib/debug"})

	// Files in excluded path should not be stripped
	if helpers.ShouldStrip("/usr/lib/debug/foo.debug") {
		t.Error("expected /usr/lib/debug/foo.debug to NOT be stripped")
	}

	// Files outside excluded path should be stripped
	if !helpers.ShouldStrip("/usr/bin/foo") {
		t.Error("expected /usr/bin/foo to be stripped")
	}
}

func TestHelpers_ShouldStrip_WithInclude(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Only include /usr/bin
	_ = helpers.Dostrip([]string{"/usr/bin"})

	// Files in included path should be stripped
	if !helpers.ShouldStrip("/usr/bin/foo") {
		t.Error("expected /usr/bin/foo to be stripped")
	}

	// Files outside included path should NOT be stripped
	if helpers.ShouldStrip("/usr/lib/libfoo.so") {
		t.Error("expected /usr/lib/libfoo.so to NOT be stripped")
	}
}

func TestHelpers_ShouldStrip_ExcludeWins(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Include entire /usr/lib
	_ = helpers.Dostrip([]string{"/usr/lib"})
	// But exclude debug subdirectory
	_ = helpers.Dostrip([]string{"-x", "/usr/lib/debug"})

	// Should strip files in /usr/lib
	if !helpers.ShouldStrip("/usr/lib/libfoo.so") {
		t.Error("expected /usr/lib/libfoo.so to be stripped")
	}

	// Should NOT strip files in excluded subpath
	if helpers.ShouldStrip("/usr/lib/debug/foo.debug") {
		t.Error("expected /usr/lib/debug/foo.debug to NOT be stripped (exclude wins)")
	}
}

// ============================================================================
// Einstalldocs Tests
// ============================================================================

func TestHelpers_Einstalldocs_StandardFiles(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set source directory
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	helpers.env.S = sourceDir

	// Create standard documentation files
	docsToCreate := []string{"README", "LICENSE", "CHANGELOG", "AUTHORS"}
	for _, doc := range docsToCreate {
		docPath := filepath.Join(sourceDir, doc)
		if err := os.WriteFile(docPath, []byte("test content for "+doc), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", doc, err)
		}
	}

	err := helpers.Einstalldocs([]string{})
	if err != nil {
		t.Fatalf("Einstalldocs failed: %v", err)
	}

	// Check that files were installed
	docDir := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF)
	for _, doc := range docsToCreate {
		docPath := filepath.Join(docDir, doc)
		if _, err := os.Stat(docPath); os.IsNotExist(err) {
			t.Errorf("expected %s to be installed at %s", doc, docPath)
		}
	}
}

func TestHelpers_Einstalldocs_WithDOCS(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set source directory
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	helpers.env.S = sourceDir

	// Create custom doc file
	customDoc := filepath.Join(sourceDir, "CUSTOM.md")
	if err := os.WriteFile(customDoc, []byte("custom doc"), 0644); err != nil {
		t.Fatalf("failed to create custom doc: %v", err)
	}

	// Set DOCS variable
	helpers.env.SetVar("DOCS", "CUSTOM.md")

	err := helpers.Einstalldocs([]string{})
	if err != nil {
		t.Fatalf("Einstalldocs failed: %v", err)
	}

	// Check that CUSTOM.md was installed
	docDir := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF)
	customPath := filepath.Join(docDir, "CUSTOM.md")
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Errorf("expected CUSTOM.md to be installed at %s", customPath)
	}
}

func TestHelpers_Einstalldocs_NoSourceDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Einstalldocs([]string{})
	if err == nil {
		t.Error("expected error when S is not set")
	}
}

func TestHelpers_Einstalldocs_EmptySourceDir(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set source directory (empty)
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	helpers.env.S = sourceDir

	// Should succeed even with no files to install
	err := helpers.Einstalldocs([]string{})
	if err != nil {
		t.Errorf("Einstalldocs failed on empty dir: %v", err)
	}
}

func TestHelpers_Einstalldocs_PatternMatching(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set source directory
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	helpers.env.S = sourceDir

	// Create files matching various patterns
	files := []string{
		"README.md",
		"README.rst",
		"LICENSE-MIT",
		"ChangeLog.txt",
		"NEWS",
	}
	for _, f := range files {
		fPath := filepath.Join(sourceDir, f)
		if err := os.WriteFile(fPath, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	err := helpers.Einstalldocs([]string{})
	if err != nil {
		t.Fatalf("Einstalldocs failed: %v", err)
	}

	// Check files were installed
	docDir := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF)
	for _, f := range files {
		fPath := filepath.Join(docDir, f)
		if _, err := os.Stat(fPath); os.IsNotExist(err) {
			t.Errorf("expected %s to be installed at %s", f, fPath)
		}
	}
}
