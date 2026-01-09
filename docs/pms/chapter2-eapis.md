# Chapter 2: EAPIs

> **Source:** [Gentoo Package Manager Specification (PMS)](https://projects.gentoo.org/pms/)
>
> This document is derived from the official PMS HTML version. For authoritative information, always refer to the [official PMS](https://wiki.gentoo.org/wiki/Package_Manager_Specification).

---

## 2.1 Definition

An EAPI can be thought of as a 'version' of this specification to which a package conforms. An EAPI value is a string as per section 3.1.9, and is part of an ebuild's metadata.

If a package manager encounters a package version with an unrecognised EAPI, it must not attempt to perform any operations upon it. It could, for example, ignore the package version entirely (although this can lead to user confusion), or it could mark the package version as masked. A package manager must not use any metadata generated from a package with an unrecognised EAPI.

The package manager must not attempt to perform any kind of comparison test other than equality upon EAPIs.

EAPIs are also used for profile directories, as described in section 5.2.2.

## 2.2 Defined EAPIs

This specification defines EAPIs '0', '1', '2', '3', '4', '5', '6', '7', '8', and '9'. EAPI '0' is the 'original' base EAPI. Each of the later EAPIs contains a number of extensions to its predecessor.

Except where explicitly noted, everything in this specification applies to all of the above EAPIs.

## 2.3 Reserved EAPIs

- EAPIs whose value consists purely of an integer are reserved for future versions of this specification.
- EAPIs whose value starts with the string `paludis-` are reserved for experimental use by the Paludis package manager.

---

*Converted from PMS HTML for GRPM development reference.*
