# Chapter 11: The ebuild environment

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#the-ebuild-environment)
> **EAPI:** All versions (with EAPI-specific features noted)
> **Last Updated:** 2025-12-14 (PMS commit 43857e7)

## Overview

This chapter describes the environment variables that package managers must provide to ebuilds, and rules about variable persistence between phase functions.

## 11.1 Defined variables

The package manager must define the following variables. Not all variables are universally meaningful; variables that are not meaningful in a given phase or in global scope may be unset or set to any value. Ebuilds must not attempt to modify any of these variables, unless otherwise specified.

Because of their special meanings, these variables may not be preserved consistently across all phases as would normally happen due to environment saving (see section 11.2). For example, `EBUILD_PHASE` is different for every phase, and `ROOT` may have changed between the various different `pkg_*` phases. Ebuilds must recalculate any variable they derive from an inconsistent variable.

**Feature:** `export-vars`

These variables are either exported to the environment or kept as unexported shell variables, as specified for EAPIs in table 11.1; exceptions are `TMPDIR` and `HOME` which are always exported. In EAPIs where variables are not exported, the package manager must pass those that are required by ebuild-specific external commands (see section 12.3) in an implementation-defined manner.

### Table 11.1: EAPIs with variables exported to the environment

| EAPI | Variables exported? |
|------|---------------------|
| 0, 1, 2, 3, 4, 5, 6, 7, 8 | Yes |
| 9 | No |

## Table 11.2: Defined variables

| Variable | Legal in | Consistent? | Description |
|----------|----------|-------------|-------------|
| `P` | All | No¹ | Package name and version, without the revision part. For example, `vim-7.0.174`. |
| `PF` | All | No¹ | Package name, version, and revision (if any), for example `vim-7.0.174-r1`. |
| `PN` | All | No¹ | Package name, for example `vim`. |
| `CATEGORY` | All | No¹ | The package's category, for example `app-editors`. |
| `PV` | All | Yes | Package version, with no revision. For example `7.0.174`. |
| `PR` | All | Yes | Package revision, or `r0` if none exists. |
| `PVR` | All | Yes | Package version and revision (if any), for example `7.0.174` or `7.0.174-r1`. |
| `A` | `src_*`, `pkg_nofetch` | Yes | All source files available for the package, whitespace separated with no leading or trailing whitespace, and in the order in which the item first appears in a matched component of `SRC_URI`. Does not include any that are disabled because of USE conditionals. The value is calculated from the base names of each element of the `SRC_URI` ebuild metadata variable. |
| `AA`² | `src_*`, `pkg_nofetch` | Yes | All source files that could be available for the package, including any that are disabled in `A` because of USE conditionals. The value is calculated from the base names of each element of the `SRC_URI` ebuild metadata variable. Only for EAPIs listed in table 11.4 as supporting `AA`. |
| `FILESDIR` | `src_*`, global scope³ | Yes | The full path to a directory where the files from the package's files directory (used for small support files or patches) are available. See section 4.3. May or may not exist; if a repository provides no support files for the package in question then an ebuild must be prepared for the situation where `FILESDIR` points to a non-existent directory. |
| `DISTDIR` | `src_*`, global scope³ | Yes | The full path to the directory in which the files in the `A` variable are stored. |
| `WORKDIR` | `src_*`, global scope³ | Yes | The full path to the ebuild's working directory, where all build data should be contained. |
| `S` | `src_*` | Yes | The full path to the temporary build directory, used by `src_compile`, `src_install` etc. Defaults to `${WORKDIR}/${P}`. May be modified by ebuilds. If `S` is assigned in the global scope of an ebuild, then the restrictions of section 11.2 for global variables apply. |
| `PORTDIR` | `src_*` | No | The full path to the master repository's base directory. Only for EAPIs listed in table 11.4 as supporting `PORTDIR`. |
| `ECLASSDIR` | `src_*` | No | The full path to the master repository's eclass directory. Only for EAPIs listed in table 11.4 as supporting `ECLASSDIR`. |
| `ROOT` | `pkg_*` | No | The absolute path to the root directory into which the package is to be merged. Phases which run with full filesystem access must not touch any files outside of the directory given in `ROOT`. Also of note is that in a cross-compiling environment, binaries inside of `ROOT` will not be executable on the build machine, so ebuilds must not call them. The presence of a trailing slash is EAPI dependent as listed in table 11.8. |
| `EROOT` | `pkg_*` | No | Contains the concatenation of the paths in the `ROOT` and `EPREFIX` variables, for convenience. See also the `EPREFIX` variable. Only for EAPIs listed in table 11.5 as supporting `EROOT`. The presence of a trailing slash is EAPI dependent as listed in table 11.8. |
| `SYSROOT`⁴ | `src_*`, `pkg_setup` | No | The absolute path to the root directory containing build dependencies satisfied by `DEPEND`. Only for EAPIs listed in table 11.3 as supporting `SYSROOT`. |
| `ESYSROOT` | `src_*`, `pkg_setup`⁴ | No | Contains the concatenation of the `SYSROOT` path and applicable prefix value, as determined by table 8.3. Only for EAPIs listed in table 11.5 as supporting `ESYSROOT`. |
| `BROOT`⁴,⁵ | `src_*`, `pkg_setup`, `pkg_preinst`, `pkg_postinst`, `pkg_prerm`, `pkg_postrm` | No | The absolute path to the root directory containing build dependencies satisfied by `BDEPEND` and `IDEPEND`, typically executable build tools. This includes any applicable offset prefix. Only for EAPIs listed in table 11.3 as supporting `BROOT`. |
| `T` | All | Partially⁶ | The full path to a temporary directory for use by the ebuild. |
| `TMPDIR` | All | Partially⁶ | Must be set to the location of a usable temporary directory, for any applications called by an ebuild. Must not be used by ebuilds directly; see `T` above. `TMPDIR` is always exported to the environment. |
| `HOME` | All | Partially⁶ | The full path to an appropriate temporary directory for use by any programs invoked by the ebuild that may read or modify the home directory. `HOME` is always exported to the environment. |
| `EPREFIX` | All | Yes | The normalised offset-prefix path of an offset installation. When `EPREFIX` is not set in the calling environment, `EPREFIX` defaults to the built-in offset-prefix that was set during installation of the package manager. When a different `EPREFIX` value than the built-in value is set in the calling environment, a cross-prefix build is performed where using the existing utilities, a package is built for the given `EPREFIX`, akin to `ROOT`. See also section 11.1.3. Only for EAPIs listed in table 11.5 as supporting `EPREFIX`. |
| `D` | `src_*` | Yes | Contains the full path to the image directory into which the package should be installed. Ebuilds must not attempt to access the directory in `src_*` phases other than `src_install`. The presence of a trailing slash is EAPI dependent as listed in table 11.8. |
| `D` (continued) | `pkg_preinst` | No⁷ | Contains the full path to the image directory of the package about to be merged. The presence of a trailing slash is EAPI dependent as listed in table 11.8. |
| `ED` | `src_*`, `pkg_preinst` | See `D` | Contains the concatenation of the paths in the `D` and `EPREFIX` variables, for convenience. See also the `EPREFIX` variable. Only for EAPIs listed in table 11.5 as supporting `ED`. Ebuilds must not attempt to access the directory in `src_*` phases other than `src_install`. The presence of a trailing slash is EAPI dependent as listed in table 11.8. |
| `DESTTREE` | `src_install` | No | Controls the location where `dobin`, `dolib`, `domo`, and `dosbin` install things. Only for EAPIs listed in table 11.4 as supporting `DESTTREE`. In all other EAPIs, this is retained as a conceptual variable inaccessible from the ebuild environment. |
| `INSDESTTREE` | `src_install` | No | Controls the location where `doins` installs things. Only for EAPIs listed in table 11.4 as supporting `INSDESTTREE`. In all other EAPIs, this is retained as a conceptual variable inaccessible from the ebuild environment. |
| `USE` | All | Yes | A whitespace-delimited list of all active USE flags for this ebuild. See section 11.1.1 for details. |
| `EBUILD_PHASE` | `src_*`, `pkg_*` | No | Takes one of the values `config`, `setup`, `nofetch`, `unpack`, `prepare`, `configure`, `compile`, `test`, `install`, `preinst`, `postinst`, `prerm`, `postrm`, `info`, `pretend` according to the top level ebuild function that was executed by the package manager. Behaviour is unspecified when the ebuild is being sourced for other (e.g. metadata or QA) purposes. |
| `EBUILD_PHASE_FUNC` | `src_*`, `pkg_*` | No | Takes one of the values `pkg_config`, `pkg_setup`, `pkg_nofetch`, `src_unpack`, `src_prepare`, `src_configure`, `src_compile`, `src_test`, `src_install`, `pkg_preinst`, `pkg_postinst`, `pkg_prerm`, `pkg_postrm`, `pkg_info`, `pkg_pretend` according to the top level ebuild function that was executed by the package manager. Behaviour is unspecified when the ebuild is being sourced for other (e.g. metadata or QA) purposes. Only for EAPIs listed in table 11.3 as supporting `EBUILD_PHASE_FUNC`. |
| `KV` | All | Yes | The version of the running kernel at the time the ebuild was first executed, as returned by the `uname -r` command or equivalent. May be modified by ebuilds. Only for EAPIs listed in table 11.4 as supporting `KV`. |
| `MERGE_TYPE` | `pkg_*` | No | The type of package that is being merged. Possible values are: `source` if building and installing a package from source, `binary` if installing a binary package, and `buildonly` if building a binary package without installing it. Only for EAPIs listed in table 11.3 as supporting `MERGE_TYPE`. |
| `REPLACING_VERSIONS` | `pkg_preinst`, `pkg_postinst` (`pkg_pretend`, `pkg_setup`) | Yes | A list of all versions of this package (including revision, if specified), whitespace separated with no leading or trailing whitespace, that are being replaced (uninstalled or overwritten) as a result of this install. See section 11.1.2, especially for the phases in which the variable is legal. Only for EAPIs listed in table 11.3 as supporting `REPLACING_VERSIONS`. |
| `REPLACED_BY_VERSION` | `pkg_prerm`, `pkg_postrm` | Yes | The single version of this package (including revision, if specified) that is replacing us, if we are being uninstalled as part of an install, or an empty string otherwise. See section 11.1.2. Only for EAPIs listed in table 11.3 as supporting `REPLACED_BY_VERSION`. |

**Footnotes:**
1. Not consistent across all phases (e.g., may change during updates)
2. `AA` deprecated in EAPI 4+
3. Global scope access for convenience, not required by spec
4. Available in additional phases for cross-compilation support
5. `BROOT` also available in additional pkg phases (EAPI 7+)
6. Partially consistent - path exists but may change between invocations
7. Becomes inconsistent in `pkg_preinst` as it points to about-to-be-merged package

### Table 11.3: EAPIs supporting various added env variables

| EAPI | MERGE_TYPE? | REPLACING_VERSIONS? | REPLACED_BY_VERSION? | EBUILD_PHASE_FUNC? | SYSROOT? | BROOT? |
|------|-------------|---------------------|----------------------|--------------------|----------|--------|
| 0, 1, 2, 3 | No | No | No | No | No | No |
| 4 | Yes | Yes | Yes | No | No | No |
| 5, 6 | Yes | Yes | Yes | Yes | No | No |
| 7, 8, 9 | Yes | Yes | Yes | Yes | Yes | Yes |

### Table 11.4: EAPIs supporting various removed env variables

| EAPI | AA? | KV? | PORTDIR? | ECLASSDIR? | DESTTREE? | INSDESTTREE? |
|------|-----|-----|----------|------------|-----------|--------------|
| 0, 1, 2, 3 | Yes | Yes | Yes | Yes | Yes | Yes |
| 4, 5, 6 | No | No | Yes | Yes | Yes | Yes |
| 7, 8, 9 | No | No | No | No | No | No |

### Table 11.5: EAPIs supporting offset-prefix env variables

| EAPI | EPREFIX? | EROOT? | ED? | ESYSROOT? |
|------|----------|--------|-----|-----------|
| 0, 1, 2 | No | No | No | No |
| 3, 4, 5, 6 | Yes | Yes | Yes | No |
| 7, 8, 9 | Yes | Yes | Yes | Yes |

### Additional Requirements

- `CHOST`, `CBUILD` and `CTARGET`, if not set by profiles, must contain either an appropriate machine tuple or be unset.
- `PATH` must be initialized by the package manager to a "usable" default (should include "sbin", "bin", and package manager specific directories).
- `GZIP`, `BZIP`, `BZIP2`, `CDPATH`, `GREP_OPTIONS`, `GREP_COLOR` and `GLOBIGNORE` must not be set.

**Feature:** `env-unset` (EAPI 4+)

In addition, any variable whose name appears in the `ENV_UNSET` variable must be unset, for EAPIs listed in table 5.7 as supporting `ENV_UNSET`.

**Feature:** `locale-settings` (EAPI 6+)

The package manager must ensure that the `LC_CTYPE` and `LC_COLLATE` locale categories are equivalent to the POSIX locale, as far as characters in the ASCII range (U+0000 to U+007F) are concerned. Only for EAPIs listed in table 11.6.

### Table 11.6: Locale settings for EAPIs

| EAPI | Sane LC_CTYPE and LC_COLLATE? |
|------|-------------------------------|
| 0, 1, 2, 3, 4, 5 | Undefined |
| 6, 7, 8, 9 | Yes |

## 11.1.1 USE and IUSE handling

This section discusses the handling of four variables:

- **IUSE**: is the variable calculated from the `IUSE` values defined in ebuilds and eclasses.
- **IUSE_REFERENCEABLE**: is a variable calculated from `IUSE` and a variety of other sources described below. It is purely a conceptual variable; it is inaccessible from the ebuild environment. Values in `IUSE_REFERENCEABLE` may legally be used in queries from other packages about an ebuild's state (for example, for use dependencies).
- **IUSE_EFFECTIVE**: is another conceptual, inaccessible variable. Values in `IUSE_EFFECTIVE` are those which an ebuild may legally use in queries about itself (for example, for the `use` function, and for use in dependency specification conditional blocks).
- **USE**: is a variable calculated by the package manager and exported to the ebuild environment.

In all cases, the values of `IUSE_REFERENCEABLE` and `IUSE_EFFECTIVE` are undefined during metadata generation.

For EAPIs listed in table 5.6 as not supporting profile defined `IUSE` injection, `IUSE_REFERENCEABLE` is equal to the calculated `IUSE` value, and `IUSE_EFFECTIVE` contains the following values:

- All values in the calculated `IUSE` value.
- All possible values for the `ARCH` variable.
- All legal use flag names whose name starts with the lower-case equivalent of any value in the profile `USE_EXPAND` variable followed by an underscore.

**Feature:** `profile-iuse-inject` (EAPI 5+)

For EAPIs listed in table 5.6 as supporting profile defined `IUSE` injection, `IUSE_REFERENCEABLE` and `IUSE_EFFECTIVE` are equal and contain the following values:

- All values in the calculated `IUSE` value.
- All values in the profile `IUSE_IMPLICIT` variable.
- All values in the profile variable named `USE_EXPAND_VALUES_${v}`, where `${v}` is any value in the intersection of the profile `USE_EXPAND_UNPREFIXED` and `USE_EXPAND_IMPLICIT` variables.
- All values for `${lower_v}_${x}`, where `${x}` is all values in the profile variable named `USE_EXPAND_VALUES_${v}`, where `${v}` is any value in the intersection of the profile `USE_EXPAND` and `USE_EXPAND_IMPLICIT` variables and `${lower_v}` is the lower-case equivalent of `${v}`.

The `USE` variable is set by the package manager. For each value in `IUSE_EFFECTIVE`, `USE` shall contain that value if the flag is to be enabled for the ebuild in question, and shall not contain that value if it is to be disabled. In EAPIs listed in table 5.6 as not supporting profile defined `IUSE` injection, `USE` may contain other flag names that are not relevant for the ebuild.

For EAPIs listed in table 5.6 as supporting profile defined `IUSE` injection, the variables named in `USE_EXPAND` and `USE_EXPAND_UNPREFIXED` shall have their profile-provided values reduced to contain only those values that are present in `IUSE_EFFECTIVE`.

For EAPIs listed in table 5.6 as supporting profile defined `IUSE` injection, the package manager must save the calculated value of `IUSE_EFFECTIVE` when installing a package. Details are beyond the scope of this specification.

## 11.1.2 REPLACING_VERSIONS and REPLACED_BY_VERSION

**Feature:** `replace-version-vars` (EAPI 4+)

In EAPIs listed in table 11.3 as supporting it, the `REPLACING_VERSIONS` variable shall be defined in `pkg_preinst` and `pkg_postinst`. In addition, it *may* be defined in `pkg_pretend` and `pkg_setup`, although ebuild authors should take care to handle binary package creation and installation correctly when using it in these phases.

`REPLACING_VERSIONS` is a list, not a single optional value, to handle pathological cases such as installing `foo-2:2` to replace `foo-2:1` and `foo-3:2`.

In EAPIs listed in table 11.3 as supporting it, the `REPLACED_BY_VERSION` variable shall be defined in `pkg_prerm` and `pkg_postrm`. It shall contain at most one value.

## 11.1.3 Offset-prefix variables

**Feature:** `offset-prefix-vars` (EAPI 3+)

### Table 11.7: EAPIs supporting offset-prefix

| EAPI | Supports offset-prefix? |
|------|-------------------------|
| 0, 1, 2 | No |
| 3, 4, 5, 6, 7, 8, 9 | Yes |

Table 11.7 lists the EAPIs which support offset-prefix installations. This support was initially added in EAPI 3, in the form of three extra variables. Two of these, `EROOT` and `ED`, are convenience variables using the variable `EPREFIX`. In EAPIs that do not support an offset-prefix, the installation offset is hardwired to `/usr`. In offset-prefix supporting EAPIs the installation offset is set as `${EPREFIX}/usr` and hence can be adjusted using the variable `EPREFIX`. Note that the behaviour of offset-prefix aware and agnostic is the same when `EPREFIX` is set to the empty string in offset-prefix aware EAPIs. The latter do have the variables `ED` and `EROOT` properly set, though.

## 11.1.4 Path variables and trailing slash

Unless specified otherwise, the paths provided through package manager variables do not end with a trailing slash. Consequently, the system root directory will be represented by the empty string. A few exceptions to this rule are listed in table 11.8 along with applicable EAPIs.

For EAPIs where those variables are defined to always end with a trailing slash, the package manager guarantees that a trailing slash will always be appended to the path in question. If the path specifies the system root directory, it will consist of a single slash (`/`).

**Feature:** `trailing-slash` (EAPI 7+)

For EAPIs where those variables are defined to never end with a trailing slash, the package manager guarantees that a trailing slash will never be present. If the path specifies the system root directory, it will be empty.

### Table 11.8: Variables that always or never end with a trailing slash

| EAPI | ROOT, EROOT | D, ED |
|------|-------------|-------|
| 0, 1, 2, 3, 4, 5, 6 | always | always |
| 7, 8, 9 | never | never |

## 11.2 The state of variables between functions

Exported and default scope variables are saved between functions. A non-local variable set in a function earlier in the call sequence must have its value preserved for later functions, including functions executed as part of a later uninstall.

**Note:** `pkg_pretend` is *not* part of the normal call sequence, and does not take part in environment saving.

Variables that were exported must remain exported in later functions; variables with default visibility may retain default visibility or be exported. Variables with special meanings to the package manager are excluded from this rule.

Global variables must only contain invariant values (see section 7.1). If a global variable's value is invariant, it may have the value that would be generated at any given point in the build sequence.

### Example: Environment state between functions

```bash
GLOBAL_VARIABLE="a"

src_compile()
{
    GLOBAL_VARIABLE="b"
    DEFAULT_VARIABLE="c"
    export EXPORTED_VARIABLE="d"
    local LOCAL_VARIABLE="e"
}

src_install(){
    [[ ${GLOBAL_VARIABLE} == "a" ]] \
        || [[ ${GLOBAL_VARIABLE} == "b" ]] \
        || die "broken env saving for globals"

    [[ ${DEFAULT_VARIABLE} == "c" ]] \
        || die "broken env saving for default"

    [[ ${EXPORTED_VARIABLE} == "d" ]] \
        || die "broken env saving for exported"

    [[ $(printenv EXPORTED_VARIABLE ) == "d" ]] \
        || die "broken env saving for exported"

    [[ -z ${LOCAL_VARIABLE} ]] \
        || die "broken env saving for locals"
}
```

## 11.3 The state of the system between functions

For the sake of this section:

- **Variancy** is any package manager action that modifies either `ROOT` or `/` in any way that isn't merely a simple addition of something that doesn't alter other packages. This includes any non-default call to any `pkg` phase function except `pkg_setup`, a merge of any package or an unmerge of any package.
- As an exception, changes to `DISTDIR` do not count as variancy.
- The `pkg_setup` function may be assumed not to introduce variancy. Thus, ebuilds must not perform variant actions in this phase.

The following exclusivity and invariancy requirements are mandated:

- No variancy shall be introduced at any point between a package's `pkg_setup` being started up to the point that that package is merged, except for any variancy introduced by that package.
- There must be no variancy between a package's `pkg_setup` and a package's `pkg_postinst`, except for any variancy introduced by that package.
- Any non-default `pkg` phase function must be run exclusively.
- Each phase function must be called at most once during the build process for any given package.

## Implementation Notes

### For GRPM implementation:

**Environment Variable Management:**
- Set all required variables before sourcing ebuild
- Export variables according to EAPI (table 11.1): EAPI 0-8 exports all, EAPI 9 keeps as shell vars
- Calculate derived variables: `PF = ${PN}-${PVR}`, `EROOT = ${ROOT}${EPREFIX}`, etc.
- Handle trailing slashes correctly (table 11.8): EAPI 0-6 always include, EAPI 7+ never include

**Variable Consistency:**
- Track which variables are consistent across phases vs. which can change
- `EBUILD_PHASE` and `EBUILD_PHASE_FUNC` must be updated for each phase
- `ROOT`, `SYSROOT`, `BROOT` may change between phases - ebuilds must recalculate derived values

**EAPI-Specific Variables:**
- EAPI 0-3: Support `AA`, `KV`, `PORTDIR`, `ECLASSDIR`, `DESTTREE`, `INSDESTTREE`
- EAPI 3+: Add offset-prefix support (`EPREFIX`, `EROOT`, `ED`)
- EAPI 4+: Add `MERGE_TYPE`, `REPLACING_VERSIONS`, `REPLACED_BY_VERSION`
- EAPI 5+: Add `EBUILD_PHASE_FUNC`
- EAPI 7+: Add `SYSROOT`, `ESYSROOT`, `BROOT` for cross-compilation, remove PORTDIR/ECLASSDIR
- EAPI 9: Don't export variables (except `TMPDIR`, `HOME`)

**USE Flag Handling:**
- Calculate `IUSE_EFFECTIVE` from `IUSE`, `ARCH`, `USE_EXPAND`, profile injection
- Populate `USE` variable with enabled flags from `IUSE_EFFECTIVE`
- EAPI 5+: Implement profile-defined IUSE injection
- Save `IUSE_EFFECTIVE` to installed package database

**Environment Saving:**
- Save environment between phases (except `pkg_pretend`)
- Preserve exported variables as exported, default visibility variables may be exported
- Don't save local variables (declared with `local`)
- Special variables (like `EBUILD_PHASE`) are exempt from normal saving rules

**System State Requirements:**
- No variancy between `pkg_setup` and package merge
- No variancy between `pkg_setup` and `pkg_postinst`
- Run non-default `pkg` phases exclusively (no concurrent execution)
- Each phase function called at most once per package build

**Locale Settings:**
- EAPI 6+: Ensure `LC_CTYPE` and `LC_COLLATE` are POSIX-equivalent for ASCII range

**Environment Cleanup:**
- Unset: `GZIP`, `BZIP`, `BZIP2`, `CDPATH`, `GREP_OPTIONS`, `GREP_COLOR`, `GLOBIGNORE`
- EAPI 4+: Unset any variable in `ENV_UNSET` profile variable

**Testing:**
- Test environment saving between phases
- Verify EAPI-specific variable availability
- Test offset-prefix calculations (EAPI 3+)
- Verify trailing slash handling (EAPI 7+)
- Test cross-compilation variables (EAPI 7+)

## References

- [PMS Chapter 11: The ebuild environment](https://projects.gentoo.org/pms/latest/pms.html#the-ebuild-environment)
- [Gentoo Developer Manual: Environment](https://devmanual.gentoo.org/ebuild-writing/environment/)
