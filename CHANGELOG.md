# Changelog

All notable changes to GRPM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Performance optimization for large dependency graphs
- Web UI for daemon management
- Native GUI application ([gogpu/ui](https://github.com/gogpu/ui))

---

## [0.7.6] - 2026-01-13

### Critical Bug Fix: Version Selection

> **Fixes incorrect package version selection**
>
> GRPM was selecting `hello-2.12` instead of `hello-2.12.2` because versions
> were sorted alphabetically, not by Portage version comparison algorithm.

### Fixed

#### Version Selection Bug
- **Correct Portage version sorting** — LoadPackage now uses `pkg.CompareVersions()` for proper version ordering
  - Previously: `versions[len(versions)-1]` took last element alphabetically
  - Alphabetically: `"2.12.2" < "2.12"` (string comparison), selecting wrong version
  - Now: Sorts using PMS Chapter 3 version comparison algorithm
  - Example: `2.12.2 > 2.12 > 2.10 > 2.9` (correct numeric ordering)
  - Fixed in both `PortageRepository` and `CachedPortageRepository`

### Added

#### Comprehensive Version Selection Tests
- **New test file**: `internal/repo/version_selection_test.go`
  - `TestLoadPackage_SelectsHighestVersion` — 8 test cases covering:
    - Patch versions (`2.12` vs `2.12.2`)
    - Minor versions (`1.9` vs `1.10`)
    - Major versions (`2.0` vs `10.0`)
    - Suffix ordering (`1.0_alpha` < `1.0_beta` < `1.0_rc1` < `1.0`)
    - Revisions (`1.0` < `1.0-r1` < `1.0-r2`)
    - Letter suffixes (`1.0a` < `1.0b` < `1.0`)
    - Post-release patches (`1.0` < `1.0_p1` < `1.0_p2`)
  - `TestLoadPackage_SingleVersion` — Single ebuild handling
  - `TestLoadPackage_NoEbuilds` — Error handling for empty packages
  - `BenchmarkLoadPackage_ManyVersions` — Performance benchmark

### Changed
- `internal/repo/portage.go` — Version sorting before selection
- `internal/repo/cached_portage.go` — Version sorting + logging migration
- `internal/repo/cache/cached_repo.go` — Logging migration to `internal/logging`
- `internal/repo/manager.go` — Logging migration to `internal/logging`

### Technical Details
- Bug discovered during real Gentoo WSL2 testing
- Portage shows `hello-2.12.2`, GRPM was building `hello-2.12`
- Root cause: alphabetic sorting instead of version comparison
- Fix verified with comprehensive test suite
- Zero issues from `golangci-lint`

---

## [0.7.5] - 2026-01-13

### Build Process & Output Quality Release

> **Professional Output and Reliable Builds**
>
> This release focuses on two critical areas: fixing source build reliability
> and improving output quality to match Portage's professional appearance.

### Added

#### Unified Logging Infrastructure
- **Complete codebase logging refactor** — All output now uses `internal/logging/` package
  - Solver, CLI, and ebuild modules migrated from `log.Printf` to unified logging
  - Debug output suppressed by default (only shown with `-v` or `--verbose`)
  - Consistent Portage-style formatting across all commands
- **New package-level convenience functions**:
  - `logging.Verbose()` — Verbose-level messages
  - `logging.Debug()` — Debug-level messages (hidden by default)
  - `logging.Action()`, `logging.Info()`, `logging.Warn()`, `logging.Error()`

### Fixed

#### Source Build Reliability
- **Ebuild S variable parsing** — Custom source directories are now properly handled
  - Added `ParseSVariable()` with bash parameter expansion support
  - Supports `${var/pattern/replacement}`, `${var^}`, `${var%pattern}`, etc.
  - Enables packages like `screenfetch` with `S=${WORKDIR}/${PN/f/F}-${PV}`
  - Previously always used default `S=${WORKDIR}/${P}`, causing "no such file" errors
- **Tar/Zip timestamp preservation** — Archive extraction now preserves original file timestamps
  - Critical fix for automake-based packages (hello, glibc, etc.)
  - Added `os.Chtimes()` calls after file extraction
  - Directory timestamps restored in reverse order (deepest first)
  - Previously all files got current timestamp, causing `make` to trigger regeneration
  - Symptom: `aclocal-1.16: command not found` during build
- **Package replacement mode** — Replace mode now works correctly
  - Fixed collision detection to compare by package name (not full atom)
  - Files owned by same package (any version) no longer cause collisions
  - Added `findInstalledVersion()` to detect existing installations
  - When `--replace`/`-R` used: old package is unmerged before new merge
  - Implements Portage's "protect-owned" behavior
- **Work directory cleanup timing** — Fixed premature cleanup of build directory
  - Build directory now preserved until after installation completes
  - Cleanup happens in `installFromImageDir` after successful merge
  - Previously cleanup occurred immediately after `ExecutePhases`, before install

#### CLI Improvements
- **CLI --help exit code** — All CLI commands now return exit code 0 when using `--help`
  - Previously returned exit code 1 with "Error: flag: help requested"
  - Fixed in 13 places across 6 files (emerge, tools, fetch, analyze, commands, app)
  - Uses `errors.Is(err, flag.ErrHelp)` for proper error comparison
- **mirror:// URL expansion in emerge** — Emerge command now properly expands mirror:// URLs
  - Added SRC_URI parsing in ebuild executor (`internal/ebuild/executor.go`)
  - Uses `profiles/thirdpartymirrors` for URL expansion
  - Example: `mirror://gnu/hello/hello-2.12.tar.gz` → `https://ftp.gnu.org/gnu/hello/hello-2.12.tar.gz`
  - Previously only fetch command supported mirror expansion, now emerge does too
- **Ebuild phase function detection** — Custom ebuild functions are now properly detected
  - Added `ParseEbuild()` call in emerge command to populate `ParsedEbuild`
  - Enables proper dispatch to custom `src_configure`, `src_install`, etc. functions
  - Previously always used default phase implementations

### Changed
- `internal/solver/resolver.go` — Uses `logging.Info()` for resolution output
- `internal/cli/app.go` — Global logging level from `-v`/`-vv`/`-vvv` flags
- `internal/cli/emerge.go` — Full migration to unified logging
- `internal/cli/install_real.go` — Full migration to unified logging
- `internal/cli/analyze.go` — Full migration to unified logging
- `internal/cli/fetch.go` — Full migration to unified logging
- `internal/cli/commands.go` — Full migration to unified logging
- `internal/ebuild/executor.go` — Debug output via `logging.Debug()`
- `internal/ebuild/phases_impl.go` — Debug output via `logging.Debug()`
- `internal/repo/portage.go` — Debug output via `logging.Debug()`

### Technical Details
- Tested on real Gentoo WSL2: resolve, update, emerge, fetch, analyze, tools
- Zero issues from `golangci-lint` after refactoring
- All tests pass
- Output matches Portage's emerge format for familiarity

---

## [0.7.4] - 2026-01-13

### Critical Hotfix Release

> **Fixes for Real-World Gentoo Usage**
>
> Two critical bugs discovered during Gentoo WSL2 testing are now fixed:
> mirror:// URL expansion and rsync sync hanging.

### Fixed
- **mirror:// URL expansion** — Third-party mirror URLs are now properly expanded
  - Added `ThirdPartyMirrors` parser for `profiles/thirdpartymirrors` file
  - `mirror://gnu/`, `mirror://sourceforge/`, etc. now resolve to real HTTP URLs
  - Previously `grpm fetch` failed with "unsupported protocol scheme mirror"
  - Example: `mirror://gnu/hello/hello-2.12.tar.gz` now expands to
    `https://ftp.gnu.org/gnu/hello/hello-2.12.tar.gz` and fallback mirrors
- **rsync sync hanging** — Native Go rsync no longer hangs on real Gentoo mirrors
  - Removed debug code that limited sync to 10 files
  - Added timeout handling with connection deadline
  - Added fallback to system rsync binary if native implementation times out
  - Context cancellation now properly closes connection to unblock operations

### Added
- **Integration tests for fetch/SRC_URI parsing** — Validates v0.7.3 functionality
  - Tests for variable expansion (P, PN, PV, etc.)
  - Tests for USE flag conditionals (nil = include all)
  - Tests for arrow rename syntax
  - Tests against real Gentoo repository
- **Integration tests for config directories** — Validates directory handling
  - Tests for package.mask as directory (EAPI 7+)
  - Tests for package.use as directory
  - Tests for dotfiles/backups being ignored
  - Tests for subdirectories not being recursed
- **Integration tests for profile directories**
  - Tests for use.force/use.mask as directories
  - Tests for lexicographic file ordering (PMS compliance)
- **Integration tests for v0.7.4 fixes**
  - Tests for mirror:// URL expansion with real repository
  - Tests for rsync syncer timeout and fallback behavior

---

## [0.7.3] - 2026-01-13

### Portage Compatibility Release

> **Critical Fixes for Portage Compatibility**
>
> This is the definitive Portage compatibility release with code properly on main branch.
> Fixes two important issues discovered during real-world testing on Gentoo WSL2:
> proper handling of `package.mask` directories and signature file (`.asc`) fetching.

#### Fixed

##### package.mask Directory Handling (EAPI 7+)
- **Directory support** - `package.mask` can now be a directory containing multiple files
- **PMS-compliant file ordering** - Files sorted lexicographically (POSIX locale)
- **Skip patterns** - Dotfiles (`.hidden`) and backup files (`file~`) are ignored
- **Subdirectory handling** - Subdirectories are not recursed into (matches Portage behavior)
- Applied same fix to `package.use`, `package.accept_keywords`, and profile files

##### Signature File (.asc) Fetching
- **SRC_URI parsing for fetch** - Now parses ebuild's SRC_URI to get explicit URLs
- **USE conditional handling** - Passing `nil` for activeFlags now includes ALL conditionals
- **verify-sig support** - `.asc` files inside `verify-sig? ( ... )` are now properly extracted
- **Upstream URLs** - Signature files use explicit upstream URLs instead of trying Gentoo mirrors
- Example: `zlib-1.3.1.tar.xz.asc` now fetches from `https://zlib.net/` and GitHub

##### Documentation
- **README.md** - REST API endpoint corrected to Unix socket (`/var/run/grpm-rest.sock`)
- **ROADMAP.md** - Updated to reflect current state with 98.2% coverage

#### Changed
- `internal/config/config.go` - `loadPackageMask()` now supports file or directory
- `internal/profile/parser.go` - `parseListFile()` handles directory traversal
- `internal/repo/srcuri_parser.go` - `nil` activeFlags means "include all conditionals"
- `internal/cli/fetch.go` - `getDistfilesWithURIs()` parses SRC_URI from ebuild

#### Technical Details
- Verified against Portage source code (`lib/portage/util/__init__.py:grabfile`)
- Follows PMS specification for profile directory handling
- New test cases: `TestParseSrcURI_NilActiveFlags`, `TestParseSrcURI_FetchAllDistfiles`
- Tested on real Gentoo WSL2 installation with sys-libs/zlib

---

## [0.7.2] - 2026-01-13

### Superseded Release

> ⚠️ **This release was superseded by v0.7.3.**
>
> Released before PR was properly merged to main. While binaries contain the code,
> **please use v0.7.3** for proper git history and documentation.

---

## [0.7.0] - 2026-01-13

### Portage-Style Logging Release

> **Professional Output**
>
> This release introduces Portage-style logging with colored output,
> progress indicators, and professional formatting matching Gentoo's emerge output.

#### Added

##### Portage-Style Logging (`internal/logging/`)
- **Logger package** - Professional logging with Portage-style formatting
  - `>>>` prefix for actions (emerging, installing, syncing)
  - ` * ` prefix for informational messages (green)
  - ` ! ` prefix for warnings (yellow)
  - `!!!` prefix for errors (red)
- **ANSI color support** - Automatic terminal detection via `golang.org/x/term`
- **Log levels** - Quiet, Normal, Verbose, Debug
- **File logging** - Optional log file output (colors stripped automatically)
- **Specialized methods** for package management:
  - `Emerge(current, total, atom)` - Package emergence progress
  - `Installing(current, total, atom)` - Installation progress
  - `Syncing(repo)` - Repository sync start
  - `SyncComplete(duration, filesChanged)` - Sync completion
  - `Mirror(index, total, host)` - Mirror selection
  - `MirrorFailed(host, err)` - Mirror failure
  - `Retry(attempt, max, delay)` - Retry notification

##### Progress Indicators (`internal/logging/progress.go`)
- **Spinner** - Animated progress indicator with multiple styles
  - Styles: dots, line, circle, square, arrow, bounce
  - Configurable interval and message
- **ProgressBar** - Percentage-based progress bar
  - Customizable width and style
  - ETA calculation support
- **SyncProgress** - Repository sync-specific progress tracking

##### CLI Improvements
- **Verbose flag parsing** - `-v`, `-vv`, `-vvv` and `--verbose` support
- **Log level mapping** - Verbose levels map to logger levels automatically
- **Consistent output** - All CLI commands use unified logging

#### Changed
- `internal/cli/app.go` - Integrated Portage-style logger
- `internal/sync/rsync.go` - Uses new logging for sync output
- `cmd/grpm/main.go` - Improved verbose flag handling

#### Technical Details
- New dependency: `golang.org/x/term` for terminal detection
- Thread-safe logging with mutex protection
- Zero external dependencies for core logging (stdlib only)
- Verified against Portage reference (`lib/portage/output.py`)

---

## [0.6.0] - 2026-01-12

### Infrastructure & Quality Release

> **Production Readiness Focus**
>
> This release completes the infrastructure needed for real-world package building:
> automatic source downloading, comprehensive eclass testing, coverage analysis,
> and external tool detection.

#### Added

##### Distfile Fetching (v0.6.0-001)
- **`grpm fetch` command** - Standalone distfile downloading
  - `grpm fetch app-misc/hello` - Download sources for a package
  - `--pretend/-p` - Dry-run mode showing what would be downloaded
  - `--verify` - Verify existing distfiles without downloading
- **Automatic fetching in emerge** - Sources downloaded before build
- **Mirror support** - GENTOO_MIRRORS from make.conf
- **Resume support** - Partial downloads can be resumed
- **Checksum verification** - BLAKE2B, SHA512, SHA256 from Manifest

##### Debug Helpers (v0.6.0-002)
- `debug-print` - Debug output when PORTAGE_DEBUG=1 or GRPM_DEBUG=1
- `debug-print-function` - Log function entry points
- `debug-print-section` - Mark logical sections in execution
- PMS Section 12.3.16 compliant implementation

##### Eclass Integration Testing (v0.6.0-003)
- **`tests/integration/eclass_test.go`** - Comprehensive eclass test suite
- **21 eclasses tested** with mock content for CI
- Test categories: Load, Execute, Inherit Chain, Metadata Accumulation
- Build tag `integration` for conditional compilation
- Coverage: toolchain-funcs, cmake, meson, python-*, cargo, go-module, xdg, systemd, etc.

##### Coverage Analyzer (v0.6.0-004)
- **`grpm analyze` command** - Repository coverage analysis
  - `grpm analyze` - Analyze default Gentoo repository
  - `--repo/-r PATH` - Custom repository path
  - `--output/-o FORMAT` - Output: text, json, markdown
  - `--category/-c NAME` - Filter by category
  - `--verbose/-v` - Show per-package details
- **Analysis engine** (`internal/analyze/`)
  - EAPI validation (supports 0-8)
  - Eclass availability checking
  - Helper function coverage
  - Blocker categorization
- **Reports** - Actionable coverage reports with blocker breakdown

##### External Tool Detection (v0.6.0-005)
- **`grpm tools` command** - Tool availability management
  - `grpm tools` - List all known tools with status
  - `--check` - Summary of tool availability
  - `--missing` - Show missing tools with install hints
  - `--available` - Show available tools with paths
  - `--category NAME` - Filter by category (compilers, build, etc.)
  - `--for-eclass NAME` - Tools needed for specific eclass
- **Tool registry** (`internal/tools/`)
  - 50+ tools registered across 7 categories
  - Gentoo package suggestions for missing tools
  - Eclass-to-tool mappings
- **Pre-build validation** - Check required tools before emerge
  - `--skip-tool-check` flag to bypass validation

#### Technical Details
- New packages: `internal/analyze/`, `internal/tools/`
- New CLI commands: `fetch`, `analyze`, `tools`
- 70+ new tests across all features
- All code passes golangci-lint with 0 issues

---

## [0.5.2] - 2026-01-10

### Refactor: Dynamic Eclass Loading

> **Community Feedback Addressed**
>
> This release addresses criticism from the Gentoo community (forums.gentoo.org)
> about hardcoded eclass implementations. Eclasses are now loaded dynamically
> from the repository, matching Portage's behavior.

#### Added
- **Dynamic eclass loading** - Eclasses are now loaded from repository eclass/ directories
  - New `internal/eclass/` package (3300+ lines) for dynamic loading
  - `eclass.Cache` - Scans and caches eclass files with mtime tracking
  - `eclass.Executor` - Executes eclasses via mvdan.cc/sh interpreter
  - `eclass.HybridLoader` - Dynamic loading with Go fallbacks
  - `ebuild.DynamicEclassLoader` - Bridge for integration
  - `ebuild.SetupDynamicEclassLoading()` - Configuration helper

- **Metadata accumulation** - Proper handling of DEPEND, IUSE, RDEPEND from eclasses
  - Backup/restore of metadata variables during inherit
  - E_DEPEND, E_IUSE accumulator variables

- **EXPORT_FUNCTIONS support** - Phase function exports from eclasses

#### Changed
- `Executor` now uses dynamic eclass loading by default (`EnableDynamicEclass: true`)
- Go eclass implementations (cmake, meson, python, cargo, etc.) serve as fallbacks
- Added `EclassLocations` option for custom eclass search paths

#### Technical Details
- Uses mvdan.cc/sh bash interpreter for eclass execution
- Priority: dynamic loading → Go fallbacks
- Thread-safe with mutex protection
- Non-fatal fallback on cache creation errors

---

## [0.5.1] - 2026-01-10

### Hotfix: Multilib ABI Lookup

#### Fixed
- **Deterministic ABI lookup** - Fixed non-deterministic map iteration in multilib functions
  - `computeABILibdir`, `GetABIChost`, `GetABICflags`, `GetABILdflags`, `setupABIEnvironment`
  - Added `getCurrentArch()` to determine system architecture from CHOST
  - Now uses deterministic search order as fallback
  - Fixes CI test failures on Linux (TestComputeLibdir_WithABI, TestGetABICflags)

---

## [0.5.0] - 2026-01-10

### Language Ecosystems Release

> **Rapid Development Phase Complete**
>
> This release marks the end of the initial rapid development phase.
> GRPM now supports approximately 75% of the Gentoo package tree.
> Future development will focus on stability, testing, and community feedback.

This release adds comprehensive support for Python, Rust, and Go package ecosystems.

### Added

#### Python Eclasses (v0.5.0-001)
- `internal/ebuild/eclass_python.go` - Full Python eclass suite
- `python-utils-r1` - Core Python utilities
- `python-single-r1` - Single Python implementation packages
- `python-r1` - Multi-implementation packages
- `python-any-r1` - Build-time only Python dependency
- `distutils-r1` - Distutils/setuptools/flit/poetry build system
- PYTHON_COMPAT validation and USE flag generation
- `python_foreach_impl`, `python_setup`, `python_fix_shebang`

#### Package Sets (v0.5.0-002)
- `internal/state/sets.go` - Package set management
- @world - All explicitly installed packages
- @system - Essential system packages
- @selected - Packages in /var/lib/portage/world
- @preserved-rebuild - Packages needing rebuild
- Set operations: union, intersection, difference, expand

#### Multilib Eclass (v0.5.0-003)
- `internal/ebuild/eclass_multilib.go` - Core multilib support
- `internal/ebuild/eclass_multilib_build.go` - multilib-build eclass
- ABI types: amd64, x86, arm64, arm, ppc64
- `get_libdir`, `get_abi_CHOST`, `get_abi_CFLAGS`, `get_abi_LIBDIR`
- `multilib_foreach_abi`, `multilib_is_native_abi`
- CFLAGS -m32/-m64 handling

#### REQUIRED_USE Solver (v0.5.0-004)
- `internal/pkg/required_use.go` - Full REQUIRED_USE implementation
- Operators: `||` (any-of), `^^` (exactly-one), `??` (at-most-one)
- Conditionals: `flag?`, `!flag?`
- Complex nested expressions
- Automatic USE flag resolution

#### cargo.eclass (v0.5.0-005)
- `internal/ebuild/eclass_cargo.go` - Rust package support
- Crate URI generation for SRC_URI
- Crate vendoring from DISTDIR
- .cargo/config.toml generation
- CFLAGS to RUSTFLAGS conversion
- CHOST to Rust target triple conversion
- Full phase support: src_unpack, src_configure, src_compile, src_test, src_install

#### go-module.eclass (v0.5.0-006)
- `internal/ebuild/eclass_go_module.go` - Go package support
- EGO_SUM parsing and SRC_URI generation
- Go module cache setup
- Vendor directory support
- GOPROXY=off for offline builds
- CGO integration with system compilers
- Full phase support: src_unpack, src_compile, src_install
- `ego` wrapper for Go commands

### Changed
- Ebuild execution now supports Python, Rust, and Go build workflows
- ~75% estimated Gentoo package tree coverage (up from ~60%)

---

## [0.4.0] - 2026-01-10

### Build Systems Release

This release adds comprehensive build system support for CMake and Meson, covering approximately 60% of the Gentoo package tree.

### Added

#### CMake Build System (v0.4.0-001, v0.4.0-005)
- `internal/ebuild/build_cmake.go` - CMake execution engine
- `internal/ebuild/eclass_cmake.go` - Full cmake.eclass implementation
- Support for Ninja and Unix Makefiles generators
- `cmake_use`, `cmake_use_find_package` helpers
- `cmake_comment_add_subdirectory` for directory exclusion
- Full phase support: src_configure, src_compile, src_test, src_install
- CMAKE_MAKEFILE_GENERATOR auto-detection

#### Meson Build System (v0.4.0-002, v0.4.0-006)
- `internal/ebuild/build_meson.go` - Meson execution engine
- `internal/ebuild/eclass_meson.go` - Full meson.eclass implementation
- `meson_use`, `meson_feature` helpers for USE flag mapping
- Full phase support with ninja backend
- Cross-compilation support via meson cross files

#### toolchain-funcs Eclass (v0.4.0-003)
- `internal/ebuild/helpers_toolchain.go` - Toolchain detection functions
- Compiler detection: `tc-getCC`, `tc-getCXX`, `tc-getLD`, `tc-getAR`, `tc-getRANLIB`
- Cross-compilation: `tc-getBUILD_CC`, `tc-getBUILD_CXX`, `tc-is-cross-compiler`
- Architecture: `tc-arch`, `tc-arch-kernel`, `tc-endian`
- Toolchain export: `tc-export`, `tc-export_build_env`
- Compiler type detection: `tc-is-gcc`, `tc-is-clang`

#### flag-o-matic Eclass (v0.4.0-008)
- `internal/ebuild/eclass_flag_o_matic.go` - Flag manipulation with FlagSet value object
- Append operations: `append-cflags`, `append-cxxflags`, `append-cppflags`, `append-ldflags`, `append-flags`
- Filter operations: `filter-flags`, `filter-ldflags`, `filter-lfs-flags`, `filter-lto`
- Replace operations: `replace-flags`, `replace-cpu-flags`
- Strip operations: `strip-flags`, `strip-unsupported-flags`
- Test operations: `test-flags`, `test-flag-CC`, `test-flag-CXX`, `test-flags-CCLD`
- Utility: `get-flag`, `raw-ldflags`, `no-as-needed`, `is-ldflagq`
- Glob pattern matching for flag filtering

#### Repository Cache (v0.4.0-004)
- `internal/cache/` - Core cache package with pluggable backends
  - `cache.go` - Cache interface and factory
  - `sqlite.go` - SQLite backend with WAL mode (modernc.org/sqlite, pure Go)
  - `memory.go` - In-memory LRU cache backend
  - `entry.go` - Cache entry types with mtime validation
- `internal/repo/cache/` - Repository-specific cache wrapper
  - `cache.go` - RepoCache with automatic invalidation
  - `index.go` - Directory indexer for fast package lookups
  - `cached_repo.go` - CachedRepository wrapper
- `internal/repo/cached_portage.go` - Cached PortageRepository integration
- Thread-safe concurrent access with prepared statements
- Automatic stale entry invalidation on ebuild modification

#### Integration Tests (v0.4.0-007)
- `tests/integration/framework.go` - Test framework (504 lines)
- `tests/integration/autotools_test.go` - 10 autotools packages (383 lines)
- `tests/integration/cmake_test.go` - 15 CMake packages (485 lines)
- `tests/integration/meson_test.go` - 15 Meson packages (523 lines)
- `.github/workflows/integration.yml` - CI workflow for Gentoo container
- Total: 2768 lines of integration tests
- Build tag: `//go:build integration` for conditional compilation

### Changed
- Ebuild execution now supports autotools, CMake, and Meson workflows
- Repository operations can use cache for faster metadata access

---

## [0.3.0] - 2026-01-10

### PMS Compliance Release

This release brings comprehensive PMS (Package Manager Specification) compliance, EAPI 0-8 feature matrix, and direct mvdan.cc/sh integration for bash compatibility.

### Added

#### PMS-Compliant Atom Parser (v0.3.0-001)
- Character-by-character parser for package atoms per PMS Section 8.3
- All version operators: `=`, `>=`, `>`, `<=`, `<`, `~`, `=*` (glob)
- Blocker support: `!` (weak), `!!` (strong)
- Slot dependencies: `:slot`, `:slot/subslot`, `:*`, `:=`
- Repository constraints: `::repo-name`
- USE dependencies: `[flag,-flag,flag?,flag(+),flag(-)]`
- `ParseAtom()` function with comprehensive validation
- `Atom.Matches(*Package)` for package matching
- `Atom.ToConstraint()` for solver integration

#### Full EAPI 8 Support (v0.3.0-002)
- **REQUIRED_USE validator** - Full expression parsing (||, ^^, ??, flag?, !flag?, conditionals)
- **SRC_URI enhancements** - Arrow syntax (`-> filename`), USE-conditional downloads
- **USE flag helpers** - `use_enable`, `use_with` (1-4 argument modes per PMS 11.3.2)
- **EAPI 8 installation helpers** - `dosym -r` (relative symlinks), `dostrip`, `einstalldocs`
- **New ebuild variables** - IDEPEND, PROPERTIES, RESTRICT parsing
- **Atom parser integration** - `FindByAtom()` in Repository, `AddAtomConstraint()` in SAT solver

#### EAPI Feature Matrix (v0.3.0-004)
- `EAPIFeatures` struct with 30+ feature flags for EAPI-dependent behavior
- `GetEAPIFeatures()`, `IsEAPISupported()`, `ValidateEAPI()` functions
- Complete EAPI 0-8 coverage per PMS Appendix A
- Feature flags: BDEPEND, IDEPEND, SlotOperators, Nonfatal, OffsetPrefix, etc.
- Trailing slash behavior per EAPI (0-6 has trailing, 7+ doesn't)

#### Version Manipulation Commands (v0.3.0-005)
- `ver_test` - Version comparison per PMS Algorithm 3.2-3.7
- `ver_cut` - Extract version components
- `ver_rs` - Replace version separators

#### PMS Environment Variables (v0.3.0-006)
- `PVR` - Package version with revision (per PMS 11.1)
- `ROOT`, `EROOT` - Target filesystem root with EAPI-specific trailing slash
- `SYSROOT`, `ESYSROOT` - Cross-compilation support (EAPI 7+)
- `BROOT` - Build dependencies root (EAPI 7+)
- `EPREFIX` - Gentoo Prefix support (EAPI 3+)

#### Error Handling Commands (v0.3.0-007, v0.3.0-008)
- `assert` - Check PIPESTATUS array, die if non-zero (per PMS 12.3.6)
- `nonfatal` - Suppress die in called command (EAPI 4+, per PMS 12.3.8)
- Exit status tracking (`lastExitStatus`, `pipeStatus`)

#### Missing Phase Functions (v0.3.0-009)
- `pkg_pretend` - Pre-merge sanity checks (EAPI 4+)
- `pkg_config` - Post-install configuration
- `pkg_info` - Package information display
- `pkg_nofetch` - Fetch restriction handling

#### Installation Helpers (v0.3.0-011, v0.3.0-012, v0.3.0-013)
- `into` - Set installation destination prefix
- `doinfo` - Install GNU info files to `/usr/share/info`
- `domo` - Install gettext message catalogs to `/usr/share/locale`

#### Banned Command Validation (v0.3.0-014)
- `IsBannedCommand()`, `GetBannedCommands()` for EAPI-specific validation
- Stub implementations with deprecation warnings: `dohard`, `dosed`, `useq`, `einstall`, `dohtml`, `dolib`, `libopts`, `hasv`, `hasq`
- EAPI-aware command availability per PMS Chapter 12

#### Default Phase Functions (v0.3.0-015)
- `default_src_unpack` - Default unpack implementation
- `default_src_prepare` - Default prepare (eapply_user in EAPI 6+)
- `default_src_configure` - Default configure (econf)
- `default_src_compile` - Default compile (emake or nothing)
- `default_src_test` - Default test (emake check/test)
- `default_src_install` - Default install (emake DESTDIR or einstalldocs)
- `default_pkg_nofetch` - Default fetch restriction message
- `default` dispatcher function

#### EAPI Validation (v0.3.0-016)
- `ValidateEAPI()` with detailed error messages
- Pre-execution EAPI compatibility checks
- Graceful degradation for unknown EAPIs

#### PMS Compliance Test Suite (v0.3.0-017)
- 1076 lines of comprehensive PMS compliance tests
- Version comparison tests (40+ cases per PMS Algorithm 3.2-3.7)
- Dependency atom parsing tests
- EAPI feature availability tests
- Located in `tests/pms_compliance_test.go`

### Changed

#### Bash Compatibility
- **mvdan.cc/sh direct integration** - GoSh wrapper NOT needed
- Direct use of mvdan.cc/sh/v3 for bash parsing and execution
- Full compatibility with Portage bash requirements (EAPI 0-6: Bash 3.2, EAPI 7: Bash 4.2, EAPI 8: Bash 5.0)

#### Test Infrastructure
- Split `helpers_test.go` (4717 lines) into 12 focused test files
- Improved test organization and maintainability
- Shared test utilities in `helpers_test.go`

### Documentation

- Added PMS Chapter 6 - Ebuild File Format (`docs/pms/chapter6-ebuild-format.md`)
- Updated PMS documentation index

---

## [0.2.1] - 2026-01-09

### Fixed

#### PMS-Compliant Version Comparison (v0.2.1-001, v0.2.1-002, v0.2.1-003)

Critical bug fixes for version comparison per PMS Chapter 3.2-3.3:

- **Version suffix ordering** - Implemented PMS Algorithm 3.5-3.6:
  - Correct: `_alpha < _beta < _pre < _rc < (release) < _p`
  - Previously: Used alphabetical comparison (`_p < _pre < _rc`)
  - **Impact**: Fixes incorrect package version selection during dependency resolution

- **Leading zero handling** - Implemented PMS Algorithm 3.3:
  - Components with leading zeros compared lexicographically
  - Trailing zeros stripped before comparison

- **Letter suffix parsing** - Implemented PMS Section 3.2:
  - Single letter suffix (`1.0a`, `1.0b`) parsed separately
  - Correct ordering: `1.0a < 1.0b < 1.0z < 1.0` (letter < no letter)

- **Revision handling** - `-r0` now equals no revision per PMS

### Changed

- `Version` struct refactored to rich domain model with:
  - `VersionComponent` - numeric parts with leading zero tracking
  - `VersionSuffix` - suffix type and number
  - `letterSuffix` field for single letter
  - `revision` field for `-rN`
- Comprehensive test suite for PMS compliance (30+ new test cases)

---

## [0.2.0] - 2026-01-09

### Ebuild Parser Improvements

This release focuses on improving ebuild parsing with proper variable expansion and removing hardcoded eclass handling.

### Added

#### Package Variable Expansion (v0.2.0-003)
- `PackageMetadata` struct for PMS 11.1 variables (P, PN, PV, PR, PVR, PF, CATEGORY)
- `NewPackageMetadata()` constructor for building metadata from package info
- `NewEbuildParserWithMetadata()` for creating parser with variable expansion context
- Proper `${P}`, `${PN}`, `${PV}`, `${PVR}`, `${PF}`, `${CATEGORY}` expansion in ebuild content
- Exported `ExtractVariable()` method for public access

#### REST API Improvements (v0.2.0-001)
- Unix socket support for daemon communication
- Improved socket handling and error messages

#### xargs Helper (v0.2.0-004)
- Native `xargs` implementation for ebuild helpers
- Cross-platform support without external dependencies

### Changed

#### Eclass Loading (v0.2.0-005)
- **BREAKING**: Removed `handleBuiltinEclass()` function
- All eclasses now loaded exclusively from repository
- Go helper implementations (tc-getCC, etc.) registered in interpreter
- More accurate Portage behavior emulation

### Fixed
- Eclass loading now correctly sources from repository filesystem
- Variable expansion works in DEPEND, RDEPEND, SRC_URI, SLOT, etc.
- Tests updated to use real eclass files instead of builtin mocks

---

## [0.1.1] - 2026-01-09

### Module Architecture Improvements

Major refactoring and improvements to the ebuild execution module.

### Changed

#### Code Architecture
- Split monolithic `helpers.go` (3200+ lines) into 11 focused modules:
  - `helpers_build.go` - Build commands (emake, econf)
  - `helpers_install.go` - Installation helpers (dobin, doins, dosym, fowners, fperms)
  - `helpers_fs.go` - Filesystem utilities
  - `helpers_msg.go` - Messaging (einfo, ewarn, die)
  - `helpers_use.go` - USE flag functions
  - `helpers_toolchain.go` - Toolchain detection
  - `helpers_version.go` - Version manipulation
  - `helpers_patch.go` - Patching functions
  - `helpers_doc.go` - Documentation helpers
  - `helpers_unpack.go` - Archive extraction
  - `helpers_default.go` - Default phase implementations
- Platform-specific code isolation (`helpers_chown_unix.go`, `helpers_chown_stub.go`)

#### Ebuild Execution
- Real eclass `inherit` with `EXPORT_FUNCTIONS` support
- Proper phase dispatch to custom ebuild functions (no more hardcoded defaults)
- Hook phases (pkg_preinst, pkg_postinst, pkg_prerm, pkg_postrm) working correctly

#### Installation Helpers
- Real `fowners` implementation with chown on Unix systems
- User/group lookup via `os/user` package
- Recursive ownership changes with `-R` flag

#### Package Queries
- Enhanced `has_version` with real VarDB queries
- Enhanced `best_version` with real VarDB queries
- Proper version constraint matching

#### Binary Packages
- Improved GPKG metadata parsing per GLEP 78
- SHA256 checksum generation
- Fixed test fixtures with valid nested tar archives

### Added
- `internal/ebuild/eclass.go` - Eclass registry, loader, stack management
- `internal/ebuild/script.go` - Ebuild script parsing for phase detection
- `internal/ebuild/phase_dispatch_test.go` - Phase dispatch tests
- `tests/integration/v0.2.0_integration_test.go` - Comprehensive integration tests

### Fixed
- Phase dispatch now calls custom ebuild functions instead of hardcoded defaults
- Binary package tests use valid nested metadata.tar archives
- Test fixtures properly structured for GPKG format

---

## [0.1.0] - 2026-01-08

### Initial Public Release

GRPM (Go Resource Package Manager) is a modern reimplementation of Gentoo's Portage package manager in pure Go.

### Added

#### Core Features
- SAT-based dependency resolution using gophersat library
- Domain-Driven Design architecture with rich domain model
- Gentoo version comparison and constraint handling
- Slot and subslot support with ABI tracking
- USE flag resolution and conditional dependencies
- Circular dependency detection
- Slot and version conflict resolution

#### Package Management
- Binary package support (GPKG and TBZ2 formats)
- Package installation with collision detection
- Package removal with dependency tracking
- VarDB integration (Gentoo `/var/db/pkg` format)
- Package signing (GPG/SSH/RSA)
- Local and remote binhost support

#### Source Building
- Full ebuild execution engine
- PMS phase implementation (setup through install)
- Autotools workflow support (configure/make/make install)
- Parallel compilation support

#### System Integration
- Profile system with inheritance
- Configuration management (make.conf, package.*)
- Metadata caching (SQLite backend)
- CONFIG_PROTECT support
- Privilege dropping (userpriv/userfetch)
- Virtual package support
- Multiple repository/overlay support

#### Daemon & API
- Single binary with CLI and daemon modes
- gRPC service on Unix socket
- REST API for monitoring and integration
- Job queue with worker pool
- Parallel build scheduler

#### Repository Management
- Native rsync implementation
- Git sync with GPG verification
- Auto-selection strategy
- Pluggable sync module

#### Advanced Features
- Slot collision resolution with autounmask
- @world/@system/@selected package sets
- System update with --deep/--newuse flags
- Dependency cleanup (depclean)
- Parallel package builds (--jobs)

### CLI Commands

| Command | Description |
|---------|-------------|
| `resolve` | Resolve dependencies with SAT solver |
| `install` | Install packages (binary or source) |
| `emerge` | Build packages from source |
| `remove` | Remove installed packages |
| `search` | Search for packages |
| `info` | Display package information |
| `sync` | Synchronize repository |
| `update` | Update @world/@system packages |
| `depclean` | Remove orphaned packages |
| `build` | Create binary packages |
| `status` | Show daemon status |
| `daemon` | Start daemon mode |

### Technical Details

- **Language**: Go 1.25+
- **License**: Apache-2.0
- **Platforms**: Linux (amd64, arm64, armv7, armv6, i386)
- **Test Coverage**: ~70%
- **Total Code**: ~60,000 lines

### Known Limitations

- Ebuild execution limited to autotools workflow
- Limited eclass support (toolchain-funcs, eutils, multilib)
- No EAPI 8 features yet
- CMake/Meson not supported

### Dependencies

- github.com/crillab/gophersat - SAT solver
- github.com/spf13/cobra - CLI framework
- google.golang.org/grpc - gRPC server
- github.com/gokrazy/rsync - Native rsync
- modernc.org/sqlite - Pure Go SQLite
- mvdan.cc/sh/v3 - Bash interpreter

---

## Links

- **Repository**: https://github.com/grpmsoft/grpm
- **Documentation**: https://github.com/grpmsoft/grpm/tree/master/docs
- **Issues**: https://github.com/grpmsoft/grpm/issues
- **License**: [Apache-2.0](LICENSE)

[Unreleased]: https://github.com/grpmsoft/grpm/compare/v0.7.5...HEAD
[0.7.5]: https://github.com/grpmsoft/grpm/compare/v0.7.4...v0.7.5
[0.7.4]: https://github.com/grpmsoft/grpm/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/grpmsoft/grpm/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/grpmsoft/grpm/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/grpmsoft/grpm/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/grpmsoft/grpm/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/grpmsoft/grpm/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/grpmsoft/grpm/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/grpmsoft/grpm/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/grpmsoft/grpm/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/grpmsoft/grpm/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/grpmsoft/grpm/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/grpmsoft/grpm/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/grpmsoft/grpm/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/grpmsoft/grpm/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/grpmsoft/grpm/releases/tag/v0.1.0
