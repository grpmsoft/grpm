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

	// PF = full package name: PN-PVR (e.g., "zlib-1.2.13-r1")
	PF string

	// CATEGORY = package category (e.g., "sys-libs")
	CATEGORY string

	// Directory paths

	// PORTDIR = portage tree directory
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
	D string

	// ED = ${D%/}${EPREFIX%/}/ (installation directory with EPREFIX)
	ED string

	// T = temporary directory (${PORTAGE_TMPDIR}/portage/${CATEGORY}/${PF}/temp)
	T string

	// HOME = temporary home directory (${T}/homedir)
	HOME string

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

	// ExtraVars holds additional environment variables set by ebuilds.
	// This includes ebuild-specific variables like DOCS, HTML_DOCS, etc.
	ExtraVars map[string]string
}

// NewEnvironment creates a new ebuild execution environment.
//
// Parameters:
//   - pkg: Package to build
//   - tmpDir: Temporary build directory (e.g., /var/tmp/portage)
//   - portDir: Portage tree directory (e.g., /var/db/repos/gentoo)
//   - distDir: Distfiles directory (e.g., /var/cache/distfiles)
func NewEnvironment(pkg *pkg.Package, tmpDir, portDir, distDir string) (*Environment, error) {
	if pkg == nil {
		return nil, fmt.Errorf("package is nil")
	}

	// Parse package name: "category/package-name"
	parts := strings.Split(pkg.Name, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package name format: %s", pkg.Name)
	}

	category := parts[0]
	packageName := parts[1]

	// Extract revision from version (e.g., "1.2.13-r1" -> "r1")
	version := pkg.Version
	revision := "r0"
	if idx := strings.LastIndex(version, "-r"); idx != -1 {
		revision = version[idx+1:]
		version = version[:idx]
	}

	// Build standard variables
	p := fmt.Sprintf("%s-%s", packageName, version)
	pf := p
	if revision != "r0" {
		pf = fmt.Sprintf("%s-%s", p, revision)
	}

	// Build directory structure
	// tmpDir is already /var/tmp/portage, so don't add "portage" again
	workBase := filepath.Join(tmpDir, category, pf)
	workDir := filepath.Join(workBase, "work")
	imageDir := filepath.Join(workBase, "image")
	tempDir := filepath.Join(workBase, "temp")
	sourceDir := filepath.Join(workDir, p)

	// Collect USE flags
	useFlags := make([]string, 0)
	for flag, enabled := range pkg.UseFlags {
		if enabled {
			useFlags = append(useFlags, flag)
		}
	}

	// FILESDIR = files directory for patches and support files
	filesDir := filepath.Join(portDir, category, packageName, "files")

	return &Environment{
		Package:  pkg,
		P:        p,
		PN:       packageName,
		PV:       version,
		PR:       revision,
		PF:       pf,
		CATEGORY: category,

		PORTDIR:        portDir,
		DISTDIR:        distDir,
		PORTAGE_TMPDIR: tmpDir,
		WORKDIR:        workDir,
		S:              sourceDir,
		D:              imageDir,
		ED:             imageDir + "/",
		T:              tempDir,
		HOME:           filepath.Join(tempDir, "homedir"),
		FILESDIR:       filesDir,

		CFLAGS:   os.Getenv("CFLAGS"),
		CXXFLAGS: os.Getenv("CXXFLAGS"),
		LDFLAGS:  os.Getenv("LDFLAGS"),
		MAKEOPTS: os.Getenv("MAKEOPTS"),
		USE:      strings.Join(useFlags, " "),
		FEATURES: "sandbox userpriv usersandbox",
		EAPI:     "8", // Default to EAPI 8
		SLOT:     pkg.Slot.String(),
	}, nil
}

// ToMap converts environment to map[string]string for exec.Cmd.Env.
func (env *Environment) ToMap() map[string]string {
	return map[string]string{
		"P":              env.P,
		"PN":             env.PN,
		"PV":             env.PV,
		"PR":             env.PR,
		"PF":             env.PF,
		"CATEGORY":       env.CATEGORY,
		"PORTDIR":        env.PORTDIR,
		"DISTDIR":        env.DISTDIR,
		"PORTAGE_TMPDIR": env.PORTAGE_TMPDIR,
		"WORKDIR":        env.WORKDIR,
		"S":              env.S,
		"D":              env.D,
		"ED":             env.ED,
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
	}
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
