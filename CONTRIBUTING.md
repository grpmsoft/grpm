# Contributing to GRPM

Thank you for your interest in contributing to GRPM! This guide will help you get started.

## Quick Start

```bash
# Clone and setup
git clone https://github.com/grpmsoft/grpm.git
cd grpm
go mod download

# Development cycle
make fmt      # Format code
make test     # Run tests
make lint     # Run linter
make build    # Build binary
```

## Requirements

- **Go 1.25+**
- **golangci-lint v2.5.0+**
- Linux recommended (GRPM targets Gentoo)

## Code Standards

### Formatting

All code must be formatted with `gofmt`. Unformatted code will fail CI.

```bash
make fmt        # Format all code
make fmt-check  # Verify formatting
```

### Testing

- Minimum **70%** overall coverage
- Minimum **90%** for business logic (`internal/pkg/`, `internal/solver/`)
- Use table-driven tests

```bash
make test           # Run all tests
make test-coverage  # Generate coverage report
```

### Linting

```bash
make lint  # Run golangci-lint
```

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`

**Examples:**
```bash
git commit -m "feat: add slot collision detection"
git commit -m "fix: resolve version parsing for suffixes"
git commit -m "docs: update CLI reference"
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make changes and ensure all checks pass:
   ```bash
   make dev  # Runs: fmt, lint, test, build
   ```
4. Commit with meaningful messages
5. Push and create a Pull Request

### PR Checklist

- [ ] Code formatted (`make fmt`)
- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] New features have tests
- [ ] Documentation updated if needed

## Architecture

GRPM follows **Domain-Driven Design (DDD)** with a layered architecture:

```
internal/
├── pkg/          # Domain layer (entities, value objects, version comparison)
├── solver/       # SAT-based dependency resolution
├── repo/         # Repository abstraction (Portage tree, ebuild parsing, SRC_URI)
├── ebuild/       # Ebuild execution (~160 helpers, 12 eclass modules, phase runner)
├── eclass/       # Dynamic eclass loading and caching
├── distfile/     # Distfile resolution with USE-conditional filtering
├── fetch/        # Download engine (mirrors, checksums, Manifest parsing)
├── config/       # Configuration (make.conf, repos.conf with variable expansion)
├── profile/      # Profile system (inheritance, masks, USE defaults)
├── mask/         # Package masking (package.mask, KEYWORDS filtering)
├── sets/         # Package sets (@world, @system, @selected)
├── install/      # Package installation (merge, unmerge, collision detection)
├── binpkg/       # Binary packages (GPKG, TBZ2 read/write, signing)
├── sync/         # Repository sync (rsync, Git with GPG)
├── state/        # System state (VarDB, Btrfs/ZFS snapshots)
├── cli/          # CLI commands (Cobra-based)
├── daemon/       # gRPC + REST daemon
├── application/  # Application layer (use case orchestration, DTOs)
└── compat/       # Portage compatibility layer
```

### Key Principles

- **Rich Domain Model** — business logic in domain objects
- **Interface-based design** — depend on abstractions
- **Repository pattern** — data access abstraction
- **SOLID principles** — throughout the codebase

## What to Contribute

### Good First Issues

Look for issues labeled `good first issue` or `help wanted`.

### Areas Needing Help

- **Bash interpreter evolution** — fix mvdan/sh upstream or help design a custom Go interpreter for ebuilds
- Complex eclass support (kernel, LLVM, Java)
- CMake/Meson build system edge cases
- Real-world package testing and bug reports on Gentoo
- Audit bug fixes (see [PMS_COMPLIANCE.md Known Bugs](docs/PMS_COMPLIANCE.md#known-bugs))
- Test coverage improvements
- Documentation

## Useful References

- [Architecture](docs/ARCHITECTURE.md) — system diagram and package descriptions
- [PMS Compliance](docs/PMS_COMPLIANCE.md) — implementation status per PMS specification
- [CLI Reference](docs/CLI_REFERENCE.md) — complete command documentation
- [Roadmap](ROADMAP.md) — current phase and planned work

## Getting Help

- **Issues**: [github.com/grpmsoft/grpm/issues](https://github.com/grpmsoft/grpm/issues)
- **Discussions**: [github.com/grpmsoft/grpm/discussions](https://github.com/grpmsoft/grpm/discussions)
- **Security**: See [SECURITY.md](SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the [Apache-2.0](LICENSE) license.
