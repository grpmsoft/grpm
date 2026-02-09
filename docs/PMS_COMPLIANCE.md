# PMS Compliance Matrix

> **GRPM Implementation Status per [Package Manager Specification](https://projects.gentoo.org/pms/)**
>
> **Version:** v0.9.4
> **Last Updated:** 2026-02-09
> **License:** This document follows PMS under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/)

---

## Status Legend

| Status | Description |
|--------|-------------|
| **Full** | Fully implemented per PMS specification |
| **Partial** | Core functionality works, some edge cases or features missing |
| **Minimal** | Basic support only, significant work remaining |
| **Not Yet** | Not implemented |
| **N/A** | Not applicable to GRPM architecture |

---

## Executive Summary

| Chapter | Coverage | Notes |
|---------|----------|-------|
| Ch. 1: Introduction | N/A | Informational only |
| Ch. 2: EAPIs | **Full** | EAPI 0-8 feature matrix complete |
| Ch. 3: Names and Versions | **Full** | PMS-compliant version comparison |
| Ch. 4: Repository Layout | **Full** | Portage tree structure supported |
| Ch. 5: Profiles | **Partial** | Profile loading works, some features missing |
| Ch. 6: Ebuild File Format | **Partial** | Parsing via mvdan.cc/sh with hardened workarounds |
| Ch. 7: Ebuild Variables | **Full** | All mandatory/optional variables parsed |
| Ch. 8: Dependencies | **Full** | All operators, slots, USE deps supported |
| Ch. 9: Phase Functions | **Partial** | Core phases work, hardened metadata extraction |
| Ch. 10: Eclasses | **Partial** | 14 eclass modules, dynamic loading, BASH_VERSINFO emulation |
| Ch. 11: Environment | **Full** | All core variables, CHOST/CBUILD, USE_EXPAND |
| Ch. 12: Commands | **Partial** | 214 helpers, 159 routed commands |

**Overall Estimate:** ~80% PMS compliance for common use cases.

---

## Chapter 2: EAPIs

**Status: Full**

GRPM maintains a complete EAPI feature matrix in `internal/pkg/eapi.go`.

### EAPI Support Matrix

| EAPI | Status | Key Features |
|------|--------|--------------|
| 0 | Full | Basic ebuild support |
| 1 | Full | Slot dependencies, IUSE defaults |
| 2 | Full | USE deps, SRC_URI arrows, src_prepare/src_configure |
| 3 | Full | Offset-prefix (EPREFIX, EROOT, ED) |
| 4 | Full | REQUIRED_USE, pkg_pretend, USE dep defaults |
| 5 | Full | Slot operators (:=, :*), subslots, usev/usex |
| 6 | Full | eapply, eapply_user, einstalldocs, Bash 4.2 |
| 7 | Full | BDEPEND, SYSROOT/BROOT, no trailing slashes |
| 8 | Full | IDEPEND, dosym -r, SRC_URI selective restrictions |

### Implementation

```
internal/pkg/eapi.go        # EAPIFeatures struct with 40+ feature flags
internal/pkg/eapi_test.go   # Comprehensive EAPI feature tests
```

---

## Chapter 3: Names and Versions

**Status: Full**

### Section 3.1: Name Restrictions

| Entity | Validation | Status |
|--------|------------|--------|
| Category names | `[A-Za-z0-9+_.-]`, no leading `-./+` | Full |
| Package names | `[A-Za-z0-9+_-]`, no leading `-+` | Full |
| Slot names | `[A-Za-z0-9+_.-]`, no leading `-./+` | Full |
| USE flag names | `[A-Za-z0-9+_@-]`, alphanumeric start | Full |
| Repository names | `[A-Za-z0-9_-]`, valid package name | Full |
| Eclass names | `[A-Za-z0-9_.-]`, letter/underscore start | Full |
| EAPI names | `[A-Za-z0-9+_.-]`, no leading `-./+` | Full |

### Section 3.2-3.3: Version Specification and Comparison

| Feature | Status | Notes |
|---------|--------|-------|
| Numeric components `[0-9]+(\.[0-9]+)*` | Full | |
| Letter suffix `[a-z]` | Full | |
| Suffixes `_alpha`, `_beta`, `_pre`, `_rc`, `_p` | Full | With optional integer |
| Revision `-r[0-9]+` | Full | Defaults to -r0 |
| PMS Algorithm 3.1-3.7 | Full | All 7 algorithms implemented |
| Leading zero handling (Algorithm 3.3) | Full | ASCII stringwise comparison |

### Implementation

```
internal/pkg/version.go        # Version struct and CompareTo()
internal/pkg/constraint.go     # CompareVersions(), version parsing
internal/pkg/atom.go           # IsValidCategory(), IsValidPackageName()
internal/pkg/version_test.go   # PMS compliance tests
```

---

## Chapter 4: Repository Layout

**Status: Full**

### Section 4.1-4.3: Tree Structure

| Component | Status | Notes |
|-----------|--------|-------|
| `category/package/` structure | Full | |
| `category/package/*.ebuild` | Full | |
| `category/package/Manifest` | Full | GLEP 74 parsing |
| `category/package/metadata.xml` | Full | |
| `category/package/files/` | Full | |
| `profiles/` directory | Full | |
| `eclass/` directory | Full | Dynamic loading |
| `licenses/` directory | Partial | Existence checked, not validated |
| `metadata/` directory | Full | |

### Section 4.4: Mirrors

| Feature | Status | Notes |
|---------|--------|-------|
| `mirror://` URI expansion | Full | |
| `thirdpartymirrors` parsing | Full | |
| Mirror failover | Full | Reliability-based selection |

### Implementation

```
internal/repo/portage.go       # Repository structure
internal/fetch/manifest.go     # GLEP 74 Manifest parsing
internal/fetch/mirror.go       # Mirror selection
```

---

## Chapter 5: Profiles

**Status: Partial**

### Profile Features

| Feature | Status | Notes |
|---------|--------|-------|
| Profile inheritance (`parent`) | Full | Multi-parent support |
| `make.defaults` | Full | Variable expansion |
| `package.mask` | Full | v0.8.1 |
| `package.unmask` | Full | v0.8.1 |
| `package.use` | Full | v0.8.0, atom specificity matching |
| `package.use.force` | Partial | |
| `package.use.mask` | Partial | |
| `use.force` | Full | Profile-wide forced USE flags |
| `use.mask` | Full | Profile-wide masked USE flags |
| `package.provided` | Not Yet | |
| `packages` (system set) | Full | System package list loaded |
| `profile.bashrc` | Not Yet | |
| EAPI 7+ directory-based configs | Full | |

### Implementation

```
internal/profile/profile.go   # Profile loading
internal/profile/parser.go    # Profile file parsing
internal/mask/mask.go         # MaskManager
internal/config/config.go     # make.conf parsing with variable expansion
```

---

## Chapter 6: Ebuild File Format

**Status: Partial**

### Section 6.1: Bash Version

| EAPI | Required Bash | GRPM Status |
|------|---------------|-------------|
| 0-5 | 3.2 | N/A (Go interpreter) |
| 6-7 | 4.2 | N/A (Go interpreter) |
| 8+ | 5.0 | N/A (Go interpreter) |

GRPM uses `mvdan.cc/sh` as a Go-native bash interpreter with hardened workarounds.

### Section 6.2: Encoding and Format

| Feature | Status | Notes |
|---------|--------|-------|
| UTF-8 encoding | Full | |
| EAPI line detection | Full | Regex per PMS |
| Failglob (EAPI 6+) | Partial | Interpreter limitation |
| Umask | Full | Set in environment |

### Bash Interpreter Hardening (v0.9.4)

GRPM uses `mvdan.cc/sh` (pure Go) with the following workarounds for known limitations:

| Limitation | Workaround | Status |
|------------|-----------|--------|
| `type -P` not implemented | Preprocessed to `command -v` | Full |
| `declare -f` / `declare -p` | Custom `__grpm_has_func` / `__grpm_has_var` | Full |
| `${var@a}` param attributes | `BASH_VERSINFO=(4...)` forces bash 4 code paths | Full |
| Brace expansion in var names | `stripFunctionBodies()` removes phase function bodies | Full |
| Process substitution `>()` | Override `multibuild_foreach_variant` | Partial |
| `read -a` (array read) | Not yet worked around | Not Yet |
| Extended globbing `!(pat)` | Warnings suppressed, non-fatal | Partial |
| Eclass stdout pollution | `>/dev/null` redirect during sourcing | Full |

### Metadata Extraction

| Feature | Status | Notes |
|---------|--------|-------|
| SRC_URI evaluation via bash | Full | With eclass support |
| stripFunctionBodies | Full | Removes phase functions before parsing |
| Raw SRC_URI text extraction | Full | Fallback for interpreter failures |
| Signature file filtering | Full | .sig/.asc/.sign filtered when verify-sig disabled |
| Multi-variable extraction | Full | DEPEND, RDEPEND, IUSE, etc. |

### Implementation

```
internal/ebuild/metadata.go     # Metadata evaluator with stripFunctionBodies
internal/ebuild/interpreter.go  # mvdan.cc/sh wrapper with exec handler
internal/distfile/service.go    # Distfile resolution with 3-layer defense
```

---

## Chapter 7: Ebuild Variables

**Status: Full**

### Section 7.2: Mandatory Variables

| Variable | Status | Notes |
|----------|--------|-------|
| DESCRIPTION | Full | Parsed from ebuild |
| SLOT | Full | Including subslots |

### Section 7.3: Optional Variables

| Variable | Status | Notes |
|----------|--------|-------|
| EAPI | Full | 0-8 supported |
| HOMEPAGE | Full | |
| SRC_URI | Full | Arrows, USE conditionals, mirror://, verify-sig filtering |
| LICENSE | Full | |
| KEYWORDS | Full | v0.8.1 filtering |
| IUSE | Full | With defaults (+/-) |
| REQUIRED_USE | Full | ^^, ??, || operators |
| PROPERTIES | Full | |
| RESTRICT | Full | fetch, mirror, strip, etc. |
| DEPEND | Full | |
| BDEPEND | Full | EAPI 7+ |
| RDEPEND | Full | |
| PDEPEND | Full | |
| IDEPEND | Full | EAPI 8+ |

### Implementation

```
internal/repo/ebuild_parser.go  # Variable extraction
internal/pkg/package.go         # Package struct
```

---

## Chapter 8: Dependencies

**Status: Full**

### Section 8.2: Dependency Syntax

| Syntax | Status | Notes |
|--------|--------|-------|
| All-of groups `( )` | Full | |
| Any-of groups `\|\| ( )` | Full | |
| Exactly-one-of `^^ ( )` | Full | |
| At-most-one-of `?? ( )` | Full | EAPI 5+ |
| USE conditionals `use? ( )` | Full | |
| Negated conditionals `!use? ( )` | Full | |

### Section 8.3: Package Dependency Specs

| Feature | Status | Notes |
|---------|--------|-------|
| Simple `category/package` | Full | |
| Version operators `<`, `<=`, `=`, `~`, `>=`, `>` | Full | |
| Glob match `=cat/pkg-1.2*` | Full | |
| Weak blocker `!cat/pkg` | Full | |
| Strong blocker `!!cat/pkg` | Full | |
| Slot deps `:slot` | Full | |
| Slot operators `:=`, `:*`, `:slot=` | Full | EAPI 5+ |
| Subslots `:slot/subslot` | Full | EAPI 5+ |
| USE deps `[flag]`, `[-flag]` | Full | |
| USE dep defaults `[flag(+)]`, `[flag(-)]` | Full | EAPI 4+ |

### Implementation

```
internal/pkg/atom.go           # Atom parsing
internal/pkg/constraint.go     # Version constraints
internal/solver/resolver.go    # Dependency resolution
internal/solver/gophersat_adapter.go  # SAT encoding
```

---

## Chapter 9: Phase Functions

**Status: Partial**

### Phase Execution Order

| Phase | Status | Notes |
|-------|--------|-------|
| pkg_pretend | Partial | Called but limited checks |
| pkg_setup | Full | |
| src_unpack | Full | 11 archive formats including .tar.lz |
| src_prepare | Full | eapply_user handling |
| src_configure | Full | econf with ECONF_SOURCE, CHOST, CBUILD |
| src_compile | Partial | Simple builds work |
| src_test | Partial | When --test flag used |
| src_install | Partial | Basic installation works |
| pkg_preinst | Full | |
| pkg_postinst | Full | |
| pkg_prerm | Full | |
| pkg_postrm | Full | |
| pkg_config | Partial | |
| pkg_info | Partial | |
| pkg_nofetch | Full | Default implementation per PMS 9.1.16 |

### Default Phase Implementations

| EAPI | default_src_prepare | Status |
|------|---------------------|--------|
| 0-1 | No-op | Full |
| 2-5 | No-op | Full |
| 6-8 | eapply_user | Full |

| EAPI | default_src_install | Status |
|------|---------------------|--------|
| 0-3 | No-op | Full |
| 4-5 | emake DESTDIR install, einstalldocs | Partial |
| 6-8 | Same as EAPI 4 | Partial |

### Unpack Format Support (v0.9.4)

| Format | Implementation | Notes |
|--------|---------------|-------|
| `.tar.gz` / `.tgz` | Pure Go (compress/gzip) | |
| `.tar.bz2` / `.tbz2` | Pure Go (compress/bzip2) | |
| `.tar.xz` / `.txz` | Pure Go (ulikunitz/xz) | |
| `.tar.lz` | External (xz/plzip/lzip) | v0.9.4: Portage unpacker.eclass strategy |
| `.tar.zst` / `.tar.zstd` | Pure Go (klauspost/zstd) | |
| `.tar` | Pure Go (archive/tar) | |
| `.zip` | Pure Go (archive/zip) | |
| `.gz`, `.bz2`, `.xz`, `.zst` | Pure Go | Single-file decompression |
| Non-archive files | Copy to WORKDIR | Per PMS spec |

### Implementation

```
internal/ebuild/phases.go       # Phase definitions
internal/ebuild/phases_impl.go  # Phase implementations
internal/ebuild/executor.go     # Phase execution with eclass workarounds
internal/ebuild/helpers_unpack.go  # 11 archive formats
```

---

## Chapter 10: Eclasses

**Status: Partial**

### Eclass System

| Feature | Status | Notes |
|---------|--------|-------|
| `inherit` command | Full | Dynamic sourcing via mvdan.cc/sh |
| EXPORT_FUNCTIONS | Full | Phase wrapper generation |
| Eclass variable inheritance | Full | |
| Eclass function inheritance | Full | |
| BASH_VERSINFO emulation | Full | Forces bash 4 code paths in eclasses |
| Eclass stdout isolation | Full | `>/dev/null` redirect during sourcing |

### Eclass Go Modules (14 specialized implementations)

| Eclass Module | File | Status | Notes |
|---------------|------|--------|-------|
| autotools | (via econf) | Full | eautoreconf, AT_M4DIR support |
| cargo | `eclass_cargo.go` | Partial | cargo_crate_uris, cargo_src_* |
| cmake | `eclass_cmake.go` + `build_cmake.go` | Partial | Full phase support, Ninja/Make |
| distutils-r1 | `eclass_distutils.go` | Partial | Phase dispatch, recursion guard |
| flag-o-matic | `eclass_flag_o_matic.go` | Full | 17+ functions: append/filter/replace/strip/test-flags |
| go-module | `eclass_go_module.go` | Partial | go-module_set_globals, src_unpack |
| meson | `eclass_meson.go` + `build_meson.go` | Partial | Cross-file generation, feature flags |
| multilib | `eclass_multilib.go` | Partial | ABI handling, get_libdir |
| multilib-build | `eclass_multilib_build.go` | Partial | foreach_abi, native_abi checks |
| python-any-r1 | `eclass_python_any.go` | Partial | pkg_setup, version detection |
| python-r1 | `eclass_python_r1.go` | Partial | foreach_impl, pkg_setup |
| python-single-r1 | `eclass_python_single.go` | Partial | Single implementation setup |
| python-utils-r1 | `eclass_python_utils.go` | Partial | python_export, python_get_*, 35+ functions |
| toolchain-funcs | `helpers_toolchain.go` | Full | 27 functions: tc-get*, tc-is-*, tc-arch |

### Eclass Coverage by Category

| Eclass Category | Status | Notes |
|-----------------|--------|-------|
| Simple utility eclasses | Full | xdg, desktop, optfeature, edo |
| Autotools-based | Full | autotools.eclass via econf |
| CMake-based | Partial | cmake.eclass with Ninja support |
| Meson-based | Partial | meson.eclass with cross-compile |
| Python-based | Partial | python-r1, python-single-r1, python-any-r1, distutils-r1 |
| Go-based | Partial | go-module.eclass |
| Rust-based | Partial | cargo.eclass |
| Multilib | Partial | 32/64-bit ABI management |
| Kernel-related | Not Yet | kernel-install.eclass |
| LLVM-related | Not Yet | llvm.eclass |

### Implementation

```
internal/eclass/loader.go           # Dynamic eclass loading and caching
internal/ebuild/eclass_bridge.go    # Go <-> bash bridging layer
internal/ebuild/eclass_*.go         # 14 eclass-specific modules
internal/ebuild/build_*.go          # Build system implementations
```

---

## Chapter 11: Environment

**Status: Full**

### Section 11.1: Environment Variables

| Variable | Status | Notes |
|----------|--------|-------|
| P, PF, PN, PV, PR, PVR | Full | |
| CATEGORY | Full | |
| A | Full | From Manifest DIST entries |
| FILESDIR | Full | |
| DISTDIR | Full | |
| WORKDIR | Full | |
| S | Full | v0.9.4: correctly computed from bash env |
| T | Full | |
| D | Full | |
| ED | Full | EAPI 3+ |
| ROOT | Full | `--root` flag support |
| EROOT | Full | EAPI 3+ |
| EPREFIX | Full | EAPI 3+ |
| SYSROOT | Partial | EAPI 7+ |
| ESYSROOT | Partial | EAPI 7+ |
| BROOT | Partial | EAPI 7+ |
| EBUILD_PHASE | Full | |
| EBUILD_PHASE_FUNC | Full | |
| MERGE_TYPE | Full | |
| REPLACING_VERSIONS | Partial | |
| REPLACED_BY_VERSION | Partial | |
| CHOST | Full | v0.9.4: from env, used by econf/tc-arch |
| CBUILD | Full | v0.9.4: from env, cross-compile detection |

### Section 11.1.1: USE and USE_EXPAND

| Feature | Status | Notes |
|---------|--------|-------|
| USE variable | Full | From make.conf + package.use + profile |
| USE_EXPAND | Full | v0.9.4: passed through from make.conf |
| IUSE_EFFECTIVE | Partial | |

### Implementation

```
internal/ebuild/environment.go  # Environment setup (20+ Portage variables)
internal/config/config.go       # USE flag resolution with variable expansion
```

---

## Chapter 12: Commands (Helpers)

**Status: Partial**

### Overview

GRPM implements **214 helper functions** in Go, with **159 bash commands** routed through the interpreter's exec handler.

### Section 12.3: Package Manager Commands

#### Installation Helpers (36 functions)

| Command | Status | Notes |
|---------|--------|-------|
| dobin | Full | Resolves paths against $S |
| doconfd | Full | |
| dodir | Full | |
| dodoc | Full | |
| doenvd | Full | |
| doexe | Full | |
| doheader | Full | |
| dohtml | Full | Deprecated |
| doinfo | Full | |
| doinitd | Full | |
| doins | Full | Recursive (-r) support |
| dolib, dolib.a, dolib.so | Full | |
| doman | Full | |
| domo | Full | |
| dosbin | Full | |
| dosym | Full | Including -r (EAPI 8+) |
| dostrip | Full | EAPI 7+ |
| fowners | Full | |
| fperms | Full | |
| newbin, newsbin, newins, newdoc, newman | Full | Rename-on-install variants |
| into, insinto, exeinto, docinto | Full | Destination directory setters |
| insopts, exeopts, diropts | Full | Permission setters |

#### Build Helpers (11 functions)

| Command | Status | Notes |
|---------|--------|-------|
| econf | Full | ECONF_SOURCE, CHOST, CBUILD, --libdir detection |
| emake | Full | MAKEOPTS integration |
| einstall | Full | Deprecated |
| eautoreconf | Full | AT_M4DIR support |
| unpack | Full | 11 formats including .tar.lz |

#### Patch/Apply Helpers (6 functions)

| Command | Status | Notes |
|---------|--------|-------|
| eapply | Full | EAPI 6+ |
| eapply_user | Full | EAPI 6+ |
| epatch | Partial | Deprecated, basic support |
| eshopts_push/pop | Full | |
| estack_push/pop | Full | |

#### USE Flag Helpers (10 functions)

| Command | Status | Notes |
|---------|--------|-------|
| use | Full | Checks runtime env |
| useq | Full | Deprecated |
| usev | Full | EAPI 5+ |
| usex | Full | EAPI 5+ |
| use_enable | Full | |
| use_with | Full | |
| in_iuse | Full | Checks runtime IUSE |
| has | Full | |
| hasv | Full | Deprecated |
| hasq | Full | Deprecated |

#### Output Helpers (14 functions)

| Command | Status | Notes |
|---------|--------|-------|
| einfo / einfon | Full | |
| ewarn | Full | |
| eerror | Full | |
| ebegin | Full | |
| eend | Full | |
| elog | Full | |
| eqawarn | Full | |
| die | Full | |
| nonfatal | Full | EAPI 4+ |
| assert | Full | |
| debug-print | Full | |
| debug-print-function | Full | |
| debug-print-section | Full | |

#### Toolchain Helpers (27 functions)

| Command | Status | Notes |
|---------|--------|-------|
| tc-getCC/CXX/LD/AR/RANLIB/NM | Full | |
| tc-getSTRIP/OBJCOPY/OBJDUMP | Full | |
| tc-getPKG_CONFIG | Full | |
| tc-getBUILD_CC/CXX | Full | |
| tc-is-cross-compiler | Full | |
| tc-is-gcc / tc-is-clang | Full | |
| tc-arch | Full | CHOST-based detection |
| tc-endian | Full | |
| tc-export | Full | |

#### Version Helpers (10 functions)

| Command | Status | Notes |
|---------|--------|-------|
| ver_cut | Full | Defaults to $PV when arg omitted |
| ver_rs | Full | Defaults to $PV when arg omitted |
| ver_test | Full | PMS-compliant version comparison |

#### File System Helpers (33 functions)

| Command | Status | Notes |
|---------|--------|-------|
| sed | Full | Go-native implementation |
| cp / mv / rm / mkdir | Full | Including cross-device fallback |
| chmod | Full | Symbolic modes support |
| ln | Full | Symlink/hardlink/relative/force |
| find / grep / xargs | Full | |
| which / touch | Full | |
| install | Full | GNU install compatible |
| pkg-config | Full | |

#### Default Phase Functions (8 functions)

| Command | Status | Notes |
|---------|--------|-------|
| default | Full | Phase-aware dispatch |
| default_pkg_nofetch | Full | |
| default_src_unpack | Full | |
| default_src_prepare | Full | eapply_user |
| default_src_configure | Full | econf fallback |
| default_src_compile | Full | emake fallback |
| default_src_test | Full | |
| default_src_install | Partial | emake DESTDIR install |

#### Query Helpers

| Command | Status | Notes |
|---------|--------|-------|
| best_version | Partial | |
| has_version | Partial | |

### Implementation

```
internal/ebuild/interpreter.go       # Command dispatch (159 routed commands)
internal/ebuild/helpers*.go          # 214 helper functions across 15 files
internal/ebuild/eclass_*.go          # Eclass-specific helpers
internal/ebuild/build_*.go           # Build system helpers (CMake, Meson)
```

---

## Distfile Resolution (v0.9.4)

**Status: Full**

### SRC_URI Processing Pipeline

| Feature | Status | Notes |
|---------|--------|-------|
| SRC_URI evaluation via mvdan.cc/sh | Full | With eclass support |
| SRC_URI USE conditional filtering | Full | `verify-sig? ( )`, `test? ( )`, etc. |
| `SRC_URI+=` append support | Full | Raw text extraction fallback |
| mirror:// expansion | Full | thirdpartymirrors database |
| SRC_URI `->` rename operator | Full | |
| Signature file filtering (.sig/.asc/.sign) | Full | v0.9.4: 3-layer defense |

### Fallback Chain

When the primary bash evaluator fails, GRPM applies defense-in-depth:

1. **mvdan.cc/sh evaluation** — Full bash execution with eclass support
2. **Raw SRC_URI extraction** — Regex-based fallback from ebuild text (handles `SRC_URI=` and `SRC_URI+=`)
3. **Manifest filtering** — Safety net: filters signature files from raw manifest entries

This aligns with Portage behavior of never returning unfiltered manifest distfiles.

### Implementation

```
internal/distfile/service.go    # Unified distfile resolution (single source of truth)
internal/ebuild/metadata.go     # SRC_URI evaluation with eclass support
internal/fetch/mirror.go        # Mirror expansion and failover
internal/repo/srcuri_parser.go  # SRC_URI parsing with USE filtering
```

---

## Known Limitations

### Bash Interpreter (mvdan.cc/sh)

GRPM uses `mvdan.cc/sh` instead of system bash. Known issues documented in `docs/dev/MVDAN_SH_WORKAROUNDS.md`:

| Issue | Impact | Workaround |
|-------|--------|-----------|
| `type -P` not implemented | libtool.eclass | Preprocess to `command -v` |
| `declare -f`/`declare -p` | Eclass function checks | Custom Go handlers |
| `${var@a}` param attributes | python/guile eclasses | BASH_VERSINFO=(4...) emulation |
| Brace expansion in var names | findutils, sed test funcs | stripFunctionBodies() |
| Process substitution `>()` | multibuild.eclass | Override with simplified version |
| `read -a` | linux-headers | No workaround yet |
| Eclass stdout pollution | sed, toolchain-funcs | `>/dev/null` redirect |

### Build Systems

1. **Autotools** — Fully supported (econf with ECONF_SOURCE, CHOST, CBUILD)
2. **CMake** — Partial: Ninja/Make generators, basic configuration
3. **Meson** — Partial: cross-file generation, feature flags
4. **Cargo** — Partial: crate URIs, basic build phases
5. **Go modules** — Partial: module download, basic build

### Cross-Compilation

EAPI 7+ cross-compilation variables (SYSROOT, ESYSROOT, BROOT) are defined but not fully tested in cross-compile scenarios.

---

## Testing Coverage

| Area | Test Files | Tests |
|------|------------|-------|
| Version comparison | `version_test.go`, `constraint_test.go` | High |
| Atom parsing | `atom_test.go`, `atom_parse_test.go` | High |
| EAPI features | `eapi_test.go` | High |
| Dependency resolution | `resolver_test.go`, `solver_test.go` | Medium |
| Ebuild execution | `executor_test.go`, `phases_test.go` | Medium |
| Helpers | `helpers_*_test.go` (15 test files) | Medium |
| Metadata extraction | `metadata_test.go` | High (v0.9.4) |
| Distfile resolution | `service_test.go` | High (v0.9.4) |
| Ebuild tests total | 710+ passing | |
| Distfile tests total | 17 passing | |

---

## Roadmap

### v0.8.2-v0.8.4 (Complete)

- [x] `emerge --info` command (system environment display)
- [x] USE flags display in `emerge --pretend`
- [x] User-friendly error messages with package suggestions
- [x] Apply filtering to info/search commands
- [x] Per-package tool check based on eclasses
- [x] Search version sorting (PMS-compliant)
- [x] Dependency deduplication in info output
- [x] SRC_URI evaluation with eclass support (v0.8.3)
- [x] Package sets (@world, @system, @selected) in all commands (v0.8.4)
- [x] Profile symlink resolution for @system (v0.8.4)
- [x] Multi-parent profile inheritance (v0.8.4)

### v0.9.0 (Complete)

- [x] Enterprise tool check refactoring (Portage-compatible BDEPEND)
- [x] `--check-tools` opt-in flag (replaces `--skip-tool-check`)
- [x] Tool dependencies via BDEPEND like Portage

### v0.9.1 (Complete)

- [x] Mirror fallback support (Portage-compatible distfile fetching)
- [x] Enterprise CLI help formatter (professional help output)
- [x] Shell completion (bash/zsh/fish)
- [x] Man page generation (`grpm doc man`)
- [x] "Did you mean?" command suggestions
- [x] Type-safe CommandName constants

### v0.9.2 (Complete)

- [x] Emerge respects installed packages (VarDB filtering)
- [x] `--deep`, `--with-bdeps`, `--emptytree`, `--vardb` flags for emerge

### v0.9.3 (Complete)

- [x] Install helpers path resolution against `$S`
- [x] Unpack phase uses `$A` variable from Manifest
- [x] Non-standard archive name support (e.g., unix-tree)

### v0.9.4 (Complete)

- [x] Hardened bash interpreter with `stripFunctionBodies()`
- [x] Signature file filtering (3-layer defense: eval → raw extraction → manifest filter)
- [x] `.tar.lz` (lzip) unpack support via external decompressor
- [x] `BASH_VERSINFO` emulation for eclass compatibility
- [x] Eclass stdout isolation during sourcing
- [x] `econf` with ECONF_SOURCE, CHOST, CBUILD
- [x] `ver_cut`/`ver_rs` default to `$PV`
- [x] Correct `$S` variable computation
- [x] Silent manifest fallbacks removed (align with Portage)
- [x] golangci-lint: 0 issues
- [x] 33 new tests for metadata extraction and distfile resolution

### v0.9.x (Planned)

- [ ] `package.provided` support
- [ ] Improved cross-compilation support
- [ ] `read -a` workaround for linux-headers
- [ ] Performance optimization

### v1.0.0 (Target)

- [ ] Full PMS compliance for EAPI 8
- [ ] All eclasses tested on real tree
- [ ] Community validation complete

---

## References

- [Official PMS](https://projects.gentoo.org/pms/)
- [PMS Git Repository](https://gitweb.gentoo.org/proj/pms.git/)
- [Gentoo Devmanual](https://devmanual.gentoo.org/)
- [GRPM Source Code](https://github.com/grpmsoft/grpm)
- [Local PMS Reference](pms/README.md)

---

*This document is maintained alongside GRPM development.*
*Contributions and corrections welcome via GitHub Issues.*
