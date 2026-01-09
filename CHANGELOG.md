# Changelog

All notable changes to GRPM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### PMS-Compliant Atom Parser (v0.3.0-001)
- Character-by-character parser for package atoms per PMS Section 8.3
- All version operators: `=`, `>=`, `>`, `<=`, `<`, `~`, `=*` (glob)
- Blocker support: `!` (weak), `!!` (strong)
- Slot dependencies: `:slot`, `:slot/subslot`, `:*`, `:=`
- Repository constraints: `::repo-name`
- USE dependencies: `[flag,-flag,flag?,flag(+),flag(-)]`
- `ParseAtom()` function with comprehensive validation
- `Atom.Matches(*Package)` for package matching
- `Atom.ToConstraint()` for solver integration
- Full test coverage (30+ test cases)

### Planned
- Full EAPI 8 support (v0.3.0-002)
- GoSh integration for bash compatibility (v0.3.0-003)
- CMake/Meson build systems
- Performance optimization for large dependency graphs
- Web UI for daemon management
- Native GUI application ([gogpu/ui](https://github.com/gogpu/ui))

---

## [0.2.0] - 2026-01-09

### Ebuild Parser Improvements

This release focuses on improving ebuild parsing with proper variable expansion and removing hardcoded eclass handling.

### Added

#### Package Variable Expansion (v0.2.0-003)
- `PackageMetadata` struct for PMS 11.1 variables (P, PN, PV, PR, PVR, PF, CATEGORY)
- `NewPackageMetadata()` constructor for building metadata from package info
- `NewEbuildParserWithMetadata()` for creating parser with variable expansion context
- Proper `${P}`, `${PN}`, `${PV}`, `${PVR}`, `${PF}`, `${CATEGORY}` expansion in ebuild content
- Exported `ExtractVariable()` method for public access

#### REST API Improvements (v0.2.0-001)
- Unix socket support for daemon communication
- Improved socket handling and error messages

#### xargs Helper (v0.2.0-004)
- Native `xargs` implementation for ebuild helpers
- Cross-platform support without external dependencies

### Changed

#### Eclass Loading (v0.2.0-005)
- **BREAKING**: Removed `handleBuiltinEclass()` function
- All eclasses now loaded exclusively from repository
- Go helper implementations (tc-getCC, etc.) registered in interpreter
- More accurate Portage behavior emulation

### Fixed
- Eclass loading now correctly sources from repository filesystem
- Variable expansion works in DEPEND, RDEPEND, SRC_URI, SLOT, etc.
- Tests updated to use real eclass files instead of builtin mocks

---

## [0.1.1] - 2026-01-09

### Module Architecture Improvements

Major refactoring and improvements to the ebuild execution module.

### Changed

#### Code Architecture
- Split monolithic `helpers.go` (3200+ lines) into 11 focused modules:
  - `helpers_build.go` - Build commands (emake, econf)
  - `helpers_install.go` - Installation helpers (dobin, doins, dosym, fowners, fperms)
  - `helpers_fs.go` - Filesystem utilities
  - `helpers_msg.go` - Messaging (einfo, ewarn, die)
  - `helpers_use.go` - USE flag functions
  - `helpers_toolchain.go` - Toolchain detection
  - `helpers_version.go` - Version manipulation
  - `helpers_patch.go` - Patching functions
  - `helpers_doc.go` - Documentation helpers
  - `helpers_unpack.go` - Archive extraction
  - `helpers_default.go` - Default phase implementations
- Platform-specific code isolation (`helpers_chown_unix.go`, `helpers_chown_stub.go`)

#### Ebuild Execution
- Real eclass `inherit` with `EXPORT_FUNCTIONS` support
- Proper phase dispatch to custom ebuild functions (no more hardcoded defaults)
- Hook phases (pkg_preinst, pkg_postinst, pkg_prerm, pkg_postrm) working correctly

#### Installation Helpers
- Real `fowners` implementation with chown on Unix systems
- User/group lookup via `os/user` package
- Recursive ownership changes with `-R` flag

#### Package Queries
- Enhanced `has_version` with real VarDB queries
- Enhanced `best_version` with real VarDB queries
- Proper version constraint matching

#### Binary Packages
- Improved GPKG metadata parsing per GLEP 78
- SHA256 checksum generation
- Fixed test fixtures with valid nested tar archives

### Added
- `internal/ebuild/eclass.go` - Eclass registry, loader, stack management
- `internal/ebuild/script.go` - Ebuild script parsing for phase detection
- `internal/ebuild/phase_dispatch_test.go` - Phase dispatch tests
- `tests/integration/v0.2.0_integration_test.go` - Comprehensive integration tests

### Fixed
- Phase dispatch now calls custom ebuild functions instead of hardcoded defaults
- Binary package tests use valid nested metadata.tar archives
- Test fixtures properly structured for GPKG format

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

[Unreleased]: https://github.com/grpmsoft/grpm/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/grpmsoft/grpm/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/grpmsoft/grpm/releases/tag/v0.1.0
