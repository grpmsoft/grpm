# Chapter 9: Ebuild-defined functions

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#ebuilddefined-functions)
> **EAPI:** All versions (with EAPI-specific features noted)
> **Last Updated:** 2025-12-14 (PMS commit 43857e7)

## Overview

This chapter describes functions (phase functions) that an ebuild or eclass may define, and which will be called by the package manager as part of the build and/or install process. In all cases the package manager must provide a default implementation of these functions; unless otherwise stated this must be a no-op.

All functions may assume that they have read access to all system libraries, binaries and configuration files that are accessible to normal users, as well as write access to the temporary directories specified by the `T`, `TMPDIR` and `HOME` variables (see section 11.1). Most functions must assume only that they have additional write access to the package's working directory (the `WORKDIR` variable); exceptions are noted below.

The environment for functions run outside of the build sequence (that is, `pkg_config`, `pkg_info`, `pkg_prerm` and `pkg_postrm`) must be the environment used for the build of the package, not the current configuration.

Ebuilds must not call nor assume the existence of any phase functions.

## 9.1 List of functions

### 9.1.1 Initial working directories

**Feature:** `phase-function-dir` (EAPI 8+)

Some functions may assume that their initial working directory is set to a particular location; these are noted below. If no initial working directory is mandated, then for EAPIs listed in table 9.1 as having an empty directory, it must be set to a dedicated directory that is empty at the start of the function and may be read-only. For other EAPIs, it may be set to anything. The ebuild must not rely upon a particular location for it. The ebuild *may* assume that the initial working directory for any phase is a trusted location that may only be written to by a privileged user and group.

**Feature:** `s-workdir-fallback` (EAPI 4+)

Some functions are described as having an initial working directory of `S` with an error or fallback to `WORKDIR`. For EAPIs listed in table 9.2 as having the fallback, this means that if `S` is not a directory before the start of the phase function, the initial working directory shall be `WORKDIR` instead. For EAPIs where it is a conditional error, if `S` is not a directory before the start of the phase function, it is a fatal error, unless all of the following conditions are true, in which case the fallback to `WORKDIR` is used:

- The `A` variable contains no items.
- The phase function in question is not in `DEFINED_PHASES`.
- None of the phase functions `unpack`, `prepare`, `configure`, `compile`, `test` or `install`, if supported by the EAPI in question and occurring prior to the phase about to be executed, are in `DEFINED_PHASES`.

### Table 9.1: Initial working directory in pkg_* phase functions for EAPIs

| EAPI | Initial working directory? |
|------|----------------------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | Any |
| 8, 9 | Empty |

### Table 9.2: EAPIs with S to WORKDIR fallbacks

| EAPI | Fallback to WORKDIR permitted? |
|------|--------------------------------|
| 0, 1, 2, 3 | Always |
| 4, 5, 6, 7, 8, 9 | Conditional error |

### 9.1.2 pkg_pretend

**Feature:** `pkg-pretend` (EAPI 4+)

The `pkg_pretend` function is only called for EAPIs listed in table 9.3 as supporting it.

The `pkg_pretend` function may be used to carry out sanity checks early on in the install process. For example, if an ebuild requires a particular kernel configuration, it may perform that check in `pkg_pretend` and call `eerror` and then `die` with appropriate messages if the requirement is not met.

`pkg_pretend` is run separately from the main phase function sequence, and does not participate in any kind of environment saving. There is no guarantee that any of an ebuild's dependencies will be met at this stage, and no guarantee that the system state will not have changed substantially before the next phase is executed.

`pkg_pretend` must not write to the filesystem.

### Table 9.3: EAPIs supporting pkg_pretend

| EAPI | Supports pkg_pretend? |
|------|-----------------------|
| 0, 1, 2, 3 | No |
| 4, 5, 6, 7, 8, 9 | Yes |

### 9.1.3 pkg_setup

The `pkg_setup` function sets up the ebuild's environment for all following functions, before the build process starts. Further, it checks whether any necessary prerequisites not covered by the package manager, e.g. that certain kernel configuration options are fulfilled.

`pkg_setup` must be run with full filesystem permissions, including the ability to add new users and/or groups to the system.

### 9.1.4 src_unpack

The `src_unpack` function extracts all of the package's sources. In EAPIs lacking `src_prepare`, it may also apply patches and set up the package's build system for further use.

The initial working directory must be `WORKDIR`, and the default implementation used when the ebuild lacks the `src_unpack` function shall behave as:

```bash
src_unpack() {
    if [[ -n ${A} ]]; then
        unpack ${A}
    fi
}
```

### 9.1.5 src_prepare

**Feature:** `src-prepare` (EAPI 2+)

The `src_prepare` function is only called for EAPIs listed in table 9.4 as supporting it. The `src_prepare` function can be used for post-unpack source preparation.

The initial working directory is `S`, with an error or fallback to `WORKDIR` as discussed in section 9.1.1.

For EAPIs listed in table 9.4 as using format 6 or 8, the default implementation used when the ebuild lacks the `src_prepare` function shall behave as shown below.

For other EAPIs supporting `src_prepare`, the default implementation used when the ebuild lacks the `src_prepare` function is a no-op.

### Table 9.4: src_prepare support and behaviour for EAPIs

| EAPI | Supports src_prepare? | Format |
|------|-----------------------|--------|
| 0, 1 | No | Not applicable |
| 2, 3, 4, 5 | Yes | no-op |
| 6, 7 | Yes | 6 |
| 8, 9 | Yes | 8 |

**Format 6:**
```bash
src_prepare() {
    if [[ $(declare -p PATCHES 2>/dev/null) == "declare -a"* ]]; then
        [[ -n ${PATCHES[@]} ]] && eapply "${PATCHES[@]}"
    else
        [[ -n ${PATCHES} ]] && eapply ${PATCHES}
    fi
    eapply_user
}
```

**Format 8:**
```bash
src_prepare() {
    if [[ ${PATCHES@a} == *a* ]]; then
        [[ -n ${PATCHES[@]} ]] && eapply -- "${PATCHES[@]}"
    else
        [[ -n ${PATCHES} ]] && eapply -- ${PATCHES}
    fi
    eapply_user
}
```

### 9.1.6 src_configure

**Feature:** `src-configure` (EAPI 2+)

The `src_configure` function is only called for EAPIs listed in table 9.5 as supporting it.

The initial working directory is `S`, with an error or fallback to `WORKDIR` as discussed in section 9.1.1.

The `src_configure` function configures the package's build environment. The default implementation used when the ebuild lacks the `src_configure` function shall behave as:

```bash
src_configure() {
    if [[ -x ${ECONF_SOURCE:-.}/configure ]]; then
        econf
    fi
}
```

### Table 9.5: EAPIs supporting src_configure

| EAPI | Supports src_configure? |
|------|-------------------------|
| 0, 1 | No |
| 2, 3, 4, 5, 6, 7, 8, 9 | Yes |

### 9.1.7 src_compile

**Feature:** `src-compile`

The `src_compile` function configures the package's build environment in EAPIs lacking `src_configure`, and builds the package in all EAPIs.

The initial working directory is `S`, with an error or fallback to `WORKDIR` as discussed in section 9.1.1.

For EAPIs listed in table 9.6 as using format 0, 1 or 2, the default implementation shall behave as shown below.

### Table 9.6: src_compile behaviour for EAPIs

| EAPI | Format |
|------|--------|
| 0 | 0 |
| 1 | 1 |
| 2, 3, 4, 5, 6, 7, 8, 9 | 2 |

**Format 0:**
```bash
src_compile() {
    if [[ -x ./configure ]]; then
        econf
    fi
    if [[ -f Makefile ]] || [[ -f GNUmakefile ]] || [[ -f makefile ]]; then
        emake || die "emake failed"
    fi
}
```

**Format 1:**
```bash
src_compile() {
    if [[ -x ${ECONF_SOURCE:-.}/configure ]]; then
        econf
    fi
    if [[ -f Makefile ]] || [[ -f GNUmakefile ]] || [[ -f makefile ]]; then
        emake || die "emake failed"
    fi
}
```

**Format 2:**
```bash
src_compile() {
    if [[ -f Makefile ]] || [[ -f GNUmakefile ]] || [[ -f makefile ]]; then
        emake || die "emake failed"
    fi
}
```

### 9.1.8 src_test

The `src_test` function runs unit tests for the newly built but not yet installed package as provided.

The initial working directory is `S`, with an error or fallback to `WORKDIR` as discussed in section 9.1.1.

The default implementation used when the ebuild lacks the `src_test` function must, if tests are enabled, run `emake check` if and only if such a target is available, or if not run `emake test` if and only if such a target is available. In both cases, if `emake` returns non-zero the build must be aborted.

**Feature:** `parallel-tests` (EAPI 5+)

For EAPIs listed in table 9.7 as not supporting parallel tests, the `emake` command must be called with option `-j1`.

The `src_test` function may be disabled by `RESTRICT`. See section 7.3.6. It may be disabled by user too, using a PM-specific mechanism.

### Table 9.7: src_test behaviour for EAPIs

| EAPI | Supports parallel tests? |
|------|--------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

### 9.1.9 src_install

**Feature:** `src-install`

The `src_install` function installs the package's content to a directory specified in `D`.

The initial working directory is `S`, with an error or fallback to `WORKDIR` as discussed in section 9.1.1.

For EAPIs listed in table 9.8 as using format 4 or 6, the default implementation shall behave as shown below.

For other EAPIs, the default implementation used when the ebuild lacks the `src_install` function is a no-op.

### Table 9.8: src_install behaviour for EAPIs

| EAPI | Format |
|------|--------|
| 0, 1, 2, 3 | no-op |
| 4, 5 | 4 |
| 6, 7, 8, 9 | 6 |

**Format 4:**
```bash
src_install() {
    if [[ -f Makefile ]] || [[ -f GNUmakefile ]] || [[ -f makefile ]]; then
        emake DESTDIR="${D}" install
    fi

    if ! declare -p DOCS >/dev/null 2>&1; then
        local d
        for d in README* ChangeLog AUTHORS NEWS TODO CHANGES \
                THANKS BUGS FAQ CREDITS CHANGELOG; do
            [[ -s "${d}" ]] && dodoc "${d}"
        done
    elif [[ $(declare -p DOCS) == "declare -a"* ]]; then
        dodoc "${DOCS[@]}"
    else
        dodoc ${DOCS}
    fi
}
```

**Format 6:**
```bash
src_install() {
    if [[ -f Makefile ]] || [[ -f GNUmakefile ]] || [[ -f makefile ]]; then
        emake DESTDIR="${D}" install
    fi
    einstalldocs
}
```

### 9.1.10 pkg_preinst

The `pkg_preinst` function performs any special tasks that are required immediately before merging the package to the live filesystem. It must not write outside of the directories specified by the `ROOT` and `D` variables.

`pkg_preinst` must be run with full access to all files and directories below that specified by the `ROOT` and `D` variables.

### 9.1.11 pkg_postinst

The `pkg_postinst` function performs any special tasks that are required immediately after merging the package to the live filesystem. It must not write outside of the directory specified in the `ROOT` variable.

`pkg_postinst`, like `pkg_preinst`, must be run with full access to all files and directories below that specified by the `ROOT` variable.

### 9.1.12 pkg_prerm

The `pkg_prerm` function performs any special tasks that are required immediately before unmerging the package from the live filesystem. It must not write outside of the directory specified by the `ROOT` variable.

`pkg_prerm` must be run with full access to all files and directories below that specified by the `ROOT` variable.

### 9.1.13 pkg_postrm

The `pkg_postrm` function performs any special tasks that are required immediately after unmerging the package from the live filesystem. It must not write outside of the directory specified by the `ROOT` variable.

`pkg_postrm` must be run with full access to all files and directories below that specified by the `ROOT` variable.

### 9.1.14 pkg_config

The `pkg_config` function performs any custom steps required to configure a package after it has been fully installed. It is the only ebuild function which may be interactive and prompt for user input.

`pkg_config` must be run with full access to all files and directories inside of `ROOT`.

### 9.1.15 pkg_info

**Feature:** `pkg-info` (EAPI 4+)

The `pkg_info` function may be called by the package manager when displaying information about an installed package. In EAPIs listed in table 9.9 as supporting `pkg_info` on non-installed packages, it may also be called by the package manager when displaying information about a non-installed package. In this case, ebuild authors should note that dependencies may not be installed.

`pkg_info` must not write to the filesystem.

### Table 9.9: EAPIs supporting pkg_info on non-installed packages

| EAPI | Supports pkg_info on non-installed packages? |
|------|-----------------------------------------------|
| 0, 1, 2, 3 | No |
| 4, 5, 6, 7, 8, 9 | Yes |

### 9.1.16 pkg_nofetch

The `pkg_nofetch` function is run when the fetch phase of a fetch-restricted ebuild is run, and the relevant source files are not available. It should direct the user to download all relevant source files from their respective locations, with notes concerning licensing if applicable.

`pkg_nofetch` must require no write access to any part of the filesystem.

### 9.1.17 Default phase functions

**Feature:** `default-phase-funcs` (EAPI 2+)

In EAPIs listed in table 9.10 as supporting `default_` phase functions, a function named `default_<phase-function>` that behaves as the default implementation for that EAPI shall be defined when executing any ebuild phase function listed in the table. Ebuilds must not call these functions except when in the phase in question.

### Table 9.10: EAPIs supporting default_ phase functions

| EAPI | Supports default_ functions in phases |
|------|----------------------------------------|
| 0, 1 | None |
| 2, 3 | `pkg_nofetch`, `src_unpack`, `src_prepare`, `src_configure`, `src_compile`, `src_test` |
| 4, 5, 6, 7, 8, 9 | `pkg_nofetch`, `src_unpack`, `src_prepare`, `src_configure`, `src_compile`, `src_test`, `src_install` |

## 9.2 Call order

### Installing a package:

1. `pkg_pretend` (only for EAPIs listed in table 9.3), which is called outside of the normal call order process.
2. `pkg_setup`
3. `src_unpack`
4. `src_prepare` (only for EAPIs listed in table 9.4)
5. `src_configure` (only for EAPIs listed in table 9.5)
6. `src_compile`
7. `src_test` (except if `RESTRICT=test` or disabled by user)
8. `src_install`
9. `pkg_preinst`
10. `pkg_postinst`

### Uninstalling a package:

1. `pkg_prerm`
2. `pkg_postrm`

### Upgrading, downgrading or reinstalling a package:

1. `pkg_pretend` (only for EAPIs listed in table 9.3), which is called outside of the normal call order process.
2. `pkg_setup`
3. `src_unpack`
4. `src_prepare` (only for EAPIs listed in table 9.4)
5. `src_configure` (only for EAPIs listed in table 9.5)
6. `src_compile`
7. `src_test` (except if `RESTRICT=test`)
8. `src_install`
9. `pkg_preinst`
10. `pkg_prerm` for the package being replaced
11. `pkg_postrm` for the package being replaced
12. `pkg_postinst`

**Note:** When up- or downgrading a package in EAPI 0 or 1, the last four phase functions can alternatively be called in the order `pkg_preinst`, `pkg_postinst`, `pkg_prerm`, `pkg_postrm`. This behaviour is deprecated.

The `pkg_config`, `pkg_info` and `pkg_nofetch` functions are not called in a normal sequence. The `pkg_pretend` function is called some unspecified time before a (possibly hypothetical) normal sequence.

For installing binary packages, the `src` phases are not called.

When building binary packages that are not to be installed locally, the `pkg_preinst` and `pkg_postinst` functions are not called.

## Implementation Notes

### For GRPM implementation:

**Phase Function Execution:**
- Provide default implementations for all phases (most are no-ops)
- Track `DEFINED_PHASES` metadata variable
- Support EAPI-specific default implementations (tables 9.4, 9.6, 9.8)

**Working Directory Management:**
- EAPI 8+: `pkg_*` phases start in empty directory
- Most `src_*` phases: initial dir = `S` with fallback to `WORKDIR`
- Implement S-to-WORKDIR fallback logic (table 9.2):
  - EAPI 0-3: always fallback if S doesn't exist
  - EAPI 4+: conditional error based on `A`, `DEFINED_PHASES`

**Phase Function Features by EAPI:**
- `pkg_pretend`: EAPI 4+ (sanity checks before build)
- `src_prepare`: EAPI 2+ (patch application)
- `src_configure`: EAPI 2+ (separate configure phase)
- Parallel tests: EAPI 5+ (don't force `-j1`)
- `default_*` functions: EAPI 2+ (table 9.10)

**src_prepare Implementations:**
- EAPI 2-5: no-op
- EAPI 6-7: format 6 (apply `PATCHES`, call `eapply_user`)
- EAPI 8-9: format 8 (use Bash 4.4+ array detection: `${PATCHES@a}`)

**src_compile Implementations:**
- EAPI 0: check `./configure`, run `econf`, then `emake`
- EAPI 1: check `${ECONF_SOURCE:-.}/configure`, run `econf`, then `emake`
- EAPI 2+: only run `emake` if Makefile exists (no auto-configure)

**src_install Implementations:**
- EAPI 0-3: no-op (ebuild must define it)
- EAPI 4-5: format 4 (`emake install`, manual `DOCS` handling)
- EAPI 6+: format 6 (`emake install`, `einstalldocs` helper)

**Phase Call Order:**
- Normal install: setup → unpack → prepare → configure → compile → test → install → preinst → postinst
- Upgrade: includes prerm/postrm for old package *after* preinst
- Binary install: skip all `src_*` phases
- Build-only: skip `pkg_preinst`, `pkg_postinst`

**Access Permissions:**
- Most phases: read-only system, write to `WORKDIR`, `T`, `TMPDIR`, `HOME`
- `pkg_setup`: full filesystem permissions (can add users/groups)
- `pkg_preinst`, `pkg_postinst`: full access below `ROOT` and `D`
- `pkg_prerm`, `pkg_postrm`: full access below `ROOT`
- `pkg_config`: full access below `ROOT` (may be interactive)
- `pkg_pretend`, `pkg_info`, `pkg_nofetch`: read-only (no filesystem writes)

**Environment Handling:**
- `pkg_config`, `pkg_info`, `pkg_prerm`, `pkg_postrm`: use build-time environment, not current config

**Testing:**
- Test EAPI-specific default implementations
- Verify working directory handling for each phase
- Test S-to-WORKDIR fallback conditions
- Validate phase call order for install/upgrade/uninstall

## References

- [PMS Chapter 9: Ebuild-defined functions](https://projects.gentoo.org/pms/latest/pms.html#ebuilddefined-functions)
- [Gentoo Developer Manual: Ebuild Writing](https://devmanual.gentoo.org/ebuild-writing/)
- [EAPI Cheat Sheet](https://dev.gentoo.org/~ulm/pms/head/pms.html)
