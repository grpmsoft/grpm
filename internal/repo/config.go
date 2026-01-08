package repo

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RepoConfig represents a single repository configuration.
// Matches Portage repos.conf format.
type RepoConfig struct {
	Name     string // Repository name (section name in repos.conf)
	Location string // Local filesystem path
	SyncType string // Sync method: rsync, git, etc.
	SyncURI  string // Source URI for syncing
	Priority int    // Higher priority = checked first (default: 0)
	AutoSync bool   // Whether to sync automatically
	Masters  string // Master repository (for overlays)
}

// Errors for configuration parsing.
var (
	ErrNoConfigFound   = errors.New("no repos.conf configuration found")
	ErrInvalidSection  = errors.New("invalid section format")
	ErrMissingLocation = errors.New("repository location is required")
	ErrDuplicateRepo   = errors.New("duplicate repository name")
)

// DefaultGentooConfig returns the default configuration for the main Gentoo repository.
func DefaultGentooConfig() *RepoConfig {
	return &RepoConfig{
		Name:     "gentoo",
		Location: "/var/db/repos/gentoo",
		SyncType: "rsync",
		SyncURI:  "rsync://rsync.gentoo.org/gentoo-portage",
		Priority: -1000, // Portage default for main repo
		AutoSync: true,
	}
}

// DefaultReposConf returns default repository configurations.
func DefaultReposConf() []*RepoConfig {
	return []*RepoConfig{
		DefaultGentooConfig(),
	}
}

// LoadReposConf loads repository configuration from a file or directory.
// If path is a directory, it loads all .conf files in it.
// If path is a file, it loads that single file.
// Returns error if no valid configuration is found.
func LoadReposConf(path string) ([]*RepoConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("repos.conf path does not exist: %s: %w", path, ErrNoConfigFound)
		}
		return nil, fmt.Errorf("checking repos.conf path: %w", err)
	}

	if info.IsDir() {
		return loadReposConfDir(path)
	}
	return loadReposConfFile(path)
}

// loadReposConfDir loads all .conf files from a directory.
func loadReposConfDir(dirPath string) ([]*RepoConfig, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading repos.conf directory: %w", err)
	}

	var allConfigs []*RepoConfig
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only process .conf files
		if !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		configs, err := loadReposConfFile(filePath)
		if err != nil {
			// Log warning but continue with other files
			continue
		}

		// Check for duplicates
		for _, cfg := range configs {
			if seen[cfg.Name] {
				return nil, fmt.Errorf("%w: %s in %s", ErrDuplicateRepo, cfg.Name, entry.Name())
			}
			seen[cfg.Name] = true
			allConfigs = append(allConfigs, cfg)
		}
	}

	if len(allConfigs) == 0 {
		return nil, ErrNoConfigFound
	}

	return allConfigs, nil
}

// loadReposConfFile loads repository configuration from a single file.
// Format is INI-style:
//
//	[repo-name]
//	location = /path/to/repo
//	sync-type = rsync
//	sync-uri = rsync://...
//	priority = -1000
//	auto-sync = yes
func loadReposConfFile(filePath string) ([]*RepoConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening repos.conf file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var configs []*RepoConfig
	var current *RepoConfig
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header: [repo-name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous section if valid
			if current != nil {
				if err := validateRepoConfig(current); err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNum, err)
				}
				configs = append(configs, current)
			}

			// Start new section
			name := strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("line %d: %w", lineNum, ErrInvalidSection)
			}

			current = &RepoConfig{
				Name:     name,
				Priority: 0, // Default priority
				AutoSync: true,
			}
			continue
		}

		// Key-value pair: key = value
		if current == nil {
			// Key-value outside of section - skip or error
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if err := parseConfigKeyValue(current, key, value); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading repos.conf: %w", err)
	}

	// Save last section
	if current != nil {
		if err := validateRepoConfig(current); err != nil {
			return nil, err
		}
		configs = append(configs, current)
	}

	return configs, nil
}

// parseConfigKeyValue parses a key-value pair and sets the appropriate field.
func parseConfigKeyValue(cfg *RepoConfig, key, value string) error {
	switch key {
	case "location":
		cfg.Location = value
	case "sync-type":
		cfg.SyncType = value
	case "sync-uri":
		cfg.SyncURI = value
	case "priority":
		priority, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid priority value: %s", value)
		}
		cfg.Priority = priority
	case "auto-sync":
		cfg.AutoSync = parseBoolValue(value)
	case "masters":
		cfg.Masters = value
		// Ignore unknown keys for forward compatibility
	}
	return nil
}

// parseBoolValue parses Portage-style boolean values.
// Accepts: yes, true, 1 for true; no, false, 0 for false.
func parseBoolValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "yes", "true", "1":
		return true
	default:
		return false
	}
}

// validateRepoConfig validates a repository configuration.
func validateRepoConfig(cfg *RepoConfig) error {
	if cfg.Location == "" {
		return fmt.Errorf("repository %s: %w", cfg.Name, ErrMissingLocation)
	}
	return nil
}

// WriteReposConf writes repository configurations to a file.
// Useful for generating or modifying repos.conf.
func WriteReposConf(path string, configs []*RepoConfig) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating repos.conf file: %w", err)
	}
	defer func() { _ = file.Close() }()

	for i, cfg := range configs {
		if i > 0 {
			if _, err := file.WriteString("\n"); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}

		// Write section header
		if _, err := fmt.Fprintf(file, "[%s]\n", cfg.Name); err != nil {
			return fmt.Errorf("writing repos.conf: %w", err)
		}

		// Write location (required)
		if _, err := fmt.Fprintf(file, "location = %s\n", cfg.Location); err != nil {
			return fmt.Errorf("writing repos.conf: %w", err)
		}

		// Write optional fields
		if cfg.SyncType != "" {
			if _, err := fmt.Fprintf(file, "sync-type = %s\n", cfg.SyncType); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}
		if cfg.SyncURI != "" {
			if _, err := fmt.Fprintf(file, "sync-uri = %s\n", cfg.SyncURI); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}
		if cfg.Priority != 0 {
			if _, err := fmt.Fprintf(file, "priority = %d\n", cfg.Priority); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}
		if !cfg.AutoSync {
			if _, err := fmt.Fprintf(file, "auto-sync = no\n"); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}
		if cfg.Masters != "" {
			if _, err := fmt.Fprintf(file, "masters = %s\n", cfg.Masters); err != nil {
				return fmt.Errorf("writing repos.conf: %w", err)
			}
		}
	}

	return nil
}
