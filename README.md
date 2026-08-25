# GRPM

<p align="center">
  <img src="assets/devto_cover.png" alt="GRPM - Go Resource Package Manager" width="100%">
</p>

<p align="center">
  <strong>Next-generation Source-based Package Manager</strong>
</p>

<p align="center">
  <a href="https://github.com/grpmsoft/grpm/releases/latest"><img src="https://img.shields.io/github/v/release/grpmsoft/grpm?style=flat-square&logo=github&color=blue" alt="GitHub Release"></a>
  <a href="https://go.dev/dl/"><img src="https://img.shields.io/github/go-mod/go-version/grpmsoft/grpm?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://pkg.go.dev/github.com/grpmsoft/grpm"><img src="https://pkg.go.dev/badge/github.com/grpmsoft/grpm.svg" alt="Go Reference"></a>
  <a href="https://github.com/grpmsoft/grpm/actions"><img src="https://img.shields.io/github/actions/workflow/status/grpmsoft/grpm/test.yml?branch=main&style=flat-square&logo=github-actions&label=CI" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/grpmsoft/grpm"><img src="https://goreportcard.com/badge/github.com/grpmsoft/grpm?style=flat-square" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/grpmsoft/grpm?style=flat-square" alt="License"></a>
  <a href="https://github.com/grpmsoft/grpm/stargazers"><img src="https://img.shields.io/github/stars/grpmsoft/grpm?style=flat-square&logo=github" alt="GitHub Stars"></a>
  <a href="https://github.com/grpmsoft/grpm/issues"><img src="https://img.shields.io/github/issues/grpmsoft/grpm?style=flat-square&logo=github" alt="GitHub Issues"></a>
</p>

GRPM (Go Resource Package Manager) is a modern source-based package manager written in Go, inspired by and fully compatible with **Gentoo's Portage**. It brings SAT-based dependency resolution, transactional updates with filesystem snapshots, and binary package support. While rooted in the Gentoo ecosystem, GRPM is designed to be distribution-agnostic and extensible for any Linux distribution.

**Current Version:** [Latest Release](https://github.com/grpmsoft/grpm/releases/latest)

---

## Features

| Feature | Description |
|---------|-------------|
| **SAT-based Dependency Resolution** | Boolean satisfiability solver for guaranteed conflict-free resolution |
| **Binary Package Support** | Full GPKG (.gpkg.tar) and legacy TBZ2 (.tbz2) format support |
| **Transactional Updates** | Btrfs/ZFS snapshot-based rollbacks for safe system updates |
| **Source Building** | Ebuild execution with autotools (full); CMake and Meson (basic); hardened bash interpreter (~160 Go helpers) |
| **Build Systems** | toolchain-funcs, flag-o-matic, cmake.eclass, meson.eclass |
| **Language Ecosystems** | Python (distutils-r1), Rust (cargo.eclass), Go (go-module.eclass) — basic support |
| **Multilib Support** | 32-bit/64-bit library support with ABI management |
| **Package Sets** | @world, @system, @selected in ALL commands (resolve, install, emerge, fetch) |
| **Distfile Fetching** | Automatic source downloading with mirror failover and USE-conditional filtering |
| **Coverage Analysis** | Repository compatibility analysis with `grpm analyze` |
| **Tool Detection** | External tool checking with `grpm tools` |
| **Daemon Architecture** | gRPC + REST API for background operations |
| **Portage-Style Output** | Professional colored logging matching emerge output |
| **Repository Sync** | Native rsync and Git sync with GPG verification |
| **Virtual Packages** | Provider selection with configuration support |
| **Metadata Caching** | SQLite-backed cache for fast package lookups |
| **Configuration Management** | Dynamic make.conf, repos.conf, package.use with full Portage compatibility |
| **Shell Completion** | Bash, Zsh, Fish completion with `grpm completion` |
| **Man Pages** | Unix manual pages with `grpm doc man` |
| **Smart Suggestions** | "Did you mean?" typo correction for commands |

---

## Quick Start

### Install from Binary

```bash
# Download latest release (check https://github.com/grpmsoft/grpm/releases for VERSION)
VERSION="0.9.4"
wget "https://github.com/grpmsoft/grpm/releases/download/v${VERSION}/grpm_${VERSION}_linux_x86_64.tar.gz"
tar -xzf "grpm_${VERSION}_linux_x86_64.tar.gz"
sudo install -m 0755 grpm /usr/bin/grpm

# Verify
grpm -V
```

### Build from Source

```bash
# Requires Go 1.25+
git clone https://github.com/grpmsoft/grpm.git
cd grpm
make build
sudo make install
```

### Basic Usage

```bash
# Sync repository
sudo grpm sync

# Search packages
grpm search firefox

# Show package info
grpm info dev-lang/go

# Download sources (without building)
grpm fetch app-misc/hello

# Build from source (auto-fetches sources)
sudo grpm emerge app-misc/hello

# Build specific version (PMS-compliant atoms)
sudo grpm emerge "=sys-devel/gcc-13.4.1_p20250807"
sudo grpm emerge ">=dev-libs/openssl-3.0"

# Build to alternative root (chroot, stage tarball)
sudo grpm emerge --root /mnt/gentoo app-misc/hello

# Build dependencies only (useful for Docker layer caching)
sudo grpm emerge --onlydeps app-misc/hello

# Work with package sets
grpm resolve @world              # Resolve all world packages
grpm resolve @system             # Resolve system packages
sudo grpm emerge @selected       # Build all selected packages
grpm fetch @system               # Download sources for system packages

# Install from binary
sudo grpm install --binpkg app-misc/hello

# Remove package
sudo grpm remove app-misc/hello

# Analyze repository coverage
grpm analyze --category app-misc

# Check available tools
grpm tools --missing
```

See [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) for complete command reference.

### Shell Completion

```bash
# Bash
grpm completion bash > /etc/bash_completion.d/grpm

# Zsh
grpm completion zsh > ~/.zsh/completions/_grpm

# Fish
grpm completion fish > ~/.config/fish/completions/grpm.fish
```

### Man Pages

```bash
# View main man page
grpm doc man | man -l -

# View command-specific page
grpm doc man emerge | man -l -

# Generate all pages to directory
grpm doc man --all --dir /usr/share/man/man1
```

---

## Architecture

See **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** for the complete system diagram.

GRPM operates as a single binary in two modes:

```
grpm
├── CLI Mode (default)
│   ├── Package: resolve, install, emerge, remove, search, info
│   ├── Repository: sync, fetch
│   └── Analysis: analyze, tools
│
└── Daemon Mode (grpm daemon)
    ├── gRPC Server (unix:///var/run/grpm.sock)
    ├── REST API (unix:///var/run/grpm-rest.sock)
    └── Job Queue with conflict detection
```

**Design Principles:**

- **Domain-Driven Design** — Clean separation of domain, application, and infrastructure
- **SAT Solver** — Guaranteed conflict-free dependency resolution
- **Portage Compatibility** — Standard Gentoo paths and formats
- **Transactional Safety** — Filesystem snapshots before destructive operations

---

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | x86_64 (amd64) | Primary |
| Linux | ARM64 | Supported |
| Linux | ARMv7, ARMv6 | Supported |
| Linux | i386 | Supported |

**Primary target:** Gentoo Linux and compatible distributions (Calculate, Funtoo).

**Designed for:** Any Linux distribution seeking source-based package management.

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/ARCHITECTURE.md) | System architecture diagram |
| [Installation Guide](docs/INSTALL.md) | Detailed installation instructions |
| [CLI Reference](docs/CLI_REFERENCE.md) | Complete command documentation |
| [PMS Compliance](docs/PMS_COMPLIANCE.md) | Implementation status per PMS specification |
| [PMS Reference](docs/pms/README.md) | Gentoo Package Manager Specification |
| [Contributing](CONTRIBUTING.md) | Development guidelines |
| [Changelog](CHANGELOG.md) | Version history |
| [Roadmap](ROADMAP.md) | Future plans |

---

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make fmt        # Format code
make ci         # Full CI checks
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines and [AGENTS.md](AGENTS.md) for AI-assisted development.

---

## Roadmap

> **v0.10.0 Phase — Audit-Driven Quality & Bash Evolution**
>
> Rapid development complete. Infrastructure, build quality, and security hardening done.
> **Community audit completed (2026-02-09): PMS compliance validated at ~60%.**
> **98.2% tree coverage verified on real Gentoo WSL2.**

**Completed Features:**
- ✅ SAT-based dependency resolution
- ✅ Ebuild execution (autotools full; CMake, Meson basic)
- ✅ Language ecosystem support (Python, Rust, Go — basic)
- ✅ Multilib support (32-bit/64-bit)
- ✅ Binary package support (GPKG, TBZ2)
- ✅ Repository sync (rsync, git) with GPG verification
- ✅ Portage-style logging with colored output
- ✅ Verbose modes (`-v`, `-vv`, `-vvv`) for debugging
- ✅ Dynamic make.conf parsing with variable expansion
- ✅ repos.conf support with Portage fallback chain
- ✅ package.use pattern matching with atom specificity
- ✅ package.mask filtering in solver (masked packages excluded)
- ✅ KEYWORDS filtering in solver (unkeyworded packages excluded)
- ✅ Package sets (@world, @system, @selected) in all commands
- ✅ Enterprise tool handling (Portage-compatible BDEPEND)
- ✅ Portage-compatible dependency filtering (installed packages, BDEPEND/DEPEND)
- ✅ Hardened bash interpreter with function body stripping and USE-conditional distfile filtering
- ✅ Eclass metadata extraction with BASH_VERSINFO emulation and stdout isolation

**Known Limitations:**
- Bash interpreter (`mvdan.cc/sh`) handles ~90% of bash; complex eclasses may fail (interpreter evolution planned for v0.10.0)
- Build success ~20% of @world (autotools packages pass; Python/CMake/Meson builds need interpreter improvements)
- PMS compliance ~60% for simple packages (validated by community audit, 2026-02-09)

**v1.0.0** — Production ready after community validation (no fixed date).

See [ROADMAP.md](ROADMAP.md) for detailed plans and [CHANGELOG.md](CHANGELOG.md) for full release history.

---

## Security

To report security vulnerabilities, please see [SECURITY.md](SECURITY.md).

Do not open public issues for security reports.

---

## Community

- **Issues:** [GitHub Issues](https://github.com/grpmsoft/grpm/issues)
- **Discussions:** [GitHub Discussions](https://github.com/grpmsoft/grpm/discussions)

---



## Star History

<a href="https://starhistory.io">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.starhistory.io/png?repos=grpmsoft/grpm&style=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.starhistory.io/png?repos=grpmsoft/grpm&style=professional" />
   <img alt="Star History Chart" src="https://api.starhistory.io/png?repos=grpmsoft/grpm" width="800" />
 </picture>
</a>

## License

GRPM is licensed under the [Apache License 2.0](LICENSE).

```
Copyright 2025-2026 Andrey Kolkov and GRPM contributors
```
