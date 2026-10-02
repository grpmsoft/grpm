# Gentoo Test Fixtures

Real package metadata from Gentoo WSL2 tree.

**Tree date:** 2026-01-13
**Extracted:** 2026-10-02
**Packages:** 138 entries across 8 packages

**This fixture is a subset, NOT transitively closed.** Most packages resolve
to UNSAT because their transitive deps are not in the fixture. UNSAT times
are not representative of real resolve performance — they measure time to
trivial prohibit, not cost of full dependency closure.

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

## Verified on Full Tree (2026-10-02, independent review)

Tested on WSL2 Gentoo rsync snapshot (`/var/db/repos/gentoo`, with md5-cache).
No profile or make.conf — resolver used `NewResolver` without keyword filtering:

| Package | Status | Packages | Notes |
|---------|--------|----------|-------|
| dev-libs/libassuan | OK | 2 | libassuan + libgpg-error |
| app-crypt/gpgme | OK | 4 | gpgme + libassuan + libgpg-error + gpg |
| dev-db/sqlite | OK | 7 | sqlite + readline + ncurses + zlib + ... |
| dev-libs/libxml2 | OK | 7 | libxml2 + zlib + libiconv + ... |
| dev-libs/libgpg-error | OK | 5 | WSL2 verified |
| dev-libs/libassuan | OK | 6 | WSL2 verified |

`postPass=0` on all resolved packages.

Full-tree resolve with md5-cache not yet measured on this fixture (fixture is
not transitively closed). Perf numbers require either a closed fixture subset
or a CI harness with shallow clone of `gentoo/gentoo`.

## Stub Ebuilds

Ebuild files are empty (0 bytes). PortageRepository requires them for directory
scanning (`ReadDir`) but reads actual metadata from `metadata/md5-cache/`.
This is by design — md5-cache is Portage's canonical metadata source.
