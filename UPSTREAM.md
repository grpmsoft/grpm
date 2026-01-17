# UPSTREAM - Portage Reference Tracking

This file tracks the upstream Portage package manager and PMS specification
that GRPM implementation is based on.

## Primary References

### Package Manager Specification (PMS)
URL:        https://projects.gentoo.org/pms/
Version:    PMS 8 (EAPI 8)
Date:       2024-05-01 (last major update)
Status:     Fully supported (EAPI 0-8)

### Portage (Reference Implementation)
Repository: https://github.com/gentoo/portage
Branch:     master
Commit:     [`f41f766f3e`](https://github.com/gentoo/portage/commit/f41f766f3e1d0c60c34e4cb6a9e6b06c67d18fe1)
Date:       2026-01-14
Local Copy: D:\projects\grpmsoft\reference\portage

### Gentoo Repository (Ebuilds)
Repository: https://github.com/gentoo/gentoo
Sync:       Via `grpm sync` or `emerge --sync`
Note:       Eclasses loaded dynamically, no tracking needed

---

## Upstream Sync History

### Latest Sync: 2026-01-17

**Commit**: [`f41f766f3e`](https://github.com/gentoo/portage/commit/f41f766f3e1d0c60c34e4cb6a9e6b06c67d18fe1)
**Local Ref**: `f41f766f3e` (synced)
**Commits Synced**: 11 commits

#### Notable Changes Reviewed

| Commit | Description | Impact |
|--------|-------------|--------|
| `0783d82` | **EAPI 9 enabled** | NEW EAPI! |
| `d466b96` | Scheduler: implicit jobserver slot support | Low |
| `a3f0843` | Scheduler: fix error handling typo | None |
| `faaf697` | gpkg: checksum_helper improvements | Low |

#### EAPI 9 Features (Bug #965922)
- Profile EAPI defaults to top-level
- `use.stable` and `package.use.stable` support
- PMS special profile variables handling
- `FEATURES=export-pms-vars`

#### Tasks Created
- **v0.9.0-001**: Add EAPI 9 support (HIGH)
- Review: https://wiki.gentoo.org/wiki/Future_EAPI/EAPI_9_tentative_features

---

### Previous Sync: 2026-01-07

**Commit**: [`af1ba8bc34`](https://github.com/gentoo/portage/commit/af1ba8bc34153e10b667482036774faac630dcc9)
**Version**: portage-3.0.75

#### Features Referenced
- action_info() implementation (v0.8.2)
- package.mask loading logic (v0.8.1)
- KEYWORDS filtering (v0.8.1)

---

## Implementation Notes

GRPM is a **Go reimplementation**, not a Python port or wrapper.

### Approach
- PMS specification as primary reference for behavior
- Portage source code consulted for edge cases
- Go idioms preferred over Python patterns
- Independent test suite with real Gentoo repository validation

### Key Differences from Portage
1. **Language**: Go vs Python
2. **Bash Execution**: mvdan.cc/sh interpreter vs subprocess
3. **Dependency Resolution**: SAT solver vs backtracking
4. **Architecture**: Single binary vs Python package
5. **Daemon Mode**: gRPC/REST API vs D-Bus

### Feature Parity Status

| Feature | Portage | GRPM | Notes |
|---------|---------|------|-------|
| Dependency Resolution | ✅ | ✅ | SAT-based |
| Package Masks | ✅ | ✅ | v0.8.1 |
| KEYWORDS Filtering | ✅ | ✅ | v0.8.1 |
| USE Flags | ✅ | ✅ | Full support |
| Slots/Subslots | ✅ | ✅ | Full support |
| Binary Packages | ✅ | ✅ | GPKG + TBZ2 |
| emerge --info | ✅ | ✅ | v0.8.2 |
| emerge --pretend | ✅ | ✅ | With USE flags |
| Sandboxing | ✅ | ❌ | Planned v1.0.0 |
| preserved-rebuild | ✅ | Partial | Basic support |
| news system | ✅ | ❌ | Not planned |
| elog system | ✅ | Partial | Basic support |

---

## Files to Monitor in Portage

When syncing with upstream, focus on these files:

```
lib/_emerge/actions.py          # emerge commands (info, pretend, etc.)
lib/_emerge/resolver/            # Dependency resolution
lib/_emerge/Package.py          # Package model
lib/portage/dbapi/              # Database API
lib/portage/package/ebuild/     # Ebuild execution
lib/portage/dep/                # Dependency parsing
lib/portage/versions.py         # Version comparison
lib/portage/eapi.py             # EAPI features
lib/portage/repository/         # Repository handling
lib/portage/sync/               # Sync modules
```

### Recently Referenced (v0.8.x)

| File | GRPM Feature | Version |
|------|--------------|---------|
| `lib/_emerge/actions.py` lines 1837-2230 | emerge --info | v0.8.2 |
| `lib/_emerge/UseFlagDisplay.py` | USE flag display | v0.8.2 |
| `lib/portage/versions.py` vercmp() | Version sorting | v0.8.2 |
| `lib/portage/dbapi/_similar_name_search.py` | Fuzzy matching | v0.8.2 |
| `lib/portage/package/ebuild/config.py` | make.conf | v0.8.0 |
| `lib/portage/repository/config.py` | repos.conf | v0.8.0 |

---

## Sync Workflow

When syncing with upstream changes:

1. **Check Portage releases**: https://github.com/gentoo/portage/releases
2. **Pull latest**: `git pull origin master` (in reference/portage)
3. **Review commits**: Focus on _emerge/ and dep/ changes
4. **Check PMS updates**: https://projects.gentoo.org/pms/
5. **Update this file**: Document what was synced
6. **Create tasks**: Add to docs/dev/kanban/backlog/

### Automated Check (TODO)

```bash
# Compare local reference with upstream
cd reference/portage
git fetch origin
git log --oneline HEAD..origin/master
```

---

## Quality Validation

### Gentoo Repository Coverage
- Packages tested: 31,396
- Supported: 30,843 (98.2%)
- Unsupported: 553 (1.8%)

### Commands Verified Against Portage
| Command | GRPM | Portage | Match |
|---------|------|---------|-------|
| resolve | ✅ | ✅ | Same versions selected |
| info | ✅ | ✅ | Same output format |
| search | ✅ | ✅ | Same results |
| emerge --pretend | ✅ | ✅ | Same package order |

---

## Community Feedback Integration

### Gentoo Forums (2026-01-17)
- grknight: Consider hybrid bash execution
- NeddySeagoon: Bug-for-bug compatibility concerns
- pietinger: Track gentoo-dev@ for eclass changes

**Response**: GRPM follows PMS specification, not Portage implementation details.
Eclasses are loaded dynamically from repository, not hardcoded.

---

Last Updated: 2026-01-17
Maintainer: GRPM Team
