# Chapter 12: Available Commands

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html)
>
> This document is extracted from the official Gentoo PMS specification for GRPM development reference.

---

This chapter documents the commands available to an ebuild. Unless otherwise specified, they may be aliases, shell functions, or executables in the ebuild's `PATH`.

When an ebuild is being sourced for metadata querying rather than for a build (that is to say, when none of the `src_` or `pkg_` functions are to be called), no external command may be executed. The package manager may take steps to enforce this.

---

## 12.1 System Commands

Any ebuild not listed in the system set for the active profile(s) may assume the presence of every command that is always provided by the system set for that profile. However, it must target the lowest common denominator of all systems on which it might be installed -- in most cases this means that the only packages that can be assumed to be present are those listed in the `base` profile or equivalent, which is inherited by all available profiles. If an ebuild requires any applications not provided by the system profile, or that are provided conditionally based on USE flags, appropriate dependencies must be used to ensure their presence.

### 12.1.1 Guaranteed System Commands

The following commands must always be available in the ebuild environment:

- All builtin commands in GNU bash, version as listed in table 6.1.
- `sed` must be available, and must support all forms of invocations valid for GNU sed version 4 or later.
- **gnu-patch** `patch` must be available, and must support all inputs valid for GNU patch, version as listed in table 12.1.
- **gnu-find** `find` and `xargs` must be available, and must support all forms of invocations valid for GNU findutils version 4.4 or later. Only for EAPIs listed in table 12.1 as requiring GNU find.

**Table 12.1: System commands for EAPIs**

| EAPI | GNU `find`? | GNU `patch` version |
|------|-------------|---------------------|
| 0, 1, 2, 3, 4 | Undefined | Any |
| 5, 6 | Yes | Any |
| 7, 8, 9 | Yes | 2.7 |

---

## 12.2 Commands Provided by Package Dependencies

In some cases a package's build process will require the availability of executables not provided by the core system, a common example being autotools. The availability of commands provided by the particular types of dependencies is explained in section 8.1.

---

## 12.3 Ebuild-specific Commands

The following commands will always be available in the ebuild environment, provided by the package manager. Except where otherwise noted, they may be internal (shell functions or aliases) or external commands available in `PATH`; where this is not specified, ebuilds may not rely upon either behaviour. Unless otherwise specified, it is an error if an ebuild calls any of these commands in global scope.

Unless otherwise noted, any output of these commands ends with a newline.

### 12.3.1 Failure Behaviour and Related Commands

**die-on-failure** Where a command is listed as having EAPI dependent failure behaviour, a failure shall either result in a non-zero exit status or abort the build process, as determined by table 12.2.

The following commands affect this behaviour:

**nonfatal**

**nonfatal** Takes one or more arguments and executes them as a command, preserving the exit status. If this results in a command being called that would normally abort the build process due to a failure, instead a non-zero exit status shall be returned. Only in EAPIs listed in table 12.2 as supporting `nonfatal`.

In EAPIs listed in table 12.2 as having `nonfatal` defined both as a shell function and as an external command, the package manager must provide both implementations to account for calling directly in ebuild scope or through `xargs`.

Explicit `die` or `assert` commands only respect `nonfatal` when called with the `-n` option and in EAPIs supporting this option, see table 12.6.

**Table 12.2: EAPI command failure behaviour**

| EAPI | Command failure behaviour | Supports `nonfatal`? | `nonfatal` is both a function and an external command? |
|------|---------------------------|----------------------|--------------------------------------------------------|
| 0, 1, 2, 3 | Non-zero exit | No | n/a |
| 4, 5, 6 | Aborts | Yes | No |
| 7, 8, 9 | Aborts | Yes | Yes |

### 12.3.2 Banned Commands

**banned-commands** Some commands are banned in some EAPIs. If a banned command is called, the package manager must abort the build process indicating an error.

**Table 12.3: Banned commands**

| EAPI | `dohard` | `dosed` | `einstall` | `dohtml` | `dolib` | `libopts` |
|------|----------|---------|------------|----------|---------|-----------|
| 0, 1, 2, 3 | No | No | No | No | No | No |
| 4, 5 | Yes | Yes | No | No | No | No |
| 6 | Yes | Yes | Yes | No | No | No |
| 7, 8, 9 | Yes | Yes | Yes | Yes | Yes | Yes |

| EAPI | `useq` | `hasv` | `hasq` | `assert` | `domo` |
|------|--------|--------|--------|----------|--------|
| 0, 1, 2, 3, 4, 5, 6, 7 | No | No | No | No | No |
| 8 | Yes | Yes | Yes | No | No |
| 9 | Yes | Yes | Yes | Yes | Yes |

### 12.3.3 Sandbox Commands

These commands affect the behaviour of the sandbox. Each command takes a single path as argument. Ebuilds must not run any of these commands once the current phase function has returned.

**addread**
: Add a path to the permitted read list.

**addwrite**
: Add a path to the permitted write list.

**addpredict**
: Add a path to the predict list. Any write to a location in this list will be denied, but will not trigger access violation messages or abort the build process.

**adddeny**
: Add a path to the deny list.

### 12.3.4 Package Manager Query Commands

These commands are used to extract information about the system. Ebuilds must not run any of these commands in parallel with any other package manager command. Ebuilds must not run any of these commands once the current phase function has returned.

**pm-query-options** In EAPIs listed in table 12.4 as supporting option `--host-root`, this flag as the first argument will cause the query to apply to the host root. Otherwise, it applies to `ROOT`.

In EAPIs listed in table 12.4 as supporting options `-b`, `-d` and `-r`, these mutually exclusive flags as the first argument will cause the query to apply to locations targeted by `BDEPEND`, `DEPEND` and `RDEPEND`, respectively. When none of these options are given, `-r` is assumed.

**has_version**
: Takes exactly one package dependency specification as an argument. Returns true if a package matching the specification is installed, and false otherwise.

**best_version**
: Takes exactly one package dependency specification as an argument. If a matching package is installed, prints `category/package-version` of the highest matching version; otherwise, prints an empty string. The exit code is unspecified.

**Table 12.4: Package manager query command options supported by EAPIs**

| EAPI | `--host-root`? | `-b`? | `-d`? | `-r`? |
|------|----------------|-------|-------|-------|
| 0, 1, 2, 3, 4 | No | No | No | No |
| 5, 6 | Yes | No | No | No |
| 7, 8, 9 | No | Yes | Yes | Yes |

### 12.3.5 Output Commands

These commands display messages to the user. Unless otherwise stated, the entire argument list is used as a message, with backslash-escaped characters interpreted as for the `echo -e` command of bash, notably `\t` for a horizontal tab, `\n` for a new line, and `\\` for a literal backslash. These commands must be implemented internally as shell functions and may be called in global scope. Ebuilds must not run any of these commands once the current phase function has returned.

**output-no-stdout** Unless otherwise noted, output may be sent to stderr or some other appropriate facility. In EAPIs listed in table 12.5 as not allowing stdout output, using stdout as an output facility is forbidden.

**einfo**
: Displays an informational message.

**einfon**
: Displays an informational message without a trailing newline.

**elog**
: Displays an informational message of slightly higher importance. The package manager may choose to log `elog` messages by default where `einfo` messages are not, for example.

**ewarn**
: Displays a warning message. Must not go to stdout.

**eqawarn**
: **eqawarn** Display a QA warning message intended for ebuild developers. The package manager may provide appropriate mechanisms to skip those messages for normal users. Must not go to stdout. Only available in EAPIs listed in table 12.5 as supporting `eqawarn`.

**eerror**
: Displays an error message. Must not go to stdout.

**ebegin**
: Displays an informational message. Should be used when beginning a possibly lengthy process, and followed by a call to `eend`.

**eend**
: Indicates that the process begun with an `ebegin` message has completed. Takes one fixed argument, which is a numeric return code, and an optional message in all subsequent arguments. If the first argument is 0, prints a success indicator; otherwise, prints the message followed by a failure indicator. Returns its first argument as exit status.

**Table 12.5: Output commands for EAPIs**

| EAPI | Commands can output to stdout? | Supports `eqawarn`? |
|------|--------------------------------|---------------------|
| 0, 1, 2, 3, 4, 5, 6 | Yes | No |
| 7, 8, 9 | No | Yes |

### 12.3.6 Error Commands

These commands are used when an error is detected that will prevent the build process from completing. Ebuilds must not run any of these commands once the current phase function has returned.

**die**
: **nonfatal-die** If called under the `nonfatal` command (as per section 12.3.1) and with `-n` as its first parameter, displays a failure message provided in its following argument and then returns a non-zero exit status. Only in EAPIs listed in table 12.6 as supporting option `-n`. Otherwise, displays a failure message provided in its first and only argument, and then aborts the build process.

: **subshell-die** In EAPIs listed in table 12.6 as not providing subshell support, `die` is *not* guaranteed to work correctly if called from a subshell environment.

**assert**
: Checks the shell's pipe status array, and if any element is non-zero (indicating failure), calls `die`, passing any parameters to it. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**pipestatus**
: **pipestatus** Checks the shell's pipe status array, i.e. the exit status of the command(s) in the most recently executed foreground pipeline. Returns shell true (0) if all elements are zero, or the last non-zero element otherwise. If called with `-v` as the first argument, also outputs the pipe status array as a space-separated list. Only available in EAPIs listed in table 12.7 as supporting `pipestatus`.

**Table 12.6: Properties of `die` command in EAPIs**

| EAPI | Supports `die -n`? | `die` works in subshell? |
|------|--------------------|--------------------------|
| 0, 1, 2, 3, 4, 5 | No | No |
| 6 | Yes | No |
| 7, 8, 9 | Yes | Yes |

**Table 12.7: EAPIs supporting `pipestatus`**

| EAPI | Supports `pipestatus`? |
|------|------------------------|
| 0, 1, 2, 3, 4, 5, 6, 7, 8 | No |
| 9 | Yes |

### 12.3.7 Patch Commands

These commands are used during the `src_prepare` phase to apply patches to the package's sources. Ebuilds must not run any of these commands once the current phase function has returned.

**eapply**
: **eapply** Takes zero or more GNU patch options, followed by one or more file or directory paths. Processes options and applies all patches found in specified locations according to algorithm 12.1. If applying the patches fails, it aborts the build using `die`, unless run using `nonfatal`, in which case it returns non-zero exit status. Only available in EAPIs listed in table 12.8 as supporting `eapply`.

**Algorithm 12.1: `eapply` logic**

```
1: if any parameter is equal to "--" then
2:     collect all parameters before the first "--" in the options array
3:     collect all parameters after the first "--" in the files array
4: else if any parameter that begins with a hyphen follows one that does not then
5:     abort the build process with an error
6: else
7:     collect all parameters beginning with a hyphen in the options array
8:     collect all remaining parameters in the files array
9: end if
10: if the files array is empty then
11:    abort the build process with an error
12: end if
13: for all x in the files array do
14:    if $x is a directory then
15:       if not any files match $x/*.diff or $x/*.patch then
16:          abort the build process with an error
17:       end if
18:       for all files f matching $x/*.diff or $x/*.patch, sorted in POSIX locale do
19:          call patch -p1 -f -g0 --no-backup-if-mismatch "${options[@]}" < "$f"
20:          if child process returned with non-zero exit status then
21:             return immediately with that status
22:          end if
23:       end for
24:    else
25:       call patch -p1 -f -g0 --no-backup-if-mismatch "${options[@]}" < "$x"
26:       if child process returned with non-zero exit status then
27:          return immediately with that status
28:       end if
29:    end if
30: end for
31: return shell true (0)
```

**eapply_user**
: **eapply-user** Takes no arguments. Package managers supporting it apply user-provided patches to the source tree in the current working directory. Exact behaviour is implementation defined and beyond the scope of this specification. Package managers not supporting it must implement the command as a no-op. Returns shell true (0) if patches applied successfully, or if no patches were provided. Otherwise, aborts the build process, unless run using `nonfatal`, in which case it returns non-zero exit status. Only available in EAPIs listed in table 12.8 as supporting `eapply_user`. In EAPIs where it is supported, `eapply_user` must be called once in the `src_prepare` phase. For any subsequent calls, the command will do nothing and return 0.

**Table 12.8: Patch commands for EAPIs**

| EAPI | `eapply`? | `eapply_user`? |
|------|-----------|----------------|
| 0, 1, 2, 3, 4, 5 | No | No |
| 6, 7, 8, 9 | Yes | Yes |

### 12.3.8 Build Commands

These commands are used during the `src_configure`, `src_compile`, `src_test`, and `src_install` phases to run the package's build commands. Ebuilds must not run any of these commands once the current phase function has returned.

**econf**
: Calls the program's `./configure` script. This is designed to work with GNU Autoconf-generated scripts. Any additional parameters passed to `econf` are passed directly to `./configure`, after the default options below. `econf` will look in the current working directory for a configure script unless the `ECONF_SOURCE` environment variable is set, in which case it is taken to be the directory containing it.

: **econf-options** `econf` must pass the following options to the configure script:

  - `--prefix` must default to `${EPREFIX}/usr` unless overridden by `econf`'s caller.
  - `--mandir` must be `${EPREFIX}/usr/share/man`
  - `--infodir` must be `${EPREFIX}/usr/share/info`
  - `--datadir` must be `${EPREFIX}/usr/share`
  - `--datarootdir` must be `${EPREFIX}/usr/share`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--datarootdir` occurs in the output of `configure --help`.
  - `--sysconfdir` must be `${EPREFIX}/etc`
  - `--localstatedir` must be `${EPREFIX}/var/lib`
  - `--docdir` must be `${EPREFIX}/usr/share/doc/${PF}`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--docdir` occurs in the output of `configure --help`.
  - `--htmldir` must be `${EPREFIX}/usr/share/doc/${PF}/html`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--htmldir` occurs in the output of `configure --help`.
  - `--with-sysroot` must be `${ESYSROOT:-/}`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--with-sysroot` occurs in the output of `configure --help`.
  - `--build` must be the value of the `CBUILD` environment variable. This option will only be passed if `CBUILD` is non-empty.
  - `--host` must be the value of the `CHOST` environment variable.
  - `--target` must be the value of the `CTARGET` environment variable. This option will only be passed if `CTARGET` is non-empty.
  - `--libdir` must be set according to algorithm 12.2.
  - `--disable-dependency-tracking`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--disable-dependency-tracking` occurs in the output of `configure --help`.
  - `--disable-silent-rules`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if the string `--disable-silent-rules` occurs in the output of `configure --help`.
  - `--disable-static`, if the EAPI is listed in table 12.9 as using it. This option will only be passed if both strings `--enable-static` and `--enable-shared` occur in the output of `configure --help`.

: For the option names beginning with `with-`, `disable-` or `enable-`, a string in `configure --help` output matches only if it is not immediately followed by any of the characters `[A-Za-z0-9+_.-]`.

: Note that the `${EPREFIX}` component represents the same offset-prefix as described in table 11.2. It facilitates offset-prefix installations which is supported by EAPIs listed in table 11.5. When no offset-prefix installation is in effect, `EPREFIX` becomes the empty string.

: `econf` must be implemented internally -- that is, as a bash function and not an external script. Should any portion of it fail, it must abort the build using `die`, unless run using `nonfatal`, in which case it must return non-zero exit status.

**Algorithm 12.2: `econf --libdir` logic**

```
1: let prefix=${EPREFIX}/usr
2: if the caller specified --exec-prefix=$ep then
3:    let prefix=$ep
4: else if the caller specified --prefix=$p then
5:    let prefix=$p
6: end if
7: let libdir=
8: if the ABI environment variable is set then
9:    let libvar=LIBDIR_$ABI
10:   if the environment variable named by libvar is set then
11:      let libdir=the value of the variable named by libvar
12:   end if
13: end if
14: if libdir is non-empty then
15:    pass --libdir=$prefix/$libdir to configure
16: end if
```

**Table 12.9: Extra `econf` arguments for EAPIs**

| EAPI | --datarootdir | --docdir | --htmldir | --with-sysroot |
|------|---------------|----------|-----------|----------------|
| 0, 1, 2, 3, 4, 5 | No | No | No | No |
| 6 | No | Yes | Yes | No |
| 7 | No | Yes | Yes | Yes |
| 8, 9 | Yes | Yes | Yes | Yes |

| EAPI | --disable-dependency-tracking | --disable-silent-rules | --disable-static |
|------|------------------------------|------------------------|------------------|
| 0, 1, 2, 3 | No | No | No |
| 4 | Yes | No | No |
| 5, 6, 7 | Yes | Yes | No |
| 8, 9 | Yes | Yes | Yes |

**emake**
: Calls the `${MAKE}` program, or GNU make if the `MAKE` variable is unset. Any arguments given are passed directly to the make command, as are the user's chosen `MAKEOPTS`. Arguments given to `emake` override user configuration. See also section 12.1.1. `emake` must be an external program and cannot be a function or alias -- it must be callable from e.g. `xargs`. Failure behaviour is EAPI dependent as per section 12.3.1.

**einstall**
: A shortcut for the command given in listing 12.1. Any arguments given to `einstall` are passed verbatim to `emake`, as shown. Failure behaviour is EAPI dependent as per section 12.3.1. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

: The variable `ED` is defined as in table 11.2 and depends on the use of an offset-prefix. When such offset-prefix is absent, `ED` is equivalent to `D`. Variable `libdir` is an auxiliary local variable whose value is determined by algorithm 12.4.

**Listing 12.1: `einstall` command**

```bash
emake \
    prefix="${ED}"/usr \
    datadir="${ED}"/usr/share \
    mandir="${ED}"/usr/share/man \
    infodir="${ED}"/usr/share/info \
    libdir="${ED}"/usr/${libdir} \
    localstatedir="${ED}"/var/lib \
    sysconfdir="${ED}"/etc \
    -j1 \
    "$@" \
    install
```

### 12.3.9 Installation Commands

These commands are used to install files into the staging area, in cases where the package's `make install` target cannot be used or does not install all needed files. Except where otherwise stated, all filenames created or modified are relative to the staging directory including the offset-prefix `ED` in offset-prefix aware EAPIs, or just the staging directory `D` in offset-prefix agnostic EAPIs. Existing destination files are overwritten. These commands must all be external programs and not bash functions or aliases -- that is, they must be callable from `xargs`. Calling any of these commands without a filename parameter is an error. Ebuilds must not run any of these commands once the current phase function has returned.

**dobin**
: Installs the given files into `DESTTREE/bin`, where `DESTTREE` defaults to `/usr`. Gives the files mode `0755` and transfers file ownership to the superuser or its equivalent on the system or installation at hand. In a non-offset-prefix installation this ownership is `0:0`, while in an offset-prefix aware installation this may be e.g. `joe:users`. Failure behaviour is EAPI dependent as per section 12.3.1.

**doconfd**
: Installs the given config files into `/etc/conf.d/`, by default with file mode `0644`. For EAPIs listed in table 12.17 as respecting `insopts` in `doconfd`, the `install` options set by the most recent `insopts` call override the default. Failure behaviour is EAPI dependent as per section 12.3.1.

**dodir**
: Creates the given directories, by default with file mode `0755`, or with the `install` options set by the most recent `diropts` call. Failure behaviour is EAPI dependent as per section 12.3.1.

**dodoc**
: **dodoc** Installs the given files into a subdirectory under `/usr/share/doc/${PF}/` with file mode `0644`. The subdirectory is set by the most recent call to `docinto`. If `docinto` has not yet been called, instead installs to the directory `/usr/share/doc/${PF}/`. For EAPIs listed in table 12.10 as supporting `-r`, if the first argument is `-r`, any subsequent arguments that are directories are installed recursively to the appropriate location; in any other case, it is an error for a directory to be specified. Any directories that don't already exist are created using `install -d` with no additional options. Failure behaviour is EAPI dependent as per section 12.3.1.

**doenvd**
: Installs the given environment files into `/etc/env.d/`, by default with file mode `0644`. For EAPIs listed in table 12.17 as respecting `insopts` in `doenvd`, the `install` options set by the most recent `insopts` call override the default. Failure behaviour is EAPI dependent as per section 12.3.1.

**doexe**
: Installs the given files into the directory specified by the most recent `exeinto` call. If `exeinto` has not yet been called, behaviour is undefined. Files are installed by default with file mode `0755`, or with the `install` options set by the most recent `exeopts` call. Failure behaviour is EAPI dependent as per section 12.3.1.

**dohard**
: Takes two parameters. Creates a hardlink from the second to the first. Both paths are relative to the staging directory including the offset-prefix `ED` in offset-prefix aware EAPIs, or just the staging directory `D` in offset-prefix agnostic EAPIs. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**doheader**
: **doheader** Installs the given header files into `/usr/include/`, by default with file mode `0644`. For EAPIs listed in table 12.17 as respecting `insopts` in `doheader`, the `install` options set by the most recent `insopts` call override the default. If the first argument is `-r`, then operates recursively, descending into any directories given. Only available in EAPIs listed in table 12.11 as supporting `doheader`. Failure behaviour is EAPI dependent as per section 12.3.1.

**dohtml**
: Installs the given HTML files into a subdirectory under `/usr/share/doc/${PF}/`. The subdirectory is `html` by default, but this can be overridden with the `docinto` function. Files to be installed automatically are determined by extension and the default extensions are `css`, `gif`, `htm`, `html`, `jpeg`, `jpg`, `js` and `png`. These default extensions can be extended or reduced (see below). The options that can be passed to `dohtml` are as follows:

  - `-r` enables recursion into directories.
  - `-V` enables verbosity.
  - `-A` adds file type extensions to the default list.
  - `-a` sets file type extensions to only those specified.
  - `-f` list of files that are able to be installed.
  - `-x` list of directories that files will not be installed from (only used in conjunction with `-r`).
  - `-p` sets a document prefix for installed files, not to be confused with the global offset-prefix.

: In EAPIs listed in table 12.3, this command is banned as per section 12.3.2. Failure behaviour is EAPI dependent as per section 12.3.1.

: It is undefined whether a failure shall occur if `-r` is not specified and a directory is encountered. Ebuilds must not rely upon any particular behaviour.

**doinfo**
: Installs the given GNU Info files into the `/usr/share/info` area with file mode `0644`. Failure behaviour is EAPI dependent as per section 12.3.1.

**doinitd**
: Installs the given initscript files into `/etc/init.d`, by default with file mode `0755`. For EAPIs listed in table 12.17 as respecting `insopts` in `doinitd`, the `install` options set by the most recent `exeopts` call override the default. Failure behaviour is EAPI dependent as per section 12.3.1.

**doins**
: **doins** Takes one or more files as arguments and installs them into `INSDESTTREE`, by default with file mode `0644`, or with the `install` options set by the most recent `insopts` call. If the first argument is `-r`, then operates recursively, descending into any directories given. Any directories are created as if `dodir` was called. For EAPIs listed in table 12.12, `doins` must install symlinks as symlinks; for other EAPIs, behaviour is undefined if any symlink is encountered. Failure behaviour is EAPI dependent as per section 12.3.1.

**dolib.a**
: For each argument, installs it into the appropriate library subdirectory under `DESTTREE`, as determined by algorithm 12.4. Files are installed with file mode `0644`. Any symlinks are installed into the same directory as relative links to their original target. Failure behaviour is EAPI dependent as per section 12.3.1.

**dolib.so**
: As for `dolib.a` except each file is installed with mode `0755`.

**dolib**
: As for `dolib.a` except that the default install mode can be overridden with the `install` options set by the most recent `libopts` call. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**doman**
: Installs the given man pages into the appropriate subdirectory of `/usr/share/man` depending upon its apparent section suffix (e.g. `foo.1` goes to `/usr/share/man/man1/foo.1`) with file mode `0644`.

: **doman-langs** In EAPIs listed in table 12.13 as supporting language detection by filename, a man page with name of the form `foo.lang.1` shall go to `/usr/share/man/lang/man1/foo.1`, where *lang* refers to a pair of lower-case ASCII letters optionally followed by an underscore and a pair of upper-case ASCII letters. Failure behaviour is EAPI dependent as per section 12.3.1.

: With option `-i18n=lang`, a man page shall be installed into an appropriate subdirectory of `/usr/share/man/lang` (e.g. `/usr/share/man/lang/man1/foo.pl.1` would be the destination for `foo.pl.1`). The *lang* subdirectory level is skipped if *lang* is the empty string. In EAPIs specified by table 12.13, the `-i18n` option takes precedence over the language code in the filename.

**domo**
: **domo-path** Installs the given `.mo` files with file mode `0644` into the appropriate subdirectory of the locale tree, generated by taking the basename of the file, removing the `.*` suffix, and appending `/LC_MESSAGES`. The name of the installed files is the package name with `.mo` appended. Failure behaviour is EAPI dependent as per section 12.3.1. The locale tree location is EAPI dependent as per table 12.15. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**dosbin**
: As `dobin`, but installs to `DESTTREE/sbin`.

**dosym**
: Creates a symbolic link named as for its second parameter, pointing to the first. If the directory containing the new link does not exist, creates it.

: **dosym-relative** In EAPIs listed in table 12.16 as supporting creation of relative paths, when called with option `-r`, the first parameter (the link target) is converted from an absolute path to a path relative to the second parameter (the link name). The algorithm must return a result identical to the one returned by the function in listing 12.2, with `realpath` and `dirname` from GNU coreutils version 8.32. Specifying option `-r` together with a relative path as first (target) parameter is an error.

: Failure behaviour is EAPI dependent as per section 12.3.1.

**Listing 12.2: Create a relative path for `dosym -r`**

```bash
dosym_relative_path() {
    local link=$(realpath -m -s "/${2#/}")
    local linkdir=$(dirname "${link}")
    realpath -m -s --relative-to="${linkdir}" "$1"
}
```

**fowners**
: Acts as for `chown`, but takes paths relative to the image directory. Failure behaviour is EAPI dependent as per section 12.3.1.

**fperms**
: Acts as for `chmod`, but takes paths relative to the image directory. Failure behaviour is EAPI dependent as per section 12.3.1.

**keepdir**
: For each argument, creates a directory as for `dodir`, and an empty file whose name starts with `.keep` in that directory to ensure that the directory does not get removed by the package manager should it be empty at any point. Failure behaviour is EAPI dependent as per section 12.3.1.

**newbin**
: **newfoo-stdin** As for `dobin`, but takes two parameters. The first is the file to install; the second is the new filename under which it will be installed. In EAPIs specified by table 12.14, standard input is read when the first parameter is `-` (a hyphen). In this case, it is an error if standard input is a terminal.

**newconfd** - As for `doconfd`, but takes two parameters as for `newbin`.

**newdoc** - As above, for `dodoc`.

**newenvd** - As above, for `doenvd`.

**newexe** - As above, for `doexe`.

**newheader** - As above, for `doheader`.

**newinitd** - As above, for `doinitd`.

**newins** - As above, for `doins`.

**newlib.a** - As above, for `dolib.a`.

**newlib.so** - As above, for `dolib.so`.

**newman** - As above, for `doman`.

**newsbin** - As above, for `dosbin`.

**Table 12.10: EAPIs supporting `dodoc -r`**

| EAPI | Supports `dodoc -r`? |
|------|----------------------|
| 0, 1, 2, 3 | No |
| 4, 5, 6, 7, 8, 9 | Yes |

**Table 12.11: EAPIs supporting `doheader` and `newheader`**

| EAPI | Supports `doheader` and `newheader`? |
|------|--------------------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

**Table 12.12: EAPIs supporting symlinks for `doins`**

| EAPI | `doins` supports symlinks? |
|------|---------------------------|
| 0, 1, 2, 3 | No |
| 4, 5, 6, 7, 8, 9 | Yes |

**Table 12.13: `doman` language support options for EAPIs**

| EAPI | Language detection by filename? | Option `-i18n` takes precedence? |
|------|---------------------------------|----------------------------------|
| 0, 1 | No | Not applicable |
| 2, 3 | Yes | No |
| 4, 5, 6, 7, 8, 9 | Yes | Yes |

**Table 12.14: EAPIs supporting stdin for `new*` commands**

| EAPI | `new*` can read from stdin? |
|------|-----------------------------|
| 0, 1, 2, 3, 4 | No |
| 5, 6, 7, 8, 9 | Yes |

**Table 12.15: `domo` destination path in EAPIs**

| EAPI | Destination path |
|------|------------------|
| 0, 1, 2, 3, 4, 5, 6 | `${DESTTREE}/share/locale` |
| 7, 8, 9 | `/usr/share/locale` |

**Table 12.16: EAPIs supporting `dosym -r`**

| EAPI | `dosym` supports creation of relative paths? |
|------|---------------------------------------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | No |
| 8, 9 | Yes |

### 12.3.10 Commands Affecting Install Destinations

The following commands are used to set the various destination trees and options used by the above installation commands. They must be shell functions or aliases, due to the need to set variables read by the above commands. Ebuilds must not run any of these commands once the current phase function has returned.

**into**
: Takes exactly one argument, and sets the value of `DESTTREE` for future invocations of the above utilities to it. Creates the directory under `${ED}` in offset-prefix aware EAPIs or under `${D}` in offset-prefix agnostic EAPIs, using `install -d` with no additional options, if it does not already exist. Failure behaviour is EAPI dependent as per section 12.3.1.

**insinto**
: As `into`, for `INSDESTTREE`.

**exeinto**
: As `into`, for install path of `doexe` and `newexe`.

**docinto**
: As `into`, for install subdirectory of `dodoc` et al.

**insopts**
: **insopts** Takes one or more arguments, and sets the options passed by `doins` et al. to the `install` command to them. Behaviour upon encountering empty arguments is undefined. Depending on EAPI, affects only those commands that are specified by table 12.17 as respecting `insopts`.

**diropts**
: As `insopts`, for `dodir` et al.

**exeopts**
: **exeopts** As `insopts`, for `doexe` et al. Depending on EAPI, affects only those commands that are specified by table 12.18 as respecting `exeopts`.

**libopts**
: As `insopts`, for `dolib` et al. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**Table 12.17: Commands respecting `insopts` for EAPIs**

| EAPI | `doins`? | `doconfd`? | `doenvd`? | `doheader`? |
|------|----------|------------|-----------|-------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | Yes | Yes | Yes | Yes |
| 8, 9 | Yes | No | No | No |

**Table 12.18: Commands respecting `exeopts` for EAPIs**

| EAPI | `doexe`? | `doinitd`? |
|------|----------|------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | Yes | Yes |
| 8, 9 | Yes | No |

### 12.3.11 Commands Controlling Manipulation of Files in the Staging Area

These commands are used to control optional manipulations that the package manager may perform on files in the staging directory `ED`, like compressing files or stripping symbols from object files.

For each of the operations mentioned below, the package manager shall maintain an inclusion list and an exclusion list, in order to control which directories and files the operation may or may not be performed upon. The initial contents of the two lists is specified below for each of the commands, respectively.

Any of these operations shall be carried out after `src_install` has completed, and before the execution of any subsequent phase function. For each item in the inclusion list, pretend it has the value of the `ED` variable prepended, then:

- If it is a directory, act as if every file or directory immediately under this directory were in the inclusion list.
- If the item is a file, the operation may be performed on it, unless it has been excluded as described below.
- If the item does not exist, it is ignored.

Whether an item is to be excluded is determined as follows: For each item in the exclusion list, pretend it has the value of the `ED` variable prepended, then:

- If it is a directory, act as if every file or directory immediately under this directory were in the exclusion list.
- If the item is a file, the operation shall not be performed on it.
- If the item does not exist, it is ignored.

The package manager shall take appropriate steps to ensure that any operations that it performs on files in the staging area behave sensibly even if an item is listed in the inclusion list multiple times or if an item is a symlink.

**docompress** In EAPIs listed in table 12.19 as supporting controllable compression, the package manager may optionally compress a subset of the files under the `ED` directory. The package manager shall ensure that its compression mechanisms do not compress a file twice if it is already compressed using the same compressed file format. For compression, the initial values of the two lists are as follows:

- The inclusion list contains `/usr/share/doc`, `/usr/share/info` and `/usr/share/man`.
- The exclusion list contains `/usr/share/doc/${PF}/html`.

**dostrip** In EAPIs listed in table 12.19 as supporting controllable stripping of symbols, the package manager may strip a subset of the files under the `ED` directory. For stripping of symbols, the initial values of the two lists are as follows:

- If the `RESTRICT` variable described in section 7.3.6 enables a `strip` token, the inclusion list is empty; otherwise it contains `/` (the root path).
- The exclusion list is empty.

The following commands may be used in `src_install` to alter these lists. It is an error to call any of these functions from any other phase.

**docompress**
: If the first argument is `-x`, add each of its subsequent arguments to the exclusion list for compression. Otherwise, add each argument to the respective inclusion list. Only available in EAPIs listed in table 12.19 as supporting `docompress`.

**dostrip**
: If the first argument is `-x`, add each of its subsequent arguments to the exclusion list for stripping of symbols. Otherwise, add each argument to the respective inclusion list. Only available in EAPIs listed in table 12.19 as supporting `dostrip`.

**Table 12.19: Commands controlling manipulation of files in the staging area in EAPIs**

| EAPI | Supports controllable compression and `docompress`? | Supports controllable stripping and `dostrip`? |
|------|-----------------------------------------------------|-----------------------------------------------|
| 0, 1, 2, 3 | No | No |
| 4, 5, 6 | Yes | No |
| 7, 8, 9 | Yes | Yes |

### 12.3.12 USE List Functions

These functions provide behaviour based upon set or unset use flags. Ebuilds must not run any of these commands once the current phase function has returned.

Unless otherwise noted, if any of these functions is called with a flag value that is not included in `IUSE_EFFECTIVE`, either behaviour is undefined or it is an error as decided by table 12.20.

**use**
: Returns shell true (0) if the first argument (a `USE` flag name) is enabled, false otherwise. If the flag name is prefixed with `!`, returns true if the flag is disabled, and false if it is enabled. It is guaranteed that this command is quiet.

**usev**
: **usev** The same as `use`, but also prints the flag name if the condition is met. In EAPIs listed in table 12.21 as supporting an optional second argument for `usev`, prints the second argument instead, if it is specified and if the condition is met.

**useq**
: Deprecated synonym for `use`. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**use_with**
: **use-with** Has one-, two-, and three-argument forms. The first argument is a USE flag name, the second a `configure` option name (`${opt}`), defaulting to the same as the first argument if not provided, and the third is a string value (`${value}`). For EAPIs listed in table 12.21 as not supporting it, an empty third argument is treated as if it weren't provided. If the USE flag is set, outputs `--with-${opt}=${value}` if the third argument was provided, and `--with-${opt}` otherwise. If the flag is not set, then it outputs `--without-${opt}`. The condition is inverted if the flag name is prefixed with `!`; this is valid only for the two- and three-argument forms.

**use_enable**
: Works the same as `use_with()`, but outputs `--enable-` or `--disable-` instead of `--with-` or `--without-`.

**usex**
: **usex** Accepts at least one and at most five arguments. The first argument is a USE flag name, any subsequent arguments (`${arg2}` to `${arg5}`) are string values. If not provided, `${arg2}` and `${arg3}` default to `yes` and `no`, respectively; `${arg4}` and `${arg5}` default to the empty string. If the USE flag is set, outputs `${arg2}${arg4}`. Otherwise, outputs `${arg3}${arg5}`. The condition is inverted if the flag name is prefixed with `!`. Only available in EAPIs listed in table 12.22 as supporting `usex`.

**in_iuse**
: **in-iuse** Returns shell true (0) if the first argument (a `USE` flag name) is included in `IUSE_EFFECTIVE`, false otherwise. Only available in EAPIs listed in table 12.22 as supporting `in_iuse`.

**Table 12.20: EAPI behaviour for use queries not in `IUSE_EFFECTIVE`**

| EAPI | Behaviour |
|------|-----------|
| 0, 1, 2, 3 | Undefined |
| 4, 5, 6, 7, 8, 9 | Error |

**Table 12.21: `usev`, `use_with` and `use_enable` arguments for EAPIs**

| EAPI | `usev` has optional second argument? | `use_with` and `use_enable` support empty third argument? |
|------|--------------------------------------|----------------------------------------------------------|
| 0, 1, 2, 3 | No | No |
| 4, 5, 6, 7 | No | Yes |
| 8, 9 | Yes | Yes |

**Table 12.22: EAPIs supporting `usex` and `in_iuse`**

| EAPI | `usex`? | `in_iuse`? |
|------|---------|------------|
| 0, 1, 2, 3, 4 | No | No |
| 5 | Yes | No |
| 6, 7, 8, 9 | Yes | Yes |

### 12.3.13 Text List Functions

These functions check a list of arguments for a particular value. They must be implemented internally as shell functions and may be called in global scope.

**has**
: Returns shell true (0) if the first argument (a word) is found in the list of subsequent arguments, false otherwise. Guaranteed quiet.

**hasv**
: The same as `has`, but also prints the first argument if found. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**hasq**
: Deprecated synonym for `has`. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

### 12.3.14 Version Manipulation and Comparison Commands

**ver-commands** These commands provide utilities for working with version strings. They must all be implemented internally as shell functions, i.e. they are callable in global scope. Availability of these commands per EAPI is listed in table 12.23.

For the purpose of version manipulation commands, the specification provides a method for splitting an arbitrary version string (not necessarily conforming to section 3.2) into a series of version components and version separators.

A version component consists either purely of digits (`[0-9]+`) or purely of upper- and lower-case ASCII letters (`[A-Za-z]+`). A version separator is either a string of any other characters (`[^A-Za-z0-9]+`), or it occurs at the transition between a sequence of digits and a sequence of letters, or vice versa. In the latter case, the version separator is an empty string.

The version string is processed left-to-right, with the successive version components being assigned successive indices starting with 1. The separator following a version component is assigned the index of the preceding version component. If the first version component is preceded by a non-empty string of version separator characters, this separator is assigned the index 0.

The version components are presumed present if not empty. The version separators between version components are always presumed present, even if they are empty. The version separators preceding the first version component and following the last are only presumed present if they are not empty.

Whenever the commands support ranges, the range is specified as an unsigned integer, optionally followed by a hyphen (`-`), which in turn is optionally followed by another unsigned integer.

A single integer specifies a single component or separator index. An integer followed by a hyphen specifies all components or separators starting with the one at the specified index. Two integers separated by a hyphen specify a range starting at the index specified by the first and ending at the second, inclusively.

**ver_cut**
: Takes a range as the first argument, and optionally a version string as the second. Prints a substring of the version string starting at the version component specified as start of the range and ending at the version component specified as end of the range. If the version string is not specified, `${PV}` is used.

: If the range spans outside the present version components, the missing components and separators are presumed empty. In particular, the range starting at zero includes the zeroth version separator if present, and the range spanning past the last version component includes the suffix following it if present. A range that does not intersect with any present version components yields an empty string.

**ver_rs**
: Takes one or more pairs of arguments, optionally followed by a version string. Every argument pair specifies a range and a replacement string. Prints a version string after performing the specified separator substitutions. If the version string is not specified, `${PV}` is used.

: For every argument pair specified, each of the version separators present at indices specified by the range is replaced with the replacement string, in order. If the range spans outside the range of present version separators, it is silently truncated.

**ver_test**
: Takes two or three arguments. In the 3-argument form, takes an LHS version string, followed by an operator, followed by an RHS version string. In the 2-argument form, the first version string is omitted and `${PVR}` is used as LHS version string. The operator can be `-eq` (equal to), `-ne` (not equal to), `-gt` (greater than), `-ge` (greater than or equal to), `-lt` (less than) or `-le` (less than or equal to). Returns shell true (0) if the specified relation between the LHS and RHS version strings is fulfilled.

: Both version strings must conform to the version specification in section 3.2. Comparison is done using algorithm 3.1.

**ver_replacing**
: **ver-replacing** Takes an operator and a version string as arguments, which follow the same specification as in `ver_test`. Iterates over the elements of `REPLACING_VERSIONS`, using `ver_test` to compare each element against the version string. Returns shell true (0) if the specified relation holds for at least one element, shell false (1) otherwise. In particular, shell false is returned when `REPLACING_VERSIONS` is empty.

: Only available in EAPIs listed in table 12.23 as supporting `ver_replacing`. The command is only meaningful in phases where `REPLACING_VERSIONS` is defined.

**Table 12.23: EAPIs supporting version manipulation commands**

| EAPI | `ver_cut`? | `ver_rs`? | `ver_test`? | `ver_replacing`? |
|------|------------|-----------|-------------|------------------|
| 0, 1, 2, 3, 4, 5, 6 | No | No | No | No |
| 7, 8 | Yes | Yes | Yes | No |
| 9 | Yes | Yes | Yes | Yes |

### 12.3.15 Misc Commands

The following commands are always available in the ebuild environment, but don't really fit in any of the above categories. Ebuilds must not run any of these commands once the current phase function has returned.

**dosed**
: Takes any number of arguments, which can be files or `sed` expressions. For each argument, if it names, relative to `ED` (offset-prefix aware EAPIs) or `D` (offset-prefix agnostic EAPIs) a file which exists, then `sed` is run with the current expression on that file. Otherwise, the current expression is set to the text of the argument. The initial value of the expression is `s:${ED}::g` in offset-prefix aware EAPIs and `s:${D}::g` in offset-prefix agnostic EAPIs. In EAPIs listed in table 12.3, this command is banned as per section 12.3.2.

**unpack**
: Unpacks one or more source archives, in order, into the current directory. For compressed files, creates the target file in the current directory, with the compression suffix removed from its name. After unpacking, must ensure that all filesystem objects inside the current working directory (but not the current working directory itself) have permissions `a+r,u+w,go-w` and that all directories under the current working directory additionally have permissions `a+x`.

: Arguments to `unpack` are interpreted as follows:

  - A filename without path (i.e. not containing any slash) is looked up in `DISTDIR`.
  - An argument starting with the string `./` is a path relative to the working directory.
  - **unpack-absolute** Otherwise, for EAPIs listed in table 12.24 as supporting absolute and relative paths, the argument is interpreted as a literal path (absolute, or relative to the working directory); for EAPIs listed as *not* supporting such paths, `unpack` shall abort the build process.

: Any unrecognised file format shall be skipped without raising an error. If unpacking a supported file format fails, `unpack` shall abort the build process.

: **unpack-extensions** Must be able to unpack the following file formats, if the relevant binaries are available:

  - tar files (`*.tar`). Ebuilds must ensure that GNU tar is installed.
  - gzip-compressed files (`*.gz, *.z, *.Z`). Ebuilds must ensure that GNU gzip is installed.
  - gzip-compressed tar files (`*.tar.gz, *.tgz, *.tar.z, *.tar.Z`). Ebuilds must ensure that GNU gzip and GNU tar are installed.
  - bzip2-compressed files (`*.bz2, *.bz`). Ebuilds must ensure that bzip2 is installed.
  - bzip2-compressed tar files (`*.tar.bz2, *.tbz2, *.tar.bz, *.tbz`). Ebuilds must ensure that bzip2 and GNU tar are installed.
  - zip files (`*.zip, *.ZIP, *.jar`). Ebuilds must ensure that Info-ZIP Unzip is installed.
  - 7zip files (`*.7z, *.7Z`). Ebuilds must ensure that P7ZIP is installed. Only for EAPIs listed in table 12.25 as supporting `.7z`.
  - rar files (`*.rar, *.RAR`). Ebuilds must ensure that RARLAB's unrar is installed. Only for EAPIs listed in table 12.25 as supporting `.rar`.
  - LHA archives (`*.LHA, *.LHa, *.lha, *.lzh`). Ebuilds must ensure that the lha program is installed. Only for EAPIs listed in table 12.25 as supporting `.lha`.
  - ar archives (`*.a`). Ebuilds must ensure that GNU binutils is installed.
  - deb packages (`*.deb`). Ebuilds must ensure that the deb2targz program is installed on those platforms where the GNU binutils ar program is not available and the installed ar program is incompatible with GNU archives. Otherwise, ebuilds must ensure that GNU binutils is installed.
  - lzma-compressed files (`*.lzma`). Ebuilds must ensure that XZ Utils is installed.
  - lzma-compressed tar files (`*.tar.lzma`). Ebuilds must ensure that XZ Utils and GNU tar are installed.
  - xz-compressed files (`*.xz`). Ebuilds must ensure that XZ Utils is installed. Only for EAPIs listed in table 12.25 as supporting `.xz`.
  - xz-compressed tar files (`*.tar.xz, *.txz`). Ebuilds must ensure that XZ Utils and GNU tar are installed. Only for EAPIs listed in table 12.25 as supporting `.tar.xz` or `.txz`.

: It is up to the ebuild to ensure that the relevant external utilities are available, whether by being in the system set or via dependencies.

: **unpack-ignore-case** `unpack` matches filename extensions in a case-insensitive manner, for EAPIs listed such in table 12.24.

**Table 12.24: `unpack` behaviour for EAPIs**

| EAPI | Supports absolute and relative paths? | Case-insensitive matching? |
|------|---------------------------------------|---------------------------|
| 0, 1, 2, 3, 4, 5 | No | No |
| 6, 7, 8, 9 | Yes | Yes |

**Table 12.25: `unpack` extensions for EAPIs**

| EAPI | `.xz`? | `.tar.xz`? | `.txz`? | `.7z`? | `.rar`? | `.lha`? |
|------|--------|------------|---------|--------|---------|---------|
| 0, 1, 2 | No | No | No | Yes | Yes | Yes |
| 3, 4, 5 | Yes | Yes | No | Yes | Yes | Yes |
| 6, 7 | Yes | Yes | Yes | Yes | Yes | Yes |
| 8, 9 | Yes | Yes | Yes | No | No | No |

**inherit**
: See section 10.1.

**default**
: **default-func** Calls the `default_` function for the current phase (see section 9.1.17). Must not be called if the `default_` function does not exist for the current phase in the current EAPI. Only available in EAPIs listed in table 12.26 as supporting `default`.

**einstalldocs**
: **einstalldocs** Takes no arguments. Installs the files specified by the `DOCS` and `HTML_DOCS` variables or a default set of files, according to algorithm 12.3. If called using `nonfatal` and any of the called commands returns a non-zero exit status, returns immediately with the same exit status. Only available in EAPIs listed in table 12.26 as supporting `einstalldocs`.

**Algorithm 12.3: `einstalldocs` logic**

```
1: save the value of the install directory for dodoc
2: set the install directory for dodoc to /usr/share/doc/${PF}
3: if the DOCS variable is a non-empty array then
4:    call dodoc -r "${DOCS[@]}"
5: else if the DOCS variable is a non-empty scalar then
6:    call dodoc -r ${DOCS}
7: else if the DOCS variable is unset then
8:    for all d matching the filename expansion of README* ChangeLog AUTHORS NEWS TODO CHANGES THANKS BUGS FAQ CREDITS CHANGELOG do
9:       if file d exists and has a size greater than zero then
10:         call dodoc with d as argument
11:      end if
12:   end for
13: end if
14: set the install directory for dodoc to /usr/share/doc/${PF}/html
15: if the HTML_DOCS variable is a non-empty array then
16:    call dodoc -r "${HTML_DOCS[@]}"
17: else if the HTML_DOCS variable is a non-empty scalar then
18:    call dodoc -r ${HTML_DOCS}
19: end if
20: restore the value of the install directory for dodoc
21: return shell true (0)
```

**get_libdir**
: **get-libdir** Prints the libdir name obtained according to algorithm 12.4. Must be implemented internally as a shell function and may be called in global scope. Only available in EAPIs listed in table 12.26 as supporting `get_libdir`.

**Algorithm 12.4: Library directory logic**

```
1: let libdir=lib
2: if the ABI environment variable is set then
3:    let libvar=LIBDIR_$ABI
4:    if the environment variable named by libvar is set then
5:       let libdir=the value of the variable named by libvar
6:    end if
7: end if
8: return the value of libdir
```

**edo**
: **edo** Takes a command line, prints it to stderr and executes the command. Specifically, the entire list of (one or more) arguments is output as an informational message to stderr; individual tokens may be reformatted to avoid ambiguity. The first argument is then executed as a command, with the remaining arguments passed to it. If the command fails, `edo` aborts the build process using `die`, unless it was called under `nonfatal`, in which case it returns a non-zero exit status.

: `edo` must be implemented internally as a shell function. Only available in EAPIs listed in table 12.26 as supporting `edo`.

**Table 12.26: Misc commands for EAPIs**

| EAPI | `default`? | `einstalldocs`? | `get_libdir`? | `edo`? |
|------|------------|-----------------|---------------|--------|
| 0, 1 | No | No | No | No |
| 2, 3, 4, 5 | Yes | No | No | No |
| 6, 7, 8 | Yes | Yes | Yes | No |
| 9 | Yes | Yes | Yes | Yes |

### 12.3.16 Debug Commands

The following commands are available for debugging. Normally all of these commands should be no ops; a package manager may provide a special debug mode where these commands instead do something. These commands must be implemented internally as shell functions and may be called in global scope. Ebuilds must not run any of these commands once the current phase function has returned.

**debug-print**
: If in a special debug mode, the arguments should be outputted or recorded using some kind of debug logging.

**debug-print-function**
: Calls `debug-print` with `$1: entering function` as the first argument and the remaining arguments as additional arguments.

**debug-print-section**
: Calls `debug-print` with `now in section $*`.

### 12.3.17 Reserved Commands and Variables

Except where documented otherwise, all functions and variables that begin with any of the following strings (ignoring case) are reserved for package manager use and may not be used or relied upon by ebuilds:

- `__` (two underscores)
- `abort`
- `dyn`
- `prep`

The same applies to functions and variables that contain any of the following strings (ignoring case):

- `ebuild` (unless immediately preceded by another letter)
- `hook`
- `paludis`
- `portage`

---

*Document generated from PMS HTML source for GRPM development reference.*
