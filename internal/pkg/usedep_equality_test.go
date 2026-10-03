package pkg

import "testing"

func TestParseAtom_USEDepEquality(t *testing.T) {
	atom, err := ParseAtom(">=dev-lang/perl-5.38.2-r3[perl_features_debug=,perl_features_ithreads=,perl_features_quadmath=]")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	c := atom.ToConstraint()

	if len(c.UseRequire) > 0 {
		t.Errorf("UseRequire should be empty for equality deps, got %v", c.UseRequire)
	}
	if len(c.UseEqual) != 3 {
		t.Fatalf("UseEqual should have 3 flags, got %d: %v", len(c.UseEqual), c.UseEqual)
	}
	expected := []string{"perl_features_debug", "perl_features_ithreads", "perl_features_quadmath"}
	for i, want := range expected {
		if c.UseEqual[i] != want {
			t.Errorf("UseEqual[%d] = %q, want %q", i, c.UseEqual[i], want)
		}
	}
}

func TestPackageSatisfiesUseDeps_Equality(t *testing.T) {
	c := Constraint{
		UseEqual: []string{"perl_features_ithreads"},
	}

	provOn := map[string]bool{"perl_features_ithreads": true}
	depOn := map[string]bool{"perl_features_ithreads": true}
	if !c.PackageSatisfiesUseDeps(provOn, depOn) {
		t.Error("both on: should satisfy [flag=]")
	}

	provOff := map[string]bool{"perl_features_ithreads": false}
	depOff := map[string]bool{"perl_features_ithreads": false}
	if !c.PackageSatisfiesUseDeps(provOff, depOff) {
		t.Error("both off: should satisfy [flag=]")
	}

	if c.PackageSatisfiesUseDeps(provOff, depOn) {
		t.Error("dep=on prov=off: should NOT satisfy [flag=]")
	}

	if c.PackageSatisfiesUseDeps(provOn, depOff) {
		t.Error("dep=off prov=on: should NOT satisfy [flag=]")
	}

	if !c.PackageSatisfiesUseDeps(provOn) {
		t.Error("no dependent USE: should pass (skip equality)")
	}
}

func TestParseAtom_AllConditionalForms(t *testing.T) {
	tests := []struct {
		atom     string
		equal    []string
		equalInv []string
		condIf   []string
		condUnl  []string
	}{
		{"dev-libs/foo[bar=]", []string{"bar"}, nil, nil, nil},
		{"dev-libs/foo[!bar=]", nil, []string{"bar"}, nil, nil},
		{"dev-libs/foo[bar?]", nil, nil, []string{"bar"}, nil},
		{"dev-libs/foo[!bar?]", nil, nil, nil, []string{"bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			atom, err := ParseAtom(tt.atom)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			c := atom.ToConstraint()
			check := func(name string, got, want []string) {
				if len(got) != len(want) {
					t.Errorf("%s: got %v, want %v", name, got, want)
				}
			}
			check("UseEqual", c.UseEqual, tt.equal)
			check("UseEqualInverse", c.UseEqualInverse, tt.equalInv)
			check("UseConditionalIf", c.UseConditionalIf, tt.condIf)
			check("UseConditionalUnless", c.UseConditionalUnless, tt.condUnl)
		})
	}
}

func TestRevisionMatch_TildeOperator(t *testing.T) {
	// ~perl-core/File-Temp-0.231.100 must match 0.231.100-r1
	atom, err := ParseAtom("~perl-core/File-Temp-0.231.100")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if atom.Operator != "~" {
		t.Fatalf("operator should be ~, got %q", atom.Operator)
	}

	c := atom.ToConstraint()
	if c.Version == nil {
		t.Fatal("expected version constraint")
	}
	if c.Version.Operator() != OpRevisionMatch {
		t.Errorf("expected OpRevisionMatch, got %v", c.Version.Operator())
	}

	// Must match any revision
	if !c.Version.Satisfies("0.231.100") {
		t.Error("~ should match base version 0.231.100")
	}
	if !c.Version.Satisfies("0.231.100-r1") {
		t.Error("~ should match revision 0.231.100-r1")
	}
	if !c.Version.Satisfies("0.231.100-r99") {
		t.Error("~ should match any revision 0.231.100-r99")
	}
	// Must NOT match different base
	if c.Version.Satisfies("0.231.200") {
		t.Error("~ should NOT match different base version")
	}
	if c.Version.Satisfies("0.231.100.1") {
		t.Error("~ should NOT match extended version")
	}
}
