# GRPM Roadmap

> **Last Updated:** 2025-10-10
> **Current Version:** v0.1.0
> **Next Milestone:** v0.1.0-rc.1 - Daemon Architecture (Release Candidate)

---

## 🎯 Vision

Transform GRPM into a production-ready package manager for Gentoo Linux with:
- Advanced SAT-based dependency resolution
- Transactional updates with snapshot support
- Full binary package support (read + build)
- Complete Portage compatibility
- Modern Go architecture with excellent performance

---

## 📅 Release Timeline

| Version | Status | Target Date | Key Features |
|---------|--------|-------------|--------------|
| v0.1.0  | ✅ Released | Q1 2025 | Architecture foundation, SAT solver |
| v0.2.0  | ✅ Released | Q2 2025 | Portage compatibility layer |
| v0.3.0  | ✅ Released | Q3 2025 | USE flag resolution |
| v0.4.0  | ✅ Released | Q4 2025 | Dependency graph solver |
| v0.5.0  | ✅ Released | Oct 2025 | System integration (5 phases) |
| v0.6.0  | ✅ Released | Oct 2025 | Binary packages (core) |
| v0.7.0  | ✅ Released | Oct 2025 | Binary packages (building) |
| v0.8.0  | ✅ Released | Oct 2025 | Release automation (GoReleaser) |
| **v0.1.0**  | **🚀 In Progress** | **Q1 2026** | **Daemon + API/CLI + Repository Sync** ⚡ |
| v1.0.0  | 🎯 Target | Q2 2026 | Production-ready release |

---

## ✅ Completed Milestones

### v0.6.0 - Binary Packages (Core) ✅ October 2025

**Goal:** Complete binary package reading and binhost support

**Delivered:**
- ✅ GPKG format reader (.gpkg.tar, GLEP 78 standard)
- ✅ TBZ2 format reader (.tbz2, legacy format)
- ✅ XPAK metadata parser (big-endian binary format)
- ✅ Binhost repository (local directories + HTTP)
- ✅ Binary/source selection with 5 strategies:
  - `prefer-binary`: Try binary first, fall back to source
  - `prefer-source`: Try source first, fall back to binary
  - `binary-only`: Only use binary packages
  - `source-only`: Only use source packages
  - `auto`: Intelligent selection based on age/USE flags
- ✅ USE flag compatibility scoring
- ✅ Package age filtering and scoring

**Code Statistics:**
- 7 files created (~2,073 lines)
- 15+ tests + 3 benchmarks
- 100% test pass rate
- Zero linter issues

**Impact:** GRPM can now read and use binary packages from Calculate Linux and other Gentoo-based distributions!

---

### v0.7.0 - Binary Packages (Building) ✅ October 2025

**Goal:** Build binary packages from source installations and manage local binhosts

**Delivered:**
- ✅ Binary package builder (source → .gpkg.tar / .tbz2)
- ✅ GPKG format writer (.gpkg.tar, GLEP 78 standard)
- ✅ TBZ2 format writer (.tbz2, legacy format)
- ✅ Package signing (GPG/SSH/RSA signatures)
- ✅ Local binhost manager:
  - Create binhost directory structure
  - Generate Packages index file
  - Category directories and symlinks
  - Index compression (gz, bz2)
- ✅ Remote binhost uploader:
  - HTTP/HTTPS upload support
  - SSH/SFTP upload support
  - Authentication (SSH keys, username/password)
  - Progress reporting
- ✅ Build metadata collection:
  - Capture build environment (CFLAGS, CXXFLAGS, LDFLAGS)
  - Record USE flags and FEATURES
  - Store build logs and duration
  - System information tracking

**Code Statistics:**
- 1,533 lines of tests
- 29 tests + 6 benchmarks
- 100% test pass rate
- Zero linter issues

**Impact:** GRPM can now build and distribute binary packages, creating a complete binhost infrastructure!

---

### v0.8.0 - Release Automation ✅ October 2025

**Goal:** Automate binary releases with GoReleaser for multi-platform distribution

**Delivered:**
- ✅ GoReleaser configuration (.goreleaser.yml v2 syntax)
- ✅ Multi-platform binary releases:
  - Linux amd64 (primary target)
  - Linux arm64 (ARM servers)
  - Linux 386 (32-bit systems)
  - Linux arm/6 and arm/7 (Raspberry Pi, embedded)
- ✅ GitHub Actions workflow (.github/workflows/release.yml)
- ✅ Automated release pipeline:
  - Tag push triggers workflow
  - Full test suite execution
  - Multi-platform builds (~2-3 minutes)
  - GitHub Release creation
- ✅ Release artifacts:
  - Compressed archives (.tar.gz) per platform
  - SHA256 checksums (checksums.txt)
  - Auto-generated changelog
  - Pre-release flag support (beta/rc)
- ✅ Version injection via ldflags (Git commit, build date)

**Impact:** Fully automated release process from tag push to GitHub Release with multi-platform binaries. Released v0.1.0 through beta.5 using this pipeline!

---

### v0.5.0 - System Integration ✅ October 2025

**Goal:** Transform from resolver into a functional package manager

**Delivered (5 Phases):**

#### Phase 1: Profile System
- ✅ Profile loading and parsing (`/etc/portage/make.profile`)
- ✅ Parent profile inheritance resolution (recursive)
- ✅ make.defaults, use.mask, use.force parsing
- ✅ package.use, package.mask, packages support
- ✅ USE flag deduplication and merging
- 📊 13 tests + 2 benchmarks, 72.1% coverage

#### Phase 2: System State Tracking
- ✅ Package database with thread-safe operations (sync.RWMutex)
- ✅ VarDB compatibility - load/save to `/var/db/pkg` format
- ✅ File ownership index for O(1) file owner lookups
- ✅ Advanced query system (category, pattern, USE, size, time)
- ✅ CONTENTS parser (obj, dir, sym file types)
- 📊 30+ tests + 3 benchmarks, 39.3% coverage

#### Phase 3: Package Installation Engine
- ✅ Installer API - Install(), Uninstall(), Upgrade()
- ✅ File merging (regular files, symlinks, directories)
- ✅ SHA256 hashing for file integrity
- ✅ Collision detection (3 types: exists, owned by other, protected)
- ✅ Unmerger - safe removal with modified file backup
- ✅ Hook system (extensible pre/post install hooks)
- 📊 40+ tests, 22.3% coverage baseline

#### Phase 4: Ebuild Execution
- ✅ Ebuild executor with phase management
- ✅ Environment system (20+ Portage variables: P, PN, PV, D, S, ED, etc)
- ✅ 11 phase definitions (setup → install)
- ✅ Stub executor for testing
- 📊 8 tests, stub implementation

#### Phase 5: Configuration Management
- ✅ Configuration loader (make.conf, package.*)
- ✅ MakeConf parser (CFLAGS, MAKEOPTS, USE, FEATURES)
- ✅ Package configuration (package.use, package.mask, package.accept_keywords)
- ✅ Support for both file and directory-based configs
- 📊 10 tests, 100% passing

**Total Impact:** 4,000+ lines of production code

---

### v0.4.0 - Dependency Graph Solver ✅ Q4 2025

**Delivered:**
- ✅ Dependency graph structure with BFS/DFS traversal
- ✅ Circular dependency detection with stack tracking
- ✅ Slot and version conflict detection
- ✅ Solution optimization with 4 strategies
- ✅ Weighted scoring system
- 📊 50+ tests, 75%+ coverage

---

### v0.3.0 - USE Flag Resolution ✅ Q3 2025

**Delivered:**
- ✅ USE flag solver with global/package-specific flags
- ✅ USE flag condition evaluation
- ✅ Ebuild parser refactoring and optimization
- ✅ Repository caching (sync.Map)
- 📊 92.3% domain layer coverage

---

### v0.2.0 - Portage Compatibility Layer ✅ Q2 2025

**Delivered:**
- ✅ Advanced ebuild parser (RDEPEND, DEPEND, BDEPEND)
- ✅ Slot and subslot parsing (`:0/1.22`, `:=`)
- ✅ USE flag conditional dependencies (`ssl? ( ... )`)
- ✅ Package atom parsing (`>=sys-libs/zlib-1.2.13`)
- ✅ Blocker detection (`!` soft, `!!` hard)
- ✅ Version operators support

---

### v0.1.0 - Architecture Foundation ✅ Q1 2025

**Delivered:**
- ✅ DDD-inspired modular architecture
- ✅ Repository interface abstraction
- ✅ Domain model (Package, Constraint, Slot, Version)
- ✅ SAT solver integration (gophersat)
- ✅ Basic dependency graph traversal
- ✅ Cobra-based CLI structure

---

## 📋 Planned Milestones

### v1.0.0 - Production Release 🎯

**Target:** Q4 2026
**Priority:** 🚀 Major Milestone

**Goals:**
- Full Portage compatibility
- Production-ready quality
- Complete binary package support
- Security audit passed
- Performance benchmarks

**Requirements:**
- ✅ All core features complete (v0.1-v0.7)
- ✅ Binary package support (read + build)
- ✅ 80%+ test coverage overall
- ✅ Security audit completed
- ✅ Performance benchmarks vs emerge
- ✅ Full Calculate Linux compatibility
- ✅ End-user documentation
- ✅ Migration guide from Portage

**Success Criteria:**
- Can replace Portage for daily use
- Passes Gentoo compatibility test suite
- No critical bugs in 1 month beta period
- Community feedback addressed
- Professional documentation

---

### v1.1.0+ - Daemon Architecture 💡

**Target:** TBD (post-v1.0)
**Status:** Research phase

**Vision:**
- Background system monitoring
- Scheduled automatic updates
- Warm cache for instant CLI responses
- Job queue for parallel operations
- Web dashboard
- Notification system

**See:** [DAEMON_ARCHITECTURE.md](docs/DAEMON_ARCHITECTURE.md) for detailed design

---

## 🔍 Focus Areas

### Critical Path to v1.0
1. ✅ **v0.6.0** - Binary package reading ✅ DONE
2. ✅ **v0.7.0** - Binary package building ✅ DONE
3. ✅ **v0.8.0** - Release automation ✅ DONE
4. **v0.1.0** - Daemon + API/CLI + Repository Sync ⚡ IN PROGRESS
5. **v1.0.0** - Production release 🎯 NEXT

### Quality Targets
- **Test Coverage:** 80%+ overall (90%+ for business logic)
- **Performance:** Competitive with emerge (within 20%)
- **Documentation:** Complete API docs + user guides
- **Security:** Passed security audit
- **Stability:** Zero critical bugs for 1 month

### Platform Targets
- **Primary:** Gentoo Linux (amd64, arm64)
- **Secondary:** Calculate Linux (full compatibility)
- **Future:** Funtoo Linux (compatibility layer)

---

## 🤝 Community & Contribution

### How to Contribute
1. Check [PROJECT_STATUS.md](PROJECT_STATUS.md) for current work
2. Read [.claude/CLAUDE.md](.claude/CLAUDE.md) for development guidelines
3. Follow [.claude/PRE_COMMIT_RULES.md](.claude/PRE_COMMIT_RULES.md) workflow
4. Submit PRs with conventional commits

### Current Priorities
1. **v0.1.0 completion** (daemon + API/CLI + repository sync)
2. Critical bug fixes (ebuild parser, USE flags, OR-dependencies)
3. Git-based sync implementation (GPG signature verification)
4. Integration testing and optimization
5. Real-world testing on Gentoo systems

---

## 📊 Progress Metrics

### Overall Progress
- **Features Complete:** 8/10 major milestones (80%)
- **Test Coverage:** ~70% (target: 80%+)
- **Code Quality:** ✅ Zero linter issues
- **Documentation:** 📝 In progress

### Module Maturity
| Module | Status | Coverage | Quality |
|--------|--------|----------|---------|
| pkg (domain) | ✅ Stable | 92.3% | ⭐⭐⭐⭐⭐ |
| solver | ✅ Stable | 75%+ | ⭐⭐⭐⭐ |
| repo | ⚠️ Needs work | 39.9% | ⭐⭐⭐ |
| state | ✅ Stable | 39.3% | ⭐⭐⭐⭐ |
| install | ✅ Stable | 22.3% | ⭐⭐⭐⭐ |
| ebuild | ⚠️ Stub only | 0% | ⭐⭐ |
| profile | ✅ Stable | 72.1% | ⭐⭐⭐⭐ |
| config | ✅ Stable | 100% | ⭐⭐⭐⭐⭐ |
| binpkg | ✅ New | 100% | ⭐⭐⭐⭐⭐ |

---

## 🔗 Resources

- **Repository:** https://github.com/grpmsoft/grpm
- **Documentation:** [docs/INDEX.md](docs/INDEX.md)
- **Issues:** https://github.com/grpmsoft/grpm/issues
- **License:** Apache-2.0

---

*This roadmap is a living document and will be updated as the project evolves.*

**Next Update:** After v0.1.0 completion
