package distfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// --- filterSignatureFiles tests ---

func TestFilterSignatureFiles_RemovesSigWhenVerifySigDisabled(t *testing.T) {
	distfiles := []fetch.Distfile{
		{Filename: "findutils-4.10.0.tar.xz", Size: 2189412},
		{Filename: "findutils-4.10.0.tar.xz.sig", Size: 119},
	}
	activeFlags := map[string]bool{} // verify-sig not enabled

	result := filterSignatureFiles(distfiles, activeFlags)

	if len(result) != 1 {
		t.Fatalf("expected 1 distfile, got %d: %v", len(result), filenames(result))
	}
	if result[0].Filename != "findutils-4.10.0.tar.xz" {
		t.Errorf("expected findutils tarball, got %s", result[0].Filename)
	}
}

func TestFilterSignatureFiles_KeepsSigWhenVerifySigEnabled(t *testing.T) {
	distfiles := []fetch.Distfile{
		{Filename: "findutils-4.10.0.tar.xz", Size: 2189412},
		{Filename: "findutils-4.10.0.tar.xz.sig", Size: 119},
	}
	activeFlags := map[string]bool{"verify-sig": true}

	result := filterSignatureFiles(distfiles, activeFlags)

	if len(result) != 2 {
		t.Fatalf("expected 2 distfiles when verify-sig enabled, got %d", len(result))
	}
}

func TestFilterSignatureFiles_AllExtensions(t *testing.T) {
	distfiles := []fetch.Distfile{
		{Filename: "pkg-1.0.tar.gz", Size: 1000},
		{Filename: "pkg-1.0.tar.gz.sig", Size: 100},
		{Filename: "pkg-1.0.tar.gz.asc", Size: 100},
		{Filename: "pkg-1.0.tar.gz.sign", Size: 100},
	}
	activeFlags := map[string]bool{}

	result := filterSignatureFiles(distfiles, activeFlags)

	if len(result) != 1 {
		t.Fatalf("expected 1 distfile after filtering all sig types, got %d: %v",
			len(result), filenames(result))
	}
	if result[0].Filename != "pkg-1.0.tar.gz" {
		t.Errorf("expected tarball, got %s", result[0].Filename)
	}
}

func TestFilterSignatureFiles_NilActiveFlags(t *testing.T) {
	// nil activeFlags means "include everything" — signature files should
	// still be filtered because nil means we don't know the USE state,
	// which is the conservative choice.
	distfiles := []fetch.Distfile{
		{Filename: "pkg-1.0.tar.gz", Size: 1000},
		{Filename: "pkg-1.0.tar.gz.sig", Size: 100},
	}

	result := filterSignatureFiles(distfiles, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 distfile with nil flags, got %d", len(result))
	}
}

func TestFilterSignatureFiles_NoSignatureFiles(t *testing.T) {
	distfiles := []fetch.Distfile{
		{Filename: "zlib-1.3.1.tar.xz", Size: 1234},
		{Filename: "zlib-1.3.1-patches.tar.xz", Size: 567},
	}
	activeFlags := map[string]bool{}

	result := filterSignatureFiles(distfiles, activeFlags)

	if len(result) != 2 {
		t.Fatalf("expected 2 distfiles (no sigs to filter), got %d", len(result))
	}
}

// --- isSignatureFile tests ---

func TestIsSignatureFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"findutils-4.10.0.tar.xz.sig", true},
		{"sed-4.9.tar.xz.sig", true},
		{"pkg-1.0.tar.gz.asc", true},
		{"pkg-1.0.tar.gz.sign", true},
		{"pkg-1.0.tar.gz.SIG", true},  // case insensitive
		{"pkg-1.0.tar.gz.ASC", true},  // case insensitive
		{"pkg-1.0.tar.gz", false},
		{"signal-desktop-1.0.tar.gz", false}, // "sig" in name but not extension
		{"design-1.0.tar.gz", false},         // "sign" in name but not extension
		{"zlib-1.3.1.tar.xz", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := isSignatureFile(tt.filename); got != tt.want {
				t.Errorf("isSignatureFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// --- extractRawSrcURI tests ---

func TestExtractRawSrcURI_SimpleAssignment(t *testing.T) {
	dir := t.TempDir()
	ebuild := filepath.Join(dir, "test-1.0.ebuild")
	content := `EAPI=8
DESCRIPTION="Test"
SRC_URI="https://example.com/${P}.tar.gz"
SLOT="0"
`
	os.WriteFile(ebuild, []byte(content), 0644)

	vars := map[string]string{
		"P":  "test-1.0",
		"PN": "test",
		"PV": "1.0",
	}

	result := extractRawSrcURI(ebuild, vars)

	if result != "https://example.com/test-1.0.tar.gz" {
		t.Errorf("expected expanded URL, got %q", result)
	}
}

func TestExtractRawSrcURI_WithAppendAssignment(t *testing.T) {
	dir := t.TempDir()
	ebuild := filepath.Join(dir, "test-1.0.ebuild")
	// Real-world pattern: SRC_URI + SRC_URI+= for verify-sig
	content := `EAPI=8
DESCRIPTION="Test"
SRC_URI="mirror://gnu/${PN}/${P}.tar.xz"
SRC_URI+=" verify-sig? ( mirror://gnu/${PN}/${P}.tar.xz.sig )"
SLOT="0"
`
	os.WriteFile(ebuild, []byte(content), 0644)

	vars := map[string]string{
		"P":  "findutils-4.10.0",
		"PN": "findutils",
		"PV": "4.10.0",
	}

	result := extractRawSrcURI(ebuild, vars)

	// Should contain both the tarball URL and the verify-sig conditional
	if result == "" {
		t.Fatal("expected non-empty SRC_URI")
	}
	if !contains(result, "findutils-4.10.0.tar.xz") {
		t.Errorf("expected tarball URL in result, got %q", result)
	}
	if !contains(result, "verify-sig?") {
		t.Errorf("expected verify-sig conditional preserved, got %q", result)
	}
}

func TestExtractRawSrcURI_MultiLineAssignment(t *testing.T) {
	dir := t.TempDir()
	ebuild := filepath.Join(dir, "test-1.0.ebuild")
	content := `EAPI=8
DESCRIPTION="Test"
SRC_URI="
	https://example.com/${P}.tar.gz
	https://mirror.example.com/${P}.tar.gz
"
SLOT="0"
`
	os.WriteFile(ebuild, []byte(content), 0644)

	vars := map[string]string{
		"P":  "test-1.0",
		"PN": "test",
		"PV": "1.0",
	}

	result := extractRawSrcURI(ebuild, vars)

	if !contains(result, "https://example.com/test-1.0.tar.gz") {
		t.Errorf("expected first URL, got %q", result)
	}
	if !contains(result, "https://mirror.example.com/test-1.0.tar.gz") {
		t.Errorf("expected second URL, got %q", result)
	}
}

func TestExtractRawSrcURI_NonexistentFile(t *testing.T) {
	result := extractRawSrcURI("/nonexistent/path.ebuild", nil)
	if result != "" {
		t.Errorf("expected empty for nonexistent file, got %q", result)
	}
}

func TestExtractRawSrcURI_NoSrcURI(t *testing.T) {
	dir := t.TempDir()
	ebuild := filepath.Join(dir, "test-1.0.ebuild")
	content := `EAPI=8
DESCRIPTION="Virtual package"
SLOT="0"
`
	os.WriteFile(ebuild, []byte(content), 0644)

	result := extractRawSrcURI(ebuild, nil)
	if result != "" {
		t.Errorf("expected empty for no SRC_URI, got %q", result)
	}
}

// --- ResolveDistfiles integration tests ---

type mockEvaluator struct {
	srcURI string
	err    error
}

func (m *mockEvaluator) EvaluateSrcURI(_ context.Context, _, _ string, _ *pkg.Package) (string, error) {
	return m.srcURI, m.err
}

func TestResolveDistfiles_FiltersSigFromEvaluatedSrcURI(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "eclass"), 0755)
	pkgDir := filepath.Join(dir, "sys-apps", "findutils")
	os.MkdirAll(pkgDir, 0755)

	// Create ebuild
	ebuildPath := filepath.Join(pkgDir, "findutils-4.10.0.ebuild")
	os.WriteFile(ebuildPath, []byte(`EAPI=8
SRC_URI="mirror://gnu/${PN}/${P}.tar.xz verify-sig? ( mirror://gnu/${PN}/${P}.tar.xz.sig )"
`), 0644)

	// Create manifest with both files
	manifest := &fetch.Manifest{
		DistFiles: []fetch.ManifestEntry{
			{Filename: "findutils-4.10.0.tar.xz", Size: 2189412},
			{Filename: "findutils-4.10.0.tar.xz.sig", Size: 119},
		},
	}

	// Mock evaluator returns SRC_URI with verify-sig conditional
	eval := &mockEvaluator{
		srcURI: "mirror://gnu/findutils/findutils-4.10.0.tar.xz verify-sig? ( mirror://gnu/findutils/findutils-4.10.0.tar.xz.sig )",
	}

	svc := &Service{repoPath: dir, evaluator: eval}
	pkgInfo := &pkg.Package{
		Name:     "sys-apps/findutils",
		Version:  "4.10.0",
		Slot:     pkg.NewSlot("0", ""),
		UseFlags: map[string]bool{}, // verify-sig NOT enabled
	}

	distfiles, err := svc.ResolveDistfiles(context.Background(), pkgInfo, ebuildPath, manifest)
	if err != nil {
		t.Fatalf("ResolveDistfiles error: %v", err)
	}

	if len(distfiles) != 1 {
		t.Fatalf("expected 1 distfile (no .sig), got %d: %v", len(distfiles), filenames(distfilesToDistfiles(distfiles)))
	}
	if distfiles[0].Filename != "findutils-4.10.0.tar.xz" {
		t.Errorf("expected tarball, got %s", distfiles[0].Filename)
	}
}

func TestResolveDistfiles_FallbackToRawExtraction(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "eclass"), 0755)
	pkgDir := filepath.Join(dir, "sys-apps", "sed")
	os.MkdirAll(pkgDir, 0755)

	// Create ebuild with SRC_URI and SRC_URI+=
	ebuildPath := filepath.Join(pkgDir, "sed-4.9.ebuild")
	os.WriteFile(ebuildPath, []byte(`EAPI=8
SRC_URI="mirror://gnu/sed/${P}.tar.xz"
SRC_URI+=" verify-sig? ( mirror://gnu/sed/${P}.tar.xz.sig )"
`), 0644)

	manifest := &fetch.Manifest{
		DistFiles: []fetch.ManifestEntry{
			{Filename: "sed-4.9.tar.xz", Size: 1000},
			{Filename: "sed-4.9.tar.xz.sig", Size: 100},
		},
	}

	// Evaluator fails — triggers raw extraction fallback
	eval := &mockEvaluator{err: fmt.Errorf("eclass parse error")}
	svc := &Service{repoPath: dir, evaluator: eval}

	pkgInfo := &pkg.Package{
		Name:     "sys-apps/sed",
		Version:  "4.9",
		Slot:     pkg.NewSlot("0", ""),
		UseFlags: map[string]bool{},
	}

	distfiles, err := svc.ResolveDistfiles(context.Background(), pkgInfo, ebuildPath, manifest)
	if err != nil {
		t.Fatalf("ResolveDistfiles error: %v", err)
	}

	// Raw extraction should get SRC_URI with verify-sig conditional,
	// ParseSrcURI should filter out .sig
	if len(distfiles) != 1 {
		t.Fatalf("expected 1 distfile after raw fallback, got %d: %v",
			len(distfiles), filenames(distfilesToDistfiles(distfiles)))
	}
}

func TestResolveDistfiles_FallbackToFilteredManifest(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "eclass"), 0755)
	pkgDir := filepath.Join(dir, "dev-build", "make")
	os.MkdirAll(pkgDir, 0755)

	// Create ebuild with conditional SRC_URI (not extractable by raw method)
	ebuildPath := filepath.Join(pkgDir, "make-4.4.1.ebuild")
	os.WriteFile(ebuildPath, []byte(`EAPI=8
DESCRIPTION="GNU make"
if [[ $(ver_cut 3) -ge 90 ]]; then
SRC_URI="mirror://gnu/make/${P}.tar.lz"
else
SRC_URI="mirror://gnu/make/${P}.tar.lz"
fi
SRC_URI+=" verify-sig? ( mirror://gnu/make/${P}.tar.lz.sig )"
`), 0644)

	manifest := &fetch.Manifest{
		DistFiles: []fetch.ManifestEntry{
			{Filename: "make-4.4.1.tar.lz", Size: 1000},
			{Filename: "make-4.4.1.tar.lz.sig", Size: 100},
		},
	}

	// Evaluator fails AND raw extraction should pick up SRC_URI+=
	eval := &mockEvaluator{err: fmt.Errorf("@a param expansion")}
	svc := &Service{repoPath: dir, evaluator: eval}

	pkgInfo := &pkg.Package{
		Name:     "dev-build/make",
		Version:  "4.4.1",
		Slot:     pkg.NewSlot("0", ""),
		UseFlags: map[string]bool{},
	}

	distfiles, err := svc.ResolveDistfiles(context.Background(), pkgInfo, ebuildPath, manifest)
	if err != nil {
		t.Fatalf("ResolveDistfiles error: %v", err)
	}

	// Should NOT contain .sig file
	for _, df := range distfiles {
		if isSignatureFile(df.Filename) {
			t.Errorf("unexpected signature file in result: %s", df.Filename)
		}
	}
}

func TestResolveDistfiles_NoEbuildPath(t *testing.T) {
	manifest := &fetch.Manifest{
		DistFiles: []fetch.ManifestEntry{
			{Filename: "pkg-1.0.tar.gz", Size: 1000},
			{Filename: "pkg-1.0.tar.gz.sig", Size: 100},
		},
	}

	svc := &Service{repoPath: "/tmp"}
	pkgInfo := &pkg.Package{
		Name:     "test/pkg",
		Version:  "1.0",
		Slot:     pkg.NewSlot("0", ""),
		UseFlags: map[string]bool{},
	}

	distfiles, err := svc.ResolveDistfiles(context.Background(), pkgInfo, "", manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even without ebuild path, .sig should be filtered
	for _, df := range distfiles {
		if isSignatureFile(df.Filename) {
			t.Errorf("unexpected signature file: %s", df.Filename)
		}
	}
}

// --- extractQuotedVariable tests ---

func TestExtractQuotedVariable(t *testing.T) {
	tests := []struct {
		name    string
		content string
		varName string
		want    string
	}{
		{
			name:    "simple single-line",
			content: `SRC_URI="https://example.com/pkg-1.0.tar.gz"`,
			varName: "SRC_URI",
			want:    "https://example.com/pkg-1.0.tar.gz",
		},
		{
			name: "multi-line",
			content: `SRC_URI="
	https://example.com/a.tar.gz
	https://example.com/b.tar.gz
"`,
			varName: "SRC_URI",
			want:    "\n\thttps://example.com/a.tar.gz\n\thttps://example.com/b.tar.gz\n",
		},
		{
			name:    "not found",
			content: `DEPEND="dev-libs/foo"`,
			varName: "SRC_URI",
			want:    "",
		},
		{
			name:    "avoids partial match",
			content: `MY_SRC_URI="bad"` + "\n" + `SRC_URI="good"`,
			varName: "SRC_URI",
			want:    "good",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQuotedVariable(tt.content, tt.varName)
			if got != tt.want {
				t.Errorf("extractQuotedVariable() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- expandVars tests ---

func TestExpandVars(t *testing.T) {
	vars := map[string]string{
		"P":  "findutils-4.10.0",
		"PN": "findutils",
		"PV": "4.10.0",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"${P}.tar.gz", "findutils-4.10.0.tar.gz"},
		{"${PN}/${P}.tar.xz", "findutils/findutils-4.10.0.tar.xz"},
		{"mirror://gnu/${PN}/${P}.tar.xz", "mirror://gnu/findutils/findutils-4.10.0.tar.xz"},
	}

	for _, tt := range tests {
		got := expandVars(tt.input, vars)
		if got != tt.want {
			t.Errorf("expandVars(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- helpers ---

func filenames(distfiles []fetch.Distfile) []string {
	var names []string
	for _, df := range distfiles {
		names = append(names, df.Filename)
	}
	return names
}

func distfilesToDistfiles(distfiles []fetch.Distfile) []fetch.Distfile {
	return distfiles
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
