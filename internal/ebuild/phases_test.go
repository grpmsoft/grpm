package ebuild

import (
	"testing"
)

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PhaseSetup, "setup"},
		{PhaseUnpack, "unpack"},
		{PhasePrepare, "prepare"},
		{PhaseConfigure, "configure"},
		{PhaseCompile, "compile"},
		{PhaseTest, "test"},
		{PhaseInstall, "install"},
		{PhasePreinst, "preinst"},
		{PhasePostinst, "postinst"},
		{PhasePrerem, "prerm"},
		{PhasePostrm, "postrm"},
	}

	for _, tt := range tests {
		if tt.phase.String() != tt.expected {
			t.Errorf("Phase.String() = %s, expected %s", tt.phase.String(), tt.expected)
		}
	}
}

func TestStandardPhases(t *testing.T) {
	phases := StandardPhases()

	if len(phases) != 7 {
		t.Errorf("StandardPhases() returned %d phases, expected 7", len(phases))
	}

	// Check order
	expected := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
	}

	for i, phase := range phases {
		if phase != expected[i] {
			t.Errorf("StandardPhases()[%d] = %s, expected %s", i, phase, expected[i])
		}
	}
}

func TestPhaseIsTestPhase(t *testing.T) {
	if !PhaseTest.IsTestPhase() {
		t.Error("PhaseTest.IsTestPhase() should return true")
	}

	if PhaseCompile.IsTestPhase() {
		t.Error("PhaseCompile.IsTestPhase() should return false")
	}
}

func TestPhaseIsBuildPhase(t *testing.T) {
	buildPhases := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseInstall,
	}

	for _, phase := range buildPhases {
		if !phase.IsBuildPhase() {
			t.Errorf("Phase %s should be a build phase", phase)
		}
	}

	// Hook phases should not be build phases
	if PhasePreinst.IsBuildPhase() {
		t.Error("PhasePreinst should not be a build phase")
	}
}

func TestPhaseIsHookPhase(t *testing.T) {
	hookPhases := []Phase{
		PhasePreinst,
		PhasePostinst,
		PhasePrerem,
		PhasePostrm,
	}

	for _, phase := range hookPhases {
		if !phase.IsHookPhase() {
			t.Errorf("Phase %s should be a hook phase", phase)
		}
	}

	// Build phases should not be hook phases
	if PhaseCompile.IsHookPhase() {
		t.Error("PhaseCompile should not be a hook phase")
	}
}

func BenchmarkStandardPhases(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = StandardPhases()
	}
}
