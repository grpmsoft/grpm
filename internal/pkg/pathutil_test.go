package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsValidCategory tests category name validation per PMS.
func TestIsValidCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
		want     bool
	}{
		// Valid categories
		{"simple category", "sys-libs", true},
		{"numeric start", "x11-libs", true},
		{"with plus", "media+libs", true},
		{"with underscore", "sys_libs", true},
		{"with dot", "sys.libs", true},
		{"single char", "a", true},
		{"all numeric", "123", true},

		// Invalid categories - path traversal attempts
		{"empty", "", false},
		{"dot dot", "..", false},
		{"dot", ".", false},
		{"starts with dot", ".hidden", false},
		{"starts with dash", "-category", false},
		{"starts with underscore", "_category", false},
		{"contains slash", "sys/libs", false},
		{"contains backslash", "sys\\libs", false},
		{"path traversal simple", "../etc", false},
		{"path traversal complex", "sys-libs/../../../etc", false},
		{"null byte", "sys\x00libs", false},
		{"space", "sys libs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidCategory(tt.category); got != tt.want {
				t.Errorf("IsValidCategory(%q) = %v, want %v", tt.category, got, tt.want)
			}
		})
	}
}

// TestIsValidPackageName tests package name validation per PMS.
func TestIsValidPackageName(t *testing.T) {
	tests := []struct {
		name    string
		pkgName string
		want    bool
	}{
		// Valid package names
		{"simple name", "zlib", true},
		{"with version suffix", "python3", true},
		{"with dash", "package-name", true},
		{"with underscore", "package_name", true},
		{"with plus", "package+extra", true},
		{"single char", "a", true},
		{"numeric start", "123tool", true},

		// Invalid package names - note: '.' is NOT allowed in package names!
		{"empty", "", false},
		{"dot dot", "..", false},
		{"dot", ".", false},
		{"starts with dot", ".hidden", false},
		{"starts with dash", "-package", false},
		{"starts with underscore", "_package", false},
		{"contains dot", "package.name", false}, // '.' forbidden per PMS
		{"contains slash", "pkg/name", false},
		{"contains backslash", "pkg\\name", false},
		{"path traversal simple", "../etc/passwd", false},
		{"path traversal relative", "../../etc", false},
		{"null byte", "pkg\x00name", false},
		{"space", "pkg name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidPackageName(tt.pkgName); got != tt.want {
				t.Errorf("IsValidPackageName(%q) = %v, want %v", tt.pkgName, got, tt.want)
			}
		})
	}
}

// TestValidateCategoryPackageName tests combined category/package validation.
func TestValidateCategoryPackageName(t *testing.T) {
	tests := []struct {
		name     string
		category string
		pkgName  string
		wantErr  bool
	}{
		// Valid combinations
		{"valid simple", "sys-libs", "zlib", false},
		{"valid complex", "dev-lang", "python", false},
		{"valid x11", "x11-libs", "gtk+", false},

		// Invalid category
		{"invalid category dotdot", "..", "zlib", true},
		{"invalid category hidden", ".hidden", "zlib", true},
		{"invalid category slash", "sys/libs", "zlib", true},

		// Invalid package name
		{"invalid package dotdot", "sys-libs", "..", true},
		{"invalid package hidden", "sys-libs", ".hidden", true},
		{"invalid package dot", "sys-libs", "pkg.name", true}, // '.' forbidden

		// Path traversal attempts
		{"path traversal category", "../../../etc", "passwd", true},
		{"path traversal package", "sys-libs", "../../etc", true},
		{"path traversal both", "../etc", "../passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCategoryPackageName(tt.category, tt.pkgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCategoryPackageName(%q, %q) error = %v, wantErr %v",
					tt.category, tt.pkgName, err, tt.wantErr)
			}
		})
	}
}

// TestValidatePathContainment tests the defense-in-depth path containment check.
func TestValidatePathContainment(t *testing.T) {
	// Use OS-agnostic path separator for tests
	sep := string(os.PathSeparator)

	tests := []struct {
		name         string
		basePath     string
		resolvedPath string
		wantErr      bool
	}{
		// Valid paths (contained within base)
		{"simple child", "/var/db/pkg", "/var/db/pkg/sys-libs/zlib", false},
		{"nested child", "/var/db/pkg", "/var/db/pkg/sys-libs/zlib/2.0", false},
		{"exact match", "/var/db/pkg", "/var/db/pkg", false},

		// Invalid paths (escape base directory)
		{"parent escape", "/var/db/pkg", "/var/db", true},
		{"sibling escape", "/var/db/pkg", "/var/db/pkgother", true},
		{"root escape", "/var/db/pkg", "/etc/passwd", true},
		{"dotdot escape", "/var/db/pkg", "/var/db/pkg/../etc", true},
		{"complex escape", "/var/db/pkg", "/var/db/pkg/sys-libs/../../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert paths for current OS
			basePath := filepath.FromSlash(tt.basePath)
			resolvedPath := filepath.FromSlash(tt.resolvedPath)

			err := ValidatePathContainment(basePath, resolvedPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathContainment(%q, %q) error = %v, wantErr %v",
					basePath, resolvedPath, err, tt.wantErr)
			}
		})
	}

	// Test with relative paths
	t.Run("relative paths", func(t *testing.T) {
		base := "repo"
		contained := "repo" + sep + "category" + sep + "package"
		escaped := "repo" + sep + ".." + sep + "etc"

		if err := ValidatePathContainment(base, contained); err != nil {
			t.Errorf("ValidatePathContainment with relative contained path should pass: %v", err)
		}

		if err := ValidatePathContainment(base, escaped); err == nil {
			t.Error("ValidatePathContainment with escaped relative path should fail")
		}
	})
}

// TestSafeJoinPath tests the safe path joining utility.
func TestSafeJoinPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		elems    []string
		wantErr  bool
	}{
		// Valid paths
		{"simple join", "/var/db/pkg", []string{"sys-libs", "zlib"}, false},
		{"single element", "/var/db/pkg", []string{"sys-libs"}, false},

		// Invalid paths - path traversal
		{"dotdot element", "/var/db/pkg", []string{"..", "etc"}, true},
		{"dotdot in middle", "/var/db/pkg", []string{"sys-libs", "..", "..", "etc"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basePath := filepath.FromSlash(tt.basePath)
			result, err := SafeJoinPath(basePath, tt.elems...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeJoinPath(%q, %v) error = %v, wantErr %v",
					basePath, tt.elems, err, tt.wantErr)
			}
			if err == nil && result == "" {
				t.Error("SafeJoinPath should return non-empty path on success")
			}
		})
	}
}
