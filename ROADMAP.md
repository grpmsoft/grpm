# GRPM Roadmap

> **Last Updated:** 2026-01-09
> **Current Version:** v0.2.0
> **In Development:** v0.3.0 (PMS Compliance + GoSh)

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

### v0.3.0 — PMS Compliance + GoSh (In Progress)

**Focus:** Full PMS compliance and bash compatibility

| Task | Status | Description |
|------|--------|-------------|
| Atom Parser | ✅ Done | PMS Section 8.3 compliant parser |
| EAPI 8 | 🔲 TODO | Full EAPI 8 feature support |
| GoSh Integration | 🔲 TODO | Replace mvdan.cc/sh with GoSh |

---

## Recent Releases

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
- Distfile fetching with mirror failover
- Profile system and configuration management
- Package installation/removal with collision detection

**Known Limitations:**
- Ebuild execution limited to autotools workflow
- Limited eclass support
- No EAPI 8 features
- CMake/Meson not supported

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

### Near-term (v0.3.0)

- [x] PMS-compliant atom parser (Section 8.3)
- [ ] Full EAPI 8 support
- [ ] GoSh integration (bash compatibility)
- [ ] Improved error messages and diagnostics

### Medium-term (v0.4.0+)

- [ ] CMake/Meson build system support
- [ ] Extended eclass support
- [ ] Performance optimization for large dependency graphs
- [ ] Web UI for daemon management
- [ ] Parallel package builds
- [ ] Distributed build support
- [ ] Plugin system for custom build systems

### Long-term (post-1.0)

- [ ] Native GUI application
- [ ] Cross-compilation support
- [ ] Container integration

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
