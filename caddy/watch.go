package caddy

import (
	"os"
	"slices"
	"strings"
)

// generatedDirs hold caches, logs and front-end dependencies: changes there must not restart workers or reload pages.
var generatedDirs = []string{"node_modules", "storage", "var"}

// expandDefaultPattern replaces the default "./**/*.{...}" pattern with one pattern per source directory, as globs can't exclude paths.
func expandDefaultPattern(patterns []string, defaultPattern string) []string {
	i := slices.Index(patterns, defaultPattern)
	if i == -1 {
		return patterns
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		return patterns
	}

	filePattern := strings.TrimPrefix(defaultPattern, "./**/")
	expanded := []string{"./" + filePattern}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || slices.Contains(generatedDirs, name) {
			continue
		}

		expanded = append(expanded, "./"+name+"/**/"+filePattern)
	}

	return slices.Concat(patterns[:i], expanded, patterns[i+1:])
}
