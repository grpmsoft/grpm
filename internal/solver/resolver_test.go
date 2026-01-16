package solver

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// TestResolveVersionedAtom tests that Resolve correctly handles versioned atoms.
// This is a regression test for Issue #30 where atoms like "=sys-devel/gcc-13.4.1"
// were passed directly to LoadPackage(), causing "no such file or directory" errors.
func TestResolveVersionedAtom(t *testing.T) {
	// Create mock repository with the exact version we're looking for
	mockRepo := repo.NewEmptyMockRepository()

	// Add package with version similar to Issue #30
	gcc := pkg.NewPackage("sys-devel/gcc", "13.4.1_p20250807", "13")
	mockRepo.Add(gcc)

	resolver := NewResolver(mockRepo)

	tests := []struct {
		name        string
		atomStr     string
		wantName    string
		wantVersion string
		wantErr     bool
	}{
		{
			// This is the exact case from Issue #30
			name:        "exact version with = operator (Issue #30)",
			atomStr:     "=sys-devel/gcc-13.4.1_p20250807",
			wantName:    "sys-devel/gcc",
			wantVersion: "13.4.1_p20250807",
			wantErr:     false,
		},
		{
			name:        "simple category/package",
			atomStr:     "sys-devel/gcc",
			wantName:    "sys-devel/gcc",
			wantVersion: "13.4.1_p20250807",
			wantErr:     false,
		},
		{
			name:        "greater-equal constraint matching",
			atomStr:     ">=sys-devel/gcc-13.0.0",
			wantName:    "sys-devel/gcc",
			wantVersion: "13.4.1_p20250807",
			wantErr:     false,
		},
		{
			name:    "greater-equal constraint NOT matching",
			atomStr: ">=sys-devel/gcc-99.0.0",
			wantErr: true,
		},
		{
			name:    "non-existent package",
			atomStr: "non-existent/package",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve([]string{tt.atomStr})

			if tt.wantErr {
				if err == nil {
					t.Errorf("Resolve(%q) expected error, got nil", tt.atomStr)
				}
				return
			}

			if err != nil {
				t.Errorf("Resolve(%q) unexpected error: %v", tt.atomStr, err)
				return
			}

			p, ok := result[tt.wantName]
			if !ok {
				t.Errorf("Resolve(%q) missing package %s in result", tt.atomStr, tt.wantName)
				return
			}

			if p.Version != tt.wantVersion {
				t.Errorf("Resolve(%q) version = %q, want %q",
					tt.atomStr, p.Version, tt.wantVersion)
			}
		})
	}
}

// TestResolveWithMockRepositoryVersionedAtom tests Resolve with versioned atoms
// using the built-in MockRepository which has hello-2.10 version.
func TestResolveWithMockRepositoryVersionedAtom(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	resolver := NewResolver(mockRepo)

	// Test resolving with exact version atom for existing hello-2.10
	result, err := resolver.Resolve([]string{"=app-misc/hello-2.10"})
	if err != nil {
		t.Fatalf("Resolve with versioned atom failed: %v", err)
	}

	// Should have hello and its dependency zlib
	if len(result) < 1 {
		t.Errorf("Expected at least 1 package in result, got %d", len(result))
	}

	if p, ok := result["app-misc/hello"]; !ok {
		t.Error("Expected app-misc/hello in result")
	} else if p.Version != "2.10" {
		t.Errorf("Expected version 2.10, got %s", p.Version)
	}
}

// TestResolveVersionedAtomPathNotTreatedAsDirectory verifies that versioned atoms
// like "=sys-devel/gcc-13.4.1_p20250807" are not passed directly to the filesystem.
// This is the core fix for Issue #30.
func TestResolveVersionedAtomPathNotTreatedAsDirectory(t *testing.T) {
	mockRepo := repo.NewEmptyMockRepository()

	// Add a package
	hello := pkg.NewPackage("app-misc/hello", "2.12.1", "0")
	mockRepo.Add(hello)

	resolver := NewResolver(mockRepo)

	// The bug in Issue #30 was that "=app-misc/hello-2.12.1" was passed
	// directly to LoadPackage, which treated "=app-misc" as a category name.
	// The fix parses the atom first, extracts "app-misc/hello" and version "2.12.1".

	result, err := resolver.Resolve([]string{"=app-misc/hello-2.12.1"})
	if err != nil {
		t.Fatalf("Resolve should not fail for valid versioned atom: %v", err)
	}

	if _, ok := result["app-misc/hello"]; !ok {
		t.Error("Expected app-misc/hello in result")
	}
}
