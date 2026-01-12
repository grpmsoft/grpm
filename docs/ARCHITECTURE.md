# GRPM Architecture

## System Overview

```mermaid
flowchart TB
    subgraph Client["Client Layer"]
        CLI[CLI Commands]
        GRPC_CLIENT[gRPC Client]
    end

    subgraph Daemon["Daemon Layer"]
        GRPC_SERVER[gRPC Server<br/>unix:///var/run/grpm.sock]
        REST_API[REST API<br/>:8080]
        JOB_QUEUE[Job Queue]
        WORKERS[Worker Pool]
    end

    subgraph Application["Application Layer"]
        PKG_SERVICE[Package Service]
    end

    subgraph Domain["Domain Layer"]
        SOLVER[SAT Solver<br/>gophersat]
        PKG[Package Model<br/>Version, Slot, Constraint]
        ATOM[Atom Parser<br/>PMS Section 8.3]
        DEP_SERVICE[Dependency Service]
    end

    subgraph Infrastructure["Infrastructure Layer"]
        REPO[Repository<br/>Portage/Mock]
        SYNC[Sync Module<br/>rsync/git]
        FETCH[Fetch Module<br/>planned v0.6.0]
        INSTALL[Install Engine<br/>merge/unmerge]
        BINPKG[Binary Packages<br/>GPKG/TBZ2]
        STATE[System State<br/>VarDB]
        SNAPSHOT[Snapshot Manager<br/>Btrfs/ZFS]
        CACHE[Metadata Cache<br/>SQLite]
    end

    subgraph External["External Layer"]
        PORTAGE_TREE[Portage Tree<br/>/var/db/repos/gentoo]
        MIRRORS[Distfile Mirrors]
        BINHOST[Binary Host]
        FS[File System]
    end

    CLI --> GRPC_CLIENT
    CLI -.->|standalone| PKG_SERVICE
    GRPC_CLIENT --> GRPC_SERVER
    REST_API --> JOB_QUEUE
    GRPC_SERVER --> JOB_QUEUE
    JOB_QUEUE --> WORKERS
    WORKERS --> PKG_SERVICE

    PKG_SERVICE --> SOLVER
    PKG_SERVICE --> DEP_SERVICE
    SOLVER --> PKG
    SOLVER --> ATOM
    DEP_SERVICE --> PKG
    DEP_SERVICE --> ATOM
    ATOM --> PKG

    SOLVER --> REPO
    REPO --> CACHE
    REPO --> PORTAGE_TREE

    PKG_SERVICE --> SYNC
    SYNC --> PORTAGE_TREE

    PKG_SERVICE --> FETCH
    FETCH --> MIRRORS

    PKG_SERVICE --> INSTALL
    INSTALL --> STATE
    INSTALL --> SNAPSHOT
    INSTALL --> FS

    PKG_SERVICE --> BINPKG
    BINPKG --> BINHOST
```

## Component Descriptions

### Client Layer
- **CLI Commands**: User-facing commands (resolve, install, emerge, sync, etc.)
- **gRPC Client**: Connects to daemon when available, falls back to standalone

### Daemon Layer
- **gRPC Server**: Unix socket server for CLI communication
- **REST API**: HTTP API for external integrations
- **Job Queue**: Prioritized queue with conflict detection
- **Worker Pool**: Parallel job execution

### Application Layer
- **Package Service**: Use case orchestration, coordinates domain and infrastructure

### Domain Layer
- **SAT Solver**: Boolean satisfiability for dependency resolution (gophersat)
- **Package Model**: Core entities (Package, Version, Slot, Constraint)
- **Atom Parser**: PMS-compliant package atom parser (Section 8.3)
- **Dependency Service**: Domain logic for dependency handling

### Infrastructure Layer
- **Repository**: Package metadata abstraction (Portage tree, mock)
- **Sync Module**: Repository synchronization (native rsync, git with GPG)
- **Fetch Module**: Distfile downloading *(planned for v0.6.0)*
- **Install Engine**: Package merge/unmerge operations
- **Binary Packages**: GPKG and TBZ2 format support
- **System State**: Installed package database (VarDB)
- **Snapshot Manager**: Btrfs/ZFS transactional rollback
- **Metadata Cache**: SQLite-backed fast lookups
- **Eclass System**: Dynamic eclass loading via mvdan.cc/sh

### External Systems
- **Portage Tree**: Gentoo package repository
- **Distfile Mirrors**: Source tarball mirrors
- **Binary Host**: Pre-built binary packages
- **File System**: Target system root

---

## Dynamic Eclass Loading (v0.5.2+)

```
Eclass source (bash)
        |
        v
mvdan.cc/sh interpreter
        |
        v
execHandler intercepts commands
        |
        +------------------+------------------+
        |                                     |
        v                                     v
+-------------------+               +-------------------+
| Known commands    |               | Unknown commands  |
| (tc-getCC, use,   |               | (pkg-config,      |
| cmake_*, meson_*) |               | install, ninja)   |
+-------------------+               +-------------------+
        |                                     |
        v                                     v
+-------------------+               +-------------------+
| Go implementations|               | Real shell        |
| (100+ helpers)    |               | execution         |
+-------------------+               +-------------------+
```

### Key Components

- **eclass.Cache** (`internal/eclass/cache.go`): Scans repository eclass/ directories, tracks mtime for invalidation
- **eclass.Executor** (`internal/eclass/executor.go`): Executes eclasses via mvdan.cc/sh bash interpreter
- **eclass.HybridLoader** (`internal/eclass/loader.go`): Dynamic loading with Go fallbacks
- **ebuild.Interpreter** (`internal/ebuild/interpreter.go`): 100+ helper functions mapped to Go implementations

### Command Interception

When an eclass calls a command:
1. **Known commands** (tc-getCC, use, cmake_src_configure, etc.) → Go implementations
2. **Unknown commands** (pkg-config, cmake, ninja, etc.) → Real shell pass-through

This enables most eclasses to work without Go reimplementation.
