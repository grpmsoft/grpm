package ebuild

import (
	"errors"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/interp"
)

// ============================================================================
// Version Manipulation Tests
// ============================================================================

func TestHelpers_VerCut(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
		wantErr  bool
	}{
		{[]string{"1", "1.2.3"}, "1", false},
		{[]string{"2", "1.2.3"}, "2", false},
		{[]string{"3", "1.2.3"}, "3", false},
		{[]string{"1-2", "1.2.3"}, "1.2", false},
		{[]string{"2-3", "1.2.3"}, "2.3", false},
		{[]string{"1-3", "1.2.3"}, "1.2.3", false},
		{[]string{"2-", "1.2.3"}, "2.3", false},
		{[]string{"-2", "1.2.3"}, "1.2", false},
		{[]string{"5", "1.2.3"}, "", false}, // Out of range
		{[]string{"1"}, "", true},           // Too few args
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			helpers, _, stdout, _ := createBuildTestHelpers(t)
			stdout.Reset()

			err := helpers.VerCut(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerCut failed: %v", err)
			}

			output := stdout.String()
			if output != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestHelpers_VerRs(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
		wantErr  bool
	}{
		{[]string{"1", "-", "1.2.3"}, "1-2.3", false},
		{[]string{"1-2", "-", "1.2.3"}, "1-2-3", false},
		{[]string{"1"}, "", true}, // Too few args
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			helpers, _, stdout, _ := createBuildTestHelpers(t)
			stdout.Reset()

			err := helpers.VerRs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerRs failed: %v", err)
			}

			output := stdout.String()
			if output != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestHelpers_SplitVersion(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	tests := []struct {
		version  string
		expected []string
	}{
		{"1.2.3", []string{"1", "2", "3"}},
		{"1_2_3", []string{"1", "2", "3"}},
		{"1-2-3", []string{"1", "2", "3"}},
		{"1.2.3_rc1", []string{"1", "2", "3", "rc1"}},
		{"1", []string{"1"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := helpers.splitVersion(tt.version)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestHelpers_VerTest(t *testing.T) {
	tests := []struct {
		args      []string
		wantExit1 bool // true if comparison should be false (exit 1)
		wantErr   bool // true if DieError expected
	}{
		// Basic numeric comparisons
		{[]string{"1.0", "-lt", "2.0"}, false, false}, // true
		{[]string{"2.0", "-lt", "1.0"}, true, false},  // false
		{[]string{"1.0", "-eq", "1.0"}, false, false}, // true
		{[]string{"1.0", "-eq", "2.0"}, true, false},  // false
		{[]string{"1.0", "-ne", "2.0"}, false, false}, // true
		{[]string{"1.0", "-ne", "1.0"}, true, false},  // false
		{[]string{"2.0", "-gt", "1.0"}, false, false}, // true
		{[]string{"1.0", "-gt", "2.0"}, true, false},  // false
		{[]string{"2.0", "-ge", "1.0"}, false, false}, // true
		{[]string{"2.0", "-ge", "2.0"}, false, false}, // true
		{[]string{"1.0", "-ge", "2.0"}, true, false},  // false
		{[]string{"1.0", "-le", "2.0"}, false, false}, // true
		{[]string{"1.0", "-le", "1.0"}, false, false}, // true
		{[]string{"2.0", "-le", "1.0"}, true, false},  // false

		// Complex version comparisons (PMS-compliant)
		{[]string{"1.2.3", "-lt", "1.2.4"}, false, false},
		{[]string{"1.2.3", "-eq", "1.2.3"}, false, false},
		{[]string{"1.10", "-gt", "1.9"}, false, false},

		// Suffix comparisons: _alpha < _beta < _pre < _rc < (release) < _p
		{[]string{"1.0_alpha", "-lt", "1.0_beta"}, false, false},
		{[]string{"1.0_beta", "-lt", "1.0_pre"}, false, false},
		{[]string{"1.0_pre", "-lt", "1.0_rc"}, false, false},
		{[]string{"1.0_rc", "-lt", "1.0"}, false, false},
		{[]string{"1.0", "-lt", "1.0_p1"}, false, false},

		// Revision comparisons
		{[]string{"1.0-r1", "-lt", "1.0-r2"}, false, false},
		{[]string{"1.0", "-lt", "1.0-r1"}, false, false},

		// Error cases
		{[]string{"1.0", "-xx", "2.0"}, false, true}, // unknown operator
		{[]string{"1.0", "-lt"}, false, true},        // too few args
		{[]string{"1.0"}, false, true},               // too few args
		{[]string{}, false, true},                    // no args
	}

	for _, tt := range tests {
		name := strings.Join(tt.args, "_")
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			helpers, _, _, _ := createBuildTestHelpers(t)

			err := helpers.VerTest(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
					return
				}
				var dieErr *DieError
				if !errors.As(err, &dieErr) {
					t.Errorf("expected DieError, got: %T", err)
				}
				return
			}

			if tt.wantExit1 {
				if err == nil {
					t.Error("expected exit 1 (false result)")
					return
				}
				var exitErr interp.ExitStatus
				if !errors.As(err, &exitErr) {
					t.Errorf("expected ExitStatus, got: %T (%v)", err, err)
					return
				}
				if exitErr != 1 {
					t.Errorf("expected exit status 1, got: %d", exitErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("VerTest failed: %v", err)
			}
		})
	}
}
