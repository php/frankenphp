package caddy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandDefaultPatternSkipsGeneratedDirs(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{".git", "config", "node_modules", "src", "storage", "var", "vendor"} {
		require.NoError(t, os.Mkdir(dir+"/"+d, 0o755))
	}
	require.NoError(t, os.WriteFile(dir+"/composer.json", nil, 0o644))
	t.Chdir(dir)

	assert.Equal(t, []string{
		"./src/**/*.php",
		"./*.{env,php}",
		"./config/**/*.{env,php}",
		"./src/**/*.{env,php}",
		"./vendor/**/*.{env,php}",
	}, expandDefaultPattern([]string{"./src/**/*.php", "./**/*.{env,php}"}, "./**/*.{env,php}"))
}

func TestExpandDefaultPatternKeepsCustomPatterns(t *testing.T) {
	patterns := []string{"./**/*.php"}

	assert.Equal(t, patterns, expandDefaultPattern(patterns, defaultWatchPattern))
}
