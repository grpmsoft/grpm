package privilege

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

// TestUserInfo tests the UserInfo struct.
func TestUserInfo(t *testing.T) {
	u := UserInfo{
		Name:  "portage",
		UID:   250,
		GID:   250,
		Home:  "/var/tmp/portage",
		Shell: "/bin/false",
	}

	if u.Name != "portage" {
		t.Errorf("Name = %s, want portage", u.Name)
	}

	if u.UID != 250 {
		t.Errorf("UID = %d, want 250", u.UID)
	}

	if u.GID != 250 {
		t.Errorf("GID = %d, want 250", u.GID)
	}

	if u.Home != "/var/tmp/portage" {
		t.Errorf("Home = %s, want /var/tmp/portage", u.Home)
	}

	if u.Shell != "/bin/false" {
		t.Errorf("Shell = %s, want /bin/false", u.Shell)
	}
}

// TestGroupInfo tests the GroupInfo struct.
func TestGroupInfo(t *testing.T) {
	g := GroupInfo{
		Name: "portage",
		GID:  250,
	}

	if g.Name != "portage" {
		t.Errorf("Name = %s, want portage", g.Name)
	}

	if g.GID != 250 {
		t.Errorf("GID = %d, want 250", g.GID)
	}
}

// TestIsGroupExists tests the isGroupExists function.
func TestIsGroupExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "already exists error",
			err:  &testError{msg: "group already exists"},
			want: true,
		},
		{
			name: "exit status 9",
			err:  &testError{msg: "exit status 9"},
			want: true,
		},
		{
			name: "other error",
			err:  &testError{msg: "some other error"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGroupExists(tt.err)
			if got != tt.want {
				t.Errorf("isGroupExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsUserExists tests the isUserExists function.
func TestIsUserExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "already exists error",
			err:  &testError{msg: "user already exists"},
			want: true,
		},
		{
			name: "exit status 9",
			err:  &testError{msg: "exit status 9"},
			want: true,
		},
		{
			name: "other error",
			err:  &testError{msg: "some other error"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUserExists(tt.err)
			if got != tt.want {
				t.Errorf("isUserExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestLookupGroup_InvalidFile tests LookupGroup with invalid /etc/group paths.
func TestLookupGroup_InvalidFile(t *testing.T) {
	// Note: This test only works if /etc/group exists
	// Skip on systems where it doesn't exist
	if _, err := os.Stat("/etc/group"); os.IsNotExist(err) {
		t.Skip("Skipping test: /etc/group does not exist")
	}

	// Test looking up a non-existent group
	_, err := LookupGroup("nonexistent_group_12345")
	if err == nil {
		t.Error("LookupGroup should return error for non-existent group")
	}
}

// TestLookupGroupByID_InvalidFile tests LookupGroupByID with non-existent GID.
func TestLookupGroupByID_InvalidFile(t *testing.T) {
	// Note: This test only works if /etc/group exists
	if _, err := os.Stat("/etc/group"); os.IsNotExist(err) {
		t.Skip("Skipping test: /etc/group does not exist")
	}

	// Test looking up a non-existent GID
	_, err := LookupGroupByID(99999)
	if err == nil {
		t.Error("LookupGroupByID should return error for non-existent GID")
	}
}

// TestLookupUserByID_InvalidFile tests LookupUserByID with non-existent UID.
func TestLookupUserByID_InvalidFile(t *testing.T) {
	// Note: This test only works if /etc/passwd exists
	if _, err := os.Stat("/etc/passwd"); os.IsNotExist(err) {
		t.Skip("Skipping test: /etc/passwd does not exist")
	}

	// Test looking up a non-existent UID
	_, err := LookupUserByID(99999)
	if err == nil {
		t.Error("LookupUserByID should return error for non-existent UID")
	}
}

// TestManager_SetupBuildDirectories tests SetupBuildDirectories.
func TestManager_SetupBuildDirectories(t *testing.T) {
	// Create a disabled manager (doesn't try to chown)
	mgr, err := NewManager(Features{UserPriv: false})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tmpDir := t.TempDir()
	workdir := filepath.Join(tmpDir, "work")
	destdir := filepath.Join(tmpDir, "image")
	tempdir := filepath.Join(tmpDir, "temp")
	home := filepath.Join(tmpDir, "home")

	err = mgr.SetupBuildDirectories(workdir, destdir, tempdir, home)
	if err != nil {
		t.Fatalf("SetupBuildDirectories() error = %v", err)
	}

	// Verify directories were created
	for _, dir := range []string{workdir, destdir, tempdir, home} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

// TestManager_SetupBuildDirectories_EmptyPaths tests with empty directory paths.
func TestManager_SetupBuildDirectories_EmptyPaths(t *testing.T) {
	mgr, err := NewManager(Features{UserPriv: false})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Empty paths should be skipped
	err = mgr.SetupBuildDirectories("", "", "", "")
	if err != nil {
		t.Errorf("SetupBuildDirectories() with empty paths should not error: %v", err)
	}
}

// TestManager_SetupBuildDirectories_PartialPaths tests with some empty paths.
func TestManager_SetupBuildDirectories_PartialPaths(t *testing.T) {
	mgr, err := NewManager(Features{UserPriv: false})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tmpDir := t.TempDir()
	workdir := filepath.Join(tmpDir, "work")

	// Only workdir is set, others are empty
	err = mgr.SetupBuildDirectories(workdir, "", "", "")
	if err != nil {
		t.Errorf("SetupBuildDirectories() error = %v", err)
	}

	// Verify only workdir was created
	if _, err := os.Stat(workdir); os.IsNotExist(err) {
		t.Error("workdir should have been created")
	}
}

// TestPortageUserInfo_NotExists tests PortageUserInfo when portage user doesn't exist.
func TestPortageUserInfo_NotExists(t *testing.T) {
	// Save and restore original lookupUser
	originalLookup := lookupUser
	defer func() { lookupUser = originalLookup }()

	// Mock lookupUser to fail
	lookupUser = func(username string) (*user.User, error) {
		return nil, user.UnknownUserError(username)
	}

	// PortageUserInfo should return nil when user doesn't exist
	info := PortageUserInfo()
	if info != nil {
		t.Error("PortageUserInfo() should return nil when portage user doesn't exist")
	}
}

// TestEnsurePortageUser_AlreadyExists tests EnsurePortageUser when user exists.
func TestEnsurePortageUser_AlreadyExists(t *testing.T) {
	// This test verifies the function doesn't error when user already exists
	// The actual creation would require root privileges

	// If PortageUserExists returns true, EnsurePortageUser should return nil
	if PortageUserExists() {
		err := EnsurePortageUser()
		if err != nil {
			t.Errorf("EnsurePortageUser() should return nil when user exists: %v", err)
		}
	}
}

// TestErrorTypes tests the error type variables.
func TestErrorTypes(t *testing.T) {
	// Verify error types are properly defined
	if ErrUserNotFound == nil {
		t.Error("ErrUserNotFound should not be nil")
	}

	if ErrGroupNotFound == nil {
		t.Error("ErrGroupNotFound should not be nil")
	}

	if ErrNotRoot == nil {
		t.Error("ErrNotRoot should not be nil")
	}

	if ErrPrivilegeDropFailed == nil {
		t.Error("ErrPrivilegeDropFailed should not be nil")
	}

	// Verify error messages
	if ErrUserNotFound.Error() != "privilege: portage user not found" {
		t.Errorf("ErrUserNotFound.Error() = %s, want 'privilege: portage user not found'", ErrUserNotFound.Error())
	}

	if ErrGroupNotFound.Error() != "privilege: portage group not found" {
		t.Errorf("ErrGroupNotFound.Error() = %s, want 'privilege: portage group not found'", ErrGroupNotFound.Error())
	}

	if ErrNotRoot.Error() != "privilege: root privileges required" {
		t.Errorf("ErrNotRoot.Error() = %s, want 'privilege: root privileges required'", ErrNotRoot.Error())
	}

	if ErrPrivilegeDropFailed.Error() != "privilege: failed to drop privileges" {
		t.Errorf("ErrPrivilegeDropFailed.Error() = %s, want 'privilege: failed to drop privileges'", ErrPrivilegeDropFailed.Error())
	}
}

// BenchmarkManager_SetupBuildDirectories benchmarks directory setup.
func BenchmarkManager_SetupBuildDirectories(b *testing.B) {
	mgr, _ := NewManager(Features{UserPriv: false})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		_ = mgr.SetupBuildDirectories(
			filepath.Join(tmpDir, "work"),
			filepath.Join(tmpDir, "image"),
			filepath.Join(tmpDir, "temp"),
			filepath.Join(tmpDir, "home"),
		)
	}
}

// BenchmarkIsGroupExists benchmarks isGroupExists.
func BenchmarkIsGroupExists(b *testing.B) {
	err := &testError{msg: "group already exists"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isGroupExists(err)
	}
}

// BenchmarkIsUserExists benchmarks isUserExists.
func BenchmarkIsUserExists(b *testing.B) {
	err := &testError{msg: "user already exists"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isUserExists(err)
	}
}
