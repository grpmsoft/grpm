# Chapter 7: Ebuild-defined variables

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#ebuilddefined-variables)
> **EAPI:** All versions (with EAPI-specific features noted)
> **Last Updated:** 2025-12-14 (PMS commit 43857e7)

## Overview

This chapter describes variables that may or must be defined by ebuilds. For variables that are passed from the package manager to the ebuild, see section 11.1.

If any of these variables are set to invalid values, or if any of the mandatory variables are undefined, the package manager's behaviour is undefined; ideally, an error in one ebuild should not prevent operations upon other ebuilds or packages.

## 7.1 Metadata invariance

All ebuild-defined variables discussed in this chapter must be defined independently of any system, profile or tree dependent data, and must not vary depending upon the ebuild phase. In particular, ebuild metadata can and will be generated on a different system from that upon which the ebuild will be used, and the ebuild must generate identical metadata every time it is used.

Globally defined ebuild variables without a special meaning must similarly not rely upon variable data.

## 7.2 Mandatory ebuild-defined variables

All ebuilds must define at least the following variables:

### DESCRIPTION

A short human-readable description of the package's purpose. May be defined by an eclass. Must not be empty.

### SLOT

The package's slot. Must be a valid slot name, as per section 3.1.3. May be defined by an eclass. Must not be empty.

In EAPIs shown in table 8.7 as supporting sub-slots, the `SLOT` variable may contain an optional sub-slot part that follows the regular slot and is delimited by a `/` character. The sub-slot must be a valid slot name, as per section 3.1.3. The sub-slot is used to represent cases in which an upgrade to a new version of a package with a different sub-slot may require dependent packages to be rebuilt. When the sub-slot part is omitted from the `SLOT` definition, the package is considered to have an implicit sub-slot which is equal to the regular slot.

## 7.3 Optional ebuild-defined variables

Ebuilds may define any of the following variables. Unless otherwise stated, any of them may be defined by an eclass.

### EAPI

The EAPI. See section 7.3.1 below.

### HOMEPAGE

The URI or URIs for a package's homepage, including protocols. See section 8.2 for full syntax.

### SRC_URI

A list of source URIs for the package. Valid protocols are `http://`, `https://`, `ftp://` and `mirror://` (see section 4.4.2 for mirror behaviour). Fetch restricted packages may include URL parts consisting of just a filename. See section 7.3.2 for description and section 8.2 for full syntax.

### LICENSE

The package's license. Each text token must be a valid license name, as per section 3.1.7, and must correspond to a tree "licenses/" entry (see section 4.5). See section 8.2 for full syntax.

### KEYWORDS

A whitespace separated list of keywords for the ebuild. Each token must be a valid keyword name, as per section 3.1.8. See section 7.3.3 for full syntax.

### IUSE

The `USE` flags used by the ebuild. Any eclass that works with `USE` flags must also set `IUSE`, listing only the variables used by that eclass. The package manager is responsible for merging these values. See section 11.1.1 for discussion on which values must be listed in this variable.

**IUSE defaults:** In EAPIs shown in table 7.1 as supporting `IUSE` defaults, any use flag name in `IUSE` may be prefixed by at most one of a plus or a minus sign. If such a prefix is present, the package manager may use it as a suggestion as to the default value of the use flag if no other configuration overrides it.

### REQUIRED_USE

**Feature:** `required-use` (EAPI 4+)

Zero or more assertions that must be met by the configuration of `USE` flags to be valid for this ebuild. See section 7.3.4 for description and section 8.2 for full syntax. Only in EAPIs listed in table 7.2 as supporting `REQUIRED_USE`.

### PROPERTIES

**Feature:** `properties` (EAPI 4+, optional in EAPI 0-3)

Zero or more properties for this package. See section 7.3.5 for value meanings and section 8.2 for full syntax. For EAPIs listed in table 7.2 as having optional support, ebuilds must not rely upon the package manager recognising or understanding this variable in any way.

### RESTRICT

Zero or more behaviour restrictions for this package. See section 7.3.6 for value meanings and section 8.2 for full syntax.

### DEPEND

See chapter 8 (Dependencies).

### BDEPEND

See chapter 8 (Dependencies).

### RDEPEND

See chapter 8 (Dependencies). For some EAPIs, `RDEPEND` has special behaviour for its value if unset and when used with an eclass. See section 7.3.7 for details.

### PDEPEND

See chapter 8 (Dependencies).

### IDEPEND

See chapter 8 (Dependencies).

## Tables

### Table 7.1: EAPIs supporting IUSE defaults

| EAPI | Supports IUSE defaults? |
|------|-------------------------|
| 0 | No |
| 1, 2, 3, 4, 5, 6, 7, 8, 9 | Yes |

### Table 7.2: EAPIs supporting various ebuild-defined variables

| EAPI | Supports PROPERTIES? | Supports REQUIRED_USE? |
|------|----------------------|------------------------|
| 0, 1, 2, 3 | Optionally | No |
| 4, 5, 6, 7, 8, 9 | Yes | Yes |

### Table 7.3: EAPIs supporting SRC_URI arrows and selective URI restrictions

| EAPI | Supports SRC_URI arrows? | Supports selective URI restrictions? |
|------|--------------------------|--------------------------------------|
| 0, 1 | No | No |
| 2, 3, 4, 5, 6, 7 | Yes | No |
| 8, 9 | Yes | Yes |

### Table 7.4: EAPIs with RDEPEND=DEPEND default

| EAPI | RDEPEND=DEPEND? |
|------|-----------------|
| 0, 1, 2, 3 | Yes |
| 4, 5, 6, 7, 8, 9 | No |

### Table 7.5: EAPIs supporting DEFINED_PHASES

| EAPI | Supports DEFINED_PHASES? |
|------|--------------------------|
| 0, 1, 2, 3 | Optionally |
| 4, 5, 6, 7, 8, 9 | Yes |

## 7.3.1 EAPI

An empty or unset `EAPI` value is equivalent to `0`. Ebuilds must not assume that they will get a particular one of these two values if they are expecting one of these two values.

The package manager must either pre-set the `EAPI` variable to `0` or ensure that it is unset before sourcing the ebuild for metadata generation. When using the ebuild for other purposes, the package manager must either pre-set `EAPI` to the value specified by the ebuild's metadata or ensure that it is unset.

If any of these variables are set to invalid values, the package manager's behaviour is undefined; ideally, an error in one ebuild should not prevent operations upon other ebuilds or packages.

If the EAPI is to be specified in an ebuild, the `EAPI` variable must be assigned to precisely once. The assignment must not be preceded by any lines other than blank lines or those that start with optional whitespace (spaces or tabs) followed by a `#` character, and the line containing the assignment statement must match the following regular expression:

```
^[ \t]*EAPI=(['"]?)([A-Za-z0-9+_.-]*)\1[ \t]*([ \t]#.*)?$
```

The package manager must determine the EAPI of an ebuild by parsing its first non-blank and non-comment line, using the above regular expression. If it matches, the EAPI is the substring matched by the capturing parentheses (`0` if empty), otherwise it is `0`. For a recognised EAPI, the package manager must make sure that the `EAPI` value obtained by sourcing the ebuild with bash is identical to the EAPI obtained by parsing. The ebuild must be treated as invalid if these values are different.

Eclasses must not attempt to modify the `EAPI` variable.

## 7.3.2 SRC_URI

All filename components that are enabled (i.e. not inside a use-conditional block that is not matched) in `SRC_URI` must be available in the `DISTDIR` directory. In addition, these components are used to make the `A` and `AA` variables.

If a component contains a full URI with protocol, that download location must be used. Package managers may also consult mirrors for their files.

The special `mirror://` protocol must be supported. See section 4.4.2 for mirror details.

The `RESTRICT` metadata key can be used to impose additional restrictions upon downloading—see section 7.3.6 for details. Fetch restricted packages may use a simple filename instead of a full URI.

**Feature:** `src-uri-arrows` (EAPI 2+)

In EAPIs listed in table 7.3 as supporting arrows, if an arrow is used, the filename used when saving to `DISTDIR` shall instead be the name on the right of the arrow. When consulting mirrors (except for those explicitly listed on the left of the arrow, if `mirror://` is used), the filename to the right of the arrow shall be requested instead of the filename in the URI.

**Feature:** `uri-restrict` (EAPI 8+)

In EAPIs listed in table 7.3 as supporting selective URI restrictions, the URI protocol can be prefixed by an additional `fetch+` or `mirror+` term. If the ebuild is fetch restricted, the `fetch+` prefix undoes the fetch restriction for the URI (but not the implied mirror restriction). If the ebuild is fetch or mirror restricted, the `mirror+` prefix undoes both fetch and mirror restrictions for the URI.

## 7.3.3 Keywords

Keywords are used to indicate levels of stability of a package on a respective architecture `arch`. The following conventions are used:

- `arch`: Both the package version and the ebuild are widely tested, known to work and not have any serious issues on the indicated platform. This is referred to as a *stable keyword*.
- `~arch`: The package version and the ebuild are believed to work and do not have any known serious bugs, but more testing is required before the package version is considered suitable for obtaining a stable keyword. This is referred to as an *unstable keyword* or a *testing keyword*.
- No keyword: It is not known whether the package will work, or insufficient testing has occurred.
- `-arch`: The package version will not work on the architecture.

The `-*` keyword is used to indicate package versions which are not worth trying to test on unlisted architectures.

An empty `KEYWORDS` variable indicates uncertain functionality on any architecture.

## 7.3.4 USE state constraints

`REQUIRED_USE` contains a list of assertions that must be met by the configuration of `USE` flags to be valid for this ebuild. In order to be matched, a `USE` flag in a terminal element must be enabled (or disabled if it has an exclamation mark prefix).

If the package manager encounters a package version where `REQUIRED_USE` assertions are not met, it must treat this package version as if it was masked. No phase functions must be called.

It is an error for a flag to be used if it is not included in `IUSE_EFFECTIVE`.

## 7.3.5 Properties

The following tokens are permitted inside `PROPERTIES`:

- **interactive**: The package may require interaction with the user via the tty.
- **live**: The package uses "live" source code that may vary each time that the package is installed.
- **test_network**: The package manager may run tests that require an internet connection, even if the ebuild has `RESTRICT=test`.
- **test_privileged**: The package manager may run tests that require superuser privileges, even if the ebuild has `RESTRICT=test`.

Package managers may recognise other tokens. Ebuilds may not rely upon any token being supported.

## 7.3.6 Restrict

The following tokens are permitted inside `RESTRICT`:

- **mirror**: The package's `SRC_URI` entries may not be mirrored, and mirrors should not be checked when fetching.
- **fetch**: The package's `SRC_URI` entries may not be downloaded automatically. If entries are not available, `pkg_nofetch` is called. Implies `mirror`.
- **strip**: No stripping of debug symbols from files to be installed may be performed. In EAPIs listed in table 12.19 as supporting controllable stripping, this behaviour may be altered by the `dostrip` command.
- **userpriv**: The package manager may not drop superuser privileges when building the package.
- **test**: The `src_test` phase must not be run.

Package managers may recognise other tokens, but ebuilds may not rely upon them being supported.

## 7.3.7 RDEPEND value

**Feature:** `rdepend-depend` (EAPI 0-3 only)

In EAPIs listed in table 7.4 as having `RDEPEND=DEPEND`, if `RDEPEND` is unset (but not if it is set to an empty string) in an ebuild, when generating metadata the package manager must treat its value as being equal to the value of `DEPEND`.

When dealing with eclasses, only values set in the ebuild itself are considered for this behaviour; any `DEPEND` or `RDEPEND` set in an eclass does not change the implicit `RDEPEND=DEPEND` for the ebuild portion, and any `DEPEND` value set in an eclass does not get treated as being part of `RDEPEND`.

## 7.4 Magic ebuild-defined variables

The following variables must be defined by `inherit` (see section 10.1), and may be considered to be part of the ebuild's metadata:

- **ECLASS**: The current eclass, or unset if there is no current eclass. This is handled magically by `inherit` and must not be modified manually.
- **INHERITED**: List of inherited eclass names. Again, this is handled magically by `inherit`.

**Note:** Thus, by extension of section 7.1, `inherit` may not be used conditionally, except upon constant conditions.

The following is a special variable defined by the package manager for internal use and may or may not be available in the ebuild environment:

- **DEFINED_PHASES**: **Feature:** `defined-phases` (EAPI 4+, optional in EAPI 0-3)

  A space separated arbitrarily ordered list of phase names (e.g. `configure setup unpack`) whose phase functions are defined by the ebuild or an eclass inherited by the ebuild. If no phase functions are defined, a single hyphen is used instead of an empty string. For EAPIs listed in table 7.5 as having optional `DEFINED_PHASES` support, package managers may not rely upon the metadata cache having this variable defined, and must treat an empty string as "this information is not available".

**Note:** Thus, by extension of section 7.1, phase functions must not be defined based upon any variant condition.

For EAPIs listed in table 11.1 with the property that variables are not exported, the package manager must not export any of the variables specified in this section to the environment.

## Implementation Notes

### For GRPM implementation:

**Metadata Parsing:**
- Implement EAPI detection via regex parsing (first non-comment line)
- Validate that parsed EAPI matches sourced EAPI value
- Handle default EAPI 0 for unset/empty EAPI

**Variable Validation:**
- Enforce mandatory variables: `DESCRIPTION`, `SLOT`
- Validate slot names (section 3.1.3)
- Parse sub-slots for EAPI 5+ (format: `SLOT="0/1.2"`)

**SRC_URI Processing:**
- Support `http://`, `https://`, `ftp://`, `mirror://` protocols
- Implement arrow syntax for EAPI 2+ (`URI -> filename`)
- Implement selective URI restrictions for EAPI 8+ (`fetch+` / `mirror+`)
- Generate `A` and `AA` variables from enabled URIs

**USE Flag Handling:**
- Merge `IUSE` from ebuild and inherited eclasses
- Parse IUSE defaults for EAPI 1+ (`+flag`, `-flag`)
- Validate `REQUIRED_USE` constraints for EAPI 4+
- Check flags against `IUSE_EFFECTIVE`

**Keywords:**
- Parse keyword list: `arch`, `~arch`, `-arch`, `-*`
- Distinguish stable vs testing vs blocked keywords

**RESTRICT/PROPERTIES:**
- Parse space-separated token lists
- Handle recognized tokens: mirror, fetch, strip, userpriv, test
- Support EAPI-specific features (PROPERTIES mandatory in EAPI 4+)

**RDEPEND Special Behaviour:**
- For EAPI 0-3: if `RDEPEND` is unset (not empty), default to `DEPEND` value
- Only apply to ebuild-defined values, not eclass-defined

**Magic Variables:**
- Track `ECLASS`, `INHERITED` during eclass sourcing
- Generate `DEFINED_PHASES` list from defined phase functions
- Use `-` for empty DEFINED_PHASES

**Metadata Invariance:**
- Ensure all variables are deterministic (no system/profile dependency)
- Cache metadata separately from runtime execution
- Validate that metadata doesn't change between systems

## References

- [PMS Chapter 7: Ebuild-defined variables](https://projects.gentoo.org/pms/latest/pms.html#ebuilddefined-variables)
- [EAPI Cheat Sheet](https://dev.gentoo.org/~ulm/pms/head/pms.html)
- [Gentoo Developer Manual: Variables](https://devmanual.gentoo.org/ebuild-writing/variables/)
