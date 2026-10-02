# Gentoo Closed Test Fixture

Transitively closed package fixture extracted from a real Gentoo rsync snapshot.
Every package's DEPEND/RDEPEND/BDEPEND can be satisfied within this fixture,
so `Resolve()` returns `status=SAT` with `postPass=0`.

**Source:** WSL2 Gentoo rsync snapshot
**Timestamp:** Fri, 02 Oct 2026 12:45:00 +0000
**Packages:** 16 names, 50 versions
**Closure:** DEPEND + RDEPEND + BDEPEND (real md5-cache, unmodified)

## Roots

| Root | Versions | Why |
|------|----------|-----|
| dev-libs/libassuan | 3 | Version constraints (`>=1.33`), multi-version SAT |
| dev-db/sqlite | 3 | Non-zero slot (`:3`), slot operators (`:=`), virtual deps, BDEPEND |

## Dependency Graph

```
dev-libs/libassuan (3 versions)
  DEPEND/RDEPEND: >=dev-libs/libgpg-error-1.33

dev-db/sqlite:3 (3 versions)
  BDEPEND: app-arch/unzip
  DEPEND/RDEPEND: virtual/zlib:=[...], readline?(), icu?(), tcl?()

virtual/zlib → sys-libs/zlib
virtual/pkgconfig → dev-util/pkgconf
app-arch/bzip2 → app-alternatives/bzip2
```

## All Packages

| Package | Versions | Role |
|---------|----------|------|
| dev-libs/libassuan | 3 | Root: version constraint chain |
| dev-libs/libgpg-error | 3 | Leaf of libassuan chain |
| dev-db/sqlite | 3 | Root: slot :3, virtual deps |
| sys-libs/zlib | 2 | Dep of sqlite via virtual/zlib |
| sys-libs/readline | 5 | USE-conditional dep of sqlite |
| sys-libs/ncurses | 3 | Dep of readline |
| virtual/zlib | 2 | Virtual → sys-libs/zlib |
| virtual/pkgconfig | 1 | Virtual → dev-util/pkgconf |
| dev-util/pkgconf | 7 | Provider of virtual/pkgconfig |
| app-arch/xz-utils | 3 | BDEPEND chain |
| app-arch/bzip2 | 2 | BDEPEND chain |
| app-arch/unzip | 2 | BDEPEND of sqlite |
| app-alternatives/bzip2 | 1 | app-alternatives provider |
| app-portage/elt-patches | 5 | BDEPEND chain |
| sys-apps/findutils | 2 | BDEPEND chain |
| sys-apps/gentoo-functions | 6 | BDEPEND chain |

## md5-cache Format

Files in `metadata/md5-cache/` are unmodified copies from the rsync snapshot.
They contain real DEPEND, RDEPEND, BDEPEND, SLOT, KEYWORDS, IUSE, EAPI, etc.
USE-conditional deps (e.g., `readline?`, `icu?`) evaluate to inactive with
default settings (no profile, no make.conf).

Ebuild files are empty stubs (0 bytes) — PortageRepository requires them
for directory scanning but reads metadata from md5-cache.

## Regeneration

```bash
# On WSL2 Gentoo (after emerge --sync):
bash /mnt/d/tmp/extract_fixture.sh
# Then copy /tmp/gentoo-closed/ to tests/fixtures/gentoo-closed/
```
