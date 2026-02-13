package ebuild

import (
	"testing"
)

func TestParseEbuildScriptFromString(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantFuncs   []string
		wantNoFuncs []string
		wantEAPI    string
		wantInherit []string
		wantErr     bool
	}{
		{
			name: "simple ebuild with src_configure",
			content: `
EAPI=8
DESCRIPTION="Test package"
SLOT="0"

src_configure() {
	econf --with-feature
}

src_compile() {
	emake
}
`,
			wantFuncs:   []string{"src_configure", "src_compile"},
			wantNoFuncs: []string{"src_install", "src_unpack"},
			wantEAPI:    "8",
		},
		{
			name: "ebuild with inherit",
			content: `
EAPI=8
inherit cmake

src_configure() {
	cmake_src_configure
}
`,
			wantFuncs:   []string{"src_configure"},
			wantInherit: []string{"cmake"},
			wantEAPI:    "8",
		},
		{
			name: "ebuild with multiple inherit",
			content: `
EAPI=7
inherit toolchain-funcs eutils multilib

src_compile() {
	tc-getCC
	emake
}
`,
			wantFuncs:   []string{"src_compile"},
			wantInherit: []string{"toolchain-funcs", "eutils", "multilib"},
			wantEAPI:    "7",
		},
		{
			name: "ebuild with no phase functions",
			content: `
EAPI=8
DESCRIPTION="Simple package"
SRC_URI="https://example.com/foo.tar.gz"
`,
			wantNoFuncs: []string{"src_configure", "src_compile", "src_install"},
			wantEAPI:    "8",
		},
		{
			name: "ebuild with all standard phases",
			content: `
EAPI=8

pkg_setup() { :; }
src_unpack() { default; }
src_prepare() { eapply_user; }
src_configure() { econf; }
src_compile() { emake; }
src_test() { emake check; }
src_install() { emake install DESTDIR="${D}"; }
pkg_postinst() { einfo "Done"; }
`,
			wantFuncs: []string{
				"pkg_setup", "src_unpack", "src_prepare", "src_configure",
				"src_compile", "src_test", "src_install", "pkg_postinst",
			},
			wantEAPI: "8",
		},
		{
			name: "ebuild with function keyword",
			content: `
EAPI=8

function src_configure {
	econf
}

function src_compile {
	emake
}
`,
			wantFuncs: []string{"src_configure", "src_compile"},
			wantEAPI:  "8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := ParseEbuildScriptFromString(tt.content, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEbuildScriptFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check expected functions exist
			for _, fn := range tt.wantFuncs {
				if !script.HasFunction(fn) {
					t.Errorf("expected function %s not found", fn)
				}
			}

			// Check functions that should NOT exist
			for _, fn := range tt.wantNoFuncs {
				if script.HasFunction(fn) {
					t.Errorf("function %s should not exist", fn)
				}
			}

			// Check EAPI
			if tt.wantEAPI != "" && script.EAPI != tt.wantEAPI {
				t.Errorf("EAPI = %s, want %s", script.EAPI, tt.wantEAPI)
			}

			// Check inherited eclasses
			if len(tt.wantInherit) > 0 {
				if len(script.InheritedEclasses) != len(tt.wantInherit) {
					t.Errorf("InheritedEclasses = %v, want %v", script.InheritedEclasses, tt.wantInherit)
				} else {
					for i, ec := range tt.wantInherit {
						if script.InheritedEclasses[i] != ec {
							t.Errorf("InheritedEclasses[%d] = %s, want %s", i, script.InheritedEclasses[i], ec)
						}
					}
				}
			}
		})
	}
}

func TestParseEbuildScript_ConditionalInherit(t *testing.T) {
	// Reproduces the make-4.4.1 pattern: inherit git-r3 inside if [[ ${PV} == 9999 ]]
	makeEbuild := `
EAPI=8
inherit flag-o-matic unpacker verify-sig

if [[ ${PV} == 9999 ]] ; then
	EGIT_REPO_URI="https://git.savannah.gnu.org/git/make.git"
	inherit autotools git-r3
elif [[ $(ver_cut 3) -ge 90 ]] ; then
	SRC_URI="https://alpha.gnu.org/gnu/make/${P}.tar.lz"
else
	SRC_URI="mirror://gnu/make/${P}.tar.lz"
	KEYWORDS="amd64 x86"
fi
`
	tests := []struct {
		name        string
		vars        map[string]string
		wantInherit []string
	}{
		{
			name:        "PV=4.4.1 should skip git-r3",
			vars:        map[string]string{"PV": "4.4.1"},
			wantInherit: []string{"flag-o-matic", "unpacker", "verify-sig"},
		},
		{
			name:        "PV=9999 should include git-r3",
			vars:        map[string]string{"PV": "9999"},
			wantInherit: []string{"flag-o-matic", "unpacker", "verify-sig", "autotools", "git-r3"},
		},
		{
			name: "nil vars infers PV from filename (conservative for string parse)",
			vars: nil,
			// Without vars, condition can't be evaluated → all branches included
			wantInherit: []string{"flag-o-matic", "unpacker", "verify-sig", "autotools", "git-r3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := ParseEbuildScriptFromString(makeEbuild, tt.vars)
			if err != nil {
				t.Fatalf("ParseEbuildScriptFromString() error = %v", err)
			}
			if len(script.InheritedEclasses) != len(tt.wantInherit) {
				t.Errorf("InheritedEclasses = %v, want %v", script.InheritedEclasses, tt.wantInherit)
				return
			}
			for i, ec := range tt.wantInherit {
				if script.InheritedEclasses[i] != ec {
					t.Errorf("InheritedEclasses[%d] = %s, want %s", i, script.InheritedEclasses[i], ec)
				}
			}
		})
	}
}

func TestParseEbuildScript_ConditionalPatternMatch(t *testing.T) {
	// Test glob patterns in conditions
	content := `
EAPI=8
inherit base-eclass

if [[ ${PV} == *_p* ]] ; then
	inherit snapshot-eclass
fi
`
	tests := []struct {
		name        string
		vars        map[string]string
		wantInherit []string
	}{
		{
			name:        "PV without _p skips snapshot eclass",
			vars:        map[string]string{"PV": "1.2.3"},
			wantInherit: []string{"base-eclass"},
		},
		{
			name:        "PV with _p includes snapshot eclass",
			vars:        map[string]string{"PV": "1.2.3_p20250101"},
			wantInherit: []string{"base-eclass", "snapshot-eclass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := ParseEbuildScriptFromString(content, tt.vars)
			if err != nil {
				t.Fatalf("ParseEbuildScriptFromString() error = %v", err)
			}
			if len(script.InheritedEclasses) != len(tt.wantInherit) {
				t.Errorf("InheritedEclasses = %v, want %v", script.InheritedEclasses, tt.wantInherit)
				return
			}
			for i, ec := range tt.wantInherit {
				if script.InheritedEclasses[i] != ec {
					t.Errorf("InheritedEclasses[%d] = %s, want %s", i, script.InheritedEclasses[i], ec)
				}
			}
		})
	}
}

func TestParseEbuildScript_PVFromFilename(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"make-4.4.1-r102.ebuild", "4.4.1"},
		{"make-4.4.1.ebuild", "4.4.1"},
		{"zlib-1.3.1.ebuild", "1.3.1"},
		{"python-3.12.0_beta1.ebuild", "3.12.0_beta1"},
		{"make-9999.ebuild", "9999"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pvFromPath(tt.path)
			if got != tt.want {
				t.Errorf("pvFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestEbuildScriptHasPhaseFunction(t *testing.T) {
	content := `
EAPI=8
src_configure() { econf; }
src_compile() { emake; }
`
	script, err := ParseEbuildScriptFromString(content, nil)
	if err != nil {
		t.Fatalf("ParseEbuildScriptFromString() error = %v", err)
	}

	tests := []struct {
		phase Phase
		want  bool
	}{
		{PhaseConfigure, true},
		{PhaseCompile, true},
		{PhaseUnpack, false},
		{PhasePrepare, false},
		{PhaseInstall, false},
		{PhaseTest, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := script.HasPhaseFunction(tt.phase); got != tt.want {
				t.Errorf("HasPhaseFunction(%s) = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestFindDefinedPhases(t *testing.T) {
	content := `
EAPI=8
pkg_setup() { :; }
src_configure() { econf; }
src_install() { emake install DESTDIR="${D}"; }
pkg_postinst() { einfo "Done"; }
`
	script, err := ParseEbuildScriptFromString(content, nil)
	if err != nil {
		t.Fatalf("ParseEbuildScriptFromString() error = %v", err)
	}

	defined := script.FindDefinedPhases()

	// Should contain setup, configure, install, postinst
	expectedPhases := map[Phase]bool{
		PhaseSetup:     true,
		PhaseConfigure: true,
		PhaseInstall:   true,
		PhasePostinst:  true,
	}

	for _, phase := range defined {
		if !expectedPhases[phase] {
			t.Errorf("unexpected phase %s in defined phases", phase)
		}
		delete(expectedPhases, phase)
	}

	if len(expectedPhases) > 0 {
		for phase := range expectedPhases {
			t.Errorf("expected phase %s not found", phase)
		}
	}
}

func TestPhaseFunctionName(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhasePretend, "pkg_pretend"},
		{PhaseSetup, "pkg_setup"},
		{PhaseUnpack, "src_unpack"},
		{PhasePrepare, "src_prepare"},
		{PhaseConfigure, "src_configure"},
		{PhaseCompile, "src_compile"},
		{PhaseTest, "src_test"},
		{PhaseInstall, "src_install"},
		{PhasePreinst, "pkg_preinst"},
		{PhasePostinst, "pkg_postinst"},
		{PhasePrerem, "pkg_prerm"},
		{PhasePostrm, "pkg_postrm"},
		{PhaseConfig, "pkg_config"},
		{PhaseInfo, "pkg_info"},
		{PhaseNofetch, "pkg_nofetch"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := phaseFunctionName(tt.phase); got != tt.want {
				t.Errorf("phaseFunctionName(%s) = %s, want %s", tt.phase, got, tt.want)
			}
		})
	}
}

func TestQuickParseFunctions(t *testing.T) {
	// This test would require creating a temp file
	// Skip for now as it's a convenience function
	t.Skip("QuickParseFunctions requires temp file")
}

// BenchmarkParseEbuildScript benchmarks parsing performance
func BenchmarkParseEbuildScript(b *testing.B) {
	content := `
EAPI=8
DESCRIPTION="Test package"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/foo-1.0.tar.gz"
LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64 ~x86"
IUSE="doc test"

inherit cmake

src_configure() {
	local mycmakeargs=(
		-DBUILD_TESTS=$(usex test)
		-DBUILD_DOCS=$(usex doc)
	)
	cmake_src_configure
}

src_install() {
	cmake_src_install
	dodoc README.md
}
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseEbuildScriptFromString(content, nil)
	}
}

// TestParseEbuildScript_BashSyntax tests LangBash variant support for bash-specific syntax.
func TestParseEbuildScript_BashSyntax(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "brace expansion in path",
			content: `
src_install() {
	rm "${ED}"/bin/{egrep,fgrep} || die
}
`,
			wantErr: false,
		},
		{
			name: "local array declaration",
			content: `
src_configure() {
	local myeconfargs=(
		--prefix=/usr
		--with-foo
	)
	econf "${myeconfargs[@]}"
}
`,
			wantErr: false,
		},
		{
			name: "array with usex conditional",
			content: `
src_configure() {
	local mycmakeargs=(
		-DBUILD_TESTING=$(usex test)
	)
}
`,
			wantErr: false,
		},
		{
			name: "complex brace expansion",
			content: `
src_prepare() {
	touch {,doc/,lib/,src/}Makefile.in || die
}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEbuildScriptFromString(tt.content, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEbuildScriptFromString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseEbuildScript_UnsupportedSyntax documents known parser limitations.
// These patterns are valid bash but not supported by mvdan.cc/sh parser.
func TestParseEbuildScript_UnsupportedSyntax(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		description string
	}{
		{
			name:        "brace expansion in variable name",
			content:     `export RUN_{VERY_,}EXPENSIVE_TESTS=yes`,
			wantErr:     true,
			description: "Brace expansion in variable names not supported by mvdan.cc/sh",
		},
		{
			name:        "brace expansion in export statement",
			content:     `export FOO_{A,B}_BAR=value`,
			wantErr:     true,
			description: "Multiple variable names via brace expansion not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEbuildScriptFromString(tt.content, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEbuildScriptFromString() error = %v, wantErr %v\nNote: %s",
					err, tt.wantErr, tt.description)
			}
			if tt.wantErr && err != nil {
				t.Logf("Expected limitation: %s - error: %v", tt.description, err)
			}
		})
	}
}

// TestParseEbuildScript_RealWorldPatterns tests patterns from real Gentoo ebuilds.
func TestParseEbuildScript_RealWorldPatterns(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "cmake with options array",
			content: `
EAPI=8
inherit cmake

src_configure() {
	local mycmakeargs=(
		-DCMAKE_INSTALL_PREFIX="${EPREFIX}/usr"
		-DBUILD_SHARED_LIBS=ON
		$(cmake_use_find_package doc Doxygen)
	)
	cmake_src_configure
}
`,
			wantErr: false,
		},
		{
			name: "meson with options",
			content: `
EAPI=8
inherit meson

src_configure() {
	local emesonargs=(
		-Ddefault_library=shared
		$(meson_use test tests)
	)
	meson_src_configure
}
`,
			wantErr: false,
		},
		{
			name: "autotools with flag-o-matic",
			content: `
EAPI=8
inherit autotools flag-o-matic

src_configure() {
	append-cflags -fPIC
	econf \
		--disable-static \
		$(use_enable nls)
}
`,
			wantErr: false,
		},
		{
			name: "complex use conditional",
			content: `
src_install() {
	default
	use doc && dodoc -r docs/*
	if use examples; then
		docinto examples
		dodoc -r examples/*
	fi
}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEbuildScriptFromString(tt.content, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEbuildScriptFromString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
