package ebuild

import (
	"strings"
	"testing"
)

func TestSplitEbuildAtInherit(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantPre    string
		wantPost   string
		wantNilPre bool
	}{
		{
			name: "simple ebuild with inherit",
			content: `EAPI=8
PYTHON_COMPAT=( python3_{11..14} )

inherit distutils-r1

DESCRIPTION="test"
SRC_URI="http://example.com/test.tar.gz"

src_configure() {
	econf
}`,
			wantPre: `EAPI=8
PYTHON_COMPAT=( python3_{11..14} )
`,
			wantPost: `
DESCRIPTION="test"
SRC_URI="http://example.com/test.tar.gz"

src_configure() {
	econf
}`,
		},
		{
			name: "ebuild without inherit",
			content: `EAPI=8
DESCRIPTION="test"
SRC_URI="http://example.com/test.tar.gz"`,
			wantNilPre: true,
			wantPost: `EAPI=8
DESCRIPTION="test"
SRC_URI="http://example.com/test.tar.gz"`,
		},
		{
			name: "multiple inherit lines - split at first",
			content: `EAPI=8
DISTUTILS_USE_PEP517=setuptools
DISTUTILS_OPTIONAL=1
PYTHON_COMPAT=( python3_{11..14} )

inherit distutils-r1 toolchain-funcs multilib-minimal

if [[ ${PV} == 9999 ]] ; then
	inherit autotools git-r3
else
	inherit libtool verify-sig
fi

DESCRIPTION="test"`,
			wantPre: `EAPI=8
DISTUTILS_USE_PEP517=setuptools
DISTUTILS_OPTIONAL=1
PYTHON_COMPAT=( python3_{11..14} )
`,
			wantPost: `
if [[ ${PV} == 9999 ]] ; then
	inherit autotools git-r3
else
	inherit libtool verify-sig
fi

DESCRIPTION="test"`,
		},
		{
			name: "inherit on first line",
			content: `inherit toolchain-funcs
DESCRIPTION="test"`,
			wantPre:  ``,
			wantPost: `DESCRIPTION="test"`,
		},
		{
			name: "commented inherit ignored",
			content: `EAPI=8
# inherit old-eclass
MY_VAR=1

inherit real-eclass

DESCRIPTION="test"`,
			wantPre: `EAPI=8
# inherit old-eclass
MY_VAR=1
`,
			wantPost: `
DESCRIPTION="test"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pre, post := splitEbuildAtInherit([]byte(tt.content))

			if tt.wantNilPre {
				if pre != nil {
					t.Errorf("expected nil pre, got %q", string(pre))
				}
			} else {
				if string(pre) != tt.wantPre {
					t.Errorf("pre mismatch:\ngot:  %q\nwant: %q", string(pre), tt.wantPre)
				}
			}

			if string(post) != tt.wantPost {
				t.Errorf("post mismatch:\ngot:  %q\nwant: %q", string(post), tt.wantPost)
			}
		})
	}
}

func TestScanExportFunctions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "single EXPORT_FUNCTIONS",
			content: `# multilib-minimal.eclass
EXPORT_FUNCTIONS src_configure src_compile src_test src_install`,
			want: []string{"src_configure", "src_compile", "src_test", "src_install"},
		},
		{
			name: "multiple EXPORT_FUNCTIONS",
			content: `EXPORT_FUNCTIONS src_prepare
some_function() { true; }
EXPORT_FUNCTIONS src_configure`,
			want: []string{"src_prepare", "src_configure"},
		},
		{
			name: "commented out EXPORT_FUNCTIONS ignored",
			content: `# EXPORT_FUNCTIONS src_prepare
EXPORT_FUNCTIONS src_install`,
			want: []string{"src_install"},
		},
		{
			name: "no EXPORT_FUNCTIONS",
			content: `some_function() {
	echo "no exports"
}`,
			want: nil,
		},
		{
			name:    "EXPORT_FUNCTIONS with single phase",
			content: `EXPORT_FUNCTIONS pkg_setup`,
			want:    []string{"pkg_setup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanExportFunctions(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("len mismatch: got %d, want %d\ngot:  %v\nwant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("phase[%d] mismatch: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEclassPreprocessing(t *testing.T) {
	// Test that the preprocessing replacements work correctly.
	// These are applied to eclass content before embedding in the combined script.
	tests := []struct {
		name    string
		input   string
		check   string // substring that must be present in output
		noCheck string // substring that must NOT be present in output
	}{
		{
			name:    "declare -f replaced with __grpm_has_func",
			input:   `if declare -f my_func >/dev/null; then`,
			check:   `if __grpm_has_func my_func >/dev/null; then`,
			noCheck: `declare -f`,
		},
		{
			name:    "declare -p replaced with __grpm_has_var",
			input:   `[[ $(declare -p PYTHON_COMPAT) == "declare -a"* ]]`,
			check:   `[[ $(__grpm_has_var PYTHON_COMPAT) == "declare -a"* ]]`,
			noCheck: `declare -p`,
		},
		{
			name:  "type -P replaced with command -v",
			input: `type -P eltpatch &>/dev/null || die`,
			// mvdan.cc/sh: type -P returns "NOT IMPLEMENTED" (exit 3)
			// command -v is equivalent and supported
			check:   `command -v eltpatch &>/dev/null || die`,
			noCheck: `type -P`,
		},
		{
			name:    "unrelated declare not replaced",
			input:   `declare MY_VAR="test"`,
			check:   `declare MY_VAR="test"`,
			noCheck: "__grpm_has",
		},
		{
			name:    "declare -a not replaced",
			input:   `declare -a MY_ARRAY=()`,
			check:   `declare -a MY_ARRAY=()`,
			noCheck: "__grpm_has",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply the same replacements as in RunPhaseFunction
			content := []byte(tt.input)
			content = replaceBytes(content, "declare -f ", "__grpm_has_func ")
			content = replaceBytes(content, "declare -p ", "__grpm_has_var ")
			content = replaceBytes(content, "type -P ", "command -v ")
			result := string(content)

			if !strings.Contains(result, tt.check) {
				t.Errorf("expected output to contain %q, got: %q", tt.check, result)
			}
			if tt.noCheck != "" && strings.Contains(result, tt.noCheck) {
				t.Errorf("expected output NOT to contain %q, got: %q", tt.noCheck, result)
			}
		})
	}
}

// replaceBytes mirrors bytes.ReplaceAll for test purposes.
func replaceBytes(s []byte, old, new string) []byte {
	return []byte(strings.ReplaceAll(string(s), old, new))
}
