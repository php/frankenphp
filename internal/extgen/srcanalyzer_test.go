package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name            string
		sourceContent   string
		expectedPackage string
	}{
		{
			name: "package main",
			sourceContent: `package main

import "fmt"

var globalVar = "test"

func helper() {
	fmt.Println("helper")
}`,
			expectedPackage: "main",
		},
		{
			name:            "custom package name",
			sourceContent:   "package myextension\n",
			expectedPackage: "myextension",
		},
	}

	analyzer := &SourceAnalyzer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "source.go")
			require.NoError(t, os.WriteFile(filename, []byte(tt.sourceContent), 0644))

			packageName, err := analyzer.analyze(filename)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPackage, packageName)
		})
	}
}

func TestSourceAnalyzer_Analyze_InvalidFile(t *testing.T) {
	analyzer := &SourceAnalyzer{}

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := analyzer.analyze("/nonexistent/file.go")
		assert.Error(t, err, "expected error for nonexistent file")
	})

	t.Run("invalid Go syntax", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "invalid.go")

		invalidContent := `package main
		func incomplete( {
			// invalid syntax
		`

		require.NoError(t, os.WriteFile(filename, []byte(invalidContent), 0644))

		_, err := analyzer.analyze(filename)
		assert.Error(t, err, "expected error for invalid syntax")
	})

	t.Run("missing package clause", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "orphan.go")
		require.NoError(t, os.WriteFile(filename, []byte("func orphan() {}\n"), 0644))

		_, err := analyzer.analyze(filename)
		assert.Error(t, err, "expected error for a file without a package clause")
	})
}

func BenchmarkSourceAnalyzer_Analyze(b *testing.B) {
	content := `package main

import "fmt"

var globalVar = "test"

func internalOne() {
	fmt.Println("one")
}

func internalTwo() {
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			fmt.Println(i)
		}
	}
}`

	filename := filepath.Join(b.TempDir(), "bench.go")
	require.NoError(b, os.WriteFile(filename, []byte(content), 0644))

	analyzer := &SourceAnalyzer{}

	for b.Loop() {
		_, err := analyzer.analyze(filename)
		require.NoError(b, err)
	}
}
