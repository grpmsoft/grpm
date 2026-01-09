package repo

import (
	"testing"
)

func TestParseSrcURI_SimpleURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantURL  string
		wantFile string
	}{
		{
			name:     "simple https URL",
			input:    "https://example.com/foo-1.0.tar.gz",
			wantLen:  1,
			wantURL:  "https://example.com/foo-1.0.tar.gz",
			wantFile: "foo-1.0.tar.gz",
		},
		{
			name:     "simple http URL",
			input:    "http://example.com/bar.tar.bz2",
			wantLen:  1,
			wantURL:  "http://example.com/bar.tar.bz2",
			wantFile: "bar.tar.bz2",
		},
		{
			name:     "ftp URL",
			input:    "ftp://ftp.example.com/pub/file.tar.xz",
			wantLen:  1,
			wantURL:  "ftp://ftp.example.com/pub/file.tar.xz",
			wantFile: "file.tar.xz",
		},
		{
			name:     "mirror URL",
			input:    "mirror://sourceforge/project/file.tar.gz",
			wantLen:  1,
			wantURL:  "mirror://sourceforge/project/file.tar.gz",
			wantFile: "file.tar.gz",
		},
		{
			name:     "multiple URLs",
			input:    "https://example.com/a.tar.gz https://example.com/b.tar.gz",
			wantLen:  2,
			wantURL:  "https://example.com/a.tar.gz",
			wantFile: "a.tar.gz",
		},
		{
			name:    "empty input",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			input:   "   \t\n  ",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, nil, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantLen)
			}

			if tt.wantLen > 0 {
				if entries[0].URL != tt.wantURL {
					t.Errorf("URL = %q, want %q", entries[0].URL, tt.wantURL)
				}
				if entries[0].Filename != tt.wantFile {
					t.Errorf("Filename = %q, want %q", entries[0].Filename, tt.wantFile)
				}
			}
		})
	}
}

func TestParseSrcURI_ArrowSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantURL  string
		wantFile string
	}{
		{
			name:     "simple arrow",
			input:    "https://example.com/foo.tar.gz -> bar.tar.gz",
			wantURL:  "https://example.com/foo.tar.gz",
			wantFile: "bar.tar.gz",
		},
		{
			name:     "github archive rename",
			input:    "https://github.com/user/repo/archive/v1.0.tar.gz -> myproject-1.0.tar.gz",
			wantURL:  "https://github.com/user/repo/archive/v1.0.tar.gz",
			wantFile: "myproject-1.0.tar.gz",
		},
		{
			name:     "arrow with path basename",
			input:    "https://downloads.example.com/releases/v1.2.3/latest.tar.gz -> specific-1.2.3.tar.gz",
			wantURL:  "https://downloads.example.com/releases/v1.2.3/latest.tar.gz",
			wantFile: "specific-1.2.3.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, nil, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(entries))
			}

			if entries[0].URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", entries[0].URL, tt.wantURL)
			}
			if entries[0].Filename != tt.wantFile {
				t.Errorf("Filename = %q, want %q", entries[0].Filename, tt.wantFile)
			}
		})
	}
}

func TestParseSrcURI_VariableExpansion(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"P":  "hello-1.0",
		"PV": "1.0",
		"PN": "hello",
		"PF": "hello-1.0-r0",
	}

	tests := []struct {
		name     string
		input    string
		vars     map[string]string
		wantFile string
	}{
		{
			name:     "expand P in filename",
			input:    "https://example.com/v1.0.tar.gz -> ${P}.tar.gz",
			vars:     vars,
			wantFile: "hello-1.0.tar.gz",
		},
		{
			name:     "expand PV in filename",
			input:    "https://example.com/archive.tar.gz -> myapp-${PV}.tar.gz",
			vars:     vars,
			wantFile: "myapp-1.0.tar.gz",
		},
		{
			name:     "expand PN in filename",
			input:    "https://example.com/src.tar.gz -> ${PN}-src.tar.gz",
			vars:     vars,
			wantFile: "hello-src.tar.gz",
		},
		{
			name:     "multiple variables",
			input:    "https://example.com/src.tar.gz -> ${PN}-${PV}-src.tar.gz",
			vars:     vars,
			wantFile: "hello-1.0-src.tar.gz",
		},
		{
			name:     "no expansion without vars",
			input:    "https://example.com/src.tar.gz -> ${P}.tar.gz",
			vars:     nil,
			wantFile: ".tar.gz", // Variable removed, leaving just extension
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, nil, tt.vars)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(entries))
			}

			if entries[0].Filename != tt.wantFile {
				t.Errorf("Filename = %q, want %q", entries[0].Filename, tt.wantFile)
			}
		})
	}
}

func TestParseSrcURI_Conditionals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		flags   map[string]bool
		wantLen int
		wantURL string
	}{
		{
			name:    "enabled flag includes URL",
			input:   "doc? ( https://example.com/doc.pdf )",
			flags:   map[string]bool{"doc": true},
			wantLen: 1,
			wantURL: "https://example.com/doc.pdf",
		},
		{
			name:    "disabled flag excludes URL",
			input:   "doc? ( https://example.com/doc.pdf )",
			flags:   map[string]bool{"doc": false},
			wantLen: 0,
		},
		{
			name:    "missing flag excludes URL",
			input:   "doc? ( https://example.com/doc.pdf )",
			flags:   map[string]bool{},
			wantLen: 0,
		},
		{
			name:    "negated flag - disabled includes URL",
			input:   "!minimal? ( https://example.com/extras.tar.gz )",
			flags:   map[string]bool{"minimal": false},
			wantLen: 1,
			wantURL: "https://example.com/extras.tar.gz",
		},
		{
			name:    "negated flag - enabled excludes URL",
			input:   "!minimal? ( https://example.com/extras.tar.gz )",
			flags:   map[string]bool{"minimal": true},
			wantLen: 0,
		},
		{
			name:    "negated flag - missing includes URL",
			input:   "!minimal? ( https://example.com/extras.tar.gz )",
			flags:   map[string]bool{},
			wantLen: 1,
			wantURL: "https://example.com/extras.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, tt.flags, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantLen)
			}

			if tt.wantLen > 0 && entries[0].URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", entries[0].URL, tt.wantURL)
			}
		})
	}
}

func TestParseSrcURI_ConditionalMetadata(t *testing.T) {
	t.Parallel()

	input := "ssl? ( https://example.com/ssl.tar.gz )"
	flags := map[string]bool{"ssl": true}

	entries, err := ParseSrcURI(input, flags, nil)
	if err != nil {
		t.Fatalf("ParseSrcURI() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.UseFlag != "ssl" {
		t.Errorf("UseFlag = %q, want %q", entry.UseFlag, "ssl")
	}
	if entry.Negate {
		t.Error("Negate = true, want false")
	}
}

func TestParseSrcURI_NegatedConditionalMetadata(t *testing.T) {
	t.Parallel()

	input := "!minimal? ( https://example.com/extras.tar.gz )"
	flags := map[string]bool{"minimal": false}

	entries, err := ParseSrcURI(input, flags, nil)
	if err != nil {
		t.Fatalf("ParseSrcURI() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.UseFlag != "minimal" {
		t.Errorf("UseFlag = %q, want %q", entry.UseFlag, "minimal")
	}
	if !entry.Negate {
		t.Error("Negate = false, want true")
	}
}

func TestParseSrcURI_NestedConditionals(t *testing.T) {
	t.Parallel()

	input := `
		ssl? (
			gnutls? ( https://example.com/gnutls.tar.gz )
			openssl? ( https://example.com/openssl.tar.gz )
		)
	`

	tests := []struct {
		name    string
		flags   map[string]bool
		wantLen int
		wantURL string
	}{
		{
			name:    "ssl and gnutls enabled",
			flags:   map[string]bool{"ssl": true, "gnutls": true, "openssl": false},
			wantLen: 1,
			wantURL: "https://example.com/gnutls.tar.gz",
		},
		{
			name:    "ssl and openssl enabled",
			flags:   map[string]bool{"ssl": true, "gnutls": false, "openssl": true},
			wantLen: 1,
			wantURL: "https://example.com/openssl.tar.gz",
		},
		{
			name:    "ssl enabled, both backends",
			flags:   map[string]bool{"ssl": true, "gnutls": true, "openssl": true},
			wantLen: 2,
		},
		{
			name:    "ssl disabled",
			flags:   map[string]bool{"ssl": false, "gnutls": true, "openssl": true},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(input, tt.flags, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantLen)
			}

			if tt.wantLen == 1 && entries[0].URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", entries[0].URL, tt.wantURL)
			}
		})
	}
}

func TestParseSrcURI_ConditionalWithArrow(t *testing.T) {
	t.Parallel()

	input := "ssl? ( https://example.com/ssl-support.tar.gz -> ${P}-ssl.tar.gz )"
	flags := map[string]bool{"ssl": true}
	vars := map[string]string{"P": "myapp-1.0"}

	entries, err := ParseSrcURI(input, flags, vars)
	if err != nil {
		t.Fatalf("ParseSrcURI() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.URL != "https://example.com/ssl-support.tar.gz" {
		t.Errorf("URL = %q, want %q", entry.URL, "https://example.com/ssl-support.tar.gz")
	}
	if entry.Filename != "myapp-1.0-ssl.tar.gz" {
		t.Errorf("Filename = %q, want %q", entry.Filename, "myapp-1.0-ssl.tar.gz")
	}
}

func TestParseSrcURI_MixedContent(t *testing.T) {
	t.Parallel()

	// Real-world style SRC_URI
	input := `
		https://example.com/main-1.0.tar.gz
		doc? ( https://example.com/manual.pdf -> ${PN}-manual.pdf )
		!minimal? ( https://example.com/extras.tar.gz )
		test? (
			https://example.com/test-data.tar.gz
			https://example.com/test-scripts.tar.gz -> test-scripts-1.0.tar.gz
		)
	`

	vars := map[string]string{"PN": "myapp", "P": "myapp-1.0", "PV": "1.0"}
	flags := map[string]bool{
		"doc":     true,
		"minimal": false,
		"test":    true,
	}

	entries, err := ParseSrcURI(input, flags, vars)
	if err != nil {
		t.Fatalf("ParseSrcURI() error = %v", err)
	}

	// Expected: main.tar.gz + manual.pdf + extras.tar.gz + test-data.tar.gz + test-scripts.tar.gz
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}

	// Check specific entries
	expectedFiles := map[string]bool{
		"main-1.0.tar.gz":         true,
		"myapp-manual.pdf":        true,
		"extras.tar.gz":           true,
		"test-data.tar.gz":        true,
		"test-scripts-1.0.tar.gz": true,
	}

	for _, entry := range entries {
		if !expectedFiles[entry.Filename] {
			t.Errorf("unexpected filename: %q", entry.Filename)
		}
		delete(expectedFiles, entry.Filename)
	}

	if len(expectedFiles) > 0 {
		for f := range expectedFiles {
			t.Errorf("missing expected filename: %q", f)
		}
	}
}

func TestParseSrcURI_URLWithQueryString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantFile string
	}{
		{
			name:     "URL with query string",
			input:    "https://example.com/download.php?file=foo.tar.gz",
			wantFile: "download.php",
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/file.tar.gz#section",
			wantFile: "file.tar.gz",
		},
		{
			name:     "arrow overrides query string",
			input:    "https://example.com/download.php?file=foo -> actual-file.tar.gz",
			wantFile: "actual-file.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, nil, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(entries))
			}

			if entries[0].Filename != tt.wantFile {
				t.Errorf("Filename = %q, want %q", entries[0].Filename, tt.wantFile)
			}
		})
	}
}

func TestParseSrcURI_MirrorURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantFile string
	}{
		{
			name:     "sourceforge mirror",
			input:    "mirror://sourceforge/project/file.tar.gz",
			wantFile: "file.tar.gz",
		},
		{
			name:     "gnu mirror",
			input:    "mirror://gnu/hello/hello-2.10.tar.gz",
			wantFile: "hello-2.10.tar.gz",
		},
		{
			name:     "mirror with arrow",
			input:    "mirror://sourceforge/proj/src.tar.gz -> custom-name.tar.gz",
			wantFile: "custom-name.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseSrcURI(tt.input, nil, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI() error = %v", err)
			}

			if len(entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(entries))
			}

			if entries[0].Filename != tt.wantFile {
				t.Errorf("Filename = %q, want %q", entries[0].Filename, tt.wantFile)
			}
		})
	}
}

func TestTokenizeSrcURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "simple URL",
			input:  "https://example.com/file.tar.gz",
			expect: []string{"https://example.com/file.tar.gz"},
		},
		{
			name:   "arrow syntax",
			input:  "https://example.com/file.tar.gz -> renamed.tar.gz",
			expect: []string{"https://example.com/file.tar.gz", "->", "renamed.tar.gz"},
		},
		{
			name:   "conditional",
			input:  "ssl? ( https://example.com/ssl.tar.gz )",
			expect: []string{"ssl?", "(", "https://example.com/ssl.tar.gz", ")"},
		},
		{
			name:   "negated conditional",
			input:  "!minimal? ( https://example.com/extras.tar.gz )",
			expect: []string{"!minimal?", "(", "https://example.com/extras.tar.gz", ")"},
		},
		{
			name:   "multiline",
			input:  "https://a.com/a.tar.gz\nhttps://b.com/b.tar.gz",
			expect: []string{"https://a.com/a.tar.gz", "https://b.com/b.tar.gz"},
		},
		{
			name:   "tabs and spaces",
			input:  "https://a.com/a.tar.gz \t https://b.com/b.tar.gz",
			expect: []string{"https://a.com/a.tar.gz", "https://b.com/b.tar.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tokens := tokenizeSrcURI(tt.input)

			if len(tokens) != len(tt.expect) {
				t.Fatalf("got %d tokens %v, want %d %v", len(tokens), tokens, len(tt.expect), tt.expect)
			}

			for i, tok := range tokens {
				if tok != tt.expect[i] {
					t.Errorf("token[%d] = %q, want %q", i, tok, tt.expect[i])
				}
			}
		})
	}
}

func TestIsUseCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"ssl?", true},
		{"!ssl?", true},
		{"doc?", true},
		{"!minimal?", true},
		{"ssl", false},
		{"?", false},
		{"!?", false},
		{"->", false},
		{"(", false},
		{")", false},
		{"https://example.com?query=1", false},
		{"USE_EXPAND_HIDDEN?", true},
		{"flag-with-dash?", true},
		{"flag_with_underscore?", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := isUseCondition(tt.input)
			if got != tt.want {
				t.Errorf("isUseCondition(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUseCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input      string
		wantFlag   string
		wantNegate bool
	}{
		{"ssl?", "ssl", false},
		{"!ssl?", "ssl", true},
		{"doc?", "doc", false},
		{"!minimal?", "minimal", true},
		{"USE_EXPAND?", "USE_EXPAND", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			flag, negate := parseUseCondition(tt.input)
			if flag != tt.wantFlag {
				t.Errorf("flag = %q, want %q", flag, tt.wantFlag)
			}
			if negate != tt.wantNegate {
				t.Errorf("negate = %v, want %v", negate, tt.wantNegate)
			}
		})
	}
}

func TestExpandFilename(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"P":  "hello-1.0",
		"PV": "1.0",
		"PN": "hello",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"${P}.tar.gz", "hello-1.0.tar.gz"},
		{"${PN}-${PV}.tar.gz", "hello-1.0.tar.gz"},
		{"no-vars.tar.gz", "no-vars.tar.gz"},
		{"${UNKNOWN}.tar.gz", ".tar.gz"},
		{"prefix-${P}-suffix.tar.gz", "prefix-hello-1.0-suffix.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := ExpandFilename(tt.input, vars)
			if got != tt.want {
				t.Errorf("ExpandFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url      string
		wantFile string
	}{
		{"https://example.com/file.tar.gz", "file.tar.gz"},
		{"https://example.com/path/to/file.tar.gz", "file.tar.gz"},
		{"https://example.com/file.tar.gz?query=1", "file.tar.gz"},
		{"https://example.com/file.tar.gz#section", "file.tar.gz"},
		{"mirror://sourceforge/proj/file.tar.gz", "file.tar.gz"},
		{"mirror://gnu/hello/hello-2.10.tar.gz", "hello-2.10.tar.gz"},
		{"ftp://ftp.example.com/pub/file.tar.xz", "file.tar.xz"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()

			got := extractFilename(tt.url)
			if got != tt.wantFile {
				t.Errorf("extractFilename(%q) = %q, want %q", tt.url, got, tt.wantFile)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/file.tar.gz", true},
		{"http://example.com/file.tar.gz", true},
		{"ftp://ftp.example.com/file.tar.gz", true},
		{"mirror://sourceforge/proj/file.tar.gz", true},
		{"HTTPS://EXAMPLE.COM/FILE.TAR.GZ", true},
		{"file.tar.gz", false},
		{"ssl?", false},
		{"->", false},
		{"(", false},
		{")", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := isURL(tt.input)
			if got != tt.want {
				t.Errorf("isURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandDefaultSyntax(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"P":  "hello-1.0",
		"PV": "1.0",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"${P:-unknown}.tar.gz", "hello-1.0.tar.gz"},
		{"${MISSING:-default}.tar.gz", "default.tar.gz"},
		{"${MISSING:-}.tar.gz", ".tar.gz"},
		{"no-default.tar.gz", "no-default.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := expandDefaultSyntax(tt.input, vars)
			if got != tt.want {
				t.Errorf("expandDefaultSyntax(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSrcURIParser_NewParser(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"P": "test-1.0"}
	flags := map[string]bool{"ssl": true}

	parser := NewSrcURIParser(vars, flags)

	// Verify vars are copied (not shared)
	vars["P"] = "modified"
	if parser.vars["P"] != "test-1.0" {
		t.Error("parser vars should be a copy, not shared reference")
	}

	// Verify flags are copied
	flags["ssl"] = false
	if !parser.activeFlags["ssl"] {
		t.Error("parser flags should be a copy, not shared reference")
	}
}

// Benchmark tests
func BenchmarkParseSrcURI_Simple(b *testing.B) {
	input := "https://example.com/file.tar.gz"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSrcURI(input, nil, nil)
	}
}

func BenchmarkParseSrcURI_Complex(b *testing.B) {
	input := `
		https://example.com/main-1.0.tar.gz
		doc? ( https://example.com/manual.pdf -> ${PN}-manual.pdf )
		!minimal? ( https://example.com/extras.tar.gz )
		test? (
			https://example.com/test-data.tar.gz
			https://example.com/test-scripts.tar.gz -> test-scripts-1.0.tar.gz
		)
	`
	vars := map[string]string{"PN": "myapp", "P": "myapp-1.0", "PV": "1.0"}
	flags := map[string]bool{"doc": true, "minimal": false, "test": true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseSrcURI(input, flags, vars)
	}
}

func BenchmarkTokenizeSrcURI(b *testing.B) {
	input := "ssl? ( https://example.com/ssl.tar.gz -> ${P}-ssl.tar.gz )"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenizeSrcURI(input)
	}
}
