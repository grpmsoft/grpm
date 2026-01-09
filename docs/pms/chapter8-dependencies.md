# Chapter 8: Dependencies

> **Source:** [Gentoo Package Manager Specification (PMS)](https://projects.gentoo.org/pms/)
>
> This document is derived from the official PMS HTML version. For authoritative information, always refer to the [official PMS](https://wiki.gentoo.org/wiki/Package_Manager_Specification).

---

## 8.1 Dependency Classes

There are three classes of dependencies supported by ebuilds:

- **Build dependencies (`DEPEND`)**: These must be installed and usable before the `pkg_setup` phase function is executed as a part of source build and throughout all `src_*` phase functions executed as part of that build. These may not be installed at all if a binary package is being merged.

- **Runtime dependencies (`RDEPEND`)**: These must be installed and usable before the results of an ebuild merging are treated as usable.

- **Post dependencies (`PDEPEND`)**: These must be installed at some point before the package manager finishes the batch of installs.

### BDEPEND (EAPI 7+)

Additionally, in EAPIs supporting `BDEPEND`, the build dependencies are split into two subclasses:

- **`BDEPEND`** build dependencies that are binary compatible with the native build system (`CBUILD`). The ebuild is allowed to call binary executables installed by this kind of dependency.

- **`DEPEND`** build dependencies that are binary compatible with the system being built (`CHOST`). The ebuild must not execute binary executables installed by this kind of dependency.

### IDEPEND (EAPI 8+)

Additionally, in EAPIs supporting `IDEPEND`, install-time dependencies can be specified. These dependencies are binary compatible with the native build system (`CBUILD`). Ebuilds are allowed to call them in `pkg_preinst` and `pkg_postinst`. Ebuilds may also call them in `pkg_prerm` and `pkg_postrm` but must not rely on them being available.

### Table 8.1: Dependency classes required to be satisfied for a particular phase function

| Phase function | Satisfied dependency classes |
|----------------|------------------------------|
| `pkg_pretend`, `pkg_info`, `pkg_nofetch` | None (ebuilds can rely only on the packages in the system set) |
| `pkg_setup` | Same as `src_unpack` if executed as part of source build, same as `pkg_pretend` otherwise |
| `src_unpack`, `src_prepare`, `src_configure`, `src_compile`, `src_test`, `src_install` | `DEPEND`, `BDEPEND` |
| `pkg_preinst`, `pkg_postinst`, `pkg_prerm`, `pkg_postrm` | `RDEPEND`, `IDEPEND` |
| `pkg_config` | `RDEPEND`, `PDEPEND` |

### Table 8.2: Summary of other interfaces related to dependency classes

| | `BDEPEND`, `IDEPEND` | `DEPEND` | `RDEPEND`, `PDEPEND` |
|---|---|---|---|
| Binary compatible with | `CBUILD` | `CHOST` | `CHOST` |
| Base unprefixed path | `/` | `${SYSROOT}` | `${ROOT}` |
| Relevant offset-prefix | `${BROOT}` | See table 8.3 | `${EPREFIX}` |
| Path combined with prefix | `${BROOT}` | `${ESYSROOT}` | `${EROOT}` |
| PM query command option | `-b` | `-d` | `-r` |

### Table 8.3: Prefix values for `DEPEND`

| If `SYSROOT` is: | `${ROOT}` | Empty, and `ROOT` is non-empty | Other |
|---|---|---|---|
| Then offset-prefix is: | `${EPREFIX}` | `${BROOT}` | Empty |
| And `ESYSROOT` is: | `${EROOT}` | `${BROOT}` | `${SYSROOT}` |

### Table 8.4: EAPIs supporting additional dependency types

| EAPI | Supports `BDEPEND`? | Supports `IDEPEND`? |
|------|---------------------|---------------------|
| 0, 1, 2, 3, 4, 5, 6 | No | No |
| 7 | Yes | No |
| 8, 9 | Yes | Yes |

In addition, `HOMEPAGE`, `SRC_URI`, `LICENSE`, `REQUIRED_USE`, `PROPERTIES` and `RESTRICT` use dependency-style specifications to specify their values.

---

## 8.2 Dependency Specification Format

The following elements are recognised in at least one class of specification. All elements must be surrounded on both sides by whitespace, except at the start and end of the string.

- **A package dependency specification.** Permitted in `DEPEND`, `BDEPEND`, `RDEPEND`, `PDEPEND`, `IDEPEND`.

- **A URI**, in the form `proto://host/path`. Permitted in `HOMEPAGE` and `SRC_URI`. In EAPIs supporting `SRC_URI` arrows, may optionally be followed by whitespace, then `->`, then whitespace, then a simple filename when in `SRC_URI`.

- **A flat filename.** Permitted in `SRC_URI`.

- **A license name** (e.g. `GPL-2`). Permitted in `LICENSE`.

- **A use flag name**, optionally preceded by an exclamation mark. Permitted in `REQUIRED_USE`.

- **A simple string.** Permitted in `PROPERTIES` and `RESTRICT`.

- **An all-of group**, which consists of an open parenthesis, followed by whitespace, followed by one or more of (a dependency item of any kind followed by whitespace), followed by a close parenthesis. More formally:
  ```
  all-of ::= '(' whitespace (item whitespace)+ ')'
  ```
  Permitted in all specification style variables.

- **An any-of group**, which consists of the string `||`, followed by whitespace, followed by an open parenthesis, followed by whitespace, followed by one or more of (a dependency item of any kind followed by whitespace), followed by a close parenthesis. More formally:
  ```
  any-of ::= '||' whitespace '(' whitespace (item whitespace)+ ')'
  ```
  Permitted in `DEPEND`, `BDEPEND`, `RDEPEND`, `PDEPEND`, `IDEPEND`, `LICENSE`, `REQUIRED_USE`.

- **An exactly-one-of group**, which has the same format as the any-of group, but begins with the string `^^` instead. Permitted in `REQUIRED_USE`.

- **An at-most-one-of group**, which has the same format as the any-of group, but begins with the string `??` instead. Permitted in `REQUIRED_USE` in EAPIs supporting `REQUIRED_USE ??` groups.

- **A use-conditional group**, which consists of an optional exclamation mark, followed by a use flag name, followed by a question mark, followed by whitespace, followed by an open parenthesis, followed by whitespace, followed by one or more of (a dependency item of any kind followed by whitespace), followed by a close parenthesis. More formally:
  ```
  use-conditional ::= '!'? flag-name '?' whitespace '(' whitespace (item whitespace)+ ')'
  ```
  Permitted in all specification style variables.

**Note:** Whitespace is not optional.

### Table 8.5: EAPIs supporting `REQUIRED_USE ??` groups

| EAPI | Supports `REQUIRED_USE ??` groups? |
|------|-----------------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

### 8.2.1 All-of dependency specifications

In an all-of group, all of the child elements must be matched.

### 8.2.2 USE-conditional dependency specifications

In a use-conditional group, if the associated use flag is enabled (or disabled if it has an exclamation mark prefix), all of the child elements must be matched.

It is an error for a flag to be used if it is not included in `IUSE_EFFECTIVE`.

### 8.2.3 Any-of dependency specifications (`||`)

Any use-conditional group that is an immediate child of an any-of group, if not enabled (disabled for an exclamation mark prefixed use flag name), is not considered a member of the any-of group for match purposes.

In an any-of group, at least one immediate child element must be matched. A blocker is considered to be matched if its associated package dependency specification is not matched.

**Empty group matching:** In EAPIs specified in table 8.6, an empty any-of group counts as being matched.

### 8.2.4 Exactly-one-of dependency specifications (`^^`)

Any use-conditional group that is an immediate child of an exactly-one-of group, if not enabled (disabled for an exclamation mark prefixed use flag name), is not considered a member of the exactly-one-of group for match purposes.

In an exactly-one-of group, exactly one immediate child element must be matched.

In EAPIs specified in table 8.6, an empty exactly-one-of group counts as being matched.

### Table 8.6: Matching of empty dependency groups in EAPIs

| EAPI | Empty `||` and `^^` groups are matched? |
|------|----------------------------------------|
| 0, 1, 2, 3, 4, 5, 6 | Yes |
| 7, 8, 9 | No |

### 8.2.5 At-most-one-of dependency specifications (`??`)

Any use-conditional group that is an immediate child of an at-most-one-of group, if not enabled (disabled for an exclamation mark prefixed use flag name), is not considered a member of the at-most-one-of group for match purposes.

In an at-most-one-of group, at most one immediate child element must be matched.

---

## 8.3 Package Dependency Specifications

A package dependency can be in one of the following base formats. A package manager must warn or error on non-compliant input.

- A simple `category/package` name.
- An operator, as described in section 8.3.1, followed immediately by `category/package`, followed by a hyphen, followed by a version specification.

In EAPIs supporting `SLOT` dependencies, either of the above formats may additionally be suffixed by a `:slot` restriction, as described in section 8.3.3. A package manager must warn or error if slot dependencies are used with an EAPI not supporting `SLOT` dependencies.

In EAPIs supporting 2-style or 4-style `USE` dependencies, a specification may additionally be suffixed by at most one 2-style or 4-style `[use]` restriction, as described in section 8.3.4. A package manager must warn or error if this feature is used with an EAPI not supporting use dependencies.

> **Note:** Order is important. The slot restriction must come before use dependencies.

### Table 8.7: Support for `SLOT` dependencies and sub-slots in EAPIs

| EAPI | Supports `SLOT` dependencies? | Supports sub-slots? |
|------|-------------------------------|---------------------|
| 0 | No | No |
| 1, 2, 3, 4 | Named only | No |
| 5, 6, 7, 8, 9 | Named and operator | Yes |

### Table 8.8: EAPIs supporting `USE` dependencies

| EAPI | Supports `USE` dependencies? |
|------|------------------------------|
| 0, 1 | No |
| 2, 3 | 2-style |
| 4, 5, 6, 7, 8, 9 | 4-style |

### 8.3.1 Operators

The following operators are available:

| Operator | Description |
|----------|-------------|
| `<` | Strictly less than the specified version. |
| `<=` | Less than or equal to the specified version. |
| `=` | Exactly equal to the specified version. Special exception: if the version specified has an asterisk immediately following it, then only the given number of version components is used for comparison, i.e. the asterisk acts as a wildcard for any further components. When an asterisk is used, the specification must remain valid if the asterisk were removed. (An asterisk used with any other operator is illegal.) |
| `~` | Equal to the specified version when revision parts are ignored. |
| `>=` | Greater than or equal to the specified version. |
| `>` | Strictly greater than the specified version. |

**Examples:**

```bash
# Exact version
=app-misc/hello-2.10

# Minimum version
>=sys-libs/zlib-1.2.11

# Version range (using wildcard)
=dev-lang/python-3.9*

# Revision-ignoring match
~app-editors/vim-8.2
```

### 8.3.2 Block operator

If the specification is prefixed with one or two exclamation marks, the named dependency is a block rather than a requirement--that is to say, the specified package must not be installed. As an exception, weak blocks on the package version of the ebuild itself do not count.

There are two strengths of block: **weak** and **strong**. A weak block may be ignored by the package manager, so long as any blocked package will be uninstalled later on. A strong block must not be ignored.

### Table 8.9: Exclamation mark strengths for EAPIs

| EAPI | `!` | `!!` |
|------|-----|------|
| 0, 1 | Unspecified | Forbidden |
| 2, 3, 4, 5, 6, 7, 8, 9 | Weak | Strong |

**Examples:**

```bash
# Weak block (can be uninstalled later)
!app-misc/conflicting-package

# Strong block (must not be installed at all)
!!sys-apps/dangerous-conflict
```

### 8.3.3 Slot dependencies

A **named slot dependency** consists of a colon followed by a slot name. A specification with a named slot dependency matches only if the slot of the matched package is equal to the slot specified. If the slot of the package to match cannot be determined (e.g. because it is not a supported EAPI), the match is treated as unsuccessful.

In EAPIs supporting sub-slots, a slot dependency may contain an optional sub-slot part that follows the regular slot and is delimited by a `/` character.

#### Slot Operators (EAPI 5+)

An **operator slot dependency** consists of a colon followed by one of the following operators:

| Operator | Description |
|----------|-------------|
| `*` | Indicates that any slot value is acceptable. In addition, for runtime dependencies, indicates that the package will not break if the matched package is uninstalled and replaced by a different matching package in a different slot. |
| `=` | Indicates that any slot value is acceptable. In addition, for runtime dependencies, indicates that the package will break unless a matching package with slot and sub-slot equal to the slot and sub-slot of the best version installed as a build-time (`DEPEND`) dependency is available. |
| `slot=` | Indicates that only a specific slot value is acceptable, and otherwise behaves identically to the `=` operator. The specified slot must not contain a sub-slot part. |

To implement the equals slot operators `=` and `slot=`, the package manager will need to store the slot/sub-slot pair of the best installed version of the matching package. This syntax is only for package manager use and must not be used by ebuilds.

**Important:** Whenever the equals slot operator is used in an enabled dependency group, the dependencies (`DEPEND`) must ensure that a matching package is installed at build time. It is invalid to use the equals slot operator inside `PDEPEND` or inside any-of dependency specifications.

**Examples:**

```bash
# Specific slot
dev-lang/python:3.11

# Slot with sub-slot
sys-libs/zlib:0/1.2.11

# Any slot, rebuilds tracked
dev-libs/openssl:=

# Specific slot, rebuilds tracked
dev-qt/qtcore:5=

# Any slot, no rebuild tracking
media-libs/libpng:*
```

### 8.3.4 2-style and 4-style USE dependencies

A 2-style or 4-style use dependency consists of one of the following:

| Syntax | Description |
|--------|-------------|
| `[opt]` | The flag must be enabled. |
| `[opt=]` | The flag must be enabled if the flag is enabled for the package with the dependency, or disabled otherwise. |
| `[!opt=]` | The flag must be disabled if the flag is enabled for the package with the dependency, or enabled otherwise. |
| `[opt?]` | The flag must be enabled if the flag is enabled for the package with the dependency. |
| `[!opt?]` | The flag must be disabled if the use flag is disabled for the package with the dependency. |
| `[-opt]` | The flag must be disabled. |

Multiple requirements may be combined using commas, e.g. `[first,-second,third?]`.

When multiple requirements are specified, all must match for a successful match.

#### 4-style USE dependency defaults

In a 4-style use dependency, the flag name may immediately be followed by a *default* specified by either `(+)` or `(-)`. The former indicates that, when applying the use dependency to a package that does not have the flag in question in `IUSE_REFERENCEABLE`, the package manager shall behave as if the flag were present and enabled; the latter, present and disabled.

Unless a 4-style default is specified, it is an error for a use dependency to be applied to an ebuild which does not have the flag in question in `IUSE_REFERENCEABLE`.

> **Note:** By extension of the above, a default that could reference an ebuild using an EAPI not supporting profile `IUSE` injections cannot rely upon any particular behaviour for flags that would not have to be part of `IUSE`.

It is an error for an ebuild to use a conditional use dependency when that ebuild does not have the flag in `IUSE_EFFECTIVE`.

**Examples:**

```bash
# Flag must be enabled
dev-libs/libfoo[ssl]

# Flag must be disabled
net-misc/curl[-gnutls]

# Flag must match this package's USE
media-libs/libav[mp3=]

# Multiple requirements
dev-libs/openssl[ssl,zlib,-static-libs]

# With defaults (4-style)
app-text/ghostscript[cups(+)]
sys-libs/readline[unicode(-)]
```

---

## Summary of Dependency Syntax

### Complete Atom Format

```
[block][operator]category/package[-version][:slot[/subslot][slot-op]][use-deps]
```

Where:
- `block`: `!` (weak) or `!!` (strong)
- `operator`: `<`, `<=`, `=`, `~`, `>=`, `>`
- `slot-op`: `*`, `=`
- `use-deps`: `[flag1,flag2,...]`

### Examples

```bash
# Basic
app-misc/hello

# With version constraints
>=sys-libs/zlib-1.2.11
<dev-lang/python-4
=app-editors/vim-8.2*
~net-misc/wget-1.21

# With slot
dev-lang/python:3.11
sys-libs/ncurses:0/6

# With slot operator (rebuild tracking)
dev-libs/openssl:0=
media-libs/libpng:0/16=

# With USE dependencies
net-misc/curl[ssl]
dev-libs/libxml2[python,-doc]

# Combined
>=dev-libs/boost-1.74:=[python,threads]
!!<sys-apps/portage-2.3

# USE conditionals in dependency string
DEPEND="
    ssl? ( dev-libs/openssl:= )
    !ssl? ( net-libs/gnutls )
    || ( dev-lang/python:3.11 dev-lang/python:3.10 )
"
```

---

*Converted from PMS HTML for GRPM development reference.*
