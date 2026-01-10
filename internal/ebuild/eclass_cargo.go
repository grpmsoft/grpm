// Package ebuild implements ebuild execution engine.
//
// This file provides cargo.eclass support for building Rust packages.
// The cargo eclass integrates Rust/Cargo build system with ebuild phases.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/cargo.eclass/
package ebuild

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// Cargo Eclass Registration
// ============================================================================

// CargoEclass represents the cargo.eclass implementation.
type CargoEclass struct{}

// Name returns the eclass name.
func (e *CargoEclass) Name() string {
	return "cargo"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *CargoEclass) ExportedFunctions() []string {
	return []string{
		"src_unpack",
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}
}

// Variables returns the default variables set by this eclass.
func (e *CargoEclass) Variables() map[string]string {
	return map[string]string{
		"CARGO_HOME":        "",
		"ECARGO_VENDOR":     "",
		"CARGO_NET_OFFLINE": "true",
	}
}

// ============================================================================
// Crate URI Generation
// ============================================================================

// CargoCrateUris generates SRC_URI entries for crate dependencies.
//
// Usage: SRC_URI="$(cargo_crate_uris ${CRATES})"
//
// Input format: "name-version name-version ..."
// Output format: URI list with -> rename syntax.
func (h *Helpers) CargoCrateUris(args []string) error {
	var uris []string

	for _, crate := range args {
		// Parse name-version format (e.g., "libc-0.2.150")
		// Need to find the last dash before version number
		uri := h.crateDependencyURI(crate)
		if uri != "" {
			uris = append(uris, uri)
		}
	}

	h.writeStdout(strings.Join(uris, "\n\t"))
	return nil
}

// crateDependencyURI generates download URI for a crate.
func (h *Helpers) crateDependencyURI(crate string) string {
	// Parse crate name-version (find last dash before version)
	name, version := h.parseCrateName(crate)
	if name == "" || version == "" {
		return ""
	}

	// crates.io download URL format
	return fmt.Sprintf(
		"https://crates.io/api/v1/crates/%s/%s/download -> %s-%s.crate",
		name, version, name, version,
	)
}

// parseCrateName parses "name-version" into name and version parts.
// Handles crates with dashes in names (e.g., "proc-macro2-1.0.0").
func (h *Helpers) parseCrateName(crate string) (name, version string) {
	// Find the last dash that precedes a digit (version start)
	for i := len(crate) - 1; i >= 0; i-- {
		if crate[i] == '-' && i < len(crate)-1 && isDigit(crate[i+1]) {
			return crate[:i], crate[i+1:]
		}
	}
	return "", ""
}

// isDigit returns true if byte is a digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ============================================================================
// Cargo Source Phases
// ============================================================================

// CargoSrcUnpack unpacks the main source and vendors crate dependencies.
func (h *Helpers) CargoSrcUnpack(args []string) error {
	// 1. Call default unpack first
	if err := h.DefaultSrcUnpack(nil); err != nil {
		return err
	}

	// 2. Setup vendor directory
	workdir := h.getEnvOrDefault("WORKDIR", "")
	if workdir == "" {
		return &DieError{Message: "cargo_src_unpack: WORKDIR not set"}
	}

	vendorDir := filepath.Join(workdir, ".cargo", "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("cargo_src_unpack: failed to create vendor dir: %v", err)}
	}

	// 3. Unpack all .crate files from DISTDIR
	distdir := h.getEnvOrDefault("DISTDIR", "")
	if distdir == "" {
		h.writeStderr(">>> No DISTDIR set, skipping crate vendoring\n")
		return nil
	}

	entries, err := os.ReadDir(distdir)
	if err != nil {
		h.writeStderr(fmt.Sprintf(">>> Warning: cannot read DISTDIR: %v\n", err))
		return nil
	}

	crateCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".crate") {
			cratePath := filepath.Join(distdir, entry.Name())
			if err := h.unpackCrate(cratePath, vendorDir); err != nil {
				h.writeStderr(fmt.Sprintf(">>> Warning: failed to unpack %s: %v\n", entry.Name(), err))
				continue
			}
			crateCount++
		}
	}

	if crateCount > 0 {
		h.writeStdout(fmt.Sprintf(">>> Vendored %d crates\n", crateCount))
	}

	// 4. Generate .cargo/config.toml
	return h.generateCargoConfig(workdir, vendorDir)
}

// unpackCrate unpacks a .crate file (gzipped tarball) to vendor directory.
func (h *Helpers) unpackCrate(cratePath, vendorDir string) error {
	file, err := os.Open(cratePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// .crate files are gzipped tarballs
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Construct target path
		targetPath := filepath.Join(vendorDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return err
			}
			_ = outFile.Close()
		}
	}

	return nil
}

// generateCargoConfig creates .cargo/config.toml for vendored builds.
func (h *Helpers) generateCargoConfig(workdir, vendorDir string) error {
	configDir := filepath.Join(workdir, ".cargo")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	config := fmt.Sprintf(`[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "%s"

[net]
offline = true

[build]
jobs = %s
`, vendorDir, h.getEnvOrDefault("MAKEOPTS_JOBS", "1"))

	configPath := filepath.Join(configDir, "config.toml")
	return os.WriteFile(configPath, []byte(config), 0644)
}

// ============================================================================
// Cargo Build Phases
// ============================================================================

// CargoSrcConfigure sets up Cargo build environment.
func (h *Helpers) CargoSrcConfigure(args []string) error {
	// Setup Cargo environment
	h.setupCargoEnv()

	h.writeStdout(">>> Cargo configure complete (using Cargo.toml)\n")
	return nil
}

// CargoSrcCompile builds the Rust project.
func (h *Helpers) CargoSrcCompile(args []string) error {
	h.setupCargoEnv()

	buildArgs := []string{
		"build",
		"--release",
		"--offline",
	}

	// Add target if cross-compiling
	if chost := h.getEnvOrDefault("CHOST", ""); chost != "" {
		target := h.chostToRustTarget(chost)
		if target != "" {
			buildArgs = append(buildArgs, "--target", target)
		}
	}

	h.writeStdout(fmt.Sprintf(">>> Running: cargo %s\n", strings.Join(buildArgs, " ")))

	return h.executeCommand("cargo", buildArgs)
}

// CargoSrcTest runs cargo test.
func (h *Helpers) CargoSrcTest(args []string) error {
	h.setupCargoEnv()

	testArgs := []string{
		"test",
		"--release",
		"--offline",
	}

	h.writeStdout(fmt.Sprintf(">>> Running: cargo %s\n", strings.Join(testArgs, " ")))

	return h.executeCommand("cargo", testArgs)
}

// CargoSrcInstall installs Rust binaries.
func (h *Helpers) CargoSrcInstall(args []string) error {
	h.setupCargoEnv()

	s := h.getEnvOrDefault("S", "")
	d := h.getEnvOrDefault("D", "")
	eprefix := h.getEnvOrDefault("EPREFIX", "")

	if s == "" || d == "" {
		return &DieError{Message: "cargo_src_install: S or D not set"}
	}

	// Determine target directory
	var targetDir string
	if chost := h.getEnvOrDefault("CHOST", ""); chost != "" {
		target := h.chostToRustTarget(chost)
		if target != "" {
			targetDir = filepath.Join(s, "target", target, "release")
		}
	}
	if targetDir == "" {
		targetDir = filepath.Join(s, "target", "release")
	}

	// Check if target directory exists
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return &DieError{Message: fmt.Sprintf("cargo_src_install: target directory not found: %s", targetDir)}
	}

	// Install binaries
	binDir := filepath.Join(d, eprefix, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("cargo_src_install: failed to create bin dir: %v", err)}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("cargo_src_install: failed to read target dir: %v", err)}
	}

	installed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip non-executable files
		if strings.HasSuffix(name, ".d") ||
			strings.HasSuffix(name, ".rlib") ||
			strings.HasSuffix(name, ".rmeta") ||
			strings.HasSuffix(name, ".so") ||
			strings.HasSuffix(name, ".a") {
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

		src := filepath.Join(targetDir, name)
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
// Cargo Environment Setup
// ============================================================================

// setupCargoEnv configures Cargo-specific environment variables.
func (h *Helpers) setupCargoEnv() {
	workdir := h.getEnvOrDefault("WORKDIR", "")
	if workdir == "" {
		return
	}

	// CARGO_HOME
	cargoHome := filepath.Join(workdir, ".cargo")
	h.setEnvVar("CARGO_HOME", cargoHome)

	// Vendor directory
	vendorDir := filepath.Join(cargoHome, "vendor")
	h.setEnvVar("ECARGO_VENDOR", vendorDir)

	// Offline mode
	h.setEnvVar("CARGO_NET_OFFLINE", "true")

	// Build jobs
	if jobs := h.getEnvOrDefault("MAKEOPTS_JOBS", ""); jobs != "" {
		h.setEnvVar("CARGO_BUILD_JOBS", jobs)
	}

	// Convert CFLAGS to RUSTFLAGS
	if cflags := h.getEnvOrDefault("CFLAGS", ""); cflags != "" {
		rustflags := h.convertCflagsToRustflags(cflags)
		if rustflags != "" {
			h.setEnvVar("CARGO_ENCODED_RUSTFLAGS", rustflags)
		}
	}
}

// CargoEnv is the eclass function to setup Cargo environment.
func (h *Helpers) CargoEnv(args []string) error {
	h.setupCargoEnv()
	return nil
}

// convertCflagsToRustflags converts common CFLAGS to RUSTFLAGS.
func (h *Helpers) convertCflagsToRustflags(cflags string) string {
	var rustflags []string

	for _, flag := range strings.Fields(cflags) {
		switch {
		case strings.HasPrefix(flag, "-O"):
			// Optimization level
			level := strings.TrimPrefix(flag, "-O")
			if level == "" {
				level = "2"
			}
			rustflags = append(rustflags, "-C", "opt-level="+level)

		case flag == "-g":
			// Debug info
			rustflags = append(rustflags, "-C", "debuginfo=2")

		case strings.HasPrefix(flag, "-march="):
			// CPU architecture
			arch := strings.TrimPrefix(flag, "-march=")
			rustflags = append(rustflags, "-C", "target-cpu="+arch)

		case strings.HasPrefix(flag, "-mtune="):
			// CPU tuning
			tune := strings.TrimPrefix(flag, "-mtune=")
			rustflags = append(rustflags, "-C", "target-cpu="+tune)

		case flag == "-fPIC":
			// Position independent code
			rustflags = append(rustflags, "-C", "relocation-model=pic")

			// Note: -pipe and other -f flags are silently skipped
			// as they are not directly translatable to RUSTFLAGS
		}
	}

	// CARGO_ENCODED_RUSTFLAGS uses 0x1f (unit separator) as delimiter
	return strings.Join(rustflags, "\x1f")
}

// ============================================================================
// Rust Target Conversion
// ============================================================================

// chostToRustTarget converts CHOST to Rust target triple.
func (h *Helpers) chostToRustTarget(chost string) string {
	// Common CHOST to Rust target mappings
	// x86_64-pc-linux-gnu -> x86_64-unknown-linux-gnu
	// i686-pc-linux-gnu -> i686-unknown-linux-gnu
	// aarch64-unknown-linux-gnu stays the same

	parts := strings.Split(chost, "-")
	if len(parts) < 3 {
		return chost
	}

	arch := parts[0]
	os := parts[len(parts)-2]
	env := parts[len(parts)-1]

	// Build Rust target triple
	switch arch {
	case "x86_64", "i686", "i386", "aarch64", "arm", "armv7":
		// These translate directly
	case "powerpc64":
		arch = "powerpc64"
	case "powerpc64le":
		arch = "powerpc64le"
	default:
		return chost
	}

	// Vendor is usually "unknown" in Rust
	return fmt.Sprintf("%s-unknown-%s-%s", arch, os, env)
}

// ============================================================================
// Utility Functions
// ============================================================================

// copyFilePreserve copies a file preserving its mode.
func (h *Helpers) copyFilePreserve(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
