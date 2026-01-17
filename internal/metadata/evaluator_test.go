package metadata

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// TestVerCut tests the Portage-compatible ver_cut implementation.
func TestVerCut(t *testing.T) {
	// This tests our ver_cut reimplementation that avoids mvdan.cc/sh bugs
	// with array slicing in subshells.
	script := `
# === VERSION FUNCTIONS (workaround for mvdan.cc/sh array slicing bug) ===
ver_cut() {
    local range="${1}"
    local v="${2:-${PV}}"

    local start end
    if [[ "${range}" == *-* ]]; then
        start="${range%-*}"
        end="${range#*-}"
        [[ -z "${end}" ]] && end=999
    else
        start="${range}"
        end="${range}"
    fi

    local idx=0
    local char prev_is_num=-1
    local current=""
    local len=${#v}
    local i

    eval "comp_0=''"
    idx=1

    i=0
    while (( i < len )); do
        char="${v:i:1}"

        if [[ "${char}" =~ [0-9] ]]; then
            if (( prev_is_num == 1 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 0 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=1
            fi
        elif [[ "${char}" =~ [a-zA-Z] ]]; then
            if (( prev_is_num == 0 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 1 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=0
            fi
        else
            if [[ -n "${current}" ]]; then
                eval "comp_${idx}='${current}'"
                ((idx++))
            fi
            eval "comp_${idx}='${char}'"
            ((idx++))
            current=""
            prev_is_num=-1
        fi
        ((i++))
    done

    if [[ -n "${current}" ]]; then
        eval "comp_${idx}='${current}'"
        ((idx++))
    fi

    local max=$(( (idx) / 2 ))
    [[ ${end} -gt ${max} ]] && end=${max}

    local result=""
    local n
    for (( n=start; n<=end; n++ )); do
        local comp_idx=$(( n*2 - 1 ))
        eval "result=\"\${result}\${comp_${comp_idx}}\""
        if (( n < end )); then
            local sep_idx=$(( n*2 ))
            eval "result=\"\${result}\${comp_${sep_idx}}\""
        fi
    done

    echo "${result}"
}

# Test cases
PV="13.4.1_p20250807"

echo "VER_CUT_1=$(ver_cut 1)"
echo "VER_CUT_2=$(ver_cut 2)"
echo "VER_CUT_3=$(ver_cut 3)"
echo "VER_CUT_1_2=$(ver_cut 1-2)"
echo "VER_CUT_1_3=$(ver_cut 1-3)"
echo "VER_CUT_4=$(ver_cut 4)"
echo "VER_CUT_5=$(ver_cut 5)"

# Test with explicit version
echo "VER_CUT_1_EXPLICIT=$(ver_cut 1 1.2.3_alpha4)"
`

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, io.Discard),
	)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}

	if err := runner.Run(context.Background(), prog); err != nil {
		t.Fatalf("run error: %v", err)
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Parse results
	results := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if idx := strings.Index(line, "="); idx > 0 {
			key := line[:idx]
			value := line[idx+1:]
			results[key] = value
		}
	}

	// Expected values for PV="13.4.1_p20250807"
	tests := []struct {
		key      string
		expected string
	}{
		{"VER_CUT_1", "13"},
		{"VER_CUT_2", "4"},
		{"VER_CUT_3", "1"},
		{"VER_CUT_1_2", "13.4"},
		{"VER_CUT_1_3", "13.4.1"},
		{"VER_CUT_4", "p"},
		{"VER_CUT_5", "20250807"},
		{"VER_CUT_1_EXPLICIT", "1"},
	}

	for _, tc := range tests {
		got := results[tc.key]
		if got != tc.expected {
			t.Errorf("%s: got %q, want %q", tc.key, got, tc.expected)
		}
	}
}

// TestGCCVersionExtraction tests the full gcc version extraction logic.
func TestGCCVersionExtraction(t *testing.T) {
	// Simulate toolchain.eclass version extraction using our ver_cut workaround
	script := `
# VERSION FUNCTIONS (workaround for mvdan.cc/sh array slicing bug)
ver_cut() {
    local range="${1}"
    local v="${2:-${PV}}"

    local start end
    if [[ "${range}" == *-* ]]; then
        start="${range%-*}"
        end="${range#*-}"
        [[ -z "${end}" ]] && end=999
    else
        start="${range}"
        end="${range}"
    fi

    local idx=0
    local char prev_is_num=-1
    local current=""
    local len=${#v}
    local i

    eval "comp_0=''"
    idx=1

    i=0
    while (( i < len )); do
        char="${v:i:1}"

        if [[ "${char}" =~ [0-9] ]]; then
            if (( prev_is_num == 1 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 0 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=1
            fi
        elif [[ "${char}" =~ [a-zA-Z] ]]; then
            if (( prev_is_num == 0 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 1 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=0
            fi
        else
            if [[ -n "${current}" ]]; then
                eval "comp_${idx}='${current}'"
                ((idx++))
            fi
            eval "comp_${idx}='${char}'"
            ((idx++))
            current=""
            prev_is_num=-1
        fi
        ((i++))
    done

    if [[ -n "${current}" ]]; then
        eval "comp_${idx}='${current}'"
        ((idx++))
    fi

    local max=$(( (idx) / 2 ))
    [[ ${end} -gt ${max} ]] && end=${max}

    local result=""
    local n
    for (( n=start; n<=end; n++ )); do
        local comp_idx=$(( n*2 - 1 ))
        eval "result=\"\${result}\${comp_${comp_idx}}\""
        if (( n < end )); then
            local sep_idx=$(( n*2 ))
            eval "result=\"\${result}\${comp_${sep_idx}}\""
        fi
    done

    echo "${result}"
}

# Simulate toolchain.eclass logic
PV="13.4.1_p20250807"
GCC_PV=${TOOLCHAIN_GCC_PV:-${PV}}
GCC_PVR=${GCC_PV}
GCC_RELEASE_VER=$(ver_cut 1-3 ${GCC_PV})
GCC_BRANCH_VER=$(ver_cut 1-2 ${GCC_PV})
GCCMAJOR=$(ver_cut 1 ${GCC_PV})
GCCMINOR=$(ver_cut 2 ${GCC_PV})
GCCMICRO=$(ver_cut 3 ${GCC_PV})

# Snapshot extraction (from toolchain.eclass lines 259-264)
if [[ ${GCC_PV} == *_pre* ]] ; then
    SNAPSHOT=${GCCMAJOR}-${GCC_PV##*_pre}
elif [[ ${GCC_PV} == *_p* ]] ; then
    SNAPSHOT=${GCCMAJOR}-${GCC_PV##*_p}
fi

echo "GCC_PV=${GCC_PV}"
echo "GCCMAJOR=${GCCMAJOR}"
echo "GCCMINOR=${GCCMINOR}"
echo "GCCMICRO=${GCCMICRO}"
echo "GCC_BRANCH_VER=${GCC_BRANCH_VER}"
echo "GCC_RELEASE_VER=${GCC_RELEASE_VER}"
echo "SNAPSHOT=${SNAPSHOT}"
`

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, io.Discard),
	)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}

	if err := runner.Run(context.Background(), prog); err != nil {
		t.Fatalf("run error: %v", err)
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Parse and verify
	results := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if idx := strings.Index(line, "="); idx > 0 {
			results[line[:idx]] = line[idx+1:]
		}
	}

	// Critical assertions
	if results["GCCMAJOR"] != "13" {
		t.Errorf("GCCMAJOR: got %q, want %q", results["GCCMAJOR"], "13")
	}
	if results["GCCMINOR"] != "4" {
		t.Errorf("GCCMINOR: got %q, want %q", results["GCCMINOR"], "4")
	}
	if results["GCCMICRO"] != "1" {
		t.Errorf("GCCMICRO: got %q, want %q", results["GCCMICRO"], "1")
	}
	if results["GCC_BRANCH_VER"] != "13.4" {
		t.Errorf("GCC_BRANCH_VER: got %q, want %q", results["GCC_BRANCH_VER"], "13.4")
	}
	if results["SNAPSHOT"] != "13-20250807" {
		t.Errorf("SNAPSHOT: got %q, want %q", results["SNAPSHOT"], "13-20250807")
	}
}

// TestMvdanShFunctionArgs verifies that mvdan.cc/sh correctly passes
// function arguments via $@. This is the root cause investigation for #50.
func TestMvdanShFunctionArgs(t *testing.T) {
	script := `
inherit() {
    for eclass in "$@"; do
        echo "ECLASS:$eclass"
    done
}
inherit toolchain flag-o-matic multilib
`

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.Env(expand.ListEnviron("PORTDIR=/var/db/repos/gentoo")),
		interp.StdIO(nil, &stdout, io.Discard),
	)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}

	if err := runner.Run(context.Background(), prog); err != nil {
		t.Fatalf("run error: %v", err)
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Verify all eclasses were processed
	expected := []string{
		"ECLASS:toolchain",
		"ECLASS:flag-o-matic",
		"ECLASS:multilib",
	}
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("missing expected output: %q", exp)
		}
	}
}

// TestMvdanShNestedFunctionArgs tests nested function calls with $@.
func TestMvdanShNestedFunctionArgs(t *testing.T) {
	script := `
process() {
    echo "PROCESS:$1"
}

inherit() {
    for eclass in "$@"; do
        process "$eclass"
    done
}

inherit one two three
`

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, io.Discard),
	)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}

	if err := runner.Run(context.Background(), prog); err != nil {
		t.Fatalf("run error: %v", err)
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	expected := []string{"PROCESS:one", "PROCESS:two", "PROCESS:three"}
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("missing expected output: %q", exp)
		}
	}
}

// TestMvdanShSourceWithInherit tests the actual pattern used in our code.
func TestMvdanShSourceWithInherit(t *testing.T) {
	// This simulates what happens when ebuild calls `inherit toolchain`
	// and inherit() sources the eclass file
	script := `
PORTDIR="/tmp/test"
INHERITED=""

inherit() {
    local eclass
    for eclass in "$@"; do
        if [[ " ${INHERITED} " == *" ${eclass} "* ]]; then
            continue
        fi
        echo "INHERIT:$eclass"
        INHERITED="${INHERITED:+${INHERITED} }${eclass}"
    done
}

# Simulate ebuild calling inherit
inherit toolchain flag-o-matic

echo "INHERITED=${INHERITED}"
`

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, io.Discard),
	)
	if err != nil {
		t.Fatalf("interp.New error: %v", err)
	}

	if err := runner.Run(context.Background(), prog); err != nil {
		t.Fatalf("run error: %v", err)
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Verify inherit was called with correct arguments
	if !strings.Contains(output, "INHERIT:toolchain") {
		t.Error("toolchain not inherited")
	}
	if !strings.Contains(output, "INHERIT:flag-o-matic") {
		t.Error("flag-o-matic not inherited")
	}
	if !strings.Contains(output, "INHERITED=toolchain flag-o-matic") {
		t.Error("INHERITED variable not set correctly")
	}
}
