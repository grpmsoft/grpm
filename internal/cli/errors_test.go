package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestUserError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *UserError
		contains []string
	}{
		{
			name: "basic error",
			err: &UserError{
				Message: "Package 'foo/bar' not found.",
			},
			contains: []string{"Error:", "Package 'foo/bar' not found."},
		},
		{
			name: "error with package suggestions",
			err: &UserError{
				Message:     "Package 'neofatch' not found.",
				Suggestions: []string{"app-misc/neofetch", "sys-apps/systemd"},
			},
			contains: []string{
				"Package 'neofatch' not found.",
				"Did you mean one of these?",
				"app-misc/neofetch",
				"sys-apps/systemd",
			},
		},
		{
			name: "error with action suggestions",
			err: &UserError{
				Message:     "Package 'foo/bar' not found.",
				Suggestions: []string{"Use 'grpm search' to find packages."},
			},
			contains: []string{
				"Package 'foo/bar' not found.",
				"grpm search",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := tc.err.Error()
			for _, want := range tc.contains {
				if !strings.Contains(output, want) {
					t.Errorf("Error() output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestUserError_Unwrap(t *testing.T) {
	inner := errors.New("original error")
	err := &UserError{
		Message:   "User-friendly message",
		Technical: inner,
	}

	unwrapped := errors.Unwrap(err)
	if !errors.Is(unwrapped, inner) {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}
}

func TestWrapPackageNotFound(t *testing.T) {
	tests := []struct {
		name     string
		atom     string
		similar  []string
		wantMsg  string
		wantSugg bool
	}{
		{
			name:     "with similar packages",
			atom:     "app-misc/neofatch",
			similar:  []string{"app-misc/neofetch"},
			wantMsg:  "Package 'app-misc/neofatch' not found",
			wantSugg: true,
		},
		{
			name:     "no similar packages",
			atom:     "nonexistent/package",
			similar:  nil,
			wantMsg:  "Package 'nonexistent/package' not found",
			wantSugg: true, // Should suggest grpm search
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := WrapPackageNotFound(tc.atom, tc.similar, nil)
			var userErr *UserError
			if !errors.As(err, &userErr) {
				t.Fatal("WrapPackageNotFound should return *UserError")
			}

			if !strings.Contains(userErr.Message, tc.wantMsg) {
				t.Errorf("Message = %q, want to contain %q", userErr.Message, tc.wantMsg)
			}

			if tc.wantSugg && len(userErr.Suggestions) == 0 {
				t.Error("Expected suggestions but got none")
			}
		})
	}
}

func TestWrapVersionNotFound(t *testing.T) {
	err := WrapVersionNotFound("sys-libs/glibc", "1.0", []string{"2.38", "2.39", "2.40"}, nil)
	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatal("WrapVersionNotFound should return *UserError")
	}

	output := userErr.Error()
	if !strings.Contains(output, "sys-libs/glibc-1.0") {
		t.Error("Output should contain package and version")
	}
	if !strings.Contains(output, "Available versions:") {
		t.Error("Output should list available versions")
	}
}

func TestWrapMaskedPackage(t *testing.T) {
	err := WrapMaskedPackage("sys-devel/gcc", "stability", nil)
	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatal("WrapMaskedPackage should return *UserError")
	}

	output := userErr.Error()
	if !strings.Contains(output, "masked") {
		t.Error("Output should mention 'masked'")
	}
	if !strings.Contains(output, "sys-devel/gcc") {
		t.Error("Output should contain package name")
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		atom string
		want string
	}{
		{"sys-libs/glibc", "glibc"},
		{">=sys-libs/glibc-2.0", "glibc"},
		{"=sys-devel/gcc-13.4.1_p20250807", "gcc"},
		{"dev-lang/python", "python"},
		{"foo", "foo"},
	}

	for _, tc := range tests {
		got := extractPackageName(tc.atom)
		if got != tc.want {
			t.Errorf("extractPackageName(%q) = %q, want %q", tc.atom, got, tc.want)
		}
	}
}

func TestIsUserError(t *testing.T) {
	userErr := &UserError{Message: "test"}
	normalErr := errors.New("test")

	if !IsUserError(userErr) {
		t.Error("IsUserError should return true for *UserError")
	}
	if IsUserError(normalErr) {
		t.Error("IsUserError should return false for regular error")
	}
}
