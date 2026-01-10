// Package ebuild implements ebuild execution engine.
//
// This file provides go-module.eclass support for building Go packages.
// The go-module eclass integrates Go module build system with ebuild phases.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/go-module.eclass/
package ebuild

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// Go Module Eclass Registration
// ============================================================================

// GoModuleEclass represents the go-module.eclass implementation.
type GoModuleEclass struct{}

// Name returns the eclass name.
func (e *GoModuleEclass) Name() string {
	return "go-module"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *GoModuleEclass) ExportedFunctions() []string {
	return []string{
		"src_unpack",
		"src_compile",
		"src_install",
	}
}

// Variables returns the default variables set by this eclass.
func (e *GoModuleEclass) Variables() map[string]string {
	return map[string]string{
		"GOPROXY":     "off",
		"CGO_ENABLED": "1",
	}
}

// ============================================================================
// EGO_SUM Processing
// ============================================================================

// GoModuleSetGlobals generates SRC_URI from EGO_SUM.
//
// Usage: go-module_set_globals
//
// Reads EGO_SUM variable and generates SRC_URI for all dependencies.
func (h *Helpers) GoModuleSetGlobals(args []string) error {
	egoSum := h.getEnvOrDefault("EGO_SUM", "")
	if egoSum == "" {
		return nil
	}

	var uris []string

	for _, line := range strings.Split(egoSum, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: module/path version hash
		// Example: github.com/spf13/cobra v1.6.1 h1:o94o...
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			module, version := parts[0], parts[1]
			uri := h.goModuleURI(module, version)
			if uri != "" {
				uris = append(uris, uri)
			}
		}
	}

	// Set SRC_URI
	if len(uris) > 0 {
		h.setEnvVar("SRC_URI", strings.Join(uris, "\n\t"))
	}

	return nil
}

// goModuleURI generates download URI for a Go module.
func (h *Helpers) goModuleURI(module, version string) string {
	// Skip go.mod entries
	if strings.HasSuffix(module, "/go.mod") {
		return ""
	}

	// Convert module path to proxy URL
	// github.com/spf13/cobra -> https://proxy.golang.org/github.com/spf13/cobra/@v/v1.6.1.zip
	escapedModule := strings.ReplaceAll(module, "/", "%2F")

	return fmt.Sprintf(
		"https://proxy.golang.org/%s/@v/%s.zip -> %s-@v-%s.zip",
		module, version, escapedModule, version,
	)
}

// ============================================================================
// Go Source Phases
// ============================================================================

// GoModuleSrcUnpack unpacks the main source and sets up module cache.
func (h *Helpers) GoModuleSrcUnpack(args []string) error {
	// 1. Call default unpack first
	if err := h.DefaultSrcUnpack(nil); err != nil {
		return err
	}

	// 2. Setup directories
	workdir := h.getEnvOrDefault("WORKDIR", "")
	if workdir == "" {
		return &DieError{Message: "go-module_src_unpack: WORKDIR not set"}
	}

	s := h.getEnvOrDefault("S", workdir)

	// 3. Check for vendored dependencies
	vendorDir := filepath.Join(s, "vendor")
	if _, err := os.Stat(vendorDir); err == nil {
		// Vendor directory exists, use it
		goflags := h.getEnvOrDefault("GOFLAGS", "")
		if !strings.Contains(goflags, "-mod=vendor") {
			h.setEnvVar("GOFLAGS", strings.TrimSpace(goflags+" -mod=vendor"))
		}
		h.writeStdout(">>> Using vendored dependencies\n")
		return nil
	}

	// 4. Set up module cache from downloaded modules
	goModCache := filepath.Join(workdir, "go-mod-cache")
	if err := os.MkdirAll(goModCache, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("go-module_src_unpack: failed to create mod cache: %v", err)}
	}
	h.setEnvVar("GOMODCACHE", goModCache)

	// 5. Unpack all .zip module files from DISTDIR
	distdir := h.getEnvOrDefault("DISTDIR", "")
	if distdir == "" {
		return nil
	}

	entries, err := os.ReadDir(distdir)
	if err != nil {
		h.writeStderr(fmt.Sprintf(">>> Warning: cannot read DISTDIR: %v\n", err))
		return nil
	}

	moduleCount := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "-@v-") && strings.HasSuffix(name, ".zip") {
			zipPath := filepath.Join(distdir, name)
			if err := h.unpackGoModule(zipPath, goModCache); err != nil {
				h.writeStderr(fmt.Sprintf(">>> Warning: failed to unpack %s: %v\n", name, err))
				continue
			}
			moduleCount++
		}
	}

	if moduleCount > 0 {
		h.writeStdout(fmt.Sprintf(">>> Unpacked %d Go modules\n", moduleCount))
	}

	return nil
}

// unpackGoModule unpacks a Go module .zip file to the module cache.
func (h *Helpers) unpackGoModule(zipPath, cacheDir string) error {
	// Extract module path and version from filename
	// github.com%2Fspf13%2Fcobra-@v-v1.6.1.zip
	filename := filepath.Base(zipPath)
	filename = strings.TrimSuffix(filename, ".zip")

	parts := strings.Split(filename, "-@v-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid module zip: %s", filename)
	}

	modulePath := strings.ReplaceAll(parts[0], "%2F", "/")
	version := parts[1]

	// Target directory: $GOMODCACHE/module@version
	targetDir := filepath.Join(cacheDir, modulePath+"@"+version)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}

	// Unzip the module
	return h.unzipFile(zipPath, targetDir)
}

// unzipFile extracts a zip file to a directory.
func (h *Helpers) unzipFile(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)

		// Ensure directory exists
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Extract file
		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			_ = rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// Go Build Phases
// ============================================================================

// GoModuleSrcCompile builds Go binaries.
func (h *Helpers) GoModuleSrcCompile(args []string) error {
	h.setupGoEnv()

	buildArgs := []string{"build", "-v"}

	// Add ldflags if specified
	if ldflags := h.getEnvOrDefault("GOLDFLAGS", ""); ldflags != "" {
		buildArgs = append(buildArgs, "-ldflags", ldflags)
	}

	// Determine build target
	buildTarget := h.getEnvOrDefault("GO_BUILD_TARGET", "")
	if buildTarget == "" {
		s := h.getEnvOrDefault("S", "")
		cmdDir := filepath.Join(s, "cmd")
		if _, err := os.Stat(cmdDir); err == nil {
			buildTarget = "./cmd/..."
		} else {
			buildTarget = "."
		}
	}
	buildArgs = append(buildArgs, buildTarget)

	h.writeStdout(fmt.Sprintf(">>> Running: go %s\n", strings.Join(buildArgs, " ")))

	return h.Ego(buildArgs)
}

// GoModuleSrcInstall installs Go binaries.
func (h *Helpers) GoModuleSrcInstall(args []string) error {
	s := h.getEnvOrDefault("S", "")
	d := h.getEnvOrDefault("D", "")
	eprefix := h.getEnvOrDefault("EPREFIX", "")

	if s == "" || d == "" {
		return &DieError{Message: "go-module_src_install: S or D not set"}
	}

	// Install binaries
	binDir := filepath.Join(d, eprefix, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("go-module_src_install: failed to create bin dir: %v", err)}
	}

	// Find binaries in S (go build puts them there by default)
	entries, err := os.ReadDir(s)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("go-module_src_install: failed to read S: %v", err)}
	}

	installed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip Go source files and known non-binaries
		if strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, ".mod") ||
			strings.HasSuffix(name, ".sum") ||
			strings.HasSuffix(name, ".txt") ||
			strings.HasSuffix(name, ".md") ||
			name == "LICENSE" ||
			name == "README" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if executable
		if info.Mode()&0111 == 0 {
			continue
		}

		src := filepath.Join(s, name)
		dst := filepath.Join(binDir, name)

		if err := h.copyFilePreserve(src, dst); err != nil {
			h.writeStderr(fmt.Sprintf(">>> Warning: failed to install %s: %v\n", name, err))
			continue
		}

		installed++
		h.writeStdout(fmt.Sprintf(">>> Installed: %s\n", name))
	}

	if installed == 0 {
		h.writeStderr(">>> Warning: no binaries installed\n")
	}

	return nil
}

// ============================================================================
// Go Environment Setup
// ============================================================================

// setupGoEnv configures Go-specific environment variables.
func (h *Helpers) setupGoEnv() {
	workdir := h.getEnvOrDefault("WORKDIR", "")
	if workdir == "" {
		return
	}

	// Set GOPATH
	gopath := filepath.Join(workdir, "go")
	h.setEnvVar("GOPATH", gopath)

	// Set GOCACHE
	gocache := filepath.Join(workdir, "go-cache")
	h.setEnvVar("GOCACHE", gocache)

	// Set GOMODCACHE if not already set
	if h.getEnvOrDefault("GOMODCACHE", "") == "" {
		h.setEnvVar("GOMODCACHE", filepath.Join(workdir, "go-mod-cache"))
	}

	// Disable network - builds must be offline
	h.setEnvVar("GOPROXY", "off")
	h.setEnvVar("GOFLAGS", h.getEnvOrDefault("GOFLAGS", "")+" -mod=readonly")

	// CGO settings - respect system compiler
	h.setEnvVar("CGO_ENABLED", "1")

	// Get toolchain compilers
	cc := h.tcGetCC()
	cxx := h.tcGetCXX()

	if cc != "" {
		h.setEnvVar("CC", cc)
	}
	if cxx != "" {
		h.setEnvVar("CXX", cxx)
	}

	// CGO flags from environment
	if cflags := h.getEnvOrDefault("CFLAGS", ""); cflags != "" {
		h.setEnvVar("CGO_CFLAGS", cflags)
	}
	if cxxflags := h.getEnvOrDefault("CXXFLAGS", ""); cxxflags != "" {
		h.setEnvVar("CGO_CXXFLAGS", cxxflags)
	}
	if ldflags := h.getEnvOrDefault("LDFLAGS", ""); ldflags != "" {
		h.setEnvVar("CGO_LDFLAGS", ldflags)
	}

	// Parallel builds
	if jobs := h.getEnvOrDefault("MAKEOPTS_JOBS", ""); jobs != "" {
		h.setEnvVar("GOMAXPROCS", jobs)
	}
}

// Ego runs go command with correct environment.
//
// Usage: ego build ./cmd/...
func (h *Helpers) Ego(args []string) error {
	h.setupGoEnv()

	if len(args) == 0 {
		return &DieError{Message: "ego: requires command argument"}
	}

	return h.executeCommand("go", args)
}

// ============================================================================
// Helper Methods
// ============================================================================

// tcGetCC gets the C compiler from toolchain-funcs.
func (h *Helpers) tcGetCC() string {
	// Check if tc-getCC result is cached
	if cc := h.getEnvOrDefault("CC", ""); cc != "" {
		return cc
	}

	// Try to get from CHOST
	chost := h.getEnvOrDefault("CHOST", "")
	if chost != "" {
		return chost + "-gcc"
	}

	return "gcc"
}

// tcGetCXX gets the C++ compiler from toolchain-funcs.
func (h *Helpers) tcGetCXX() string {
	// Check if tc-getCXX result is cached
	if cxx := h.getEnvOrDefault("CXX", ""); cxx != "" {
		return cxx
	}

	// Try to get from CHOST
	chost := h.getEnvOrDefault("CHOST", "")
	if chost != "" {
		return chost + "-g++"
	}

	return "g++"
}
