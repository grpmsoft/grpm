package virtual

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultConfigPath is the default path for virtual provider configuration.
const DefaultConfigPath = "/etc/grpm/virtuals.conf"

// PortageVirtualsPath is the Portage-compatible path for virtual preferences.
const PortageVirtualsPath = "/etc/portage/package.provided"

// Config holds virtual package configuration.
type Config struct {
	// Defaults maps virtual packages to preferred providers.
	// Key: virtual package name (e.g., "virtual/jdk")
	// Value: preferred provider (e.g., "dev-java/openjdk")
	Defaults map[string]string
}

// NewConfig creates an empty configuration.
func NewConfig() *Config {
	return &Config{
		Defaults: make(map[string]string),
	}
}

// LoadDefaults loads provider preferences from a configuration file.
//
// The configuration file format is:
//
//	# Comment line
//	virtual/jdk dev-java/openjdk
//	virtual/editor app-editors/vim
//	virtual/mta mail-mta/postfix
//
// Lines starting with # are treated as comments.
// Empty lines are ignored.
//
// Example:
//
//	cfg, err := LoadDefaults("/etc/grpm/virtuals.conf")
func LoadDefaults(configPath string) (*Config, error) {
	cfg := NewConfig()

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file is optional - return empty config
			return cfg, nil
		}
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse: virtual/name provider/name
		fields := strings.Fields(line)
		if len(fields) < 2 {
			// Log warning but continue
			continue
		}

		virtual := fields[0]
		provider := fields[1]

		// Validate virtual package name
		if !IsVirtual(virtual) {
			// Skip non-virtual entries (could be package.provided format)
			continue
		}

		cfg.Defaults[virtual] = provider
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	return cfg, nil
}

// LoadDefaultsFromDir loads provider preferences from a directory.
//
// All files in the directory are parsed. Files are processed in
// alphabetical order, with later entries overriding earlier ones.
//
// Example:
//
//	cfg, err := LoadDefaultsFromDir("/etc/grpm/virtuals.d")
func LoadDefaultsFromDir(dirPath string) (*Config, error) {
	cfg := NewConfig()

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory is optional
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		fileCfg, err := LoadDefaults(filePath)
		if err != nil {
			// Log warning but continue with other files
			continue
		}

		// Merge entries
		for virtual, provider := range fileCfg.Defaults {
			cfg.Defaults[virtual] = provider
		}
	}

	return cfg, nil
}

// LoadAllDefaults loads provider preferences from standard locations.
//
// Checks in order:
// 1. /etc/grpm/virtuals.conf
// 2. /etc/grpm/virtuals.d/*.conf
// 3. /etc/portage/package.provided (for compatibility)
//
// Later entries override earlier ones.
func LoadAllDefaults() (*Config, error) {
	cfg := NewConfig()

	// Load from primary config file
	primaryCfg, err := LoadDefaults(DefaultConfigPath)
	if err != nil {
		return nil, err
	}
	for virtual, provider := range primaryCfg.Defaults {
		cfg.Defaults[virtual] = provider
	}

	// Load from config directory
	dirPath := strings.TrimSuffix(DefaultConfigPath, ".conf") + ".d"
	dirCfg, err := LoadDefaultsFromDir(dirPath)
	if err != nil {
		return nil, err
	}
	for virtual, provider := range dirCfg.Defaults {
		cfg.Defaults[virtual] = provider
	}

	// Load from Portage-compatible path
	portageCfg, err := LoadDefaults(PortageVirtualsPath)
	if err != nil {
		return nil, err
	}
	for virtual, provider := range portageCfg.Defaults {
		cfg.Defaults[virtual] = provider
	}

	return cfg, nil
}

// ApplyToResolver applies this configuration to a resolver.
//
// All configured defaults are set on the resolver.
func (c *Config) ApplyToResolver(r *Resolver) {
	for virtual, provider := range c.Defaults {
		r.SetDefault(virtual, provider)
	}
}

// Save writes the configuration to a file.
//
// The file format is compatible with LoadDefaults.
func (c *Config) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Write header comment
	_, err = fmt.Fprintln(file, "# GRPM Virtual Package Provider Configuration")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(file, "# Format: virtual/name provider/name")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(file, "")
	if err != nil {
		return err
	}

	// Write entries
	for virtual, provider := range c.Defaults {
		_, err := fmt.Fprintf(file, "%s %s\n", virtual, provider)
		if err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
	}

	return nil
}

// Set adds or updates a default provider.
func (c *Config) Set(virtual, provider string) {
	c.Defaults[virtual] = provider
}

// Get returns the default provider for a virtual, if configured.
func (c *Config) Get(virtual string) (string, bool) {
	provider, ok := c.Defaults[virtual]
	return provider, ok
}

// Remove removes a default provider configuration.
func (c *Config) Remove(virtual string) {
	delete(c.Defaults, virtual)
}

// Count returns the number of configured defaults.
func (c *Config) Count() int {
	return len(c.Defaults)
}
