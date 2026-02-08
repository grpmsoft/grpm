package ebuild

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/ulikunitz/xz"
)

// ExecutePhaseReal executes a single ebuild phase with proper dispatch.
//
// Phase dispatch follows PMS Section 8:
//  1. Check if ebuild defines custom phase function (e.g., src_configure)
//  2. If yes -> execute ebuild's function via interpreter
//  3. If no -> call default_src_* implementation
//
// This ensures ebuilds using cmake, meson, or custom build systems work correctly.
func (e *Executor) ExecutePhaseReal(phase Phase) PhaseResult {
	startTime := time.Now()

	result := PhaseResult{
		Phase:   phase,
		Success: false,
	}

	// Set current phase for EBUILD_PHASE environment variable
	e.SetCurrentPhase(phase)

	var err error
	var output string

	switch phase {
	case PhasePretend:
		output, err = e.dispatchPhase(phase, e.phasePretend)

	case PhaseSetup:
		output, err = e.dispatchPhase(phase, e.phaseSetup)

	case PhaseUnpack:
		output, err = e.dispatchPhase(phase, e.phaseUnpack)

	case PhasePrepare:
		output, err = e.dispatchPhase(phase, e.phasePrepare)

	case PhaseConfigure:
		output, err = e.dispatchPhase(phase, e.phaseConfigure)

	case PhaseCompile:
		output, err = e.dispatchPhase(phase, e.phaseCompile)

	case PhaseTest:
		output, err = e.dispatchPhase(phase, e.phaseTest)

	case PhaseInstall:
		output, err = e.dispatchPhase(phase, e.phaseInstall)

	case PhasePreinst, PhasePostinst, PhasePrerem, PhasePostrm:
		// Hook phases use custom dispatch as well
		output, err = e.dispatchPhase(phase, func() (string, error) {
			return fmt.Sprintf("%s completed (default)", phase), nil
		})

	case PhaseConfig:
		output, err = e.dispatchPhase(phase, e.phaseConfig)

	case PhaseInfo:
		output, err = e.dispatchPhase(phase, e.phaseInfo)

	case PhaseNofetch:
		output, err = e.dispatchPhase(phase, e.phaseNofetch)

	default:
		err = fmt.Errorf("unknown phase: %s", phase)
	}

	if err != nil {
		result.Error = err
		result.Output = output
		result.Success = false
	} else {
		result.Success = true
		result.Output = output
	}

	result.Duration = time.Since(startTime).Milliseconds()
	return result
}

// dispatchPhase dispatches a phase to either custom or default implementation.
//
// Per PMS Section 8:
//   - If ebuild defines the phase function (src_configure, etc.) -> call it
//   - If eclass exports the function via EXPORT_FUNCTIONS -> call eclass version
//   - Otherwise -> call the default implementation
//
// Panics from the interpreter are caught and converted to descriptive errors.
func (e *Executor) dispatchPhase(phase Phase, defaultImpl func() (string, error)) (output string, phaseErr error) {
	funcName := phaseFunctionName(phase)
	logging.Debug("[ebuild] dispatching phase %s (function: %s)", phase, funcName)

	// Recover from panics during phase execution.
	// This catches panics from both custom phase functions (via interpreter)
	// and default implementations that may invoke external tools.
	defer func() {
		if r := recover(); r != nil {
			pkgName := ""
			if e.Package != nil {
				pkgName = e.Package.Name
			}
			logging.Debug("[ebuild] panic recovered in phase %s for %s: %v", phase, pkgName, r)
			output = ""
			phaseErr = fmt.Errorf("phase %s panicked for %s: %v (unsupported bash construct)", phase, pkgName, r)
		}
	}()

	// Check if custom phase function exists
	if e.HasPhaseFunction(phase) {
		logging.Debug("[ebuild] found custom %s, executing ebuild function", funcName)
		return e.RunPhaseFunction(phase)
	}

	// No custom function, use default implementation
	logging.Debug("[ebuild] no custom %s found, using default", funcName)
	return defaultImpl()
}

// phaseSetup performs pkg_setup phase.
func (e *Executor) phaseSetup() (string, error) {
	// pkg_setup is typically used for checking system requirements
	// For now, just verify directories exist
	if err := e.Env.CreateDirectories(); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	return "Setup completed - work directories created", nil
}

// phaseUnpack performs src_unpack phase - extracts source archives.
//
// Uses the $A variable (populated from Manifest) to determine which
// archives to extract. Falls back to pattern matching if $A is empty.
// Supports: .tar.gz, .tar.bz2, .tar.xz, .tar.zst, .tar, .zip, .gz, .bz2, .xz, .zst
func (e *Executor) phaseUnpack() (string, error) {
	var archives []string

	// Primary: Use $A variable which contains the list of archives
	if e.Env.A != "" {
		archives = strings.Fields(e.Env.A)
		logging.Debug("[ebuild] Using $A variable for unpack: %v", archives)
	}

	// Fallback: Search by pattern if $A is empty
	if len(archives) == 0 {
		tarballPattern := fmt.Sprintf("%s-%s.tar.*", e.Env.PN, e.Env.PV)
		matches, err := filepath.Glob(filepath.Join(e.Env.DISTDIR, tarballPattern))
		if err != nil {
			return "", fmt.Errorf("failed to search for source tarball: %w", err)
		}
		for _, m := range matches {
			archives = append(archives, filepath.Base(m))
		}
		logging.Debug("[ebuild] Using pattern fallback for unpack: %s -> %v", tarballPattern, archives)
	}

	if len(archives) == 0 {
		// No tarball found - this might be a binary package or ebuild-only
		logging.Debug("[ebuild] No source archives found, skipping unpack")
		return "No source tarball found (ebuild-only package or binary)", nil
	}

	// Extract each archive using helpers unpack (supports all formats).
	// Skip non-archive files like PGP signatures and patches.
	helpers := &Helpers{}
	var extracted []string
	for _, archive := range archives {
		// Skip non-archive files (PGP signatures, attestations, patches, etc.)
		lower := strings.ToLower(archive)
		if strings.HasSuffix(lower, ".asc") || strings.HasSuffix(lower, ".sig") ||
			strings.HasSuffix(lower, ".sign") || strings.HasSuffix(lower, ".patch") ||
			strings.HasSuffix(lower, ".diff") || strings.HasSuffix(lower, ".provenance") {
			logging.Debug("[ebuild] Skipping non-archive file: %s", archive)
			continue
		}

		tarball := filepath.Join(e.Env.DISTDIR, archive)
		if _, err := os.Stat(tarball); err != nil {
			logging.Debug("[ebuild] Archive %s not found in DISTDIR, skipping", archive)
			continue
		}

		logging.Debug("[ebuild] Extracting %s to %s", archive, e.Env.WORKDIR)
		if err := helpers.unpackArchive(tarball, e.Env.WORKDIR); err != nil {
			return "", fmt.Errorf("failed to extract %s: %w", archive, err)
		}
		extracted = append(extracted, archive)
	}

	if len(extracted) == 0 {
		return "No archives extracted", nil
	}

	return fmt.Sprintf("Extracted %d archive(s): %s", len(extracted), strings.Join(extracted, ", ")), nil
}

// phasePrepare performs src_prepare phase - applies patches.
func (e *Executor) phasePrepare() (string, error) {
	// TODO: Apply patches from FILESDIR
	// TODO: Run eapply_user for user patches

	// For now, just check if source directory exists
	if _, err := os.Stat(e.Env.S); os.IsNotExist(err) {
		logging.Debug("[ebuild] Source directory %s does not exist, skipping prepare", e.Env.S)
		return "Source directory not found (skipping)", nil
	}

	return "Prepare phase completed (no patches)", nil
}

// phaseConfigure performs src_configure phase - runs ./configure.
func (e *Executor) phaseConfigure() (string, error) {
	configurePath := filepath.Join(e.Env.S, "configure")

	// Check if configure script exists
	if _, err := os.Stat(configurePath); os.IsNotExist(err) {
		logging.Debug("[ebuild] No configure script found, skipping configure phase")
		return "No configure script (skipping)", nil
	}

	// Run ./configure with standard flags
	args := []string{
		"--prefix=/usr",
		"--sysconfdir=/etc",
		"--localstatedir=/var",
		"--libdir=/usr/lib64", // TODO: Detect from ARCH
		"--mandir=/usr/share/man",
		"--infodir=/usr/share/info",
	}

	logging.Debug("[ebuild] Running: ./configure %s", strings.Join(args, " "))

	cmd := exec.Command("./configure", args...)
	cmd.Dir = e.Env.S
	cmd.Env = e.Env.ToSlice()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("configure failed: %w", err)
	}

	return string(output), nil
}

// phaseCompile performs src_compile phase - runs make.
func (e *Executor) phaseCompile() (string, error) {
	// Check if Makefile exists
	makefilePath := filepath.Join(e.Env.S, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		logging.Debug("[ebuild] No Makefile found, skipping compile phase")
		return "No Makefile (skipping)", nil
	}

	// Parse MAKEOPTS (e.g., "-j4" -> []string{"-j4"})
	makeArgs := strings.Fields(e.Env.MAKEOPTS)

	logging.Debug("[ebuild] Running: make %s", strings.Join(makeArgs, " "))

	cmd := exec.Command("make", makeArgs...)
	cmd.Dir = e.Env.S
	cmd.Env = e.Env.ToSlice()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("make failed: %w", err)
	}

	return string(output), nil
}

// phaseTest performs src_test phase - runs test suite.
func (e *Executor) phaseTest() (string, error) {
	// Common test targets: check, test
	testTargets := []string{"check", "test"}

	for _, target := range testTargets {
		cmd := exec.Command("make", target)
		cmd.Dir = e.Env.S
		cmd.Env = e.Env.ToSlice()

		output, err := cmd.CombinedOutput()
		if err == nil {
			return string(output), nil
		}

		// Try next target
		logging.Debug("[ebuild] Test target '%s' not available, trying next", target)
	}

	return "No test target found (skipping)", nil
}

// phaseInstall performs src_install phase - runs make install DESTDIR=${D}.
//
// Per PMS Section 9.1.13, the default src_install runs:
//   - EAPI 4+: emake DESTDIR="${D}" install
//
// If no Makefile exists (e.g., virtual packages, acct-* packages),
// this is not an error — the package simply has nothing to install
// via make, and eclass-provided src_install handles it instead.
func (e *Executor) phaseInstall() (string, error) {
	// Check if Makefile exists — if not, this is a no-op
	// (virtual packages, acct-* packages, Python packages use eclass install)
	makefilePath := filepath.Join(e.Env.S, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		logging.Debug("[ebuild] no Makefile in %s, skipping default make install", e.Env.S)
		return "No Makefile found (skipping default install)", nil
	}

	// Run make install DESTDIR=${D}
	args := []string{
		"install",
		fmt.Sprintf("DESTDIR=%s", e.Env.D),
	}

	logging.Debug("[ebuild] Running: make %s", strings.Join(args, " "))

	cmd := exec.Command("make", args...)
	cmd.Dir = e.Env.S
	cmd.Env = e.Env.ToSlice()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("make install failed: %w", err)
	}

	// Verify that files were installed
	entries, err := os.ReadDir(e.Env.D)
	if err != nil {
		return string(output), fmt.Errorf("failed to read image directory: %w", err)
	}

	if len(entries) == 0 {
		return string(output), fmt.Errorf("no files installed to ${D}")
	}

	return fmt.Sprintf("Installed %d top-level entries to ${D}", len(entries)), nil
}

// phasePretend performs pkg_pretend phase - pre-fetch sanity checks.
//
// Per PMS Section 9.1.2:
//   - Only available in EAPI 4+
//   - Runs BEFORE fetching sources
//   - Must NOT write to the filesystem
//   - Used for sanity checks (kernel config, system requirements)
//   - No guarantee dependencies are installed
//
// The default implementation is a no-op (returns success).
func (e *Executor) phasePretend() (string, error) {
	// Check EAPI support
	if !e.EAPIFeatures.SupportsPkgPretend() {
		return "pkg_pretend not supported in this EAPI (requires EAPI 4+)", nil
	}

	// Default implementation is a no-op per PMS
	return "pkg_pretend completed (default no-op)", nil
}

// phaseConfig performs pkg_config phase - post-install configuration.
//
// Per PMS Section 9.1.14:
//   - Run manually via `grpm config category/package`
//   - Interactive configuration after package installation
//   - May prompt for user input
//   - Must have full access to ROOT
//
// The default implementation is a no-op (returns success).
func (e *Executor) phaseConfig() (string, error) {
	// Default implementation is a no-op per PMS
	return "pkg_config completed (default no-op)", nil
}

// phaseInfo performs pkg_info phase - display package information.
//
// Per PMS Section 9.1.15:
//   - Called when displaying information about an installed package
//   - EAPI 4+: Can also be called for non-installed packages
//   - Must NOT write to the filesystem
//
// The default implementation is a no-op (returns success).
func (e *Executor) phaseInfo() (string, error) {
	// Default implementation is a no-op per PMS
	return "pkg_info completed (default no-op)", nil
}

// phaseNofetch performs pkg_nofetch phase - handle fetch-restricted packages.
//
// Per PMS Section 9.1.16:
//   - Called when RESTRICT=fetch and source files are unavailable
//   - Should print instructions for manual download
//   - Must NOT write to the filesystem
//
// The default implementation prints a generic message.
func (e *Executor) phaseNofetch() (string, error) {
	// Default implementation prints a message per PMS
	msg := fmt.Sprintf(`
pkg_nofetch: This package has fetch restrictions.

Package: %s/%s-%s

The source files for this package cannot be automatically downloaded.
Please obtain the following files manually and place them in:
  %s

Required files (A variable):
  %s

After downloading, re-run the installation.
`,
		e.Env.CATEGORY, e.Env.PN, e.Env.PV,
		e.Env.DISTDIR,
		e.Env.A)

	logging.Debug("[ebuild] %s", msg)
	return msg, nil
}

// extractTarball extracts a tarball to the destination directory.
//
// Supports: .tar.gz, .tar.bz2, .tar.xz
func extractTarball(tarballPath, destDir string) error {
	file, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open tarball: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create compression reader
	reader, err := createCompressionReader(file, tarballPath)
	if err != nil {
		return err
	}

	// Extract tar archive
	tarReader := tar.NewReader(reader)
	return extractTarEntries(tarReader, destDir)
}

// createCompressionReader creates appropriate reader based on file extension.
func createCompressionReader(file *os.File, tarballPath string) (io.Reader, error) {
	switch {
	case strings.HasSuffix(tarballPath, ".tar.gz") || strings.HasSuffix(tarballPath, ".tgz"):
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return gzReader, nil

	case strings.HasSuffix(tarballPath, ".tar.bz2") || strings.HasSuffix(tarballPath, ".tbz2"):
		return bzip2.NewReader(file), nil

	case strings.HasSuffix(tarballPath, ".tar.xz") || strings.HasSuffix(tarballPath, ".txz"):
		xzReader, err := xz.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		return xzReader, nil

	case strings.HasSuffix(tarballPath, ".tar"):
		return file, nil

	default:
		return nil, fmt.Errorf("unsupported tarball format: %s", tarballPath)
	}
}

// extractTarEntries extracts all entries from a tar reader.
func extractTarEntries(tarReader *tar.Reader, destDir string) error {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		if err := extractTarEntry(tarReader, header, target); err != nil {
			return err
		}
	}
	return nil
}

// extractTarEntry extracts a single tar entry based on its type.
func extractTarEntry(tarReader *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return extractDirectory(target, header.Mode)
	case tar.TypeReg:
		return extractRegularFile(tarReader, target, header.Mode)
	case tar.TypeSymlink:
		return extractSymlink(target, header.Linkname)
	default:
		logging.Debug("[ebuild] Unsupported tar entry type %d for %s", header.Typeflag, header.Name)
		return nil
	}
}

// extractDirectory creates a directory with specified permissions.
func extractDirectory(target string, mode int64) error {
	if err := os.MkdirAll(target, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// extractRegularFile extracts a regular file from tar archive.
func extractRegularFile(tarReader *tar.Reader, target string, mode int64) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(mode))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractSymlink creates a symbolic link.
func extractSymlink(target, linkname string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if err := os.Symlink(linkname, target); err != nil {
		// Ignore errors for existing symlinks
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	return nil
}
