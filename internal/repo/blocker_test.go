package repo

import (
	"testing"
)

// TestParseBlockers tests that blockers are correctly identified and flagged
func TestParseBlockers(t *testing.T) {
	tests := []struct {
		name          string
		ebuildContent string
		wantBlockers  int
		wantDeps      int
	}{
		{
			name: "single blocker",
			ebuildContent: `RDEPEND="!sys-libs/zlib-ng[compat]"
DEPEND="${RDEPEND}"`,
			wantBlockers: 2, // RDEPEND + DEPEND (both reference same blocker)
			wantDeps:     0,
		},
		{
			name: "hard blocker",
			ebuildContent: `RDEPEND="!!sys-libs/old-package"
DEPEND="${RDEPEND}"`,
			wantBlockers: 2,
			wantDeps:     0,
		},
		{
			name: "blocker with regular dependency",
			ebuildContent: `RDEPEND="
	!sys-libs/zlib-ng[compat]
	>=sys-libs/glibc-2.17
"
DEPEND="${RDEPEND}"`,
			wantBlockers: 2, // !zlib-ng appears twice (RDEPEND + DEPEND)
			wantDeps:     2, // glibc appears twice
		},
		{
			name: "no blockers",
			ebuildContent: `RDEPEND="
	>=sys-libs/glibc-2.17
	dev-libs/libffi
"
DEPEND="${RDEPEND}"`,
			wantBlockers: 0,
			wantDeps:     4, // 2 deps × 2 (RDEPEND + DEPEND)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewEbuildParser(tt.ebuildContent)
			deps, err := parser.ParseDependencies()
			if err != nil {
				t.Fatalf("ParseDependencies() error = %v", err)
			}

			blockerCount := 0
			depCount := 0

			for _, dep := range deps {
				if dep.IsBlocker {
					blockerCount++
				} else {
					depCount++
				}
			}

			if blockerCount != tt.wantBlockers {
				t.Errorf("blocker count = %d, want %d", blockerCount, tt.wantBlockers)
			}

			if depCount != tt.wantDeps {
				t.Errorf("dependency count = %d, want %d", depCount, tt.wantDeps)
			}
		})
	}
}

// TestBlockerTypes tests soft vs hard blockers
func TestBlockerTypes(t *testing.T) {
	tests := []struct {
		name        string
		atom        string
		wantBlocker bool
		wantHard    bool
	}{
		{
			name:        "soft blocker",
			atom:        "!sys-libs/zlib-ng",
			wantBlocker: true,
			wantHard:    false,
		},
		{
			name:        "hard blocker",
			atom:        "!!sys-libs/old-package",
			wantBlocker: true,
			wantHard:    true,
		},
		{
			name:        "no blocker",
			atom:        "sys-libs/glibc",
			wantBlocker: false,
			wantHard:    false,
		},
		{
			name:        "blocker with version",
			atom:        "!>=sys-libs/zlib-1.2.13",
			wantBlocker: true,
			wantHard:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewEbuildParser("")
			dep, err := parser.parsePackageAtom(tt.atom, DepTypeRuntime, "", 0)
			if err != nil {
				t.Fatalf("parsePackageAtom() error = %v", err)
			}

			if dep.IsBlocker != tt.wantBlocker {
				t.Errorf("IsBlocker = %v, want %v", dep.IsBlocker, tt.wantBlocker)
			}

			if dep.IsHardBlock != tt.wantHard {
				t.Errorf("IsHardBlock = %v, want %v", dep.IsHardBlock, tt.wantHard)
			}
		})
	}
}

// TestBlockersNotAddedToDeps tests that portage.go skips blockers when building Package.Deps
func TestBlockersNotAddedToDeps(t *testing.T) {
	// This is an integration test - it would need a real ebuild file
	// For now, this is a placeholder to remind us to test this at integration level
	t.Skip("Integration test - requires real ebuild parsing")

	// Test logic:
	// 1. Create a test ebuild with both blockers and regular deps
	// 2. Parse it with portage.parseEbuild()
	// 3. Verify that Package.Deps contains only regular deps (no blockers)
	// 4. Verify that blockers were logged/skipped
}
