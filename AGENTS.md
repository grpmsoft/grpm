# AGENTS.md

Instructions for AI coding agents working with this codebase.

## Overview

GRPM (Go Resource Package Manager) is a next-generation package manager for Gentoo Linux. It reimplements Portage with SAT-based dependency resolution, transactional updates, and full Portage ecosystem compatibility.

## Development Commands

```bash
# Build
make build                          # Creates bin/grpm
make proto                          # Regenerates api/gen/*.pb.go

# Test (required before every commit)
gofmt -w internal/
go test -v -race ./...
golangci-lint run --timeout=5m      # Requires v2.8.0+

# Run
./bin/grpm daemon                   # Daemon mode (gRPC server)
./bin/grpm status                   # Check daemon status
./bin/grpm resolve --mock sys-libs/zlib
```

## Architecture

### Core Components

| Component | Location | Purpose |
|-----------|----------|---------|
| Domain Models | `internal/pkg/` | Package, Version, Slot, Constraint |
| SAT Solver | `internal/solver/` | Dependency resolution via gophersat |
| Repository | `internal/repo/` | Package metadata abstraction |
| Daemon | `internal/daemon/` | gRPC server, job queue |
| CLI | `internal/cli/` | Command-line interface |
| Ebuild | `internal/ebuild/` | Ebuild parsing and execution |

### Data Flow

```
CLI Request → gRPC Client → Daemon → Job Queue → Worker
                                         ↓
                              Solver → Repository → Portage Tree
```

### Design Patterns

- **DDD with Rich Domain Model**: Entities contain behavior, not just data
- **Repository Pattern**: Abstract data access behind interfaces
- **Specification Pattern**: Composable query criteria
- **Value Objects**: Immutable types (Version, Slot, Constraint)

## Code Conventions

### Go Style

```go
// Constructors validate invariants
func NewPackage(name, version string) (*Package, error) {
    if name == "" {
        return nil, errors.New("package name required")
    }
    return &Package{Name: name, Version: Version(version)}, nil
}

// Methods on domain entities
func (p *Package) Satisfies(c Constraint) bool { ... }

// Error wrapping with context
return fmt.Errorf("resolving %s: %w", pkg, err)
```

### Testing

```go
// Table-driven tests
func TestVersion_Compare(t *testing.T) {
    tests := []struct {
        name     string
        a, b     string
        expected int
    }{
        {"equal", "1.0", "1.0", 0},
        {"less", "1.0", "2.0", -1},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CompareVersions(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("got %d, want %d", got, tt.expected)
            }
        })
    }
}
```

## Key Files

| Path | Description |
|------|-------------|
| `cmd/grpm/main.go` | Entry point, mode detection |
| `api/proto/grpm.proto` | gRPC service definitions |
| `internal/pkg/package.go` | Core Package entity |
| `internal/solver/resolver.go` | Dependency resolution logic |
| `internal/daemon/daemon.go` | Daemon lifecycle |
| `.goreleaser.yml` | Release configuration |

## Boundaries

### Always

- Format with `gofmt` before committing
- Run tests with `-race` flag
- Wrap errors with `%w` verb
- Use US English spelling (canceled, not cancelled)
- Add godoc comments to exported symbols

### Ask First

- Changes to `api/proto/*.proto`
- New external dependencies
- Architectural changes to domain models
- Modifications to CI/CD workflows

### Never

- Edit `api/gen/*` (auto-generated)
- Commit credentials or secrets
- Use `panic()` for recoverable errors
- Add platform-specific code (Linux only)
- Push without passing lint and tests

## Testing Strategy

| Type | Command | Coverage Target |
|------|---------|-----------------|
| Unit | `go test ./internal/...` | 70% overall |
| Domain | `go test ./internal/pkg/...` | 90% |
| Race | `go test -race ./...` | Required in CI |
| Integration | `go test ./tests/integration/...` | Real Portage tree |

Use `t.Skip()` for tests requiring real Gentoo environment.

## Common Issues

**Lint failures**: Ensure golangci-lint v2.8.0+ with `.golangci.yml` v2 format.

**Proto changes**: Run `make proto` after modifying `.proto` files.

**Test failures on CI**: CI runs on Ubuntu. Use `t.Skip()` for Gentoo-specific tests.

**ldconfig errors in tests**: Expected on non-Gentoo systems. Tests handle gracefully.

## Tech Stack

- Go 1.25
- gRPC + Protocol Buffers
- gophersat (SAT solver)
- Cobra (CLI)
- golangci-lint v2.8.0+
