# GRPM Roadmap

> **v0.10.0 Phase — Audit-Driven Quality & Bash Evolution**
>
> Rapid development complete (v0.1.0 → v0.5.0).
> Infrastructure & quality complete (v0.6.0).
> Portage compatibility & security (v0.7.x).
> Configuration management (v0.8.0-v0.8.4).
> Enterprise CLI & emerge filtering (v0.9.0-v0.9.2).
> Install helpers & bash interpreter hardening (v0.9.3-v0.9.4).
> **Community code audit completed (2026-02-09): PMS compliance validated at ~60%.**
> **98.2% tree coverage verified on real Gentoo WSL2.**

---

## Vision

GRPM aims to be a modern, reliable package manager for Gentoo Linux with:
- SAT-based dependency resolution for guaranteed conflict-free solutions
- Full Portage/ebuild compatibility via advanced bash interpretation
- Binary package support (GPKG and TBZ2 formats)
- Modern daemon architecture with gRPC/REST APIs
- High performance through native Go implementation

---

## Path to v1.0.0

```
v0.9.4 ← LATEST RELEASE (Bash Interpreter Hardening)
    │
    │   ✅ v0.6.0: Distfile fetching, debug helpers, coverage analyzer
    │   ✅ v0.7.x: Portage compatibility, security fixes
    │   ✅ v0.8.x: Configuration management, mask/keywords, package sets
    │   ✅ v0.9.x: Enterprise CLI, emerge filtering, bash hardening
    │   ✅ Community audit: PMS compliance validated (~60%/~51%)
    │   ✅ 98.2% tree coverage on real Gentoo WSL2
    ↓
v0.10.x ← CURRENT PHASE (Audit-Driven Quality & Bash Evolution)
    │
    │   Wave 1: Audit bug fixes (=* glob, phase defaults, dead code)
    │   Wave 2: Bash interpreter evolution (fix mvdan/sh or custom interpreter)
    │   Wave 3: @world 90%+ build success, community validation
    ↓
v0.11.x — Community Testing & Feedback
    │
    │   Alpha/beta testing with community
    │   Documentation finalization
    │   Bug fixes from real-world usage
    ↓
v1.0.0-rc — Release Candidate
    │
    │   API freeze, stability testing
    ↓
v1.0.0 — Production Release
         90%+ @world success, community sign-off
```

---

## v0.10.0 — Audit-Driven Quality & Bash Evolution (Current)

Following the community code audit (2026-02-09), development is organized in three waves.

### Wave 1: Quick Wins from Audit

| Task | Priority | Description |
|------|----------|-------------|
| Audit bug fixes | P0 | Fix `=*` glob operator, phase defaults routing, dead code removal |
| Signature file defense | P0 | 3-layer defense against incorrect .sig downloads |

### Wave 2: Architecture Evolution

| Task | Priority | Blocked By |
|------|----------|------------|
| Bash interpreter hardening (Phase 2+) | P0 | — |
| Bash interpreter evolution (fix mvdan/sh or custom) | P1 | Bash hardening |
| Reduce Go command map shadowing | P1 | Interpreter evolution |
| Python/distutils build system | P1 | Interpreter evolution |
| Top 20 eclass compatibility | P1 | Bash hardening |

**Key architectural decision pending:** Either fix `mvdan.cc/sh` upstream (contribute PRs for needed bash features) or write a custom Go bash interpreter optimized for ebuild/eclass semantics. Additionally, the interpreter backend will be **configurable** — users who prefer real `/bin/bash` can enable it via settings for full compatibility.

### Wave 3: Validation & Community

| Task | Priority | Blocked By |
|------|----------|------------|
| @world 90%+ build success | P2 | Python build system, Interpreter evolution |
| PMS compliance honest baseline | P2 | Audit bug fixes |
| Community testing preparation | P2 | @world validation |

### Known Bugs (from audit)

| Bug | Location | Status |
|-----|----------|--------|
| `=*` glob operator overly permissive | `internal/pkg/atom.go:748` | Tracked |
| Phase defaults routing | `internal/ebuild/phases_impl.go` | Tracked |
| Hardcoded `--libdir=/usr/lib64` | `internal/ebuild/phases_impl.go` | Tracked |
| Dead code in compat | `internal/compat/portage.go` | Tracked |

See [PMS_COMPLIANCE.md](docs/PMS_COMPLIANCE.md) for the full compliance matrix.

---

## Release History

### v0.9.4 — Bash Interpreter Hardening (2026-02-09)

- **stripFunctionBodies** — removes phase functions before metadata extraction
- **3-layer .sig file filtering** — eval → regex fallback → manifest filter
- **`.tar.lz` unpack support** — xz/plzip/lzip, matching Portage unpacker.eclass
- **Eclass stdout isolation**, BASH_VERSINFO emulation
- **ver_cut/ver_rs** default to PV, econf ECONF_SOURCE, S variable resolution
- **33 new tests**, golangci-lint clean (0 issues)

### v0.9.3 — Install Helper Path Resolution (2026-02-08)

- Install helpers resolve relative paths against `$S`
- Unpack phase uses `$A` from Manifest
- Non-standard archive name support

### v0.9.2 — Emerge Filtering Fix (2026-01-19)

- Emerge respects installed packages (VarDB filtering)
- `--deep`, `--with-bdeps`, `--emptytree`, `--vardb` flags

### v0.9.1 — Enterprise CLI & Mirror Fallback (2026-01-19)

- Enterprise CLI help formatter, shell completion (bash/zsh/fish)
- Man page generation (`grpm doc man`)
- "Did you mean?" command suggestions
- Mirror fallback (GENTOO_MIRRORS → SRC_URI)

### v0.9.0 — Enterprise Tool Check (2026-01-19)

- Portage-compatible tool handling via BDEPEND
- `--check-tools` opt-in flag
- Collision detection fixes, VarDB persistence

### v0.8.0-v0.8.4 — Configuration Management (2026-01-17-18)

- Dynamic make.conf parsing with variable expansion
- repos.conf with Portage fallback chain
- package.use atom specificity, package.mask, KEYWORDS filtering
- Package sets (@world, @system, @selected) in all commands
- SRC_URI evaluation with eclass support (MetadataEvaluator)
- UX improvements (emerge --info, fuzzy search, USE flag display)

### v0.7.0-v0.7.11 — Portage Compatibility & Security (2026-01-13-17)

- Portage-style colored logging
- Path traversal prevention (CVE-level fix)
- Docker layer caching (`--onlydeps`)
- Alternative root installation (`--root`)
- E2E integration tests
- mirror:// expansion, rsync fixes

### v0.6.0 — Infrastructure & Quality (2026-01-12)

- `grpm fetch` — automatic source downloading with mirror failover
- `grpm analyze` — repository coverage analysis
- `grpm tools` — external tool detection
- 21 eclass integration tests

### v0.5.0-v0.5.2 — Language Ecosystems & Dynamic Loading (2026-01-10-11)

- Dynamic eclass loading via mvdan.cc/sh
- Python eclasses (distutils-r1, python-r1, python-single-r1, python-any-r1)
- Rust (cargo.eclass), Go (go-module.eclass)
- Package sets, multilib, REQUIRED_USE solver

### v0.4.0 — Build Systems (2026-01-08)

- CMake and Meson build system support
- toolchain-funcs, flag-o-matic eclasses
- SQLite-backed metadata cache

### Earlier Releases

- **v0.3.0** — PMS compliance (EAPI 0-8, ver_* commands, mvdan.cc/sh)
- **v0.2.x** — Parser improvements, version comparison
- **v0.1.x** — Foundation, module architecture, initial release

---

## Current Capabilities

### What Works (~98% Tree Coverage)

| Category | Features |
|----------|----------|
| **Core** | SAT solver, version comparison, slot/subslot, USE flags |
| **Configuration** | make.conf, repos.conf, package.use/mask/keywords — full Portage compatibility |
| **Build Systems** | Autotools (full), CMake (partial), Meson (partial) |
| **Languages** | Python, Rust, Go — basic support via eclasses |
| **Multilib** | 32-bit/64-bit ABI management |
| **Binary Packages** | GPKG (.gpkg.tar), TBZ2 (.tbz2) read/write |
| **Sync** | rsync, Git with GPG verification |
| **Eclass Loading** | Dynamic via mvdan.cc/sh (~160 Go helpers) |

### Known Limitations

| Limitation | Impact | Planned Resolution |
|------------|--------|-------------------|
| mvdan.cc/sh interpreter | ~10% bash features unsupported | Fix upstream or custom interpreter (v0.10.0) |
| ~160 Go command map entries shadow eclass functions | Correctness concern for custom overlays | Reduce shadowing (v0.10.0) |
| PMS compliance ~60% (simple) / ~51% (weighted) | Complex eclasses may fail | Interpreter evolution + audit fixes (v0.10.0) |
| @world build success ~20% | Only autotools packages pass | Interpreter evolution + build system improvements |
| Daemon scaffolding | Functional but not production-hardened | Production hardening (v0.11.0) |

---

## v1.0.0 Release Criteria

| Criterion | Target | Current |
|-----------|--------|---------|
| @world build success | 90%+ | ~20% |
| PMS compliance | 80%+ for common packages | ~60% |
| Tree coverage | 98%+ | 98.2% |
| Critical bugs | 0 | 6 tracked |
| API stability | Frozen | Evolving |
| Community validation | Gentoo developer sign-off | Audit completed |
| Documentation | Complete user/dev guides | In progress |

---

## Quality Targets

| Metric | Target | Current |
|--------|--------|---------|
| Test functions | Comprehensive | 1,971 passing |
| Lint issues | 0 | 0 |
| Test coverage | 70%+ overall, 90%+ domain | Achieved |
| Tree coverage | 98%+ packages | 98.2% |

---

## How to Contribute

1. **Try GRPM** on real Gentoo and report issues
2. **Test eclasses** — report what works and what doesn't
3. Submit feature requests via [GitHub Issues](https://github.com/grpmsoft/grpm/issues)
4. Contribute code following [CONTRIBUTING.md](CONTRIBUTING.md)
5. Help with documentation and testing

---

## Resources

- **Repository:** https://github.com/grpmsoft/grpm
- **Issues:** https://github.com/grpmsoft/grpm/issues
- **Discussions:** https://github.com/grpmsoft/grpm/discussions
- **PMS Compliance:** [docs/PMS_COMPLIANCE.md](docs/PMS_COMPLIANCE.md)
- **Architecture:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)

---

*This roadmap evolves based on community feedback and project needs.*
*Last updated: 2026-02-09 (post-audit roadmap restructuring)*
