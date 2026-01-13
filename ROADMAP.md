# GRPM Roadmap

> **Documentation Hotfix (v0.7.3)**
>
> Rapid development complete (v0.1.0 → v0.5.0).
> Infrastructure release complete (v0.6.0).
> Portage-style logging (v0.7.0) + compatibility fixes (v0.7.2) + docs (v0.7.3).
> **98.2% tree coverage verified on real Gentoo!**

---

## Vision

GRPM aims to be a modern, reliable package manager for Gentoo Linux with:
- SAT-based dependency resolution for guaranteed conflict-free solutions
- Full Portage/ebuild compatibility via dynamic eclass loading
- Binary package support (GPKG and TBZ2 formats)
- Modern daemon architecture with gRPC/REST APIs
- High performance through native Go implementation

---

## Architecture Breakthrough (v0.5.2)

**Dynamic Eclass Loading via mvdan.cc/sh**

```
Eclass (bash) → mvdan.cc/sh interpreter → execHandler → Go helpers OR shell pass-through
```

Key insight: **Eclasses don't need Go implementations.** They are loaded dynamically from the repository and executed by the bash interpreter. Commands are either:
- Intercepted by Go helpers (100+ implemented)
- Passed through to real shell execution

**Implication:** Most eclasses (xdg, desktop, systemd, edo, optfeature, etc.) work out-of-box.

---

## Current Status

### What's Implemented (~75% Tree Coverage)

| Category | Features |
|----------|----------|
| **Core** | SAT solver, version comparison, slot/subslot, USE flags |
| **Build Systems** | autotools, CMake, Meson |
| **Languages** | Python (distutils-r1), Rust (cargo.eclass), Go (go-module.eclass) |
| **Multilib** | 32-bit/64-bit ABI support |
| **Binary Packages** | GPKG (.gpkg.tar), TBZ2 (.tbz2) read/write |
| **Sync** | rsync, git with GPG verification |
| **API** | gRPC + REST daemon architecture |
| **Eclass Loading** | Dynamic via mvdan.cc/sh (100+ helpers) |

### What's Blocking 75% → 90%?

| Blocker | Impact | Status |
|---------|--------|--------|
| ~~No distfile fetching~~ | ~4% | ✅ v0.6.0 |
| ~~Missing debug-print functions~~ | < 1% | ✅ v0.6.0 |
| ~~Untested eclasses~~ | Unknown | ✅ v0.6.0 (21 tested) |
| ~~External tools on user's system~~ | Variable | ✅ v0.6.0 (detection) |
| **Remaining gaps** | ~5% | v0.7.0 (analysis) |

---

## Release History

### v0.7.2 — Portage Compatibility Hotfix (Current)

**Critical fixes for real-world Portage compatibility:**

- **package.mask directory** — EAPI 7+ supports directories, not just files
- **SRC_URI parsing for fetch** — Explicit upstream URLs for .asc signatures
- **USE conditional handling** — `nil` activeFlags includes ALL conditionals

### v0.7.0 — Portage-Style Logging

**Professional terminal output:**

- **Portage-style prefixes** — `>>>`, `***`, `!!!` for different message types
- **Colored output** — Green (success), Yellow (warning), Red (error)
- **Phase headers** — Clear separation of build phases

### v0.6.0 — Infrastructure & Quality

**Production readiness infrastructure:**

- **Distfile Fetching** — `grpm fetch` command, automatic source downloading
- **Debug Helpers** — debug-print family (PMS 12.3.16 compliant)
- **Eclass Testing** — 21 eclasses with integration tests
- **Coverage Analyzer** — `grpm analyze` command with text/json/markdown output
- **Tool Detection** — `grpm tools` command, 50+ tools registered

### v0.5.2 — Dynamic Eclass Loading

**Architecture overhaul** enabling universal eclass support:

- **Dynamic eclass loading** via mvdan.cc/sh bash interpreter
- **HybridLoader** with Go fallback implementations
- New `internal/eclass/` package (3300+ lines)
- Addresses community feedback about hardcoded eclasses

### v0.5.1 — Hotfix: Multilib ABI Lookup

- Fix deterministic ABI lookup in multilib functions

### v0.5.0 — Language Ecosystems

**Final release of rapid development phase** with Python, Rust, and Go support:

- **Python Eclasses** — python-utils-r1, python-single-r1, python-r1, python-any-r1, distutils-r1
- **Package Sets** — @world, @system, @selected, @preserved-rebuild
- **Multilib Eclass** — 32-bit/64-bit ABI support
- **REQUIRED_USE Solver** — Automatic USE flag resolution
- **cargo.eclass** — Rust packages with crate vendoring
- **go-module.eclass** — Go packages with EGO_SUM support

### v0.4.0 — Build Systems

**Major release** with CMake and Meson build system support (~60% tree coverage):

- **CMake Build System** — Full cmake.eclass with Ninja/Makefiles generators
- **Meson Build System** — Full meson.eclass with ninja backend
- **toolchain-funcs Eclass** — tc-getCC, tc-getCXX, tc-export, tc-arch, cross-compilation
- **flag-o-matic Eclass** — append-flags, filter-flags, strip-flags with glob patterns
- **Repository Cache** — SQLite-backed metadata cache (modernc.org/sqlite, pure Go)
- **Integration Tests** — 2768 lines covering autotools, cmake, meson packages

### Earlier Releases

- **v0.3.0** — PMS Compliance (EAPI 0-8, ver_* commands, mvdan.cc/sh)
- **v0.2.x** — Parser improvements, version comparison hotfixes
- **v0.1.x** — Foundation, module architecture, initial release

---

## Roadmap to v1.0.0

```
v0.7.3 ← CURRENT (Documentation Hotfix)
    │   ✅ v0.6.0: Distfile fetching, debug helpers, coverage analyzer
    │   ✅ v0.7.0: Portage-style logging
    │   ✅ v0.7.2: package.mask directories, SRC_URI parsing
    │   ✅ v0.7.3: Documentation fixes
    │   ✅ 98.2% tree coverage on real Gentoo!
    ↓
v0.8.0 — Production Hardening (2 tasks)
    │   • Performance optimization
    │   • Structured logging
    ↓
v0.9.0 — Pre-Release (2 tasks)
    │   • Community testing program
    │   • Documentation finalization
    ↓
v1.0.0 — Production Release
         90%+ coverage verified, community sign-off
```

---

## v0.6.0 — Infrastructure & Quality ✅ COMPLETE

**Focus:** Enable actual package building and verify coverage claims

| ID | Task | Priority | Status |
|----|------|----------|--------|
| v0.6.0-001 | **Distfile Fetching** | P0 | ✅ Done |
| v0.6.0-002 | Missing Helpers | P1 | ✅ Done |
| v0.6.0-003 | Eclass Integration Testing | P1 | ✅ Done |
| v0.6.0-004 | Coverage Analyzer | P2 | ✅ Done |
| v0.6.0-005 | External Tool Detection | P2 | ✅ Done |

**Delivered:**
- `grpm fetch` — Automatic source downloading with mirror failover
- `debug-print`, `debug-print-function`, `debug-print-section` helpers
- 21 eclass integration tests (toolchain-funcs, cmake, meson, python-*, etc.)
- `grpm analyze` — Repository coverage analysis (text/json/markdown)
- `grpm tools` — External tool detection with install suggestions

---

## v0.7.0 — Validation & Documentation

**Focus:** Document what works, identify remaining gaps

| Task | Priority |
|------|----------|
| Eclass Compatibility Matrix | P1 |
| Helper Function Gap Analysis | P1 |
| Error Handling Improvements | P2 |

Key deliverables:
- docs/eclass-compatibility.md (machine + human readable)
- Complete helper function catalog
- Production-ready error messages

---

## v0.8.0 — Production Hardening

**Focus:** Performance and operational quality

| Task | Priority |
|------|----------|
| Performance Optimization | P1 |
| Structured Logging | P2 |

Key deliverables:
- 2x faster dependency resolution
- Structured logging with levels (-v, -vv, -vvv)
- Per-package build logs

---

## v0.9.0 — Pre-Release

**Focus:** Community validation

| Task | Priority |
|------|----------|
| Community Testing Program | P0 |
| Documentation Finalization | P1 |

Key deliverables:
- Alpha/beta testing with 50+ testers
- 1,000+ packages tested on real systems
- Complete user documentation and migration guide

---

## v1.0.0 — Production Release

**Release Criteria:**
- ✅ 90%+ tree coverage verified by community
- ✅ 0 critical bugs, < 5 high bugs
- ✅ API stable and documented
- ✅ User guide and migration guide complete
- ✅ Ebuild in GURU overlay
- ✅ Community sign-off

---

## Quality Targets

| Metric | Target |
|--------|--------|
| Test Coverage | 70%+ overall, 90%+ for domain logic |
| Performance | Competitive with emerge |
| Documentation | Complete API docs + user guides |
| Stability | Zero critical bugs |
| Tree Coverage | 90%+ of Gentoo packages |

---

## How to Contribute

1. **Try GRPM** and report issues
2. **Test eclasses** and report what works/doesn't
3. Submit feature requests via GitHub Issues
4. Contribute code following [CONTRIBUTING.md](CONTRIBUTING.md)
5. Help with documentation and testing

---

## Resources

- **Repository:** https://github.com/grpmsoft/grpm
- **Issues:** https://github.com/grpmsoft/grpm/issues
- **Kanban Board:** [docs/dev/kanban/BOARD.md](docs/dev/kanban/BOARD.md)
- **Documentation:** [docs/](docs/)

---

*This roadmap evolves based on community feedback and project needs.*
*Last updated: 2026-01-12 (v0.6.0 release)*
