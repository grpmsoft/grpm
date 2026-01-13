package fetch

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseThirdPartyMirrors(t *testing.T) {
	// Create temp directory with thirdpartymirrors file
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("creating profiles dir: %v", err)
	}

	content := `# Third party mirrors
gnu https://ftp.gnu.org/gnu/ https://mirrors.kernel.org/gnu/
sourceforge https://downloads.sourceforge.net/
apache https://archive.apache.org/dist/ https://www.apache.org/dist/

# Comment line
pypi https://files.pythonhosted.org/packages/source

`
	if err := os.WriteFile(filepath.Join(profilesDir, "thirdpartymirrors"), []byte(content), 0644); err != nil {
		t.Fatalf("writing thirdpartymirrors: %v", err)
	}

	mirrors := ParseThirdPartyMirrors(tmpDir)

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name:     "gnu",
			expected: []string{"https://ftp.gnu.org/gnu/", "https://mirrors.kernel.org/gnu/"},
		},
		{
			name:     "sourceforge",
			expected: []string{"https://downloads.sourceforge.net/"},
		},
		{
			name:     "apache",
			expected: []string{"https://archive.apache.org/dist/", "https://www.apache.org/dist/"},
		},
		{
			name:     "pypi",
			expected: []string{"https://files.pythonhosted.org/packages/source/"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			urls, ok := mirrors[tc.name]
			if !ok {
				t.Fatalf("mirror %q not found", tc.name)
			}
			if !reflect.DeepEqual(urls, tc.expected) {
				t.Errorf("got %v, want %v", urls, tc.expected)
			}
		})
	}
}

func TestParseThirdPartyMirrors_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	mirrors := ParseThirdPartyMirrors(tmpDir)

	if len(mirrors) != 0 {
		t.Errorf("expected empty map for missing file, got %v", mirrors)
	}
}

func TestThirdPartyMirrors_ExpandMirrorURL(t *testing.T) {
	mirrors := ThirdPartyMirrors{
		"gnu": {
			"https://ftp.gnu.org/gnu/",
			"https://mirrors.kernel.org/gnu/",
		},
		"sourceforge": {
			"https://downloads.sourceforge.net/",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "gnu mirror expansion",
			input: "mirror://gnu/hello/hello-2.12.tar.gz",
			expected: []string{
				"https://ftp.gnu.org/gnu/hello/hello-2.12.tar.gz",
				"https://mirrors.kernel.org/gnu/hello/hello-2.12.tar.gz",
			},
		},
		{
			name:  "sourceforge mirror expansion",
			input: "mirror://sourceforge/project/file.tar.gz",
			expected: []string{
				"https://downloads.sourceforge.net/project/file.tar.gz",
			},
		},
		{
			name:     "non-mirror URL passthrough",
			input:    "https://example.com/file.tar.gz",
			expected: []string{"https://example.com/file.tar.gz"},
		},
		{
			name:     "ftp URL passthrough",
			input:    "ftp://ftp.example.com/file.tar.gz",
			expected: []string{"ftp://ftp.example.com/file.tar.gz"},
		},
		{
			name:     "unknown mirror returns original",
			input:    "mirror://unknown/path/file.tar.gz",
			expected: []string{"mirror://unknown/path/file.tar.gz"},
		},
		{
			name:     "malformed mirror URL",
			input:    "mirror://gnu",
			expected: []string{"mirror://gnu"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mirrors.ExpandMirrorURL(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("ExpandMirrorURL(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestThirdPartyMirrors_ExpandURIs(t *testing.T) {
	mirrors := ThirdPartyMirrors{
		"gnu": {
			"https://ftp.gnu.org/gnu/",
			"https://mirrors.kernel.org/gnu/",
		},
	}

	input := []string{
		"mirror://gnu/hello/hello-2.12.tar.gz",
		"https://example.com/extra.tar.gz",
	}

	expected := []string{
		"https://ftp.gnu.org/gnu/hello/hello-2.12.tar.gz",
		"https://mirrors.kernel.org/gnu/hello/hello-2.12.tar.gz",
		"https://example.com/extra.tar.gz",
	}

	result := mirrors.ExpandURIs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("ExpandURIs() = %v, want %v", result, expected)
	}
}

func TestThirdPartyMirrors_KnownMirrors(t *testing.T) {
	mirrors := ThirdPartyMirrors{
		"gnu":         {"https://ftp.gnu.org/gnu/"},
		"sourceforge": {"https://downloads.sourceforge.net/"},
		"apache":      {"https://archive.apache.org/dist/"},
	}

	known := mirrors.KnownMirrors()
	sort.Strings(known)

	expected := []string{"apache", "gnu", "sourceforge"}
	if !reflect.DeepEqual(known, expected) {
		t.Errorf("KnownMirrors() = %v, want %v", known, expected)
	}
}

func TestThirdPartyMirrors_TrailingSlash(t *testing.T) {
	// Create temp directory with thirdpartymirrors without trailing slashes
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("creating profiles dir: %v", err)
	}

	// URLs without trailing slashes should be normalized
	content := `gnu https://ftp.gnu.org/gnu https://mirrors.kernel.org/gnu`
	if err := os.WriteFile(filepath.Join(profilesDir, "thirdpartymirrors"), []byte(content), 0644); err != nil {
		t.Fatalf("writing thirdpartymirrors: %v", err)
	}

	mirrors := ParseThirdPartyMirrors(tmpDir)

	// Should have trailing slashes added
	expected := []string{"https://ftp.gnu.org/gnu/", "https://mirrors.kernel.org/gnu/"}
	if !reflect.DeepEqual(mirrors["gnu"], expected) {
		t.Errorf("got %v, want %v (with trailing slashes)", mirrors["gnu"], expected)
	}

	// Test expansion produces correct URLs
	expanded := mirrors.ExpandMirrorURL("mirror://gnu/hello/file.tar.gz")
	expectedExpanded := []string{
		"https://ftp.gnu.org/gnu/hello/file.tar.gz",
		"https://mirrors.kernel.org/gnu/hello/file.tar.gz",
	}
	if !reflect.DeepEqual(expanded, expectedExpanded) {
		t.Errorf("expansion = %v, want %v", expanded, expectedExpanded)
	}
}
