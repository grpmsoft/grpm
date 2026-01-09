package pkg

import (
	"testing"
)

func TestRequiredUseValidator_SimpleFlags(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Single flag enabled",
			expr:    "ssl",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "Single flag disabled",
			expr:    "ssl",
			flags:   []string{},
			wantErr: true,
		},
		{
			name:    "Negated flag - flag disabled",
			expr:    "!debug",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "Negated flag - flag enabled",
			expr:    "!debug",
			flags:   []string{"debug"},
			wantErr: true,
		},
		{
			name:    "Multiple flags - all enabled",
			expr:    "ssl gnutls",
			flags:   []string{"ssl", "gnutls"},
			wantErr: false,
		},
		{
			name:    "Multiple flags - one missing",
			expr:    "ssl gnutls",
			flags:   []string{"ssl"},
			wantErr: true,
		},
		{
			name:    "Empty expression",
			expr:    "",
			flags:   []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseValidator_AnyOf(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Any-of: one enabled",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "Any-of: other enabled",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{"gnutls"},
			wantErr: false,
		},
		{
			name:    "Any-of: both enabled",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{"ssl", "gnutls"},
			wantErr: false,
		},
		{
			name:    "Any-of: none enabled",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{},
			wantErr: true,
		},
		{
			name:    "Any-of: three options, one enabled",
			expr:    "|| ( mysql postgres sqlite )",
			flags:   []string{"sqlite"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseValidator_ExactlyOne(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Exactly-one: one enabled",
			expr:    "^^ ( ssl gnutls )",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "Exactly-one: other enabled",
			expr:    "^^ ( ssl gnutls )",
			flags:   []string{"gnutls"},
			wantErr: false,
		},
		{
			name:    "Exactly-one: both enabled (invalid)",
			expr:    "^^ ( ssl gnutls )",
			flags:   []string{"ssl", "gnutls"},
			wantErr: true,
		},
		{
			name:    "Exactly-one: none enabled (invalid)",
			expr:    "^^ ( ssl gnutls )",
			flags:   []string{},
			wantErr: true,
		},
		{
			name:    "Exactly-one: three options, one enabled",
			expr:    "^^ ( openssl gnutls libressl )",
			flags:   []string{"openssl"},
			wantErr: false,
		},
		{
			name:    "Exactly-one: three options, two enabled (invalid)",
			expr:    "^^ ( openssl gnutls libressl )",
			flags:   []string{"openssl", "gnutls"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseValidator_AtMostOne(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "At-most-one: one enabled",
			expr:    "?? ( ssl gnutls )",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "At-most-one: none enabled",
			expr:    "?? ( ssl gnutls )",
			flags:   []string{},
			wantErr: false,
		},
		{
			name:    "At-most-one: both enabled (invalid)",
			expr:    "?? ( ssl gnutls )",
			flags:   []string{"ssl", "gnutls"},
			wantErr: true,
		},
		{
			name:    "At-most-one: three options, one enabled",
			expr:    "?? ( mysql postgres sqlite )",
			flags:   []string{"mysql"},
			wantErr: false,
		},
		{
			name:    "At-most-one: three options, two enabled (invalid)",
			expr:    "?? ( mysql postgres sqlite )",
			flags:   []string{"mysql", "postgres"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseValidator_Conditionals(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Conditional: condition not met, skipped",
			expr:    "ssl? ( gnutls )",
			flags:   []string{},
			wantErr: false, // ssl not enabled, so gnutls not required
		},
		{
			name:    "Conditional: condition met, inner satisfied",
			expr:    "ssl? ( gnutls )",
			flags:   []string{"ssl", "gnutls"},
			wantErr: false,
		},
		{
			name:    "Conditional: condition met, inner not satisfied",
			expr:    "ssl? ( gnutls )",
			flags:   []string{"ssl"},
			wantErr: true, // ssl enabled but gnutls not
		},
		{
			name:    "Negated conditional: condition not met, inner evaluated",
			expr:    "!ssl? ( gnutls )",
			flags:   []string{"gnutls"},
			wantErr: false, // ssl not enabled, gnutls required and enabled
		},
		{
			name:    "Negated conditional: condition met, skipped",
			expr:    "!ssl? ( gnutls )",
			flags:   []string{"ssl"},
			wantErr: false, // ssl enabled, gnutls not required
		},
		{
			name:    "Negated conditional: condition not met, inner not satisfied",
			expr:    "!ssl? ( gnutls )",
			flags:   []string{},
			wantErr: true, // ssl not enabled, gnutls required but not enabled
		},
		{
			name:    "Conditional with negated inner",
			expr:    "ssl? ( !gnutls )",
			flags:   []string{"ssl"},
			wantErr: false, // ssl enabled, gnutls not enabled (satisfied)
		},
		{
			name:    "Conditional with negated inner - violation",
			expr:    "ssl? ( !gnutls )",
			flags:   []string{"ssl", "gnutls"},
			wantErr: true, // ssl enabled, gnutls should be disabled but is enabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseValidator_Complex(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Combined: simple + any-of",
			expr:    "python || ( ssl gnutls )",
			flags:   []string{"python", "ssl"},
			wantErr: false,
		},
		{
			name:    "Combined: simple missing",
			expr:    "python || ( ssl gnutls )",
			flags:   []string{"ssl"},
			wantErr: true,
		},
		{
			name:    "Combined: any-of not satisfied",
			expr:    "python || ( ssl gnutls )",
			flags:   []string{"python"},
			wantErr: true,
		},
		{
			name:    "Multiple operators",
			expr:    "^^ ( ssl gnutls ) || ( mysql postgres )",
			flags:   []string{"ssl", "postgres"},
			wantErr: false,
		},
		{
			name:    "Real-world: audio backend selection",
			expr:    "^^ ( alsa pulseaudio pipewire )",
			flags:   []string{"pipewire"},
			wantErr: false,
		},
		{
			name:    "Real-world: SSL provider",
			expr:    "ssl? ( ^^ ( openssl gnutls libressl ) )",
			flags:   []string{"ssl", "openssl"},
			wantErr: false,
		},
		{
			name:    "Real-world: SSL provider - ssl not enabled",
			expr:    "ssl? ( ^^ ( openssl gnutls libressl ) )",
			flags:   []string{},
			wantErr: false, // ssl not enabled, inner not evaluated
		},
		{
			name:    "Real-world: SSL provider - ssl enabled, no provider",
			expr:    "ssl? ( ^^ ( openssl gnutls libressl ) )",
			flags:   []string{"ssl"},
			wantErr: true, // ssl enabled, exactly one provider needed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewRequiredUseValidator()
			err := v.Validate(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredUseError(t *testing.T) {
	err := &RequiredUseError{
		Expression: "|| ( ssl gnutls )",
		Reason:     "USE flag constraints not satisfied",
	}

	expected := "REQUIRED_USE violation: USE flag constraints not satisfied (expression: || ( ssl gnutls ))"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestValidateRequiredUse_ConvenienceFunction(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "Valid expression",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{"ssl"},
			wantErr: false,
		},
		{
			name:    "Invalid expression",
			expr:    "|| ( ssl gnutls )",
			flags:   []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredUse(tt.expr, tt.flags)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequiredUse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkRequiredUseValidator_Simple(b *testing.B) {
	expr := "ssl gnutls python"
	flags := []string{"ssl", "gnutls", "python"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := NewRequiredUseValidator()
		_ = v.Validate(expr, flags)
	}
}

func BenchmarkRequiredUseValidator_Complex(b *testing.B) {
	expr := "ssl? ( ^^ ( openssl gnutls libressl ) ) || ( mysql postgres sqlite )"
	flags := []string{"ssl", "openssl", "mysql"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := NewRequiredUseValidator()
		_ = v.Validate(expr, flags)
	}
}
