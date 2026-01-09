package daemon

import (
	"time"
)

// Config holds daemon configuration
type Config struct {
	// Socket configuration
	SocketPath string `toml:"socket"`

	// Portage repository path
	PortageRepoPath string `toml:"portage_repo_path"`

	// Logging
	LogLevel string `toml:"log_level"`
	LogFile  string `toml:"log_file"`

	// Cache configuration
	CacheEnabled       bool   `toml:"cache_enabled"`
	CacheMaxSize       string `toml:"cache_max_size"` // e.g., "1GB"
	CacheTTL           string `toml:"cache_ttl"`      // e.g., "24h"
	CachePreloadCommon bool   `toml:"cache_preload_common"`

	// Monitoring
	MonitoringEnabled       bool   `toml:"monitoring_enabled"`
	MonitoringInterval      string `toml:"monitoring_interval"` // e.g., "1h"
	MonitoringCheckSecurity bool   `toml:"monitoring_check_security"`

	// Job queue
	QueueMaxWorkers int `toml:"queue_max_workers"`
	QueueMaxSize    int `toml:"queue_max_size"`

	// REST API
	RESTEnabled      bool   `toml:"rest_enabled"`
	RESTSocketPath   string `toml:"rest_socket"` // Unix socket path for REST API
	RESTBind         string `toml:"rest_bind"`   // TCP bind address (empty = disabled)
	RESTAuthRequired bool   `toml:"rest_auth_required"`
}

// DefaultConfig returns default daemon configuration
func DefaultConfig() *Config {
	return &Config{
		SocketPath:              "/var/run/grpm.sock",
		PortageRepoPath:         "/var/db/repos/gentoo", // Standard Gentoo repository location
		LogLevel:                "info",
		LogFile:                 "/var/log/grpm/daemon.log",
		CacheEnabled:            true,
		CacheMaxSize:            "1GB",
		CacheTTL:                "24h",
		CachePreloadCommon:      true,
		MonitoringEnabled:       true,
		MonitoringInterval:      "1h",
		MonitoringCheckSecurity: true,
		QueueMaxWorkers:         4,
		QueueMaxSize:            100,
		RESTEnabled:             true,
		RESTSocketPath:          "/var/run/grpm-rest.sock",
		RESTBind:                "", // Empty = TCP disabled, use socket only
		RESTAuthRequired:        false,
	}
}

// ParseCacheTTL converts TTL string to duration
func (c *Config) ParseCacheTTL() (time.Duration, error) {
	return time.ParseDuration(c.CacheTTL)
}

// ParseMonitoringInterval converts interval string to duration
func (c *Config) ParseMonitoringInterval() (time.Duration, error) {
	return time.ParseDuration(c.MonitoringInterval)
}
