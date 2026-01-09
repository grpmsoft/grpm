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

**Current Version:** v0.2.0 (January 2026)

---

## Features

| Feature | Description |
|---------|-------------|
| **SAT-based Dependency Resolution** | Boolean satisfiability solver for guaranteed conflict-free resolution |
| **Binary Package Support** | Full GPKG (.gpkg.tar) and legacy TBZ2 (.tbz2) format support |
| **Transactional Updates** | Btrfs/ZFS snapshot-based rollbacks for safe system updates |
| **Source Building** | Complete ebuild execution with autotools workflow |
| **Distfile Fetching** | Automatic source download with mirror failover and resume |
| **Build Sandboxing** | Linux namespace isolation (mount, PID, network, IPC) |
| **Daemon Architecture** | gRPC + REST API for background operations |
| **Repository Sync** | Native rsync and Git sync with GPG verification |
| **Virtual Packages** | Provider selection with configuration support |
| **Metadata Caching** | SQLite-backed cache for fast package lookups |

---

## Quick Start

### Install from Binary

```bash
# Download latest release
wget https://github.com/grpmsoft/grpm/releases/latest/download/grpm_linux_amd64.tar.gz
tar -xzf grpm_linux_amd64.tar.gz
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

# Install from binary
sudo grpm install --binpkg app-misc/hello

# Build from source
sudo grpm emerge app-misc/hello

# Remove package
sudo grpm remove app-misc/hello
```

See [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) for complete command reference.

---

## Architecture

See **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** for the complete system diagram.

GRPM operates as a single binary in two modes:

```
grpm
├── CLI Mode (default)
│   └── resolve, install, emerge, remove, search, info, sync
│
└── Daemon Mode (grpm daemon)
    ├── gRPC Server (unix:///var/run/grpm.sock)
    ├── REST API (http://127.0.0.1:8080)
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

**v0.2.0** (Released) — Ebuild parser improvements, variable expansion, xargs helper.

**v0.3.0** (In Progress) — PMS compliance:
- ✅ PMS-compliant atom parser
- ✅ Full EAPI 8 support (REQUIRED_USE, SRC_URI arrow, IDEPEND, use_enable/use_with, dosym -r)
- ⏸️ GoSh integration (blocked, waiting for gosh library)

**v1.0.0** — Production ready after community validation (no fixed date).

See [CHANGELOG.md](CHANGELOG.md) for full release history.

---

## Security

To report security vulnerabilities, please see [SECURITY.md](SECURITY.md).

Do not open public issues for security reports.

---

## Community

- **Issues:** [GitHub Issues](https://github.com/grpmsoft/grpm/issues)
- **Discussions:** [GitHub Discussions](https://github.com/grpmsoft/grpm/discussions)

---

## License

GRPM is licensed under the [Apache License 2.0](LICENSE).

```
Copyright 2025 Andrey Kolkov and GRPM contributors
```
