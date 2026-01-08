package privilege

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// UserInfo contains information about a system user.
type UserInfo struct {
	// Name is the username.
	Name string

	// UID is the user ID.
	UID uint32

	// GID is the primary group ID.
	GID uint32

	// Home is the home directory.
	Home string

	// Shell is the login shell.
	Shell string
}

// GroupInfo contains information about a system group.
type GroupInfo struct {
	// Name is the group name.
	Name string

	// GID is the group ID.
	GID uint32
}

// EnsurePortageUser creates the portage user and group if they don't exist.
//
// On Gentoo Linux, the portage user is typically created during system
// installation with UID/GID 250. This function creates the user with
// standard Gentoo settings:
//   - Username: portage
//   - UID: 250
//   - GID: 250
//   - Home: /var/tmp/portage
//   - Shell: /bin/false
//
// Returns nil if the user already exists or was created successfully.
// Returns an error if creation fails (e.g., insufficient privileges).
func EnsurePortageUser() error {
	// Check if user already exists
	if PortageUserExists() {
		return nil
	}

	// Create group first (groupadd)
	if err := createGroup(DefaultPortageGroup, int(DefaultPortageGID)); err != nil {
		// Group might already exist, try to continue
		if !isGroupExists(err) {
			return fmt.Errorf("creating portage group: %w", err)
		}
	}

	// Create user (useradd)
	if err := createUser(DefaultPortageUser, int(DefaultPortageUID), int(DefaultPortageGID), DefaultPortageHome, "/bin/false"); err != nil {
		// User might already exist
		if !isUserExists(err) {
			return fmt.Errorf("creating portage user: %w", err)
		}
	}

	return nil
}

// createGroup creates a system group.
//
// Uses groupadd command which is available on most Linux distributions.
func createGroup(name string, gid int) error {
	cmd := exec.Command("groupadd",
		"-g", strconv.Itoa(gid),
		"-r", // System group
		name,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupadd failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// createUser creates a system user.
//
// Uses useradd command which is available on most Linux distributions.
func createUser(name string, uid, gid int, home, shell string) error {
	cmd := exec.Command("useradd",
		"-u", strconv.Itoa(uid),
		"-g", strconv.Itoa(gid),
		"-d", home,
		"-s", shell,
		"-r", // System user
		"-M", // Don't create home directory
		name,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("useradd failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// isGroupExists checks if the error indicates the group already exists.
func isGroupExists(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "exit status 9") // groupadd exit code for existing group
}

// isUserExists checks if the error indicates the user already exists.
func isUserExists(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "exit status 9") // useradd exit code for existing user
}

// LookupGroup looks up a group by name from /etc/group.
//
// This is a pure Go implementation that doesn't require CGO.
func LookupGroup(name string) (*GroupInfo, error) {
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("opening /etc/group: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}

		if fields[0] == name {
			gid, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parsing GID for %s: %w", name, err)
			}

			return &GroupInfo{
				Name: fields[0],
				GID:  uint32(gid),
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading /etc/group: %w", err)
	}

	return nil, fmt.Errorf("%w: %s", ErrGroupNotFound, name)
}

// LookupGroupByID looks up a group by GID from /etc/group.
func LookupGroupByID(gid uint32) (*GroupInfo, error) {
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("opening /etc/group: %w", err)
	}
	defer func() { _ = file.Close() }()

	gidStr := strconv.FormatUint(uint64(gid), 10)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}

		if fields[2] == gidStr {
			return &GroupInfo{
				Name: fields[0],
				GID:  gid,
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading /etc/group: %w", err)
	}

	return nil, fmt.Errorf("%w: %d", ErrGroupNotFound, gid)
}

// LookupUserByID looks up a user by UID from /etc/passwd.
func LookupUserByID(uid uint32) (*UserInfo, error) {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("opening /etc/passwd: %w", err)
	}
	defer func() { _ = file.Close() }()

	uidStr := strconv.FormatUint(uint64(uid), 10)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}

		if fields[2] == uidStr {
			gid, err := strconv.ParseUint(fields[3], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parsing GID for UID %d: %w", uid, err)
			}

			return &UserInfo{
				Name:  fields[0],
				UID:   uid,
				GID:   uint32(gid),
				Home:  fields[5],
				Shell: fields[6],
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading /etc/passwd: %w", err)
	}

	return nil, fmt.Errorf("%w: %d", ErrUserNotFound, uid)
}

// SetupBuildDirectories creates and sets ownership for build directories.
//
// When userpriv is enabled, the portage user needs write access to:
//   - WORKDIR: Build working directory
//   - D (DESTDIR): Installation image directory
//   - T: Temporary directory
//   - HOME: Home directory for build caches
//
// This function creates these directories and sets them to be owned
// by the portage user.
func (m *Manager) SetupBuildDirectories(workdir, destdir, tempdir, home string) error {
	dirs := []string{workdir, destdir, tempdir, home}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		// Create directory with permissive mode
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}

		// Set ownership if userpriv is enabled
		if m.enabled {
			if err := m.SetOwnership(dir); err != nil {
				return fmt.Errorf("setting ownership of %s: %w", dir, err)
			}
		}
	}

	return nil
}

// PortageUserInfo returns information about the portage user.
//
// Returns nil if the user doesn't exist.
func PortageUserInfo() *UserInfo {
	u, err := lookupUser(DefaultPortageUser)
	if err != nil {
		return nil
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil
	}

	return &UserInfo{
		Name:  u.Username,
		UID:   uint32(uid),
		GID:   uint32(gid),
		Home:  u.HomeDir,
		Shell: "",
	}
}
