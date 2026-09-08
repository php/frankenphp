//go:build !nomercure

package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrustedIssuers(t *testing.T) {
	t.Parallel()

	for env, expected := range map[string][]string{
		"":                                    {"https://localhost"},
		"  ":                                  {"https://localhost"},
		"https://example.com":                 {"https://example.com"},
		"https://a.example,https://b.example": {"https://a.example", "https://b.example"},
		"https://a.example https://b.example": {"https://a.example", "https://b.example"},
		" https://a.example, \thttps://b.example ,, ": {"https://a.example", "https://b.example"},
	} {
		assert.Equal(t, expected, trustedIssuers(env), env)
	}
}
