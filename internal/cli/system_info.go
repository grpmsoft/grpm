package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
)

// SystemInfo contains system environment information for --info display.
type SystemInfo struct {
	// GRPMVersion is the current GRPM version
	GRPMVersion string

	// GoVersion is the Go runtime version
	GoVersion string

	// Platform is the OS/architecture combination
	Platform string

	// Uname is the system uname information
	Uname string

	// Memory contains memory statistics (Linux only)
	Memory MemoryInfo

	// Repositories contains repository information
	Repositories []RepoInfo

	// InstalledPkgs contains key installed package versions
	InstalledPkgs []InstalledPkg

	// Profile is the active profile path
	Profile string

	// ConfigVars contains configuration variables from make.conf
	ConfigVars map[string]string
}

// MemoryInfo contains memory statistics.
type MemoryInfo struct {
	// Total memory in bytes
	Total int64

	// Free memory in bytes
	Free int64

	// SwapTotal in bytes
	SwapTotal int64

	// SwapFree in bytes
	SwapFree int64
}

// RepoInfo contains repository information.
type RepoInfo struct {
	// Name is the repository name (e.g., "gentoo")
	Name string

	// Location is the repository path
	Location string

	// SyncType is the sync method (rsync, git, etc.)
	SyncType string

	// Timestamp is the last sync timestamp
	Timestamp string
}

// InstalledPkg contains installed package information.
type InstalledPkg struct {
	// CP is the category/package name
	CP string

	// Version is the installed version
	Version string
}

// GatherSystemInfo collects system information for --info display.
//
// Gathers:
//   - GRPM version and Go version
//   - System uname and memory info
//   - Key installed packages (gcc, glibc, binutils, python)
//   - Repository information
//   - Configuration variables (CFLAGS, USE, etc.)
func GatherSystemInfo(grpmVersion string, cfg *config.Config, repoPath string) *SystemInfo {
	info := &SystemInfo{
		GRPMVersion: grpmVersion,
		GoVersion:   runtime.Version(),
		Platform:    runtime.GOOS + "-" + runtime.GOARCH,
		ConfigVars:  make(map[string]string),
	}

	// Get uname-like info
	info.Uname = gatherUname()

	// Memory from /proc/meminfo (Linux only)
	if runtime.GOOS == "linux" {
		info.Memory = readMemoryInfo()
	}

	// Key installed packages
	info.InstalledPkgs = readKeyPackages()

	// Repository info
	info.Repositories = gatherRepoInfo(repoPath, cfg)

	// Profile path
	info.Profile = findActiveProfile()

	// Configuration variables
	if cfg != nil && cfg.MakeConf != nil {
		info.ConfigVars["CFLAGS"] = cfg.MakeConf.CFLAGS
		info.ConfigVars["CXXFLAGS"] = cfg.MakeConf.CXXFLAGS
		if cfg.MakeConf.LDFLAGS != "" {
			info.ConfigVars["LDFLAGS"] = cfg.MakeConf.LDFLAGS
		}
		info.ConfigVars["USE"] = strings.Join(cfg.MakeConf.USE, " ")
		if len(cfg.MakeConf.ACCEPT_KEYWORDS) > 0 {
			info.ConfigVars["ACCEPT_KEYWORDS"] = strings.Join(cfg.MakeConf.ACCEPT_KEYWORDS, " ")
		}
		if len(cfg.MakeConf.ACCEPT_LICENSE) > 0 {
			info.ConfigVars["ACCEPT_LICENSE"] = strings.Join(cfg.MakeConf.ACCEPT_LICENSE, " ")
		}
		if len(cfg.MakeConf.FEATURES) > 0 {
			info.ConfigVars["FEATURES"] = strings.Join(cfg.MakeConf.FEATURES, " ")
		}
		if cfg.MakeConf.MAKEOPTS != "" {
			info.ConfigVars["MAKEOPTS"] = cfg.MakeConf.MAKEOPTS
		}
	}

	return info
}

// FormatSystemInfo formats SystemInfo for display.
//
// Output format matches Portage's emerge --info:
//
//	GRPM 0.8.2 (go go1.25.0, linux-amd64)
//	=================================================================
//	System uname: Linux-6.6.87-x86_64
//	KiB Mem:   8133872 total,   6836952 free
//	...
func FormatSystemInfo(info *SystemInfo) string {
	var sb strings.Builder

	// Header line (like Portage)
	sb.WriteString(fmt.Sprintf("GRPM %s (%s, %s)\n",
		info.GRPMVersion, info.GoVersion, info.Platform))
	sb.WriteString(strings.Repeat("=", 65) + "\n")

	// System info
	if info.Uname != "" {
		sb.WriteString(fmt.Sprintf("System uname: %s\n", info.Uname))
	}

	// Memory info (Linux only)
	if info.Memory.Total > 0 {
		sb.WriteString(fmt.Sprintf("KiB Mem:  %10d total", info.Memory.Total/1024))
		if info.Memory.Free > 0 {
			sb.WriteString(fmt.Sprintf(",%10d free", info.Memory.Free/1024))
		}
		sb.WriteString("\n")
	}
	if info.Memory.SwapTotal > 0 {
		sb.WriteString(fmt.Sprintf("KiB Swap: %10d total", info.Memory.SwapTotal/1024))
		if info.Memory.SwapFree > 0 {
			sb.WriteString(fmt.Sprintf(",%10d free", info.Memory.SwapFree/1024))
		}
		sb.WriteString("\n")
	}

	// Key packages (like Portage info_pkgs)
	if len(info.InstalledPkgs) > 0 {
		sb.WriteString("\n")

		// Find max CP length for alignment
		maxLen := 0
		for _, pkg := range info.InstalledPkgs {
			if len(pkg.CP) > maxLen {
				maxLen = len(pkg.CP)
			}
		}

		for _, pkg := range info.InstalledPkgs {
			if pkg.Version != "" {
				sb.WriteString(fmt.Sprintf("%-*s %s\n", maxLen+1, pkg.CP+":", pkg.Version))
			} else {
				sb.WriteString(fmt.Sprintf("%-*s [Not installed]\n", maxLen+1, pkg.CP+":"))
			}
		}
	}

	// Repository info
	if len(info.Repositories) > 0 {
		sb.WriteString("\nRepositories:\n")
		for _, r := range info.Repositories {
			sb.WriteString(fmt.Sprintf("    %s\n", r.Name))
			sb.WriteString(fmt.Sprintf("        location: %s\n", r.Location))
			if r.SyncType != "" {
				sb.WriteString(fmt.Sprintf("        sync-type: %s\n", r.SyncType))
			}
			if r.Timestamp != "" {
				sb.WriteString(fmt.Sprintf("        timestamp: %s\n", r.Timestamp))
			}
		}
	}

	// Profile
	if info.Profile != "" {
		sb.WriteString(fmt.Sprintf("\nProfile: %s\n", info.Profile))
	}

	// Configuration variables
	sb.WriteString("\n")
	varOrder := []string{"USE", "CFLAGS", "CXXFLAGS", "LDFLAGS", "MAKEOPTS", "ACCEPT_KEYWORDS", "ACCEPT_LICENSE", "FEATURES"}
	for _, key := range varOrder {
		if val, ok := info.ConfigVars[key]; ok && val != "" {
			sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", key, val))
		}
	}

	return sb.String()
}

// gatherUname returns uname-like system information.
func gatherUname() string {
	// On Linux, read /proc/sys/kernel/ostype and /proc/sys/kernel/osrelease
	if runtime.GOOS == "linux" {
		ostype, err := os.ReadFile("/proc/sys/kernel/ostype")
		if err != nil {
			return runtime.GOOS + "-" + runtime.GOARCH
		}
		osrelease, err := os.ReadFile("/proc/sys/kernel/osrelease")
		if err != nil {
			return runtime.GOOS + "-" + runtime.GOARCH
		}
		return strings.TrimSpace(string(ostype)) + "-" + strings.TrimSpace(string(osrelease)) + "-" + runtime.GOARCH
	}

	return runtime.GOOS + "-" + runtime.GOARCH
}

// readMemoryInfo reads memory information from /proc/meminfo.
func readMemoryInfo() MemoryInfo {
	info := MemoryInfo{}

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return info
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSuffix(parts[0], ":")
		value := parseInt64(parts[1])

		// Values in /proc/meminfo are in kB
		switch key {
		case "MemTotal":
			info.Total = value * 1024
		case "MemFree":
			info.Free = value * 1024
		case "SwapTotal":
			info.SwapTotal = value * 1024
		case "SwapFree":
			info.SwapFree = value * 1024
		}
	}

	return info
}

// parseInt64 parses a string as int64, returning 0 on error.
func parseInt64(s string) int64 {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result
}

// readKeyPackages reads key installed package versions from VarDB.
func readKeyPackages() []InstalledPkg {
	// Key packages to check (same as Portage info_pkgs)
	keyPkgs := []string{
		"dev-build/autoconf",
		"dev-build/automake",
		"sys-devel/binutils",
		"dev-build/libtool",
		"dev-lang/python",
		"sys-devel/gcc",
		"sys-libs/glibc",
	}

	result := make([]InstalledPkg, 0, len(keyPkgs))
	vardbPath := "/var/db/pkg"

	for _, cp := range keyPkgs {
		version := findInstalledVersion(vardbPath, cp)
		result = append(result, InstalledPkg{
			CP:      cp,
			Version: version,
		})
	}

	return result
}

// findInstalledVersion finds the installed version of a package.
//
// VarDB structure is /var/db/pkg/<category>/<package>-<version>/
// Example: /var/db/pkg/sys-devel/gcc-13.4.1/
func findInstalledVersion(vardbPath, cp string) string {
	// Parse cp into category and package name
	parts := strings.SplitN(cp, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	category := parts[0]
	pkgName := parts[1]

	// Read the category directory
	categoryDir := filepath.Join(vardbPath, category)
	entries, err := os.ReadDir(categoryDir)
	if err != nil {
		return ""
	}

	// Find packages matching the name
	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check if directory starts with package name followed by hyphen and digit
		if strings.HasPrefix(name, pkgName+"-") {
			suffix := strings.TrimPrefix(name, pkgName+"-")
			if len(suffix) > 0 && suffix[0] >= '0' && suffix[0] <= '9' {
				versions = append(versions, suffix)
			}
		}
	}

	if len(versions) == 0 {
		return ""
	}

	// Return the last (newest) version
	// Note: For proper version sorting, we'd need to use pkg.CompareVersions
	return versions[len(versions)-1]
}

// gatherRepoInfo gathers repository information.
func gatherRepoInfo(repoPath string, cfg *config.Config) []RepoInfo {
	repos := []RepoInfo{}

	// Check if repoPath exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return repos
	}

	// Get repository name from metadata/layout.conf or directory name
	name := filepath.Base(repoPath)
	layoutConf := filepath.Join(repoPath, "metadata", "layout.conf")
	if data, err := os.ReadFile(layoutConf); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "repo-name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}

	info := RepoInfo{
		Name:     name,
		Location: repoPath,
	}

	// Check sync type
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		info.SyncType = "git"
	} else {
		info.SyncType = "rsync"
	}

	// Get timestamp
	timestampFile := filepath.Join(repoPath, "metadata", "timestamp.chk")
	if data, err := os.ReadFile(timestampFile); err == nil {
		info.Timestamp = strings.TrimSpace(string(data))
	}

	repos = append(repos, info)

	return repos
}

// findActiveProfile finds the active profile path.
func findActiveProfile() string {
	profileLink := "/etc/portage/make.profile"

	// Read symlink target
	target, err := os.Readlink(profileLink)
	if err != nil {
		return ""
	}

	// If relative path, make it absolute
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(profileLink), target)
	}

	// Simplify path like "default/linux/amd64/23.0"
	if strings.Contains(target, "/profiles/") {
		parts := strings.Split(target, "/profiles/")
		if len(parts) == 2 {
			return parts[1]
		}
	}

	return target
}
