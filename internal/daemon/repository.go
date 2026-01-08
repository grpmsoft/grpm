package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DaemonRepository manages daemon state persistence (PID file, status)
type DaemonRepository struct {
	pidFile string
	logPath string
}

// DaemonInfo holds daemon runtime information
type DaemonInfo struct {
	Status    DaemonState   `json:"status"`
	PID       int           `json:"pid"`
	Port      int           `json:"port"`
	StartTime time.Time     `json:"start_time"`
	Uptime    time.Duration `json:"uptime"`
	Version   string        `json:"version"`
}

// NewDaemonRepository creates a new daemon repository with platform-specific paths
func NewDaemonRepository() *DaemonRepository {
	var pidFile, logPath string

	if runtime.GOOS == "windows" {
		// Windows: Use ProgramData for system-wide daemon
		if programData := os.Getenv("PROGRAMDATA"); programData != "" {
			grpmDir := filepath.Join(programData, "grpm")
			_ = os.MkdirAll(grpmDir, 0755)
			pidFile = filepath.Join(grpmDir, "grpm.pid")
			logPath = filepath.Join(grpmDir, "grpm.log")
		} else {
			// Fallback to current directory
			pidFile = "grpm.pid"
			logPath = "grpm.log"
		}
	} else {
		// Unix: Use standard paths
		if os.Getuid() == 0 {
			// Running as root
			pidFile = "/var/run/grpm.pid"
			logPath = "/var/log/grpm.log"
		} else {
			// Running as regular user
			home := os.Getenv("HOME")
			if home != "" {
				grpmDir := filepath.Join(home, ".grpm")
				_ = os.MkdirAll(grpmDir, 0755)
				pidFile = filepath.Join(grpmDir, "grpm.pid")
				logPath = filepath.Join(grpmDir, "grpm.log")
			} else {
				pidFile = "grpm.pid"
				logPath = "grpm.log"
			}
		}
	}

	return &DaemonRepository{
		pidFile: pidFile,
		logPath: logPath,
	}
}

// GetInfo returns current daemon information
func (r *DaemonRepository) GetInfo() (*DaemonInfo, error) {
	info := &DaemonInfo{
		Status:    StateStopped,
		PID:       0,
		Port:      0,
		StartTime: time.Time{},
		Uptime:    0,
		Version:   "v0.9.0-dev",
	}

	// Load PID if file exists
	if pid, err := r.LoadPID(); err == nil {
		if r.IsProcessRunning(pid) {
			info.Status = StateRunning
			info.PID = pid

			// Try to get start time from PID file modification time
			if stat, err := os.Stat(r.pidFile); err == nil {
				info.StartTime = stat.ModTime()
				info.Uptime = time.Since(info.StartTime)
			}
		} else {
			// Process not running - PID file is stale
			info.PID = 0
		}
	}

	return info, nil
}

// SavePID saves daemon PID to file
func (r *DaemonRepository) SavePID(pid int) error {
	pidData := map[string]interface{}{
		"pid":        pid,
		"start_time": time.Now().Unix(),
		"version":    "v0.9.0-dev",
	}

	data, err := json.Marshal(pidData)
	if err != nil {
		return fmt.Errorf("failed to marshal PID data: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(r.pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	if err := os.WriteFile(r.pidFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write PID file %s: %w", r.pidFile, err)
	}

	return nil
}

// LoadPID loads daemon PID from file
func (r *DaemonRepository) LoadPID() (int, error) {
	data, err := os.ReadFile(r.pidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Try to read as JSON (new format)
	var pidData map[string]interface{}
	if err := json.Unmarshal(data, &pidData); err == nil {
		if pidFloat, ok := pidData["pid"].(float64); ok {
			return int(pidFloat), nil
		}
	}

	// Fallback: read as plain number
	pidStr := strings.TrimSpace(string(data))
	if pid, err := strconv.Atoi(pidStr); err == nil {
		return pid, nil
	}

	return 0, fmt.Errorf("invalid PID format in file")
}

// ClearPID removes PID file
func (r *DaemonRepository) ClearPID() error {
	if err := os.Remove(r.pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// GetLogPath returns absolute path to log file
func (r *DaemonRepository) GetLogPath() string {
	if filepath.IsAbs(r.logPath) {
		return r.logPath
	}

	if currentDir, err := os.Getwd(); err == nil {
		return filepath.Join(currentDir, r.logPath)
	}

	return r.logPath
}

// GetPIDFile returns path to PID file
func (r *DaemonRepository) GetPIDFile() string {
	return r.pidFile
}
