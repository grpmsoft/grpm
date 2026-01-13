package ebuild

import (
	"testing"
)

func TestExpandBashParameters_SimpleVariable(t *testing.T) {
	vars := map[string]string{
		"PN":      "screenfetch",
		"PV":      "3.9.9",
		"P":       "screenfetch-3.9.9",
		"WORKDIR": "/var/tmp/portage/app-misc/screenfetch-3.9.9/work",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple ${VAR}",
			input:    "${WORKDIR}/${P}",
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenfetch-3.9.9",
		},
		{
			name:     "simple $VAR",
			input:    "$WORKDIR/$P",
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenfetch-3.9.9",
		},
		{
			name:     "nested variables",
			input:    "${WORKDIR}/${PN}-${PV}",
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenfetch-3.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandBashParameters_Substitution(t *testing.T) {
	vars := map[string]string{
		"PN":      "screenfetch",
		"PV":      "3.9.9",
		"WORKDIR": "/var/tmp/portage/app-misc/screenfetch-3.9.9/work",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single substitution ${PN/f/F}",
			input:    "${PN/f/F}",
			expected: "screenFetch",
		},
		{
			name:     "global substitution ${PN//e/E}",
			input:    "${PN//e/E}",
			expected: "scrEEnfEtch",
		},
		{
			name:     "substitution with removal ${PV/./-}",
			input:    "${PV/./-}",
			expected: "3-9.9",
		},
		{
			name:     "full S with substitution",
			input:    "${WORKDIR}/${PN/f/F}-${PV}",
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenFetch-3.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandBashParameters_CaseModification(t *testing.T) {
	vars := map[string]string{
		"PN":      "package",
		"PV":      "1.0",
		"WORKDIR": "/work",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uppercase first ${PN^}",
			input:    "${PN^}",
			expected: "Package",
		},
		{
			name:     "uppercase all ${PN^^}",
			input:    "${PN^^}",
			expected: "PACKAGE",
		},
		{
			name:     "lowercase first (no-op on lowercase)",
			input:    "${PN,}",
			expected: "package",
		},
		{
			name:     "lowercase all (no-op on lowercase)",
			input:    "${PN,,}",
			expected: "package",
		},
		{
			name:     "full S with case modification",
			input:    "${WORKDIR}/${PN^}-${PV}",
			expected: "/work/Package-1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandBashParameters_SuffixRemoval(t *testing.T) {
	vars := map[string]string{
		"PV":      "1.2.3_beta1",
		"P":       "mypackage-1.2.3_beta1",
		"WORKDIR": "/work",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove shortest suffix ${PV%_*}",
			input:    "${PV%_*}",
			expected: "1.2.3",
		},
		{
			name:     "remove simple suffix ${PV%.3_beta1}",
			input:    "${PV%.3_beta1}",
			expected: "1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandBashParameters_PrefixRemoval(t *testing.T) {
	vars := map[string]string{
		"P":       "prefix-mypackage-1.0",
		"WORKDIR": "/work",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove simple prefix ${P#prefix-}",
			input:    "${P#prefix-}",
			expected: "mypackage-1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSVariable(t *testing.T) {
	vars := map[string]string{
		"PN":      "screenfetch",
		"PV":      "3.9.9",
		"P":       "screenfetch-3.9.9",
		"WORKDIR": "/var/tmp/portage/app-misc/screenfetch-3.9.9/work",
	}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "custom S with substitution",
			content: `EAPI=8
S=${WORKDIR}/${PN/f/F}-${PV}
`,
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenFetch-3.9.9",
		},
		{
			name: "custom S hardcoded path",
			content: `EAPI=8
S="${WORKDIR}/source"
`,
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/source",
		},
		{
			name: "no S defined",
			content: `EAPI=8
DESCRIPTION="test"
`,
			expected: "",
		},
		{
			name: "S with MY_P variable",
			content: `EAPI=8
MY_P="CustomName-${PV}"
S="${WORKDIR}/${MY_P}"
`,
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/CustomName-3.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSVariable(tt.content, vars)
			if result != tt.expected {
				t.Errorf("ParseSVariable() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExpandBashParameters_RealWorldCases(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		input    string
		expected string
	}{
		{
			name: "screenfetch S pattern",
			vars: map[string]string{
				"PN":      "screenfetch",
				"PV":      "3.9.9",
				"WORKDIR": "/var/tmp/portage/app-misc/screenfetch-3.9.9/work",
			},
			input:    "${WORKDIR}/${PN/f/F}-${PV}",
			expected: "/var/tmp/portage/app-misc/screenfetch-3.9.9/work/screenFetch-3.9.9",
		},
		{
			name: "version munging with underscore to dash",
			vars: map[string]string{
				"PN":      "mypackage",
				"PV":      "1.0_beta2",
				"P":       "mypackage-1.0_beta2",
				"WORKDIR": "/work",
			},
			input:    "${WORKDIR}/${P/_/-}",
			expected: "/work/mypackage-1.0-beta2",
		},
		{
			name: "uppercase first letter",
			vars: map[string]string{
				"PN":      "somepackage",
				"PV":      "2.0",
				"WORKDIR": "/work",
			},
			input:    "${WORKDIR}/${PN^}-${PV}",
			expected: "/work/Somepackage-2.0",
		},
		{
			name: "strip version suffix",
			vars: map[string]string{
				"PN":      "app",
				"PV":      "3.0.10_beta1",
				"WORKDIR": "/work",
			},
			input:    "${WORKDIR}/${PN}-${PV%_*}",
			expected: "/work/app-3.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandBashParameters(tt.input, tt.vars)
			if result != tt.expected {
				t.Errorf("ExpandBashParameters(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
