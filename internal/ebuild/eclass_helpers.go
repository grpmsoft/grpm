// Package ebuild implements ebuild execution engine.
//
// This file provides eclass-related helper functions implemented in pure Go.
// These functions are typically provided by eclasses like eutils, toolchain-funcs,
// multilib, flag-o-matic, and linux-info.
package ebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// stateInstalledPackage is a type alias to avoid import cycles.
// Used by findBestVersion to return package information.
type stateInstalledPackage = state.InstalledPackage

// ============================================================================
// eutils.eclass Functions
// ============================================================================

// Epatch applies patches using eapply.
//
// Usage: epatch file.patch
// Usage: epatch directory/
//
// Deprecated in EAPI 6+ in favor of eapply, but still widely used.
// This implementation delegates to eapply.
func (h *Helpers) Epatch(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "epatch: no patches specified"}
	}

	h.writeStdout(">>> epatch is deprecated, use eapply instead\n")
	return h.Eapply(args)
}

// EshoptsPush saves the current shell options state.
//
// Usage: eshopts_push [-s|-u] [options...]
//
// Pushes the current shopts state onto a stack, optionally setting/unsetting options.
// Use eshopts_pop to restore.
func (h *Helpers) EshoptsPush(args []string) error {
	// Get current shell options (simulated)
	// In real bash, this would be $BASHOPTS
	currentOpts := h.getShellOptions()
	h.eclassStack.Push("eshopts", currentOpts)

	if len(args) == 0 {
		return nil
	}

	// Parse and apply options
	setMode := true
	for _, arg := range args {
		switch arg {
		case "-s":
			setMode = true
		case "-u":
			setMode = false
		default:
			// Apply option (simulated - would need real shell integration)
			if setMode {
				h.setShellOption(arg, true)
			} else {
				h.setShellOption(arg, false)
			}
		}
	}

	return nil
}

// EshoptsPop restores the previous shell options state.
//
// Usage: eshopts_pop
//
// Pops the last saved shopts state from the stack and restores it.
func (h *Helpers) EshoptsPop(args []string) error {
	savedOpts, ok := h.eclassStack.Pop("eshopts")
	if !ok {
		return &DieError{Message: "eshopts_pop: stack underflow"}
	}

	// Restore options (simulated)
	h.restoreShellOptions(savedOpts)
	return nil
}

// EstackPush pushes a value onto a named stack.
//
// Usage: estack_push stackname value
func (h *Helpers) EstackPush(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "estack_push: requires stackname and value"}
	}

	stackName := args[0]
	value := strings.Join(args[1:], " ")
	h.eclassStack.Push(stackName, value)
	return nil
}

// EstackPop pops a value from a named stack.
//
// Usage: estack_pop stackname [variable]
//
// If variable is provided, the popped value is assigned to it (simulated via stdout).
func (h *Helpers) EstackPop(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "estack_pop: requires stackname"}
	}

	stackName := args[0]
	value, ok := h.eclassStack.Pop(stackName)
	if !ok {
		return exitFalse()
	}

	// If variable name provided, output for capture
	if len(args) >= 2 {
		h.writeStdout(value)
	}

	return nil
}

// Helper functions for shell options (simulated)
func (h *Helpers) getShellOptions() string {
	// In real implementation, this would query bash $BASHOPTS
	return ""
}

func (h *Helpers) setShellOption(opt string, enable bool) {
	// In real implementation, this would use shopt -s/-u
	_ = opt
	_ = enable
}

func (h *Helpers) restoreShellOptions(opts string) {
	// In real implementation, this would restore $BASHOPTS
	_ = opts
}

// ============================================================================
// toolchain-funcs.eclass Functions (Additional)
// ============================================================================
//
// Note: Core toolchain functions (tc-getCC, tc-getCXX, tc-getAR, etc.) are
// implemented in helpers_toolchain.go with full CHOST/CBUILD support.
//
// This section contains additional toolchain-funcs.eclass functions.

// TcIsGcc checks if the C compiler is GCC.
//
// Usage: tc-is-gcc && echo "Using GCC"
//
// Returns exit code 0 if CC is GCC, 1 otherwise.
func (h *Helpers) TcIsGcc(args []string) error {
	cc := h.getEnvOrDefault("CC", "gcc")

	// Check if CC contains "gcc" and not "clang"
	if strings.Contains(cc, "gcc") && !strings.Contains(cc, "clang") {
		return nil // Exit code 0
	}

	return exitFalse()
}

// TcIsClang checks if the C compiler is Clang.
//
// Usage: tc-is-clang && echo "Using Clang"
//
// Returns exit code 0 if CC is Clang, 1 otherwise.
func (h *Helpers) TcIsClang(args []string) error {
	cc := h.getEnvOrDefault("CC", "gcc")

	if strings.Contains(cc, "clang") {
		return nil // Exit code 0
	}

	return exitFalse()
}

// TcEndianBig checks if the target is big-endian.
//
// Usage: tc-endian big && echo "Big endian"
func (h *Helpers) TcEndianBig(args []string) error {
	endian := h.detectEndian()
	if endian == "big" {
		return nil
	}
	return exitFalse()
}

// TcEndianLittle checks if the target is little-endian.
//
// Usage: tc-endian little && echo "Little endian"
func (h *Helpers) TcEndianLittle(args []string) error {
	endian := h.detectEndian()
	if endian == "little" {
		return nil
	}
	return exitFalse()
}

// ============================================================================
// multilib.eclass Functions
// ============================================================================

// GetLibdir returns the library directory for the current ABI.
//
// Usage: libdir=$(get_libdir)
//
// Returns "lib" or "lib64" depending on architecture.
func (h *Helpers) GetLibdir(args []string) error {
	libdir := h.libDir
	h.writeStdout(libdir)
	return nil
}

// MultilibNativeUseWith generates --with-foo=/usr/lib64 style arguments.
//
// Usage: multilib_native_use_with ssl openssl
//
// If USE flag is set and we're on native ABI, outputs configure flag.
func (h *Helpers) MultilibNativeUseWith(args []string) error {
	if len(args) < 1 {
		return nil
	}

	flag := args[0]
	optName := flag
	if len(args) >= 2 {
		optName = args[1]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(fmt.Sprintf("--with-%s", optName))
	} else {
		h.writeStdout(fmt.Sprintf("--without-%s", optName))
	}
	return nil
}

// MultilibNativeUseEnable generates --enable-foo style arguments.
//
// Usage: multilib_native_use_enable ssl
func (h *Helpers) MultilibNativeUseEnable(args []string) error {
	if len(args) < 1 {
		return nil
	}

	flag := args[0]
	optName := flag
	if len(args) >= 2 {
		optName = args[1]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(fmt.Sprintf("--enable-%s", optName))
	} else {
		h.writeStdout(fmt.Sprintf("--disable-%s", optName))
	}
	return nil
}

// ============================================================================
// flag-o-matic.eclass Functions
// ============================================================================

// AppendCflags appends flags to CFLAGS.
//
// Usage: append-cflags -O2 -march=native
func (h *Helpers) AppendCflags(args []string) error {
	h.cflags = append(h.cflags, args...)

	// Update environment if available
	if h.env != nil {
		if h.env.CFLAGS != "" {
			h.env.CFLAGS += " " + strings.Join(args, " ")
		} else {
			h.env.CFLAGS = strings.Join(args, " ")
		}
	}

	return nil
}

// AppendCxxflags appends flags to CXXFLAGS.
//
// Usage: append-cxxflags -O2 -march=native
func (h *Helpers) AppendCxxflags(args []string) error {
	h.cxxflags = append(h.cxxflags, args...)

	// Update environment if available
	if h.env != nil {
		if h.env.CXXFLAGS != "" {
			h.env.CXXFLAGS += " " + strings.Join(args, " ")
		} else {
			h.env.CXXFLAGS = strings.Join(args, " ")
		}
	}

	return nil
}

// AppendLdflags appends flags to LDFLAGS.
//
// Usage: append-ldflags -Wl,-rpath,/usr/lib
func (h *Helpers) AppendLdflags(args []string) error {
	h.ldflags = append(h.ldflags, args...)

	// Update environment if available
	if h.env != nil {
		if h.env.LDFLAGS != "" {
			h.env.LDFLAGS += " " + strings.Join(args, " ")
		} else {
			h.env.LDFLAGS = strings.Join(args, " ")
		}
	}

	return nil
}

// AppendFlags appends flags to both CFLAGS and CXXFLAGS.
//
// Usage: append-flags -fPIC
func (h *Helpers) AppendFlags(args []string) error {
	if err := h.AppendCflags(args); err != nil {
		return err
	}
	return h.AppendCxxflags(args)
}

// FilterFlags removes matching flags from CFLAGS and CXXFLAGS.
//
// Usage: filter-flags -O* -march=*
func (h *Helpers) FilterFlags(args []string) error {
	h.cflags = filterFlagList(h.cflags, args)
	h.cxxflags = filterFlagList(h.cxxflags, args)

	// Update environment if available
	if h.env != nil {
		h.env.CFLAGS = strings.Join(filterFlagList(strings.Fields(h.env.CFLAGS), args), " ")
		h.env.CXXFLAGS = strings.Join(filterFlagList(strings.Fields(h.env.CXXFLAGS), args), " ")
	}

	return nil
}

// FilterLdflags removes matching flags from LDFLAGS.
//
// Usage: filter-ldflags -Wl,-rpath*
func (h *Helpers) FilterLdflags(args []string) error {
	h.ldflags = filterFlagList(h.ldflags, args)

	// Update environment if available
	if h.env != nil {
		h.env.LDFLAGS = strings.Join(filterFlagList(strings.Fields(h.env.LDFLAGS), args), " ")
	}

	return nil
}

// StripFlags removes all optimization and CPU flags.
//
// Usage: strip-flags
func (h *Helpers) StripFlags(args []string) error {
	patterns := []string{"-O*", "-march=*", "-mcpu=*", "-mtune=*"}
	return h.FilterFlags(patterns)
}

// ReplaceCpuFlags replaces CPU-specific flags.
//
// Usage: replace-cpu-flags i686 pentium4
func (h *Helpers) ReplaceCpuFlags(args []string) error {
	if len(args) < 2 {
		return nil
	}

	oldFlag := "-march=" + args[0]
	newFlag := "-march=" + args[1]

	if h.env != nil {
		h.env.CFLAGS = strings.ReplaceAll(h.env.CFLAGS, oldFlag, newFlag)
		h.env.CXXFLAGS = strings.ReplaceAll(h.env.CXXFLAGS, oldFlag, newFlag)
	}

	return nil
}

// IsFlagSupported checks if a compiler flag is supported.
//
// Usage: is-flag-supported -fstack-protector-strong && append-flags -fstack-protector-strong
func (h *Helpers) IsFlagSupported(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]
	cc := h.getEnvOrDefault("CC", "gcc")

	// Test if compiler accepts the flag
	cmd := exec.Command(cc, flag, "-x", "c", "-c", "-o", os.DevNull, "-")
	cmd.Stdin = strings.NewReader("int main(){return 0;}")
	if err := cmd.Run(); err != nil {
		return exitFalse()
	}

	return nil
}

// filterFlagList removes flags matching patterns from a list.
func filterFlagList(flags, patterns []string) []string {
	result := make([]string, 0, len(flags))
	for _, flag := range flags {
		keep := true
		for _, pattern := range patterns {
			if matchFlagPattern(flag, pattern) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, flag)
		}
	}
	return result
}

// matchFlagPattern checks if a flag matches a pattern (supports * wildcard).
func matchFlagPattern(flag, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(flag, prefix)
	}
	return flag == pattern
}

// ============================================================================
// linux-info.eclass Functions
// ============================================================================

// GetVersion returns the running kernel version.
//
// Usage: kernel_version=$(get_version)
func (h *Helpers) GetVersion(args []string) error {
	// Read from /proc/version or use uname
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		// Fallback to uname
		cmd := exec.Command("uname", "-r")
		output, err := cmd.Output()
		if err != nil {
			h.writeStdout("unknown")
			return nil
		}
		h.writeStdout(strings.TrimSpace(string(output)))
		return nil
	}

	// Parse version from /proc/version
	// Format: Linux version X.Y.Z-...
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		h.writeStdout(parts[2])
		return nil
	}

	h.writeStdout("unknown")
	return nil
}

// LinuxConfigExists checks if a kernel config option exists.
//
// Usage: linux_config_exists CONFIG_MODULES && echo "Modules enabled"
func (h *Helpers) LinuxConfigExists(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	configName := args[0]

	// Try to find kernel config
	configPaths := []string{
		"/proc/config.gz",
		"/boot/config-" + h.getKernelVersion(),
		"/usr/src/linux/.config",
	}

	for _, path := range configPaths {
		if content, err := h.readKernelConfig(path); err == nil {
			// Search for CONFIG_XXX=y or CONFIG_XXX=m
			needle := configName + "="
			if strings.Contains(content, needle) {
				return nil
			}
		}
	}

	return exitFalse()
}

// LinuxConfigSrc checks if the kernel source is available.
//
// Usage: linux_config_src_exists || die "Need kernel sources"
func (h *Helpers) LinuxConfigSrcExists(args []string) error {
	configPaths := []string{
		"/usr/src/linux/.config",
		"/lib/modules/" + h.getKernelVersion() + "/build/.config",
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}

	return exitFalse()
}

// RequireConfiguredKernel dies if kernel is not configured.
//
// Usage: require_configured_kernel
func (h *Helpers) RequireConfiguredKernel(args []string) error {
	if err := h.LinuxConfigSrcExists(nil); err != nil {
		return &DieError{Message: "require_configured_kernel: kernel is not configured"}
	}
	return nil
}

func (h *Helpers) getKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (h *Helpers) readKernelConfig(path string) (string, error) {
	if strings.HasSuffix(path, ".gz") {
		// Would need to decompress - for now skip
		return "", fmt.Errorf("gzip not supported")
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// ============================================================================
// EXPORT_FUNCTIONS Support
// ============================================================================

// ExportFunctions registers phase functions from an eclass.
//
// Usage: EXPORT_FUNCTIONS src_compile src_install
//
// Makes eclass_src_compile be called when src_compile is invoked.
func (h *Helpers) ExportFunctions(args []string) error {
	for _, phase := range args {
		if err := h.eclassRegistry.ExportFunction(phase); err != nil {
			return &DieError{Message: fmt.Sprintf("EXPORT_FUNCTIONS: %v", err)}
		}
	}
	return nil
}

// ============================================================================
// Additional Utility Functions
// ============================================================================

// Eqawarn prints a QA warning message.
//
// Usage: eqawarn "QA Notice: something is wrong"
func (h *Helpers) Eqawarn(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorYellow + "QA Notice: " + colorReset + msg + "\n")
	return nil
}

// GetEclassRegistry returns the eclass registry.
func (h *Helpers) GetEclassRegistry() *EclassRegistry {
	return h.eclassRegistry
}

// GetEclassStack returns the eclass stack.
func (h *Helpers) GetEclassStack() *EclassStack {
	return h.eclassStack
}

// SetEclassRegistry sets the eclass registry (for testing).
func (h *Helpers) SetEclassRegistry(registry *EclassRegistry) {
	h.eclassRegistry = registry
}

// Edosym creates a relative symlink suitable for installation.
//
// Usage: edosym target linkname
//
// EAPI 8 replacement for dosym that uses relative paths.
func (h *Helpers) Edosym(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "edosym: requires target and linkname"}
	}

	target := args[0]
	linkname := args[1]

	// Calculate relative path from linkname to target
	linkDir := filepath.Dir(linkname)
	relTarget, err := filepath.Rel(linkDir, target)
	if err != nil {
		// Fall back to absolute target
		relTarget = target
	}

	return h.Dosym([]string{relTarget, linkname})
}

// HasVersion checks if a package is installed.
//
// Usage: has_version ">=sys-libs/zlib-1.2" && echo "zlib installed"
//
// Queries the package database (VarDB) to check if a package matching
// the given atom is installed. Supports version operators (>=, <=, >, <, =).
//
// Returns exit code 0 (nil) if found, exit code 1 (exitFalse) if not found.
func (h *Helpers) HasVersion(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	atom := args[0]

	// Check if package database is available
	if h.pkgDB == nil {
		// No database available - cannot query, assume not installed
		return exitFalse()
	}

	// Parse the atom and check against installed packages
	found := h.checkPackageInstalled(atom)
	if found {
		return nil // Exit code 0 - found
	}
	return exitFalse() // Exit code 1 - not found
}

// BestVersion returns the best installed version of a package.
//
// Usage: best=$(best_version sys-libs/zlib)
//
// Queries the package database (VarDB) and outputs the best (highest)
// installed version of the specified package to stdout.
//
// Output format: category/package-version (e.g., "sys-libs/zlib-1.2.13")
// If no matching package is installed, outputs nothing.
func (h *Helpers) BestVersion(args []string) error {
	if len(args) < 1 {
		return nil
	}

	atom := args[0]

	// Check if package database is available
	if h.pkgDB == nil {
		// No database available - output nothing
		return nil
	}

	// Find the best version
	bestPkg := h.findBestVersion(atom)
	if bestPkg != nil {
		// Output the full package atom (category/name-version)
		h.writeStdout(bestPkg.Package.ID())
	}

	return nil
}

// atomPattern matches Portage atom specifications.
// Examples: sys-libs/zlib, >=sys-libs/zlib-1.2, =app-misc/hello-2.10
var atomPattern = regexp.MustCompile(`^([<>=!~]*)([a-zA-Z0-9]+-[a-zA-Z0-9]+/[a-zA-Z0-9_+-]+)(?:-([0-9].*))?$`)

// parseAtom parses a Portage atom specification into its components.
//
// Atom format: [operator]category/name[-version]
// Examples:
//   - sys-libs/zlib -> (none, sys-libs/zlib, "")
//   - >=sys-libs/zlib-1.2 -> (>=, sys-libs/zlib, 1.2)
//   - =app-misc/hello-2.10 -> (=, app-misc/hello, 2.10)
//
// Returns operator (>=, <=, >, <, =, or ""), package name, and version.
func parseAtom(atom string) (operator, name, version string) {
	// Remove any slot specification (:slot)
	if idx := strings.Index(atom, ":"); idx != -1 {
		atom = atom[:idx]
	}

	// Remove any USE flag requirements ([use])
	if idx := strings.Index(atom, "["); idx != -1 {
		atom = atom[:idx]
	}

	// Extract operator prefix
	for _, op := range []string{">=", "<=", ">", "<", "=", "~", "!"} {
		if strings.HasPrefix(atom, op) {
			operator = op
			atom = strings.TrimPrefix(atom, op)
			break
		}
	}

	// Match the remaining atom
	matches := atomPattern.FindStringSubmatch(atom)
	if matches == nil {
		// Try simpler parsing for atoms without version
		// Format: category/name
		if strings.Contains(atom, "/") && !strings.Contains(atom, "-") {
			return operator, atom, ""
		}

		// Try to extract version from end
		// Find last dash followed by a digit
		lastDash := -1
		for i := len(atom) - 1; i >= 0; i-- {
			if atom[i] == '-' && i+1 < len(atom) && atom[i+1] >= '0' && atom[i+1] <= '9' {
				lastDash = i
				break
			}
		}

		if lastDash != -1 {
			return operator, atom[:lastDash], atom[lastDash+1:]
		}

		return operator, atom, ""
	}

	// matches[1] = any operator in the middle (should be empty after prefix extraction)
	// matches[2] = category/name
	// matches[3] = version (optional)
	name = matches[2]
	if len(matches) > 3 {
		version = matches[3]
	}

	return operator, name, version
}

// checkPackageInstalled checks if a package matching the atom is installed.
func (h *Helpers) checkPackageInstalled(atom string) bool {
	operator, name, version := parseAtom(atom)

	// Get all installed packages matching the name pattern
	packages := h.pkgDB.List()

	for _, installed := range packages {
		if installed.Package == nil {
			continue
		}

		// Check if package name matches
		if installed.Package.Name != name {
			continue
		}

		// If no version specified, any version matches
		if version == "" {
			return true
		}

		// Check version constraint
		constraint := buildConstraint(operator, version)
		if constraint != nil && constraint.Satisfies(installed.Package.Version) {
			return true
		}

		// If no operator, require exact match
		if operator == "" && installed.Package.Version == version {
			return true
		}
	}

	return false
}

// findBestVersion finds the highest installed version of a package.
func (h *Helpers) findBestVersion(atom string) *stateInstalledPackage {
	_, name, _ := parseAtom(atom)

	// Get all installed packages
	packages := h.pkgDB.List()

	var bestPkg *stateInstalledPackage
	var bestVersion string

	for _, installed := range packages {
		if installed.Package == nil {
			continue
		}

		// Check if package name matches
		if installed.Package.Name != name {
			continue
		}

		// Compare versions - keep the highest
		if bestPkg == nil || pkg.CompareVersions(installed.Package.Version, bestVersion) > 0 {
			bestPkg = installed
			bestVersion = installed.Package.Version
		}
	}

	return bestPkg
}

// buildConstraint creates a version constraint from operator and version string.
func buildConstraint(operator, version string) *pkg.VersionConstraint {
	switch operator {
	case ">=":
		return pkg.NewMinVersionConstraint(version)
	case "<=":
		return pkg.NewMaxVersionConstraint(version)
	case ">":
		return pkg.NewVersionConstraint(pkg.OpGreater, version)
	case "<":
		return pkg.NewVersionConstraint(pkg.OpLess, version)
	case "=", "~":
		return pkg.NewExactVersionConstraint(version)
	default:
		return nil
	}
}

// GetAlternative implements get_alternative() from app-alternatives.eclass.
// Returns the USE flag name for the selected alternative.
func (h *Helpers) GetAlternative(_ []string) error {
	// ALTERNATIVES is an array in bash, but we have it as space-separated string.
	// Each entry is "flagname:provider" — we check which flag is enabled.
	alts := h.getEnvVar("ALTERNATIVES")
	if alts == "" {
		// Try to find the first enabled USE flag as fallback
		h.writeStdout("reference")
		return nil
	}

	for _, alt := range strings.Fields(alts) {
		flag := alt
		if idx := strings.Index(alt, ":"); idx >= 0 {
			flag = alt[:idx]
		}

		// Check if this USE flag is enabled
		if h.isUseEnabled(flag) {
			h.writeStdout(flag)
			return nil
		}
	}

	// Default: return first alternative
	first := strings.Fields(alts)[0]
	if idx := strings.Index(first, ":"); idx >= 0 {
		first = first[:idx]
	}
	h.writeStdout(first)
	return nil
}
