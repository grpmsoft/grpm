# PMS Compliance Matrix

> **GRPM Implementation Status per [Package Manager Specification](https://projects.gentoo.org/pms/)**
>
> **Version:** v0.9.3
> **Last Updated:** 2026-01-20
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
| Ch. 6: Ebuild File Format | **Partial** | Parsing works, bash execution via mvdan.cc/sh |
| Ch. 7: Ebuild Variables | **Full** | All mandatory/optional variables parsed |
| Ch. 8: Dependencies | **Full** | All operators, slots, USE deps supported |
| Ch. 9: Phase Functions | **Partial** | Core phases work, some edge cases |
| Ch. 10: Eclasses | **Partial** | Dynamic loading works, complex eclasses may fail |
| Ch. 11: Environment | **Partial** | Core variables set, some cross-compile vars missing |
| Ch. 12: Commands | **Partial** | 166+ helpers implemented, some missing |

**Overall Estimate:** ~75% PMS compliance for common use cases.

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
| Profile inheritance (`parent`) | Full | |
| `make.defaults` | Full | |
| `package.mask` | Full | v0.8.1 |
| `package.unmask` | Full | v0.8.1 |
| `package.use` | Full | v0.8.0 |
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
internal/config/config.go     # make.conf parsing
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

GRPM uses `mvdan.cc/sh` as a Go-native bash interpreter, not system bash.

### Section 6.2: Encoding and Format

| Feature | Status | Notes |
|---------|--------|-------|
| UTF-8 encoding | Full | |
| EAPI line detection | Full | Regex per PMS |
| Failglob (EAPI 6+) | Partial | Interpreter limitation |
| Umask | Full | Set in environment |

### Limitations

- Complex bash constructs may fail in Go interpreter
- Some bash builtins behave differently
- Process substitution limited

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
| SRC_URI | Full | Arrows, USE conditionals |
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
| src_unpack | Full | |
| src_prepare | Full | eapply_user handling |
| src_configure | Partial | Autotools works, others limited |
| src_compile | Partial | Simple builds work |
| src_test | Partial | When --test flag used |
| src_install | Partial | Basic installation works |
| pkg_preinst | Full | |
| pkg_postinst | Full | |
| pkg_prerm | Full | |
| pkg_postrm | Full | |
| pkg_config | Partial | |
| pkg_info | Partial | |
| pkg_nofetch | Not Yet | |

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

### Implementation

```
internal/ebuild/phases.go       # Phase definitions
internal/ebuild/phases_impl.go  # Phase implementations
internal/ebuild/executor.go     # Phase execution
```

---

## Chapter 10: Eclasses

**Status: Partial**

### Eclass System

| Feature | Status | Notes |
|---------|--------|-------|
| `inherit` command | Full | |
| EXPORT_FUNCTIONS | Full | |
| Eclass variable inheritance | Full | |
| Eclass function inheritance | Full | |

### Eclass Coverage

GRPM uses dynamic eclass loading via `mvdan.cc/sh` interpreter. Most eclasses work without Go reimplementation.

| Eclass Category | Status | Notes |
|-----------------|--------|-------|
| Simple utility eclasses | Full | xdg, desktop, optfeature, edo |
| Autotools-based | Full | autotools.eclass |
| CMake-based | Partial | cmake.eclass (basic) |
| Meson-based | Partial | meson.eclass (basic) |
| Python-based | Partial | python-r1.eclass (limited) |
| Go-based | Partial | go-module.eclass (limited) |
| Rust-based | Partial | cargo.eclass (limited) |
| Kernel-related | Not Yet | kernel-install.eclass |
| LLVM-related | Not Yet | llvm.eclass |

### Implementation

```
internal/eclass/loader.go       # Dynamic eclass loading
internal/ebuild/helpers*.go     # 103+ helper functions
```

---

## Chapter 11: Environment

**Status: Partial**

### Section 11.1: Environment Variables

| Variable | Status | Notes |
|----------|--------|-------|
| P, PF, PN, PV, PR, PVR | Full | |
| CATEGORY | Full | |
| A | Full | |
| FILESDIR | Full | |
| DISTDIR | Full | |
| WORKDIR | Full | |
| S | Full | |
| T | Full | |
| D | Full | |
| ED | Full | EAPI 3+ |
| ROOT | Full | |
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

### Section 11.1.1: USE and USE_EXPAND

| Feature | Status | Notes |
|---------|--------|-------|
| USE variable | Full | |
| USE_EXPAND | Partial | Basic support |
| IUSE_EFFECTIVE | Partial | |

### Implementation

```
internal/ebuild/environment.go  # Environment setup
internal/config/config.go       # USE flag resolution
```

---

## Chapter 12: Commands (Helpers)

**Status: Partial**

### Section 12.3: Package Manager Commands

GRPM implements 166+ helper functions in Go.

#### Installation Helpers

| Command | Status | Notes |
|---------|--------|-------|
| dobin | Full | |
| doconfd | Full | |
| dodir | Full | |
| dodoc | Full | |
| doenvd | Full | |
| doexe | Full | |
| doheader | Full | |
| dohtml | Full | Deprecated |
| doinfo | Full | |
| doinitd | Full | |
| doins | Full | |
| dolib, dolib.a, dolib.so | Full | |
| doman | Full | |
| domo | Full | |
| dosbin | Full | |
| dosym | Full | Including -r (EAPI 8+) |
| fowners | Full | |
| fperms | Full | |

#### Build Helpers

| Command | Status | Notes |
|---------|--------|-------|
| econf | Partial | Basic autoconf, limited options |
| emake | Full | |
| einstall | Partial | Deprecated |

#### Patch/Apply Helpers

| Command | Status | Notes |
|---------|--------|-------|
| eapply | Full | EAPI 6+ |
| eapply_user | Full | EAPI 6+ |
| epatch | Partial | Deprecated, basic support |

#### USE Flag Helpers

| Command | Status | Notes |
|---------|--------|-------|
| use | Full | |
| useq | Full | Deprecated |
| usev | Full | EAPI 5+ |
| usex | Full | EAPI 5+ |
| use_enable | Full | |
| use_with | Full | |
| in_iuse | Full | |

#### Output Helpers

| Command | Status | Notes |
|---------|--------|-------|
| einfo | Full | |
| ewarn | Full | |
| eerror | Full | |
| ebegin | Full | |
| eend | Full | |
| elog | Full | |
| eqawarn | Full | |

#### Misc Helpers

| Command | Status | Notes |
|---------|--------|-------|
| die | Full | |
| nonfatal | Full | EAPI 4+ |
| assert | Full | |
| debug-print | Full | |
| has | Full | |
| hasv | Full | |
| hasq | Full | Deprecated |
| best_version | Partial | |
| has_version | Partial | |
| dostrip | Full | EAPI 7+ |
| einstalldocs | Full | EAPI 6+ |
| get_libdir | Full | EAPI 6+ |

### Toolchain Helpers

| Command | Status | Notes |
|---------|--------|-------|
| tc-getCC | Full | |
| tc-getCXX | Full | |
| tc-getLD | Full | |
| tc-getAR | Full | |
| tc-getRANLIB | Full | |
| tc-getNM | Full | |
| tc-getSTRIP | Full | |
| tc-getPKG_CONFIG | Full | |
| tc-is-cross-compiler | Full | |
| tc-is-clang | Full | |
| tc-is-gcc | Full | |

### Implementation

```
internal/ebuild/interpreter.go       # Command dispatch (166+ handlers)
internal/ebuild/helpers*.go          # Helper implementations (15 files)
internal/ebuild/eclass_*.go          # Eclass-specific helpers
internal/ebuild/build_*.go           # Build system helpers
```

---

## Known Limitations

### Bash Interpreter

GRPM uses `mvdan.cc/sh` instead of system bash:

1. **Process substitution** — Limited support for `<()` and `>()`
2. **Complex here-docs** — Some edge cases may fail
3. **Bash 5.0 features** — Not all EAPI 8 bash features supported
4. **External commands** — Some shell builtins behave differently

### Build Systems

1. **Autotools** — Fully supported for standard configure/make workflow
2. **CMake** — Basic support, complex projects may fail
3. **Meson** — Basic support, complex projects may fail
4. **Cargo** — Limited support
5. **Go modules** — Limited support

### Cross-Compilation

EAPI 7+ cross-compilation variables (SYSROOT, ESYSROOT, BROOT) are defined but not fully tested in cross-compile scenarios.

---

## Testing Coverage

| Area | Test Files | Coverage |
|------|------------|----------|
| Version comparison | `version_test.go`, `constraint_test.go` | High |
| Atom parsing | `atom_test.go`, `atom_parse_test.go` | High |
| EAPI features | `eapi_test.go` | High |
| Dependency resolution | `resolver_test.go`, `solver_test.go` | Medium |
| Ebuild execution | `executor_test.go`, `phases_test.go` | Medium |
| Helpers | `helpers_test.go` | Medium |

---

## Roadmap to Full Compliance

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

- [x] Install helpers path resolution — All helpers resolve relative paths against `$S`
- [x] Unpack phase uses `$A` variable — Archive list from Manifest instead of hardcoded pattern
- [x] tree package fix — Packages with non-standard archive names now build correctly
- [x] DRY refactoring — Centralized `resolveSourcePath()` helper method

### v0.9.x (Planned)

- [ ] `package.provided` support
- [ ] Improved cross-compilation support
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
