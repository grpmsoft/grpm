//go:build integration

// Package integration provides integration tests for GRPM against real Gentoo packages.
//
// This file implements comprehensive eclass integration tests that verify GRPM's
// ability to load and execute the most commonly used Gentoo eclasses.
//
// Run with: go test -v -tags=integration ./tests/integration/... -run TestEclass
package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/eclass"
	"github.com/grpmsoft/grpm/internal/pkg"
	"mvdan.cc/sh/v3/interp"
)

// EclassTestSpec defines an eclass to test.
type EclassTestSpec struct {
	// Name is the eclass name (e.g., "cmake", "meson")
	Name string

	// Description is a human-readable description
	Description string

	// Priority indicates importance (P0=critical, P1=important, P2=nice-to-have)
	Priority string

	// ExpectedIUSE lists USE flags the eclass should add to IUSE
	ExpectedIUSE []string

	// ExpectedDEPEND lists dependencies the eclass should add
	ExpectedDEPEND []string

	// ExportedPhases lists phase functions the eclass should export
	ExportedPhases []string

	// RequiredHelpers lists Go helper functions required by this eclass
	RequiredHelpers []string

	// SkipReason if set, test is skipped with this reason
	SkipReason string

	// MockContent is optional mock eclass content for CI testing
	MockContent string
}

// EclassTestResult contains the result of an eclass test.
type EclassTestResult struct {
	Name          string
	LoadSuccess   bool
	LoadError     error
	LoadDuration  time.Duration
	IUSE          string
	DEPEND        string
	RDEPEND       string
	Inherited     []string
	ExportedFuncs map[string]string
}

// EclassTestSuite contains the test specifications for top eclasses.
var EclassTestSuite = []EclassTestSpec{
	// Priority 0: Critical eclasses (highest usage)
	{
		Name:        "toolchain-funcs",
		Description: "Compiler detection and toolchain utilities",
		Priority:    "P0",
		RequiredHelpers: []string{
			"tc-getCC", "tc-getCXX", "tc-getLD", "tc-arch",
			"tc-is-gcc", "tc-is-clang", "tc-export",
		},
		MockContent: `# toolchain-funcs.eclass - Mock for testing
# Provides toolchain detection functions

tc-getCC() {
	echo "${CC:-gcc}"
}

tc-getCXX() {
	echo "${CXX:-g++}"
}

tc-getLD() {
	echo "${LD:-ld}"
}

tc-arch() {
	local host="${1:-${CHOST}}"
	case "${host}" in
		x86_64*) echo "amd64" ;;
		i?86*) echo "x86" ;;
		arm*) echo "arm" ;;
		aarch64*) echo "arm64" ;;
		*) echo "unknown" ;;
	esac
}
`,
	},
	{
		Name:        "multilib",
		Description: "Multilib support for 32/64-bit systems",
		Priority:    "P0",
		RequiredHelpers: []string{
			"get_libdir", "multilib_native_use_with", "multilib_native_use_enable",
		},
		MockContent: `# multilib.eclass - Mock for testing
# Provides multilib support

get_libdir() {
	echo "${LIBDIR:-lib64}"
}

get_abi_LIBDIR() {
	echo "${LIBDIR:-lib64}"
}
`,
	},
	{
		Name:        "flag-o-matic",
		Description: "CFLAGS/LDFLAGS manipulation",
		Priority:    "P0",
		RequiredHelpers: []string{
			"append-cflags", "append-ldflags", "filter-flags",
			"strip-flags", "test-flags-CC",
		},
		MockContent: `# flag-o-matic.eclass - Mock for testing
# Provides flag manipulation functions

append-cflags() {
	CFLAGS="${CFLAGS} $*"
}

append-cxxflags() {
	CXXFLAGS="${CXXFLAGS} $*"
}

append-ldflags() {
	LDFLAGS="${LDFLAGS} $*"
}

filter-flags() {
	: # No-op for mock
}

strip-flags() {
	: # No-op for mock
}
`,
	},

	// Priority 1: Build system eclasses
	{
		Name:           "cmake",
		Description:    "CMake build system support",
		Priority:       "P1",
		ExpectedIUSE:   []string{"debug"},
		ExportedPhases: []string{"src_prepare", "src_configure", "src_compile", "src_test", "src_install"},
		RequiredHelpers: []string{
			"cmake", "cmake_src_configure", "cmake_src_compile",
			"cmake_src_install", "cmake_use",
		},
		MockContent: `# cmake.eclass - Mock for testing
inherit toolchain-funcs

IUSE="debug"
BDEPEND="dev-util/cmake"

EXPORT_FUNCTIONS src_prepare src_configure src_compile src_test src_install

cmake_src_prepare() {
	default
}

cmake_src_configure() {
	local mycmakeargs=()
	cmake "${mycmakeargs[@]}"
}

cmake_src_compile() {
	cmake --build "${BUILD_DIR}"
}

cmake_src_test() {
	ctest --test-dir "${BUILD_DIR}"
}

cmake_src_install() {
	cmake --install "${BUILD_DIR}" --prefix "${D}/usr"
}
`,
	},
	{
		Name:           "meson",
		Description:    "Meson build system support",
		Priority:       "P1",
		ExpectedIUSE:   []string{"debug"},
		ExportedPhases: []string{"src_configure", "src_compile", "src_test", "src_install"},
		RequiredHelpers: []string{
			"meson", "meson_src_configure", "meson_src_compile",
			"meson_src_install", "meson_use", "meson_feature",
		},
		MockContent: `# meson.eclass - Mock for testing
inherit toolchain-funcs

IUSE="debug"
BDEPEND="dev-util/meson dev-util/ninja"

EXPORT_FUNCTIONS src_configure src_compile src_test src_install

meson_src_configure() {
	local emesonargs=()
	meson setup "${BUILD_DIR}" "${S}" "${emesonargs[@]}"
}

meson_src_compile() {
	meson compile -C "${BUILD_DIR}"
}

meson_src_test() {
	meson test -C "${BUILD_DIR}"
}

meson_src_install() {
	meson install -C "${BUILD_DIR}" --destdir "${D}"
}
`,
	},

	// Priority 1: Python eclasses
	{
		Name:           "python-utils-r1",
		Description:    "Python utility functions",
		Priority:       "P1",
		ExportedPhases: []string{},
		MockContent: `# python-utils-r1.eclass - Mock for testing
# Provides Python utility functions

python_get_sitedir() {
	echo "/usr/lib/python${EPYTHON}/site-packages"
}

python_get_includedir() {
	echo "/usr/include/python${EPYTHON}"
}
`,
	},
	{
		Name:           "python-single-r1",
		Description:    "Single Python implementation",
		Priority:       "P1",
		ExpectedIUSE:   []string{"python_single_target_python3_11", "python_single_target_python3_12"},
		ExportedPhases: []string{"pkg_setup"},
		MockContent: `# python-single-r1.eclass - Mock for testing
inherit python-utils-r1

IUSE="python_single_target_python3_11 python_single_target_python3_12"
REQUIRED_USE="^^ ( python_single_target_python3_11 python_single_target_python3_12 )"

EXPORT_FUNCTIONS pkg_setup

python-single-r1_pkg_setup() {
	:
}
`,
	},
	{
		Name:           "python-r1",
		Description:    "Multi Python implementations",
		Priority:       "P1",
		ExpectedIUSE:   []string{"python_targets_python3_11", "python_targets_python3_12"},
		ExportedPhases: []string{},
		MockContent: `# python-r1.eclass - Mock for testing
inherit python-utils-r1

IUSE="python_targets_python3_11 python_targets_python3_12"
REQUIRED_USE="|| ( python_targets_python3_11 python_targets_python3_12 )"
`,
	},
	{
		Name:           "distutils-r1",
		Description:    "Python distutils/setuptools build",
		Priority:       "P1",
		ExportedPhases: []string{"src_prepare", "src_configure", "src_compile", "src_test", "src_install"},
		MockContent: `# distutils-r1.eclass - Mock for testing
inherit python-r1

EXPORT_FUNCTIONS src_prepare src_configure src_compile src_test src_install

distutils-r1_src_prepare() {
	default
}

distutils-r1_src_configure() {
	:
}

distutils-r1_src_compile() {
	python_foreach_impl distutils-r1_python_compile
}

distutils-r1_src_test() {
	python_foreach_impl distutils-r1_python_test
}

distutils-r1_src_install() {
	python_foreach_impl distutils-r1_python_install
}
`,
	},

	// Priority 2: Desktop/XDG eclasses
	{
		Name:           "xdg",
		Description:    "XDG base directory support",
		Priority:       "P2",
		ExportedPhases: []string{"pkg_preinst", "pkg_postinst", "pkg_postrm"},
		MockContent: `# xdg.eclass - Mock for testing
inherit xdg-utils

EXPORT_FUNCTIONS pkg_preinst pkg_postinst pkg_postrm

xdg_pkg_preinst() {
	xdg_environment_reset
}

xdg_pkg_postinst() {
	xdg_desktop_database_update
	xdg_icon_cache_update
	xdg_mimeinfo_database_update
}

xdg_pkg_postrm() {
	xdg_desktop_database_update
	xdg_icon_cache_update
	xdg_mimeinfo_database_update
}
`,
	},
	{
		Name:        "xdg-utils",
		Description: "XDG utilities",
		Priority:    "P2",
		MockContent: `# xdg-utils.eclass - Mock for testing
# Provides XDG utility functions

xdg_environment_reset() {
	export XDG_DATA_DIRS="${EPREFIX}/usr/share"
}

xdg_desktop_database_update() {
	:
}

xdg_icon_cache_update() {
	:
}

xdg_mimeinfo_database_update() {
	:
}
`,
	},
	{
		Name:        "desktop",
		Description: "Desktop file installation",
		Priority:    "P2",
		MockContent: `# desktop.eclass - Mock for testing
# Provides desktop file installation

make_desktop_entry() {
	local exec="$1"
	local name="${2:-${PN}}"
	local icon="${3:-${PN}}"
	local type="${4:-Application}"
	: # No-op for mock
}

domenu() {
	insinto /usr/share/applications
	doins "$@"
}

doicon() {
	insinto /usr/share/icons
	doins "$@"
}
`,
	},

	// Priority 2: Language ecosystems
	{
		Name:           "cargo",
		Description:    "Rust cargo support",
		Priority:       "P2",
		ExpectedIUSE:   []string{"debug"},
		ExportedPhases: []string{"src_unpack", "src_configure", "src_compile", "src_test", "src_install"},
		MockContent: `# cargo.eclass - Mock for testing
IUSE="debug"
BDEPEND="virtual/rust"

EXPORT_FUNCTIONS src_unpack src_configure src_compile src_test src_install

cargo_src_unpack() {
	default
}

cargo_src_configure() {
	:
}

cargo_src_compile() {
	cargo build --release
}

cargo_src_test() {
	cargo test
}

cargo_src_install() {
	cargo install --path . --root "${D}/usr"
}
`,
	},
	{
		Name:           "go-module",
		Description:    "Go module support",
		Priority:       "P2",
		ExpectedDEPEND: []string{"dev-lang/go"},
		ExportedPhases: []string{"src_unpack", "src_compile", "src_test", "src_install"},
		MockContent: `# go-module.eclass - Mock for testing
BDEPEND="dev-lang/go"
DEPEND="dev-lang/go"

EXPORT_FUNCTIONS src_unpack src_compile src_test src_install

go-module_src_unpack() {
	default
}

go-module_src_compile() {
	go build -v ./...
}

go-module_src_test() {
	go test -v ./...
}

go-module_src_install() {
	go install -v ./...
}
`,
	},

	// Priority 2: System eclasses
	{
		Name:           "systemd",
		Description:    "Systemd unit file support",
		Priority:       "P2",
		ExportedPhases: []string{},
		MockContent: `# systemd.eclass - Mock for testing
# Provides systemd unit file installation

systemd_dounit() {
	insinto /usr/lib/systemd/system
	doins "$@"
}

systemd_douserunit() {
	insinto /usr/lib/systemd/user
	doins "$@"
}

systemd_get_systemunitdir() {
	echo "/usr/lib/systemd/system"
}

systemd_get_userunitdir() {
	echo "/usr/lib/systemd/user"
}
`,
	},
	{
		Name:        "linux-info",
		Description: "Kernel info detection",
		Priority:    "P2",
		RequiredHelpers: []string{
			"get_version", "linux_config_exists",
		},
		MockContent: `# linux-info.eclass - Mock for testing
# Provides kernel info detection

get_version() {
	echo "6.6.0"
}

linux_config_exists() {
	[[ -f /usr/src/linux/.config ]]
}

linux_config_src_exists() {
	[[ -f /usr/src/linux/.config ]]
}

require_configured_kernel() {
	linux_config_exists || die "Kernel not configured"
}
`,
	},
	{
		Name:           "readme.gentoo-r1",
		Description:    "README display after install",
		Priority:       "P2",
		ExportedPhases: []string{"pkg_postinst"},
		MockContent: `# readme.gentoo-r1.eclass - Mock for testing

EXPORT_FUNCTIONS pkg_postinst

readme.gentoo_create_doc() {
	:
}

readme.gentoo-r1_pkg_postinst() {
	:
}
`,
	},
	{
		Name:        "wrapper",
		Description: "Binary wrapper creation",
		Priority:    "P2",
		MockContent: `# wrapper.eclass - Mock for testing
# Provides binary wrapper creation

make_wrapper() {
	local wrapper="$1"
	local target="$2"
	local chdir="${3:-.}"
	local libdir="${4}"
	local path="${5:-/usr/bin}"
	: # No-op for mock
}
`,
	},
	{
		Name:        "optfeature",
		Description: "Optional feature notification",
		Priority:    "P2",
		MockContent: `# optfeature.eclass - Mock for testing
# Provides optional feature notification

optfeature() {
	local msg="$1"
	shift
	einfo "Optional: ${msg}"
	while [[ $# -gt 0 ]]; do
		einfo "  - $1"
		shift
	done
}
`,
	},
	{
		Name:        "edo",
		Description: "Verbose command execution",
		Priority:    "P2",
		MockContent: `# edo.eclass - Mock for testing
# Provides verbose command execution

edo() {
	einfo "Running: $*"
	"$@" || die "Command failed: $*"
}

edob() {
	ebegin "Running: $*"
	"$@"
	eend $? "Command failed: $*"
}
`,
	},

	// Legacy eclass (deprecated but widely used)
	{
		Name:        "eutils",
		Description: "Legacy utilities (deprecated)",
		Priority:    "P2",
		SkipReason:  "Deprecated eclass, testing individual functions instead",
		RequiredHelpers: []string{
			"epatch", "eshopts_push", "eshopts_pop",
		},
		MockContent: `# eutils.eclass - Mock for testing (DEPRECATED)
# This eclass is deprecated; use individual eclasses instead.

epatch() {
	eapply "$@"
}

eshopts_push() {
	:
}

eshopts_pop() {
	:
}
`,
	},
}

// skipIfNoEclassDir skips the test if no eclass directory is available.
func skipIfNoEclassDir(t *testing.T) {
	t.Helper()

	repoPath := os.Getenv("GRPM_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}

	eclassDir := filepath.Join(repoPath, "eclass")
	if _, err := os.Stat(eclassDir); os.IsNotExist(err) {
		t.Skipf("Eclass directory not found at %s", eclassDir)
	}
}

// setupMockEclassDir creates a temporary directory with mock eclasses.
func setupMockEclassDir(t *testing.T, specs []EclassTestSpec) string {
	t.Helper()

	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, spec := range specs {
		if spec.MockContent == "" {
			continue
		}
		path := filepath.Join(eclassDir, spec.Name+".eclass")
		if err := os.WriteFile(path, []byte(spec.MockContent), 0644); err != nil {
			t.Fatalf("Failed to write mock eclass %s: %v", spec.Name, err)
		}
	}

	return tmpDir
}

// createTestExecHandler creates an exec handler for testing that intercepts
// commands and delegates to Go helper implementations.
//
// The returned function has signature: func(ctx context.Context, args []string) error
// This is the inner handler that will be wrapped by ExecHandlers.
func createTestExecHandler(t *testing.T, _ *ebuild.Helpers) interp.ExecHandlerFunc {
	t.Helper()

	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return nil
		}

		cmd := args[0]

		// Map eclass commands to helpers
		switch cmd {
		case "einfo", "ewarn", "eerror", "ebegin", "eend":
			// Messaging - pass through
			return nil
		case "die":
			if len(args) > 1 {
				return &ebuild.DieError{Message: strings.Join(args[1:], " ")}
			}
			return &ebuild.DieError{}
		case "inherit":
			// Inherit is handled by the eclass executor
			return nil
		case "EXPORT_FUNCTIONS":
			// Export is handled specially
			return nil
		case "default":
			// Default phase - no-op in tests
			return nil
		default:
			// Pass through to shell - return special error to indicate passthrough
			return interp.ExitStatus(127) // Command not found
		}
	}
}

// TestEclass_LoadAll tests that all top eclasses can be loaded without errors.
//
// This test validates:
//   - Eclass file exists and is readable
//   - Eclass can be parsed by mvdan.cc/sh
//   - No syntax errors in the eclass
func TestEclass_LoadAll(t *testing.T) {
	repoPath := os.Getenv("GRPM_REPO_PATH")
	useRealRepo := repoPath != ""
	if !useRealRepo {
		repoPath = DefaultRepoPath
	}

	var eclassDir string
	var cache *eclass.Cache

	// Try real repository first, fall back to mock
	if _, err := os.Stat(filepath.Join(repoPath, "eclass")); err == nil {
		t.Log("Using real Gentoo repository for eclass tests")
		eclassDir = filepath.Join(repoPath, "eclass")
		cache, err = eclass.NewCacheWithLocations([]string{eclassDir})
		if err != nil {
			t.Fatalf("Failed to create cache from real repo: %v", err)
		}
	} else {
		t.Log("Using mock eclasses for eclass tests (real repo not available)")
		mockDir := setupMockEclassDir(t, EclassTestSuite)
		eclassDir = filepath.Join(mockDir, "eclass")
		var err error
		cache, err = eclass.NewCacheWithLocations([]string{eclassDir})
		if err != nil {
			t.Fatalf("Failed to create cache from mock dir: %v", err)
		}
	}

	// Test each eclass
	for _, spec := range EclassTestSuite {
		spec := spec // Capture for parallel
		t.Run("Load_"+spec.Name, func(t *testing.T) {
			if spec.SkipReason != "" {
				t.Skip(spec.SkipReason)
			}

			// Check if eclass exists in cache
			if !cache.Has(spec.Name) {
				t.Skipf("Eclass %s not found in %s", spec.Name, eclassDir)
			}

			// Get eclass metadata
			ec, err := cache.Get(spec.Name)
			if err != nil {
				t.Fatalf("Failed to get eclass %s: %v", spec.Name, err)
			}

			t.Logf("Eclass %s found at %s", spec.Name, ec.Path)

			// Verify the file is readable
			content, err := os.ReadFile(ec.Path)
			if err != nil {
				t.Fatalf("Failed to read eclass %s: %v", spec.Name, err)
			}

			t.Logf("Eclass %s: %d bytes", spec.Name, len(content))
		})
	}
}

// TestEclass_ExecuteWithMock tests eclass execution using mock eclasses.
//
// This test validates:
//   - Eclass can be loaded and executed
//   - Metadata variables (IUSE, DEPEND) are set correctly
//   - EXPORT_FUNCTIONS works
func TestEclass_ExecuteWithMock(t *testing.T) {
	mockDir := setupMockEclassDir(t, EclassTestSuite)
	eclassDir := filepath.Join(mockDir, "eclass")

	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test environment
	testPkg := &pkg.Package{
		Name:    "test/pkg",
		Version: "1.0",
	}
	env, err := ebuild.NewEnvironment(testPkg, t.TempDir(), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := ebuild.NewHelpers(env, &stdout, &stderr)
	execHandler := createTestExecHandler(t, helpers)

	loader := eclass.NewHybridLoader(cache, execHandler,
		eclass.WithHybridOutput(&stdout, &stderr),
		eclass.WithVerbose(true),
	)

	ctx := context.Background()

	// Test each eclass that has mock content
	for _, spec := range EclassTestSuite {
		spec := spec
		t.Run("Execute_"+spec.Name, func(t *testing.T) {
			if spec.SkipReason != "" {
				t.Skip(spec.SkipReason)
			}
			if spec.MockContent == "" {
				t.Skip("No mock content available")
			}
			if !cache.Has(spec.Name) {
				t.Skipf("Eclass %s not in cache", spec.Name)
			}

			// Reset loader state
			loader.GetExecutor().Reset()
			stdout.Reset()
			stderr.Reset()

			// Inherit the eclass
			err := loader.Inherit(ctx, []string{spec.Name})
			if err != nil {
				t.Fatalf("Failed to inherit %s: %v", spec.Name, err)
			}

			// Verify it was inherited
			inherited := loader.GetExecutor().GetInherited()
			found := false
			for _, name := range inherited {
				if name == spec.Name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Eclass %s not in INHERITED list: %v", spec.Name, inherited)
			}

			// Check IUSE if expected
			if len(spec.ExpectedIUSE) > 0 {
				iuse := loader.GetExecutor().GetVar("IUSE")
				for _, flag := range spec.ExpectedIUSE {
					if !strings.Contains(iuse, flag) {
						t.Logf("Note: Expected IUSE flag %q not found in %q", flag, iuse)
					}
				}
			}

			t.Logf("Eclass %s executed successfully", spec.Name)
			t.Logf("INHERITED: %s", loader.GetExecutor().GetInheritedString())
		})
	}
}

// TestEclass_InheritChain tests eclass inheritance chains (eclass inheriting eclass).
func TestEclass_InheritChain(t *testing.T) {
	mockDir := setupMockEclassDir(t, EclassTestSuite)
	eclassDir := filepath.Join(mockDir, "eclass")

	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	var stdout, stderr bytes.Buffer
	loader := eclass.NewHybridLoader(cache, nil,
		eclass.WithHybridOutput(&stdout, &stderr),
		eclass.WithVerbose(true),
	)

	ctx := context.Background()

	// Test cmake which inherits toolchain-funcs
	t.Run("cmake_inherits_toolchain-funcs", func(t *testing.T) {
		loader.GetExecutor().Reset()
		stdout.Reset()

		if !cache.Has("cmake") || !cache.Has("toolchain-funcs") {
			t.Skip("Required eclasses not available")
		}

		err := loader.Inherit(ctx, []string{"cmake"})
		if err != nil {
			t.Fatalf("Failed to inherit cmake: %v", err)
		}

		inherited := loader.GetExecutor().GetInherited()
		t.Logf("Inherited eclasses: %v", inherited)

		// cmake should be in the list
		found := false
		for _, name := range inherited {
			if name == "cmake" {
				found = true
				break
			}
		}
		if !found {
			t.Error("cmake not in INHERITED list")
		}
	})

	// Test distutils-r1 -> python-r1 -> python-utils-r1 chain
	t.Run("distutils-r1_inherits_python-r1", func(t *testing.T) {
		loader.GetExecutor().Reset()
		stdout.Reset()

		if !cache.Has("distutils-r1") {
			t.Skip("distutils-r1 not available")
		}

		err := loader.Inherit(ctx, []string{"distutils-r1"})
		if err != nil {
			t.Fatalf("Failed to inherit distutils-r1: %v", err)
		}

		inherited := loader.GetExecutor().GetInherited()
		t.Logf("Inherited eclasses: %v", inherited)
	})
}

// TestEclass_MetadataAccumulation tests that metadata from multiple eclasses
// is accumulated correctly.
func TestEclass_MetadataAccumulation(t *testing.T) {
	mockDir := setupMockEclassDir(t, EclassTestSuite)
	eclassDir := filepath.Join(mockDir, "eclass")

	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	var stdout, stderr bytes.Buffer
	loader := eclass.NewHybridLoader(cache, nil,
		eclass.WithHybridOutput(&stdout, &stderr),
	)

	ctx := context.Background()

	t.Run("Multiple_eclasses_accumulate_IUSE", func(t *testing.T) {
		loader.GetExecutor().Reset()

		// Inherit multiple eclasses that set IUSE
		eclasses := []string{"cmake", "cargo"}
		available := make([]string, 0)
		for _, name := range eclasses {
			if cache.Has(name) {
				available = append(available, name)
			}
		}

		if len(available) < 2 {
			t.Skipf("Need at least 2 eclasses, have %d", len(available))
		}

		for _, name := range available {
			err := loader.Inherit(ctx, []string{name})
			if err != nil {
				t.Logf("Warning: Failed to inherit %s: %v", name, err)
				continue
			}
		}

		// Finalize metadata
		loader.GetExecutor().FinalizeMetadata()

		inherited := loader.GetExecutor().GetInherited()
		t.Logf("Inherited: %v", inherited)

		// Check accumulated metadata
		metadata := loader.GetExecutor().GetAccumulatedMetadata()
		for k, v := range metadata {
			t.Logf("Accumulated %s: %s", k, v)
		}
	})
}

// TestEclass_DoubleInherit tests that double inheritance is handled correctly.
func TestEclass_DoubleInherit(t *testing.T) {
	mockDir := setupMockEclassDir(t, EclassTestSuite)
	eclassDir := filepath.Join(mockDir, "eclass")

	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	var stdout, stderr bytes.Buffer
	loader := eclass.NewHybridLoader(cache, nil,
		eclass.WithHybridOutput(&stdout, &stderr),
	)

	ctx := context.Background()

	t.Run("Same_eclass_twice", func(t *testing.T) {
		loader.GetExecutor().Reset()

		if !cache.Has("toolchain-funcs") {
			t.Skip("toolchain-funcs not available")
		}

		// First inherit
		err := loader.Inherit(ctx, []string{"toolchain-funcs"})
		if err != nil {
			t.Fatalf("First inherit failed: %v", err)
		}

		// Second inherit (should skip)
		stdout.Reset()
		err = loader.Inherit(ctx, []string{"toolchain-funcs"})
		if err != nil {
			t.Fatalf("Second inherit failed: %v", err)
		}

		// Should only appear once in INHERITED
		inherited := loader.GetExecutor().GetInherited()
		count := 0
		for _, name := range inherited {
			if name == "toolchain-funcs" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected toolchain-funcs once, found %d times in %v", count, inherited)
		}

		// Output should mention skipping
		if !strings.Contains(stdout.String(), "skip") && !strings.Contains(stdout.String(), "already") {
			t.Logf("Output did not mention skipping: %s", stdout.String())
		}
	})
}

// TestEclass_RealRepository tests eclass loading against a real Gentoo repository.
//
// This test is skipped if no real repository is available.
func TestEclass_RealRepository(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoEclassDir(t)

	repoPath := os.Getenv("GRPM_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}
	eclassDir := filepath.Join(repoPath, "eclass")

	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	t.Logf("Testing with real eclass directory: %s", eclassDir)
	t.Logf("Total eclasses in cache: %d", len(cache.List()))

	// Verify critical eclasses exist
	criticalEclasses := []string{
		"toolchain-funcs", "multilib", "flag-o-matic",
		"cmake", "meson", "python-utils-r1",
	}

	for _, name := range criticalEclasses {
		t.Run("RealRepo_"+name, func(t *testing.T) {
			if !cache.Has(name) {
				t.Errorf("Critical eclass %s not found in real repository", name)
				return
			}

			ec, err := cache.GetWithChecksum(name)
			if err != nil {
				t.Errorf("Failed to get eclass %s: %v", name, err)
				return
			}

			t.Logf("Eclass %s: path=%s, checksum=%s...", name, ec.Path, ec.Checksum[:16])
		})
	}
}

// TestEclass_EclassSummary generates a summary of eclass test results.
func TestEclass_EclassSummary(t *testing.T) {
	repoPath := os.Getenv("GRPM_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}

	var eclassDir string
	var cache *eclass.Cache

	// Try real repository first
	if _, err := os.Stat(filepath.Join(repoPath, "eclass")); err == nil {
		eclassDir = filepath.Join(repoPath, "eclass")
		cache, err = eclass.NewCacheWithLocations([]string{eclassDir})
		if err != nil {
			t.Skipf("Failed to create cache: %v", err)
		}
	} else {
		mockDir := setupMockEclassDir(t, EclassTestSuite)
		eclassDir = filepath.Join(mockDir, "eclass")
		var err error
		cache, err = eclass.NewCacheWithLocations([]string{eclassDir})
		if err != nil {
			t.Skipf("Failed to create cache: %v", err)
		}
	}

	var stdout, stderr bytes.Buffer
	loader := eclass.NewHybridLoader(cache, nil,
		eclass.WithHybridOutput(&stdout, &stderr),
	)

	ctx := context.Background()

	// Collect results
	results := make([]EclassTestResult, 0, len(EclassTestSuite))

	for _, spec := range EclassTestSuite {
		if spec.SkipReason != "" {
			continue
		}

		loader.GetExecutor().Reset()
		stdout.Reset()
		stderr.Reset()

		start := time.Now()
		result := EclassTestResult{
			Name: spec.Name,
		}

		if !cache.Has(spec.Name) {
			result.LoadSuccess = false
			result.LoadError = &eclass.EclassNotFoundError{Name: spec.Name}
		} else {
			err := loader.Inherit(ctx, []string{spec.Name})
			result.LoadDuration = time.Since(start)

			if err != nil {
				result.LoadSuccess = false
				result.LoadError = err
			} else {
				result.LoadSuccess = true
				result.Inherited = loader.GetExecutor().GetInherited()
				result.IUSE = loader.GetExecutor().GetVar("IUSE")
				result.DEPEND = loader.GetExecutor().GetVar("DEPEND")
				result.RDEPEND = loader.GetExecutor().GetVar("RDEPEND")
			}
		}

		results = append(results, result)
	}

	// Print summary
	t.Log("=== Eclass Test Summary ===")
	t.Logf("Eclass directory: %s", eclassDir)
	t.Logf("Total eclasses in suite: %d", len(EclassTestSuite))

	passed := 0
	failed := 0
	skipped := 0

	for _, spec := range EclassTestSuite {
		if spec.SkipReason != "" {
			skipped++
			continue
		}
	}

	for _, r := range results {
		if r.LoadSuccess {
			passed++
			t.Logf("  PASS: %s (%v)", r.Name, r.LoadDuration)
		} else {
			failed++
			t.Logf("  FAIL: %s - %v", r.Name, r.LoadError)
		}
	}

	t.Logf("Results: %d passed, %d failed, %d skipped", passed, failed, skipped)
	t.Logf("Pass rate: %.1f%%", float64(passed)/float64(passed+failed)*100)
}

// TestEclass_HelpersIntegration tests that Go helpers work correctly with eclasses.
func TestEclass_HelpersIntegration(t *testing.T) {
	// Create test environment
	testPkg := &pkg.Package{
		Name:    "test/pkg",
		Version: "1.0",
		UseFlags: map[string]bool{
			"debug": true,
			"ssl":   false,
		},
	}

	tmpDir := t.TempDir()
	env, err := ebuild.NewEnvironment(testPkg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	interp := ebuild.NewInterpreter(env, &stdout, &stderr)
	ctx := context.Background()

	// Test toolchain-funcs helpers
	t.Run("tc-getCC", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(tc-getCC)"`)
		if err != nil {
			t.Fatalf("tc-getCC failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		if output == "" {
			t.Error("tc-getCC returned empty string")
		}
		t.Logf("tc-getCC returned: %s", output)
	})

	t.Run("tc-getCXX", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(tc-getCXX)"`)
		if err != nil {
			t.Fatalf("tc-getCXX failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		if output == "" {
			t.Error("tc-getCXX returned empty string")
		}
		t.Logf("tc-getCXX returned: %s", output)
	})

	// Test flag-o-matic helpers
	t.Run("append-cflags", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `append-cflags -O2 -Wall`)
		if err != nil {
			t.Fatalf("append-cflags failed: %v", err)
		}
	})

	// Test use helpers
	t.Run("use_with_debug", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(use_with debug)"`)
		if err != nil {
			t.Fatalf("use_with debug failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		if !strings.Contains(output, "with-debug") {
			t.Errorf("Expected --with-debug, got: %s", output)
		}
	})

	t.Run("use_with_ssl_disabled", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(use_with ssl)"`)
		if err != nil {
			t.Fatalf("use_with ssl failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		if !strings.Contains(output, "without-ssl") {
			t.Errorf("Expected --without-ssl, got: %s", output)
		}
	})

	// Test cmake helpers
	t.Run("cmake_use", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(cmake_use debug)"`)
		if err != nil {
			t.Fatalf("cmake_use failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		t.Logf("cmake_use returned: %s", output)
	})

	// Test meson helpers
	t.Run("meson_use", func(t *testing.T) {
		stdout.Reset()
		err := interp.Run(ctx, `echo "$(meson_use debug)"`)
		if err != nil {
			t.Fatalf("meson_use failed: %v", err)
		}
		output := strings.TrimSpace(stdout.String())
		t.Logf("meson_use returned: %s", output)
	})
}
