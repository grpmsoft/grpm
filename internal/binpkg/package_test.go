package binpkg

import (
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected BinaryFormat
	}{
		{"/var/cache/binpkgs/sys-libs/zlib-1.2.13.gpkg.tar", FormatGPKG},
		{"/tmp/packages/app-editors/vim-9.0.tbz2", FormatTBZ2},
		{"/tmp/packages/sys-apps/portage-3.0.30.tar.bz2", FormatTBZ2},
		{"/tmp/unknown.tar.gz", FormatUnknown},
		{"package.deb", FormatUnknown},
	}

	for _, tt := range tests {
		result := DetectFormat(tt.path)
		if result != tt.expected {
			t.Errorf("DetectFormat(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestBinaryFormat_String(t *testing.T) {
	tests := []struct {
		format   BinaryFormat
		expected string
	}{
		{FormatGPKG, "gpkg"},
		{FormatTBZ2, "tbz2"},
		{FormatUnknown, "unknown"},
	}

	for _, tt := range tests {
		result := tt.format.String()
		if result != tt.expected {
			t.Errorf("BinaryFormat.String() = %s, expected %s", result, tt.expected)
		}
	}
}

func TestBinaryFormat_Extension(t *testing.T) {
	tests := []struct {
		format   BinaryFormat
		expected string
	}{
		{FormatGPKG, ".gpkg.tar"},
		{FormatTBZ2, ".tbz2"},
		{FormatUnknown, ""},
	}

	for _, tt := range tests {
		result := tt.format.Extension()
		if result != tt.expected {
			t.Errorf("BinaryFormat.Extension() = %s, expected %s", result, tt.expected)
		}
	}
}

func TestBinaryPackage_IsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		buildUSE   []string
		desiredUSE []string
		expected   bool
	}{
		{
			name:       "exact match",
			buildUSE:   []string{"ssl", "python"},
			desiredUSE: []string{"ssl", "python"},
			expected:   true,
		},
		{
			name:       "subset match",
			buildUSE:   []string{"ssl", "python", "test"},
			desiredUSE: []string{"ssl", "python"},
			expected:   true,
		},
		{
			name:       "missing required flag",
			buildUSE:   []string{"python"},
			desiredUSE: []string{"ssl", "python"},
			expected:   false,
		},
		{
			name:       "negative flag present",
			buildUSE:   []string{"ssl", "debug"},
			desiredUSE: []string{"ssl", "-debug"},
			expected:   false,
		},
		{
			name:       "negative flag absent",
			buildUSE:   []string{"ssl"},
			desiredUSE: []string{"ssl", "-debug"},
			expected:   true,
		},
		{
			name:       "empty desired",
			buildUSE:   []string{"ssl", "python"},
			desiredUSE: []string{},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE: tt.buildUSE,
				},
			}

			result := bp.IsCompatible(tt.desiredUSE)
			if result != tt.expected {
				t.Errorf("IsCompatible() = %v, expected %v (build: %v, desired: %v)",
					result, tt.expected, tt.buildUSE, tt.desiredUSE)
			}
		})
	}
}

func TestBinaryPackage_IsCompatible_NilBuildInfo(t *testing.T) {
	bp := &BinaryPackage{
		BuildInfo: nil,
	}

	if bp.IsCompatible([]string{"ssl"}) {
		t.Error("IsCompatible() should return false when BuildInfo is nil")
	}
}

func TestBinaryPackage_IsFresh(t *testing.T) {
	tests := []struct {
		name      string
		buildDate time.Time
		maxAge    time.Duration
		expected  bool
	}{
		{
			name:      "fresh package",
			buildDate: time.Now().Add(-1 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  true,
		},
		{
			name:      "old package",
			buildDate: time.Now().Add(-48 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  false,
		},
		{
			name:      "exactly at limit",
			buildDate: time.Now().Add(-24 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  false, // time.Since will be slightly over 24h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				BuildInfo: &BuildMetadata{
					BuildDate: tt.buildDate,
				},
			}

			result := bp.IsFresh(tt.maxAge)
			if result != tt.expected {
				t.Errorf("IsFresh() = %v, expected %v (age: %v, max: %v)",
					result, tt.expected, time.Since(tt.buildDate), tt.maxAge)
			}
		})
	}
}

func TestBinaryPackage_IsFresh_NilBuildInfo(t *testing.T) {
	bp := &BinaryPackage{
		BuildInfo: nil,
	}

	if bp.IsFresh(24 * time.Hour) {
		t.Error("IsFresh() should return false when BuildInfo is nil")
	}
}

func TestBinaryPackage_String(t *testing.T) {
	tests := []struct {
		name     string
		bp       *BinaryPackage
		expected string
	}{
		{
			name: "valid package",
			bp: &BinaryPackage{
				Package: &pkg.Package{
					Name:    "sys-libs/zlib",
					Version: "1.2.13",
				},
				Format: FormatGPKG,
				Size:   1024000,
			},
			expected: "BinaryPackage{sys-libs/zlib-1.2.13, gpkg, 1024000 bytes}",
		},
		{
			name: "nil package",
			bp: &BinaryPackage{
				Package: nil,
			},
			expected: "BinaryPackage{unknown}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bp.String()
			if result != tt.expected {
				t.Errorf("String() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestSignatureType_String(t *testing.T) {
	tests := []struct {
		sigType  SignatureType
		expected string
	}{
		{SignatureGPG, "gpg"},
		{SignatureSSH, "ssh"},
		{SignatureNone, "none"},
	}

	for _, tt := range tests {
		result := tt.sigType.String()
		if result != tt.expected {
			t.Errorf("SignatureType.String() = %s, expected %s", result, tt.expected)
		}
	}
}

func BenchmarkIsCompatible(b *testing.B) {
	bp := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE: []string{"ssl", "python", "test", "unicode", "xml"},
		},
	}
	desiredUSE := []string{"ssl", "python", "-debug"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bp.IsCompatible(desiredUSE)
	}
}

func BenchmarkDetectFormat(b *testing.B) {
	path := "/var/cache/binpkgs/sys-libs/zlib-1.2.13.gpkg.tar"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectFormat(path)
	}
}
