# Chapter 1: Introduction

> **Source:** [Gentoo Package Manager Specification (PMS)](https://projects.gentoo.org/pms/)
>
> This document is derived from the official PMS HTML version. For authoritative information, always refer to the [official PMS](https://wiki.gentoo.org/wiki/Package_Manager_Specification).

---

## 1.1 Aims and motivation

This document aims to fully describe the format of an ebuild repository and the ebuilds therein, as well as certain aspects of package manager behaviour required to support such a repository.

This document is *not* designed to be an introduction to ebuild development. Prior knowledge of ebuild creation and an understanding of how the package management system works is assumed; certain less familiar terms are explained in the Glossary.

This document does not specify any user or package manager configuration information.

## 1.2 Rationale

At present the only definition of what an ebuild can assume about its environment, and the only definition of what is valid in an ebuild, is the source code of the latest Portage release and a general consensus about which features are too new to assume availability. This has several drawbacks: not only is it impossible to change any aspect of Portage behaviour without verifying that nothing in the tree relies upon it, but if a new package manager should appear it becomes impossible to fully support such an ill-defined standard.

This document aims to address both of these concerns by defining almost all aspects of what an ebuild repository looks like, and how an ebuild is allowed to behave. Thus, both Portage and other package managers can change aspects of their behaviour not defined here without worry of incompatibilities with any particular repository.

## 1.3 Reporting issues

Issues (inaccuracies, wording problems, omissions etc.) in this document should be reported via Gentoo Bugzilla using product *Gentoo Hosted Projects*, component *PMS/EAPI* and the default assignee. There should be one bug per issue, and one issue per bug.

Patches (in `git format-patch` form if possible) may be submitted either via Bugzilla or to the [gentoo-pms@lists.gentoo.org](mailto:gentoo-pms@lists.gentoo.org) mailing list. Patches will be reviewed by the PMS team, who will do one of the following:

- Accept and apply the patch.
- Explain why the patch cannot be applied as-is. The patch may then be updated and resubmitted if appropriate.
- Reject the patch outright.
- Take special action merited by the individual circumstances.

When reporting issues, remember that this document is not the appropriate place for pushing through changes to the tree or the package manager, except where those changes are bugs.

If any issue cannot be resolved by the PMS team, it may be escalated to the Gentoo Council.

## 1.4 Conventions

Text in `teletype` is used for filenames or variable names. *Italic* text is used for terms with a particular technical meaning in places where there may otherwise be ambiguity.

The term *package manager* is used throughout this document in a broad sense. Although some parts of this document are only relevant to fully featured package managers, many items are equally applicable to tools or other applications that interact with ebuilds or ebuild repositories.

## 1.5 Acknowledgements

Thanks to Mike Kelly (package manager provided utilities, section 12.3), Danny van Dyk (ebuild functions, chapter 9), David Leverton (various sections), Petteri Räty (environment state, section 11.2), Michał Górny (various sections), Andreas K. Hüttel (stable use masking, section 5.2.12), Zac Medico (sub-slots, section 7.2) and James Le Cuirot (build dependencies, section 11.1) for contributions. Thanks also to Mike Frysinger and Brian Harring for proof-reading and suggestions for fixes and/or clarification.

---

*Converted from PMS HTML for GRPM development reference.*
