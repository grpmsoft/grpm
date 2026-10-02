# Gentoo Test Fixtures

Real package metadata from Gentoo WSL2 tree.

**Tree date:** 2026-01-13
**Extracted:** 2026-10-02
**Packages:** 138 entries across 8 packages

## Packages

| Package | Versions | Purpose |
|---------|----------|---------|
| dev-lang/python | 49 | Multi-slot (3.11, 3.12, 3.13) |
| sys-devel/gcc | 46 | Multi-slot (13, 14) |
| dev-libs/openssl | 16 | Slot transitions, blockers |
| sys-libs/glibc | 16 | Deep deps, blockers |
| dev-lang/lua | 4 | Multi-slot (5.1, 5.3, 5.4) |
| sys-libs/zlib | 4 | Simple dep chain |
| app-misc/hello | 2 | Minimal test |
| virtual/pkgconfig | 1 | Virtual package |

## Performance Baseline (2026-10-02, main after PR #78)

Measured on WSL2 Gentoo, warm cache:

| Package | Versions | Time |
|---------|----------|------|
| app-misc/hello | 2 | 113ms |
| sys-libs/zlib | 4 | 104ms |
| dev-libs/openssl | 16 | 140ms |
| sys-devel/gcc | 46 | 220ms |
| dev-lang/python | 49 | 239ms |

All under 250ms. Cold start adds ~3s (Go binary first load).
