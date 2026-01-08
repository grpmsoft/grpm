package privilege

import (
	"os/exec"
	"os/user"
	"testing"
)

func TestRequiresRoot(t *testing.T) {
	tests := []struct {
		phase string
		want  bool
	}{
		// Build phases - do not require root
		{"fetch", false},
		{"unpack", false},
		{"prepare", false},
		{"configure", false},
		{"compile", false},
		{"test", false},
		{"install", false},

		// Merge phases - require root
		{"preinst", true},
		{"postinst", true},
		{"prerm", true},
		{"postrm", true},
		{"qmerge", true},

		// Unknown phases - default to not requiring root
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			got := RequiresRoot(tt.phase)
			if got != tt.want {
				t.Errorf("RequiresRoot(%q) = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestGetPhaseInfo(t *testing.T) {
	tests := []struct {
		phase      string
		wantRoot   bool
		wantReason bool // true if reason should be non-empty
	}{
		{"fetch", false, true},
		{"unpack", false, true},
		{"prepare", false, true},
		{"configure", false, true},
		{"compile", false, true},
		{"test", false, true},
		{"install", false, true},
		{"preinst", true, true},
		{"postinst", true, true},
		{"prerm", true, true},
		{"postrm", true, true},
		{"qmerge", true, true},
		{"unknown", false, true}, // Unknown phases have a reason too
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			info := GetPhaseInfo(tt.phase)

			if info.Phase != tt.phase {
				t.Errorf("GetPhaseInfo(%q).Phase = %q, want %q", tt.phase, info.Phase, tt.phase)
			}

			if info.RequiresRoot != tt.wantRoot {
				t.Errorf("GetPhaseInfo(%q).RequiresRoot = %v, want %v", tt.phase, info.RequiresRoot, tt.wantRoot)
			}

			if tt.wantReason && info.Reason == "" {
				t.Errorf("GetPhaseInfo(%q).Reason is empty, want non-empty", tt.phase)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	// Test with all features disabled
	t.Run("disabled", func(t *testing.T) {
		features := Features{
			UserPriv:    false,
			UserFetch:   false,
			UserSandbox: false,
		}

		mgr, err := NewManager(features)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if mgr.Enabled() {
			t.Error("Manager.Enabled() = true, want false")
		}
	})

	// Test with userpriv enabled
	t.Run("userpriv", func(t *testing.T) {
		features := Features{
			UserPriv:    true,
			UserFetch:   false,
			UserSandbox: false,
		}

		mgr, err := NewManager(features)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if !mgr.Enabled() {
			t.Error("Manager.Enabled() = false, want true")
		}

		if mgr.Features().UserPriv != true {
			t.Error("Manager.Features().UserPriv = false, want true")
		}
	})

	// Test with userfetch enabled
	t.Run("userfetch", func(t *testing.T) {
		features := Features{
			UserPriv:    false,
			UserFetch:   true,
			UserSandbox: false,
		}

		mgr, err := NewManager(features)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if !mgr.Enabled() {
			t.Error("Manager.Enabled() = false, want true (userfetch enables manager)")
		}

		if mgr.ShouldDropForFetch() != true {
			t.Error("Manager.ShouldDropForFetch() = false, want true")
		}
	})

	// Test with custom UID/GID
	t.Run("custom_uid_gid", func(t *testing.T) {
		features := Features{UserPriv: true}

		mgr, err := NewManager(features, WithUID(1000), WithGID(1000))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if mgr.PortageUID() != 1000 {
			t.Errorf("Manager.PortageUID() = %d, want 1000", mgr.PortageUID())
		}

		if mgr.PortageGID() != 1000 {
			t.Errorf("Manager.PortageGID() = %d, want 1000", mgr.PortageGID())
		}
	})
}

func TestManager_ShouldDropForPhase(t *testing.T) {
	// Manager with userpriv disabled
	t.Run("disabled", func(t *testing.T) {
		mgr, _ := NewManager(Features{UserPriv: false})

		for _, phase := range []string{"fetch", "unpack", "compile", "install"} {
			if mgr.ShouldDropForPhase(phase) {
				t.Errorf("ShouldDropForPhase(%q) = true with disabled manager", phase)
			}
		}
	})

	// Manager with userpriv enabled
	t.Run("enabled", func(t *testing.T) {
		mgr, _ := NewManager(Features{UserPriv: true})

		// Build phases should drop
		buildPhases := []string{"fetch", "unpack", "prepare", "configure", "compile", "test", "install"}
		for _, phase := range buildPhases {
			if !mgr.ShouldDropForPhase(phase) {
				t.Errorf("ShouldDropForPhase(%q) = false, want true", phase)
			}
		}

		// Merge phases should not drop
		mergePhases := []string{"preinst", "postinst", "prerm", "postrm", "qmerge"}
		for _, phase := range mergePhases {
			if mgr.ShouldDropForPhase(phase) {
				t.Errorf("ShouldDropForPhase(%q) = true, want false", phase)
			}
		}
	})
}

func TestManager_ShouldDropForFetch(t *testing.T) {
	tests := []struct {
		name     string
		features Features
		want     bool
	}{
		{
			name:     "userfetch_disabled",
			features: Features{UserFetch: false},
			want:     false,
		},
		{
			name:     "userfetch_enabled",
			features: Features{UserFetch: true},
			want:     true,
		},
		{
			name:     "userpriv_only",
			features: Features{UserPriv: true, UserFetch: false},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := NewManager(tt.features)
			if got := mgr.ShouldDropForFetch(); got != tt.want {
				t.Errorf("ShouldDropForFetch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_DropPrivileges(t *testing.T) {
	mgr, err := NewManager(Features{UserPriv: true}, WithUID(1000), WithGID(1000))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Test with nil command
	// Note: On non-Linux platforms, DropPrivileges is a no-op and returns nil
	// On Linux, it should return an error for nil command
	t.Run("nil_command", func(t *testing.T) {
		err := mgr.DropPrivileges(nil)
		// Only check for error on Linux where privilege dropping is implemented
		if CanDropPrivileges() && err == nil {
			t.Error("DropPrivileges(nil) error = nil, want error on Linux")
		}
	})

	// Test with valid command
	t.Run("valid_command", func(t *testing.T) {
		cmd := exec.Command("echo", "test")
		err := mgr.DropPrivileges(cmd)
		if err != nil {
			t.Errorf("DropPrivileges() error = %v", err)
		}

		// On non-Linux, SysProcAttr might not be set
		// On Linux, verify the credential is set
		// This is a platform-specific test
	})

	// Test with disabled manager
	t.Run("disabled", func(t *testing.T) {
		disabledMgr, _ := NewManager(Features{UserPriv: false})
		cmd := exec.Command("echo", "test")
		err := disabledMgr.DropPrivileges(cmd)
		if err != nil {
			t.Errorf("DropPrivileges() with disabled manager error = %v", err)
		}
	})
}

func TestManager_DropPrivilegesForPhase(t *testing.T) {
	mgr, _ := NewManager(Features{UserPriv: true}, WithUID(1000), WithGID(1000))

	// Test build phase (should drop)
	t.Run("build_phase", func(t *testing.T) {
		cmd := exec.Command("make")
		err := mgr.DropPrivilegesForPhase(cmd, "compile")
		if err != nil {
			t.Errorf("DropPrivilegesForPhase(compile) error = %v", err)
		}
	})

	// Test merge phase (should not drop)
	t.Run("merge_phase", func(t *testing.T) {
		cmd := exec.Command("cp", "-a", ".", "/")
		err := mgr.DropPrivilegesForPhase(cmd, "postinst")
		if err != nil {
			t.Errorf("DropPrivilegesForPhase(postinst) error = %v", err)
		}
		// SysProcAttr should not be modified for merge phases
	})
}

func TestParseUID(t *testing.T) {
	tests := []struct {
		input   string
		want    uint32
		wantErr bool
	}{
		{"0", 0, false},
		{"1000", 1000, false},
		{"250", 250, false},
		{"4294967295", 4294967295, false}, // Max uint32
		{"invalid", 0, true},
		{"-1", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseUID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseGID(t *testing.T) {
	tests := []struct {
		input   string
		want    uint32
		wantErr bool
	}{
		{"0", 0, false},
		{"1000", 1000, false},
		{"250", 250, false},
		{"4294967295", 4294967295, false}, // Max uint32
		{"invalid", 0, true},
		{"-1", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseGID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseGID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify default values match Gentoo conventions
	if DefaultPortageUID != 250 {
		t.Errorf("DefaultPortageUID = %d, want 250", DefaultPortageUID)
	}

	if DefaultPortageGID != 250 {
		t.Errorf("DefaultPortageGID = %d, want 250", DefaultPortageGID)
	}

	if DefaultPortageUser != "portage" {
		t.Errorf("DefaultPortageUser = %q, want %q", DefaultPortageUser, "portage")
	}

	if DefaultPortageGroup != "portage" {
		t.Errorf("DefaultPortageGroup = %q, want %q", DefaultPortageGroup, "portage")
	}

	if DefaultPortageHome != "/var/tmp/portage" {
		t.Errorf("DefaultPortageHome = %q, want %q", DefaultPortageHome, "/var/tmp/portage")
	}
}

func TestFeatures(t *testing.T) {
	// Test Features struct initialization
	f := Features{
		UserPriv:    true,
		UserFetch:   true,
		UserSandbox: true,
	}

	if !f.UserPriv || !f.UserFetch || !f.UserSandbox {
		t.Error("Features struct fields not set correctly")
	}

	// Test zero value
	var zero Features
	if zero.UserPriv || zero.UserFetch || zero.UserSandbox {
		t.Error("Features zero value should have all fields false")
	}
}

// TestLookupUserMock tests user lookup with a mock.
func TestLookupUserMock(t *testing.T) {
	// Save original lookup function
	originalLookup := lookupUser
	defer func() { lookupUser = originalLookup }()

	t.Run("user_found", func(t *testing.T) {
		// Mock user lookup to return a fake user
		lookupUser = func(username string) (*user.User, error) {
			if username == "portage" {
				return &user.User{
					Username: "portage",
					Uid:      "250",
					Gid:      "250",
					HomeDir:  "/var/tmp/portage",
				}, nil
			}
			return nil, user.UnknownUserError(username)
		}

		mgr, err := NewManager(Features{UserPriv: true})
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if mgr.PortageUID() != 250 {
			t.Errorf("PortageUID() = %d, want 250", mgr.PortageUID())
		}

		if mgr.PortageGID() != 250 {
			t.Errorf("PortageGID() = %d, want 250", mgr.PortageGID())
		}
	})

	t.Run("user_not_found", func(t *testing.T) {
		// Mock user lookup to fail
		lookupUser = func(username string) (*user.User, error) {
			return nil, user.UnknownUserError(username)
		}

		// Should still succeed but use defaults
		mgr, err := NewManager(Features{UserPriv: true})
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		// Should fall back to defaults
		if mgr.PortageUID() != DefaultPortageUID {
			t.Errorf("PortageUID() = %d, want default %d", mgr.PortageUID(), DefaultPortageUID)
		}
	})
}

// TestManagerConcurrency tests thread safety of the Manager.
func TestManagerConcurrency(t *testing.T) {
	mgr, err := NewManager(Features{UserPriv: true})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mgr.Enabled()
			_ = mgr.Features()
			_ = mgr.PortageUID()
			_ = mgr.PortageGID()
			_ = mgr.ShouldDropForPhase("compile")
			_ = mgr.ShouldDropForFetch()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
