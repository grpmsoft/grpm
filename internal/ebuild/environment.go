package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// Environment holds ebuild execution environment variables.
//
// Portage Environment Variables Reference:
// https://dev.gentoo.org/~zmedico/portage/doc/man/ebuild.5.html
// PMS Reference: https://projects.gentoo.org/pms/latest/pms.html#the-ebuild-environment
type Environment struct {
	// Package metadata
	Package *pkg.Package

	// P = package-version (e.g., "zlib-1.2.13")
	P string

	// PN = package name without category (e.g., "zlib")
	PN string

	// PV = package version (e.g., "1.2.13")
	PV string

	// PR = package revision (e.g., "r1" or "r0")
	PR string

	// PVR = package version and revision (e.g., "1.2.13" or "1.2.13-r1")
	// Per PMS 11.1: ${PV}-${PR} if PR != r0, otherwise ${PV}
	PVR string

	// PF = full package name: PN-PVR (e.g., "zlib-1.2.13-r1")
	PF string

	// CATEGORY = package category (e.g., "sys-libs")
	CATEGORY string

	// Directory paths

	// PORTDIR = portage tree directory (legacy, removed in EAPI 7+)
	PORTDIR string

	// DISTDIR = directory for downloaded sources
	DISTDIR string

	// PORTAGE_TMPDIR = temporary build directory
	PORTAGE_TMPDIR string

	// WORKDIR = work directory for building (${PORTAGE_TMPDIR}/portage/${CATEGORY}/${PF}/work)
	WORKDIR string

	// S = source directory (usually ${WORKDIR}/${P})
	S string

	// D = installation image directory (${PORTAGE_TMPDIR}/portage/${CATEGORY}/${PF}/image)
	// Per PMS 11.1.4: EAPI 0-6 always ends with '/', EAPI 7+ never has trailing slash
	D string

	// ED = ${D}${EPREFIX} (installation directory with EPREFIX)
	// Per PMS 11.1: Available in EAPI 3+
	// Per PMS 11.1.4: EAPI 0-6 always ends with '/', EAPI 7+ never has trailing slash
	ED string

	// T = temporary directory (${PORTAGE_TMPDIR}/portage/${CATEGORY}/${PF}/temp)
	T string

	// HOME = temporary home directory (${T}/homedir)
	HOME string

	// ROOT = target filesystem root (usually "/")
	// Per PMS 11.1.4: EAPI 0-6 always ends with '/', EAPI 7+ never has trailing slash
	ROOT string

	// EROOT = ${ROOT}${EPREFIX} (target root with prefix)
	// Per PMS 11.1: Available in EAPI 3+
	// Per PMS 11.1.4: EAPI 0-6 always ends with '/', EAPI 7+ never has trailing slash
	EROOT string

	// EPREFIX = prefix offset for Gentoo Prefix installations
	// Per PMS 11.1: Available in EAPI 3+, usually empty unless Prefix installation
	EPREFIX string

	// SYSROOT = build dependencies root for cross-compilation
	// Per PMS 11.1: Available in EAPI 7+, same as ROOT for native builds
	SYSROOT string

	// ESYSROOT = ${SYSROOT}${EPREFIX}
	// Per PMS 11.1: Available in EAPI 7+
	ESYSROOT string

	// BROOT = CBUILD root for build dependencies
	// Per PMS 11.1: Available in EAPI 7+, typically "/" for native builds
	BROOT string

	// Build configuration

	// CFLAGS = C compiler flags
	CFLAGS string

	// CXXFLAGS = C++ compiler flags
	CXXFLAGS string

	// LDFLAGS = linker flags
	LDFLAGS string

	// MAKEOPTS = make parallel build options (e.g., "-j4")
	MAKEOPTS string

	// USE = enabled USE flags (space-separated)
	USE string

	// FEATURES = Portage features (e.g., "sandbox ccache parallel-fetch")
	FEATURES string

	// EAPI = EAPI version
	EAPI string

	// SLOT = package slot
	SLOT string

	// A = space-separated list of source archives to unpack
	// Populated by fetch phase from Manifest DIST entries
	A string

	// EBUILD_PHASE = current ebuild phase being executed
	// Set by Executor during phase dispatch
	EBUILD_PHASE string

	// FILESDIR = directory containing ebuild support files (patches, etc.)
	// Usually ${PORTDIR}/${CATEGORY}/${PN}/files
	FILESDIR string

	// EBUILD = path to the ebuild file being executed
	EBUILD string

	// EAPIFeatures holds the feature set for the current EAPI
	// Used for EAPI-specific variable availability checks
	EAPIFeatures pkg.EAPIFeatures

	// ExtraVars holds additional environment variables set by ebuilds.
	// This includes ebuild-specific variables like DOCS, HTML_DOCS, etc.
	ExtraVars map[string]string
}

// NewEnvironment creates a new ebuild execution environment.
//
// Parameters:
//   - p: Package to build
//   - tmpDir: Temporary build directory (e.g., /var/tmp/portage)
//   - portDir: Portage tree directory (e.g., /var/db/repos/gentoo)
//   - distDir: Distfiles directory (e.g., /var/cache/distfiles)
func NewEnvironment(p *pkg.Package, tmpDir, portDir, distDir string) (*Environment, error) {
	return NewEnvironmentWithEAPI(p, tmpDir, portDir, distDir, "8")
}

// NewEnvironmentWithEAPI creates a new ebuild execution environment with a specific EAPI.
//
// Parameters:
//   - p: Package to build
//   - tmpDir: Temporary build directory (e.g., /var/tmp/portage)
//   - portDir: Portage tree directory (e.g., /var/db/repos/gentoo)
//   - distDir: Distfiles directory (e.g., /var/cache/distfiles)
//   - eapi: EAPI version string (e.g., "8")
func NewEnvironmentWithEAPI(p *pkg.Package, tmpDir, portDir, distDir, eapi string) (*Environment, error) {
	if p == nil {
		return nil, fmt.Errorf("package is nil")
	}

	// Get EAPI features
	eapiFeatures, err := pkg.GetEAPIFeatures(eapi)
	if err != nil {
		// Fall back to EAPI 8 if unknown
		eapiFeatures = pkg.MustGetEAPIFeatures("8")
		eapi = "8"
	}

	// Parse package name: "category/package-name"
	parts := strings.Split(p.Name, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package name format: %s", p.Name)
	}

	category := parts[0]
	packageName := parts[1]

	// Extract revision from version (e.g., "1.2.13-r1" -> "r1")
	version := p.Version
	revision := "r0"
	if idx := strings.LastIndex(version, "-r"); idx != -1 {
		revision = version[idx+1:]
		version = version[:idx]
	}

	// Build standard variables
	pVar := fmt.Sprintf("%s-%s", packageName, version)

	// PVR = version-revision or just version if r0
	// Per PMS 11.1: PVR is ${PV}-${PR} if PR != r0, otherwise ${PV}
	pvr := computePVR(version, revision)

	// PF = PN-PVR
	pf := fmt.Sprintf("%s-%s", packageName, pvr)

	// Build directory structure
	// tmpDir is already /var/tmp/portage, so don't add "portage" again
	workBase := filepath.Join(tmpDir, category, pf)
	workDir := filepath.Join(workBase, "work")
	imageDir := filepath.Join(workBase, "image")
	tempDir := filepath.Join(workBase, "temp")
	sourceDir := filepath.Join(workDir, pVar)

	// Collect USE flags
	useFlags := make([]string, 0)
	for flag, enabled := range p.UseFlags {
		if enabled {
			useFlags = append(useFlags, flag)
		}
	}

	// FILESDIR = files directory for patches and support files
	filesDir := filepath.Join(portDir, category, packageName, "files")

	// ROOT defaults to "/" (target filesystem root)
	root := "/"

	// EPREFIX defaults to empty (no prefix installation)
	eprefix := ""

	// Handle trailing slash behavior per PMS 11.1.4
	// EAPI 0-6: ROOT, EROOT, D, ED always end with '/'
	// EAPI 7+: These variables never have trailing slashes
	d := imageDir
	ed := imageDir + eprefix
	rootVal := root
	erootVal := root + eprefix

	if eapiFeatures.HasTrailingSlash() {
		// EAPI 0-6: Ensure trailing slash
		d = ensureTrailingSlash(d)
		ed = ensureTrailingSlash(ed)
		rootVal = ensureTrailingSlash(rootVal)
		erootVal = ensureTrailingSlash(erootVal)
	} else {
		// EAPI 7+: Remove trailing slash
		d = removeTrailingSlash(d)
		ed = removeTrailingSlash(ed)
		rootVal = removeTrailingSlash(rootVal)
		erootVal = removeTrailingSlash(erootVal)
	}

	// SYSROOT, ESYSROOT, BROOT for cross-compilation (EAPI 7+)
	// For native builds, SYSROOT = ROOT and BROOT = "/"
	sysroot := rootVal
	esysroot := erootVal
	broot := "/"
	if !eapiFeatures.HasTrailingSlash() {
		broot = "" // EAPI 7+: empty string represents root
	}

	return &Environment{
		Package:  p,
		P:        pVar,
		PN:       packageName,
		PV:       version,
		PR:       revision,
		PVR:      pvr,
		PF:       pf,
		CATEGORY: category,

		PORTDIR:        portDir,
		DISTDIR:        distDir,
		PORTAGE_TMPDIR: tmpDir,
		WORKDIR:        workDir,
		S:              sourceDir,
		D:              d,
		ED:             ed,
		T:              tempDir,
		HOME:           filepath.Join(tempDir, "homedir"),
		FILESDIR:       filesDir,

		ROOT:     rootVal,
		EROOT:    erootVal,
		EPREFIX:  eprefix,
		SYSROOT:  sysroot,
		ESYSROOT: esysroot,
		BROOT:    broot,

		CFLAGS:   os.Getenv("CFLAGS"),
		CXXFLAGS: os.Getenv("CXXFLAGS"),
		LDFLAGS:  os.Getenv("LDFLAGS"),
		MAKEOPTS: os.Getenv("MAKEOPTS"),
		USE:      strings.Join(useFlags, " "),
		FEATURES: "sandbox userpriv usersandbox",
		EAPI:     eapi,
		SLOT:     p.Slot.String(),

		EAPIFeatures: eapiFeatures,
	}, nil
}

// computePVR computes the PVR variable from version and revision.
// Per PMS 11.1: PVR is ${PV}-${PR} if PR != r0, otherwise ${PV}.
func computePVR(version, revision string) string {
	if revision == "r0" || revision == "" {
		return version
	}
	return fmt.Sprintf("%s-%s", version, revision)
}

// ensureTrailingSlash adds a trailing slash if not present.
func ensureTrailingSlash(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

// removeTrailingSlash removes trailing slash, returns empty string for root.
func removeTrailingSlash(path string) string {
	if path == "/" {
		return ""
	}
	return strings.TrimSuffix(path, "/")
}

// ToMap converts environment to map[string]string for exec.Cmd.Env.
// Variables are exported based on EAPI-specific availability per PMS 11.1.
func (env *Environment) ToMap() map[string]string {
	result := map[string]string{
		// Always available (all EAPIs)
		"P":              env.P,
		"PN":             env.PN,
		"PV":             env.PV,
		"PR":             env.PR,
		"PVR":            env.PVR,
		"PF":             env.PF,
		"CATEGORY":       env.CATEGORY,
		"DISTDIR":        env.DISTDIR,
		"PORTAGE_TMPDIR": env.PORTAGE_TMPDIR,
		"WORKDIR":        env.WORKDIR,
		"S":              env.S,
		"D":              env.D,
		"T":              env.T,
		"HOME":           env.HOME,
		"CFLAGS":         env.CFLAGS,
		"CXXFLAGS":       env.CXXFLAGS,
		"LDFLAGS":        env.LDFLAGS,
		"MAKEOPTS":       env.MAKEOPTS,
		"USE":            env.USE,
		"FEATURES":       env.FEATURES,
		"EAPI":           env.EAPI,
		"SLOT":           env.SLOT,
		"A":              env.A,
		"EBUILD_PHASE":   env.EBUILD_PHASE,
		"FILESDIR":       env.FILESDIR,
		"EBUILD":         env.EBUILD,
		"ROOT":           env.ROOT,
	}

	// PORTDIR: Available in EAPI 0-6, removed in EAPI 7+
	// Per PMS Table 11.4
	if !env.EAPIFeatures.SupportsBROOT() {
		result["PORTDIR"] = env.PORTDIR
	}

	// Offset-prefix variables: Available in EAPI 3+
	// Per PMS Table 11.5
	if env.EAPIFeatures.SupportsOffsetPrefix() {
		result["EPREFIX"] = env.EPREFIX
		result["EROOT"] = env.EROOT
		result["ED"] = env.ED
	}

	// Cross-compilation variables: Available in EAPI 7+
	// Per PMS Table 11.3 and 11.5
	if env.EAPIFeatures.SupportsSYSROOT() {
		result["SYSROOT"] = env.SYSROOT
	}
	if env.EAPIFeatures.SupportsBROOT() {
		result["BROOT"] = env.BROOT
	}
	// ESYSROOT: Available in EAPI 7+ (per PMS Table 11.5)
	if env.EAPIFeatures.SupportsSYSROOT() && env.EAPIFeatures.SupportsOffsetPrefix() {
		result["ESYSROOT"] = env.ESYSROOT
	}

	// Include ExtraVars (ebuild-specific and metadata extraction variables)
	for k, v := range env.ExtraVars {
		result[k] = v
	}

	return result
}

// ToSlice converts environment to []string format for exec.Cmd.Env.
func (env *Environment) ToSlice() []string {
	envMap := env.ToMap()
	result := make([]string, 0, len(envMap))

	for key, value := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}

	// Add PATH from current environment
	if path := os.Getenv("PATH"); path != "" {
		result = append(result, "PATH="+path)
	}

	return result
}

// CreateDirectories creates all necessary build directories.
func (env *Environment) CreateDirectories() error {
	dirs := []string{
		env.WORKDIR,
		env.S,
		env.D,
		env.T,
		env.HOME,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Cleanup removes temporary build directories.
func (env *Environment) Cleanup() error {
	// Remove work base directory
	workBase := filepath.Dir(env.WORKDIR)
	return os.RemoveAll(workBase)
}

// GetVar retrieves an environment variable by name.
//
// First checks built-in variables (P, PN, PV, etc.), then checks ExtraVars.
// Returns empty string if not found.
func (env *Environment) GetVar(name string) string {
	// Use ToMap to get all built-in variables
	builtIn := env.ToMap()
	if val, ok := builtIn[name]; ok {
		return val
	}

	// Check ExtraVars
	if env.ExtraVars != nil {
		if val, ok := env.ExtraVars[name]; ok {
			return val
		}
	}

	return ""
}

// SetVar sets an extra environment variable.
//
// Built-in variables cannot be set via this method.
// Use this for ebuild-specific variables like DOCS, HTML_DOCS, etc.
func (env *Environment) SetVar(name, value string) {
	if env.ExtraVars == nil {
		env.ExtraVars = make(map[string]string)
	}
	env.ExtraVars[name] = value
}
