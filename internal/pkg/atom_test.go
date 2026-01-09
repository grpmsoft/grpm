package pkg

import (
	"testing"
)

func TestParseAtom_Basic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantCat string
		wantPkg string
		wantVer string
		wantOp  string
		wantErr bool
	}{
		{
			name:    "simple category/package",
			input:   "sys-libs/glibc",
			wantCat: "sys-libs",
			wantPkg: "glibc",
		},
		{
			name:    "with exact version",
			input:   "=sys-libs/glibc-2.38",
			wantCat: "sys-libs",
			wantPkg: "glibc",
			wantVer: "2.38",
			wantOp:  "=",
		},
		{
			name:    "with greater-equal version",
			input:   ">=dev-lang/python-3.12",
			wantCat: "dev-lang",
			wantPkg: "python",
			wantVer: "3.12",
			wantOp:  ">=",
		},
		{
			name:    "with greater version",
			input:   ">app-misc/hello-2.0",
			wantCat: "app-misc",
			wantPkg: "hello",
			wantVer: "2.0",
			wantOp:  ">",
		},
		{
			name:    "with less-equal version",
			input:   "<=dev-libs/openssl-3.0.0",
			wantCat: "dev-libs",
			wantPkg: "openssl",
			wantVer: "3.0.0",
			wantOp:  "<=",
		},
		{
			name:    "with less version",
			input:   "<net-misc/curl-8.0",
			wantCat: "net-misc",
			wantPkg: "curl",
			wantVer: "8.0",
			wantOp:  "<",
		},
		{
			name:    "revision match operator",
			input:   "~sys-libs/glibc-2.38",
			wantCat: "sys-libs",
			wantPkg: "glibc",
			wantVer: "2.38",
			wantOp:  "~",
		},
		{
			name:    "glob match",
			input:   "=dev-lang/python-3.12*",
			wantCat: "dev-lang",
			wantPkg: "python",
			wantVer: "3.12",
			wantOp:  "=*",
		},
		{
			name:    "version with revision",
			input:   "=sys-libs/glibc-2.38-r1",
			wantCat: "sys-libs",
			wantPkg: "glibc",
			wantVer: "2.38-r1",
			wantOp:  "=",
		},
		{
			name:    "complex version",
			input:   "=dev-lang/python-3.12.0_beta4",
			wantCat: "dev-lang",
			wantPkg: "python",
			wantVer: "3.12.0_beta4",
			wantOp:  "=",
		},
		{
			name:    "underscore in package name",
			input:   "dev-python/typing_extensions",
			wantCat: "dev-python",
			wantPkg: "typing_extensions",
		},
		{
			name:    "plus in package name",
			input:   "dev-cpp/cpp-gsl",
			wantCat: "dev-cpp",
			wantPkg: "cpp-gsl",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if atom.Category != tc.wantCat {
				t.Errorf("Category: got %q, want %q", atom.Category, tc.wantCat)
			}
			if atom.Package != tc.wantPkg {
				t.Errorf("Package: got %q, want %q", atom.Package, tc.wantPkg)
			}
			if atom.Version != tc.wantVer {
				t.Errorf("Version: got %q, want %q", atom.Version, tc.wantVer)
			}
			if atom.Operator != tc.wantOp {
				t.Errorf("Operator: got %q, want %q", atom.Operator, tc.wantOp)
			}
		})
	}
}

func TestParseAtom_Blockers(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantBlocker string
		wantCat     string
		wantPkg     string
	}{
		{
			name:        "weak blocker",
			input:       "!sys-libs/uclibc",
			wantBlocker: "!",
			wantCat:     "sys-libs",
			wantPkg:     "uclibc",
		},
		{
			name:        "strong blocker",
			input:       "!!sys-libs/uclibc",
			wantBlocker: "!!",
			wantCat:     "sys-libs",
			wantPkg:     "uclibc",
		},
		{
			name:        "weak blocker with version",
			input:       "!>=sys-libs/glibc-2.38",
			wantBlocker: "!",
			wantCat:     "sys-libs",
			wantPkg:     "glibc",
		},
		{
			name:        "strong blocker with version",
			input:       "!!=app-misc/hello-1.0",
			wantBlocker: "!!",
			wantCat:     "app-misc",
			wantPkg:     "hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if atom.Blocker != tc.wantBlocker {
				t.Errorf("Blocker: got %q, want %q", atom.Blocker, tc.wantBlocker)
			}
			if atom.Category != tc.wantCat {
				t.Errorf("Category: got %q, want %q", atom.Category, tc.wantCat)
			}
			if atom.Package != tc.wantPkg {
				t.Errorf("Package: got %q, want %q", atom.Package, tc.wantPkg)
			}
		})
	}
}

func TestParseAtom_Slots(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSlot    string
		wantSubslot string
	}{
		{
			name:     "simple slot",
			input:    "sys-libs/glibc:2.38",
			wantSlot: "2.38",
		},
		{
			name:     "slot zero",
			input:    "dev-libs/openssl:0",
			wantSlot: "0",
		},
		{
			name:        "slot with subslot",
			input:       "dev-libs/openssl:0/1.1",
			wantSlot:    "0",
			wantSubslot: "1.1",
		},
		{
			name:     "any slot operator",
			input:    "sys-libs/glibc:*",
			wantSlot: "*",
		},
		{
			name:     "slot rebuild operator",
			input:    "dev-libs/openssl:=",
			wantSlot: "=",
		},
		{
			name:     "slot with version",
			input:    ">=sys-libs/glibc-2.38:2.38",
			wantSlot: "2.38",
		},
		{
			name:        "slot and subslot with version",
			input:       "=dev-libs/openssl-3.0.0:0/3",
			wantSlot:    "0",
			wantSubslot: "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if atom.Slot != tc.wantSlot {
				t.Errorf("Slot: got %q, want %q", atom.Slot, tc.wantSlot)
			}
			if atom.Subslot != tc.wantSubslot {
				t.Errorf("Subslot: got %q, want %q", atom.Subslot, tc.wantSubslot)
			}
		})
	}
}

func TestParseAtom_Repository(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRepo string
	}{
		{
			name:     "simple repository",
			input:    "sys-libs/glibc::gentoo",
			wantRepo: "gentoo",
		},
		{
			name:     "repository with version",
			input:    ">=sys-libs/glibc-2.38::gentoo",
			wantRepo: "gentoo",
		},
		{
			name:     "repository with slot",
			input:    "sys-libs/glibc:2.38::gentoo",
			wantRepo: "gentoo",
		},
		{
			name:     "repository with hyphen",
			input:    "dev-libs/openssl::gentoo-overlay",
			wantRepo: "gentoo-overlay",
		},
		{
			name:     "repository with underscore",
			input:    "app-misc/hello::my_overlay",
			wantRepo: "my_overlay",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if atom.Repository != tc.wantRepo {
				t.Errorf("Repository: got %q, want %q", atom.Repository, tc.wantRepo)
			}
		})
	}
}

func TestParseAtom_UseDeps(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantRequire     []string
		wantBlock       []string
		wantConditional []string
	}{
		{
			name:        "single required flag",
			input:       "dev-libs/openssl[ssl]",
			wantRequire: []string{"ssl"},
		},
		{
			name:        "multiple required flags",
			input:       "dev-libs/openssl[ssl,threads]",
			wantRequire: []string{"ssl", "threads"},
		},
		{
			name:      "single blocked flag",
			input:     "dev-libs/openssl[-static]",
			wantBlock: []string{"static"},
		},
		{
			name:        "mixed required and blocked",
			input:       "dev-libs/openssl[ssl,-static,threads]",
			wantRequire: []string{"ssl", "threads"},
			wantBlock:   []string{"static"},
		},
		{
			name:            "conditional flag",
			input:           "dev-libs/openssl[ssl?]",
			wantConditional: []string{"ssl?"},
		},
		{
			name:            "negative conditional",
			input:           "dev-libs/openssl[-debug?]",
			wantConditional: []string{"!debug?"},
		},
		{
			name:        "with version and slot",
			input:       ">=dev-libs/openssl-3.0:0[ssl,-static]",
			wantRequire: []string{"ssl"},
			wantBlock:   []string{"static"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !stringSliceEqual(atom.UseRequire, tc.wantRequire) {
				t.Errorf("UseRequire: got %v, want %v", atom.UseRequire, tc.wantRequire)
			}
			if !stringSliceEqual(atom.UseBlock, tc.wantBlock) {
				t.Errorf("UseBlock: got %v, want %v", atom.UseBlock, tc.wantBlock)
			}
			if !stringSliceEqual(atom.UseConditional, tc.wantConditional) {
				t.Errorf("UseConditional: got %v, want %v", atom.UseConditional, tc.wantConditional)
			}
		})
	}
}

func TestParseAtom_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: ErrEmptyAtom,
		},
		{
			name:    "no category",
			input:   "glibc",
			wantErr: ErrNoCategory,
		},
		{
			name:    "empty category",
			input:   "/glibc",
			wantErr: ErrEmptyCategory,
		},
		{
			name:    "empty package",
			input:   "sys-libs/",
			wantErr: ErrEmptyPackage,
		},
		{
			name:    "operator without version",
			input:   ">=sys-libs/glibc",
			wantErr: ErrOperatorWithoutVer,
		},
		{
			name:    "glob without = operator",
			input:   ">=sys-libs/glibc-2.38*",
			wantErr: ErrGlobWithoutOperator,
		},
		{
			name:    "unmatched bracket",
			input:   "dev-libs/openssl[ssl",
			wantErr: ErrUnmatchedBracket,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseAtom(tc.input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			// Check error wrapping
			if tc.wantErr != nil && err.Error() != tc.wantErr.Error() &&
				!containsError(err, tc.wantErr) {
				t.Errorf("got error %q, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseAtom_Complex(t *testing.T) {
	// Test complex real-world atoms
	tests := []struct {
		name  string
		input string
		check func(*testing.T, *Atom)
	}{
		{
			name:  "full featured atom",
			input: "!!>=dev-libs/openssl-3.0.0:0/3::gentoo[ssl,-static,threads?]",
			check: func(t *testing.T, a *Atom) {
				if a.Blocker != "!!" {
					t.Errorf("Blocker: got %q, want %q", a.Blocker, "!!")
				}
				if a.Operator != ">=" {
					t.Errorf("Operator: got %q, want %q", a.Operator, ">=")
				}
				if a.Category != "dev-libs" {
					t.Errorf("Category: got %q, want %q", a.Category, "dev-libs")
				}
				if a.Package != "openssl" {
					t.Errorf("Package: got %q, want %q", a.Package, "openssl")
				}
				if a.Version != "3.0.0" {
					t.Errorf("Version: got %q, want %q", a.Version, "3.0.0")
				}
				if a.Slot != "0" {
					t.Errorf("Slot: got %q, want %q", a.Slot, "0")
				}
				if a.Subslot != "3" {
					t.Errorf("Subslot: got %q, want %q", a.Subslot, "3")
				}
				if a.Repository != "gentoo" {
					t.Errorf("Repository: got %q, want %q", a.Repository, "gentoo")
				}
				if len(a.UseRequire) != 1 || a.UseRequire[0] != "ssl" {
					t.Errorf("UseRequire: got %v, want [ssl]", a.UseRequire)
				}
				if len(a.UseBlock) != 1 || a.UseBlock[0] != "static" {
					t.Errorf("UseBlock: got %v, want [static]", a.UseBlock)
				}
			},
		},
		{
			name:  "python with glob",
			input: "=dev-lang/python-3.12*:3.12",
			check: func(t *testing.T, a *Atom) {
				if a.Operator != "=*" {
					t.Errorf("Operator: got %q, want %q", a.Operator, "=*")
				}
				if a.Version != "3.12" {
					t.Errorf("Version: got %q, want %q", a.Version, "3.12")
				}
				if a.Slot != "3.12" {
					t.Errorf("Slot: got %q, want %q", a.Slot, "3.12")
				}
			},
		},
		{
			name:  "glibc typical dependency",
			input: ">=sys-libs/glibc-2.38:2.38",
			check: func(t *testing.T, a *Atom) {
				if a.Operator != ">=" {
					t.Errorf("Operator: got %q, want %q", a.Operator, ">=")
				}
				if a.Category != "sys-libs" {
					t.Errorf("Category: got %q, want %q", a.Category, "sys-libs")
				}
				if a.Package != "glibc" {
					t.Errorf("Package: got %q, want %q", a.Package, "glibc")
				}
				if a.Version != "2.38" {
					t.Errorf("Version: got %q, want %q", a.Version, "2.38")
				}
				if a.Slot != "2.38" {
					t.Errorf("Slot: got %q, want %q", a.Slot, "2.38")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, atom)
		})
	}
}

func TestAtom_String(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple",
			input: "sys-libs/glibc",
			want:  "sys-libs/glibc",
		},
		{
			name:  "with version",
			input: "=sys-libs/glibc-2.38",
			want:  "=sys-libs/glibc-2.38",
		},
		{
			name:  "with glob",
			input: "=dev-lang/python-3.12*",
			want:  "=dev-lang/python-3.12*",
		},
		{
			name:  "with slot",
			input: "sys-libs/glibc:2.38",
			want:  "sys-libs/glibc:2.38",
		},
		{
			name:  "with repository",
			input: "sys-libs/glibc::gentoo",
			want:  "sys-libs/glibc::gentoo",
		},
		{
			name:  "with use flags",
			input: "dev-libs/openssl[ssl,-static]",
			want:  "dev-libs/openssl[ssl,-static]",
		},
		{
			name:  "blocker",
			input: "!sys-libs/uclibc",
			want:  "!sys-libs/uclibc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := atom.String()
			if got != tc.want {
				t.Errorf("String(): got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAtom_CPV(t *testing.T) {
	tests := []struct {
		input   string
		wantCPV string
	}{
		{"sys-libs/glibc", "sys-libs/glibc"},
		{"=sys-libs/glibc-2.38", "sys-libs/glibc-2.38"},
		{">=dev-lang/python-3.12:3.12", "dev-lang/python-3.12"},
		{"!app-misc/hello", "app-misc/hello"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := atom.CPV()
			if got != tc.wantCPV {
				t.Errorf("CPV(): got %q, want %q", got, tc.wantCPV)
			}
		})
	}
}

func TestAtom_CP(t *testing.T) {
	tests := []struct {
		input  string
		wantCP string
	}{
		{"sys-libs/glibc", "sys-libs/glibc"},
		{"=sys-libs/glibc-2.38", "sys-libs/glibc"},
		{">=dev-lang/python-3.12:3.12[ssl]", "dev-lang/python"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := atom.CP()
			if got != tc.wantCP {
				t.Errorf("CP(): got %q, want %q", got, tc.wantCP)
			}
		})
	}
}

func TestAtom_IsBlocker(t *testing.T) {
	tests := []struct {
		input     string
		isBlocker bool
		isStrong  bool
		isWeak    bool
	}{
		{"sys-libs/glibc", false, false, false},
		{"!sys-libs/glibc", true, false, true},
		{"!!sys-libs/glibc", true, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			atom, err := ParseAtom(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if atom.IsBlocker() != tc.isBlocker {
				t.Errorf("IsBlocker(): got %v, want %v", atom.IsBlocker(), tc.isBlocker)
			}
			if atom.IsStrongBlocker() != tc.isStrong {
				t.Errorf("IsStrongBlocker(): got %v, want %v", atom.IsStrongBlocker(), tc.isStrong)
			}
			if atom.IsWeakBlocker() != tc.isWeak {
				t.Errorf("IsWeakBlocker(): got %v, want %v", atom.IsWeakBlocker(), tc.isWeak)
			}
		})
	}
}

func TestAtom_Matches(t *testing.T) {
	tests := []struct {
		name      string
		atom      string
		pkg       *Package
		wantMatch bool
	}{
		{
			name: "simple match",
			atom: "sys-libs/glibc",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name: "version match",
			atom: "=sys-libs/glibc-2.38",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name: "version mismatch",
			atom: "=sys-libs/glibc-2.37",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: false,
		},
		{
			name: "greater-equal match",
			atom: ">=sys-libs/glibc-2.37",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name: "greater-equal boundary",
			atom: ">=sys-libs/glibc-2.38",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name: "slot match",
			atom: "sys-libs/glibc:2.38",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name: "slot mismatch",
			atom: "sys-libs/glibc:2.37",
			pkg: &Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    Slot{Name: "2.38"},
			},
			wantMatch: false,
		},
		{
			name: "glob match",
			atom: "=dev-lang/python-3.12*",
			pkg: &Package{
				Name:    "dev-lang/python",
				Version: "3.12.1",
				Slot:    Slot{Name: "3.12"},
			},
			wantMatch: true,
		},
		{
			name: "glob no match",
			atom: "=dev-lang/python-3.11*",
			pkg: &Package{
				Name:    "dev-lang/python",
				Version: "3.12.1",
				Slot:    Slot{Name: "3.12"},
			},
			wantMatch: false,
		},
		{
			name: "different package",
			atom: "sys-libs/glibc",
			pkg: &Package{
				Name:    "sys-libs/musl",
				Version: "1.2.4",
			},
			wantMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := atom.Matches(tc.pkg)
			if got != tc.wantMatch {
				t.Errorf("Matches(): got %v, want %v", got, tc.wantMatch)
			}
		})
	}
}

func TestAtom_ToConstraint(t *testing.T) {
	tests := []struct {
		name     string
		atom     string
		wantName string
		wantOp   VersionOperator
		wantVer  string
	}{
		{
			name:     "simple",
			atom:     "sys-libs/glibc",
			wantName: "sys-libs/glibc",
		},
		{
			name:     "with version",
			atom:     ">=sys-libs/glibc-2.38",
			wantName: "sys-libs/glibc",
			wantOp:   OpGreaterEqual,
			wantVer:  "2.38",
		},
		{
			name:     "exact version",
			atom:     "=dev-lang/python-3.12",
			wantName: "dev-lang/python",
			wantOp:   OpEqual,
			wantVer:  "3.12",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			c := atom.ToConstraint()
			if c.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", c.Name, tc.wantName)
			}
			if tc.wantVer != "" {
				if c.Version == nil {
					t.Fatal("Version is nil, expected non-nil")
				}
				if c.Version.Operator() != tc.wantOp {
					t.Errorf("Operator: got %v, want %v", c.Version.Operator(), tc.wantOp)
				}
				if c.Version.Version() != tc.wantVer {
					t.Errorf("Version: got %q, want %q", c.Version.Version(), tc.wantVer)
				}
			}
		})
	}
}

func TestMatchesRevision(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   bool
	}{
		{"2.38", "2.38", true},
		{"2.38-r1", "2.38", true},
		{"2.38-r1", "2.38-r2", true},
		{"2.38-r1", "2.37", false},
		{"2.38", "2.37", false},
	}

	for _, tc := range tests {
		t.Run(tc.v1+"_"+tc.v2, func(t *testing.T) {
			got := matchesRevision(tc.v1, tc.v2)
			if got != tc.want {
				t.Errorf("matchesRevision(%q, %q): got %v, want %v", tc.v1, tc.v2, got, tc.want)
			}
		})
	}
}

func TestSplitPackageVersion(t *testing.T) {
	tests := []struct {
		input   string
		wantPkg string
		wantVer string
	}{
		{"glibc-2.38", "glibc", "2.38"},
		{"glibc-2.38-r1", "glibc", "2.38-r1"},
		{"python-3.12.0_beta4", "python", "3.12.0_beta4"},
		{"typing_extensions-4.0", "typing_extensions", "4.0"},
		{"cpp-gsl-4.0.0", "cpp-gsl", "4.0.0"},
		{"glibc", "glibc", ""},
		{"my-pkg-name", "my-pkg-name", ""},
		{"my-pkg-name-1.0", "my-pkg-name", "1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			gotPkg, gotVer := splitPackageVersion(tc.input)
			if gotPkg != tc.wantPkg {
				t.Errorf("pkg: got %q, want %q", gotPkg, tc.wantPkg)
			}
			if gotVer != tc.wantVer {
				t.Errorf("ver: got %q, want %q", gotVer, tc.wantVer)
			}
		})
	}
}

// Helper functions

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsError(err, target error) bool {
	return err != nil && target != nil &&
		(err.Error() == target.Error() ||
			len(err.Error()) > len(target.Error()) &&
				err.Error()[:len(target.Error())] == target.Error()[:len(target.Error())])
}
