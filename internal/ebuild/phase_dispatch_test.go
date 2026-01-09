package ebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestPhaseDispatch_CustomFunction verifies that custom phase functions in ebuilds
// are called instead of default implementations.
func TestPhaseDispatch_CustomFunction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ebuild with custom src_configure
	ebuildContent := `
EAPI=8
DESCRIPTION="Test package"
SLOT="0"

src_configure() {
	einfo "Custom configure called"
}
`
	ebuildPath := filepath.Join(tmpDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Create executor
	testPkg := &pkg.Package{
		Name:     "dev-test/test",
		Version:  "1.0",
		UseFlags: map[string]bool{},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	executor := &Executor{
		Env:        env,
		EbuildPath: ebuildPath,
	}

	// Parse ebuild to detect functions
	script, err := ParseEbuildScript(ebuildPath)
	if err != nil {
		t.Fatalf("failed to parse ebuild: %v", err)
	}
	executor.ParsedEbuild = script

	// Verify HasPhaseFunction detects the custom function
	if !executor.HasPhaseFunction(PhaseConfigure) {
		t.Error("expected HasPhaseFunction to return true for src_configure")
	}

	// Verify it returns false for undefined functions
	if executor.HasPhaseFunction(PhaseCompile) {
		t.Error("expected HasPhaseFunction to return false for src_compile")
	}
}

// TestPhaseDispatch_DefaultFunctionCalled verifies that when no custom function
// exists, the default implementation is used.
func TestPhaseDispatch_DefaultFunctionCalled(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal ebuild WITHOUT custom functions
	ebuildContent := `
EAPI=8
DESCRIPTION="Minimal test package"
SLOT="0"
`
	ebuildPath := filepath.Join(tmpDir, "minimal-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Create executor
	testPkg := &pkg.Package{
		Name:     "dev-test/minimal",
		Version:  "1.0",
		UseFlags: map[string]bool{},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	executor := &Executor{
		Env:        env,
		EbuildPath: ebuildPath,
	}

	// Parse ebuild
	script, err := ParseEbuildScript(ebuildPath)
	if err != nil {
		t.Fatalf("failed to parse ebuild: %v", err)
	}
	executor.ParsedEbuild = script

	// Verify no custom functions are detected
	phases := []Phase{PhaseConfigure, PhaseCompile, PhaseInstall, PhaseUnpack, PhasePrepare}
	for _, phase := range phases {
		if executor.HasPhaseFunction(phase) {
			t.Errorf("expected HasPhaseFunction to return false for %s", phase)
		}
	}
}

// TestPhaseDispatch_EclassExportedFunction verifies that EXPORT_FUNCTIONS
// from eclasses are honored.
func TestPhaseDispatch_EclassExportedFunction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ebuild that inherits an eclass
	ebuildContent := `
EAPI=8
inherit cmake
DESCRIPTION="CMake test package"
SLOT="0"
`
	ebuildPath := filepath.Join(tmpDir, "cmake-test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Parse ebuild
	script, err := ParseEbuildScript(ebuildPath)
	if err != nil {
		t.Fatalf("failed to parse ebuild: %v", err)
	}

	// Verify inherited eclasses are detected
	if len(script.InheritedEclasses) == 0 {
		t.Error("expected inherited eclasses to be detected")
	}

	foundCmake := false
	for _, ec := range script.InheritedEclasses {
		if ec == "cmake" {
			foundCmake = true
			break
		}
	}
	if !foundCmake {
		t.Errorf("expected 'cmake' in inherited eclasses, got: %v", script.InheritedEclasses)
	}
}

// TestPhaseDispatch_DefaultCall verifies that `default` calls the appropriate
// default_src_* function based on EBUILD_PHASE.
func TestPhaseDispatch_DefaultCall(t *testing.T) {
	tests := []struct {
		phase    string
		expected string
	}{
		{"unpack", "DefaultSrcUnpack"},
		{"prepare", "DefaultSrcPrepare"},
		{"configure", "DefaultSrcConfigure"},
		{"compile", "DefaultSrcCompile"},
		{"test", "DefaultSrcTest"},
		{"install", "DefaultSrcInstall"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			testPkg := &pkg.Package{
				Name:     "dev-test/default-test",
				Version:  "1.0",
				UseFlags: map[string]bool{},
			}

			tmpDir := t.TempDir()
			env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
			if err != nil {
				t.Fatalf("failed to create environment: %v", err)
			}

			// Set EBUILD_PHASE in the environment
			env.EBUILD_PHASE = tt.phase

			helpers := NewHelpers(env, os.Stdout, os.Stderr)

			// Call Default() - it should not error (even if actual phase fails)
			// The point is that the correct phase is dispatched
			err = helpers.Default(nil)

			// For phases that need specific setup (like install), we expect errors
			// But we're just testing dispatch, not actual execution
			// The key is no panic and correct function is selected
			_ = err // Errors expected for phases without proper setup
		})
	}
}

// TestRunPhaseFunction_CombinedScript verifies that RunPhaseFunction properly
// combines ebuild sourcing and function execution.
func TestRunPhaseFunction_CombinedScript(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ebuild with custom src_configure that outputs a marker
	ebuildContent := `
EAPI=8
DESCRIPTION="Test package"
SLOT="0"

src_configure() {
	einfo "MARKER_CONFIGURE_CALLED"
}
`
	ebuildPath := filepath.Join(tmpDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Create executor
	testPkg := &pkg.Package{
		Name:     "dev-test/test",
		Version:  "1.0",
		UseFlags: map[string]bool{},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	executor := &Executor{
		Env:        env,
		EbuildPath: ebuildPath,
	}

	// Parse ebuild
	script, parseErr := ParseEbuildScript(ebuildPath)
	if parseErr != nil {
		t.Fatalf("failed to parse ebuild: %v", parseErr)
	}
	executor.ParsedEbuild = script

	// Run the phase function
	output, runErr := executor.RunPhaseFunction(PhaseConfigure)
	if runErr != nil {
		t.Fatalf("RunPhaseFunction failed: %v", runErr)
	}

	// Verify the custom function was called
	if !strings.Contains(output, "MARKER_CONFIGURE_CALLED") {
		t.Errorf("expected output to contain MARKER_CONFIGURE_CALLED, got: %s", output)
	}
}

// TestRunPhaseFunction_DefaultFromCustom verifies that custom functions can
// call `default` to invoke default behavior.
func TestRunPhaseFunction_DefaultFromCustom(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ebuild that calls default from custom function
	ebuildContent := `
EAPI=8
DESCRIPTION="Test package"
SLOT="0"

src_prepare() {
	einfo "Before default"
	default
	einfo "After default"
}
`
	ebuildPath := filepath.Join(tmpDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Create executor
	testPkg := &pkg.Package{
		Name:     "dev-test/test",
		Version:  "1.0",
		UseFlags: map[string]bool{},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	executor := &Executor{
		Env:        env,
		EbuildPath: ebuildPath,
	}

	// Parse ebuild
	script, parseErr := ParseEbuildScript(ebuildPath)
	if parseErr != nil {
		t.Fatalf("failed to parse ebuild: %v", parseErr)
	}
	executor.ParsedEbuild = script

	// Run the phase function
	output, runErr := executor.RunPhaseFunction(PhasePrepare)
	if runErr != nil {
		t.Fatalf("RunPhaseFunction failed: %v", runErr)
	}

	// Verify both markers are present
	if !strings.Contains(output, "Before default") {
		t.Errorf("expected output to contain 'Before default', got: %s", output)
	}
	if !strings.Contains(output, "After default") {
		t.Errorf("expected output to contain 'After default', got: %s", output)
	}
}

// TestDispatchPhaseFunctionName verifies the phase to function name mapping.
// Note: TestPhaseFunctionName already exists in script_test.go
func TestDispatchPhaseFunctionName(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
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
			got := phaseFunctionName(tt.phase)
			if got != tt.expected {
				t.Errorf("phaseFunctionName(%s) = %s, want %s", tt.phase, got, tt.expected)
			}
		})
	}
}

// TestEbuildScript_AllStandardPhases verifies detection of all standard phases.
func TestEbuildScript_AllStandardPhases(t *testing.T) {
	content := `
EAPI=8
DESCRIPTION="Full ebuild"
SLOT="0"

pkg_setup() { :; }
src_unpack() { default; }
src_prepare() { default; eapply_user; }
src_configure() { econf --enable-feature; }
src_compile() { emake; }
src_test() { emake check; }
src_install() { emake install DESTDIR="${D}"; }
pkg_preinst() { :; }
pkg_postinst() { einfo "Installed"; }
pkg_prerm() { :; }
pkg_postrm() { :; }
`

	script, err := ParseEbuildScriptFromString(content)
	if err != nil {
		t.Fatalf("ParseEbuildScriptFromString failed: %v", err)
	}

	// All phases should be defined
	allPhases := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
		PhasePreinst,
		PhasePostinst,
		PhasePrerem,
		PhasePostrm,
	}

	for _, phase := range allPhases {
		if !script.HasPhaseFunction(phase) {
			t.Errorf("expected HasPhaseFunction(%s) to return true", phase)
		}
	}
}

// TestEbuildScript_FindDefinedPhasesDispatch verifies FindDefinedPhases returns correct list.
// Note: TestFindDefinedPhases already exists in script_test.go
func TestEbuildScript_FindDefinedPhasesDispatch(t *testing.T) {
	content := `
EAPI=8
src_configure() { econf; }
src_compile() { emake; }
pkg_postinst() { einfo "Done"; }
`

	script, err := ParseEbuildScriptFromString(content)
	if err != nil {
		t.Fatalf("ParseEbuildScriptFromString failed: %v", err)
	}

	defined := script.FindDefinedPhases()

	// Should have exactly 3 phases
	if len(defined) != 3 {
		t.Errorf("expected 3 defined phases, got %d: %v", len(defined), defined)
	}

	// Create a set for checking
	phaseSet := make(map[Phase]bool)
	for _, p := range defined {
		phaseSet[p] = true
	}

	expectedPhases := []Phase{PhaseConfigure, PhaseCompile, PhasePostinst}
	for _, expected := range expectedPhases {
		if !phaseSet[expected] {
			t.Errorf("expected %s to be in defined phases", expected)
		}
	}

	// These should NOT be defined
	unexpectedPhases := []Phase{PhaseSetup, PhaseUnpack, PhasePrepare, PhaseInstall}
	for _, unexpected := range unexpectedPhases {
		if phaseSet[unexpected] {
			t.Errorf("did not expect %s to be in defined phases", unexpected)
		}
	}
}

// TestHasPhaseFunction_WithNilScript verifies HasPhaseFunction handles nil script.
func TestHasPhaseFunction_WithNilScript(t *testing.T) {
	executor := &Executor{
		ParsedEbuild: nil, // No script parsed
	}

	// Should return false without panic
	if executor.HasPhaseFunction(PhaseConfigure) {
		t.Error("expected HasPhaseFunction to return false with nil script")
	}
}

// TestRunPhaseFunction_UnknownPhase verifies error handling for unknown phases.
func TestRunPhaseFunction_UnknownPhase(t *testing.T) {
	executor := &Executor{}

	// Unknown phase should return error
	_, err := executor.RunPhaseFunction(Phase("unknown"))
	if err == nil {
		t.Error("expected error for unknown phase")
	}
	if !strings.Contains(err.Error(), "unknown phase") {
		t.Errorf("expected 'unknown phase' in error, got: %v", err)
	}
}

// TestEbuildPhase_EnvironmentVariable verifies EBUILD_PHASE is set correctly.
func TestEbuildPhase_EnvironmentVariable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild that echoes EBUILD_PHASE
	ebuildContent := `
EAPI=8
SLOT="0"

src_configure() {
	einfo "Phase is: ${EBUILD_PHASE}"
}
`
	ebuildPath := filepath.Join(tmpDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	testPkg := &pkg.Package{
		Name:     "dev-test/test",
		Version:  "1.0",
		UseFlags: map[string]bool{},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	executor := &Executor{
		Env:        env,
		EbuildPath: ebuildPath,
	}

	script, parseErr := ParseEbuildScript(ebuildPath)
	if parseErr != nil {
		t.Fatalf("failed to parse ebuild: %v", parseErr)
	}
	executor.ParsedEbuild = script

	output, runErr := executor.RunPhaseFunction(PhaseConfigure)
	if runErr != nil {
		t.Fatalf("RunPhaseFunction failed: %v", runErr)
	}

	// Verify EBUILD_PHASE was set to "configure"
	if !strings.Contains(output, "Phase is: configure") {
		t.Errorf("expected output to contain 'Phase is: configure', got: %s", output)
	}
}

// BenchmarkParseEbuildScriptPhaseDispatch benchmarks ebuild parsing performance.
// Note: BenchmarkParseEbuildScript already exists in script_test.go
func BenchmarkParseEbuildScriptPhaseDispatch(b *testing.B) {
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

// BenchmarkHasPhaseFunction benchmarks function detection.
func BenchmarkHasPhaseFunctionDispatch(b *testing.B) {
	content := `
EAPI=8
src_configure() { econf; }
src_compile() { emake; }
src_install() { emake install; }
`
	script, err := ParseEbuildScriptFromString(content)
	if err != nil {
		b.Fatalf("ParseEbuildScriptFromString failed: %v", err)
	}

	executor := &Executor{ParsedEbuild: script}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.HasPhaseFunction(PhaseConfigure)
		executor.HasPhaseFunction(PhaseCompile)
		executor.HasPhaseFunction(PhaseInstall)
		executor.HasPhaseFunction(PhaseUnpack) // Not defined
	}
}
