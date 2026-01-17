//go:build integration
// +build integration

package metadata

import (
	"context"
	"os"
	"testing"
)

// TestEvaluator_RealGCC tests metadata extraction from real gcc ebuild.
// Run with: go test -tags=integration -v ./internal/metadata/...
func TestEvaluator_RealGCC(t *testing.T) {
	repoPath := "/var/db/repos/gentoo"
	if _, err := os.Stat(repoPath); err != nil {
		t.Skipf("Gentoo repo not available: %v", err)
	}

	ebuildPath := repoPath + "/sys-devel/gcc/gcc-13.4.1_p20250807.ebuild"
	if _, err := os.Stat(ebuildPath); err != nil {
		// Try latest available gcc
		t.Skipf("gcc ebuild not found: %v", err)
	}

	eval, err := NewEvaluator(repoPath)
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}
	eval.Verbose = true

	pkgInfo := &PackageInfo{
		Name:    "sys-devel/gcc",
		Version: "13.4.1_p20250807",
		Slot:    "13",
	}

	ctx := context.Background()
	result, err := eval.ExtractMetadata(ctx, ebuildPath, pkgInfo, []string{
		"RDEPEND", "BDEPEND", "DEPEND", "SRC_URI", "INHERITED",
	})
	if err != nil {
		t.Logf("ExtractMetadata error (may be expected): %v", err)
	}

	t.Logf("Results:")
	for k, v := range result {
		if len(v) > 200 {
			t.Logf("  %s: (len=%d) %s...", k, len(v), v[:200])
		} else {
			t.Logf("  %s: %s", k, v)
		}
	}

	// Check critical values
	if result["RDEPEND"] == "" {
		t.Error("RDEPEND is empty - eclass inheritance likely failed")
	}
	if result["INHERITED"] == "" {
		t.Error("INHERITED is empty - inherit() not called")
	}
	if result["SRC_URI"] == "" {
		t.Error("SRC_URI is empty")
	}
}
