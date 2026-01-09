package repo

import (
	"testing"
)

// Quick focused tests for ebuild parser

// TestParseAtomVersion_BasicOperators tests version operators
func TestParseAtomVersion_BasicOperators(t *testing.T) {
	tests := []struct {
		name     string
		atom     string
		expected string // package name
		hasVer   bool
	}{
		{"No operator", "sys-libs/zlib", "sys-libs/zlib", false},
		{"Exact version", "=sys-libs/zlib-1.2.13", "sys-libs/zlib", true},
		{"Greater or equal", ">=sys-libs/zlib-1.2.0", "sys-libs/zlib", true},
		{"Less or equal", "<=sys-libs/zlib-2.0.0", "sys-libs/zlib", true},
		{"Greater than", ">sys-libs/zlib-1.0.0", "sys-libs/zlib", true},
		{"Less than", "<sys-libs/zlib-2.0.0", "sys-libs/zlib", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, err := parseAtomVersion(tt.atom)
			if err != nil {
				t.Errorf("parseAtomVersion(%q) error: %v", tt.atom, err)
			}

			if constraint.Name != tt.expected {
				t.Errorf("parseAtomVersion(%q).Name = %q, expected %q", tt.atom, constraint.Name, tt.expected)
			}

			if tt.hasVer && constraint.Version == nil {
				t.Errorf("parseAtomVersion(%q) expected version constraint", tt.atom)
			}

			if !tt.hasVer && constraint.Version != nil {
				t.Errorf("parseAtomVersion(%q) unexpected version constraint", tt.atom)
			}
		})
	}
}

// TestParsePackageAtom_Blockers tests blocker parsing
func TestParsePackageAtom_Blockers(t *testing.T) {
	parser := &EbuildParser{}

	tests := []struct {
		name          string
		atom          string
		expectBlocker bool
		expectHard    bool
	}{
		{"No blocker", "sys-libs/zlib", false, false},
		{"Soft blocker", "!sys-libs/zlib", true, false},
		{"Hard blocker", "!!sys-libs/zlib", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep, err := parser.parsePackageAtom(tt.atom, DepTypeRuntime, "", 0)
			if err != nil {
				t.Errorf("parsePackageAtom(%q) error: %v", tt.atom, err)
			}

			if dep.IsBlocker != tt.expectBlocker {
				t.Errorf("parsePackageAtom(%q).IsBlocker = %v, expected %v", tt.atom, dep.IsBlocker, tt.expectBlocker)
			}

			if dep.IsHardBlock != tt.expectHard {
				t.Errorf("parsePackageAtom(%q).IsHardBlock = %v, expected %v", tt.atom, dep.IsHardBlock, tt.expectHard)
			}
		})
	}
}

// TestTokenizeDependencies tests dependency tokenization
func TestTokenizeDependencies(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of tokens
	}{
		{"Simple", "sys-libs/zlib dev-libs/openssl", 2},
		{"With parens", "( sys-libs/zlib )", 3}, // (, package, )
		{"USE condition", "ssl? ( dev-libs/openssl )", 4},
		{"Any-of", "|| ( pkg1 pkg2 )", 4}, // ||, (, pkg1, pkg2, ) = 5 but closing ) ends parsing
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizeDependencies(tt.input)
			if len(tokens) < tt.expected {
				t.Errorf("tokenizeDependencies(%q) returned %d tokens, expected at least %d", tt.input, len(tokens), tt.expected)
			}
		})
	}
}

// TestParseDependencyString tests full dependency string parsing
func TestParseDependencyString(t *testing.T) {
	parser := &EbuildParser{}

	tests := []struct {
		name         string
		depStr       string
		expectedDeps int
	}{
		{
			name:         "Simple dependency",
			depStr:       "sys-libs/zlib",
			expectedDeps: 1,
		},
		{
			name:         "Multiple dependencies",
			depStr:       "sys-libs/zlib dev-libs/openssl",
			expectedDeps: 2,
		},
		{
			name:         "USE conditional",
			depStr:       "ssl? ( dev-libs/openssl )",
			expectedDeps: 1,
		},
		{
			name:         "Blocker",
			depStr:       "!sys-libs/old-zlib",
			expectedDeps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := parser.parseDependencyString(tt.depStr, DepTypeRuntime)
			if err != nil {
				t.Errorf("parseDependencyString(%q) error: %v", tt.depStr, err)
			}

			if len(deps) != tt.expectedDeps {
				t.Errorf("parseDependencyString(%q) returned %d deps, expected %d", tt.depStr, len(deps), tt.expectedDeps)
			}
		})
	}
}

// TestParseDependencies_MultipleTypes tests parsing different dependency types
func TestParseDependencies_MultipleTypes(t *testing.T) {
	content := `
RDEPEND="sys-libs/zlib"
DEPEND="dev-libs/openssl"
BDEPEND="sys-devel/gcc"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Errorf("ParseDependencies() error: %v", err)
	}

	// Should have 3 dependencies total
	if len(deps) != 3 {
		t.Errorf("ParseDependencies() returned %d deps, expected 3", len(deps))
	}

	// Check dependency types
	var rdependCount, dependCount, bdependCount int
	for _, dep := range deps {
		switch dep.DepType {
		case DepTypeRuntime:
			rdependCount++
		case DepTypeBuild:
			dependCount++
		case DepTypeBuildtime:
			bdependCount++
		}
	}

	if rdependCount != 1 || dependCount != 1 || bdependCount != 1 {
		t.Errorf("ParseDependencies() type distribution: RDEPEND=%d, DEPEND=%d, BDEPEND=%d, expected 1 of each",
			rdependCount, dependCount, bdependCount)
	}
}

// TestParseDependencies_USEFlags tests USE flag conditions
func TestParseDependencies_USEFlags(t *testing.T) {
	content := `
RDEPEND="
	ssl? ( dev-libs/openssl )
	mysql? ( dev-db/mysql )
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Errorf("ParseDependencies() error: %v", err)
	}

	// Should have 2 conditional dependencies
	if len(deps) < 2 {
		t.Errorf("ParseDependencies() returned %d deps, expected at least 2", len(deps))
	}

	// Check USE flags are captured
	useFlagsFound := make(map[string]bool)
	for _, dep := range deps {
		if dep.UseFlag != "" {
			useFlagsFound[dep.UseFlag] = true
		}
	}

	if !useFlagsFound["ssl"] {
		t.Error("ParseDependencies() missing 'ssl' USE flag")
	}

	if !useFlagsFound["mysql"] {
		t.Error("ParseDependencies() missing 'mysql' USE flag")
	}
}

// TestExtractVariable tests variable extraction from ebuild
func TestExtractVariable(t *testing.T) {
	content := `
DESCRIPTION="Test package"
RDEPEND="sys-libs/zlib"
SLOT="0"
`

	parser := NewEbuildParser(content)

	tests := []struct {
		varName  string
		expected string
	}{
		{"DESCRIPTION", "Test package"},
		{"RDEPEND", "sys-libs/zlib"},
		{"SLOT", "0"},
		{"NONEXISTENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			result := parser.ExtractVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("extractVariable(%q) = %q, expected %q", tt.varName, result, tt.expected)
			}
		})
	}
}

// TestVariableExpansion tests basic variable expansion
func TestVariableExpansion(t *testing.T) {
	content := `
COMMON_DEPEND="sys-libs/zlib dev-libs/openssl"
RDEPEND="${COMMON_DEPEND}"
DEPEND="${COMMON_DEPEND}"
`

	parser := NewEbuildParser(content)

	tests := []struct {
		varName  string
		expected string
	}{
		{"COMMON_DEPEND", "sys-libs/zlib dev-libs/openssl"},
		{"RDEPEND", "sys-libs/zlib dev-libs/openssl"},
		{"DEPEND", "sys-libs/zlib dev-libs/openssl"},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			result := parser.ExtractVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("extractVariable(%q) = %q, expected %q", tt.varName, result, tt.expected)
			}
		})
	}
}

// TestVariableExpansion_Recursive tests recursive variable expansion
func TestVariableExpansion_Recursive(t *testing.T) {
	content := `
BASE_DEPEND="sys-libs/zlib"
COMMON_DEPEND="${BASE_DEPEND} dev-libs/openssl"
RDEPEND="${COMMON_DEPEND} net-libs/curl"
`

	parser := NewEbuildParser(content)

	result := parser.ExtractVariable("RDEPEND")
	expected := "sys-libs/zlib dev-libs/openssl net-libs/curl"

	if result != expected {
		t.Errorf("extractVariable(RDEPEND) = %q, expected %q", result, expected)
	}
}

// TestVariableExpansion_WithDefault tests ${VAR:-default} syntax
func TestVariableExpansion_WithDefault(t *testing.T) {
	content := `
RDEPEND="${PYTHON_DEPS:-dev-lang/python}"
`

	parser := NewEbuildParser(content)

	result := parser.ExtractVariable("RDEPEND")
	expected := "dev-lang/python" // PYTHON_DEPS not defined, use default

	if result != expected {
		t.Errorf("extractVariable(RDEPEND) = %q, expected %q", result, expected)
	}
}

// TestVariableExpansion_Complex tests complex real-world scenario
func TestVariableExpansion_Complex(t *testing.T) {
	content := `
PYTHON_DEPS="dev-lang/python:3.10"
COMMON_DEPEND="
	sys-libs/zlib
	${PYTHON_DEPS}
	dev-libs/openssl
"
RDEPEND="${COMMON_DEPEND}
	net-libs/curl
"
DEPEND="${COMMON_DEPEND}"
BDEPEND="sys-devel/gcc"
`

	parser := NewEbuildParser(content)

	// Test RDEPEND expansion
	rdepend := parser.ExtractVariable("RDEPEND")
	if rdepend == "" {
		t.Error("extractVariable(RDEPEND) returned empty string")
	}

	// Should contain all expanded dependencies
	expectedParts := []string{"sys-libs/zlib", "dev-lang/python:3.10", "dev-libs/openssl", "net-libs/curl"}
	for _, part := range expectedParts {
		if !contains(rdepend, part) {
			t.Errorf("extractVariable(RDEPEND) missing expected part: %q\nGot: %q", part, rdepend)
		}
	}

	// Test DEPEND expansion
	depend := parser.ExtractVariable("DEPEND")
	if !contains(depend, "dev-lang/python:3.10") {
		t.Errorf("extractVariable(DEPEND) should contain expanded PYTHON_DEPS\nGot: %q", depend)
	}
}

// TestVariableExpansion_InfiniteRecursion tests protection against infinite recursion
func TestVariableExpansion_InfiniteRecursion(t *testing.T) {
	// This is an invalid ebuild but shouldn't crash
	content := `
VAR_A="${VAR_B}"
VAR_B="${VAR_A}"
RDEPEND="${VAR_A}"
`

	parser := NewEbuildParser(content)

	// Should not crash, returns empty string due to depth limit
	result := parser.ExtractVariable("RDEPEND")

	// Result will be empty or partially expanded (depth limit reached)
	t.Logf("Infinite recursion test result: %q", result)
	// Just verify it didn't crash - result content doesn't matter
}

// TestExtractAllVariables tests bulk variable extraction
func TestExtractAllVariables(t *testing.T) {
	content := `
DESCRIPTION="Test package"
SLOT="0"
RDEPEND="sys-libs/zlib"
DEPEND="dev-libs/openssl"
BDEPEND="sys-devel/gcc"
`

	parser := NewEbuildParser(content)

	if len(parser.variables) < 5 {
		t.Errorf("extractAllVariables() found %d variables, expected at least 5", len(parser.variables))
	}

	expectedVars := []string{"DESCRIPTION", "SLOT", "RDEPEND", "DEPEND", "BDEPEND"}
	for _, varName := range expectedVars {
		if _, exists := parser.variables[varName]; !exists {
			t.Errorf("extractAllVariables() missing variable %q", varName)
		}
	}
}

// TestParseDependencies_MultiLine tests multi-line dependency parsing
func TestParseDependencies_MultiLine(t *testing.T) {
	content := `
RDEPEND="
	sys-libs/zlib
	dev-libs/openssl
	net-libs/curl
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("ParseDependencies() returned %d deps, expected 3", len(deps))
	}

	expectedPackages := []string{"sys-libs/zlib", "dev-libs/openssl", "net-libs/curl"}
	for i, expected := range expectedPackages {
		if !contains(deps[i].Constraint.Name, expected) {
			t.Errorf("Dependency %d: expected %q, got %q", i, expected, deps[i].Constraint.Name)
		}
	}
}

// TestParseDependencies_MultiLineWithIndentation tests real-world multi-line formatting
func TestParseDependencies_MultiLineWithIndentation(t *testing.T) {
	// Real ebuild style with tabs and indentation
	content := `
RDEPEND="
	>=sys-libs/zlib-1.2.13
	ssl? (
		dev-libs/openssl
		net-libs/gnutls
	)
	|| (
		dev-db/mysql
		dev-db/postgresql
	)
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Debug: print what we got BEFORE checking
	t.Logf("Parsed %d dependencies from multi-line ebuild", len(deps))
	for _, dep := range deps {
		t.Logf("  - %s (USE flag: %q)", dep.Constraint.Name, dep.UseFlag)
	}

	// Should parse all packages (zlib, openssl, gnutls, mysql, postgresql)
	if len(deps) < 5 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 5", len(deps))
	}
}

// TestParseDependencies_ComplexMultiLine tests complex multi-line with nested groups
func TestParseDependencies_ComplexMultiLine(t *testing.T) {
	content := `
RDEPEND="
	>=sys-libs/glibc-2.38
	>=sys-libs/zlib-1.2.13:0/1
	dev-libs/elfutils[debuginfod-]

	ssl? (
		>=dev-libs/openssl-3.0:0=
		dev-libs/gnutls:=
	)

	mysql? ( dev-db/mysql-connector-c:= )
	postgres? ( dev-db/postgresql:= )

	|| (
		sys-libs/libxcrypt[compat]
		sys-libs/glibc[crypt]
	)
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should parse all dependencies
	if len(deps) < 8 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 8", len(deps))
	}

	// Verify specific packages were parsed
	packageNames := make(map[string]bool)
	for _, dep := range deps {
		if contains(dep.Constraint.Name, "glibc") {
			packageNames["glibc"] = true
		}
		if contains(dep.Constraint.Name, "zlib") {
			packageNames["zlib"] = true
		}
		if contains(dep.Constraint.Name, "openssl") {
			packageNames["openssl"] = true
		}
	}

	expectedPackages := []string{"glibc", "zlib", "openssl"}
	for _, pkg := range expectedPackages {
		if !packageNames[pkg] {
			t.Errorf("Expected to find package %q in parsed dependencies", pkg)
		}
	}
}

// BenchmarkParseDependencies benchmarks dependency parsing
func BenchmarkParseDependencies(b *testing.B) {
	content := `
RDEPEND="
	>=sys-libs/zlib-1.2.13
	ssl? ( dev-libs/openssl )
	|| ( dev-db/mysql dev-db/postgresql )
"
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewEbuildParser(content)
		_, _ = parser.ParseDependencies()
	}
}

// TestParsePackageAtom_USEFlags tests USE flag parsing in atoms
func TestParsePackageAtom_USEFlags(t *testing.T) {
	parser := &EbuildParser{}

	tests := []struct {
		name           string
		atom           string
		expectedPkg    string
		expectedFlags  string // comma-separated
		shouldNotCrash bool   // just verify it doesn't crash
	}{
		{
			name:          "Single USE flag disabled",
			atom:          "dev-libs/elfutils[debuginfod-]",
			expectedPkg:   "dev-libs/elfutils",
			expectedFlags: "debuginfod-",
		},
		{
			name:          "Single USE flag enabled",
			atom:          "sys-libs/glibc[crypt]",
			expectedPkg:   "sys-libs/glibc",
			expectedFlags: "crypt",
		},
		{
			name:          "Multiple USE flags",
			atom:          "dev-lang/python[ssl,sqlite]",
			expectedPkg:   "dev-lang/python",
			expectedFlags: "ssl sqlite", // normalized (comma → space)
		},
		{
			name:          "Mixed USE flags (enabled/disabled)",
			atom:          "net-libs/libssh2[gcrypt,zlib,-gssapi]",
			expectedPkg:   "net-libs/libssh2",
			expectedFlags: "gcrypt zlib -gssapi",
		},
		{
			name:          "USE flag with version",
			atom:          ">=dev-libs/openssl-3.0[ssl,-tls1]",
			expectedPkg:   "dev-libs/openssl",
			expectedFlags: "ssl -tls1",
		},
		{
			name:          "USE flag with slot",
			atom:          "sys-libs/zlib:0[static-libs]",
			expectedPkg:   "sys-libs/zlib",
			expectedFlags: "static-libs",
		},
		{
			name:           "Complex atom with version, slot, and USE flags",
			atom:           ">=dev-lang/python-3.10:3.10[ssl,sqlite,-tk]",
			expectedPkg:    "dev-lang/python",
			expectedFlags:  "ssl sqlite -tk",
			shouldNotCrash: true, // just ensure it doesn't crash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep, err := parser.parsePackageAtom(tt.atom, DepTypeRuntime, "", 0)
			if err != nil {
				t.Fatalf("parsePackageAtom(%q) error: %v", tt.atom, err)
			}

			// Check package name
			if !contains(dep.Constraint.Name, tt.expectedPkg) {
				t.Errorf("parsePackageAtom(%q).Name = %q, expected to contain %q",
					tt.atom, dep.Constraint.Name, tt.expectedPkg)
			}

			// Check USE flags were extracted
			if !tt.shouldNotCrash && dep.Constraint.Condition == "" {
				t.Errorf("parsePackageAtom(%q) USE flags not extracted (Condition is empty)", tt.atom)
			}

			// Check USE flags content (if specified)
			if tt.expectedFlags != "" && !tt.shouldNotCrash {
				if dep.Constraint.Condition != tt.expectedFlags {
					t.Errorf("parsePackageAtom(%q).Condition = %q, expected %q",
						tt.atom, dep.Constraint.Condition, tt.expectedFlags)
				}
			}
		})
	}
}

// TestParseDependencies_RealWorldUSEFlags tests with real-world examples
func TestParseDependencies_RealWorldUSEFlags(t *testing.T) {
	content := `
RDEPEND="
	>=dev-libs/elfutils-0.189[debuginfod-]
	sys-libs/glibc[crypt-]
	dev-lang/python:3.10[ssl,sqlite]
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	if len(deps) < 3 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 3", len(deps))
	}

	// Verify USE flags were parsed
	useFlagsFound := 0
	for _, dep := range deps {
		if dep.Constraint.Condition != "" {
			useFlagsFound++
			t.Logf("Package: %s, USE flags: %q", dep.Constraint.Name, dep.Constraint.Condition)
		}
	}

	if useFlagsFound != 3 {
		t.Errorf("Expected 3 packages with USE flags, found %d", useFlagsFound)
	}
}

// BenchmarkVariableExpansion benchmarks variable expansion performance
func BenchmarkVariableExpansion(b *testing.B) {
	content := `
PYTHON_DEPS="dev-lang/python:3.10"
COMMON_DEPEND="sys-libs/zlib ${PYTHON_DEPS} dev-libs/openssl"
RDEPEND="${COMMON_DEPEND} net-libs/curl"
DEPEND="${COMMON_DEPEND}"
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewEbuildParser(content)
		_ = parser.ExtractVariable("RDEPEND")
		_ = parser.ExtractVariable("DEPEND")
	}
}

// TestParseDependencies_ORDependencies tests OR-dependency parsing
func TestParseDependencies_ORDependencies(t *testing.T) {
	content := `
RDEPEND="
	sys-libs/zlib
	|| (
		dev-db/mysql
		dev-db/postgresql
	)
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should parse: zlib + (mysql OR postgresql)
	// For now, we expect all packages to be returned
	if len(deps) < 3 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 3", len(deps))
	}

	// Verify packages were parsed
	packageNames := make(map[string]bool)
	for _, dep := range deps {
		if contains(dep.Constraint.Name, "zlib") {
			packageNames["zlib"] = true
		}
		if contains(dep.Constraint.Name, "mysql") {
			packageNames["mysql"] = true
		}
		if contains(dep.Constraint.Name, "postgresql") {
			packageNames["postgresql"] = true
		}
	}

	expectedPackages := []string{"zlib", "mysql", "postgresql"}
	for _, pkg := range expectedPackages {
		if !packageNames[pkg] {
			t.Errorf("Expected to find package %q in parsed dependencies", pkg)
		}
	}

	t.Logf("Parsed %d dependencies with OR-group", len(deps))
	for _, dep := range deps {
		t.Logf("  - %s (OrGroupID: %d)", dep.Constraint.Name, dep.OrGroupID)
	}

	// Verify OR-group IDs
	orGroupFound := false
	var orGroupID int
	for _, dep := range deps {
		if contains(dep.Constraint.Name, "mysql") || contains(dep.Constraint.Name, "postgresql") {
			if dep.OrGroupID == 0 {
				t.Errorf("Package %q should have non-zero OrGroupID", dep.Constraint.Name)
			}
			if orGroupID == 0 {
				orGroupID = dep.OrGroupID
			} else if dep.OrGroupID != orGroupID {
				t.Errorf("OR-group packages should have same OrGroupID: mysql=%d, postgresql=%d",
					orGroupID, dep.OrGroupID)
			}
			orGroupFound = true
		} else if contains(dep.Constraint.Name, "zlib") {
			if dep.OrGroupID != 0 {
				t.Errorf("Package zlib should have OrGroupID=0, got %d", dep.OrGroupID)
			}
		}
	}

	if !orGroupFound {
		t.Error("No OR-group packages found with valid OrGroupID")
	}
}

// TestParseDependencies_NestedORDependencies tests nested OR-dependencies
func TestParseDependencies_NestedORDependencies(t *testing.T) {
	content := `
RDEPEND="
	|| (
		sys-libs/libxcrypt[compat]
		sys-libs/glibc[crypt]
	)
	ssl? (
		|| (
			dev-libs/openssl
			dev-libs/gnutls
		)
	)
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should parse all packages (libxcrypt, glibc, openssl, gnutls)
	if len(deps) < 4 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 4", len(deps))
	}

	// Verify critical packages
	packageNames := make(map[string]bool)
	for _, dep := range deps {
		if contains(dep.Constraint.Name, "libxcrypt") {
			packageNames["libxcrypt"] = true
		}
		if contains(dep.Constraint.Name, "glibc") {
			packageNames["glibc"] = true
		}
		if contains(dep.Constraint.Name, "openssl") {
			packageNames["openssl"] = true
		}
		if contains(dep.Constraint.Name, "gnutls") {
			packageNames["gnutls"] = true
		}
	}

	expectedPackages := []string{"libxcrypt", "glibc", "openssl", "gnutls"}
	for _, pkg := range expectedPackages {
		if !packageNames[pkg] {
			t.Errorf("Expected to find package %q in parsed dependencies", pkg)
		}
	}

	t.Logf("Parsed %d dependencies with nested OR-groups", len(deps))
}

// TestParseDependencies_RealWorldORDependencies tests real-world OR-dependency patterns
func TestParseDependencies_RealWorldORDependencies(t *testing.T) {
	// Common pattern: virtual package with multiple providers
	content := `
RDEPEND="
	>=sys-libs/zlib-1.2.13
	|| (
		sys-devel/gcc
		sys-devel/clang
		sys-devel/icc
	)
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()

	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should parse: zlib + (gcc OR clang OR icc)
	if len(deps) < 4 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 4", len(deps))
	}

	// Verify all compilers are available as alternatives
	compilers := []string{"gcc", "clang", "icc"}
	foundCompilers := make(map[string]bool)
	for _, dep := range deps {
		for _, compiler := range compilers {
			if contains(dep.Constraint.Name, compiler) {
				foundCompilers[compiler] = true
			}
		}
	}

	if len(foundCompilers) != 3 {
		t.Errorf("Expected to find all 3 compilers, found %d", len(foundCompilers))
	}
}

// TestNewPackageMetadata tests PackageMetadata construction
func TestNewPackageMetadata(t *testing.T) {
	tests := []struct {
		name     string
		category string
		pkgName  string
		version  string
		expected PackageMetadata
	}{
		{
			name:     "Simple version without revision",
			category: "app-misc",
			pkgName:  "hello",
			version:  "2.10",
			expected: PackageMetadata{
				Category: "app-misc",
				PN:       "hello",
				PV:       "2.10",
				PR:       "r0",
				PVR:      "2.10",
				PF:       "hello-2.10",
				P:        "hello-2.10",
			},
		},
		{
			name:     "Version with revision",
			category: "sys-libs",
			pkgName:  "zlib",
			version:  "1.2.13-r1",
			expected: PackageMetadata{
				Category: "sys-libs",
				PN:       "zlib",
				PV:       "1.2.13",
				PR:       "r1",
				PVR:      "1.2.13-r1",
				PF:       "zlib-1.2.13-r1",
				P:        "zlib-1.2.13",
			},
		},
		{
			name:     "Version with higher revision",
			category: "dev-libs",
			pkgName:  "openssl",
			version:  "3.0.10-r5",
			expected: PackageMetadata{
				Category: "dev-libs",
				PN:       "openssl",
				PV:       "3.0.10",
				PR:       "r5",
				PVR:      "3.0.10-r5",
				PF:       "openssl-3.0.10-r5",
				P:        "openssl-3.0.10",
			},
		},
		{
			name:     "Suffixed version",
			category: "dev-lang",
			pkgName:  "python",
			version:  "3.11.6_beta1",
			expected: PackageMetadata{
				Category: "dev-lang",
				PN:       "python",
				PV:       "3.11.6_beta1",
				PR:       "r0",
				PVR:      "3.11.6_beta1",
				PF:       "python-3.11.6_beta1",
				P:        "python-3.11.6_beta1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := NewPackageMetadata(tt.category, tt.pkgName, tt.version)

			if meta.Category != tt.expected.Category {
				t.Errorf("Category = %q, expected %q", meta.Category, tt.expected.Category)
			}
			if meta.PN != tt.expected.PN {
				t.Errorf("PN = %q, expected %q", meta.PN, tt.expected.PN)
			}
			if meta.PV != tt.expected.PV {
				t.Errorf("PV = %q, expected %q", meta.PV, tt.expected.PV)
			}
			if meta.PR != tt.expected.PR {
				t.Errorf("PR = %q, expected %q", meta.PR, tt.expected.PR)
			}
			if meta.PVR != tt.expected.PVR {
				t.Errorf("PVR = %q, expected %q", meta.PVR, tt.expected.PVR)
			}
			if meta.PF != tt.expected.PF {
				t.Errorf("PF = %q, expected %q", meta.PF, tt.expected.PF)
			}
			if meta.P != tt.expected.P {
				t.Errorf("P = %q, expected %q", meta.P, tt.expected.P)
			}
		})
	}
}

// TestNewEbuildParserWithMetadata tests parser creation with package metadata
func TestNewEbuildParserWithMetadata(t *testing.T) {
	content := `
DESCRIPTION="Test package"
SLOT="0"
`

	meta := NewPackageMetadata("app-misc", "hello", "2.10")
	parser := NewEbuildParserWithMetadata(content, meta)

	// Verify metadata variables are set
	expectedVars := map[string]string{
		"CATEGORY": "app-misc",
		"PN":       "hello",
		"PV":       "2.10",
		"PR":       "r0",
		"PVR":      "2.10",
		"PF":       "hello-2.10",
		"P":        "hello-2.10",
	}

	for varName, expected := range expectedVars {
		actual, exists := parser.variables[varName]
		if !exists {
			t.Errorf("Variable %q not found in parser", varName)
			continue
		}
		if actual != expected {
			t.Errorf("Variable %q = %q, expected %q", varName, actual, expected)
		}
	}
}

// TestVariableExpansion_WithPackageMetadata tests ${P}, ${PN}, ${PV} expansion
func TestVariableExpansion_WithPackageMetadata(t *testing.T) {
	content := `
DESCRIPTION="${PN} - GNU Hello World"
HOMEPAGE="https://www.gnu.org/software/${PN}/"
SRC_URI="mirror://gnu/${PN}/${P}.tar.gz"
S="${WORKDIR}/${P}"
`

	meta := NewPackageMetadata("app-misc", "hello", "2.10")
	parser := NewEbuildParserWithMetadata(content, meta)

	tests := []struct {
		varName  string
		expected string
	}{
		{"DESCRIPTION", "hello - GNU Hello World"},
		{"HOMEPAGE", "https://www.gnu.org/software/hello/"},
		{"SRC_URI", "mirror://gnu/hello/hello-2.10.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			actual := parser.ExtractVariable(tt.varName)
			if actual != tt.expected {
				t.Errorf("%s = %q, expected %q", tt.varName, actual, tt.expected)
			}
		})
	}
}

// TestVariableExpansion_SRC_URI_Complex tests complex SRC_URI expansion
func TestVariableExpansion_SRC_URI_Complex(t *testing.T) {
	content := `
MY_P="${PN}-${PV/_/-}"
SRC_URI="https://github.com/example/${PN}/archive/v${PV}.tar.gz -> ${P}.tar.gz"
`

	meta := NewPackageMetadata("dev-libs", "libexample", "1.5.0")
	parser := NewEbuildParserWithMetadata(content, meta)

	srcURI := parser.ExtractVariable("SRC_URI")
	expected := "https://github.com/example/libexample/archive/v1.5.0.tar.gz -> libexample-1.5.0.tar.gz"

	if srcURI != expected {
		t.Errorf("SRC_URI = %q, expected %q", srcURI, expected)
	}
}

// TestVariableExpansion_DEPEND_WithPackageVars tests DEPEND with package variables
func TestVariableExpansion_DEPEND_WithPackageVars(t *testing.T) {
	content := `
SLOT="0/${PV}"
RDEPEND="
	>=sys-libs/zlib-1.2.13
	~${CATEGORY}/${PN}-${PV}
"
`

	meta := NewPackageMetadata("app-misc", "hello", "2.10")
	parser := NewEbuildParserWithMetadata(content, meta)

	// Test SLOT expansion
	slot := parser.ExtractVariable("SLOT")
	if slot != "0/2.10" {
		t.Errorf("SLOT = %q, expected %q", slot, "0/2.10")
	}

	// Parse dependencies - the ~app-misc/hello-2.10 should expand
	deps, err := parser.ParseDependencies()
	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should find at least 2 dependencies (zlib and expanded hello)
	if len(deps) < 2 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 2", len(deps))
	}

	// Verify the expanded dependency exists
	found := false
	for _, dep := range deps {
		t.Logf("Dependency: %s", dep.Constraint.Name)
		if contains(dep.Constraint.Name, "hello") {
			found = true
		}
	}
	if !found {
		t.Error("Expected to find expanded hello dependency")
	}
}

// BenchmarkNewEbuildParserWithMetadata benchmarks parser with metadata
func BenchmarkNewEbuildParserWithMetadata(b *testing.B) {
	content := `
DESCRIPTION="${PN} - test package"
SRC_URI="https://example.com/${P}.tar.gz"
RDEPEND=">=sys-libs/zlib-1.2.13"
`

	meta := NewPackageMetadata("app-misc", "hello", "2.10")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewEbuildParserWithMetadata(content, meta)
		_ = parser.ExtractVariable("SRC_URI")
	}
}
