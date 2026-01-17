package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigLoader(t *testing.T) {
	loader := NewConfigLoader("/etc/portage")

	if loader.repos == nil {
		t.Error("repos map should be initialized")
	}
	if loader.configRoot != "/etc/portage" {
		t.Errorf("configRoot = %q, want %q", loader.configRoot, "/etc/portage")
	}
}

func TestLoad_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewConfigLoader(tmpDir)

	// Should not error if repos.conf doesn't exist
	if err := loader.Load(); err != nil {
		t.Errorf("Load() should not error for missing repos.conf: %v", err)
	}
}

func TestLoad_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repos.conf as a single file
	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `[DEFAULT]
main-repo = gentoo

[gentoo]
location = /var/db/repos/gentoo
sync-type = rsync
sync-uri = rsync://rsync.gentoo.org/gentoo-portage
priority = -1000
auto-sync = yes
`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check main-repo
	if loader.GetMainRepo() != "gentoo" {
		t.Errorf("GetMainRepo() = %q, want %q", loader.GetMainRepo(), "gentoo")
	}

	// Check main repo location
	if loader.GetMainRepoLocation() != "/var/db/repos/gentoo" {
		t.Errorf("GetMainRepoLocation() = %q, want %q", loader.GetMainRepoLocation(), "/var/db/repos/gentoo")
	}

	// Check repo config
	repo := loader.GetRepo("gentoo")
	if repo == nil {
		t.Fatal("GetRepo(gentoo) returned nil")
	}
	if repo.SyncType != "rsync" {
		t.Errorf("SyncType = %q, want %q", repo.SyncType, "rsync")
	}
	if repo.SyncURI != "rsync://rsync.gentoo.org/gentoo-portage" {
		t.Errorf("SyncURI = %q", repo.SyncURI)
	}
	if repo.Priority != -1000 {
		t.Errorf("Priority = %d, want %d", repo.Priority, -1000)
	}
	if !repo.AutoSync {
		t.Error("AutoSync should be true")
	}
}

func TestLoad_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repos.conf as a directory
	reposConfDir := filepath.Join(tmpDir, "repos.conf")
	if err := os.MkdirAll(reposConfDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create gentoo.conf
	gentooConf := filepath.Join(reposConfDir, "gentoo.conf")
	gentooContent := `[DEFAULT]
main-repo = gentoo

[gentoo]
location = /var/db/repos/gentoo
sync-type = rsync
`
	if err := os.WriteFile(gentooConf, []byte(gentooContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create local.conf
	localConf := filepath.Join(reposConfDir, "local.conf")
	localContent := `[local]
location = /var/db/repos/local
priority = 50
masters = gentoo
`
	if err := os.WriteFile(localConf, []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check main repo
	if loader.GetMainRepo() != "gentoo" {
		t.Errorf("GetMainRepo() = %q, want %q", loader.GetMainRepo(), "gentoo")
	}

	// Check both repos are loaded
	gentoo := loader.GetRepo("gentoo")
	if gentoo == nil {
		t.Error("gentoo repo not loaded")
	}

	local := loader.GetRepo("local")
	if local == nil {
		t.Fatal("local repo not loaded")
	}
	if local.Location != "/var/db/repos/local" {
		t.Errorf("local.Location = %q", local.Location)
	}
	if local.Priority != 50 {
		t.Errorf("local.Priority = %d, want 50", local.Priority)
	}
	if len(local.Masters) != 1 || local.Masters[0] != "gentoo" {
		t.Errorf("local.Masters = %v, want [gentoo]", local.Masters)
	}
}

func TestLoad_SkipsHiddenAndNonConf(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repos.conf directory
	reposConfDir := filepath.Join(tmpDir, "repos.conf")
	if err := os.MkdirAll(reposConfDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid .conf file
	validConf := filepath.Join(reposConfDir, "valid.conf")
	if err := os.WriteFile(validConf, []byte("[test]\nlocation = /test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create hidden file (should be skipped)
	hiddenConf := filepath.Join(reposConfDir, ".hidden.conf")
	if err := os.WriteFile(hiddenConf, []byte("[hidden]\nlocation = /hidden\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-.conf file (should be skipped)
	otherFile := filepath.Join(reposConfDir, "readme.txt")
	if err := os.WriteFile(otherFile, []byte("[other]\nlocation = /other\n"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Only "test" should be loaded
	if loader.GetRepo("test") == nil {
		t.Error("test repo should be loaded")
	}
	if loader.GetRepo("hidden") != nil {
		t.Error("hidden repo should NOT be loaded")
	}
	if loader.GetRepo("other") != nil {
		t.Error("other repo should NOT be loaded")
	}
}

func TestGetMainRepo_Default(t *testing.T) {
	loader := NewConfigLoader("/nonexistent")

	// Default should be "gentoo" if not configured
	if loader.GetMainRepo() != "gentoo" {
		t.Errorf("GetMainRepo() = %q, want %q", loader.GetMainRepo(), "gentoo")
	}
}

func TestGetMainRepoLocation_NotConfigured(t *testing.T) {
	loader := NewConfigLoader("/nonexistent")

	// Should return empty string if not configured
	if loader.GetMainRepoLocation() != "" {
		t.Errorf("GetMainRepoLocation() = %q, want empty", loader.GetMainRepoLocation())
	}
}

func TestGetAllRepos(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repos.conf with multiple repos
	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `[gentoo]
location = /var/db/repos/gentoo

[local]
location = /var/db/repos/local

[custom]
location = /var/db/repos/custom
`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	allRepos := loader.GetAllRepos()
	if len(allRepos) != 3 {
		t.Errorf("GetAllRepos() returned %d repos, want 3", len(allRepos))
	}

	// Verify it's a copy (modification shouldn't affect internal state)
	delete(allRepos, "gentoo")
	if loader.GetRepo("gentoo") == nil {
		t.Error("GetAllRepos() should return a copy")
	}
}

func TestDetectMainRepoLocation_ReposConf(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repos.conf
	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `[DEFAULT]
main-repo = gentoo

[gentoo]
location = /custom/repos/gentoo
`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	location := DetectMainRepoLocation(tmpDir, "")
	if location != "/custom/repos/gentoo" {
		t.Errorf("DetectMainRepoLocation() = %q, want %q", location, "/custom/repos/gentoo")
	}
}

func TestDetectMainRepoLocation_FallbackToPortdir(t *testing.T) {
	tmpDir := t.TempDir()
	// No repos.conf

	location := DetectMainRepoLocation(tmpDir, "/my/portdir")
	if location != "/my/portdir" {
		t.Errorf("DetectMainRepoLocation() = %q, want %q (PORTDIR fallback)", location, "/my/portdir")
	}
}

func TestDetectMainRepoLocation_AutoDetect(t *testing.T) {
	tmpDir := t.TempDir()
	// No repos.conf, no PORTDIR

	location := DetectMainRepoLocation(tmpDir, "")
	// Should return modern default even if path doesn't exist
	if location != "/var/db/repos/gentoo" && location != "/usr/portage" {
		t.Errorf("DetectMainRepoLocation() = %q, want /var/db/repos/gentoo or /usr/portage", location)
	}
}

func TestLoad_CommentsAndEmptyLines(t *testing.T) {
	tmpDir := t.TempDir()

	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `# This is a comment
; This is also a comment

[DEFAULT]
# Comment in section
main-repo = gentoo

[gentoo]
; Another comment
location = /var/db/repos/gentoo

# Blank lines below


`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loader.GetMainRepo() != "gentoo" {
		t.Errorf("GetMainRepo() = %q, want %q", loader.GetMainRepo(), "gentoo")
	}
}

func TestLoad_AutoSyncVariants(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"yes", true},
		{"true", true},
		{"1", true},
		{"no", false},
		{"false", false},
		{"0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			tmpDir := t.TempDir()

			reposConfPath := filepath.Join(tmpDir, "repos.conf")
			content := "[test]\nlocation = /test\nauto-sync = " + tt.value + "\n"
			if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			loader := NewConfigLoader(tmpDir)
			if err := loader.Load(); err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			repo := loader.GetRepo("test")
			if repo == nil {
				t.Fatal("test repo not loaded")
			}
			if repo.AutoSync != tt.expected {
				t.Errorf("AutoSync = %v, want %v for value %q", repo.AutoSync, tt.expected, tt.value)
			}
		})
	}
}

func TestLoad_MultipleMasters(t *testing.T) {
	tmpDir := t.TempDir()

	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `[overlay]
location = /var/db/repos/overlay
masters = gentoo local custom
`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	repo := loader.GetRepo("overlay")
	if repo == nil {
		t.Fatal("overlay repo not loaded")
	}
	if len(repo.Masters) != 3 {
		t.Errorf("Masters = %v, want 3 entries", repo.Masters)
	}
	expected := []string{"gentoo", "local", "custom"}
	for i, m := range expected {
		if i >= len(repo.Masters) || repo.Masters[i] != m {
			t.Errorf("Masters[%d] = %q, want %q", i, repo.Masters[i], m)
		}
	}
}

func TestLoad_InvalidPriority(t *testing.T) {
	tmpDir := t.TempDir()

	reposConfPath := filepath.Join(tmpDir, "repos.conf")
	content := `[test]
location = /test
priority = not_a_number
`
	if err := os.WriteFile(reposConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tmpDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	repo := loader.GetRepo("test")
	if repo == nil {
		t.Fatal("test repo not loaded")
	}
	// Invalid priority should default to 0
	if repo.Priority != 0 {
		t.Errorf("Priority = %d, want 0 for invalid value", repo.Priority)
	}
}

func TestGetRepo_NonExistent(t *testing.T) {
	loader := NewConfigLoader("/nonexistent")

	if loader.GetRepo("nonexistent") != nil {
		t.Error("GetRepo() should return nil for non-existent repo")
	}
}
