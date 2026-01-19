// Package ebuild implements ebuild execution engine.
//
// This file provides the Interpreter type which uses mvdan.cc/sh as a pure-Go
// bash interpreter for executing ebuild scripts without external bash dependency.
package ebuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/grpmsoft/grpm/internal/state"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Interpreter executes bash scripts with Portage helper functions.
// Uses mvdan.cc/sh as embedded pure-Go bash interpreter.
//
// Example:
//
//	env := &Environment{USE: "ssl zlib", IUSE: "ssl zlib doc"}
//	interp := NewInterpreter(env, os.Stdout, os.Stderr)
//	err := interp.Run(ctx, `use ssl && echo "SSL enabled"`)
type Interpreter struct {
	env     *Environment
	stdout  io.Writer
	stderr  io.Writer
	helpers *Helpers
}

// NewInterpreter creates a new bash interpreter for ebuild execution.
//
// Parameters:
//   - env: Ebuild environment with package variables
//   - stdout: Standard output writer (for einfo, usev, etc.)
//   - stderr: Standard error writer (for ewarn, eerror, etc.)
func NewInterpreter(env *Environment, stdout, stderr io.Writer) *Interpreter {
	i := &Interpreter{
		env:    env,
		stdout: stdout,
		stderr: stderr,
	}
	i.helpers = NewHelpers(env, stdout, stderr)

	// Wire up the eclass loader to resolve circular dependency.
	// EclassLoader needs Interpreter, and Helpers.Inherit needs EclassLoader.
	eclassLoader := NewEclassLoader(i.helpers.eclassRegistry, i)
	i.helpers.SetEclassLoader(eclassLoader)

	// Wire up the command dispatcher for nonfatal support.
	// This allows nonfatal to execute helper commands through the interpreter.
	i.helpers.SetCommandDispatcher(i.dispatchCommand)

	return i
}

// SetPackageDatabase sets the package database for has_version/best_version queries.
//
// This allows the interpreter to query the system's installed package database (VarDB)
// when ebuilds use has_version or best_version commands.
func (i *Interpreter) SetPackageDatabase(db *state.PackageDatabase) {
	i.helpers.SetPackageDatabase(db)
}

// Run executes a bash script string.
//
// The script is parsed and executed with the ebuild environment variables
// available and Portage helper commands intercepted by the exec handler.
func (i *Interpreter) Run(ctx context.Context, script string) error {
	// Parse the script with bash variant for full ebuild compatibility
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "script")
	if err != nil {
		return fmt.Errorf("parsing script: %w", err)
	}

	// Create the runner with our exec handler
	runner, err := i.createRunner(ctx)
	if err != nil {
		return fmt.Errorf("creating runner: %w", err)
	}

	// Execute the program
	if err := runner.Run(ctx, prog); err != nil {
		return fmt.Errorf("executing script: %w", err)
	}

	return nil
}

// RunFile executes a bash script file.
//
// The file is read, parsed, and executed with the ebuild environment.
func (i *Interpreter) RunFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading script file %s: %w", path, err)
	}

	return i.Run(ctx, string(content))
}

// createRunner creates a new interp.Runner with ebuild configuration.
func (i *Interpreter) createRunner(ctx context.Context) (*interp.Runner, error) {
	// Build environment variables from Environment struct
	envPairs := i.buildEnvPairs()

	// Build runner options
	opts := []interp.RunnerOption{
		interp.StdIO(nil, i.stdout, i.stderr),
		interp.Env(expand.ListEnviron(envPairs...)),
		interp.ExecHandlers(i.execHandler),
	}

	// Set working directory to $S (source directory) if available and exists.
	// This is critical for commands like dodoc that reference files
	// relative to the source directory.
	if i.env != nil && i.env.S != "" {
		if info, err := os.Stat(i.env.S); err == nil && info.IsDir() {
			opts = append(opts, interp.Dir(i.env.S))
		}
	}

	// Create runner with options
	runner, err := interp.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating interpreter: %w", err)
	}

	return runner, nil
}

// buildEnvPairs converts Environment to slice of "KEY=VALUE" strings.
func (i *Interpreter) buildEnvPairs() []string {
	if i.env == nil {
		return nil
	}

	envMap := i.env.ToMap()
	pairs := make([]string, 0, len(envMap)+4)

	for k, v := range envMap {
		pairs = append(pairs, k+"="+v)
	}

	// Add IUSE if not present (for USE flag checking)
	// IUSE contains the list of valid USE flags for the package
	if i.env.Package != nil && len(i.env.Package.UseFlags) > 0 {
		iuse := make([]string, 0, len(i.env.Package.UseFlags))
		for flag := range i.env.Package.UseFlags {
			iuse = append(iuse, flag)
		}
		pairs = append(pairs, "IUSE="+strings.Join(iuse, " "))
	}

	return pairs
}

// helperFunc is a function signature for helper command handlers.
type helperFunc func(args []string) error

// buildCommandMap creates the command dispatch map for the interpreter.
// Extracted to reduce cyclomatic complexity of execHandler.
//
//nolint:gocyclo // Map initialization is inherently linear
func (i *Interpreter) buildCommandMap() map[string]helperFunc {
	return map[string]helperFunc{
		// Messaging functions
		"die":      i.helpers.Die,
		"assert":   i.helpers.Assert, // PMS Section 12.3.6 - error handling
		"einfo":    i.helpers.Einfo,
		"einfon":   i.helpers.Einfon, // PMS Section 12.3.5 - no trailing newline
		"ewarn":    i.helpers.Ewarn,
		"eerror":   i.helpers.Eerror,
		"elog":     i.helpers.Elog,
		"ebegin":   i.helpers.Ebegin,
		"eend":     i.helpers.Eend,
		"nonfatal": i.helpers.Nonfatal, // PMS Section 12.3.1 - EAPI 4+

		// Debug functions (PMS Section 12.3.16)
		"debug-print":          i.helpers.DebugPrint,
		"debug-print-function": i.helpers.DebugPrintFunction,
		"debug-print-section":  i.helpers.DebugPrintSection,

		// USE flag functions
		"has":        i.helpers.Has,
		"use":        i.helpers.Use,
		"usev":       i.helpers.Usev,
		"usex":       i.helpers.Usex,
		"in_iuse":    i.helpers.InIuse,
		"use_enable": i.helpers.UseEnable,
		"use_with":   i.helpers.UseWith,

		// Toolchain functions
		"tc-getCC":  i.helpers.TcGetCC,
		"tc-getCXX": i.helpers.TcGetCXX,
		"tc-getLD":  i.helpers.TcGetLD,
		"tc-arch":   i.helpers.TcArch,

		// Directory setting functions
		"into":    i.helpers.Into, // PMS Section 12.3.10 - sets DESTTREE
		"insinto": i.helpers.Insinto,
		"exeinto": i.helpers.Exeinto,
		"docinto": i.helpers.Docinto,

		// Option setting functions
		"insopts": i.helpers.Insopts,
		"exeopts": i.helpers.Exeopts,
		"diropts": i.helpers.Diropts,

		// Binary installation functions
		"dobin":   i.helpers.Dobin,
		"dosbin":  i.helpers.Dosbin,
		"newbin":  i.helpers.Newbin,
		"newsbin": i.helpers.Newsbin,
		"doexe":   i.helpers.Doexe,

		// File installation functions
		"doins":  i.helpers.Doins,
		"newins": i.helpers.Newins,

		// Documentation functions
		"dodoc":  i.helpers.Dodoc,
		"newdoc": i.helpers.Newdoc,
		"doman":  i.helpers.Doman,
		"newman": i.helpers.Newman,
		"doinfo": i.helpers.Doinfo,
		"domo":   i.helpers.Domo, // PMS Section 12.3.9 - gettext .mo files

		// Library/header installation functions
		"dolib":    i.helpers.Dolib,
		"dolib.so": i.helpers.DolibSo,
		"dolib.a":  i.helpers.DolibA,
		"doheader": i.helpers.Doheader,

		// Directory creation functions
		"dodir":   i.helpers.Dodir,
		"keepdir": i.helpers.Keepdir,

		// Build helper functions (Stage 3)
		"emake":  i.helpers.Emake,
		"econf":  i.helpers.Econf,
		"unpack": i.helpers.Unpack,
		"eapply": i.helpers.Eapply,

		// User patch function
		"eapply_user": i.helpers.EapplyUser,

		// Default phase functions (PMS Section 9.1.17 / 12.3.15)
		"default":               i.helpers.Default,
		"default_pkg_nofetch":   i.helpers.DefaultPkgNofetch,
		"default_src_unpack":    i.helpers.DefaultSrcUnpack,
		"default_src_prepare":   i.helpers.DefaultSrcPrepare,
		"default_src_configure": i.helpers.DefaultSrcConfigure,
		"default_src_compile":   i.helpers.DefaultSrcCompile,
		"default_src_test":      i.helpers.DefaultSrcTest,
		"default_src_install":   i.helpers.DefaultSrcInstall,

		// Version manipulation functions (EAPI 7+)
		"ver_cut":  i.helpers.VerCut,
		"ver_rs":   i.helpers.VerRs,
		"ver_test": i.helpers.VerTest,

		// Additional installation helpers
		"dosym":        i.helpers.Dosym,
		"fperms":       i.helpers.Fperms,
		"fowners":      i.helpers.Fowners,
		"doconfd":      i.helpers.Doconfd,
		"doinitd":      i.helpers.Doinitd,
		"doenvd":       i.helpers.Doenvd,
		"dostrip":      i.helpers.Dostrip,
		"einstalldocs": i.helpers.Einstalldocs,
		"inherit":      i.helpers.Inherit,

		// File system utilities (common in ebuilds)
		"sed":        i.helpers.Sed,
		"cat":        i.helpers.Cat,
		"mkdir":      i.helpers.Mkdir,
		"rm":         i.helpers.Rm,
		"cp":         i.helpers.Cp,
		"mv":         i.helpers.Mv,
		"chmod":      i.helpers.Chmod,
		"ln":         i.helpers.Ln,
		"find":       i.helpers.Find,
		"grep":       i.helpers.Grep,
		"xargs":      i.helpers.Xargs,
		"which":      i.helpers.Which,
		"touch":      i.helpers.Touch,
		"install":    i.helpers.Install,
		"pkg-config": i.helpers.PkgConfig,

		// Eclass support functions (Stage 4)

		// eutils.eclass functions
		"epatch":       i.helpers.Epatch,
		"eshopts_push": i.helpers.EshoptsPush,
		"eshopts_pop":  i.helpers.EshoptsPop,
		"estack_push":  i.helpers.EstackPush,
		"estack_pop":   i.helpers.EstackPop,

		// toolchain-funcs.eclass additions
		"tc-is-gcc":      i.helpers.TcIsGcc,
		"tc-is-clang":    i.helpers.TcIsClang,
		"tc-export":      i.helpers.TcExport,
		"tc-getAR":       i.helpers.TcGetAR,
		"tc-getRANLIB":   i.helpers.TcGetRANLIB,
		"tc-getNM":       i.helpers.TcGetNM,
		"tc-getSTRIP":    i.helpers.TcGetSTRIP,
		"tc-getOBJCOPY":  i.helpers.TcGetOBJCOPY,
		"tc-getBUILD_CC": i.helpers.TcGetBUILD_CC,
		"tc-endian":      i.helpers.TcEndianBig, // Alias, real check uses args

		// multilib.eclass functions
		"get_libdir":                 i.helpers.GetLibdir,
		"multilib_native_use_with":   i.helpers.MultilibNativeUseWith,
		"multilib_native_use_enable": i.helpers.MultilibNativeUseEnable,

		// flag-o-matic.eclass functions
		"append-cflags":           i.helpers.AppendCflags,
		"append-cxxflags":         i.helpers.AppendCxxflags,
		"append-cppflags":         i.helpers.AppendCppflags,
		"append-ldflags":          i.helpers.AppendLdflags,
		"append-flags":            i.helpers.AppendFlags,
		"append-lfs-flags":        i.helpers.AppendLfsFlags,
		"filter-flags":            i.helpers.FilterFlags,
		"filter-ldflags":          i.helpers.FilterLdflags,
		"filter-lfs-flags":        i.helpers.FilterLfsFlags,
		"replace-flags":           i.helpers.ReplaceFlagsImpl,
		"replace-cpu-flags":       i.helpers.ReplaceCpuFlags,
		"strip-flags":             i.helpers.StripFlags,
		"strip-unsupported-flags": i.helpers.StripUnsupportedFlags,
		"test-flags-CC":           i.helpers.TestFlagsCC,
		"test-flags-CXX":          i.helpers.TestFlagsCXX,
		"test-flags-F77":          i.helpers.TestFlagsF77,
		"test-flags-FC":           i.helpers.TestFlagsFC,
		"test-flags":              i.helpers.TestFlagsAll,
		"get-flag":                i.helpers.GetFlag,
		"is-flag":                 i.helpers.IsFlag,
		"is-ldflag":               i.helpers.IsLdflag,
		"is-flag-supported":       i.helpers.IsFlagSupported,
		"no-as-needed":            i.helpers.NoAsNeeded,
		"raw-ldflags":             i.helpers.RawLdflags,

		// linux-info.eclass functions
		"get_version":               i.helpers.GetVersion,
		"linux_config_exists":       i.helpers.LinuxConfigExists,
		"linux_config_src_exists":   i.helpers.LinuxConfigSrcExists,
		"require_configured_kernel": i.helpers.RequireConfiguredKernel,

		// Additional eclass functions
		"EXPORT_FUNCTIONS": i.helpers.ExportFunctions,
		"eqawarn":          i.helpers.Eqawarn,
		"edosym":           i.helpers.Edosym,
		"has_version":      i.helpers.HasVersion,
		"best_version":     i.helpers.BestVersion,

		// cmake.eclass functions
		"cmake":                          i.helpers.Cmake,
		"cmake_src_prepare":              i.helpers.CmakeSrcPrepare,
		"cmake_src_configure":            i.helpers.CmakeSrcConfigure,
		"cmake_src_compile":              i.helpers.CmakeSrcCompile,
		"cmake_src_test":                 i.helpers.CmakeSrcTest,
		"cmake_src_install":              i.helpers.CmakeSrcInstall,
		"cmake_use":                      i.helpers.CmakeUse,
		"cmake_use_find_package":         i.helpers.CmakeUseFindPackage,
		"cmake_comment_add_subdirectory": i.helpers.CmakeCommentAddSubdirectory,
		"cmake_run_in":                   i.helpers.CmakeRunIn,
		"cmake_build_type":               i.helpers.CmakeBuildType,
		"cmake_multilib_src_configure":   i.helpers.CmakeMultilibSrcConfigure,
		"eninja":                         i.helpers.Eninja,

		// meson.eclass functions
		"meson":               i.helpers.Meson,
		"meson_src_configure": i.helpers.MesonSrcConfigure,
		"meson_src_compile":   i.helpers.MesonSrcCompile,
		"meson_src_test":      i.helpers.MesonSrcTest,
		"meson_src_install":   i.helpers.MesonSrcInstall,
		"meson_use":           i.helpers.MesonUse,
		"meson_feature":       i.helpers.MesonFeature,
		"meson_use_bool":      i.helpers.MesonUseBool,

		// Banned commands (PMS Section 12.3.2 / Table 12.3)
		// These stubs check EAPI and return appropriate errors.
		"dohard":   i.helpers.Dohard,   // Banned in EAPI 4+
		"dosed":    i.helpers.Dosed,    // Banned in EAPI 4+
		"useq":     i.helpers.Useq,     // Banned in EAPI 5+
		"einstall": i.helpers.Einstall, // Banned in EAPI 6+
		"dohtml":   i.helpers.Dohtml,   // Banned in EAPI 7+
		"libopts":  i.helpers.Libopts,  // Banned in EAPI 7+
		"hasv":     i.helpers.Hasv,     // Banned in EAPI 8+
		"hasq":     i.helpers.Hasq,     // Banned in EAPI 8+
	}
}

// execHandler intercepts Portage commands and delegates to Go implementations.
//
// Commands handled:
//   - die, assert, einfo, ewarn, eerror, elog, ebegin, eend, eqawarn (messaging)
//   - has, use, usev, usex, in_iuse, use_enable, use_with (USE flags)
//   - tc-getCC, tc-getCXX, tc-getLD, tc-arch (toolchain-funcs)
//   - tc-is-gcc, tc-is-clang, tc-export, tc-getAR, etc. (toolchain-funcs)
//   - insinto, exeinto, docinto (directory setting)
//   - insopts, exeopts, diropts (option setting)
//   - dobin, dosbin, newbin, newsbin, doexe (binary installation)
//   - doins, newins (file installation)
//   - dodoc, newdoc, doman, newman (documentation)
//   - dolib, dolib.so, dolib.a, doheader (library/header installation)
//   - dodir, keepdir (directory creation)
//   - emake, econf, unpack, eapply, eapply_user (build helpers)
//   - default, default_src_* (default phase implementations)
//   - ver_cut, ver_rs, ver_test (version manipulation)
//   - dosym, edosym, fperms, fowners, doconfd, doinitd, doenvd (installation)
//   - sed, cat, mkdir, rm, cp, mv, chmod, ln, find, grep, xargs, etc. (utilities)
//   - epatch, eshopts_push, eshopts_pop, estack_push, estack_pop (eutils)
//   - get_libdir, multilib_native_use_* (multilib)
//   - append-*, filter-*, replace-*, strip-*, test-flags-*, get-flag, is-flag, is-ldflag, no-as-needed, raw-ldflags (flag-o-matic)
//   - get_version, linux_config_exists (linux-info)
//   - inherit, EXPORT_FUNCTIONS (eclass support)
//   - has_version, best_version (package queries)
//   - cmake_*, eninja (cmake.eclass)
//   - meson_*, meson (meson.eclass)
//
// Unhandled commands are passed to the next handler (real shell execution).
func (i *Interpreter) execHandler(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	// Build command map once
	commands := i.buildCommandMap()

	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return next(ctx, args)
		}

		cmd := args[0]
		cmdArgs := args[1:]
		hc := interp.HandlerCtx(ctx)

		// Special handling for xargs - needs stdin from context
		if cmd == "xargs" {
			return i.helpers.XargsWithStdin(hc.Stdin, cmdArgs)
		}

		// Special handling for inherit - needs environment from context
		if cmd == "inherit" {
			return i.helpers.InheritWithEnv(cmdArgs, hc.Env)
		}

		// Version functions need to write to context stdout for command substitution
		// to work (e.g., GCC_RELEASE_VER=$(ver_cut 1-3 ${GCC_PV}))
		switch cmd {
		case "ver_cut":
			if len(cmdArgs) >= 2 {
				result, err := i.helpers.verCutImpl(cmdArgs[0], cmdArgs[1])
				if err != nil {
					return &DieError{Message: fmt.Sprintf("ver_cut: %v", err)}
				}
				_, _ = io.WriteString(hc.Stdout, result)
				return nil
			}
			return &DieError{Message: "ver_cut: requires range and version arguments"}
		case "ver_rs":
			if len(cmdArgs) >= 3 {
				result := i.helpers.verRsImpl(cmdArgs[0], cmdArgs[1], cmdArgs[2])
				_, _ = io.WriteString(hc.Stdout, result)
				return nil
			}
			return &DieError{Message: "ver_rs: requires range, separator, and version arguments"}
		}

		// Look up command in map
		if handler, ok := commands[cmd]; ok {
			return handler(cmdArgs)
		}

		// Pass through to next handler (real shell execution)
		return next(ctx, args)
	}
}

// GetHelpers returns the helpers instance for direct access.
//
// This allows external code to call helper functions directly without
// going through the interpreter.
func (i *Interpreter) GetHelpers() *Helpers {
	return i.helpers
}

// dispatchCommand executes a helper command by name.
//
// This is used by the nonfatal helper to execute commands through the
// interpreter's command dispatch mechanism. It looks up the command in
// the command map and executes it.
func (i *Interpreter) dispatchCommand(cmd string, args []string) error {
	commands := i.buildCommandMap()

	// Skip nonfatal itself to avoid recursion
	if cmd == "nonfatal" {
		return fmt.Errorf("nonfatal: cannot call nonfatal recursively")
	}

	// Look up the command
	handler, ok := commands[cmd]
	if !ok {
		// Command not found in our handlers - return exit status 127
		// This matches shell behavior for command not found
		return interp.ExitStatus(127)
	}

	// Execute the command
	return handler(args)
}

// Eval evaluates a bash expression and returns its output.
//
// This is useful for evaluating variable expansions or command substitutions.
func (i *Interpreter) Eval(ctx context.Context, expr string) (string, error) {
	var buf bytes.Buffer

	// Create a temporary interpreter with captured output
	tempInterp := &Interpreter{
		env:     i.env,
		stdout:  &buf,
		stderr:  i.stderr,
		helpers: NewHelpers(i.env, &buf, i.stderr),
	}

	// Wrap expression in echo to capture output
	script := fmt.Sprintf("echo -n %s", expr)
	if err := tempInterp.Run(ctx, script); err != nil {
		return "", err
	}

	return buf.String(), nil
}
