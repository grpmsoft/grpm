package ebuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// Binary Installation Tests
// ============================================================================

func TestHelpers_Dobin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a test executable
	exePath := createTestFile(t, tmpDir, "myapp", "#!/bin/sh\necho hello")

	err := helpers.Dobin([]string{exePath})
	if err != nil {
		t.Fatalf("Dobin failed: %v", err)
	}

	// Verify file was installed
	installedPath := filepath.Join(helpers.env.D, "usr", "bin", "myapp")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dobin_NoFiles(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dobin([]string{})
	if err == nil {
		t.Error("expected error with no files")
	}
}

func TestHelpers_Dobin_MissingFile(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dobin([]string{"/nonexistent/file"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestHelpers_Dosbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "mydaemon", "#!/bin/sh\necho daemon")

	err := helpers.Dosbin([]string{exePath})
	if err != nil {
		t.Fatalf("Dosbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "sbin", "mydaemon")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "src.sh", "#!/bin/sh\necho src")

	err := helpers.Newbin([]string{exePath, "dest"})
	if err != nil {
		t.Fatalf("Newbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "bin", "dest")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newsbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "src.sh", "#!/bin/sh\necho src")

	err := helpers.Newsbin([]string{exePath, "dest"})
	if err != nil {
		t.Fatalf("Newsbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "sbin", "dest")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doexe(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set EXEDESTTREE first
	err := helpers.Exeinto([]string{"/usr/libexec"})
	if err != nil {
		t.Fatalf("Exeinto failed: %v", err)
	}

	exePath := createTestFile(t, tmpDir, "script.sh", "#!/bin/sh\necho script")

	err = helpers.Doexe([]string{exePath})
	if err != nil {
		t.Fatalf("Doexe failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "libexec", "script.sh")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doexe_NoExeinto(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "script.sh", "#!/bin/sh\necho script")

	err := helpers.Doexe([]string{exePath})
	if err == nil {
		t.Error("expected error when EXEDESTTREE not set")
	}
}

// ============================================================================
// File Installation Tests
// ============================================================================

func TestHelpers_Doins(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set install directory
	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "config.conf", "key=value")

	err = helpers.Doins([]string{filePath})
	if err != nil {
		t.Fatalf("Doins failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "myapp", "config.conf")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doins_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a directory with files
	subDir := filepath.Join(tmpDir, "subdir")
	createTestFile(t, subDir, "file1.txt", "content1")
	createTestFile(t, subDir, "file2.txt", "content2")

	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	err = helpers.Doins([]string{"-r", subDir})
	if err != nil {
		t.Fatalf("Doins -r failed: %v", err)
	}

	// Verify files were installed
	installedPath1 := filepath.Join(helpers.env.D, "usr", "share", "myapp", "subdir", "file1.txt")
	if _, err := os.Stat(installedPath1); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath1)
	}
}

func TestHelpers_Doins_DirectoryWithoutRecursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Doins([]string{subDir})
	if err == nil {
		t.Error("expected error for directory without -r")
	}
}

func TestHelpers_Newins(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{"/etc"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "source.conf", "key=value")

	err = helpers.Newins([]string{filePath, "dest.conf"})
	if err != nil {
		t.Fatalf("Newins failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "dest.conf")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Documentation Tests
// ============================================================================

func TestHelpers_Dodoc(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "README", "This is readme")

	err := helpers.Dodoc([]string{filePath})
	if err != nil {
		t.Fatalf("Dodoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "README")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dodoc_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	docDir := filepath.Join(tmpDir, "docs")
	createTestFile(t, docDir, "intro.txt", "intro")
	createTestFile(t, docDir, "guide.txt", "guide")

	err := helpers.Dodoc([]string{"-r", docDir})
	if err != nil {
		t.Fatalf("Dodoc -r failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "docs", "intro.txt")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dodoc_WithDocinto(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Docinto([]string{"examples"})
	if err != nil {
		t.Fatalf("Docinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "example.txt", "example content")

	err = helpers.Dodoc([]string{filePath})
	if err != nil {
		t.Fatalf("Dodoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "examples", "example.txt")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newdoc(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "README.md", "# Title")

	err := helpers.Newdoc([]string{filePath, "README"})
	if err != nil {
		t.Fatalf("Newdoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "README")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doman(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo.1", ".TH FOO 1")

	err := helpers.Doman([]string{manPath})
	if err != nil {
		t.Fatalf("Doman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man1", "foo.1")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doman_Section8(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "bar.8", ".TH BAR 8")

	err := helpers.Doman([]string{manPath})
	if err != nil {
		t.Fatalf("Doman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man8", "bar.8")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newman(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo.man", ".TH FOO 1")

	err := helpers.Newman([]string{manPath, "foo.1"})
	if err != nil {
		t.Fatalf("Newman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man1", "foo.1")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Info Page Tests
// ============================================================================

func TestHelpers_Doinfo(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	infoPath := createTestFile(t, tmpDir, "myapp.info", "GNU Info file content")

	err := helpers.Doinfo([]string{infoPath})
	if err != nil {
		t.Fatalf("Doinfo failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "info", "myapp.info")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}

	// Check file mode (skip on Windows - permissions work differently)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(installedPath)
		if err != nil {
			t.Fatalf("failed to stat installed file: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("expected mode 0644, got %o", info.Mode().Perm())
		}
	}
}

func TestHelpers_Doinfo_GzippedFile(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	infoPath := createTestFile(t, tmpDir, "myapp.info.gz", "compressed info content")

	err := helpers.Doinfo([]string{infoPath})
	if err != nil {
		t.Fatalf("Doinfo failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "info", "myapp.info.gz")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doinfo_MultipleFiles(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	info1 := createTestFile(t, tmpDir, "foo.info", "foo info")
	info2 := createTestFile(t, tmpDir, "bar.info", "bar info")

	err := helpers.Doinfo([]string{info1, info2})
	if err != nil {
		t.Fatalf("Doinfo failed: %v", err)
	}

	for _, name := range []string{"foo.info", "bar.info"} {
		installedPath := filepath.Join(helpers.env.D, "usr", "share", "info", name)
		if _, err := os.Stat(installedPath); os.IsNotExist(err) {
			t.Errorf("expected file at %s", installedPath)
		}
	}
}

func TestHelpers_Doinfo_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Doinfo([]string{})
	if err == nil {
		t.Error("expected error with no arguments")
	}
}

func TestHelpers_Doinfo_Directory(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "infos")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Doinfo([]string{subDir})
	if err == nil {
		t.Error("expected error when passing directory to doinfo")
	}
}

func TestHelpers_Doinfo_NonExistent(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Doinfo([]string{filepath.Join(tmpDir, "nonexistent.info")})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestHelpers_Doinfo_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Doinfo([]string{"/some/file.info"})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

// ============================================================================
// Domo (gettext .mo files) Tests - PMS Section 12.3.9
// ============================================================================

func TestHelpers_Domo(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "8"

	moPath := createTestFile(t, tmpDir, "de.mo", "German translations")

	err := helpers.Domo([]string{moPath})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	// Check installed path: /usr/share/locale/de/LC_MESSAGES/${PN}.mo
	installedPath := filepath.Join(helpers.env.D, "usr", "share", "locale", "de", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}

	// Check file mode (skip on Windows - permissions work differently)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(installedPath)
		if err != nil {
			t.Fatalf("failed to stat installed file: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("expected mode 0644, got %o", info.Mode().Perm())
		}
	}
}

func TestHelpers_Domo_LocaleWithCountry(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "8"

	// Test locale with country code (e.g., fr_FR, pt_BR)
	moPath := createTestFile(t, tmpDir, "fr_FR.mo", "French translations")

	err := helpers.Domo([]string{moPath})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	// Check installed path: /usr/share/locale/fr_FR/LC_MESSAGES/${PN}.mo
	installedPath := filepath.Join(helpers.env.D, "usr", "share", "locale", "fr_FR", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Domo_MultipleFiles(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "8"

	de := createTestFile(t, tmpDir, "de.mo", "German")
	fr := createTestFile(t, tmpDir, "fr.mo", "French")
	es := createTestFile(t, tmpDir, "es.mo", "Spanish")

	err := helpers.Domo([]string{de, fr, es})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	for _, locale := range []string{"de", "fr", "es"} {
		installedPath := filepath.Join(helpers.env.D, "usr", "share", "locale", locale, "LC_MESSAGES", "zlib.mo")
		if _, err := os.Stat(installedPath); os.IsNotExist(err) {
			t.Errorf("expected file at %s", installedPath)
		}
	}
}

func TestHelpers_Domo_EAPI6_UsesDestTree(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "6"

	// Set custom DESTTREE (simulates "into /opt/myapp")
	helpers.destTree = "/opt/myapp"

	moPath := createTestFile(t, tmpDir, "de.mo", "German translations")

	err := helpers.Domo([]string{moPath})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	// EAPI 6: Should use ${DESTTREE}/share/locale
	installedPath := filepath.Join(helpers.env.D, "opt", "myapp", "share", "locale", "de", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("EAPI 6 should use DESTTREE, expected file at %s", installedPath)
	}
}

func TestHelpers_Domo_EAPI7_IgnoresDestTree(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "7"

	// Set custom DESTTREE - should be ignored in EAPI 7+
	helpers.destTree = "/opt/myapp"

	moPath := createTestFile(t, tmpDir, "de.mo", "German translations")

	err := helpers.Domo([]string{moPath})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	// EAPI 7+: Should always use /usr/share/locale (fixed)
	installedPath := filepath.Join(helpers.env.D, "usr", "share", "locale", "de", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("EAPI 7+ should use fixed /usr/share/locale, expected file at %s", installedPath)
	}

	// Verify it's NOT in the DESTTREE path
	wrongPath := filepath.Join(helpers.env.D, "opt", "myapp", "share", "locale", "de", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("EAPI 7+ should NOT use DESTTREE, but file found at %s", wrongPath)
	}
}

func TestHelpers_Domo_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Domo([]string{})
	if err == nil {
		t.Error("expected error with no arguments")
	}
}

func TestHelpers_Domo_Directory(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "locales")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Domo([]string{subDir})
	if err == nil {
		t.Error("expected error when passing directory to domo")
	}
}

func TestHelpers_Domo_NonExistent(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Domo([]string{filepath.Join(tmpDir, "nonexistent.mo")})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestHelpers_Domo_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Domo([]string{"/some/file.mo"})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

func TestHelpers_Domo_SubdirectoryFile(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)
	helpers.env.EAPI = "8"

	// Create .mo file in a po/ subdirectory
	moPath := createTestFile(t, filepath.Join(tmpDir, "po"), "ja.mo", "Japanese translations")

	err := helpers.Domo([]string{moPath})
	if err != nil {
		t.Fatalf("Domo failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "locale", "ja", "LC_MESSAGES", "zlib.mo")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Library/Header Tests
// ============================================================================

func TestHelpers_Dolib(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.so", "ELF binary")

	err := helpers.Dolib([]string{libPath})
	if err != nil {
		t.Fatalf("Dolib failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.so")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_DolibSo(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.so.1.0", "ELF binary")

	err := helpers.DolibSo([]string{libPath})
	if err != nil {
		t.Fatalf("DolibSo failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.so.1.0")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_DolibA(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.a", "static lib")

	err := helpers.DolibA([]string{libPath})
	if err != nil {
		t.Fatalf("DolibA failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.a")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doheader(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	headerPath := createTestFile(t, tmpDir, "foo.h", "#ifndef FOO_H")

	err := helpers.Doheader([]string{headerPath})
	if err != nil {
		t.Fatalf("Doheader failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "include", "foo.h")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doheader_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	incDir := filepath.Join(tmpDir, "include")
	createTestFile(t, incDir, "foo.h", "#ifndef FOO_H")
	createTestFile(t, incDir, "bar.h", "#ifndef BAR_H")

	err := helpers.Doheader([]string{"-r", incDir})
	if err != nil {
		t.Fatalf("Doheader -r failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "include", "include", "foo.h")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Directory Creation Tests
// ============================================================================

func TestHelpers_Dodir(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dodir([]string{"/usr/share/myapp", "/etc/myapp"})
	if err != nil {
		t.Fatalf("Dodir failed: %v", err)
	}

	dir1 := filepath.Join(helpers.env.D, "usr", "share", "myapp")
	if info, err := os.Stat(dir1); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir1)
	}

	dir2 := filepath.Join(helpers.env.D, "etc", "myapp")
	if info, err := os.Stat(dir2); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir2)
	}
}

func TestHelpers_Dodir_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dodir([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Keepdir(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Keepdir([]string{"/var/lib/myapp"})
	if err != nil {
		t.Fatalf("Keepdir failed: %v", err)
	}

	dir := filepath.Join(helpers.env.D, "var", "lib", "myapp")
	if info, err := os.Stat(dir); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir)
	}

	// Check for .keep file
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".keep") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected .keep file in %s", dir)
	}
}

// ============================================================================
// parseMode Tests
// ============================================================================

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected os.FileMode
		wantErr  bool
	}{
		{"-m0644", 0644, false},
		{"-m0755", 0755, false},
		{"-m0600", 0600, false},
		{"-m0700", 0700, false},
		{"", 0644, false}, // Default
		{"-minvalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := parseMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tt.expected {
				t.Errorf("expected %o, got %o", tt.expected, mode)
			}
		})
	}
}

// ============================================================================
// Symlink Tests
// ============================================================================

func TestHelpers_InstallFile_Symlink(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a regular file and a symlink to it
	targetPath := createTestFile(t, tmpDir, "target.txt", "target content")
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink("target.txt", symlinkPath); err != nil {
		// Symlinks require admin privileges on Windows, skip if not available
		t.Skipf("skipping symlink test: %v", err)
	}

	err := helpers.Insinto([]string{"/usr/share/test"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	// Install the target file first
	err = helpers.Doins([]string{targetPath})
	if err != nil {
		t.Fatalf("Doins target failed: %v", err)
	}

	// Install the symlink
	err = helpers.Doins([]string{symlinkPath})
	if err != nil {
		t.Fatalf("Doins symlink failed: %v", err)
	}

	// Verify symlink was preserved
	installedLink := filepath.Join(helpers.env.D, "usr", "share", "test", "link.txt")
	info, err := os.Lstat(installedLink)
	if err != nil {
		t.Fatalf("failed to stat installed symlink: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink to be preserved")
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestHelpers_Dobin_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Dobin([]string{"somefile"})
	if err == nil {
		t.Error("expected error with nil environment")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}
}

func TestHelpers_Dodir_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Dodir([]string{"/some/dir"})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

func TestHelpers_Newbin_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newbin([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newins_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newins([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newdoc_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newdoc([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newman_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newman([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Doman_NoExtension(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo", ".TH FOO 1")

	err := helpers.Doman([]string{manPath})
	if err == nil {
		t.Error("expected error for file without section extension")
	}
}

func TestHelpers_Dobin_Directory(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Dobin([]string{subDir})
	if err == nil {
		t.Error("expected error for directory")
	}
}

// ============================================================================
// Additional Installation Helper Tests
// ============================================================================

func TestHelpers_Doconfd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	confPath := createTestFile(t, tmpDir, "myservice", "# config")

	err := helpers.Doconfd([]string{confPath})
	if err != nil {
		t.Fatalf("Doconfd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "conf.d", "myservice")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doinitd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	initPath := createTestFile(t, tmpDir, "myservice", "#!/sbin/openrc-run")

	err := helpers.Doinitd([]string{initPath})
	if err != nil {
		t.Fatalf("Doinitd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "init.d", "myservice")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doenvd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	envPath := createTestFile(t, tmpDir, "99myapp", "PATH=/opt/myapp/bin")

	err := helpers.Doenvd([]string{envPath})
	if err != nil {
		t.Fatalf("Doenvd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "env.d", "99myapp")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}
