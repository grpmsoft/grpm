# GRPM Architecture

> Go Resource Package Manager — a next-generation Gentoo package manager.
>
> This document describes the system architecture as validated against source code (2026-02-09).

---

## System Overview

```mermaid
flowchart TB
    subgraph Client["Client Layer"]
        CLI[CLI Commands<br/>resolve, emerge, sync, search, info]
        GRPC_CLIENT[gRPC Client]
    end

    subgraph Daemon["Daemon Layer"]
        GRPC_SERVER[gRPC Server<br/>unix:///var/run/grpm.sock]
        REST_API[REST API<br/>unix:///var/run/grpm-rest.sock]
        JOB_QUEUE[Job Queue<br/>conflict detection]
        WORKERS[Worker Pool]
    end

    subgraph Application["Application Layer"]
        PKG_SERVICE[Package Service]
    end

    subgraph Domain["Domain Layer"]
        RESOLVER[Dependency Resolver]
        SAT[SAT Solver<br/>gophersat]
        PKG[Package Model<br/>Version, Slot, Constraint]
        ATOM[Atom Parser<br/>PMS Section 8.3]
        DEP_SERVICE[Dependency Service]
    end

    subgraph Build["Build Execution Layer"]
        EXECUTOR[Ebuild Executor<br/>phase dispatch]
        INTERPRETER[Bash Interpreter<br/>mvdan.cc/sh]
        COMMAND_MAP[Command Map<br/>~160 Go helpers]
        METADATA[Metadata Evaluator<br/>Go + native bash]
        ECLASS[Eclass System<br/>dynamic loading]
        DISTFILE[Distfile Service<br/>SRC_URI resolution]
        SANDBOX[Sandbox<br/>namespace isolation]
        PRIVILEGE[Privilege Manager<br/>portage user]
    end

    subgraph Infrastructure["Infrastructure Layer"]
        REPO[Repository<br/>Portage / Mock / Cached]
        SYNC_MOD[Sync Module<br/>rsync / git+GPG]
        FETCH[Fetch Module<br/>mirror failover]
        INSTALL[Install Engine<br/>merge / unmerge]
        BINPKG[Binary Packages<br/>GPKG / TBZ2]
        STATE[System State<br/>VarDB]
        SNAPSHOT[Snapshot Manager<br/>Btrfs / ZFS]
        CACHE[Metadata Cache<br/>SQLite + md5-cache]
    end

    subgraph Config["Configuration Layer"]
        MAKE_CONF[Config Loader<br/>make.conf]
        PROFILE[Profile System<br/>USE defaults, masks]
        SETS[Package Sets<br/>@world, @system]
        MASK[Mask Manager<br/>package.mask, keywords]
        VIRTUAL[Virtual Providers<br/>virtual/* resolution]
    end

    subgraph External["External Systems"]
        PORTAGE_TREE[Portage Tree<br/>/var/db/repos/gentoo]
        MIRRORS[Distfile Mirrors]
        BINHOST[Binary Host]
        FS[File System<br/>target root]
        BASH[/bin/bash<br/>native fallback]
    end

    CLI --> GRPC_CLIENT
    CLI -.->|standalone| PKG_SERVICE
    GRPC_CLIENT --> GRPC_SERVER
    GRPC_SERVER --> JOB_QUEUE
    REST_API --> JOB_QUEUE
    JOB_QUEUE --> WORKERS
    WORKERS --> PKG_SERVICE

    PKG_SERVICE --> RESOLVER
    PKG_SERVICE --> EXECUTOR
    RESOLVER --> SAT
    RESOLVER --> REPO
    SAT --> PKG
    RESOLVER --> ATOM
    RESOLVER --> DEP_SERVICE
    DEP_SERVICE --> PKG

    EXECUTOR --> INTERPRETER
    EXECUTOR --> DISTFILE
    EXECUTOR --> SANDBOX
    EXECUTOR --> PRIVILEGE
    INTERPRETER --> COMMAND_MAP
    INTERPRETER --> ECLASS
    METADATA --> INTERPRETER
    METADATA --> BASH
    ECLASS --> REPO

    REPO --> CACHE
    REPO --> PORTAGE_TREE
    DISTFILE --> FETCH
    FETCH --> MIRRORS

    PKG_SERVICE --> SYNC_MOD
    SYNC_MOD --> PORTAGE_TREE

    EXECUTOR --> INSTALL
    INSTALL --> STATE
    INSTALL --> SNAPSHOT
    INSTALL --> FS

    PKG_SERVICE --> BINPKG
    BINPKG --> BINHOST

    RESOLVER --> MASK
    RESOLVER --> PROFILE
    PKG_SERVICE --> SETS
    SETS --> PROFILE
    MAKE_CONF --> PROFILE
    VIRTUAL --> REPO
```

---

## Layer Descriptions

### Client Layer (`cmd/grpm/`, `internal/cli/`)

Single binary with mode detection: CLI commands or daemon mode.

| Component | Description |
|-----------|-------------|
| **CLI Commands** | Cobra-based: `resolve`, `emerge`, `install`, `remove`, `search`, `info`, `sync`, `status`, `analyze`, `tools` |
| **gRPC Client** | Connects to daemon via Unix socket; auto-falls back to standalone mode |

### Daemon Layer (`internal/daemon/`)

Background service for concurrent package operations.

| Component | Description |
|-----------|-------------|
| **gRPC Server** | Unix socket (`/var/run/grpm.sock`) for CLI communication |
| **REST API** | Unix socket (`/var/run/grpm-rest.sock`); TCP bind optional (disabled by default) |
| **Job Queue** | Prioritized queue with per-package conflict detection |
| **Worker Pool** | Parallel job execution with goroutine workers |

> **Status**: Daemon scaffolding is functional. Health endpoint, job queue, and socket communication work. Full production hardening is planned for a future release.

### Application Layer (`internal/application/`)

Use case orchestration between domain and infrastructure. Thin layer — delegates to domain services.

### Domain Layer

Core business logic. Inner layers never depend on outer layers.

| Component | Package | Description |
|-----------|---------|-------------|
| **Dependency Resolver** | `internal/solver/` | Orchestrates dependency collection, mask filtering, SAT encoding, and solution extraction |
| **SAT Solver** | `internal/solver/` | `gophersat` adapter — encodes packages as Boolean variables, constraints as CNF clauses, `exactlyOne()` per slot |
| **Package Model** | `internal/pkg/` | Core entities: `Package`, `Version`, `Slot`, `Constraint`, `Atom`, `UseFlag` |
| **Atom Parser** | `internal/pkg/` | PMS Section 8.3 compliant parser with version, slot, USE dep, and blocker support |
| **Dependency Service** | `internal/pkg/` | Domain service for dependency logic and constraint satisfaction |

### Build Execution Layer (`internal/ebuild/`, `internal/eclass/`)

**The largest subsystem (~41K lines).** Handles the entire ebuild lifecycle from metadata extraction through phase execution to file installation.

| Component | File(s) | Description |
|-----------|---------|-------------|
| **Ebuild Executor** | `executor.go` | Phase dispatch engine: fetch → unpack → prepare → configure → compile → install → merge |
| **Bash Interpreter** | `interpreter.go` | mvdan.cc/sh wrapper with `execHandler` command interception |
| **Command Map** | `interpreter.go:240-577` | ~160 Go helper functions mapped to bash command names |
| **Phase Implementations** | `phases_impl.go` | Autotools defaults (configure/make/make install) |
| **Default Phase Functions** | `helpers_default.go` | PMS-correct default implementations (eapply_user, einstalldocs) wired via `default` command |
| **Metadata Evaluator** | `metadata.go` | Dual-mode: `EvalModeGo` (mvdan.cc/sh, cross-platform) or `EvalModeNativeBash` (`/bin/bash`, full compat) |
| **Eclass System** | `eclass/` | Dynamic loading via `Cache` (mtime tracking) + `Executor` (bash eval) + `HybridLoader` (Go fallbacks) |
| **Eclass Bridge** | `eclass_bridge.go` | Connects `internal/eclass/` dynamic loader with `internal/ebuild/` interpreter |
| **Distfile Service** | `internal/distfile/` | SRC_URI evaluation, manifest validation, download coordination |
| **Sandbox** | `internal/sandbox/` | Linux namespace isolation (`NamespaceSandbox`); noop on other platforms |
| **Privilege Manager** | `internal/privilege/` | Portage user creation, privilege drop for phases and fetch |

#### Helper Function Organization

| File | Category | Examples |
|------|----------|---------|
| `helpers_msg.go` | Messaging | `einfo`, `ewarn`, `eerror`, `die`, `ebegin`/`eend` |
| `helpers_use.go` | USE flags | `use`, `usev`, `useq`, `use_with`, `use_enable` |
| `helpers_install.go` | Installation | `dobin`, `dosbin`, `dolib`, `doins`, `doman`, `dodoc` |
| `helpers_fs.go` | Filesystem | `dosym`, `keepdir`, `dodir`, `fperms`, `fowners` |
| `helpers_build.go` | Build | `econf`, `emake`, `einstall` |
| `helpers_patch.go` | Patching | `eapply`, `eapply_user`, `epatch` |
| `helpers_unpack.go` | Unpacking | `unpack` (tar, gz, bz2, xz, lz, zstd, zip) |
| `helpers_version.go` | Version | `ver_cut`, `ver_rs`, `ver_test` |
| `helpers_toolchain.go` | Toolchain | `tc-getCC`, `tc-getCXX`, `tc-getLD`, `tc-is-cross-compiler` |
| `helpers_default.go` | PMS defaults | `default_src_prepare`, `default_src_configure`, `default_src_install` |
| `helpers_banned.go` | Banned commands | Blocks commands banned by EAPI (e.g., `dohtml` in EAPI 7+) |

#### Go Eclass Modules (fallback implementations)

| File | Eclass | Functions |
|------|--------|-----------|
| `eclass_cmake.go` | cmake | `cmake_src_configure`, `cmake_src_compile`, `cmake_src_install`, `cmake_src_test` |
| `eclass_meson.go` | meson | `meson_src_configure`, `meson_src_compile`, `meson_src_install`, `meson_src_test` |
| `eclass_cargo.go` | cargo | `cargo_src_unpack`, `cargo_src_compile`, `cargo_src_install`, `cargo_src_test` |
| `eclass_go_module.go` | go-module | `go-module_src_unpack`, `go-module_src_compile`, `go-module_src_install` |
| `eclass_distutils.go` | distutils-r1 | `distutils-r1_src_prepare`, `distutils-r1_src_configure`, `distutils-r1_src_compile`, `distutils-r1_src_install` |
| `eclass_python_r1.go` | python-r1 | `python_foreach_impl`, `python_setup` |
| `eclass_python_single.go` | python-single-r1 | `python-single-r1_pkg_setup` |
| `eclass_python_any.go` | python-any-r1 | `python-any-r1_pkg_setup`, `python_check_deps` |
| `eclass_python_utils.go` | python-utils-r1 | `python_get_sitedir`, `python_domodule`, `python_optimize` |
| `eclass_helpers.go` | multiple | `eshopts_push`/`pop`, `multilib`, `flag-o-matic`, `linux-info`, `toolchain-funcs` |
| `eclass_multilib.go` | multilib-minimal | `multilib_foreach_abi`, `multilib_src_configure` |
| `eclass_flag_o_matic.go` | flag-o-matic | `append-flags`, `replace-flags`, `strip-flags`, `filter-flags` |

### Infrastructure Layer

| Component | Package | Description |
|-----------|---------|-------------|
| **Repository** | `internal/repo/` | `Repository` interface with `PortageRepository` (real tree), `MockRepository` (testing), `CachedPortageRepository` (SQLite-backed) |
| **Sync Module** | `internal/sync/` | Native Go rsync (`internal/rsync/`) and git sync with GPG signature verification; auto-selection strategy |
| **Fetch Module** | `internal/fetch/` | Distfile downloading with Gentoo mirror rotation, checksum verification (SHA256/SHA512/BLAKE2B), third-party URI support |
| **Install Engine** | `internal/install/` | File merge with SHA256 content hashing, collision detection, unmerge with tracked file removal, pre/post hooks |
| **Binary Packages** | `internal/binpkg/` | GPKG (.gpkg.tar) and TBZ2 (.tbz2) format readers/writers, XPAK metadata parser, binhost client, package signing (GPG/SSH/RSA) |
| **System State** | `internal/state/` | VarDB — installed package database (thread-safe), query system for installed packages |
| **Snapshot Manager** | `internal/state/` | Btrfs (`btrfs subvolume snapshot`) and ZFS (`zfs snapshot`) transactional rollback |
| **Metadata Cache** | `internal/cache/` | SQLite-backed cache for package metadata; also reads Portage's native `metadata/md5-cache/` for pre-computed eclass-merged values |
| **Analyze Module** | `internal/analyze/` | Repository coverage analysis — EAPI distribution, eclass usage, category breakdown |
| **Tools Module** | `internal/tools/` | External tool detection (gcc, make, cmake, ninja, pkg-config, etc.) and validation |

### Configuration Layer

| Component | Package | Description |
|-----------|---------|-------------|
| **Config Loader** | `internal/config/` | Parses `make.conf` — CFLAGS, USE, FEATURES, ACCEPT_KEYWORDS, mirrors, etc. |
| **Profile System** | `internal/profile/` | Loads cascading profiles from `/etc/portage/make.profile` — USE defaults, package.use, parent stacking |
| **Package Sets** | `internal/sets/` | `@world` (= `@selected` + `@system`), `@preserved-rebuild`, file-based sets with `SetExpander` |
| **Mask Manager** | `internal/mask/` | `package.mask`, `package.unmask`, `package.accept_keywords` — determines package visibility |
| **Virtual Providers** | `internal/virtual/` | Maps `virtual/*` packages to preferred real providers with configurable defaults |

---

## Build Execution Pipeline

The `emerge` command drives this pipeline. Each phase runs within the ebuild executor:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Ebuild Executor                            │
│                                                                 │
│  1. METADATA EXTRACTION                                         │
│     ┌──────────────────┐    ┌──────────────────┐                │
│     │ EvalModeGo       │ OR │ EvalModeNativeBash│                │
│     │ (mvdan.cc/sh)    │    │ (/bin/bash)       │                │
│     └──────────────────┘    └──────────────────┘                │
│         ↓ SRC_URI, DEPEND, RDEPEND, SLOT, IUSE, ...            │
│                                                                 │
│  2. DEPENDENCY RESOLUTION                                       │
│     Resolver → SAT Solver → ordered install plan                │
│                                                                 │
│  3. PHASE EXECUTION (per package)                               │
│     ┌──────────┬──────────┬──────────┬──────────┬──────────┐    │
│     │ pkg_setup│  src_    │  src_    │  src_    │  src_    │    │
│     │          │ unpack   │ prepare  │configure │ compile  │    │
│     └──────────┴──────────┴──────────┴──────────┴──────────┘    │
│     ┌──────────┬──────────┬──────────┬──────────┐               │
│     │  src_    │  src_    │  pkg_    │  pkg_    │               │
│     │ install  │  test    │ preinst  │ postinst │               │
│     └──────────┴──────────┴──────────┴──────────┘               │
│                                                                 │
│  4. COMMAND DISPATCH (within each phase)                        │
│     Phase bash code → mvdan.cc/sh → execHandler                 │
│         ├── Command Map (~160 Go helpers) → Go implementation   │
│         ├── Dynamically loaded eclass function → bash eval      │
│         └── Unknown command → real shell pass-through           │
│                                                                 │
│  5. FILE MERGE                                                  │
│     ${D} image → target filesystem (collision detect, tracking) │
│                                                                 │
│  6. VarDB UPDATE                                                │
│     Record installed package, contents, environment             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Phase Dispatch Logic

When a phase function is called:

1. Check if the ebuild defines a custom phase function → execute via interpreter
2. Check if an inherited eclass provides the phase → execute eclass version
3. Fall back to built-in Go defaults (`phases_impl.go` for autotools workflow)
4. The `default` command explicitly calls PMS-correct defaults from `helpers_default.go`

### Metadata Evaluation Modes

| Mode | Backend | Use Case | Compatibility |
|------|---------|----------|---------------|
| `EvalModeGo` | mvdan.cc/sh (in-process) | Default, cross-platform | ~90% of bash constructs |
| `EvalModeNativeBash` | `/bin/bash` (subprocess) | Linux production | Full bash compatibility |

The Go mode is used for development on non-Linux platforms. Native bash mode provides full compatibility on Gentoo systems where `/bin/bash` is available.

---

## Dependency Resolution Pipeline

```
User request ("emerge app-misc/hello")
    │
    ├── Set Expansion (@world → atom list)
    │
    ├── Atom Parsing (category/name-version:slot[use])
    │
    ├── Repository Lookup
    │   ├── md5-cache (fast, pre-computed)
    │   └── Ebuild parsing (fallback)
    │
    ├── Mask & Keyword Filtering
    │   ├── package.mask / package.unmask
    │   └── ACCEPT_KEYWORDS matching
    │
    ├── Recursive Dependency Collection
    │   ├── DEPEND (build-time)
    │   ├── RDEPEND (run-time)
    │   ├── BDEPEND (CBUILD, EAPI 7+)
    │   └── PDEPEND (post-merge)
    │
    ├── SAT Encoding
    │   ├── One variable per package-version
    │   ├── exactlyOne() per slot
    │   ├── Dependency constraints as CNF clauses
    │   └── USE flag conditional dependencies
    │
    ├── SAT Solving (gophersat)
    │
    ├── Solution Optimization
    │   ├── Prefer installed versions
    │   ├── Minimize total packages
    │   └── Respect slot preferences
    │
    └── Ordered Install Plan (topological sort)
```

---

## Dynamic Eclass Loading

```
┌──────────────────────────────────────────────────────────────┐
│ Eclass source file (bash)                                    │
│ e.g., /var/db/repos/gentoo/eclass/cmake.eclass              │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│ eclass.Cache                                                 │
│ Scans eclass/ directories, tracks mtime for invalidation     │
│ File: internal/eclass/cache.go                               │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────┐
│ eclass.HybridLoader                                          │
│ Dynamic loading with Go fallback strategy                    │
│ File: internal/eclass/integration.go                         │
└──────────────────────┬───────────────────────────────────────┘
                       │
              ┌────────┴────────┐
              ▼                 ▼
┌──────────────────┐  ┌──────────────────────┐
│ eclass.Executor  │  │ Go Fallback          │
│ mvdan.cc/sh eval │  │ (12 eclass modules)  │
│ internal/eclass/ │  │ internal/ebuild/     │
│ executor.go      │  │ eclass_*.go          │
└────────┬─────────┘  └──────────────────────┘
         │
         ▼
┌──────────────────────────────────────────────────────────────┐
│ ebuild.Interpreter execHandler                               │
│ File: internal/ebuild/interpreter.go                         │
│                                                              │
│ Dispatch priority:                                           │
│  1. Command Map (~160 Go helpers)                            │
│     PMS core: die, use, dobin, econf, emake, unpack, ...    │
│     Eclass funcs: cmake_*, meson_*, python_*, cargo_*        │
│     Unix tools: sed, cp, mv, grep, find, ...                 │
│                                                              │
│  2. Unknown commands → real shell pass-through               │
└──────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | File | Description |
|-----------|------|-------------|
| **eclass.Cache** | `internal/eclass/cache.go` | Scans repository `eclass/` directories, tracks mtime for invalidation |
| **eclass.Executor** | `internal/eclass/executor.go` | Executes eclasses via mvdan.cc/sh bash interpreter |
| **eclass.HybridLoader** | `internal/eclass/integration.go` | Dynamic loading with Go fallback; options: `WithGoFallback()`, `WithVerbose()` |
| **ebuild.Interpreter** | `internal/ebuild/interpreter.go` | ~160 helper functions mapped to Go implementations via `buildCommandMap()` |
| **Eclass Bridge** | `internal/ebuild/eclass_bridge.go` | Connects eclass dynamic loader with ebuild interpreter |

---

## Package Sizes by Code Volume

| Package | Lines | Description |
|---------|------:|-------------|
| `ebuild` | 41,312 | Build execution engine (largest subsystem) |
| `cli` | 11,218 | Command-line interface |
| `binpkg` | 9,166 | Binary package formats |
| `repo` | 8,951 | Repository abstraction |
| `solver` | 8,266 | Dependency resolution |
| `pkg` | 8,066 | Domain model |
| `install` | 5,153 | File merge/unmerge |
| `daemon` | 4,981 | Background service |
| `fetch` | 4,497 | Distfile downloading |
| `state` | 4,376 | System state / VarDB |
| `config` | 4,118 | Configuration management |
| `eclass` | 3,158 | Eclass loading system |
| `tools` | 3,150 | External tool detection |
| `privilege` | 2,300 | Privilege management |
| `sets` | 2,156 | Package set expansion |
| | **~123K** | **Total internal/** |

---

## Known Limitations

### mvdan.cc/sh Interpreter

The Go-based bash interpreter (mvdan.cc/sh) handles ~90% of bash constructs correctly. The remaining ~10% causes failures in complex real-world eclasses:

| Limitation | Severity | Impact |
|------------|----------|--------|
| `declare -f` / `declare -p` not supported | High | Eclass introspection fails |
| `${var@a}` parameter attributes | High | Causes panic |
| `read -a` (read into array) | Medium | Array-based eclasses fail |
| Process substitution `>()` | High | Complex pipe patterns fail |
| Extended globbing `!(pattern)` | Medium | Pattern-matching eclasses fail |
| Brace expansion in variable names | Critical | Variable construction fails |

**Mitigation**: `EvalModeNativeBash` uses real `/bin/bash` for full compatibility. For v0.10.0-005, two options are under evaluation: (1) fix mvdan/sh upstream, or (2) write a custom Go interpreter optimized for ebuilds. Additionally, the interpreter backend will be **configurable** — users who prefer real `/bin/bash` can enable it via settings.

### Command Map Shadowing

The ~160 Go helper functions in the command map intercept commands before dynamically loaded eclass functions. If an overlay provides a custom eclass with a function name matching a Go helper (e.g., `cmake_src_configure`), the Go version takes priority. This is a correctness concern for overlays. Tracked in v0.10.0-010.

### PMS Compliance

As of v0.9.4 (validated by 4-agent audit, 2026-02-09):

| Scenario | Compliance |
|----------|-----------|
| Simple autotools (configure/make/make install) | ~80% |
| Packages using `default` command | ~70% |
| Packages with complex eclasses | ~40% |
| **Weighted average across Portage tree** | **~51%** |

The gap is primarily due to interpreter limitations, not missing domain logic. The correct PMS implementations exist in `helpers_default.go` but are not always dispatched correctly. See `docs/dev/research/audit-validated-2026-02-09.md` for the full audit report.

---

## Design Principles

- **DDD with Rich Domain Model**: Domain entities contain business logic, not just data
- **Layered Architecture**: Inner layers never depend on outer layers
- **Repository Pattern**: Abstract storage behind interfaces; swap Portage/Mock/Cached
- **SAT-based Resolution**: Dependency solving via Boolean satisfiability (provably correct)
- **Transactional Safety**: Filesystem snapshots before destructive operations
- **Fail-fast**: Prefer errors over silent fallbacks (aligned with Portage behavior)

---

## Roadmap: Target Architecture (v0.10.0+)

```
GRPM Orchestration (Go)
    │
    ├── Metadata parsing (mvdan.cc/sh) — fast, in-process
    ├── Dependency resolution (SAT solver) — Go native
    ├── Phase execution (configurable backend):
    │   ├── Default → Go interpreter (mvdan.cc/sh or custom)
    │   └── Optional → real /bin/bash (enabled via settings)
    ├── Helper functions → standalone Go binaries in PATH
    ├── File merge & VarDB → Go native (transactional)
    └── Configuration → Go native
```

The interpreter backend is configurable: by default GRPM uses its Go interpreter (fast, cross-platform), but users can enable real `/bin/bash` via settings for full compatibility. The Go interpreter itself will be evolved — either by fixing mvdan/sh upstream or by writing a custom implementation optimized for ebuild semantics.

---

*Validated: 2026-02-09 against source code*
*Source: 4-agent community audit + manual code verification*
