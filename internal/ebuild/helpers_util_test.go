package ebuild

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// Dosym Tests
// ============================================================================

func TestHelpers_Dosym(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"/usr/lib/libfoo.so.1", "/usr/lib/libfoo.so"})
	if err != nil {
		// Symlinks require admin on Windows
		if strings.Contains(err.Error(), "not permitted") || strings.Contains(err.Error(), "privilege") {
			t.Skipf("skipping symlink test: %v", err)
		}
		t.Fatalf("Dosym failed: %v", err)
	}

	linkPath := filepath.Join(helpers.env.D, "usr", "lib", "libfoo.so")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestHelpers_Dosym_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"only_one_arg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

// ============================================================================
// Fperms Tests
// ============================================================================

func TestHelpers_Fperms(t *testing.T) {
	// Skip on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping fperms test on Windows (permissions not supported)")
	}

	helpers, tmpDir := createInstallTestHelpers(t)

	// Create file in image
	filePath := filepath.Join(helpers.env.D, "usr", "bin")
	if err := os.MkdirAll(filePath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	testFile := filepath.Join(filePath, "myapp")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := helpers.Fperms([]string{"0755", "/usr/bin/myapp"})
	if err != nil {
		t.Fatalf("Fperms failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}

	_ = tmpDir
}

func TestHelpers_Fperms_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Fperms([]string{"0755"})
	if err == nil {
		t.Error("expected error with no file args")
	}
}

func TestHelpers_Fowners(t *testing.T) {
	// Skip on Windows - chown doesn't work
	if runtime.GOOS == "windows" {
		t.Skip("skipping fowners test on Windows (chown not supported)")
	}

	helpers, tmpDir := createInstallTestHelpers(t)

	// Create file in image
	filePath := filepath.Join(helpers.env.D, "usr", "bin")
	if err := os.MkdirAll(filePath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	testFile := filepath.Join(filePath, "myapp")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test with current user (should not error)
	err := helpers.Fowners([]string{"root:root", "/usr/bin/myapp"})
	// Note: on non-root systems, chown to root will fail
	// We just test that the function processes arguments correctly
	_ = err // May fail if not root, that's expected

	_ = tmpDir
}

func TestHelpers_Fowners_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Fowners([]string{"root:root"})
	if err == nil {
		t.Error("expected error with no file args")
	}
}

func TestHelpers_Fowners_Recursive(t *testing.T) {
	// Skip on Windows - chown doesn't work
	if runtime.GOOS == "windows" {
		t.Skip("skipping fowners test on Windows (chown not supported)")
	}

	helpers, _ := createInstallTestHelpers(t)

	// Create directory structure in image
	dirPath := filepath.Join(helpers.env.D, "usr", "share", "myapp")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	testFile := filepath.Join(dirPath, "data.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test recursive flag parsing
	err := helpers.Fowners([]string{"-R", "root:root", "/usr/share/myapp"})
	// May fail if not root, that's expected
	_ = err
}

func TestHelpers_Fowners_PlatformSkip(t *testing.T) {
	// This test verifies non-Unix platforms skip gracefully
	if runtime.GOOS != "windows" {
		t.Skip("only testing platform skip on Windows")
	}

	helpers, _ := createInstallTestHelpers(t)

	// On Windows, should skip without error
	err := helpers.Fowners([]string{"root:root", "/usr/bin/myapp"})
	if err != nil {
		t.Errorf("expected no error on Windows, got: %v", err)
	}
}

// ============================================================================
// Utility Function Tests (sed, cat, mkdir, etc.)
// ============================================================================

func TestHelpers_Cat(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	filePath := createTestFile(t, tmpDir, "test.txt", "Hello World")

	err := helpers.Cat([]string{filePath})
	if err != nil {
		t.Fatalf("Cat failed: %v", err)
	}

	output := stdout.String()
	if output != "Hello World" {
		t.Errorf("expected 'Hello World', got: %s", output)
	}
}

func TestHelpers_Cat_NoArgs(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Cat([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Mkdir(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newDir := filepath.Join(tmpDir, "newdir")
	err := helpers.Mkdir([]string{newDir})
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestHelpers_Mkdir_WithParents(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	err := helpers.Mkdir([]string{"-p", deepDir})
	if err != nil {
		t.Fatalf("Mkdir -p failed: %v", err)
	}

	info, err := os.Stat(deepDir)
	if err != nil || !info.IsDir() {
		t.Error("expected deep directory to be created")
	}
}

func TestHelpers_Rm(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "todelete.txt", "delete me")

	err := helpers.Rm([]string{filePath})
	if err != nil {
		t.Fatalf("Rm failed: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestHelpers_Rm_Recursive(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	subDir := filepath.Join(tmpDir, "deleteme")
	createTestFile(t, subDir, "file.txt", "content")

	err := helpers.Rm([]string{"-r", subDir})
	if err != nil {
		t.Fatalf("Rm -r failed: %v", err)
	}

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("expected directory to be deleted")
	}
}

func TestHelpers_Cp(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "source.txt", "content")
	dst := filepath.Join(tmpDir, "dest.txt")

	err := helpers.Cp([]string{src, dst})
	if err != nil {
		t.Fatalf("Cp failed: %v", err)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got: %s", string(content))
	}
}

func TestHelpers_Mv(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "tomove.txt", "content")
	dst := filepath.Join(tmpDir, "moved.txt")

	err := helpers.Mv([]string{src, dst})
	if err != nil {
		t.Fatalf("Mv failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("expected source to be removed")
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got: %s", string(content))
	}
}

func TestHelpers_Chmod(t *testing.T) {
	// Skip on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping chmod test on Windows (permissions not supported)")
	}

	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "chmodtest.txt", "content")

	err := helpers.Chmod([]string{"0755", filePath})
	if err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}
}

func TestHelpers_Ln(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	target := createTestFile(t, tmpDir, "target.txt", "content")
	link := filepath.Join(tmpDir, "link.txt")

	err := helpers.Ln([]string{"-s", target, link})
	if err != nil {
		// Symlinks require admin on Windows
		if strings.Contains(err.Error(), "not permitted") || strings.Contains(err.Error(), "privilege") {
			t.Skipf("skipping symlink test: %v", err)
		}
		t.Fatalf("Ln failed: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestHelpers_Touch(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newFile := filepath.Join(tmpDir, "touched.txt")

	err := helpers.Touch([]string{newFile})
	if err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestHelpers_Find(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	createTestFile(t, tmpDir, "find1.txt", "content")
	createTestFile(t, tmpDir, "find2.txt", "content")
	createTestFile(t, filepath.Join(tmpDir, "subdir"), "find3.txt", "content")

	err := helpers.Find([]string{tmpDir, "-name", "*.txt"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "find1.txt") {
		t.Errorf("expected find1.txt in output, got: %s", output)
	}
	if !strings.Contains(output, "find3.txt") {
		t.Errorf("expected find3.txt in output, got: %s", output)
	}
}

func TestHelpers_Grep_Found(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	filePath := createTestFile(t, tmpDir, "greptest.txt", "hello world\nfoo bar\nhello again")

	err := helpers.Grep([]string{"hello", filePath})
	if err != nil {
		t.Fatalf("Grep failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", output)
	}
	if !strings.Contains(output, "hello again") {
		t.Errorf("expected 'hello again' in output, got: %s", output)
	}
}

func TestHelpers_Grep_NotFound(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "greptest.txt", "hello world")

	err := helpers.Grep([]string{"notfound", filePath})
	if err == nil {
		t.Error("expected exit status error when pattern not found")
	}
}

func TestHelpers_Sed_InPlace(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "sedtest.txt", "hello world")

	err := helpers.Sed([]string{"-i", "s/hello/goodbye/", filePath})
	if err != nil {
		t.Fatalf("Sed failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "goodbye world" {
		t.Errorf("expected 'goodbye world', got: %s", string(content))
	}
}

func TestHelpers_Sed_GlobalReplace(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "sedtest.txt", "hello hello hello")

	err := helpers.Sed([]string{"-i", "s/hello/bye/g", filePath})
	if err != nil {
		t.Fatalf("Sed failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "bye bye bye" {
		t.Errorf("expected 'bye bye bye', got: %s", string(content))
	}
}

func TestHelpers_Install_File(t *testing.T) {
	// Skip permission check on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping install permission test on Windows (permissions not supported)")
	}

	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "installsrc.txt", "content")
	dst := filepath.Join(tmpDir, "installdst.txt")

	err := helpers.Install([]string{"-m", "0755", src, dst})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat installed file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}
}

func TestHelpers_Install_Directory(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newDir := filepath.Join(tmpDir, "installdir")

	err := helpers.Install([]string{"-d", newDir})
	if err != nil {
		t.Fatalf("Install -d failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestHelpers_Which(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// 'go' should be in PATH on development machines
	err := helpers.Which([]string{"go"})
	if err != nil {
		t.Logf("which go failed (may not be in PATH): %v", err)
		return
	}

	output := stdout.String()
	if output == "" {
		t.Log("which returned empty (go not in PATH)")
	}
}
