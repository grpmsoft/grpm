// Package pkg provides domain models for Gentoo package management.
// This file implements the EAPI feature matrix per PMS Chapter 2.
package pkg

import (
	"errors"
	"fmt"
)

// ErrUnsupportedEAPI is returned when an unknown or unsupported EAPI is encountered.
// Per PMS Section 2.1: Package managers must not perform operations on packages
// with unrecognized EAPIs.
var ErrUnsupportedEAPI = errors.New("unsupported EAPI")

// EAPIFeatures represents the features available in a specific EAPI version.
// This is a Value Object (immutable) representing the capabilities defined by
// the Package Manager Specification (PMS) for each EAPI.
//
// Per PMS Chapter 2: EAPIs are version identifiers that define which features
// and behaviors are available for ebuilds and profiles.
type EAPIFeatures struct {
	// Version is the EAPI version string (e.g., "0", "7", "8").
	Version string

	// --- Dependency Variables (PMS Section 7.2) ---

	// BDEPEND indicates if BDEPEND variable is supported.
	// EAPI 7+: Build dependencies that are run on CBUILD.
	BDEPEND bool

	// IDEPEND indicates if IDEPEND variable is supported.
	// EAPI 8+: Install-time dependencies run on ROOT.
	IDEPEND bool

	// --- Dependency Syntax (PMS Section 8) ---

	// SlotDeps indicates if slot dependencies are supported.
	// EAPI 1+: :slot dependency syntax.
	SlotDeps bool

	// SlotOperators indicates if slot operators are supported.
	// EAPI 5+: := (rebuild on slot change), :* (any slot), :slot= (subslot rebuild).
	SlotOperators bool

	// SubSlots indicates if subslots are supported.
	// EAPI 5+: Subslots for ABI tracking (e.g., "0/1.2").
	SubSlots bool

	// UseDeps indicates if USE dependencies are supported.
	// EAPI 2+: [use], [use=], [!use?], etc.
	UseDeps bool

	// UseDepDefaults indicates if USE dep default syntax is supported.
	// EAPI 4+: [use(+)], [use(-)] in dependency atoms.
	UseDepDefaults bool

	// --- USE Flags (PMS Section 11.1) ---

	// IUSEDefaults indicates if IUSE defaults (+flag, -flag) are supported.
	// EAPI 1+: Prefixing USE flags with + or - in IUSE.
	IUSEDefaults bool

	// IUSEEffective indicates if IUSE_EFFECTIVE is generated.
	// EAPI 5+: Profile-injected USE flags in IUSE_EFFECTIVE.
	IUSEEffective bool

	// --- REQUIRED_USE (PMS Section 7.2) ---

	// RequiredUse indicates if REQUIRED_USE variable is supported.
	// EAPI 4+: Declaring USE flag combinations.
	RequiredUse bool

	// RequiredUseAtMostOne indicates if ?? (at-most-one-of) operator is supported.
	// EAPI 5+: ?? ( a b c ) in REQUIRED_USE.
	RequiredUseAtMostOne bool

	// --- SRC_URI Features (PMS Section 7.2.6) ---

	// SrcURIArrows indicates if SRC_URI renaming arrows are supported.
	// EAPI 2+: SRC_URI="http://example.com/foo.tar.gz -> bar.tar.gz"
	SrcURIArrows bool

	// SrcURISelectiveRestrictions indicates if fetch+/mirror+ URIs are supported.
	// EAPI 8+: Selective RESTRICT application per URI.
	SrcURISelectiveRestrictions bool

	// --- Phase Functions (PMS Section 9) ---

	// HasPkgPretend indicates if pkg_pretend phase is supported.
	// EAPI 4+: Pre-fetch sanity checks.
	HasPkgPretend bool

	// HasPkgInfoNonInstalled indicates if pkg_info can be called on non-installed packages.
	// EAPI 4+: pkg_info works for non-installed packages.
	HasPkgInfoNonInstalled bool

	// HasSrcPrepare indicates if src_prepare phase is supported.
	// EAPI 2+: Explicit preparation phase.
	HasSrcPrepare bool

	// HasSrcConfigure indicates if src_configure phase is supported.
	// EAPI 2+: Separate configuration phase.
	HasSrcConfigure bool

	// DefaultSrcPrepareFormat defines the default src_prepare behavior.
	// 0 = no-op, 2 = EAPI 2 style, 6 = EAPI 6 style (eapply_user required), 8 = EAPI 8 style.
	DefaultSrcPrepareFormat int

	// DefaultSrcInstallFormat defines the default src_install behavior.
	// 0 = no-op, 4 = EAPI 4 style, 6 = EAPI 6 style.
	DefaultSrcInstallFormat int

	// --- RDEPEND Default Behavior (PMS Section 7.2) ---

	// RdependDefaultsToDepend indicates if RDEPEND defaults to DEPEND when unset.
	// EAPI 0-3: RDEPEND=DEPEND if RDEPEND is not set.
	// EAPI 4+: This behavior is removed.
	RdependDefaultsToDepend bool

	// --- Bash Version (PMS Section 6.1) ---

	// BashVersion is the minimum Bash version required.
	// EAPI 0-5: 3.2, EAPI 6-7: 4.2, EAPI 8+: 5.0
	BashVersion string

	// BashVersionMajor is the major version number for programmatic checks.
	BashVersionMajor int

	// BashVersionMinor is the minor version number for programmatic checks.
	BashVersionMinor int

	// --- Global Scope Behavior (PMS Section 6.2) ---

	// Failglob indicates if failglob is enabled in global scope.
	// EAPI 6+: Shell option failglob is set in global scope.
	Failglob bool

	// --- Helper Functions (PMS Section 11.3) ---

	// DosymRelative indicates if dosym -r flag is supported.
	// EAPI 8+: dosym -r creates relative symlinks.
	DosymRelative bool

	// Dostrip indicates if dostrip helper is available.
	// EAPI 7+: Fine-grained strip control.
	Dostrip bool

	// Eapply indicates if eapply helper is available.
	// EAPI 6+: Replaces epatch.
	Eapply bool

	// EapplyUser indicates if eapply_user is available and required.
	// EAPI 6+: Must be called in src_prepare (handled by default).
	EapplyUser bool

	// Einstalldocs indicates if einstalldocs helper is available.
	// EAPI 6+: Standard documentation installation.
	Einstalldocs bool

	// GetLibdir indicates if get_libdir helper is available.
	// EAPI 6+: Returns lib or lib64 based on ABI.
	GetLibdir bool

	// InDodir indicates if helpers respect in_iuse/in_dodir state.
	// EAPI 6+: Various helpers check if USE flag is in IUSE.
	InDodir bool

	// Usev indicates if usev helper is available.
	// EAPI 5+: usev returns USE flag name instead of 0.
	Usev bool

	// UsexDefaultValues indicates if usex has default value parameters.
	// EAPI 5+: usex flag [yes [no [yesval [noval]]]]
	UsexDefaultValues bool

	// --- Profile Features (PMS Section 5) ---

	// ProfileDirectories indicates if profile file directories are supported.
	// EAPI 7+: package.mask/, package.use/ directories alongside files.
	ProfileDirectories bool

	// EnvUnset indicates if ENV_UNSET profile variable is supported.
	// EAPI 7+: Unset environment variables in profile.
	EnvUnset bool

	// --- Offset-Prefix Variables (PMS Section 11.1) ---

	// OffsetPrefix indicates if offset-prefix variables are supported.
	// EAPI 3+: EPREFIX, EROOT, ED variables.
	OffsetPrefix bool

	// ESYSROOT indicates if ESYSROOT variable is supported.
	// EAPI 7+: ${SYSROOT}${EPREFIX} for cross-compilation.
	ESYSROOTSupported bool

	// --- Cross-Compilation Variables (PMS Section 11.1) ---

	// SYSROOT indicates if SYSROOT and related variables are supported.
	// EAPI 7+: SYSROOT, ESYSROOT, BROOT for cross-compilation.
	SYSROOTSupported bool

	// BROOT indicates if BROOT variable is supported.
	// EAPI 7+: Build root for build dependencies (CBUILD).
	BROOTSupported bool

	// --- Trailing Slash Behavior (PMS Section 11.1.4) ---

	// TrailingSlash indicates if path variables always have trailing slashes.
	// EAPI 0-6: ROOT, EROOT, D, ED always end with '/'.
	// EAPI 7+: These variables never have trailing slashes.
	TrailingSlash bool

	// --- Dependency Group Behavior ---

	// EmptyGroupsMatch indicates how empty || and ^^ groups behave.
	// EAPI 0-6: Empty groups may match, EAPI 7+: Empty groups never match.
	EmptyGroupsMatch bool

	// --- Metadata Variables (PMS Section 7.2) ---

	// Properties indicates if PROPERTIES variable is mandatory.
	// EAPI 4+: PROPERTIES is a defined, mandatory variable.
	Properties bool

	// Restrict indicates if RESTRICT variable is fully supported.
	// All EAPIs: RESTRICT is supported, but behaviors vary.
	Restrict bool

	// --- Failure Behavior (PMS Section 12.3.1) ---

	// Nonfatal indicates if nonfatal command is supported.
	// EAPI 4+: nonfatal prevents die from aborting the build.
	Nonfatal bool

	// NonfatalIsExternal indicates if nonfatal is both a shell function and external command.
	// EAPI 7+: nonfatal must be available as both for xargs compatibility.
	NonfatalIsExternal bool
}

// supportedEAPIs holds the feature matrix for all supported EAPI versions.
// This is the canonical source of EAPI capabilities in GRPM.
var supportedEAPIs = map[string]EAPIFeatures{
	"0": {
		Version:                 "0",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		RdependDefaultsToDepend: true,
		EmptyGroupsMatch:        true,
		Restrict:                true,
		TrailingSlash:           true, // EAPI 0-6 always have trailing slashes
	},
	"1": {
		Version:                 "1",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		RdependDefaultsToDepend: true,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		IUSEDefaults:            true,
		Restrict:                true,
		TrailingSlash:           true,
	},
	"2": {
		Version:                 "2",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		RdependDefaultsToDepend: true,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		IUSEDefaults:            true,
		UseDeps:                 true,
		SrcURIArrows:            true,
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 2,
		Restrict:                true,
		TrailingSlash:           true,
	},
	"3": {
		Version:                 "3",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		RdependDefaultsToDepend: true,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		IUSEDefaults:            true,
		UseDeps:                 true,
		SrcURIArrows:            true,
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 2,
		Restrict:                true,
		TrailingSlash:           true,
		OffsetPrefix:            true, // EAPI 3+ supports EPREFIX, EROOT, ED
	},
	"4": {
		Version:                 "4",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		IUSEDefaults:            true,
		UseDeps:                 true,
		UseDepDefaults:          true,
		SrcURIArrows:            true,
		HasPkgPretend:           true, // EAPI 4+ supports pkg_pretend
		HasPkgInfoNonInstalled:  true, // EAPI 4+ pkg_info on non-installed
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 2,
		DefaultSrcInstallFormat: 4,
		RequiredUse:             true,
		Properties:              true,
		Restrict:                true,
		RdependDefaultsToDepend: false, // Removed in EAPI 4
		TrailingSlash:           true,
		OffsetPrefix:            true,
		Nonfatal:                true, // EAPI 4+ supports nonfatal
	},
	"5": {
		Version:                 "5",
		BashVersion:             "3.2",
		BashVersionMajor:        3,
		BashVersionMinor:        2,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		SlotOperators:           true,
		SubSlots:                true,
		IUSEDefaults:            true,
		IUSEEffective:           true,
		UseDeps:                 true,
		UseDepDefaults:          true,
		SrcURIArrows:            true,
		HasPkgPretend:           true, // EAPI 4+ supports pkg_pretend
		HasPkgInfoNonInstalled:  true, // EAPI 4+ pkg_info on non-installed
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 2,
		DefaultSrcInstallFormat: 4,
		RequiredUse:             true,
		RequiredUseAtMostOne:    true,
		Properties:              true,
		Restrict:                true,
		Usev:                    true,
		UsexDefaultValues:       true,
		RdependDefaultsToDepend: false,
		TrailingSlash:           true,
		OffsetPrefix:            true,
		Nonfatal:                true, // EAPI 4+ supports nonfatal
	},
	"6": {
		Version:                 "6",
		BashVersion:             "4.2",
		BashVersionMajor:        4,
		BashVersionMinor:        2,
		Failglob:                true,
		EmptyGroupsMatch:        true,
		SlotDeps:                true,
		SlotOperators:           true,
		SubSlots:                true,
		IUSEDefaults:            true,
		IUSEEffective:           true,
		UseDeps:                 true,
		UseDepDefaults:          true,
		SrcURIArrows:            true,
		HasPkgPretend:           true, // EAPI 4+ supports pkg_pretend
		HasPkgInfoNonInstalled:  true, // EAPI 4+ pkg_info on non-installed
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 6,
		DefaultSrcInstallFormat: 6,
		RequiredUse:             true,
		RequiredUseAtMostOne:    true,
		Properties:              true,
		Restrict:                true,
		Usev:                    true,
		UsexDefaultValues:       true,
		Eapply:                  true,
		EapplyUser:              true,
		Einstalldocs:            true,
		GetLibdir:               true,
		InDodir:                 true,
		RdependDefaultsToDepend: false,
		TrailingSlash:           true,
		OffsetPrefix:            true,
		Nonfatal:                true, // EAPI 4+ supports nonfatal
	},
	"7": {
		Version:                 "7",
		BashVersion:             "4.2",
		BashVersionMajor:        4,
		BashVersionMinor:        2,
		Failglob:                true,
		EmptyGroupsMatch:        false, // Changed in EAPI 7
		SlotDeps:                true,
		SlotOperators:           true,
		SubSlots:                true,
		IUSEDefaults:            true,
		IUSEEffective:           true,
		UseDeps:                 true,
		UseDepDefaults:          true,
		SrcURIArrows:            true,
		HasPkgPretend:           true, // EAPI 4+ supports pkg_pretend
		HasPkgInfoNonInstalled:  true, // EAPI 4+ pkg_info on non-installed
		HasSrcPrepare:           true,
		HasSrcConfigure:         true,
		DefaultSrcPrepareFormat: 6,
		DefaultSrcInstallFormat: 6,
		RequiredUse:             true,
		RequiredUseAtMostOne:    true,
		Properties:              true,
		Restrict:                true,
		Usev:                    true,
		UsexDefaultValues:       true,
		Eapply:                  true,
		EapplyUser:              true,
		Einstalldocs:            true,
		GetLibdir:               true,
		InDodir:                 true,
		BDEPEND:                 true,
		Dostrip:                 true,
		ProfileDirectories:      true,
		EnvUnset:                true,
		RdependDefaultsToDepend: false,
		TrailingSlash:           false, // EAPI 7+ never have trailing slashes
		OffsetPrefix:            true,
		SYSROOTSupported:        true, // EAPI 7+ cross-compilation
		ESYSROOTSupported:       true,
		BROOTSupported:          true,
		Nonfatal:                true, // EAPI 4+ supports nonfatal
		NonfatalIsExternal:      true, // EAPI 7+ nonfatal is both function and external command
	},
	"8": {
		Version:                     "8",
		BashVersion:                 "5.0",
		BashVersionMajor:            5,
		BashVersionMinor:            0,
		Failglob:                    true,
		EmptyGroupsMatch:            false,
		SlotDeps:                    true,
		SlotOperators:               true,
		SubSlots:                    true,
		IUSEDefaults:                true,
		IUSEEffective:               true,
		UseDeps:                     true,
		UseDepDefaults:              true,
		SrcURIArrows:                true,
		SrcURISelectiveRestrictions: true,
		HasPkgPretend:               true, // EAPI 4+ supports pkg_pretend
		HasPkgInfoNonInstalled:      true, // EAPI 4+ pkg_info on non-installed
		HasSrcPrepare:               true,
		HasSrcConfigure:             true,
		DefaultSrcPrepareFormat:     8,
		DefaultSrcInstallFormat:     6,
		RequiredUse:                 true,
		RequiredUseAtMostOne:        true,
		Properties:                  true,
		Restrict:                    true,
		Usev:                        true,
		UsexDefaultValues:           true,
		Eapply:                      true,
		EapplyUser:                  true,
		Einstalldocs:                true,
		GetLibdir:                   true,
		InDodir:                     true,
		BDEPEND:                     true,
		IDEPEND:                     true,
		Dostrip:                     true,
		DosymRelative:               true,
		ProfileDirectories:          true,
		EnvUnset:                    true,
		RdependDefaultsToDepend:     false,
		TrailingSlash:               false, // EAPI 7+ never have trailing slashes
		OffsetPrefix:                true,
		SYSROOTSupported:            true, // EAPI 7+ cross-compilation
		ESYSROOTSupported:           true,
		BROOTSupported:              true,
		Nonfatal:                    true, // EAPI 4+ supports nonfatal
		NonfatalIsExternal:          true, // EAPI 7+ nonfatal is both function and external command
	},
}

// GetEAPIFeatures returns the feature set for a given EAPI version.
// Returns ErrUnsupportedEAPI if the EAPI is not recognized.
//
// Per PMS Section 2.1: If a package manager encounters a package version
// with an unrecognized EAPI, it must not attempt to perform any operations
// upon it.
func GetEAPIFeatures(eapi string) (EAPIFeatures, error) {
	features, ok := supportedEAPIs[eapi]
	if !ok {
		return EAPIFeatures{}, fmt.Errorf("%w: %s", ErrUnsupportedEAPI, eapi)
	}
	return features, nil
}

// MustGetEAPIFeatures returns the feature set for a given EAPI version.
// Panics if the EAPI is not recognized. Use only for known-valid EAPIs.
func MustGetEAPIFeatures(eapi string) EAPIFeatures {
	features, err := GetEAPIFeatures(eapi)
	if err != nil {
		panic(fmt.Sprintf("invalid EAPI: %s", eapi))
	}
	return features
}

// IsEAPISupported returns true if the given EAPI is recognized.
func IsEAPISupported(eapi string) bool {
	_, ok := supportedEAPIs[eapi]
	return ok
}

// SupportedEAPIs returns a slice of all supported EAPI versions.
func SupportedEAPIs() []string {
	eapis := make([]string, 0, len(supportedEAPIs))
	for eapi := range supportedEAPIs {
		eapis = append(eapis, eapi)
	}
	return eapis
}

// LatestEAPI returns the highest supported EAPI version.
func LatestEAPI() string {
	return "8"
}

// DefaultEAPI returns the default EAPI for ebuilds without explicit EAPI.
// Per PMS Section 7.1: If no EAPI is specified, EAPI 0 is assumed.
func DefaultEAPI() string {
	return "0"
}

// --- EAPI Validation (PMS Chapter 2) ---

// ErrInvalidEAPIFormat is returned when EAPI contains invalid characters.
// Per PMS Section 2.1: EAPI must be a single word (no whitespace).
var ErrInvalidEAPIFormat = errors.New("invalid EAPI format")

// ValidateEAPI validates an EAPI value per PMS Chapter 2.
//
// Per PMS Section 2.1:
//   - Package managers must not perform operations on packages with unrecognized EAPIs
//   - EAPI must be a single word (no whitespace)
//   - Empty EAPI is valid (defaults to EAPI 0)
//
// Returns nil if EAPI is valid, error otherwise.
//
// Error types:
//   - ErrInvalidEAPIFormat: EAPI contains whitespace or invalid characters
//   - ErrUnsupportedEAPI: EAPI is not recognized/supported
func ValidateEAPI(eapi string) error {
	// Empty EAPI defaults to "0" per PMS Section 7.1
	if eapi == "" {
		return nil
	}

	// Check for invalid characters (whitespace)
	// Per PMS Section 2.1: EAPI must be a single word
	for _, ch := range eapi {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			return fmt.Errorf("%w: EAPI must be a single word, got %q", ErrInvalidEAPIFormat, eapi)
		}
	}

	// Check if EAPI is supported
	if !IsEAPISupported(eapi) {
		return fmt.Errorf("%w: %s", ErrUnsupportedEAPI, eapi)
	}

	return nil
}

// NormalizeEAPI returns the canonical EAPI value.
// Empty string is normalized to DefaultEAPI ("0").
// All other values are returned unchanged.
func NormalizeEAPI(eapi string) string {
	if eapi == "" {
		return DefaultEAPI()
	}
	return eapi
}

// --- Feature Query Methods ---

// SupportsSlotOperators returns true if the EAPI supports slot operators (:=, :*, :slot=).
func (e EAPIFeatures) SupportsSlotOperators() bool {
	return e.SlotOperators
}

// SupportsSubSlots returns true if the EAPI supports subslots.
func (e EAPIFeatures) SupportsSubSlots() bool {
	return e.SubSlots
}

// SupportsBDEPEND returns true if the EAPI supports BDEPEND.
func (e EAPIFeatures) SupportsBDEPEND() bool {
	return e.BDEPEND
}

// SupportsIDEPEND returns true if the EAPI supports IDEPEND.
func (e EAPIFeatures) SupportsIDEPEND() bool {
	return e.IDEPEND
}

// SupportsRequiredUse returns true if the EAPI supports REQUIRED_USE.
func (e EAPIFeatures) SupportsRequiredUse() bool {
	return e.RequiredUse
}

// SupportsDosymRelative returns true if the EAPI supports dosym -r.
func (e EAPIFeatures) SupportsDosymRelative() bool {
	return e.DosymRelative
}

// SupportsEapply returns true if the EAPI supports eapply helper.
func (e EAPIFeatures) SupportsEapply() bool {
	return e.Eapply
}

// SupportsSrcURIArrows returns true if the EAPI supports SRC_URI arrow syntax.
func (e EAPIFeatures) SupportsSrcURIArrows() bool {
	return e.SrcURIArrows
}

// SupportsSrcURISelectiveRestrictions returns true if the EAPI supports fetch+/mirror+.
func (e EAPIFeatures) SupportsSrcURISelectiveRestrictions() bool {
	return e.SrcURISelectiveRestrictions
}

// RequiresBashVersion returns true if the given bash version meets requirements.
func (e EAPIFeatures) RequiresBashVersion(major, minor int) bool {
	if major > e.BashVersionMajor {
		return true
	}
	if major == e.BashVersionMajor && minor >= e.BashVersionMinor {
		return true
	}
	return false
}

// String returns the EAPI version string.
func (e EAPIFeatures) String() string {
	return e.Version
}

// IsValid returns true if this EAPIFeatures represents a valid EAPI.
func (e EAPIFeatures) IsValid() bool {
	return e.Version != ""
}

// SupportsOffsetPrefix returns true if the EAPI supports offset-prefix variables.
// EAPI 3+: EPREFIX, EROOT, ED are available.
func (e EAPIFeatures) SupportsOffsetPrefix() bool {
	return e.OffsetPrefix
}

// SupportsSYSROOT returns true if the EAPI supports SYSROOT and ESYSROOT.
// EAPI 7+: Cross-compilation support with SYSROOT, ESYSROOT, BROOT.
func (e EAPIFeatures) SupportsSYSROOT() bool {
	return e.SYSROOTSupported
}

// SupportsBROOT returns true if the EAPI supports BROOT.
// EAPI 7+: Build root for build dependencies (CBUILD).
func (e EAPIFeatures) SupportsBROOT() bool {
	return e.BROOTSupported
}

// HasTrailingSlash returns true if path variables should have trailing slashes.
// EAPI 0-6: ROOT, EROOT, D, ED always end with '/'.
// EAPI 7+: These variables never have trailing slashes.
func (e EAPIFeatures) HasTrailingSlash() bool {
	return e.TrailingSlash
}

// SupportsNonfatal returns true if the EAPI supports nonfatal command.
// EAPI 4+: nonfatal prevents die from aborting the build.
// Per PMS Section 12.3.1: nonfatal takes one or more arguments and executes
// them as a command, preserving the exit status. If this results in a command
// being called that would normally abort the build process due to a failure,
// instead a non-zero exit status shall be returned.
func (e EAPIFeatures) SupportsNonfatal() bool {
	return e.Nonfatal
}

// NonfatalIsExternalCommand returns true if nonfatal must be available as
// both a shell function and an external command.
// EAPI 7+: nonfatal must be callable from xargs, so needs external command form.
func (e EAPIFeatures) NonfatalIsExternalCommand() bool {
	return e.NonfatalIsExternal
}

// SupportsPkgPretend returns true if the EAPI supports pkg_pretend phase.
// EAPI 4+: Pre-fetch sanity checks.
// Per PMS Section 9.1.2: pkg_pretend may be used to carry out sanity checks early
// in the install process (before fetching sources).
func (e EAPIFeatures) SupportsPkgPretend() bool {
	return e.HasPkgPretend
}

// SupportsPkgInfoNonInstalled returns true if the EAPI supports calling
// pkg_info on non-installed packages.
// EAPI 4+: pkg_info can be called for non-installed packages.
// Per PMS Section 9.1.15: In EAPIs supporting this, pkg_info may also be called
// when displaying information about a non-installed package.
func (e EAPIFeatures) SupportsPkgInfoNonInstalled() bool {
	return e.HasPkgInfoNonInstalled
}
