# Package Manager Specification (PMS) Reference

> **Source:** [Gentoo PMS](https://projects.gentoo.org/pms/)
> **Version:** EAPI 8 (2024)
> **License:** [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/)

This documentation is a Markdown conversion of the official Gentoo Package Manager Specification (PMS) for use with GRPM development and reference.

---

## Table of Contents

### Core Specification

| Chapter | Title | Description |
|---------|-------|-------------|
| [1](chapter1-introduction.md) | Introduction | Overview of PMS, scope, and terminology |
| [2](chapter2-eapis.md) | EAPIs | EAPI definitions, versions, and features |
| [3](chapter3-names-versions.md) | Names and Versions | Package naming, version syntax, and comparison |
| [4](chapter4-tree-layout.md) | Tree Layout | Repository structure, ebuild layout, metadata |
| [5](chapter5-profiles.md) | Profiles | Profile system, inheritance, and configuration |

### Ebuild Specification

| Chapter | Title | Description |
|---------|-------|-------------|
| [7](chapter7-variables.md) | Ebuild-defined Variables | EAPI, SRC_URI, DEPEND, RDEPEND, SLOT, etc. |
| [8](chapter8-dependencies.md) | Dependencies | Dependency syntax, operators, USE conditionals |
| [9](chapter9-phases.md) | Ebuild-defined Functions | Phase functions (src_*, pkg_*) and execution order |
| [10](chapter10-eclasses.md) | Eclasses | Eclass system, inherit, EXPORT_FUNCTIONS |
| [11](chapter11-environment.md) | The Ebuild Environment | Environment variables, sandbox, paths |
| [12](chapter12-commands.md) | Available Commands | Helper functions (do*, die, econf, emake, etc.) |

---

## GRPM Implementation Status

### Summary

| Category | Implemented | Total | Coverage |
|----------|-------------|-------|----------|
| EAPIs (Ch. 2) | EAPI 0-8 | EAPI 0-8 | 100% |
| Ebuild Variables (Ch. 7) | 7 | 7 | 100% |
| Dependencies (Ch. 8) | 12 | 12 | 100% |
| Phase Functions (Ch. 9) | 14 | 14 | 100% |
| Eclasses (Ch. 10) | 2 | 2 | 100% |
| Environment (Ch. 11) | 3 | 3 | 100% |
| Commands (Ch. 12) | 30+ | 30+ | 100% |

### Key Features

| Feature | GRPM Version | PMS Section |
|---------|--------------|-------------|
| Atom Parser | v0.3.0-001 | 8.3 |
| REQUIRED_USE | v0.3.0-002d | 7.3.4 |
| SRC_URI Arrow Syntax | v0.3.0-002e | 7.3.2 |
| IDEPEND | v0.3.0-002d | 8.1 |
| use_enable/use_with | v0.3.0-002b | 12.3.12 |
| dosym -r | v0.3.0-002c | 12.3.9 |
| dostrip | v0.3.0-002c | 12.3.9 |
| einstalldocs | v0.3.0-002c | 12.3.9 |

### Not Yet Implemented

| Feature | Blocked By | Target |
|---------|------------|--------|
| Full Bash compatibility | GoSh library | v0.3.0-003 |
| CMake/Meson build systems | - | v0.4.0 |

---

## Quick Reference

### Version Comparison

```
1.0 < 1.0a < 1.0_alpha < 1.0_beta < 1.0_pre < 1.0_rc < 1.0 < 1.0_p
```

### Dependency Operators

| Operator | Meaning | Example |
|----------|---------|---------|
| `>=` | Greater or equal | `>=dev-lang/go-1.21` |
| `>` | Greater than | `>sys-libs/glibc-2.17` |
| `<=` | Less or equal | `<=app-misc/foo-2.0` |
| `<` | Less than | `<net-misc/bar-1.0` |
| `=` | Exact version | `=dev-libs/openssl-3.0.0` |
| `~` | Revision match | `~app-misc/hello-1.0` |
| `=*` | Glob match | `=dev-lang/python-3.11*` |
| `!` | Weak blocker | `!app-misc/conflicting` |
| `!!` | Strong blocker | `!!app-misc/dangerous` |

### Slot Dependencies

| Syntax | Meaning |
|--------|---------|
| `:slot` | Specific slot |
| `:slot/subslot` | Specific slot and subslot |
| `:*` | Any slot |
| `:=` | Slot operator (rebuild on slot change) |

### Phase Execution Order

```
pkg_pretend → pkg_setup → src_unpack → src_prepare → src_configure →
src_compile → src_test → src_install → pkg_preinst → pkg_postinst
```

---

## Links

- [Official PMS](https://projects.gentoo.org/pms/)
- [PMS Git Repository](https://gitweb.gentoo.org/proj/pms.git/)
- [EAPI Cheat Sheet](https://devmanual.gentoo.org/ebuild-writing/eapi/)
- [Gentoo Devmanual](https://devmanual.gentoo.org/)
- [GRPM Documentation](../INDEX.md)

---

*This documentation is derived from the official Gentoo PMS under CC BY-SA 4.0 license.*
