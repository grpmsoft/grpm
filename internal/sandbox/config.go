package sandbox

// Config defines sandbox configuration options.
//
// The configuration controls which sandbox features are enabled and
// which filesystem paths are accessible within the sandbox.
type Config struct {
	// Enabled controls whether sandboxing is active.
	// If false, commands run without any isolation.
	Enabled bool

	// Backend specifies the sandbox implementation to use.
	// Supported values: "namespace" (Linux), "noop"
	// Default is auto-selected based on platform.
	Backend string

	// WritableDirs lists directories that can be written to.
	// Typically includes: WORKDIR, D (image dir), T (temp), HOME
	// Paths should be absolute.
	WritableDirs []string

	// ReadOnlyDirs lists directories that can only be read.
	// Typically includes: /usr, /lib, /bin, /etc, /var/db/repos
	// Paths should be absolute.
	ReadOnlyDirs []string

	// DenyNetwork blocks all network access when true.
	// Implements Portage's network-sandbox feature.
	DenyNetwork bool

	// DenyExec blocks execution of binaries outside allowed paths.
	// Paths in WritableDirs and system paths (/bin, /usr/bin) are allowed.
	DenyExec bool

	// PIDIsolation enables PID namespace isolation.
	// When enabled, processes inside sandbox cannot see external processes.
	// Implements Portage's pid-sandbox feature.
	PIDIsolation bool

	// IPCIsolation enables IPC namespace isolation.
	// When enabled, processes cannot access external IPC resources.
	// Implements Portage's ipc-sandbox feature.
	IPCIsolation bool

	// UserNamespace enables user namespace for unprivileged operation.
	// Required for running sandbox without root privileges.
	UserNamespace bool

	// LogViolations logs violations even when Enabled is false.
	// Useful for testing sandbox policies without enforcement.
	LogViolations bool
}

// DefaultConfig returns a configuration with sensible defaults for Gentoo.
//
// Default writable directories:
//   - /var/tmp/portage (build temp)
//   - /var/cache/distfiles (source tarballs)
//
// Default read-only directories:
//   - /usr, /lib, /lib64, /bin, /sbin (system binaries)
//   - /etc (configuration)
//   - /var/db/repos (portage tree)
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Backend: "namespace",
		WritableDirs: []string{
			"/var/tmp/portage",
			"/var/cache/distfiles",
		},
		ReadOnlyDirs: []string{
			"/usr",
			"/lib",
			"/lib64",
			"/bin",
			"/sbin",
			"/etc",
			"/var/db/repos",
			"/dev",
			"/proc",
			"/sys",
		},
		DenyNetwork:   true,
		DenyExec:      false, // Too restrictive for most builds
		PIDIsolation:  true,
		IPCIsolation:  false, // Can break some builds
		UserNamespace: false, // Requires specific kernel config
		LogViolations: true,
	}
}

// WithWorkdir adds common build directories as writable paths.
//
// Parameters:
//   - workdir: WORKDIR - main build directory
//   - imageDir: D - package image directory (DESTDIR)
//   - tempDir: T - temporary directory
//   - homeDir: HOME - user home directory for build caches
func (c *Config) WithWorkdir(workdir, imageDir, tempDir, homeDir string) *Config {
	c.WritableDirs = append(c.WritableDirs,
		workdir,
		imageDir,
		tempDir,
		homeDir,
	)
	return c
}

// WithDistDir adds the distfiles directory as writable.
// This is needed for fetch operations that download source tarballs.
func (c *Config) WithDistDir(distDir string) *Config {
	c.WritableDirs = append(c.WritableDirs, distDir)
	return c
}

// WithRepoPath adds the repository path as read-only.
// This allows the sandbox to read ebuild files and eclasses.
func (c *Config) WithRepoPath(repoPath string) *Config {
	c.ReadOnlyDirs = append(c.ReadOnlyDirs, repoPath)
	return c
}

// Validate checks the configuration for errors.
//
// Returns an error if:
//   - Backend is not recognized
//   - No writable directories are specified when enabled
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // Disabled sandbox doesn't need validation
	}

	switch c.Backend {
	case "", "namespace", "noop":
		// Valid backends
	default:
		return &ConfigError{
			Field:   "Backend",
			Value:   c.Backend,
			Message: "unknown backend (valid: namespace, noop)",
		}
	}

	// Warning: no writable dirs means no build output
	// This is not an error as it might be intentional for dry-run

	return nil
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return "sandbox config: " + e.Field + "=" + e.Value + ": " + e.Message
}

// PortageFeaturesToConfig converts Portage FEATURES to sandbox config.
//
// Supported features:
//   - sandbox: Enables filesystem protection
//   - network-sandbox: Blocks network access
//   - pid-sandbox: Enables PID namespace isolation
//   - ipc-sandbox: Enables IPC namespace isolation
//   - usersandbox: Enables user namespace (unprivileged)
//
// Example:
//
//	features := []string{"sandbox", "network-sandbox", "pid-sandbox"}
//	cfg := PortageFeaturesToConfig(features)
func PortageFeaturesToConfig(features []string) *Config {
	cfg := DefaultConfig()
	cfg.Enabled = false // Start disabled

	for _, feature := range features {
		switch feature {
		case "sandbox":
			cfg.Enabled = true
		case "network-sandbox":
			cfg.DenyNetwork = true
		case "pid-sandbox":
			cfg.PIDIsolation = true
		case "ipc-sandbox":
			cfg.IPCIsolation = true
		case "usersandbox":
			cfg.UserNamespace = true
		}
	}

	return cfg
}
