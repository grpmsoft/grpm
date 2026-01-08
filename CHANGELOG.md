# Changelog

All notable changes to GRPM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Full EAPI 8 support
- CMake/Meson build systems
- Performance optimization for large dependency graphs
- Web UI for daemon management
- Native GUI application ([gogpu/ui](https://github.com/gogpu/ui))

---

## [0.1.0] - 2026-01-08

### Initial Public Release

GRPM (Go Resource Package Manager) is a modern reimplementation of Gentoo's Portage package manager in pure Go.

### Added

#### Core Features
- SAT-based dependency resolution using gophersat library
- Domain-Driven Design architecture with rich domain model
- Gentoo version comparison and constraint handling
- Slot and subslot support with ABI tracking
- USE flag resolution and conditional dependencies
- Circular dependency detection
- Slot and version conflict resolution

#### Package Management
- Binary package support (GPKG and TBZ2 formats)
- Package installation with collision detection
- Package removal with dependency tracking
- VarDB integration (Gentoo `/var/db/pkg` format)
- Package signing (GPG/SSH/RSA)
- Local and remote binhost support

#### Source Building
- Full ebuild execution engine
- PMS phase implementation (setup through install)
- Autotools workflow support (configure/make/make install)
- Distfile fetching with mirror failover and resume
- Build sandboxing with Linux namespaces
- Parallel compilation support

#### System Integration
- Profile system with inheritance
- Configuration management (make.conf, package.*)
- Metadata caching (SQLite backend)
- CONFIG_PROTECT support
- Privilege dropping (userpriv/userfetch)
- Virtual package support
- Multiple repository/overlay support

#### Daemon & API
- Single binary with CLI and daemon modes
- gRPC service on Unix socket
- REST API for monitoring and integration
- Job queue with worker pool
- Parallel build scheduler

#### Repository Management
- Native rsync implementation
- Git sync with GPG verification
- Auto-selection strategy
- Pluggable sync module

#### Advanced Features
- Slot collision resolution with autounmask
- @world/@system/@selected package sets
- System update with --deep/--newuse flags
- Dependency cleanup (depclean)
- Parallel package builds (--jobs)

### CLI Commands

| Command | Description |
|---------|-------------|
| `resolve` | Resolve dependencies with SAT solver |
| `install` | Install packages (binary or source) |
| `emerge` | Build packages from source |
| `remove` | Remove installed packages |
| `search` | Search for packages |
| `info` | Display package information |
| `sync` | Synchronize repository |
| `update` | Update @world/@system packages |
| `depclean` | Remove orphaned packages |
| `build` | Create binary packages |
| `status` | Show daemon status |
| `daemon` | Start daemon mode |

### Technical Details

- **Language**: Go 1.25+
- **License**: Apache-2.0
- **Platforms**: Linux (amd64, arm64, armv7, armv6, i386)
- **Test Coverage**: ~70%
- **Total Code**: ~60,000 lines

### Known Limitations

- Ebuild execution limited to autotools workflow
- Limited eclass support (toolchain-funcs, eutils, multilib)
- No EAPI 8 features yet
- CMake/Meson not supported

### Dependencies

- github.com/crillab/gophersat - SAT solver
- github.com/spf13/cobra - CLI framework
- google.golang.org/grpc - gRPC server
- github.com/gokrazy/rsync - Native rsync
- modernc.org/sqlite - Pure Go SQLite
- mvdan.cc/sh/v3 - Bash interpreter

---

## Links

- **Repository**: https://github.com/grpmsoft/grpm
- **Documentation**: https://github.com/grpmsoft/grpm/tree/master/docs
- **Issues**: https://github.com/grpmsoft/grpm/issues
- **License**: [Apache-2.0](LICENSE)

[Unreleased]: https://github.com/grpmsoft/grpm/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/grpmsoft/grpm/releases/tag/v0.1.0
