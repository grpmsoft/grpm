package ebuild

import (
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
