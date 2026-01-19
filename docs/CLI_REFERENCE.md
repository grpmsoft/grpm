# GRPM CLI Reference

Complete command-line reference for GRPM. See [CHANGELOG](../CHANGELOG.md) for version history.

## Table of Contents

- [Synopsis](#synopsis)
- [Global Options](#global-options)
- [Commands](#commands)
  - [resolve](#resolve)
  - [install](#install)
  - [emerge](#emerge)
  - [remove](#remove)
  - [search](#search)
  - [info](#info)
  - [sync](#sync)
  - [fetch](#fetch)
  - [build](#build)
  - [update](#update)
  - [analyze](#analyze)
  - [tools](#tools)
  - [completion](#completion)
  - [doc](#doc)
  - [help](#help)
  - [status](#status)
  - [daemon](#daemon)
- [Exit Codes](#exit-codes)
- [Environment Variables](#environment-variables)
- [Examples](#examples)

---

## Synopsis

```
grpm [global-options] <command> [command-options] [arguments...]
```

---

## Global Options

| Option | Description |
|--------|-------------|
| `-V`, `--version` | Show version information and exit |
| `-v` | Verbose output (level 1) |
| `-vv` | More verbose output (level 2) |
| `-vvv` | Maximum verbosity (level 3) |
| `--verbose` | Alias for `-v` |

**Note:** `-v` is reserved for verbose mode. Use `-V` for version.

---

## Commands

### resolve

Resolve package dependencies using the SAT solver.

```
grpm resolve [options] <package|@set>...
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--mock` | Use mock repository for testing | `false` |
| `--pretend`, `-p` | Show what would be resolved (dry-run) | `false` |
| `--dry-run` | Alias for `--pretend` | `false` |
| `--deep`, `-D` | Traverse dependencies of already-installed packages | `false` |
| `--with-bdeps` | Include build-time dependencies (BDEPEND/DEPEND) for installed packages | `false` |
| `--emptytree`, `-e` | Assume no packages are installed (full dependency tree) | `false` |
| `--vardb <path>` | Path to installed packages database | `/var/db/pkg` |

**Dependency Filtering (Portage-compatible):**

By default, `grpm resolve` filters the dependency tree like Portage:
- Already-installed packages are skipped (unless `--deep`)
- Build-time dependencies (BDEPEND/DEPEND) for installed packages are skipped (unless `--with-bdeps`)
- Use `--emptytree` to see the full dependency tree as if nothing was installed

**Package Sets:**

| Set | Description |
|-----|-------------|
| `@world` | All explicitly installed packages + @system |
| `@system` | Core system packages from profile |
| `@selected` | User-selected packages (from world file) |

**Examples:**

```bash
# Resolve dependencies for a package
grpm resolve app-misc/hello

# Resolve all world packages
grpm resolve @world

# Resolve system packages
grpm resolve @system

# Dry-run mode
grpm resolve --pretend sys-libs/zlib

# Use custom repository
grpm resolve --repo /var/db/repos/custom dev-lang/go

# Use mock repository (testing)
grpm resolve --mock app-misc/hello

# Show full dependency tree (ignore installed packages)
grpm resolve --emptytree app-misc/mc

# Include installed packages and their dependencies
grpm resolve --deep @world

# Include build-time dependencies for installed packages
grpm resolve --with-bdeps app-misc/hello
```

**Output (normal):**
```
Dependency solution:
- app-misc/hello-2.10 [slot:0]
- sys-libs/zlib-1.2.13 [slot:0]
```

**Output (pretend):**
```
*** Dependency resolution (--pretend mode):
*** The following packages would be used:
[ebuild  N    ] sys-libs/zlib-1.2.13 [0]
[ebuild  N    ] app-misc/hello-2.10 [0]

Total: 2 package(s)
```

---

### install

Install packages to the system.

```
grpm install [options] <package|@set>...
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--mock` | Use mock repository for testing | `false` |
| `--binpkg` | Prefer binary packages (.gpkg.tar) | `false` |
| `--binpkg-dir <path>` | Binary package directory | `/var/cache/binpkgs` |
| `--snapshot-dir <path>` | Snapshot directory | `/.snapshots` |
| `--fs-type <type>` | Filesystem type (btrfs or zfs) | `btrfs` |
| `--no-snapshot` | Skip snapshot creation | `false` |
| `--pretend`, `-p` | Show what would be installed | `false` |
| `--dry-run` | Alias for `--pretend` | `false` |
| `--ask`, `-a` | Ask for confirmation before installing | `false` |

**Examples:**

```bash
# Install a package
sudo grpm install app-misc/hello

# Install all selected packages
sudo grpm install @selected

# Install from binary package
sudo grpm install --binpkg www-servers/nginx

# Dry-run installation
grpm install --pretend dev-lang/go

# Ask for confirmation
sudo grpm install --ask sys-libs/zlib

# Skip snapshot creation (testing)
sudo grpm install --no-snapshot --mock app-misc/hello
```

**Output (ask mode):**
```
*** Installation plan:
*** These are the packages that would be merged, in order:

[ebuild  N    ] sys-libs/zlib-1.2.13 to / USE="..."
[ebuild  N    ] app-misc/hello-2.10 to / USE="..."

Total: 2 package(s)

Would you like to merge these packages? [Yes/No]
```

---

### emerge

Build packages from source using ebuild executor.

```
grpm emerge [options] <package|@set>...
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--distdir <path>` | Directory for source tarballs | `/var/cache/distfiles` |
| `--tmpdir <path>` | Temporary build directory | `/var/tmp/portage` |
| `--root <path>` | Install to alternative root (chroot, stage tarball) | `/` |
| `--mock` | Use mock repository for testing | `false` |
| `--pretend`, `-p` | Show build plan without building | `false` |
| `--ask`, `-a` | Ask for confirmation before building | `false` |
| `--jobs <n>` | Number of parallel make jobs | From MAKEOPTS or 4 |
| `--keep-work` | Keep work directory after build | `false` |
| `--test` | Run test phase (make check/test) | `false` |
| `--onlydeps`, `-o` | Build dependencies only, skip target | `false` |
| `--check-tools` | Perform optional pre-build tool availability check | `false` |

**Build Phases:**

1. `pkg_setup` - Create build environment
2. `src_unpack` - Extract source tarball
3. `src_prepare` - Prepare sources (patches)
4. `src_configure` - Run ./configure
5. `src_compile` - Run make
6. `src_test` - Run tests (if `--test`)
7. `src_install` - Install to staging directory

**Examples:**

```bash
# Build from source
sudo grpm emerge app-misc/hello

# Build all system packages
sudo grpm emerge @system

# Build all selected packages
sudo grpm emerge @selected

# Show build plan
grpm emerge --pretend www-servers/nginx

# Build with 8 parallel jobs
sudo grpm emerge --jobs 8 dev-lang/go

# Keep work directory for debugging
sudo grpm emerge --keep-work app-misc/hello

# Run tests during build
sudo grpm emerge --test sys-libs/zlib

# Install to alternative root (chroot, stage tarball)
sudo grpm emerge --root /mnt/gentoo app-misc/hello

# Build dependencies only (Docker layer caching)
sudo grpm emerge --onlydeps app-misc/hello
```

**Work Directory Structure:**
```
/var/tmp/portage/
  app-misc/
    hello-2.10/
      work/           # Source files
        hello-2.10/   # Extracted tarball
      image/          # Installed files (DESTDIR)
      temp/           # Temporary files
```

---

### remove

Remove packages from the system.

```
grpm remove [options] <package>...
grpm uninstall [options] <package>...
```

**Note:** `uninstall` is an alias for `remove`.

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--pretend`, `-p` | Show what would be removed | `false` |
| `--depclean`, `-c` | Remove unused dependencies | `false` |
| `--force` | Force removal (skip dependency checks) | `false` |

**Examples:**

```bash
# Remove a package
sudo grpm remove app-misc/hello-2.10

# Dry-run removal
grpm remove --pretend sys-libs/zlib-1.2.13

# Force removal (skip dependency checks)
sudo grpm remove --force net-misc/curl-8.0.0

# Show what would be removed with verbose output
grpm remove -p -v app-misc/hello-2.10
```

---

### search

Search for packages in the repository.

```
grpm search [options] <query>
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--mock` | Use mock repository | `false` |
| `--desc`, `-S` | Search in descriptions too | `false` |

**Examples:**

```bash
# Search by package name
grpm search firefox

# Search in descriptions
grpm search --desc "web browser"

# Short form
grpm search -S editor
```

**Output:**
```
Searching for 'firefox'...

[ Results for search key : firefox ]
*  www-client/firefox
      Latest version available: 120.0
*  www-client/firefox-bin
      Latest version available: 120.0

[ Applications found : 2 ]
```

---

### info

Display detailed package information.

```
grpm info [options] <package>
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--mock` | Use mock repository | `false` |

**Examples:**

```bash
# Show package information
grpm info dev-lang/go

# Use mock repository
grpm info --mock app-misc/hello
```

**Output:**
```
============================================================
Package:     dev-lang/go
Version:     1.22.0
Slot:        0
Sub-Slot:    1.22
Repository:  gentoo

USE flags:   -pie -pie_guard -ssp

Dependencies (5):
  >=sys-devel/gcc-5
  sys-libs/glibc
  ...
============================================================
```

---

### sync

Synchronize the Portage repository.

```
grpm sync [options]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Repository path to sync | `/var/db/repos/gentoo` |
| `--url <url>` | Source repository URL | Auto-detected |
| `--method <method>` | Sync method: rsync, git, or auto | `auto` |
| `--skip-gpg-verify` | Skip GPG verification (NOT RECOMMENDED) | `false` |
| `--prefer-git` | Prefer Git in auto mode | `false` |

**Sync Methods:**

| Method | Description | GPG Support |
|--------|-------------|-------------|
| `git` | Git with shallow clone | Yes |
| `rsync` | Native Go rsync | No |
| `auto` | Auto-select best method | Varies |

**Examples:**

```bash
# Sync with auto-detection
sudo grpm sync

# Use Git (recommended for GPG verification)
sudo grpm sync --method git

# Use rsync (faster, no GPG)
sudo grpm sync --method rsync

# Custom repository URL
sudo grpm sync --url rsync://mirror.example.com/gentoo-portage

# Skip GPG (not recommended)
sudo grpm sync --skip-gpg-verify
```

---

### fetch

Download source tarballs (distfiles) for packages.

```
grpm fetch [options] <package|@set>...
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `-r <path>` | Alias for `--repo` | |
| `--distdir <path>` | Directory for downloaded sources | `/var/cache/distfiles` |
| `--pretend`, `-p` | Show what would be downloaded (dry-run) | `false` |
| `--verify` | Only verify existing files, don't download | `false` |

**Examples:**

```bash
# Download sources for a package
grpm fetch app-misc/hello

# Download sources for all system packages
grpm fetch @system

# Download sources for all selected packages
grpm fetch @selected

# Dry-run mode
grpm fetch --pretend www-servers/nginx

# Verify existing distfiles
grpm fetch --verify sys-libs/zlib

# Custom distdir
grpm fetch --distdir /mnt/distfiles dev-lang/go
```

**Output (pretend):**
```
Would fetch: hello-2.10.tar.gz (45 KB)
  URL: https://ftp.gnu.org/gnu/hello/hello-2.10.tar.gz
  Checksums: BLAKE2B, SHA512
```

**Notes:**
- Sources are automatically fetched during `grpm emerge`
- Uses GENTOO_MIRRORS from make.conf
- Supports resume for partial downloads
- Verifies BLAKE2B, SHA512, SHA256 checksums from Manifest

---

### build

Create binary packages from installed packages.

```
grpm build [options] <package>...
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--output <path>` | Output directory | `/var/cache/binpkgs` |
| `--format <fmt>` | Package format: gpkg or tbz2 | `gpkg` |
| `--compression <type>` | Compression: none, gzip, bzip2, xz, zstd | `zstd` |
| `--pretend`, `-p` | Show what would be built | `false` |

**Examples:**

```bash
# Build binary package
sudo grpm build app-misc/hello-2.10

# Specify output directory
sudo grpm build --output /mnt/binpkgs sys-libs/zlib-1.2.13

# Use legacy TBZ2 format
sudo grpm build --format tbz2 app-misc/hello-2.10

# Dry-run
grpm build --pretend app-misc/hello-2.10
```

---

### update

Update installed packages.

```
grpm update [options]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `--mock` | Use mock repository | `false` |
| `--pretend`, `-p` | Show what would be updated | `false` |
| `--deep`, `-D` | Include dependencies | `false` |

**Note:** Full update functionality is planned for future releases.

**Examples:**

```bash
# Show update plan
grpm update --pretend

# Include dependencies
grpm update --deep --pretend
```

---

### analyze

Analyze repository coverage and compatibility.

```
grpm analyze [options]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--repo <path>` | Path to Portage repository | `/var/db/repos/gentoo` |
| `-r <path>` | Alias for `--repo` | |
| `--output <format>` | Output format: text, json, markdown | `text` |
| `-o <format>` | Alias for `--output` | |
| `--category <name>` | Analyze specific category only | All |
| `-c <name>` | Alias for `--category` | |
| `--verbose`, `-v` | Show per-package details | `false` |

**Examples:**

```bash
# Analyze default Gentoo repository
grpm analyze

# Analyze specific category
grpm analyze --category app-misc

# JSON output for automation
grpm analyze --output json > coverage.json

# Markdown report for documentation
grpm analyze --output markdown > COVERAGE.md

# Verbose mode with package details
grpm analyze --verbose
```

**Output (text):**
```
GRPM Coverage Analysis
======================
Repository: /var/db/repos/gentoo
Total packages: 31653
Supported: 28488 (90.0%)
Unsupported: 3165 (10.0%)

Top blockers:
  - missing_eclass:java-utils-2: 1234
  - external_tool:latex: 567
  ...

By category:
  app-misc: 234/256 (91.4%)
  sys-libs: 189/201 (94.0%)
  ...
```

**Blocker Types:**

| Type | Description |
|------|-------------|
| `missing_eclass` | Eclass not available |
| `missing_helper` | Helper function not implemented |
| `unsupported_eapi` | EAPI version not supported |
| `fetch_restricted` | RESTRICT=fetch is set |
| `external_tool` | Required external tool missing |
| `parse_error` | Ebuild parsing failed |

---

### tools

Manage and check external tool availability.

```
grpm tools [options]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--check` | Show summary of tool availability | `false` |
| `--missing` | Show only missing tools with install hints | `false` |
| `--available` | Show only available tools with paths | `false` |
| `--category <name>` | Filter by category | All |
| `--for-eclass <name>` | Show tools needed for specific eclass | |
| `--paths` | Show PATH directories being searched | `false` |

**Tool Categories:**

| Category | Examples |
|----------|----------|
| `compilers` | gcc, clang, rustc, go |
| `build` | make, ninja, cmake, meson |
| `languages` | python, perl, ruby, node |
| `utilities` | patch, sed, awk, pkg-config |
| `compression` | gzip, bzip2, xz, zstd |
| `documentation` | doxygen, sphinx-build |
| `vcs` | git, svn, hg |

**Examples:**

```bash
# List all known tools with status
grpm tools

# Summary of availability
grpm tools --check

# Show missing tools with install suggestions
grpm tools --missing

# Show available tools with paths
grpm tools --available

# Filter by category
grpm tools --category compilers

# Tools needed for cmake.eclass
grpm tools --for-eclass cmake

# Show PATH directories
grpm tools --paths
```

**Output (missing):**
```
Missing tools (install suggestions):

  cmake - CMake build system
    Install: grpm emerge dev-build/cmake

  ninja - Ninja build tool
    Install: grpm emerge dev-build/ninja

  meson - Meson build system
    Install: grpm emerge dev-build/meson
```

**Notes:**
- Tools are checked in PATH automatically
- Tool dependencies are handled via BDEPEND (like Portage)
- Use `grpm emerge --check-tools` for optional pre-build validation

---

### completion

Generate shell completion scripts.

```
grpm completion <shell>
```

**Supported Shells:**

| Shell | Installation |
|-------|--------------|
| `bash` | `/etc/bash_completion.d/grpm` |
| `zsh` | `~/.zsh/completions/_grpm` |
| `fish` | `~/.config/fish/completions/grpm.fish` |

**Examples:**

```bash
# Generate bash completion
grpm completion bash > /etc/bash_completion.d/grpm

# Generate zsh completion
grpm completion zsh > ~/.zsh/completions/_grpm

# Generate fish completion
grpm completion fish > ~/.config/fish/completions/grpm.fish

# Test without installing (bash)
source <(grpm completion bash)
```

**Notes:**
- Completions include all commands, options, and package names
- Reload shell or source the file after installation
- For zsh, ensure `compinit` is initialized

---

### doc

Generate documentation in various formats.

```
grpm doc <subcommand> [options]
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `man` | Generate Unix man pages |

#### doc man

Generate man pages in troff format.

```
grpm doc man [command] [options]
```

**Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `--all` | Generate man pages for all commands | `false` |
| `--dir <path>` | Output directory for generated pages | stdout |

**Examples:**

```bash
# View main man page
grpm doc man | man -l -

# View command-specific man page
grpm doc man emerge | man -l -

# Generate all man pages to directory
grpm doc man --all --dir /usr/share/man/man1

# Generate specific command man page to file
grpm doc man sync > grpm-sync.1
```

**Output Format:**
- Main page: `grpm.1`
- Command pages: `grpm-<command>.1`
- Format: troff/roff (standard Unix man page format)

---

### help

Display help information.

```
grpm help [command]
grpm --help
grpm <command> --help
```

**Examples:**

```bash
# Show main help
grpm help
grpm --help

# Show command-specific help
grpm help emerge
grpm emerge --help
```

**Features:**
- Combined short/long flag display (`-p, --pretend`)
- Command descriptions with examples
- "See also" cross-references
- "Did you mean?" suggestions for typos

**Typo Correction:**

```bash
# Mistyped command
$ grpm emrge app-misc/hello
unknown command: emrge

Did you mean?
    emerge

Run 'grpm help' for usage.
```

---

### status

Show daemon status and system information.

```
grpm status
```

**Output (daemon running):**
```
GRPM Daemon Status
==========================================

Status: Running
   PID: 12345
   Socket: /var/run/grpm.sock
   Started: 2026-01-08 10:30:00
   Uptime: 2h 15m 30s

Status: healthy

PID file: /var/run/grpm.pid
```

**Output (daemon not running):**
```
GRPM Daemon Status
==========================================

Status: Stopped

Daemon is not running
   Use 'grpm daemon' to start the daemon
```

---

### daemon

Start the GRPM daemon.

```
grpm daemon
```

The daemon provides:
- gRPC server on Unix socket (`/var/run/grpm.sock`)
- REST API on HTTP (`127.0.0.1:8080`)
- Job queue with conflict detection
- Background monitoring

**Examples:**

```bash
# Start daemon (foreground)
sudo grpm daemon

# Start daemon (background)
sudo grpm daemon &

# Check if running
grpm status
```

---

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Package not found |
| 4 | Dependency resolution failed |
| 5 | Installation failed |
| 6 | Permission denied |

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GRPM_VERBOSE` | Enable verbose output (1=on) | `0` |
| `GRPM_DEBUG` | Enable debug output (1=on) | `0` |
| `GRPM_SOCKET` | Daemon socket path | `/var/run/grpm.sock` |
| `PORTAGE_DEBUG` | Enable Portage-compatible debug (1=on) | `0` |
| `GENTOO_MIRRORS` | Space-separated list of mirror URLs | Auto-detected |
| `DISTDIR` | Directory for downloaded sources | `/var/cache/distfiles` |

---

## Examples

### Common Workflows

**Install a package from source:**
```bash
sudo grpm sync
sudo grpm emerge --pretend app-misc/hello
sudo grpm emerge app-misc/hello
```

**Install from binary package:**
```bash
sudo grpm install --binpkg www-servers/nginx
```

**Check and update system:**
```bash
grpm status
sudo grpm sync
grpm update --pretend --deep
```

**Create binary package from installed:**
```bash
sudo grpm build app-misc/hello-2.10
ls /var/cache/binpkgs/app-misc/
```

**Remove package safely:**
```bash
grpm remove --pretend app-misc/hello-2.10
sudo grpm remove app-misc/hello-2.10
```

### Debugging

**Enable verbose output:**
```bash
grpm -vvv resolve app-misc/hello
GRPM_VERBOSE=1 grpm sync
```

**Test with mock repository:**
```bash
grpm resolve --mock sys-libs/zlib
grpm info --mock app-misc/hello
```

---

## See Also

- [Installation Guide](INSTALL.md)
- [README](../README.md)
- [CHANGELOG](../CHANGELOG.md)
