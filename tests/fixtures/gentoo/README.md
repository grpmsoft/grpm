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

## Performance Baseline (2026-10-02, after PR #78 + slot/glob fixes)

### md5-cache (fixture repo, Go test, Windows)

| Package | Versions | Time | Status |
|---------|----------|------|--------|
| app-misc/hello | 2 | 2ms | OK |
| sys-libs/zlib | 4 | 1ms | OK |
| dev-libs/openssl | 16 | 9ms | UNSAT (fixture subset) |
| sys-devel/gcc | 46 | 41ms | UNSAT (fixture subset) |
| dev-lang/python | 49 | 83ms | UNSAT (fixture subset) |

### WSL2 full tree (CLI, ebuild parsing path)

| Package | Time | Status |
|---------|------|--------|
| app-misc/hello | 84ms | OK (1 pkg) |
| dev-libs/libgpg-error | 5s | OK (5 pkgs) |
| dev-libs/libassuan | 2.5s | OK (6 pkgs) |

WSL times include ebuild interpreter overhead (~90ms/ebuild). md5-cache 30-100x faster.
Stub ebuilds are empty — PortageRepository reads from md5-cache when available.
