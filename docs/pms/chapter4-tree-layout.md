# Chapter 4: Tree Layout

> **Source:** [Gentoo Package Manager Specification (PMS)](https://projects.gentoo.org/pms/)
>
> This document is derived from the official PMS HTML version. For authoritative information, always refer to the [official PMS](https://wiki.gentoo.org/wiki/Package_Manager_Specification).

---

This chapter defines the layout on-disk of an ebuild repository. In all cases below where a file or directory is specified, a symlink to a file or directory is also valid. In this case, the package manager must follow the operating system's semantics for symbolic links and must not behave differently from normal.

## 4.1 Top level

An ebuild repository shall occupy one directory on disk, with the following subdirectories:

- One directory per category, whose name shall be the name of the category. The layout of these directories shall be as described in section 4.2.
- A `profiles` directory, described in section 4.4.
- A `licenses` directory (optional), described in section 4.5.
- An `eclass` directory (optional), described in section 4.6.
- A `metadata` directory (optional), described in section 4.7.
- Other optional support files and directories (skeleton ebuilds or ChangeLogs, for example) may exist but are not covered by this specification. The package manager must ignore any of these files or directories that it does not recognise.

## 4.2 Category directories

Each category provided by the repository (see also: the `profiles/categories` file, section 4.4) shall be contained in one directory, whose name shall be that of the category. Each category directory shall contain:

- A `metadata.xml` file, as described in appendix A. Optional.
- Zero or more package directories, one for each package in the category, as described in section 4.3. The name of the package directory shall be the corresponding package name.

Category directories may contain additional files, whose purpose is not covered by this specification. Additional directories that are not for a package may *not* be present, to avoid conflicts with package name directories; an exception is made for filesystem components whose name starts with a dot, which the package manager must ignore, and for any directory named `CVS`.

It is not required that a directory exists for each category provided by the repository. A category directory that does not exist shall be considered equivalent to an empty category (and by extension, a package manager may treat an empty category as a category that does not exist).

## 4.3 Package directories

A package directory contains the following:

- Zero or more ebuilds. These are as described in chapter 6 and others.
- A `metadata.xml` file, as described in appendix A. Optional only for legacy support.
- A `ChangeLog`, in a format determined by the provider of the repository. Optional.
- A `Manifest` file, whose format is described in [GLEP 74](https://www.gentoo.org/glep/glep-0074.html). Can be omitted if the file would be empty.
- A `files` directory, containing any support files needed by the ebuilds. Optional.

Any ebuild in a package directory must be named `name-ver.ebuild`, where `name` is the (unqualified) package name, and `ver` is the package's version. Package managers must ignore any ebuild file that does not match these rules.

A package directory that contains no correctly named ebuilds shall be considered a package with no versions. A package with no versions shall be considered equivalent to a package that does not exist (and by extension, a package manager may treat a package that does not exist as a package with no versions).

A package directory may contain other files or directories, whose purpose is not covered by this specification.

## 4.4 The profiles directory

The profiles directory shall contain zero or more profile directories as described in chapter 5, as well as the following files and directories. In any line-based file, lines beginning with a `#` character are treated as comments, whilst blank lines are ignored. All contents of this directory, with the exception of `repo_name`, are optional.

The profiles directory may contain an `eapi` file. This file, if it exists, must contain a single line with the name of an EAPI. This specifies the EAPI to use when handling the profiles directory; a package manager must not attempt to use any repository whose profiles directory requires an EAPI it does not support. If no `eapi` file is present, EAPI 0 shall be used.

If the repository is not intended to be stand-alone, the contents of these files are to be taken from or merged with the master repository as necessary; this does not apply to the `eapi` file.

Other files not described by this specification may exist, but may not be relied upon. The package manager must ignore any files in this directory that it does not recognise.

### Files in the profiles directory

**arch.list**
Contains a list, one entry per line, of permissible values for the `ARCH` variable, and hence permissible keywords for packages in this repository.

**categories**
Contains a list, one entry per line, of categories provided by this repository.

**eapi**
See above.

**info_pkgs**
Contains a list, one entry per line, of qualified package names. Any package matching one of these is to be listed when a package manager displays a 'system information' listing.

**info_vars**
Contains a list, one entry per line, of profile, configuration, and environment variables which are considered to be of interest. The value of each of these variables may be shown when the package manager displays a 'system information' listing.

**package.mask**
Contains a list, one entry per line, of package dependency specifications (using the directory's EAPI). Any package version matching one of these is considered to be masked, and will not be installed regardless of profile unless it is unmasked by the user configuration.

For EAPIs 7+, `package.mask` can be a directory instead of a regular file. Files contained in that directory, unless their name begins with a dot, will be concatenated in order of their filename in the POSIX locale and the result will be processed as if it were a single file. Any subdirectories will be ignored.

**Table 4.1: EAPIs supporting a directory for package.mask**

| EAPI | `package.mask` can be a directory? |
|------|-----------------------------------|
| 0, 1, 2, 3, 4, 5, 6 | No |
| 7, 8, 9 | Yes |

**profiles.desc**
Described below in section 4.4.1.

**repo_name**
Contains, on a single line, the name of this repository. The repository name must conform to section 3.1.5.

**thirdpartymirrors**
Described below in section 4.4.2.

**use.desc**
Contains descriptions of valid global USE flags for this repository. The format is described in section 4.4.3.

**use.local.desc**
Contains descriptions of valid local USE flags for this repository, along with the packages to which they apply. The format is as described in section 4.4.3.

**desc/**
This directory contains files analogous to `use.desc` for the various `USE_EXPAND` variables. Each file in it is named `<varname>.desc`, where `<varname>` is the variable name, in lower case, whose possible values the file describes. The format of each file is as for `use.desc`, described in section 4.4.3. The `USE_EXPAND` name is *not* included as a prefix here.

**updates/**
This directory is described in section 4.4.4.

### 4.4.1 The profiles.desc file

`profiles.desc` is a line-based file, with the standard commenting rules from section 4.4, containing a list of profiles that are valid for use, along with their associated architecture and status. Each line has the format:

```
<keyword> <profile path> <stability>
```

Where:

- `<keyword>` is the default keyword for the profile and the `ARCH` for which the profile is valid.
- `<profile path>` is the (relative) path from the `profiles` directory to the profile in question.
- `<stability>` indicates the stability of the profile. This may be useful for QA tools, which may wish to display warnings with a reduced severity for some profiles. The values `stable` and `dev` are widely used, but repositories may use other values.

Fields are whitespace-delimited.

### 4.4.2 The thirdpartymirrors file

`thirdpartymirrors` is another simple line-based file, describing the valid mirrors for use with `mirror://` URIs in this repository, and the associated download locations. The format of each line is:

```
<mirror name> <mirror 1> <mirror 2> ... <mirror n>
```

Fields are whitespace-delimited. When parsing a URI of the form `mirror://name/path/filename`, where the `path/` part is optional, the `thirdpartymirrors` file is searched for a line whose first field is `name`. Then the download URIs in the subsequent fields have `path/filename` appended to them to generate the URIs from which a download is attempted.

Each mirror name may appear at most once in a file. Behaviour when a mirror name appears multiple times is undefined. Behaviour when a mirror is defined in terms of another mirror is undefined. A package manager may choose to fetch from all of or a subset of the listed mirrors, and may use an order other than the one described.

The mirror with the name equal to the repository's name (and if the repository has a master, the master's name) may be consulted for all downloads.

### 4.4.3 use.desc and related files

`use.desc` contains descriptions of every valid global USE flag for this repository. It is a line-based file with the standard rules for comments and blank lines. The format of each line is:

```
<flagname> - <description>
```

`use.local.desc` contains descriptions of every valid local USE flag—those that apply only to a small number of packages, or that have different meanings for different packages. Its format is:

```
<category/package>:<flagname> - <description>
```

Flags must be listed once for each package to which they apply, or if a flag is listed in both `use.desc` and `use.local.desc`, it must be listed once for each package for which its meaning differs from that described in `use.desc`.

### 4.4.4 The updates directory

The `updates` directory is used to inform the package manager that a package has moved categories, names, or that a version has changed SLOT. For EAPIs 0-7, it contains one file per quarter year, named `[1-4]Q-[YYYY]` for the first to fourth quarter of a given year, for example `1Q-2004` or `3Q-2006`. For EAPIs 8+, all regular files in this directory will be processed, unless their name begins with a dot.

The format of each file is again line-based, with each line having one of the following formats:

```
move <qpn1> <qpn2>
slotmove <spec> <slot1> <slot2>
```

The first form, where `qpn1` and `qpn2` are *qualified package names*, instructs the package manager that the package `qpn1` has changed name, category, or both, and is now called `qpn2`.

The second form instructs the package manager that any currently installed package version matching package dependency specification `spec` whose `SLOT` is set to `slot1` should have it updated to `slot2`.

It is unspecified in what order the files in the `updates` directory are processed. Lines within each file are processed in ascending order.

At any given time, a name that appears as the origin of a move may not be used as a qualified package name in the repository. A slot that appears as the origin of a slot move may not be used by packages matching the spec of that slot move.

**Table 4.2: Naming rules for files in updates directory for EAPIs**

| EAPI | Files per quarter year? |
|------|------------------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | Yes |
| 8, 9 | No |

## 4.5 The licenses directory

The `licenses` directory shall contain copies of the licenses used by packages in the repository. Each file will be named according to the name used in the `LICENSE` variable as described in section 7.3, and will contain the complete text of the license in human-readable form. Plain text format is strongly preferred but not required.

## 4.6 The eclass directory

The `eclass` directory shall contain copies of the eclasses provided by this repository. The format of these files is described in chapter 10. It may also contain, in their own directory, support files needed by these eclasses.

## 4.7 The metadata directory

The `metadata` directory contains various repository-level metadata that are not contained in `profiles/`. All contents are optional. In this standard only the `cache` subdirectory is described; other contents are optional but may include security advisories, DTD files for the various XML files used in the repository, and repository timestamps.

### 4.7.1 The metadata cache

The `metadata/cache` directory may contain a cached form of all important ebuild metadata variables. The contents of this directory are described in chapter 14.

---

*Converted from PMS HTML for GRPM development reference.*
