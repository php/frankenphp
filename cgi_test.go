package frankenphp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureLeadingSlash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"/index.php", "/index.php"},
		{"index.php", "/index.php"},
		{"/", "/"},
		{"", ""},
		{"/path/to/script.php", "/path/to/script.php"},
		{"path/to/script.php", "/path/to/script.php"},
		{"/index.php/path/info", "/index.php/path/info"},
		{"index.php/path/info", "/index.php/path/info"},
	}

	for _, tt := range tests {
		t.Run(tt.input + "-" + tt.expected, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, ensureLeadingSlash(tt.input), "ensureLeadingSlash(%q)", tt.input)
		})
	}
}
