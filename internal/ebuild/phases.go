package ebuild

// Phase represents an ebuild execution phase.
type Phase string

const (
	// PhasePretend performs pre-fetch sanity checks (EAPI 4+)
	// This phase runs before fetching sources and must not modify the filesystem.
	// Per PMS Section 9.1.2: Used for sanity checks like kernel config, system requirements.
	PhasePretend Phase = "pretend"

	// PhaseFetch downloads source tarballs
	// This phase runs automatically before unpack when a Fetcher is configured
	PhaseFetch Phase = "fetch"

	// PhaseSetup initializes the build environment
	PhaseSetup Phase = "setup"

	// PhaseUnpack extracts source archives
	PhaseUnpack Phase = "unpack"

	// PhasePrepare applies patches and prepares sources
	PhasePrepare Phase = "prepare"

	// PhaseConfigure runs configuration (./configure, cmake, etc)
	PhaseConfigure Phase = "configure"

	// PhaseCompile compiles the sources
	PhaseCompile Phase = "compile"

	// PhaseTest runs test suites (optional)
	PhaseTest Phase = "test"

	// PhaseInstall installs to temporary directory (${D})
	PhaseInstall Phase = "install"

	// PhasePreinst runs before merging to system
	PhasePreinst Phase = "preinst"

	// PhasePostinst runs after merging to system
	PhasePostinst Phase = "postinst"

	// PhasePrerem runs before package removal
	PhasePrerem Phase = "prerm"

	// PhasePostrm runs after package removal
	PhasePostrm Phase = "postrm"

	// PhaseConfig performs post-install configuration
	// Per PMS Section 9.1.14: Interactive configuration after package installation.
	// Run manually via `grpm config category/package`.
	PhaseConfig Phase = "config"

	// PhaseInfo displays information about an installed package
	// Per PMS Section 9.1.15: Called when displaying package information.
	// EAPI 4+: Can also be called for non-installed packages.
	PhaseInfo Phase = "info"

	// PhaseNofetch handles fetch-restricted packages
	// Per PMS Section 9.1.16: Called when RESTRICT=fetch and files are unavailable.
	// Should print instructions for manual download.
	PhaseNofetch Phase = "nofetch"
)

// String returns string representation of phase.
func (p Phase) String() string {
	return string(p)
}

// StandardPhases returns the standard build phases in execution order.
func StandardPhases() []Phase {
	return []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
	}
}

// InstallPhases returns phases that run during package installation.
func InstallPhases() []Phase {
	return []Phase{
		PhasePreinst,
		PhasePostinst,
	}
}

// RemovalPhases returns phases that run during package removal.
func RemovalPhases() []Phase {
	return []Phase{
		PhasePrerem,
		PhasePostrm,
	}
}

// PhaseResult represents the result of executing a phase.
type PhaseResult struct {
	Phase    Phase
	Success  bool
	Output   string
	Error    error
	Duration int64 // milliseconds
}

// IsTestPhase returns true if this is the test phase.
func (p Phase) IsTestPhase() bool {
	return p == PhaseTest
}

// IsBuildPhase returns true if this is a build phase (fetch through install).
func (p Phase) IsBuildPhase() bool {
	buildPhases := map[Phase]bool{
		PhaseFetch:     true,
		PhaseSetup:     true,
		PhaseUnpack:    true,
		PhasePrepare:   true,
		PhaseConfigure: true,
		PhaseCompile:   true,
		PhaseInstall:   true,
	}
	return buildPhases[p]
}

// IsFetchPhase returns true if this is the fetch phase.
func (p Phase) IsFetchPhase() bool {
	return p == PhaseFetch
}

// IsHookPhase returns true if this is a hook phase (preinst, postinst, etc).
func (p Phase) IsHookPhase() bool {
	hookPhases := map[Phase]bool{
		PhasePreinst:  true,
		PhasePostinst: true,
		PhasePrerem:   true,
		PhasePostrm:   true,
	}
	return hookPhases[p]
}

// IsPretendPhase returns true if this is the pretend phase.
func (p Phase) IsPretendPhase() bool {
	return p == PhasePretend
}

// IsConfigPhase returns true if this is the config phase.
func (p Phase) IsConfigPhase() bool {
	return p == PhaseConfig
}

// IsInfoPhase returns true if this is the info phase.
func (p Phase) IsInfoPhase() bool {
	return p == PhaseInfo
}

// IsNofetchPhase returns true if this is the nofetch phase.
func (p Phase) IsNofetchPhase() bool {
	return p == PhaseNofetch
}

// IsOutOfSequencePhase returns true if this phase runs outside normal build sequence.
// Per PMS Section 9.2: pkg_config, pkg_info, and pkg_nofetch are not called in normal sequence.
// pkg_pretend is called before the normal sequence (before fetch).
func (p Phase) IsOutOfSequencePhase() bool {
	outOfSequencePhases := map[Phase]bool{
		PhasePretend: true,
		PhaseConfig:  true,
		PhaseInfo:    true,
		PhaseNofetch: true,
	}
	return outOfSequencePhases[p]
}

// IsReadOnlyPhase returns true if this phase must not write to the filesystem.
// Per PMS Section 9.1.2, 9.1.15, 9.1.16: pkg_pretend, pkg_info, and pkg_nofetch
// must not modify the filesystem.
func (p Phase) IsReadOnlyPhase() bool {
	readOnlyPhases := map[Phase]bool{
		PhasePretend: true,
		PhaseInfo:    true,
		PhaseNofetch: true,
	}
	return readOnlyPhases[p]
}
