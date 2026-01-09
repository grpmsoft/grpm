# Chapter 3: Names and versions

> **Source:** [Package Manager Specification (PMS)](https://projects.gentoo.org/pms/latest/pms.html#names-and-versions)
> **EAPI:** All versions
> **Last Updated:** 2025-12-14 (PMS commit 43857e7)

## 3.1 Restrictions upon names

No name may be empty. Package managers must not impose fixed upper boundaries upon the length of any name. A package manager should indicate or reject any name that is invalid according to these rules.

### 3.1.1 Category names

A category name may contain any of the characters `[A-Za-z0-9+_.-]`. It must not begin with a hyphen, a dot or a plus sign.

### 3.1.2 Package names

A package name may contain any of the characters `[A-Za-z0-9+_-]`. It must not begin with a hyphen or a plus sign, and must not end in a hyphen followed by anything matching the version syntax described in section 3.2.

**Note:** A package name does not include the category. The term *qualified package name* is used where a `category/package` pair is meant.

### 3.1.3 Slot names

A slot name may contain any of the characters `[A-Za-z0-9+_.-]`. It must not begin with a hyphen, a dot or a plus sign.

### 3.1.4 USE flag names

A USE flag name may contain any of the characters `[A-Za-z0-9+_@-]`. It must begin with an alphanumeric character. Underscores should be considered reserved for `USE_EXPAND`, as described in section 11.1.1.

**Note:** Usage of the at-sign is deprecated. It was previously required for `LINGUAS`.

### 3.1.5 Repository names

A repository name may contain any of the characters `[A-Za-z0-9_-]`. It must not begin with a hyphen. In addition, every repository name must also be a valid package name.

### 3.1.6 Eclass names

An eclass name may contain any of the characters `[A-Za-z0-9_.-]`. It must begin with a letter or an underscore. In addition, an eclass cannot be named `default`.

### 3.1.7 License names

A license name may contain any of the characters `[A-Za-z0-9+_.-]`. It must not begin with a hyphen, a dot or a plus sign.

### 3.1.8 Keyword names

A keyword name may contain any of the characters `[A-Za-z0-9_-]`. It must not begin with a hyphen. In contexts where it makes sense to do so, a keyword name may be prefixed by a tilde or a hyphen. In `KEYWORDS`, `-*` is also acceptable as a keyword.

### 3.1.9 EAPI names

An EAPI name may contain any of the characters `[A-Za-z0-9+_.-]`. It must not begin with a hyphen, a dot or a plus sign.

## 3.2 Version specifications

The package manager must neither impose fixed limits upon the number of version components, nor upon the length of any component. Package managers should indicate or reject any version that is invalid according to the rules below.

A version starts with the number part, which is in the form `[0-9]+(\.[0-9]+)*` (an unsigned integer, followed by zero or more dot-prefixed unsigned integers).

This may optionally be followed by one of `[a-z]` (a lower-case letter).

This may be followed by zero or more of the suffixes `_alpha`, `_beta`, `_pre`, `_rc` or `_p`, each of which may optionally be followed by an unsigned integer. Suffix and integer count as separate version components.

This may optionally be followed by the suffix `-r` followed immediately by an unsigned integer (the "revision number"). If this suffix is not present, it is assumed to be `-r0`.

## 3.3 Version comparison

Version specifications are compared component by component, moving from left to right, as detailed in the algorithms below. If a sub-algorithm returns a decision, then that is the result of the whole comparison; if it terminates without returning a decision, the process continues from the point from which it was invoked.

### Algorithm 3.1: Version comparison top-level logic

1. let A and B be the versions to be compared
2. compare numeric components using Algorithm 3.2
3. compare letter components using Algorithm 3.4
4. compare suffixes using Algorithm 3.5
5. compare revision components using Algorithm 3.7
6. **return** A = B

### Algorithm 3.2: Version comparison logic for numeric components

1. define the notations An<sub>k</sub> and Bn<sub>k</sub> to mean the k<sup>th</sup> numeric component of A and B respectively, using 0-based indexing
2. **if** An<sub>0</sub> > Bn<sub>0</sub> using integer comparison **then**
3.   **return** A > B
4. **else if** An<sub>0</sub> < Bn<sub>0</sub> using integer comparison **then**
5.   **return** A < B
6. **end if**
7. let Ann be the number of numeric components of A
8. let Bnn be the number of numeric components of B
9. **for all** i such that i ≥ 1 and i < Ann and i < Bnn, in ascending order **do**
10.   compare An<sub>i</sub> and Bn<sub>i</sub> using Algorithm 3.3
11. **end for**
12. **if** Ann > Bnn **then**
13.   **return** A > B
14. **else if** Ann < Bnn **then**
15.   **return** A < B
16. **end if**

### Algorithm 3.3: Version comparison logic for each numeric component after the first

1. **if** either An<sub>i</sub> or Bn<sub>i</sub> has a leading `0` **then**
2.   let An'<sub>i</sub> be An<sub>i</sub> with any trailing `0`s removed
3.   let Bn'<sub>i</sub> be Bn<sub>i</sub> with any trailing `0`s removed
4.   **if** An'<sub>i</sub> > Bn'<sub>i</sub> using ASCII stringwise comparison **then**
5.     **return** A > B
6.   **else if** An'<sub>i</sub> < Bn'<sub>i</sub> using ASCII stringwise comparison **then**
7.     **return** A < B
8.   **end if**
9. **else**
10.   **if** An<sub>i</sub> > Bn<sub>i</sub> using integer comparison **then**
11.     **return** A > B
12.   **else if** An<sub>i</sub> < Bn<sub>i</sub> using integer comparison **then**
13.     **return** A < B
14.   **end if**
15. **end if**

### Algorithm 3.4: Version comparison logic for letter components

1. let Al be the letter component of A if any, otherwise the empty string
2. let Bl be the letter component of B if any, otherwise the empty string
3. **if** Al > Bl using ASCII stringwise comparison **then**
4.   **return** A > B
5. **else if** Al < Bl using ASCII stringwise comparison **then**
6.   **return** A < B
7. **end if**

### Algorithm 3.5: Version comparison logic for suffixes

1. define the notations As<sub>k</sub> and Bs<sub>k</sub> to mean the k<sup>th</sup> suffix of A and B respectively, using 0-based indexing
2. let Asn be the number of suffixes of A
3. let Bsn be the number of suffixes of B
4. **for all** i such that i ≥ 0 and i < Asn and i < Bsn, in ascending order **do**
5.   compare As<sub>i</sub> and Bs<sub>i</sub> using Algorithm 3.6
6. **end for**
7. **if** Asn > Bsn **then**
8.   **if** As<sub>Bsn</sub> is of type `_p` **then**
9.     **return** A > B
10.   **else**
11.     **return** A < B
12.   **end if**
13. **else if** Asn < Bsn **then**
14.   **if** Bs<sub>Asn</sub> is of type `_p` **then**
15.     **return** A < B
16.   **else**
17.     **return** A > B
18.   **end if**
19. **end if**

### Algorithm 3.6: Version comparison logic for each suffix

1. **if** As<sub>i</sub> and Bs<sub>i</sub> are of the same type (`_alpha` vs `_beta` etc) **then**
2.   let As'<sub>i</sub> be the integer part of As<sub>i</sub> if any, otherwise `0`
3.   let Bs'<sub>i</sub> be the integer part of Bs<sub>i</sub> if any, otherwise `0`
4.   **if** As'<sub>i</sub> > Bs'<sub>i</sub>, using integer comparison **then**
5.     **return** A > B
6.   **else if** As'<sub>i</sub> < Bs'<sub>i</sub>, using integer comparison **then**
7.     **return** A < B
8.   **end if**
9. **else if** the type of As<sub>i</sub> is greater than the type of Bs<sub>i</sub> using the ordering `_alpha` < `_beta` < `_pre` < `_rc` < `_p` **then**
10.   **return** A > B
11. **else**
12.   **return** A < B
13. **end if**

### Algorithm 3.7: Version comparison logic for revision components

1. let Ar be the integer part of the revision component of A if any, otherwise `0`
2. let Br be the integer part of the revision component of B if any, otherwise `0`
3. **if** Ar > Br using integer comparison **then**
4.   **return** A > B
5. **else if** Ar < Br using integer comparison **then**
6.   **return** A < B
7. **end if**

## 3.4 Uniqueness of versions

No two packages in a given repository may have the same qualified package name and equal versions. For example, a repository may not contain more than one of `foo-bar/baz-1.0.2`, `foo-bar/baz-1.0.2-r0` and `foo-bar/baz-1.000.2`.

## Implementation Notes

### For GRPM implementation:

**Version Parsing:**
- Implement parser for version specification format: `[0-9]+(\.[0-9]+)*[a-z]?(_suffix[0-9]*)*(-r[0-9]+)?`
- Handle edge cases: leading zeros in components, missing revision numbers

**Version Comparison:**
- Implement all 7 algorithms exactly as specified
- Algorithm 3.3 requires special handling for numeric components with leading zeros (use ASCII stringwise comparison after removing trailing zeros)
- Suffix ordering: `_alpha` < `_beta` < `_pre` < `_rc` < `_p`
- Pay attention to suffix count differences (lines 7-19 in Algorithm 3.5)

**Validation:**
- Enforce name restrictions for all entity types (packages, categories, slots, USE flags, etc.)
- Reject invalid version strings early in parsing
- Check for version uniqueness when building repository indexes

**Testing:**
- Test version comparison with edge cases:
  - `1.0` vs `1.00` (should be equal)
  - `1.0.0` vs `1.0` (first is greater)
  - `1.0_alpha` vs `1.0_beta` vs `1.0_pre` vs `1.0_rc` vs `1.0_p1`
  - Leading zeros: `1.01` vs `1.010` (stringwise comparison)
  - Revision handling: `1.0` vs `1.0-r0` (equal), `1.0-r1` vs `1.0-r10` (integer comparison)

## References

- [PMS Chapter 3: Names and versions](https://projects.gentoo.org/pms/latest/pms.html#names-and-versions)
- [Gentoo Developer Manual: Ebuild Writing](https://devmanual.gentoo.org/ebuild-writing/)
- [Version specification examples (Gentoo Wiki)](https://wiki.gentoo.org/wiki/Version_specifier)
