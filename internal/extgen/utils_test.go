package extgen

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		content     string
		expectError bool
	}{
		{
			name:        "write simple file",
			filename:    "test.txt",
			content:     "hello world",
			expectError: false,
		},
		{
			name:        "write empty file",
			filename:    "empty.txt",
			content:     "",
			expectError: false,
		},
		{
			name:        "write file with special characters",
			filename:    "special.txt",
			content:     "hello\nworld\t!@#$%^&*()",
			expectError: false,
		},
		{
			name:        "write to invalid directory",
			filename:    "/nonexistent/directory/file.txt",
			content:     "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filename string
			if !tt.expectError {
				tempDir := t.TempDir()
				filename = filepath.Join(tempDir, tt.filename)
			} else {
				filename = tt.filename
			}

			err := writeFile(filename, tt.content)

			if tt.expectError {
				assert.Error(t, err, "writeFile() should return an error")
				return
			}

			assert.NoError(t, err, "writeFile() should not return an error")

			content, err := os.ReadFile(filename)
			assert.NoError(t, err, "Failed to read written file")
			assert.Equal(t, tt.content, string(content), "writeFile() content mismatch")

			info, err := os.Stat(filename)
			assert.NoError(t, err, "Failed to stat file")

			expectedMode := os.FileMode(0644)
			if runtime.GOOS == "windows" {
				expectedMode = os.FileMode(0666)
			}

			assert.Equal(t, expectedMode, info.Mode().Perm(), "writeFile() wrong permissions")
		})
	}
}

func TestReadFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:        "read simple file",
			content:     "hello world",
			expectError: false,
		},
		{
			name:        "read empty file",
			content:     "",
			expectError: false,
		},
		{
			name:        "read file with special characters",
			content:     "hello\nworld\t!@#$%^&*()",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "test.txt")

			err := os.WriteFile(filename, []byte(tt.content), 0644)
			assert.NoError(t, err, "Failed to create test file")

			content, err := readFile(filename)

			if tt.expectError {
				assert.Error(t, err, "readFile() should return an error")
				return
			}

			assert.NoError(t, err, "readFile() should not return an error")
			assert.Equal(t, tt.content, content, "readFile() content mismatch")
		})
	}

	t.Run("read nonexistent file", func(t *testing.T) {
		_, err := readFile("/nonexistent/file.txt")
		assert.Error(t, err, "readFile() should return an error for nonexistent file")
	})
}

func TestSanitizePackageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple valid name",
			input:    "mypackage",
			expected: "mypackage",
		},
		{
			name:     "name with hyphens",
			input:    "my-package",
			expected: "my_package",
		},
		{
			name:     "name with dots",
			input:    "my.package",
			expected: "my_package",
		},
		{
			name:     "name with both hyphens and dots",
			input:    "my-package.name",
			expected: "my_package_name",
		},
		{
			name:     "name starting with number",
			input:    "123package",
			expected: "_123package",
		},
		{
			name:     "name starting with underscore",
			input:    "_package",
			expected: "_package",
		},
		{
			name:     "name starting with letter",
			input:    "Package",
			expected: "Package",
		},
		{
			name:     "name starting with special character",
			input:    "@package",
			expected: "_@package",
		},
		{
			name:     "complex name",
			input:    "123my-complex.package@name",
			expected: "_123my_complex_package@name",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character letter",
			input:    "a",
			expected: "a",
		},
		{
			name:     "single character number",
			input:    "1",
			expected: "_1",
		},
		{
			name:     "single character underscore",
			input:    "_",
			expected: "_",
		},
		{
			name:     "single character special",
			input:    "@",
			expected: "_@",
		},
		{
			name:     "multiple consecutive hyphens",
			input:    "my--package",
			expected: "my__package",
		},
		{
			name:     "multiple consecutive dots",
			input:    "my..package",
			expected: "my__package",
		},
		{
			name:     "mixed case with special chars",
			input:    "MyPackage-name.version",
			expected: "MyPackage_name_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePackageName(tt.input)
			assert.Equal(t, tt.expected, result, "SanitizePackageName(%q)", tt.input)
		})
	}
}

func BenchmarkSanitizePackageName(b *testing.B) {
	testCases := []string{
		"simple",
		"my-package",
		"my.package.name",
		"123complex-package.name@version",
		"very-long-package-name-with-many-special-characters.and.dots",
	}

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			for b.Loop() {
				SanitizePackageName(tc)
			}
		})
	}
}
