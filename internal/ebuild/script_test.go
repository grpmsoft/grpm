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
			script, err := ParseEbuildScriptFromString(tt.content)
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

func TestEbuildScriptHasPhaseFunction(t *testing.T) {
	content := `
EAPI=8
src_configure() { econf; }
src_compile() { emake; }
`
	script, err := ParseEbuildScriptFromString(content)
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
	script, err := ParseEbuildScriptFromString(content)
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
		_, _ = ParseEbuildScriptFromString(content)
	}
}
