# Chapter 5: Profiles

> **Source:** [Gentoo Package Manager Specification (PMS)](https://projects.gentoo.org/pms/)
>
> This document is derived from the official PMS HTML version. For authoritative information, always refer to the [official PMS](https://wiki.gentoo.org/wiki/Package_Manager_Specification).

---

## 5.1 General principles

Generally, a profile defines information specific to a certain 'type' of system—it lies somewhere between repository-level defaults and user configuration in that the information it contains is not necessarily applicable to all machines, but is sufficiently general that it should not be left to the user to configure it. Some parts of the profile can be overridden by user configuration, some only by another profile.

The format of a profile is relatively simple. Each profile is a directory containing any number of the files described in this chapter, and possibly inheriting another profile. The files themselves follow a few basic conventions as regards inheritance and format; these are described in the next section. It may also contain any number of subdirectories containing other profiles.

## 5.2 Files that make up a profile

### 5.2.1 The parent file

A profile may contain a `parent` file. Each line must contain a relative path to another profile which will be considered as one of this profile's parents. Any settings from the parent are inherited by this profile, and can be overridden by it. Precise rules for how settings are combined with the parent profile vary between files, and are described below. Parents are handled depth first, left to right, with duplicate parent paths being sourced for every time they are encountered.

It is illegal for a profile's parent tree to contain cycles. Package manager behaviour upon encountering a cycle is undefined.

This file must not make use of line continuations. Blank lines and those beginning with a `#` are discarded.

### 5.2.2 The eapi file

A profile directory may contain an `eapi` file. This file, if it exists, must contain a single line with the name of an EAPI. This specifies the EAPI to use when handling the directory in question; a package manager must not attempt to use any profile using a directory which requires an EAPI it does not support.

If no `eapi` file is present, the default depends on the EAPI of the top-level profiles directory (see section 4.4). For EAPIs 0-8, EAPI 0 shall be used. For EAPI 9+, the top-level EAPI shall be used.

**Table 5.1: Default EAPI for profiles**

| EAPI | Default EAPI? |
|------|---------------|
| 0, 1, 2, 3, 4, 5, 6, 7, 8 | 0 |
| 9 | Top-level |

The EAPI is neither inherited via the `parent` file nor in subdirectories.

### 5.2.3 deprecated

If a profile contains a file named `deprecated`, it is treated as such. The first line of this file should contain the path from the `profiles` directory of the repository to a valid profile that is the recommended upgrade path from this profile. The remainder of the file can contain any text, which may be displayed to users using this profile by the package manager. This file is not inherited—profiles which inherit from a deprecated profile are *not* deprecated.

This file must not contain comments or make use of line continuations.

### 5.2.4 make.defaults

`make.defaults` is used to define defaults for various environment and configuration variables. This file is unusual in that it is not combined at a file level with the parent—instead, each variable is combined or overridden individually as described in section 5.3.

The file itself is a line-based key-value format. Each line contains a single `VAR="value"` entry, where the value must be double quoted. A variable name must start with one of `a-zA-Z` and may contain `a-zA-Z0-9_` only. Additional syntax, which is a small subset of bash syntax, is allowed as follows:

- Variables to the right of the equals sign in the form `${foo}` or `$foo` are recognised and expanded from variables previously set in this or earlier `make.defaults` files.
- One logical line may be continued over multiple physical lines by escaping the newline with a backslash. A quoted string may be continued over multiple physical lines by either a simple newline or a backslash-escaped newline.
- Backslashes, except for line continuations, are not allowed.

### 5.2.5 Simple line-based files

These files are a simple one-item-per-line list, which is inherited in the following manner: the parent profile's list is taken, and the current profile's list appended. If any line begins with a hyphen, then any lines previous to it whose contents are equal to the remainder of that line are removed from the list. Blank lines and those beginning with a `#` are discarded.

In EAPIs 7+, any of the files `package.mask`, `package.use`, `use.*` and `package.use.*` mentioned below can be a directory instead of a regular file. Files contained in that directory, unless their name begins with a dot, will be concatenated in order of their filename in the POSIX locale and the result will be processed as if it were a single file. Any subdirectories will be ignored.

**Table 5.2: EAPIs supporting directories for profile files**

| EAPI | Supports directories for profile files? |
|------|----------------------------------------|
| 0, 1, 2, 3, 4, 5, 6 | No |
| 7, 8, 9 | Yes |

### 5.2.6 packages

The `packages` file is used to define the 'system set' for this profile. After the above rules for inheritance and comments are applied, its lines must take one of two forms: a package dependency specification prefixed by `*` denotes that it forms part of the system set. A package dependency specification on its own may also appear for legacy reasons, but should be ignored when calculating the system set.

### 5.2.7 packages.build

The `packages.build` file is used by Gentoo's Catalyst tool to generate stage1 tarballs, and has no relevance to the operation of a package manager. It is thus outside the scope of this document, but is mentioned here for completeness.

### 5.2.8 package.mask

`package.mask` is used to prevent packages from being installed on a given profile. Each line contains one package dependency specification; anything matching this specification will not be installed unless unmasked by the user's configuration. In EAPIs 7+, `package.mask` can be a directory instead of a regular file as per section 5.2.5.

Note that the `-spec` syntax can be used to remove a mask in a parent profile, but not necessarily a global mask (from `profiles/package.mask`, section 4.4).

### 5.2.9 package.provided

`package.provided` is used to tell the package manager that a certain package version should be considered to be provided by the system regardless of whether it is actually installed. Because it has severe adverse effects on USE-based and slot-based dependencies, its use is strongly deprecated and package manager support must be regarded as purely optional.

**Table 5.3: EAPIs supporting package.provided in profiles**

| EAPI | Supports `package.provided`? |
|------|----------------------------|
| 0, 1, 2, 3, 4, 5, 6 | Optionally |
| 7, 8, 9 | No |

### 5.2.10 package.use

The `package.use` file may be used by the package manager to override the default USE flags specified by `make.defaults` on a per package basis. The format is to have a package dependency specification, and then a space delimited list of USE flags to enable. A USE flag in the form of `-flag` indicates that the package should have the USE flag disabled. The package dependency specification is limited to the forms defined by the directory's EAPI. In EAPIs 7+, `package.use` can be a directory instead of a regular file as per section 5.2.5.

### 5.2.11 use.stable and package.use.stable

The `use.stable` and `package.use.stable` files may be used to override the default USE flags specified by `make.defaults`. They only apply to packages that are merged due to a stable keyword in the sense of section 7.3.3. Each line in `use.stable` contains a USE flag to enable; the `-flag` syntax indicates that the flag should be disabled. The `package.use.stable` file uses the same format as `package.use`. `USE_EXPAND` values may be enabled or disabled by using `expand_name_value`.

Stable restrictions are applied exactly when the following condition holds: If every stable keyword in `KEYWORDS` were replaced with its tilde-prefixed counterpart (see section 7.3.3), then the resulting `KEYWORDS` setting would prevent installation of the package.

If a flag appears in more than one of `package.use`, `use.stable` and `package.use.stable`, then `package.use.stable` takes precedence over `package.use`, which in turn takes precedence over `use.stable`.

**Table 5.4: EAPIs supporting use.stable and package.use.stable in profiles**

| EAPI | Supports `use.stable`? | Supports `package.use.stable`? |
|------|----------------------|-------------------------------|
| 0, 1, 2, 3, 4, 5, 6, 7, 8 | No | No |
| 9 | Yes | Yes |

### 5.2.12 USE masking and forcing

This section covers the eight files `use.mask`, `use.force`, `use.stable.mask`, `use.stable.force`, `package.use.mask`, `package.use.force`, `package.use.stable.mask`, and `package.use.stable.force`. They are described together because they interact in a non-trivial manner. In EAPIs 7+, these files can be directories instead of regular files as per section 5.2.5.

Simply speaking, `use.mask` and `use.force` are used to say that a given USE flag must never or always, respectively, be enabled when using this profile. `package.use.mask` and `package.use.force` do the same thing on a per-package, or per-version, basis.

In profile directories with an EAPI supporting stable masking (EAPIs 5+), the same is true for `use.stable.mask`, `use.stable.force`, `package.use.stable.mask` and `package.use.stable.force`. These files, however, only act on packages that are merged due to a stable keyword in the sense of section 7.3.3. Thus, these files can be used to restrict the feature set deemed stable in a package.

**Table 5.5: Profile directory support for masking/forcing use flags in stable versions only**

| EAPI | Supports masking/forcing use flags in stable versions? |
|------|-------------------------------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

The logic for `use.force`, `use.stable.force`, `package.use.force`, and `package.use.stable.force` is identical to masking. If a flag is both masked and forced, the mask is considered to take precedence.

`USE_EXPAND` values may be forced or masked by using `expand_name_value`.

A package manager may treat `ARCH` values that are not the current architecture as being masked.

## 5.3 Profile variables

This section documents variables that have special meaning, or special behaviour, when defined in a profile's `make.defaults` file.

### 5.3.1 Incremental variables

*Incremental* variables must stack between parent and child profiles in the following manner: Beginning with the highest parent profile, tokenise the variable's value based on whitespace and concatenate the lists. Then, for any token T beginning with a hyphen, remove it and any previous tokens whose value is equal to T with the hyphen removed, or, if T is equal to `-*`, remove all previous values. Note that because of this treatment, the order of tokens in the final result is arbitrary, not necessarily related to the order of tokens in any given profile.

The following variables must be treated in this fashion:

- `USE`
- `USE_EXPAND`
- `USE_EXPAND_HIDDEN`
- `CONFIG_PROTECT`
- `CONFIG_PROTECT_MASK`

For EAPIs 5+ supporting profile-defined `IUSE` injection, the following variables must also be treated incrementally:

- `IUSE_IMPLICIT`
- `USE_EXPAND_IMPLICIT`
- `USE_EXPAND_UNPREFIXED`

For EAPIs 7+ using `ENV_UNSET`, the following variable must also be treated incrementally:

- `ENV_UNSET`

**Table 5.6: Profile-defined IUSE injection for EAPIs**

| EAPI | Supports profile-defined `IUSE` injection? |
|------|-----------------------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

**Table 5.7: Profile-defined unsetting of variables in EAPIs**

| EAPI | Supports `ENV_UNSET`? |
|------|--------------------|
| 0, 1, 2, 3, 4, 5, 6 | No |
| 7, 8, 9 | Yes |

Other variables, except where they affect only package-manager-specific functionality (such as Portage's `FEATURES` variable), must not be treated incrementally—later definitions shall completely override those in parent profiles.

### 5.3.2 Specific variables and their meanings

The following variables have specific meanings when set in profiles.

**ARCH**
The system's architecture. Must be a value listed in `profiles/arch.list`; see section 4.4 for more information. Must be equal to the primary `KEYWORD` for this profile.

**CONFIG_PROTECT, CONFIG_PROTECT_MASK**
Contain whitespace-delimited lists used to control the configuration file protection. Described more fully in section 13.3.3.

**USE**
Defines the list of default USE flags for this profile. Flags may be added or removed by the user's configuration. `USE_EXPAND` values must not be specified in this way.

**USE_EXPAND**
Defines a list of variables which are to be treated incrementally, exported to the ebuild environment, and whose contents are to be expanded into the USE variable as passed to ebuilds. See section 11.1.1 for details.

**USE_EXPAND_UNPREFIXED**
Similar to `USE_EXPAND`, but no prefix is used. If the repository contains any package using an EAPI supporting profile-defined `IUSE` injection (see table 5.6), this list must contain at least `ARCH`. See section 11.1.1 for details.

**USE_EXPAND_HIDDEN**
Contains a (possibly empty) subset of names from `USE_EXPAND` and `USE_EXPAND_UNPREFIXED`. The package manager may use this set as a hint to avoid displaying uninteresting or unhelpful information to an end user.

**USE_EXPAND_IMPLICIT, IUSE_IMPLICIT**
Used to inject implicit values into `IUSE`. See section 11.1.1 for details. `USE_EXPAND_IMPLICIT` contains a subset of names from `USE_EXPAND` and `USE_EXPAND_UNPREFIXED`.

**ENV_UNSET**
Contains a whitespace-delimited list of variables that the package manager shall unset. See section 11.1 for details.

In addition, for EAPIs 5+ supporting profile defined `IUSE` injection, the following variables have special handling as described in section 11.1.1:

- All variables named in `USE_EXPAND` and `USE_EXPAND_UNPREFIXED`.
- All `USE_EXPAND_VALUES_${v}` variables, where `${v}` is a value in `USE_EXPAND_IMPLICIT`.

Any other variables set in `make.defaults` must be passed on into the ebuild environment as-is, and are not required to be interpreted by the package manager.

---

*Converted from PMS HTML for GRPM development reference.*
