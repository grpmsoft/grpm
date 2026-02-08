// Package ebuild implements ebuild execution engine.
//
// This file provides the Interpreter type which uses mvdan.cc/sh as a pure-Go
// bash interpreter for executing ebuild scripts without external bash dependency.
//
// Panic recovery: All interpreter entry points (Run, RunFile, Eval) use
// defer/recover to catch panics from unsupported bash constructs in mvdan.cc/sh
// (e.g., ${!var@a}, complex array operations). Panics are converted to
// descriptive errors instead of crashing the emerge process.
package ebuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/state"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// InterpreterPanicError is returned when the bash interpreter panics on an
// unsupported construct. It wraps the panic value with context about what
// was being executed when the panic occurred.
type InterpreterPanicError struct {
	// PanicValue is the recovered panic value.
	PanicValue interface{}
	// Context describes what was being executed (e.g., "script", "eclass toolchain-funcs").
	Context string
}

// Error returns a descriptive error message for the panic.
func (e *InterpreterPanicError) Error() string {
	return fmt.Sprintf("interpreter panic in %s: %v (unsupported bash construct)", e.Context, e.PanicValue)
}

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
//
// Panics from unsupported bash constructs (e.g., ${!var@a}, ${var@Q}) in
// mvdan.cc/sh are caught and converted to descriptive errors.
func (i *Interpreter) Run(ctx context.Context, script string) (runErr error) {
	// Recover from panics caused by unsupported bash constructs in mvdan.cc/sh.
	// Common triggers: ${!var@a} (variable attributes), complex array operations.
	defer func() {
		if r := recover(); r != nil {
			logging.Debug("[ebuild] interpreter panic recovered: %v", r)
			runErr = &InterpreterPanicError{
				PanicValue: r,
				Context:    i.panicContext("script"),
			}
		}
	}()

	// Transform unsupported bash constructs (${VAR@a}, etc.) before parsing.
	script = preprocessScript(script)

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
// Panics from unsupported bash constructs are caught and converted to errors.
func (i *Interpreter) RunFile(ctx context.Context, path string) (runErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Debug("[ebuild] interpreter panic recovered in file %s: %v", path, r)
			runErr = &InterpreterPanicError{
				PanicValue: r,
				Context:    i.panicContext(fmt.Sprintf("file %s", path)),
			}
		}
	}()

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

		// app-alternatives.eclass functions
		"get_alternative": i.helpers.GetAlternative,

		// bash-completion-r1.eclass / shell-completion.eclass
		"get_bashcompdir": i.helpers.GetBashcompdir,
		"get_zshcompdir":  i.helpers.GetZshcompdir,
		"get_fishcompdir": i.helpers.GetFishcompdir,
		"dobashcomp":      i.helpers.DoBashcomp,
		"newbashcomp":     i.helpers.NewBashcomp,

		// autotools.eclass
		"eautoreconf": i.helpers.Eautoreconf,

		// prefix.eclass
		"eprefixify": i.helpers.Eprefixify,

		// linux-info.eclass additional
		"linux-info_pkg_setup": i.helpers.LinuxInfoPkgSetup,
		"kernel_is":            i.helpers.KernelIs,

		// distutils-r1.eclass additional
		"distutils_enable_tests": i.helpers.DistutilsEnableTests,

		// unpacker.eclass
		"unpacker_src_uri_depends": i.helpers.UnpackerNoOp,

		// llvm.org.eclass
		"llvm.org_set_globals": i.helpers.LlvmOrgNoOp,
		"llvm.org_src_prepare": i.helpers.LlvmOrgNoOp,

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

		// python-utils-r1.eclass functions
		"python_export":         i.helpers.PythonExport,
		"python_get_sitedir":    i.helpers.PythonGetSitedir,
		"python_get_includedir": i.helpers.PythonGetIncludedir,
		"python_get_library":    i.helpers.PythonGetLibrary,
		"python_get_scriptdir":  i.helpers.PythonGetScriptdir,
		"python_is_installed":   i.helpers.PythonIsInstalled,
		"python_is_compatible":  i.helpers.PythonIsCompatible,
		"python_wrapper":        i.helpers.PythonWrapper,
		"python_doexe":          i.helpers.PythonDoexe,
		"python_newexe":         i.helpers.PythonNewexe,
		"python_domodule":       i.helpers.PythonDomodule,

		// python-r1.eclass functions
		"python-r1_pkg_setup":       i.helpers.PythonR1PkgSetup,
		"python_foreach_impl":       i.helpers.PythonForeachImpl,
		"python_copy_sources":       i.helpers.PythonCopySources,
		"python_optimize":           i.helpers.PythonOptimize,
		"python_gen_any_dep":        i.helpers.PythonGenAnyDep,
		"python_set_active_version": i.helpers.PythonSetActiveVersion,

		// python-single-r1.eclass functions
		"python-single-r1_pkg_setup": i.helpers.PythonSingleR1PkgSetup,
		"python_setup":               i.helpers.PythonSetup,
		"python_gen_cond_dep":        i.helpers.PythonGenCondDep,
		"python_gen_usedep":          i.helpers.PythonGenUseDep,
		"python_gen_impl_dep":        i.helpers.PythonGenImplDep,

		// python-any-r1.eclass functions
		"python-any-r1_pkg_setup": i.helpers.PythonAnyR1PkgSetup,
		"python_check_deps":       i.helpers.PythonCheckDeps,

		// Internal python-utils-r1 functions (underscore-prefixed).
		// These are bash functions in the eclass that aren't preserved
		// across interpreter runs, so we provide Go stubs.
		"_python_set_impls":           i.helpers.PythonSetImpls,
		"_python_export":              i.helpers.PythonInternalExport,
		"_python_check_locale_sanity": i.helpers.PythonNoOp,
		"_python_set_provider_pkg":    i.helpers.PythonNoOp,

		// distutils-r1.eclass functions
		"distutils-r1_src_prepare":   i.helpers.DistutilsR1SrcPrepare,
		"distutils-r1_src_configure": i.helpers.DistutilsR1SrcConfigure,
		"distutils-r1_src_compile":   i.helpers.DistutilsR1SrcCompile,
		"distutils-r1_src_test":      i.helpers.DistutilsR1SrcTest,
		"distutils-r1_src_install":   i.helpers.DistutilsR1SrcInstall,
		"python_compile":             i.helpers.PythonCompile,
		"python_test":                i.helpers.PythonTest,
		"python_install":             i.helpers.PythonInstall,
		"python_install_all":         i.helpers.PythonInstallAll,

		// cargo.eclass functions
		"cargo_crate_uris":    i.helpers.CargoCrateUris,
		"cargo_src_unpack":    i.helpers.CargoSrcUnpack,
		"cargo_src_configure": i.helpers.CargoSrcConfigure,
		"cargo_src_compile":   i.helpers.CargoSrcCompile,
		"cargo_src_test":      i.helpers.CargoSrcTest,
		"cargo_src_install":   i.helpers.CargoSrcInstall,
		"cargo_env":           i.helpers.CargoEnv,

		// go-module.eclass functions
		"go-module_set_globals": i.helpers.GoModuleSetGlobals,
		"go-module_src_unpack":  i.helpers.GoModuleSrcUnpack,
		"go-module_src_compile": i.helpers.GoModuleSrcCompile,
		"go-module_src_install": i.helpers.GoModuleSrcInstall,
		"ego":                   i.helpers.Ego,

		// multilib-build.eclass functions (multilib-minimal)
		"multilib-minimal_src_configure": i.helpers.MultilibBuildSrcConfigure,
		"multilib-minimal_src_compile":   i.helpers.MultilibBuildSrcCompile,
		"multilib-minimal_src_test":      i.helpers.MultilibBuildSrcTest,
		"multilib-minimal_src_install":   i.helpers.MultilibBuildSrcInstall,
		"multilib_foreach_abi":           i.helpers.MultilibForeachABI,
		"multilib_native_usedep":         i.helpers.MultilibUsedep,
		"multilib_is_native_abi":         i.helpers.MultilibIsNativeABI,
		"get_all_abis":                   i.helpers.GetAllABIs,
		"get_abi_LIBDIR":                 i.helpers.GetABILibdir,
		"get_abi_CHOST":                  i.helpers.GetABIChost,
		"get_abi_CFLAGS":                 i.helpers.GetABICflags,
		"get_abi_LDFLAGS":                i.helpers.GetABILdflags,

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

		// Special handling for commands that need stdin from context
		if cmd == "xargs" {
			return i.helpers.XargsWithStdin(hc.Stdin, cmdArgs)
		}
		if cmd == "cat" {
			return i.helpers.CatWithStdin(hc.Stdin, cmdArgs)
		}
		// newins/newdoc/newman with "-" source reads from stdin (heredoc piping)
		if (cmd == "newins" || cmd == "newdoc" || cmd == "newman") && len(cmdArgs) >= 2 && cmdArgs[0] == "-" {
			return i.helpers.NewinsFromStdin(hc.Stdin, cmdArgs[1], cmd)
		}

		// Special handling for inherit - needs environment from context
		if cmd == "inherit" {
			return i.helpers.InheritWithEnv(cmdArgs, hc.Env)
		}

		// Version functions need to write to context stdout for command substitution
		// to work (e.g., GCC_RELEASE_VER=$(ver_cut 1-3 ${GCC_PV}))
		switch cmd {
		case "ver_cut":
			if len(cmdArgs) < 1 {
				return &DieError{Message: "ver_cut: requires range argument"}
			}
			version := ""
			if len(cmdArgs) >= 2 {
				version = cmdArgs[1]
			} else {
				// Default to $PV per Portage spec
				version = hc.Env.Get("PV").String()
			}
			result, err := i.helpers.verCutImpl(cmdArgs[0], version)
			if err != nil {
				return &DieError{Message: fmt.Sprintf("ver_cut: %v", err)}
			}
			_, _ = io.WriteString(hc.Stdout, result)
			return nil
		case "ver_rs":
			if len(cmdArgs) < 2 {
				return &DieError{Message: "ver_rs: requires range and separator arguments"}
			}
			version := ""
			if len(cmdArgs) >= 3 {
				version = cmdArgs[2]
			} else {
				// Default to $PV per Portage spec
				version = hc.Env.Get("PV").String()
			}
			result := i.helpers.verRsImpl(cmdArgs[0], cmdArgs[1], version)
			_, _ = io.WriteString(hc.Stdout, result)
			return nil
		}

		// Internal command to sync bash variables back to Go environment
		if cmd == "__grpm_sync_env" {
			if i.env != nil {
				if s := hc.Env.Get("S").String(); s != "" {
					i.env.S = s
				}
				if workdir := hc.Env.Get("WORKDIR").String(); workdir != "" {
					i.env.WORKDIR = workdir
				}
			}
			return nil
		}

		// Look up command in map
		if handler, ok := commands[cmd]; ok {
			// Make runtime bash variables available to Go helpers.
			i.helpers.runtimeEnv = hc.Env
			i.helpers.runtimeDir = hc.Dir

			// Redirect helpers' stdout to context stdout for command substitution.
			// When bash does $(some_command), hc.Stdout is a capture pipe.
			// Without this, Go helpers write to the original stdout instead.
			origStdout := i.helpers.stdout
			i.helpers.stdout = hc.Stdout
			err := handler(cmdArgs)
			i.helpers.stdout = origStdout
			i.helpers.runtimeEnv = nil
			i.helpers.runtimeDir = ""
			return err
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

// panicContext builds a descriptive context string for panic error messages.
//
// Includes the current eclass name (if any) and package name for debugging.
func (i *Interpreter) panicContext(base string) string {
	parts := []string{base}

	// Add current eclass context if available
	if i.helpers != nil && i.helpers.eclassRegistry != nil {
		if ec := i.helpers.eclassRegistry.GetCurrentEclass(); ec != "" {
			parts = append(parts, fmt.Sprintf("eclass=%s", ec))
		}
	}

	// Add package name if available
	if i.env != nil && i.env.Package != nil {
		parts = append(parts, fmt.Sprintf("pkg=%s", i.env.Package.Name))
	}

	return strings.Join(parts, ", ")
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
// Panics from unsupported bash constructs are caught and converted to errors.
func (i *Interpreter) Eval(ctx context.Context, expr string) (result string, evalErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Debug("[ebuild] interpreter panic recovered in eval: %v", r)
			result = ""
			evalErr = &InterpreterPanicError{
				PanicValue: r,
				Context:    i.panicContext("eval"),
			}
		}
	}()

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

// paramExpansionRe matches ${VAR@op} parameter expansion operators
// unsupported by mvdan.cc/sh. Captures: $1=varname (with optional !), $2=operator.
var paramExpansionRe = regexp.MustCompile(`\$\{(!?[a-zA-Z_][a-zA-Z0-9_]*)@([aQEPAK])\}`)

// redirectRe matches ">& file" (redirect stdout+stderr to file) which mvdan.cc/sh
// doesn't support (it only handles ">& fd" for fd duplication). We transform it to
// "&> file" which is the equivalent bash syntax that mvdan.cc/sh handles correctly.
// The regex requires that >& is NOT preceded by a digit (to avoid matching n>&m fd dup)
// and IS followed by a non-digit (filename, not fd number).
var redirectRe = regexp.MustCompile(`(?m)(^|[^0-9])>&(\s*[^0-9\s&])`)

// preprocessScript transforms bash constructs unsupported by mvdan.cc/sh
// into equivalent forms that won't cause parser/runtime panics.
//
// Handles:
//   - ${VAR@a} → "a" (variable attributes; assume array for eclass compatibility)
//   - ${VAR@Q/E/P/A/K} → "" (other transformation operators)
//   - >& file → &> file (bash synonym, mvdan/sh panics on >& with filename)
//
// This is needed because Gentoo eclasses (e.g., app-alternatives.eclass) use
// ${ALTERNATIVES@a} to check if a variable is an array, which mvdan.cc/sh
// does not support and panics on.
func preprocessScript(script string) string {
	// Transform >& file → &> file (both mean redirect stdout+stderr to file).
	// mvdan.cc/sh panics on >& with a filename argument (it only supports
	// >& fd for file descriptor duplication). Bash treats >& file as &> file.
	if strings.Contains(script, ">&") {
		script = redirectRe.ReplaceAllString(script, "${1}&>${2}")
	}

	if !strings.Contains(script, "@") {
		return script
	}
	return paramExpansionRe.ReplaceAllStringFunc(script, func(match string) string {
		sub := paramExpansionRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		switch sub[2] {
		case "a":
			// @a returns variable attributes. In Gentoo eclasses, this checks
			// array declarations (e.g., ${ALTERNATIVES@a} should contain "a").
			// Returning "a" satisfies the check since ebuilds declare these as arrays.
			return "a"
		default:
			return ""
		}
	})
}
