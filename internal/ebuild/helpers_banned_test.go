package ebuild

import (
	"errors"
	"testing"
)

// TestIsBannedCommand tests the IsBannedCommand function.
func TestIsBannedCommand(t *testing.T) {
	tests := []struct {
		name     string
		eapi     string
		command  string
		expected bool
	}{
		// dohard - banned in EAPI 4+
		{"dohard_eapi0", "0", "dohard", false},
		{"dohard_eapi3", "3", "dohard", false},
		{"dohard_eapi4", "4", "dohard", true},
		{"dohard_eapi8", "8", "dohard", true},

		// dosed - banned in EAPI 4+
		{"dosed_eapi0", "0", "dosed", false},
		{"dosed_eapi3", "3", "dosed", false},
		{"dosed_eapi4", "4", "dosed", true},
		{"dosed_eapi7", "7", "dosed", true},

		// useq - banned in EAPI 5+
		{"useq_eapi0", "0", "useq", false},
		{"useq_eapi4", "4", "useq", false},
		{"useq_eapi5", "5", "useq", true},
		{"useq_eapi8", "8", "useq", true},

		// einstall - banned in EAPI 6+
		{"einstall_eapi0", "0", "einstall", false},
		{"einstall_eapi5", "5", "einstall", false},
		{"einstall_eapi6", "6", "einstall", true},
		{"einstall_eapi8", "8", "einstall", true},

		// dohtml - banned in EAPI 7+
		{"dohtml_eapi0", "0", "dohtml", false},
		{"dohtml_eapi6", "6", "dohtml", false},
		{"dohtml_eapi7", "7", "dohtml", true},
		{"dohtml_eapi8", "8", "dohtml", true},

		// dolib - banned in EAPI 7+
		{"dolib_eapi0", "0", "dolib", false},
		{"dolib_eapi6", "6", "dolib", false},
		{"dolib_eapi7", "7", "dolib", true},
		{"dolib_eapi8", "8", "dolib", true},

		// libopts - banned in EAPI 7+
		{"libopts_eapi0", "0", "libopts", false},
		{"libopts_eapi6", "6", "libopts", false},
		{"libopts_eapi7", "7", "libopts", true},
		{"libopts_eapi8", "8", "libopts", true},

		// hasv - banned in EAPI 8+
		{"hasv_eapi0", "0", "hasv", false},
		{"hasv_eapi7", "7", "hasv", false},
		{"hasv_eapi8", "8", "hasv", true},

		// hasq - banned in EAPI 8+
		{"hasq_eapi0", "0", "hasq", false},
		{"hasq_eapi7", "7", "hasq", false},
		{"hasq_eapi8", "8", "hasq", true},

		// Unknown EAPI - no commands banned
		{"unknown_eapi", "99", "dohard", false},

		// Non-banned commands
		{"einfo_eapi8", "8", "einfo", false},
		{"use_eapi8", "8", "use", false},
		{"dobin_eapi8", "8", "dobin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBannedCommand(tt.eapi, tt.command)
			if result != tt.expected {
				t.Errorf("IsBannedCommand(%q, %q) = %v, want %v",
					tt.eapi, tt.command, result, tt.expected)
			}
		})
	}
}

// TestGetBannedCommands tests the GetBannedCommands function.
func TestGetBannedCommands(t *testing.T) {
	tests := []struct {
		name         string
		eapi         string
		expectedSize int
		mustContain  []string
		mustNotHave  []string
	}{
		{
			name:         "eapi0_empty",
			eapi:         "0",
			expectedSize: 0,
			mustNotHave:  []string{"dohard", "dosed"},
		},
		{
			name:         "eapi4_two",
			eapi:         "4",
			expectedSize: 2,
			mustContain:  []string{"dohard", "dosed"},
			mustNotHave:  []string{"useq", "einstall"},
		},
		{
			name:         "eapi5_three",
			eapi:         "5",
			expectedSize: 3,
			mustContain:  []string{"dohard", "dosed", "useq"},
			mustNotHave:  []string{"einstall"},
		},
		{
			name:         "eapi6_four",
			eapi:         "6",
			expectedSize: 4,
			mustContain:  []string{"dohard", "dosed", "useq", "einstall"},
			mustNotHave:  []string{"dohtml"},
		},
		{
			name:         "eapi7_seven",
			eapi:         "7",
			expectedSize: 7,
			mustContain:  []string{"dohard", "dosed", "useq", "einstall", "dohtml", "dolib", "libopts"},
			mustNotHave:  []string{"hasv", "hasq"}, // hasv, hasq banned in EAPI 8+
		},
		{
			name:         "eapi8_nine",
			eapi:         "8",
			expectedSize: 9,
			mustContain:  []string{"dohard", "dosed", "useq", "einstall", "dohtml", "dolib", "libopts", "hasv", "hasq"},
		},
		{
			name:         "unknown_eapi",
			eapi:         "99",
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBannedCommands(tt.eapi)
			if len(result) != tt.expectedSize {
				t.Errorf("GetBannedCommands(%q) has %d items, want %d",
					tt.eapi, len(result), tt.expectedSize)
			}
			for _, cmd := range tt.mustContain {
				if !result[cmd] {
					t.Errorf("GetBannedCommands(%q) missing %q", tt.eapi, cmd)
				}
			}
			for _, cmd := range tt.mustNotHave {
				if result[cmd] {
					t.Errorf("GetBannedCommands(%q) should not have %q", tt.eapi, cmd)
				}
			}
		})
	}
}

// TestBannedCommandError tests the BannedCommandError type.
func TestBannedCommandError(t *testing.T) {
	err := &BannedCommandError{
		Command: "dohtml",
		EAPI:    "8",
	}

	expected := "dohtml: banned in EAPI 8"
	if err.Error() != expected {
		t.Errorf("BannedCommandError.Error() = %q, want %q", err.Error(), expected)
	}
}

// TestHelpers_CheckBannedCommand tests the CheckBannedCommand method.
// Note: Uses createTestHelpersWithEAPI from helpers_test.go
func TestHelpers_CheckBannedCommand(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		command     string
		expectError bool
	}{
		// dohard banned in EAPI 4+
		{"dohard_eapi3_allowed", "3", "dohard", false},
		{"dohard_eapi4_banned", "4", "dohard", true},
		{"dohard_eapi8_banned", "8", "dohard", true},

		// dohtml banned in EAPI 7+
		{"dohtml_eapi6_allowed", "6", "dohtml", false},
		{"dohtml_eapi7_banned", "7", "dohtml", true},
		{"dohtml_eapi8_banned", "8", "dohtml", true},

		// hasv banned in EAPI 8+
		{"hasv_eapi7_allowed", "7", "hasv", false},
		{"hasv_eapi8_banned", "8", "hasv", true},

		// Non-banned commands
		{"einfo_allowed", "8", "einfo", false},
		{"use_allowed", "8", "use", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.CheckBannedCommand(tt.command)
			if tt.expectError && err == nil {
				t.Errorf("CheckBannedCommand(%q) in EAPI %s: expected error, got nil",
					tt.command, tt.eapi)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CheckBannedCommand(%q) in EAPI %s: unexpected error: %v",
					tt.command, tt.eapi, err)
			}
			if err != nil {
				var bannedErr *BannedCommandError
				if !errors.As(err, &bannedErr) {
					t.Errorf("expected BannedCommandError, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Dohard tests the Dohard helper (banned in EAPI 4+).
func TestHelpers_Dohard(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		expectError bool
		errorType   string // "banned" or "notimpl"
	}{
		{"eapi3_notimpl", "3", true, "notimpl"},
		{"eapi4_banned", "4", true, "banned"},
		{"eapi8_banned", "8", true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Dohard([]string{"/source", "/target"})
			if err == nil {
				t.Error("expected error from Dohard")
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Dohtml tests the Dohtml helper (banned in EAPI 7+).
func TestHelpers_Dohtml(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		expectError bool
		errorType   string
	}{
		{"eapi6_notimpl", "6", true, "notimpl"},
		{"eapi7_banned", "7", true, "banned"},
		{"eapi8_banned", "8", true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Dohtml([]string{"index.html"})
			if err == nil {
				t.Error("expected error from Dohtml")
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Useq tests the Useq helper (banned in EAPI 5+).
func TestHelpers_Useq(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		args        []string
		expectError bool
		errorType   string
	}{
		// EAPI 4: useq allowed, acts as synonym for use
		{"eapi4_ssl_enabled", "4", []string{"ssl"}, false, ""},
		{"eapi4_doc_disabled", "4", []string{"doc"}, true, "exitfalse"},

		// EAPI 5+: useq banned
		{"eapi5_banned", "5", []string{"ssl"}, true, "banned"},
		{"eapi8_banned", "8", []string{"ssl"}, true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Useq(tt.args)
			if tt.expectError && err == nil {
				t.Errorf("expected error from Useq")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error from Useq: %v", err)
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Hasv tests the Hasv helper (banned in EAPI 8+).
func TestHelpers_Hasv(t *testing.T) {
	tests := []struct {
		name           string
		eapi           string
		args           []string
		expectError    bool
		errorType      string
		expectedOutput string
	}{
		// EAPI 7: hasv allowed
		{"eapi7_found", "7", []string{"foo", "bar", "foo", "baz"}, false, "", "foo"},
		{"eapi7_not_found", "7", []string{"xxx", "bar", "foo", "baz"}, true, "exitfalse", ""},

		// EAPI 8: hasv banned
		{"eapi8_banned", "8", []string{"foo", "bar", "foo", "baz"}, true, "banned", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, stdout, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Hasv(tt.args)
			if tt.expectError && err == nil {
				t.Errorf("expected error from Hasv")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error from Hasv: %v", err)
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}

			if tt.expectedOutput != "" {
				output := stdout.String()
				if output != tt.expectedOutput {
					t.Errorf("Hasv output = %q, want %q", output, tt.expectedOutput)
				}
			}
		})
	}
}

// TestHelpers_Hasq tests the Hasq helper (banned in EAPI 8+).
func TestHelpers_Hasq(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		args        []string
		expectError bool
		errorType   string
	}{
		// EAPI 7: hasq allowed (synonym for has)
		{"eapi7_found", "7", []string{"foo", "bar", "foo", "baz"}, false, ""},
		{"eapi7_not_found", "7", []string{"xxx", "bar", "foo", "baz"}, true, "exitfalse"},

		// EAPI 8: hasq banned
		{"eapi8_banned", "8", []string{"foo", "bar", "foo", "baz"}, true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Hasq(tt.args)
			if tt.expectError && err == nil {
				t.Errorf("expected error from Hasq")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error from Hasq: %v", err)
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Libopts tests the Libopts helper (banned in EAPI 7+).
func TestHelpers_Libopts(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		expectError bool
		errorType   string
	}{
		{"eapi6_notimpl", "6", true, "notimpl"},
		{"eapi7_banned", "7", true, "banned"},
		{"eapi8_banned", "8", true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Libopts([]string{"-m755"})
			if err == nil {
				t.Error("expected error from Libopts")
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Einstall tests the Einstall helper (banned in EAPI 6+).
func TestHelpers_Einstall(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		expectError bool
		errorType   string
	}{
		{"eapi5_notimpl", "5", true, "notimpl"},
		{"eapi6_banned", "6", true, "banned"},
		{"eapi8_banned", "8", true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Einstall([]string{})
			if err == nil {
				t.Error("expected error from Einstall")
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}

// TestHelpers_Dosed tests the Dosed helper (banned in EAPI 4+).
func TestHelpers_Dosed(t *testing.T) {
	tests := []struct {
		name        string
		eapi        string
		expectError bool
		errorType   string
	}{
		{"eapi3_notimpl", "3", true, "notimpl"},
		{"eapi4_banned", "4", true, "banned"},
		{"eapi8_banned", "8", true, "banned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, tt.eapi)

			err := helpers.Dosed([]string{"s/foo/bar/g", "file.txt"})
			if err == nil {
				t.Error("expected error from Dosed")
				return
			}

			if tt.errorType == "banned" {
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError for banned command, got %T", err)
				}
			}
		})
	}
}
