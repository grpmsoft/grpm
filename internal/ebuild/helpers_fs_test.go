package ebuild

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Directory Setting Tests
// ============================================================================

func TestHelpers_Insinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}
	if helpers.insDestTree != "/usr/share/myapp" {
		t.Errorf("expected insDestTree '/usr/share/myapp', got: %s", helpers.insDestTree)
	}
}

func TestHelpers_Insinto_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Exeinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Exeinto([]string{"/usr/libexec"})
	if err != nil {
		t.Fatalf("Exeinto failed: %v", err)
	}
	if helpers.exeDestTree != "/usr/libexec" {
		t.Errorf("expected exeDestTree '/usr/libexec', got: %s", helpers.exeDestTree)
	}
}

func TestHelpers_Docinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Docinto([]string{"examples"})
	if err != nil {
		t.Fatalf("Docinto failed: %v", err)
	}
	if helpers.docDestTree != "examples" {
		t.Errorf("expected docDestTree 'examples', got: %s", helpers.docDestTree)
	}

	// Reset with "/"
	err = helpers.Docinto([]string{"/"})
	if err != nil {
		t.Fatalf("Docinto reset failed: %v", err)
	}
	if helpers.docDestTree != "" {
		t.Errorf("expected docDestTree '', got: %s", helpers.docDestTree)
	}
}

func TestHelpers_Into(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	// Test setting DESTTREE to custom path
	err := helpers.Into([]string{"/opt/myapp"})
	if err != nil {
		t.Fatalf("Into failed: %v", err)
	}
	if helpers.destTree != "/opt/myapp" {
		t.Errorf("expected destTree '/opt/myapp', got: %s", helpers.destTree)
	}

	// Verify directory was created
	destDir := filepath.Join(helpers.env.D, "/opt/myapp")
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		t.Errorf("expected directory %s to be created", destDir)
	}
}

func TestHelpers_Into_ResetToDefault(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	// First set to custom path
	err := helpers.Into([]string{"/opt/myapp"})
	if err != nil {
		t.Fatalf("Into failed: %v", err)
	}

	// Reset to default
	err = helpers.Into([]string{"/usr"})
	if err != nil {
		t.Fatalf("Into reset failed: %v", err)
	}
	if helpers.destTree != "/usr" {
		t.Errorf("expected destTree '/usr', got: %s", helpers.destTree)
	}
}

func TestHelpers_Into_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Into([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Into_TooManyArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Into([]string{"/opt", "/usr"})
	if err == nil {
		t.Error("expected error with too many args")
	}
}

func TestHelpers_Into_EmptyString(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	// Empty string should default to /usr per PMS
	err := helpers.Into([]string{""})
	if err != nil {
		t.Fatalf("Into with empty string failed: %v", err)
	}
	if helpers.destTree != "/usr" {
		t.Errorf("expected destTree '/usr' for empty string, got: %s", helpers.destTree)
	}
}

func TestHelpers_Into_AffectsDobin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set DESTTREE to custom path
	err := helpers.Into([]string{"/opt/myapp"})
	if err != nil {
		t.Fatalf("Into failed: %v", err)
	}

	// Create a test executable
	exePath := createTestFile(t, tmpDir, "mybin", "#!/bin/sh\necho hello")

	err = helpers.Dobin([]string{exePath})
	if err != nil {
		t.Fatalf("Dobin failed: %v", err)
	}

	// Verify file was installed to /opt/myapp/bin instead of /usr/bin
	installedPath := filepath.Join(helpers.env.D, "opt", "myapp", "bin", "mybin")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Into_AffectsDosbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set DESTTREE to custom path
	err := helpers.Into([]string{"/opt/myapp"})
	if err != nil {
		t.Fatalf("Into failed: %v", err)
	}

	// Create a test executable
	exePath := createTestFile(t, tmpDir, "mysbin", "#!/bin/sh\necho daemon")

	err = helpers.Dosbin([]string{exePath})
	if err != nil {
		t.Fatalf("Dosbin failed: %v", err)
	}

	// Verify file was installed to /opt/myapp/sbin instead of /usr/sbin
	installedPath := filepath.Join(helpers.env.D, "opt", "myapp", "sbin", "mysbin")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Option Setting Tests
// ============================================================================

func TestHelpers_Insopts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insopts([]string{"-m0600"})
	if err != nil {
		t.Fatalf("Insopts failed: %v", err)
	}
	if helpers.insOpts != "-m0600" {
		t.Errorf("expected insOpts '-m0600', got: %s", helpers.insOpts)
	}
}

func TestHelpers_Exeopts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Exeopts([]string{"-m0700"})
	if err != nil {
		t.Fatalf("Exeopts failed: %v", err)
	}
	if helpers.exeOpts != "-m0700" {
		t.Errorf("expected exeOpts '-m0700', got: %s", helpers.exeOpts)
	}
}

func TestHelpers_Diropts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Diropts([]string{"-m0700"})
	if err != nil {
		t.Fatalf("Diropts failed: %v", err)
	}
	if helpers.dirOpts != "-m0700" {
		t.Errorf("expected dirOpts '-m0700', got: %s", helpers.dirOpts)
	}
}
