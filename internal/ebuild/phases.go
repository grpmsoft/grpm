package ebuild

// Phase represents an ebuild execution phase.
type Phase string

const (
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
