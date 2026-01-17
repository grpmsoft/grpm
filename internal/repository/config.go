// Package repository provides Portage repository configuration management.
//
// This package handles repos.conf parsing and repository location detection
// following Portage's fallback chain:
//
//  1. repos.conf -> [DEFAULT] main-repo -> [repo_name] location
//  2. PORTDIR from make.conf (legacy)
//  3. Auto-detect: /var/db/repos/gentoo or /usr/portage
//
// Example repos.conf structure:
//
//	/etc/portage/repos.conf/
//	├── gentoo.conf      # Main repo
//	├── local.conf       # Local overlay
//	└── custom.conf      # Custom repos
package repository

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RepoConfig represents a single repository configuration.
type RepoConfig struct {
	// Name is the repository name (e.g., "gentoo", "local").
	Name string

	// Location is the filesystem path to the repository.
	Location string

	// SyncType is the synchronization method (rsync, git, etc.).
	SyncType string

	// SyncURI is the URL for synchronization.
	SyncURI string

	// Priority determines repository precedence (higher = higher priority).
	// Default is 0 for overlays, -1000 for main repo.
	Priority int

	// AutoSync enables automatic synchronization.
	AutoSync bool

	// Masters lists repositories this one depends on.
	Masters []string
}

// ConfigLoader loads and manages repository configurations.
type ConfigLoader struct {
	repos      map[string]*RepoConfig
	mainRepo   string
	configRoot string
}

// NewConfigLoader creates a new ConfigLoader for the given config root.
// The configRoot is typically /etc/portage.
func NewConfigLoader(configRoot string) *ConfigLoader {
	return &ConfigLoader{
		repos:      make(map[string]*RepoConfig),
		configRoot: configRoot,
	}
}

// Load parses all repos.conf files and populates the configuration.
// Returns error if the directory cannot be read (not if it doesn't exist).
func (l *ConfigLoader) Load() error {
	reposConfPath := filepath.Join(l.configRoot, "repos.conf")

	info, err := os.Stat(reposConfPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error - repos.conf is optional
		}
		return err
	}

	if info.IsDir() {
		return l.loadDirectory(reposConfPath)
	}
	return l.loadFile(reposConfPath)
}

// loadDirectory loads all .conf files from the repos.conf directory.
func (l *ConfigLoader) loadDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Collect and sort filenames for deterministic order
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip directories, dotfiles, and non-.conf files
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		// Accept files with .conf extension or without extension
		if strings.HasSuffix(name, ".conf") || !strings.Contains(name, ".") {
			files = append(files, name)
		}
	}

	sort.Strings(files)

	for _, name := range files {
		if err := l.loadFile(filepath.Join(dirPath, name)); err != nil {
			// Log warning but continue
			continue
		}
	}

	return nil
}

// loadFile parses a single repos.conf file in INI format.
func (l *ConfigLoader) loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentSection string
	currentRepo := &RepoConfig{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header: [section_name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous section if it was a repo
			if currentSection != "" && currentSection != "DEFAULT" {
				l.repos[currentSection] = currentRepo
			}

			currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			currentRepo = &RepoConfig{Name: currentSection}
			continue
		}

		// Key-value pair: key = value
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Handle DEFAULT section specially
			if currentSection == "DEFAULT" {
				l.parseDefaultKey(key, value)
			} else if currentSection != "" {
				l.parseRepoKey(currentRepo, key, value)
			}
		}
	}

	// Save last section
	if currentSection != "" && currentSection != "DEFAULT" {
		l.repos[currentSection] = currentRepo
	}

	return scanner.Err()
}

// parseDefaultKey handles keys in the [DEFAULT] section.
func (l *ConfigLoader) parseDefaultKey(key, value string) {
	switch key {
	case "main-repo":
		l.mainRepo = value
	}
}

// parseRepoKey handles keys in a repository section.
func (l *ConfigLoader) parseRepoKey(repo *RepoConfig, key, value string) {
	switch key {
	case "location":
		repo.Location = value
	case "sync-type":
		repo.SyncType = value
	case "sync-uri":
		repo.SyncURI = value
	case "priority":
		if p, err := strconv.Atoi(value); err == nil {
			repo.Priority = p
		}
	case "auto-sync":
		repo.AutoSync = value == "yes" || value == "true" || value == "1"
	case "masters":
		repo.Masters = strings.Fields(value)
	}
}

// GetMainRepo returns the name of the main repository.
// Returns "gentoo" if not explicitly configured.
func (l *ConfigLoader) GetMainRepo() string {
	if l.mainRepo != "" {
		return l.mainRepo
	}
	return "gentoo"
}

// GetMainRepoLocation returns the filesystem path to the main repository.
// Returns empty string if the main repo is not configured in repos.conf.
func (l *ConfigLoader) GetMainRepoLocation() string {
	mainName := l.GetMainRepo()
	if repo, ok := l.repos[mainName]; ok && repo.Location != "" {
		return repo.Location
	}
	return ""
}

// GetRepo returns the configuration for a specific repository.
// Returns nil if the repository is not configured.
func (l *ConfigLoader) GetRepo(name string) *RepoConfig {
	return l.repos[name]
}

// GetAllRepos returns all configured repositories.
func (l *ConfigLoader) GetAllRepos() map[string]*RepoConfig {
	// Return a copy to prevent external modification
	result := make(map[string]*RepoConfig, len(l.repos))
	for k, v := range l.repos {
		result[k] = v
	}
	return result
}

// DetectMainRepoLocation implements Portage's fallback chain for finding
// the main repository location.
//
// Fallback order:
//  1. repos.conf main-repo location
//  2. portdir from make.conf (passed as parameter)
//  3. Auto-detect: /var/db/repos/gentoo or /usr/portage
func DetectMainRepoLocation(configRoot, portdirFromMakeConf string) string {
	// 1. Try repos.conf
	loader := NewConfigLoader(configRoot)
	if err := loader.Load(); err == nil {
		if loc := loader.GetMainRepoLocation(); loc != "" {
			return loc
		}
	}

	// 2. Try PORTDIR from make.conf
	if portdirFromMakeConf != "" {
		return portdirFromMakeConf
	}

	// 3. Auto-detect
	// Modern location first
	modernPath := "/var/db/repos/gentoo"
	if _, err := os.Stat(modernPath); err == nil {
		return modernPath
	}

	// Legacy location
	legacyPath := "/usr/portage"
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}

	// Default to modern path even if it doesn't exist
	return modernPath
}
