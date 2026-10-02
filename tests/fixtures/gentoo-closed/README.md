# Gentoo Closed Test Fixture

A small, **transitively closed** package fixture for GRPM integration testing.
Every package's dependencies can be satisfied by other packages in this fixture,
so `Resolve()` returns `status=SAT` with `postPass=0`.

**Created:** 2026-10-02
**Packages:** 7 entries across 6 categories
**Closure:** Complete (all transitive deps present)

## Design

Real Gentoo packages have deeply transitive BDEPEND chains through the entire
build toolchain (gcc, binutils, perl, python, autotools, meson, etc.). A
transitively closed fixture from the real tree would require 400+ packages
even for a simple library like libassuan.

This fixture uses **real md5-cache format** with realistic metadata (EAPI,
KEYWORDS, SLOT, IUSE, etc.) but omits BDEPEND entries to keep the closure
small. DEPEND/RDEPEND relationships are preserved and form real dependency
chains that the SAT resolver must handle correctly.

All USE-conditional dependencies (nls?, verify-sig?, etc.) evaluate to
inactive with default IUSE settings (no profile, no make.conf), so they
do not expand the closure.

## Dependency Graph

```
dev-libs/libassuan-2.5.7
  DEPEND:  >=dev-libs/libgpg-error-1.33
  RDEPEND: >=dev-libs/libgpg-error-1.33

dev-libs/npth-1.8
  DEPEND:  >=dev-libs/libgpg-error-1.17
  RDEPEND: >=dev-libs/libgpg-error-1.17

dev-libs/libgpg-error-1.51
  (leaf: all deps USE-conditional, defaults to inactive)

virtual/pkgconfig-3
  RDEPEND: dev-util/pkgconf

dev-util/pkgconf-2.5.1
  (leaf: RDEPEND is blocker only, skipped by parser)

sys-libs/zlib-1.3.1-r1
  (leaf: DEPEND/RDEPEND are blockers only, skipped by parser)

app-misc/hello-2.12.2
  (leaf: no dependencies)
```

## Closure Proof

| Root Package | Transitive Deps | All Present? |
|-------------|-----------------|--------------|
| dev-libs/libassuan-2.5.7 | dev-libs/libgpg-error-1.51 | Yes |
| dev-libs/npth-1.8 | dev-libs/libgpg-error-1.51 | Yes |
| virtual/pkgconfig-3 | dev-util/pkgconf-2.5.1 | Yes |
| dev-libs/libgpg-error-1.51 | (none active) | Yes |
| dev-util/pkgconf-2.5.1 | (none active) | Yes |
| sys-libs/zlib-1.3.1-r1 | (none active) | Yes |
| app-misc/hello-2.12.2 | (none active) | Yes |

## Modifications from Real Tree

- **BDEPEND stripped**: All unconditional BDEPEND entries removed to avoid
  pulling in the entire Gentoo build toolchain (autotools, meson, gcc, etc.)
- **KEYWORDS simplified**: Reduced to common architectures (amd64, arm, arm64,
  ppc, ppc64, x86) without `~` prefix for stable testing
- **Version numbers**: Based on real versions in the Gentoo tree (2026-10-02)
  except libgpg-error-1.51 which is a synthetic version to avoid multilib
  complexity present in real 1.59+

## Ebuild Stubs

Ebuild files are empty (0 bytes). PortageRepository requires them for directory
scanning (`ReadDir`) but reads actual metadata from `metadata/md5-cache/`.

## How to Regenerate

If the fixture needs updating:
1. Check which DEPEND/RDEPEND are unconditional for each package
2. Ensure all referenced packages exist in the fixture
3. Verify USE-conditional deps are inactive with default IUSE
4. Run `go test -run TestClosedFixture ./tests/integration/` to validate
