# GRPM Roadmap

> **Last Updated:** 2026-01-10
> **Current Version:** v0.4.0
> **Next Release:** v0.5.0 (Language Ecosystems)

---

## Vision

GRPM aims to be a modern, reliable package manager for Gentoo Linux with:
- SAT-based dependency resolution for guaranteed conflict-free solutions
- Full Portage/ebuild compatibility
- Binary package support (GPKG and TBZ2 formats)
- Modern daemon architecture with gRPC/REST APIs
- High performance through native Go implementation

---

## Current Development

### v0.5.0 — Language Ecosystems (Planned)

**Focus:** Python, Rust, Go package support and system management

| Task | Status | Description |
|------|--------|-------------|
| Python Eclasses | Planned | python-utils-r1, python-single-r1, python-r1 |
| Package Sets | Planned | @world, @system support |
| Multilib Eclass | Planned | 32-bit/64-bit library support |
| REQUIRED_USE Solver | Planned | Automatic USE flag resolution |
| cargo.eclass | Planned | Rust package support |
| go-module.eclass | Planned | Go package support |

---

## Recent Releases

### v0.4.0 — Build Systems (2026-01-10)

**Major release** with CMake and Meson build system support (~60% tree coverage):

- **CMake Build System** — Full cmake.eclass with Ninja/Makefiles generators
- **Meson Build System** — Full meson.eclass with ninja backend
- **toolchain-funcs Eclass** — tc-getCC, tc-getCXX, tc-export, tc-arch, cross-compilation
- **flag-o-matic Eclass** — append-flags, filter-flags, strip-flags with glob patterns
- **Repository Cache** — SQLite-backed metadata cache (modernc.org/sqlite, pure Go)
- **Integration Tests** — 2768 lines covering autotools, cmake, meson packages

### v0.3.0 — PMS Compliance (2026-01-10)

**Major release** with comprehensive PMS (Package Manager Specification) compliance:

- **EAPI Feature Matrix** — Complete EAPI 0-8 support with 30+ feature flags
- **PMS-Compliant Atom Parser** — Section 8.3 compliant parser with all operators
- **Version Commands** — `ver_test`, `ver_cut`, `ver_rs` per PMS Algorithm 3.2-3.7
- **Environment Variables** — PVR, ROOT, EROOT, SYSROOT, ESYSROOT, BROOT
- **Error Handling** — `assert`, `nonfatal` commands
- **Phase Functions** — `pkg_pretend`, `pkg_config`, `pkg_info`, `pkg_nofetch`
- **Installation Helpers** — `into`, `doinfo`, `domo`
- **Default Functions** — `default_src_*` implementations
- **Banned Commands** — EAPI-aware validation with deprecation warnings
- **mvdan.cc/sh Integration** — Direct bash compatibility (GoSh wrapper not needed)
- **PMS Test Suite** — 1076 lines of compliance tests

### v0.2.1 — PMS Version Comparison (2026-01-09)

- PMS-compliant version comparison (Algorithm 3.2-3.7)
- Version suffix ordering fix (`_alpha < _beta < _pre < _rc < release < _p`)
- Leading zero and letter suffix handling

### v0.2.0 — Ebuild Parser Improvements (2026-01-09)

- Package variable expansion (${P}, ${PN}, ${PV}, ${PVR}, ${PF}, ${CATEGORY})
- Removed builtin eclass handling (all eclasses from repository)
- REST API socket improvements
- Native xargs helper implementation

### v0.1.1 — Module Architecture (2026-01-09)

- Real eclass `inherit` with `EXPORT_FUNCTIONS`
- Proper phase dispatch to custom functions
- Hook phases working correctly
- Enhanced `has_version`/`best_version`

### v0.1.0 — Initial Public Release (2026-01-08)

First public release with core functionality:

- SAT-based dependency resolution (gophersat)
- Daemon architecture (gRPC + REST API)
- Native repository sync (rsync/git with GPG verification)
- Binary package support (read/write GPKG and TBZ2)
- Source building (emerge command with autotools workflow)
- Profile system and configuration management
- Package installation/removal with collision detection

---

## Development Approach

GRPM follows iterative development with community feedback:

1. **v0.x.x releases** — Feature development, API refinement, bug fixes
2. **Community testing** — Real-world validation on Gentoo systems
3. **API stabilization** — Freeze public APIs based on feedback
4. **v1.0.0** — Production release after community validation and API freeze

**v1.0.0 will be released only when:**
- API is stable and frozen
- Community has validated the software
- No critical bugs remain
- Documentation is complete

No fixed timeline for v1.0.0 — quality and stability over deadlines.

---

## Planned Features (v0.x.x)

### Near-term (v0.5.0)

- [ ] Python eclasses (python-utils-r1, python-single-r1, python-r1)
- [ ] Package sets (@world, @system)
- [ ] Multilib eclass
- [ ] REQUIRED_USE solver
- [ ] cargo.eclass for Rust packages
- [ ] go-module.eclass for Go packages

### Medium-term (v0.6.0+)

- [ ] Web UI for daemon management
- [ ] Parallel package builds
- [ ] Distributed build support
- [ ] Plugin system for custom build systems

### Long-term (post-1.0)

- [ ] Native GUI application
- [ ] Cross-compilation support
- [ ] Container integration

---

## Completed Features

### v0.4.0 (2026-01-10)
- [x] CMake build system support
- [x] Meson build system support
- [x] toolchain-funcs eclass
- [x] flag-o-matic eclass
- [x] Repository metadata cache
- [x] Integration test framework

### v0.3.0 (2026-01-10)
- [x] EAPI 0-8 feature matrix
- [x] PMS-compliant atom parser
- [x] Version manipulation commands
- [x] Error handling commands
- [x] Default phase functions
- [x] mvdan.cc/sh bash integration

---

## Quality Targets

| Metric | Target |
|--------|--------|
| Test Coverage | 80%+ overall, 90%+ for domain logic |
| Performance | Competitive with emerge |
| Documentation | Complete API docs + user guides |
| Stability | Zero critical bugs |

---

## How to Contribute

1. Try GRPM and report issues
2. Submit feature requests via GitHub Issues
3. Contribute code following [CONTRIBUTING.md](CONTRIBUTING.md)
4. Help with documentation and testing

---

## Resources

- **Repository:** https://github.com/grpmsoft/grpm
- **Issues:** https://github.com/grpmsoft/grpm/issues
- **Documentation:** [docs/](docs/)

---

*This roadmap evolves based on community feedback and project needs.*
