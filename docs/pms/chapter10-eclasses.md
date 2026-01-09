# Chapter 10: Eclasses

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#eclasses)
> **EAPI:** All versions (with EAPI-specific features noted)
> **Last Updated:** 2025-12-14 (PMS commit 43857e7)

## Overview

Eclasses serve to store common code that is used by more than one ebuild, which greatly aids maintainability and reduces the tree size. However, due to metadata cache issues, care must be taken in their use. In format they are similar to an ebuild, and indeed are sourced as part of any ebuild using them. The interpreter is therefore the same, and the same requirements for being parseable hold.

Eclasses must be located in the `eclass` directory in the top level of the repository—see section 4.6. Each eclass is a single file named `<name>.eclass`, where `<name>` is the name of this eclass, used by `inherit` and `EXPORT_FUNCTIONS` among other places. `<name>` must be a valid eclass name, as per section 3.1.6.

## 10.1 The inherit command

An ebuild wishing to make use of an eclass does so by using the `inherit` command in global scope. This will cause the eclass to be sourced as part of the ebuild—any function or variable definitions in the eclass will appear as part of the ebuild, with exceptions for certain metadata variables, as described below.

The `inherit` command takes one or more parameters, which must be the names of eclasses (excluding the `.eclass` suffix and the path). For each parameter, in order, the named eclass is sourced.

Eclasses may end up being sourced multiple times.

The `inherit` command must also ensure that:

- The `ECLASS` variable is set to the name of the current eclass, when sourcing that eclass.
- Once all inheriting has been done, the `INHERITED` metadata variable contains the name of every eclass used, separated by whitespace.

## 10.2 Eclass-defined metadata keys

**Feature:** `accumulate-vars` (all EAPIs for dependency variables, EAPI 8+ for PROPERTIES/RESTRICT)

The `IUSE`, `REQUIRED_USE`, `DEPEND`, `BDEPEND`, `RDEPEND`, `PDEPEND` and `IDEPEND` variables are handled specially when set by an eclass. They must be accumulated across eclasses, appending the value set by each eclass to the resulting value after the previous one is loaded. For EAPIs listed in table 10.1 as accumulating `PROPERTIES` and `RESTRICT`, the same is true for these variables. Then the eclass-defined value is appended to that defined by the ebuild. In the case of `RDEPEND`, this is done after the implicit `RDEPEND` rules in section 7.3.7 are applied.

### Table 10.1: EAPIs accumulating PROPERTIES and RESTRICT across eclasses

| EAPI | Accumulates PROPERTIES? | Accumulates RESTRICT? |
|------|-------------------------|-----------------------|
| 0, 1, 2, 3, 4, 5, 6, 7 | No | No |
| 8, 9 | Yes | Yes |

## 10.3 EXPORT_FUNCTIONS

There is one command available in the eclass environment that is neither available nor meaningful in ebuilds—`EXPORT_FUNCTIONS`. This can be used to alias ebuild phase functions from the eclass so that an ebuild inherits a default definition whilst retaining the ability to override and call the eclass-defined version from it. The use of it is best illustrated by an example; this is given below and is a snippet from a hypothetical `foo.eclass`.

### Example: foo.eclass

```bash
foo_src_compile()
{
    econf --enable-gerbil \
            $(use_enable fnord)
    emake gerbil || die "Couldn't make a gerbil"
    emake || die "emake failed"
}

EXPORT_FUNCTIONS src_compile
```

This example defines an eclass `src_compile` function and uses `EXPORT_FUNCTIONS` to alias it. Then any ebuild that inherits `foo.eclass` will have a default `src_compile` defined, but should the author wish to override it he can access the function in `foo.eclass` by calling `foo_src_compile`.

`EXPORT_FUNCTIONS` must only be used on ebuild phase functions. The function that is aliased must be named `<eclass>_<phase-function>`, where `<eclass>` is the name of the eclass.

If `EXPORT_FUNCTIONS` is called multiple times for the same phase function, the last call takes precedence. Eclasses may not rely upon any particular behaviour if they inherit another eclass after calling `EXPORT_FUNCTIONS`.

## Implementation Notes

### For GRPM implementation:

**Eclass Location:**
- Load eclasses from `${REPOSITORY_ROOT}/eclass/` directory
- Validate eclass names according to section 3.1.6: `[A-Za-z0-9_.-]`, must begin with letter or underscore

**inherit Command:**
- Implement as Bash function (or package manager internal)
- Parameters: one or more eclass names (without `.eclass` extension)
- Source each eclass file in order
- Handle multiple inheritance (eclasses may be sourced multiple times)
- Set `ECLASS` variable to current eclass name during sourcing
- Accumulate all inherited eclass names in `INHERITED` variable (space-separated)

**Variable Accumulation:**
- Always accumulate: `IUSE`, `REQUIRED_USE`, `DEPEND`, `BDEPEND`, `RDEPEND`, `PDEPEND`, `IDEPEND`
- EAPI 8+: also accumulate `PROPERTIES` and `RESTRICT`
- Order: eclass1 values + eclass2 values + ... + ebuild values
- For `RDEPEND`: apply EAPI 0-3 implicit `RDEPEND=DEPEND` rule *before* appending eclass values

**EXPORT_FUNCTIONS Implementation:**
- Define wrapper function `<phase>` that calls `<eclass>_<phase>`
- Example: if `foo.eclass` exports `src_compile`, create `src_compile() { foo_src_compile "$@"; }`
- Only valid for phase functions (pkg_*, src_*)
- Last `EXPORT_FUNCTIONS` call wins if multiple eclasses export same phase
- Allow ebuild to override by defining its own `<phase>` function (ebuild definitions override eclass)

**Metadata Caching:**
- Parse eclass inheritance at metadata generation time
- Cache `INHERITED` variable
- Regenerate metadata when any inherited eclass changes
- Track eclass modification times

**Example Workflow:**

```bash
# ebuild.ebuild
inherit foo bar

DEPEND="app-misc/hello"

# After processing:
# INHERITED="foo bar"
# DEPEND="${foo_DEPEND} ${bar_DEPEND} app-misc/hello"
# IUSE="${foo_IUSE} ${bar_IUSE} ${ebuild_IUSE}"
# src_compile() { bar_src_compile "$@"; }  # bar was last to EXPORT_FUNCTIONS
```

**Edge Cases:**
- Circular inheritance: detect and error
- Missing eclass: fail with clear error message
- Multiple `EXPORT_FUNCTIONS` calls: last one wins
- Eclass modifying `EAPI`: forbidden (see section 7.3.1)
- Eclass inheritance in conditional scope: only allowed in constant conditions (see section 7.1)

**Testing:**
- Test variable accumulation order
- Test EXPORT_FUNCTIONS precedence
- Test multiple inheritance
- Verify `ECLASS` and `INHERITED` variables
- Test EAPI-specific PROPERTIES/RESTRICT accumulation

## References

- [PMS Chapter 10: Eclasses](https://projects.gentoo.org/pms/latest/pms.html#eclasses)
- [Gentoo Developer Manual: Eclass Writing](https://devmanual.gentoo.org/eclass-writing/)
- [Gentoo Wiki: Eclasses](https://wiki.gentoo.org/wiki/Eclass)
