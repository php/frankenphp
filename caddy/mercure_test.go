//go:build !nomercure

package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCreateMercureRouteResourceIdentifier(t *testing.T) {
	t.Setenv("MERCURE_PUBLISHER_JWT_KEY", "publisher")
	t.Setenv("MERCURE_SUBSCRIBER_JWT_KEY", "subscriber")
	t.Setenv("MERCURE_RESOURCE_IDENTIFIER", "https://example.com/.well-known/mercure")

	route, err := createMercureRoute()
	require.NoError(t, err)
	require.Len(t, route.HandlersRaw, 1)
	assert.Contains(t, string(route.HandlersRaw[0]), `"resource_identifier":"https://example.com/.well-known/mercure"`)
}
