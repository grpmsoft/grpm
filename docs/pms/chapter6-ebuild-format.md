# Chapter 6: Ebuild file format

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#ebuild-file-format)
> **EAPI:** All versions (with EAPI-specific features noted)
> **Last Updated:** 2026-01-09

## Overview

The ebuild file format is in its basic form a subset of the format of a bash script. This chapter defines the requirements for ebuild file structure, encoding, shell compatibility, and execution constraints.

## Basic Requirements

### Shell Interpreter

**BASH-VERSION** The interpreter is assumed to be GNU bash, version as listed in Table 6.1, or any later version. If possible, the package manager should set the shell's compatibility level to the exact version specified. It must ensure that any such compatibility settings (e.g. the `BASH_COMPAT` variable) are not exported to external programs.

### File Creation Mask (umask)

The file creation mask (`umask`) is set to `022` in the shell execution environment. It is **not** saved between phase functions but always reset to this initial value.

### Failglob Option

**FAILGLOB** For EAPIs listed in Table 6.1, the `failglob` option of bash is set in the global scope of ebuilds. If set, failed pattern matches during filename expansion result in an error when the ebuild is being sourced.

### Name Reference Variables

Name reference variables (introduced in bash version 4.3) must not be used, except in local scope.

## File Encoding and Structure

### Encoding

The file encoding must be **UTF-8** with **Unix-style newlines**.

### Sourcing Constraints

When sourced, the ebuild must define certain variables and functions (see chapters [7](chapter7-variables.md) and [9](chapter9-phases.md) for specific information), and must **not**:

- Call any external programs
- Write anything to standard output or standard error
- Modify the state of the system in any way

These constraints ensure that ebuilds can be safely sourced for metadata extraction without side effects.

## Table 6.1: Bash version and options

| EAPI | Bash version | `failglob` in global scope? |
|------|--------------|----------------------------|
| 0, 1, 2, 3, 4, 5 | 3.2 | No |
| 6, 7 | 4.2 | Yes |
| 8 | 5.0 | Yes |
| 9 | 5.3 | Yes |

## Implementation Notes

### For GRPM Implementation

1. **Bash Version Compatibility:**
   - GRPM's `internal/ebuild/executor.go` must enforce the correct bash version for the EAPI being processed
   - Set `BASH_COMPAT` environment variable when appropriate
   - Ensure compatibility settings are not exported to external programs

2. **Failglob Handling:**
   - For EAPI 6+ ebuilds, set `shopt -s failglob` in the global scope during sourcing
   - Disable `failglob` during phase function execution (it only applies to global scope)
   - Example: `set -o failglob` before sourcing, `set +o failglob` before executing phases

3. **Umask Management:**
   - Set `umask 022` before sourcing the ebuild
   - Reset `umask 022` before each phase function execution
   - Do not preserve umask between phases

4. **Metadata Extraction:**
   - Source the ebuild in a clean, isolated environment
   - Capture variable definitions without executing external commands
   - Monitor for violations (stdout/stderr output, external program calls)

5. **UTF-8 Validation:**
   - Validate that ebuild files are valid UTF-8 before sourcing
   - Reject ebuilds with invalid encoding or non-Unix line endings

6. **Name Reference Variables:**
   - While bash 4.3+ supports `declare -n`, ebuilds should not use them in global scope
   - Only permit in local scope (inside functions)
   - Consider warning or error if detected in global scope

## Cross-References

- **Chapter 7:** [Ebuild-defined variables](chapter7-variables.md) - Variables that must/may be defined
- **Chapter 9:** [Ebuild phases](chapter9-phases.md) - Phase functions and execution order
- **Chapter 11:** [Package Manager-defined variables](chapter11-variables.md) - Variables set by the PM

## References

- [PMS Chapter 6: Ebuild file format](https://projects.gentoo.org/pms/latest/pms.html#ebuild-file-format)
- [GNU Bash Manual](https://www.gnu.org/software/bash/manual/)
- [EAPI Cheat Sheet](https://devmanual.gentoo.org/eapi-cheat-sheet/index.html)
