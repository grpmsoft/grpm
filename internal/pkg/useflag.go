package pkg

import (
	"strings"

	"github.com/coregx/coregex"
)

// Precompiled regex for USE flag conditional parsing.
// Pattern: flagname? ( dependencies )
// Example: "ssl? ( >=dev-libs/openssl-1.1.0 )"
var useFlagConditionalRe = coregex.MustCompile(`([a-zA-Z0-9_]+)\?\s*\((.*)\)`)

// UseFlag represents a Gentoo USE flag with conditional dependencies
type UseFlag struct {
	Name      string
	Default   bool
	Enabled   bool
	Condition string // USE flag condition
}

// ParseUseFlag parses a USE flag string
func ParseUseFlag(flag string) UseFlag {
	// Example: ssl? ( >=dev-libs/openssl-1.1.0 )
	matches := useFlagConditionalRe.FindStringSubmatch(flag)

	if len(matches) == 3 {
		return UseFlag{
			Name:      matches[1],
			Condition: strings.TrimSpace(matches[2]),
		}
	}

	return UseFlag{
		Name: strings.TrimPrefix(flag, "-"),
	}
}

func (u *UseFlag) IsEnabled(flags map[string]bool) bool {
	if enabled, exists := flags[u.Name]; exists {
		return enabled
	}
	return u.Default
}
