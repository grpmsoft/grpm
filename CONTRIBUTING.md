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
├── pkg/          # Domain layer (entities, value objects)
├── solver/       # Dependency resolution (SAT solver)
├── repo/         # Repository abstraction
├── daemon/       # gRPC daemon
├── cli/          # CLI commands
├── install/      # Package installation
├── ebuild/       # Ebuild execution
└── state/        # System state management
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

- EAPI 8 support
- CMake/Meson build systems
- Eclass implementations
- Test coverage improvements
- Documentation

## Getting Help

- **Issues**: [github.com/grpmsoft/grpm/issues](https://github.com/grpmsoft/grpm/issues)
- **Discussions**: [github.com/grpmsoft/grpm/discussions](https://github.com/grpmsoft/grpm/discussions)
- **Security**: See [SECURITY.md](SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the [Apache-2.0](LICENSE) license.
