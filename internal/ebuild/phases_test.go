package ebuild

import (
	"testing"
)

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PhasePretend, "pretend"},
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
		{PhaseConfig, "config"},
		{PhaseInfo, "info"},
		{PhaseNofetch, "nofetch"},
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

func TestPhaseIsPretendPhase(t *testing.T) {
	if !PhasePretend.IsPretendPhase() {
		t.Error("PhasePretend.IsPretendPhase() should return true")
	}

	if PhaseSetup.IsPretendPhase() {
		t.Error("PhaseSetup.IsPretendPhase() should return false")
	}
}

func TestPhaseIsConfigPhase(t *testing.T) {
	if !PhaseConfig.IsConfigPhase() {
		t.Error("PhaseConfig.IsConfigPhase() should return true")
	}

	if PhaseInstall.IsConfigPhase() {
		t.Error("PhaseInstall.IsConfigPhase() should return false")
	}
}

func TestPhaseIsInfoPhase(t *testing.T) {
	if !PhaseInfo.IsInfoPhase() {
		t.Error("PhaseInfo.IsInfoPhase() should return true")
	}

	if PhaseSetup.IsInfoPhase() {
		t.Error("PhaseSetup.IsInfoPhase() should return false")
	}
}

func TestPhaseIsNofetchPhase(t *testing.T) {
	if !PhaseNofetch.IsNofetchPhase() {
		t.Error("PhaseNofetch.IsNofetchPhase() should return true")
	}

	if PhaseFetch.IsNofetchPhase() {
		t.Error("PhaseFetch.IsNofetchPhase() should return false")
	}
}

func TestPhaseIsOutOfSequencePhase(t *testing.T) {
	outOfSequencePhases := []Phase{
		PhasePretend,
		PhaseConfig,
		PhaseInfo,
		PhaseNofetch,
	}

	for _, phase := range outOfSequencePhases {
		if !phase.IsOutOfSequencePhase() {
			t.Errorf("Phase %s should be out of sequence", phase)
		}
	}

	// Normal phases should not be out of sequence
	normalPhases := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
	}

	for _, phase := range normalPhases {
		if phase.IsOutOfSequencePhase() {
			t.Errorf("Phase %s should not be out of sequence", phase)
		}
	}
}

func TestPhaseIsReadOnlyPhase(t *testing.T) {
	readOnlyPhases := []Phase{
		PhasePretend,
		PhaseInfo,
		PhaseNofetch,
	}

	for _, phase := range readOnlyPhases {
		if !phase.IsReadOnlyPhase() {
			t.Errorf("Phase %s should be read-only", phase)
		}
	}

	// Phases that can write
	writablePhases := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseInstall,
		PhaseConfig,
	}

	for _, phase := range writablePhases {
		if phase.IsReadOnlyPhase() {
			t.Errorf("Phase %s should not be read-only", phase)
		}
	}
}

func BenchmarkStandardPhases(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = StandardPhases()
	}
}
