package tools

// NewDefaultRegistry creates a Registry pre-populated with common external tools.
//
// This includes compilers, build systems, languages, utilities, and more.
// Tools are tagged with categories and eclasses that require them.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()

	// Register all default tools
	registerCompilers(r)
	registerBuildSystems(r)
	registerLanguages(r)
	registerUtilities(r)
	registerCompression(r)
	registerDocumentation(r)
	registerVCS(r)

	return r
}

// registerCompilers adds compiler tools to the registry.
func registerCompilers(r *Registry) {
	// GCC
	r.Register(NewTool(
		"gcc",
		"gcc",
		"sys-devel/gcc",
		"GNU Compiler Collection - C compiler",
	).WithCategories(CategoryCompiler).WithRequiredBy("toolchain-funcs"))

	r.Register(NewTool(
		"g++",
		"g++",
		"sys-devel/gcc",
		"GNU Compiler Collection - C++ compiler",
	).WithCategories(CategoryCompiler).WithRequiredBy("toolchain-funcs"))

	// Clang
	r.Register(NewTool(
		"clang",
		"clang",
		"sys-devel/clang",
		"LLVM C/C++ compiler",
	).WithCategories(CategoryCompiler).WithRequiredBy("toolchain-funcs").WithOptional())

	r.Register(NewTool(
		"clang++",
		"clang++",
		"sys-devel/clang",
		"LLVM C++ compiler",
	).WithCategories(CategoryCompiler).WithRequiredBy("toolchain-funcs").WithOptional())

	// Rust
	r.Register(NewTool(
		"rustc",
		"rustc",
		"dev-lang/rust",
		"Rust compiler",
	).WithCategories(CategoryCompiler, CategoryLanguage).WithRequiredBy("cargo", "rust"))

	// Go
	r.Register(NewTool(
		"go",
		"go",
		"dev-lang/go",
		"Go programming language compiler",
	).WithCategories(CategoryCompiler, CategoryLanguage).WithRequiredBy("go-module"))

	// Fortran
	r.Register(NewTool(
		"gfortran",
		"gfortran",
		"sys-devel/gcc",
		"GNU Fortran compiler",
	).WithCategories(CategoryCompiler).WithOptional())
}

// registerBuildSystems adds build system tools to the registry.
func registerBuildSystems(r *Registry) {
	// Make
	r.Register(NewTool(
		"make",
		"make",
		"sys-devel/make",
		"GNU Make build tool",
	).WithCategories(CategoryBuildSystem))

	// Ninja
	r.Register(NewTool(
		"ninja",
		"ninja",
		"dev-build/ninja",
		"Small build system with focus on speed",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("cmake", "meson"))

	// CMake
	r.Register(NewTool(
		"cmake",
		"cmake",
		"dev-build/cmake",
		"Cross-platform build system generator",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("cmake"))

	// Meson
	r.Register(NewTool(
		"meson",
		"meson",
		"dev-build/meson",
		"High performance build system",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("meson"))

	// Autotools
	r.Register(NewTool(
		"autoconf",
		"autoconf",
		"sys-devel/autoconf",
		"GNU Autoconf - configure script generator",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("autotools"))

	r.Register(NewTool(
		"automake",
		"automake",
		"sys-devel/automake",
		"GNU Automake - Makefile.in generator",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("autotools"))

	r.Register(NewTool(
		"libtool",
		"libtool",
		"sys-devel/libtool",
		"GNU Libtool - portable library support",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("autotools"))

	// pkg-config
	r.Register(NewTool(
		"pkg-config",
		"pkg-config",
		"dev-util/pkgconf",
		"Package configuration tool",
	).WithCategories(CategoryBuildSystem, CategoryUtility))

	// SCons
	r.Register(NewTool(
		"scons",
		"scons",
		"dev-build/scons",
		"Software construction tool (Python-based)",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("scons"))

	// Waf
	r.Register(NewTool(
		"waf",
		"waf",
		"dev-build/waf",
		"Python-based build system",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("waf"))

	// QMake
	r.Register(NewTool(
		"qmake",
		"qmake",
		"dev-qt/qtcore",
		"Qt build system",
	).WithCategories(CategoryBuildSystem).WithRequiredBy("qmake"))
}

// registerLanguages adds language runtime tools to the registry.
func registerLanguages(r *Registry) {
	// Python (multiple versions)
	r.Register(NewTool(
		"python",
		"python",
		"dev-lang/python",
		"Python interpreter",
	).WithCategories(CategoryLanguage).WithRequiredBy(
		"python-single-r1", "python-r1", "python-any-r1", "distutils-r1"))

	r.Register(NewTool(
		"python3",
		"python3",
		"dev-lang/python",
		"Python 3 interpreter",
	).WithCategories(CategoryLanguage).WithRequiredBy(
		"python-single-r1", "python-r1", "python-any-r1", "distutils-r1"))

	// Perl
	r.Register(NewTool(
		"perl",
		"perl",
		"dev-lang/perl",
		"Perl interpreter",
	).WithCategories(CategoryLanguage).WithRequiredBy("perl-module"))

	// Ruby
	r.Register(NewTool(
		"ruby",
		"ruby",
		"dev-lang/ruby",
		"Ruby interpreter",
	).WithCategories(CategoryLanguage).WithRequiredBy("ruby-ng"))

	// Node.js
	r.Register(NewTool(
		"node",
		"node",
		"net-libs/nodejs",
		"Node.js JavaScript runtime",
	).WithCategories(CategoryLanguage))

	r.Register(NewTool(
		"npm",
		"npm",
		"net-libs/nodejs",
		"Node.js package manager",
	).WithCategories(CategoryLanguage))

	// Lua
	r.Register(NewTool(
		"lua",
		"lua",
		"dev-lang/lua",
		"Lua interpreter",
	).WithCategories(CategoryLanguage).WithRequiredBy("lua"))

	// Java
	r.Register(NewTool(
		"java",
		"java",
		"virtual/jre",
		"Java runtime",
	).WithCategories(CategoryLanguage).WithRequiredBy("java-pkg-2", "java-pkg-simple"))

	r.Register(NewTool(
		"javac",
		"javac",
		"virtual/jdk",
		"Java compiler",
	).WithCategories(CategoryLanguage, CategoryCompiler).WithRequiredBy("java-pkg-2"))

	// Cargo (Rust)
	r.Register(NewTool(
		"cargo",
		"cargo",
		"dev-lang/rust",
		"Rust package manager",
	).WithCategories(CategoryLanguage, CategoryBuildSystem).WithRequiredBy("cargo"))
}

// registerUtilities adds utility tools to the registry.
func registerUtilities(r *Registry) {
	// Core utilities
	r.Register(NewTool(
		"patch",
		"patch",
		"sys-devel/patch",
		"Apply diff files to source code",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"diff",
		"diff",
		"sys-apps/diffutils",
		"Compare files line by line",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"sed",
		"sed",
		"sys-apps/sed",
		"Stream editor for text transformation",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"awk",
		"awk",
		"sys-apps/gawk",
		"Pattern scanning and processing language",
	).WithCategories(CategoryUtility))

	// Network utilities
	r.Register(NewTool(
		"wget",
		"wget",
		"net-misc/wget",
		"Network file retriever",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"curl",
		"curl",
		"net-misc/curl",
		"Command line tool for transferring data",
	).WithCategories(CategoryUtility))

	// File utilities
	r.Register(NewTool(
		"find",
		"find",
		"sys-apps/findutils",
		"Search for files in directory hierarchy",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"install",
		"install",
		"sys-apps/coreutils",
		"Copy files and set attributes",
	).WithCategories(CategoryUtility))

	// Development utilities
	r.Register(NewTool(
		"flex",
		"flex",
		"sys-devel/flex",
		"Fast lexical analyzer generator",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"bison",
		"bison",
		"sys-devel/bison",
		"GNU parser generator",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"m4",
		"m4",
		"sys-devel/m4",
		"GNU macro processor",
	).WithCategories(CategoryUtility))

	r.Register(NewTool(
		"gettext",
		"gettext",
		"sys-devel/gettext",
		"GNU internationalization utilities",
	).WithCategories(CategoryUtility))

	// Desktop utilities
	r.Register(NewTool(
		"desktop-file-validate",
		"desktop-file-validate",
		"dev-util/desktop-file-utils",
		"Desktop entry file validator",
	).WithCategories(CategoryUtility).WithRequiredBy("xdg-utils", "desktop"))

	r.Register(NewTool(
		"update-desktop-database",
		"update-desktop-database",
		"dev-util/desktop-file-utils",
		"Update desktop MIME database",
	).WithCategories(CategoryUtility).WithRequiredBy("xdg-utils", "desktop"))
}

// registerCompression adds compression tools to the registry.
func registerCompression(r *Registry) {
	r.Register(NewTool(
		"gzip",
		"gzip",
		"app-arch/gzip",
		"GNU compression utility",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"bzip2",
		"bzip2",
		"app-arch/bzip2",
		"High-quality block-sorting file compressor",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"xz",
		"xz",
		"app-arch/xz-utils",
		"XZ compression utility (LZMA2)",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"zstd",
		"zstd",
		"app-arch/zstd",
		"Zstandard fast compression",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"lz4",
		"lz4",
		"app-arch/lz4",
		"LZ4 extremely fast compression",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"tar",
		"tar",
		"app-arch/tar",
		"Tape archiver",
	).WithCategories(CategoryCompression, CategoryUtility))

	r.Register(NewTool(
		"unzip",
		"unzip",
		"app-arch/unzip",
		"ZIP archive extractor",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"zip",
		"zip",
		"app-arch/zip",
		"ZIP archive creator",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"7z",
		"7z",
		"app-arch/p7zip",
		"7-Zip file archiver",
	).WithCategories(CategoryCompression))

	r.Register(NewTool(
		"unrar",
		"unrar",
		"app-arch/unrar",
		"RAR archive extractor",
	).WithCategories(CategoryCompression))
}

// registerDocumentation adds documentation tools to the registry.
func registerDocumentation(r *Registry) {
	r.Register(NewTool(
		"doxygen",
		"doxygen",
		"app-text/doxygen",
		"Documentation generator for source code",
	).WithCategories(CategoryDocumentation).WithRequiredBy("docs"))

	r.Register(NewTool(
		"sphinx-build",
		"sphinx-build",
		"dev-python/sphinx",
		"Sphinx documentation generator",
	).WithCategories(CategoryDocumentation).WithRequiredBy("sphinx"))

	r.Register(NewTool(
		"asciidoc",
		"asciidoc",
		"app-text/asciidoc",
		"Text document format and toolchain",
	).WithCategories(CategoryDocumentation))

	r.Register(NewTool(
		"asciidoctor",
		"asciidoctor",
		"app-text/asciidoctor",
		"Ruby-based AsciiDoc processor",
	).WithCategories(CategoryDocumentation))

	r.Register(NewTool(
		"makeinfo",
		"makeinfo",
		"sys-apps/texinfo",
		"GNU Texinfo documentation system",
	).WithCategories(CategoryDocumentation))

	r.Register(NewTool(
		"pod2man",
		"pod2man",
		"dev-lang/perl",
		"Convert Perl POD to man pages",
	).WithCategories(CategoryDocumentation))

	r.Register(NewTool(
		"help2man",
		"help2man",
		"sys-apps/help2man",
		"Generate man pages from --help output",
	).WithCategories(CategoryDocumentation))
}

// registerVCS adds version control tools to the registry.
func registerVCS(r *Registry) {
	r.Register(NewTool(
		"git",
		"git",
		"dev-vcs/git",
		"Git distributed version control",
	).WithCategories(CategoryVCS).WithRequiredBy("git-r3"))

	r.Register(NewTool(
		"svn",
		"svn",
		"dev-vcs/subversion",
		"Subversion version control",
	).WithCategories(CategoryVCS).WithRequiredBy("subversion"))

	r.Register(NewTool(
		"hg",
		"hg",
		"dev-vcs/mercurial",
		"Mercurial distributed version control",
	).WithCategories(CategoryVCS).WithRequiredBy("mercurial"))

	r.Register(NewTool(
		"cvs",
		"cvs",
		"dev-vcs/cvs",
		"Concurrent Versions System",
	).WithCategories(CategoryVCS).WithRequiredBy("cvs"))

	r.Register(NewTool(
		"darcs",
		"darcs",
		"dev-vcs/darcs",
		"Darcs advanced revision control",
	).WithCategories(CategoryVCS).WithRequiredBy("darcs"))
}

// EclassToolMap returns a map of eclass names to required tools.
//
// This is useful for looking up what tools an eclass needs.
// Eclasses that don't require any special tools are NOT listed here -
// they simply won't trigger any tool checks.
//
// IMPORTANT: This map is used by CheckForEclassesOnly() for per-package
// tool checking. Only add eclasses here if they REQUIRE specific external
// tools that aren't part of the base system.
func EclassToolMap() map[string][]string {
	return map[string][]string{
		// Build systems
		"cmake":     {"cmake", "ninja", "make"},
		"meson":     {"meson", "ninja"},
		"autotools": {"autoconf", "automake", "libtool", "make"},
		"scons":     {"scons", "python"},
		"waf":       {"waf", "python"},
		"qmake":     {"qmake", "make"},

		// Languages - only for packages that NEED these runtimes at build time
		"cargo":       {"cargo", "rustc"},
		"go-module":   {"go"},
		"perl-module": {"perl"},
		"ruby-ng":     {"ruby"},
		"lua":         {"lua"},
		"java-pkg-2":  {"java", "javac"},

		// Python eclasses - python is typically always available on Gentoo
		// but we include it for completeness
		"python-single-r1": {"python3"},
		"python-r1":        {"python3"},
		"python-any-r1":    {"python3"},
		"distutils-r1":     {"python3"},

		// VCS - only for live ebuilds that fetch from version control
		"git-r3":     {"git"},
		"subversion": {"svn"},
		"mercurial":  {"hg"},
		"cvs":        {"cvs"},
		"darcs":      {"darcs"},

		// Desktop utilities
		"xdg-utils": {"desktop-file-validate", "update-desktop-database"},
		"desktop":   {"desktop-file-validate"},

		// Documentation
		"docs":   {"doxygen"},
		"sphinx": {"sphinx-build", "python3"},

		// NOTE: The following eclasses do NOT require special tools:
		// - toolchain-funcs: Uses system's default compiler (always available)
		// - multilib: No special tools needed
		// - linux-info: Only reads kernel config, no tools needed
		// - pam: No special tools needed
		// - bash-completion-r1: No special tools needed
		// - flag-o-matic: No special tools needed
		// - eutils: No special tools needed (deprecated)
		// - xdg: No special tools needed (unlike xdg-utils)
		// - optfeature: No special tools needed
		// - edo: No special tools needed
		// - wrapper: No special tools needed
		// - systemd: No special tools needed (uses pkg-config internally)
		// - multiprocessing: No special tools needed
		// - verify-sig: No special tools needed (uses GPG internally)
	}
}
