package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReposConfFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []*RepoConfig
		wantErr  bool
		errType  error
	}{
		{
			name: "single repository",
			content: `[gentoo]
location = /var/db/repos/gentoo
sync-type = rsync
sync-uri = rsync://rsync.gentoo.org/gentoo-portage
priority = -1000
auto-sync = yes
`,
			expected: []*RepoConfig{
				{
					Name:     "gentoo",
					Location: "/var/db/repos/gentoo",
					SyncType: "rsync",
					SyncURI:  "rsync://rsync.gentoo.org/gentoo-portage",
					Priority: -1000,
					AutoSync: true,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple repositories",
			content: `[gentoo]
location = /var/db/repos/gentoo
priority = -1000

[overlay]
location = /var/db/repos/overlay
priority = 50
auto-sync = no
masters = gentoo
`,
			expected: []*RepoConfig{
				{
					Name:     "gentoo",
					Location: "/var/db/repos/gentoo",
					Priority: -1000,
					AutoSync: true,
				},
				{
					Name:     "overlay",
					Location: "/var/db/repos/overlay",
					Priority: 50,
					AutoSync: false,
					Masters:  "gentoo",
				},
			},
			wantErr: false,
		},
		{
			name: "with comments and empty lines",
			content: `# Main Gentoo repository
[gentoo]
# Repository location
location = /var/db/repos/gentoo

# Priority for overlay resolution
priority = -1000
`,
			expected: []*RepoConfig{
				{
					Name:     "gentoo",
					Location: "/var/db/repos/gentoo",
					Priority: -1000,
					AutoSync: true,
				},
			},
			wantErr: false,
		},
		{
			name: "boolean values",
			content: `[test1]
location = /test1
auto-sync = true

[test2]
location = /test2
auto-sync = false

[test3]
location = /test3
auto-sync = 1

[test4]
location = /test4
auto-sync = 0
`,
			expected: []*RepoConfig{
				{Name: "test1", Location: "/test1", AutoSync: true},
				{Name: "test2", Location: "/test2", AutoSync: false},
				{Name: "test3", Location: "/test3", AutoSync: true},
				{Name: "test4", Location: "/test4", AutoSync: false},
			},
			wantErr: false,
		},
		{
			name:    "missing location",
			content: "[nolocation]\npriority = 10\n",
			wantErr: true,
			errType: ErrMissingLocation,
		},
		{
			name:    "empty section name",
			content: "[]\nlocation = /test\n",
			wantErr: true,
			errType: ErrInvalidSection,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: false,
		},
		{
			name:    "only comments",
			content: "# Just a comment\n# Another comment\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "repos.conf")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			configs, err := LoadReposConf(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(configs) != len(tt.expected) {
				t.Errorf("expected %d configs, got %d", len(tt.expected), len(configs))
				return
			}

			for i, cfg := range configs {
				exp := tt.expected[i]
				if cfg.Name != exp.Name {
					t.Errorf("config[%d].Name = %q, want %q", i, cfg.Name, exp.Name)
				}
				if cfg.Location != exp.Location {
					t.Errorf("config[%d].Location = %q, want %q", i, cfg.Location, exp.Location)
				}
				if cfg.Priority != exp.Priority {
					t.Errorf("config[%d].Priority = %d, want %d", i, cfg.Priority, exp.Priority)
				}
				if cfg.AutoSync != exp.AutoSync {
					t.Errorf("config[%d].AutoSync = %v, want %v", i, cfg.AutoSync, exp.AutoSync)
				}
				if cfg.SyncType != exp.SyncType {
					t.Errorf("config[%d].SyncType = %q, want %q", i, cfg.SyncType, exp.SyncType)
				}
				if cfg.SyncURI != exp.SyncURI {
					t.Errorf("config[%d].SyncURI = %q, want %q", i, cfg.SyncURI, exp.SyncURI)
				}
				if cfg.Masters != exp.Masters {
					t.Errorf("config[%d].Masters = %q, want %q", i, cfg.Masters, exp.Masters)
				}
			}
		})
	}
}

func TestLoadReposConfDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple .conf files
	file1 := `[gentoo]
location = /var/db/repos/gentoo
priority = -1000
`
	file2 := `[overlay1]
location = /var/db/repos/overlay1
priority = 50
`
	file3 := `[overlay2]
location = /var/db/repos/overlay2
priority = 100
`

	if err := os.WriteFile(filepath.Join(tmpDir, "gentoo.conf"), []byte(file1), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "overlay1.conf"), []byte(file2), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "overlay2.conf"), []byte(file3), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	// Create a non-.conf file that should be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	configs, err := LoadReposConf(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(configs) != 3 {
		t.Errorf("expected 3 configs, got %d", len(configs))
	}

	// Check that all repos were loaded
	names := make(map[string]bool)
	for _, cfg := range configs {
		names[cfg.Name] = true
	}

	for _, expected := range []string{"gentoo", "overlay1", "overlay2"} {
		if !names[expected] {
			t.Errorf("expected repository %q not found", expected)
		}
	}
}

func TestLoadReposConfDirDuplicate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with duplicate repository name
	file1 := `[duplicate]
location = /path1
`
	file2 := `[duplicate]
location = /path2
`

	if err := os.WriteFile(filepath.Join(tmpDir, "a.conf"), []byte(file1), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.conf"), []byte(file2), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := LoadReposConf(tmpDir)
	if err == nil {
		t.Error("expected error for duplicate repository, got nil")
	}
	if !errors.Is(err, ErrDuplicateRepo) {
		t.Errorf("expected ErrDuplicateRepo, got %v", err)
	}
}

func TestLoadReposConfNotExist(t *testing.T) {
	_, err := LoadReposConf("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
	if !errors.Is(err, ErrNoConfigFound) {
		t.Errorf("expected ErrNoConfigFound, got %v", err)
	}
}

func TestWriteReposConf(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "repos.conf")

	configs := []*RepoConfig{
		{
			Name:     "gentoo",
			Location: "/var/db/repos/gentoo",
			SyncType: "rsync",
			SyncURI:  "rsync://rsync.gentoo.org/gentoo-portage",
			Priority: -1000,
			AutoSync: true,
		},
		{
			Name:     "overlay",
			Location: "/var/db/repos/overlay",
			Priority: 50,
			AutoSync: false,
			Masters:  "gentoo",
		},
	}

	// Write configs
	if err := WriteReposConf(tmpFile, configs); err != nil {
		t.Fatalf("failed to write repos.conf: %v", err)
	}

	// Read back and verify
	loaded, err := LoadReposConf(tmpFile)
	if err != nil {
		t.Fatalf("failed to load written repos.conf: %v", err)
	}

	if len(loaded) != len(configs) {
		t.Errorf("expected %d configs, got %d", len(configs), len(loaded))
	}

	for i, cfg := range loaded {
		exp := configs[i]
		if cfg.Name != exp.Name {
			t.Errorf("config[%d].Name = %q, want %q", i, cfg.Name, exp.Name)
		}
		if cfg.Location != exp.Location {
			t.Errorf("config[%d].Location = %q, want %q", i, cfg.Location, exp.Location)
		}
		if cfg.Priority != exp.Priority {
			t.Errorf("config[%d].Priority = %d, want %d", i, cfg.Priority, exp.Priority)
		}
		if cfg.AutoSync != exp.AutoSync {
			t.Errorf("config[%d].AutoSync = %v, want %v", i, cfg.AutoSync, exp.AutoSync)
		}
	}
}

func TestDefaultGentooConfig(t *testing.T) {
	cfg := DefaultGentooConfig()

	if cfg.Name != "gentoo" {
		t.Errorf("Name = %q, want %q", cfg.Name, "gentoo")
	}
	if cfg.Location != "/var/db/repos/gentoo" {
		t.Errorf("Location = %q, want %q", cfg.Location, "/var/db/repos/gentoo")
	}
	if cfg.Priority != -1000 {
		t.Errorf("Priority = %d, want %d", cfg.Priority, -1000)
	}
	if !cfg.AutoSync {
		t.Error("AutoSync = false, want true")
	}
}

func TestDefaultReposConf(t *testing.T) {
	configs := DefaultReposConf()

	if len(configs) != 1 {
		t.Errorf("expected 1 default config, got %d", len(configs))
	}
	if configs[0].Name != "gentoo" {
		t.Errorf("default config name = %q, want %q", configs[0].Name, "gentoo")
	}
}

func TestParseBoolValue(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"no", false},
		{"NO", false},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBoolValue(tt.input)
			if result != tt.expected {
				t.Errorf("parseBoolValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
