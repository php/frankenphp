package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstantsIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const STATUS_OK = iota

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const: function test(): void
func Test() {
    // Implementation
}

func main() {}
`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	generator := &Generator{
		BaseName:   "testext",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	require.NoError(t, generator.parseSource())
	assert.Len(t, generator.Constants, 2, "Expected 2 constants")

	expectedConstants := map[string]struct {
		Value  string
		IsIota bool
	}{
		"STATUS_OK":       {"0", true},
		"MAX_CONNECTIONS": {"100", false},
	}

	for _, constant := range generator.Constants {
		expected, exists := expectedConstants[constant.Name]
		assert.True(t, exists, "Unexpected constant: %s", constant.Name)
		if !exists {
			continue
		}

		assert.Equal(t, expected.Value, constant.Value, "Constant %s: value mismatch", constant.Name)
		assert.Equal(t, expected.IsIota, constant.IsIota, "Constant %s: isIota mismatch", constant.Name)
	}

	require.NoError(t, generator.setupBuildDirectory())
	require.NoError(t, generator.generateStubFile())

	stubPath := filepath.Join(generator.BuildDir, generator.BaseName+".stub.php")
	stubContent, err := os.ReadFile(stubPath)
	require.NoError(t, err)

	stubStr := string(stubContent)

	assert.Contains(t, stubStr, "* @cvalue", "Stub does not contain @cvalue annotation for iota constant")
	assert.Contains(t, stubStr, "const STATUS_OK = UNKNOWN;", "Stub does not contain STATUS_OK constant with UNKNOWN value")
	assert.Contains(t, stubStr, "const MAX_CONNECTIONS = 100;", "Stub does not contain MAX_CONNECTIONS constant with explicit value")

	require.NoError(t, generator.generateCFile())

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	require.NoError(t, err)

	cStr := string(cContent)

	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("STATUS_OK", STATUS_OK, CONST_CS | CONST_PERSISTENT);`, "C file does not contain STATUS_OK registration")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("MAX_CONNECTIONS", 100, CONST_CS | CONST_PERSISTENT);`, "C file does not contain MAX_CONNECTIONS registration")
}

func TestConstantsIntegrationOctal(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const FILE_PERM = 0o755

//export_php:const
const OTHER_PERM = 0o644

//export_php:const
const REGULAR_INT = 42

func main() {}
`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	generator := &Generator{
		BaseName:   "octalstest",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	require.NoError(t, generator.parseSource())
	assert.Len(t, generator.Constants, 3, "Expected 3 constants")

	// Verify CValue conversion
	for _, constant := range generator.Constants {
		switch constant.Name {
		case "FILE_PERM":
			assert.Equal(t, "0o755", constant.Value, "FILE_PERM value mismatch")
			assert.Equal(t, "493", constant.CValue(), "FILE_PERM CValue mismatch")
		case "OTHER_PERM":
			assert.Equal(t, "0o644", constant.Value, "OTHER_PERM value mismatch")
			assert.Equal(t, "420", constant.CValue(), "OTHER_PERM CValue mismatch")
		case "REGULAR_INT":
			assert.Equal(t, "42", constant.Value, "REGULAR_INT value mismatch")
			assert.Equal(t, "42", constant.CValue(), "REGULAR_INT CValue mismatch")
		}
	}

	require.NoError(t, generator.setupBuildDirectory())

	// Test C file generation
	require.NoError(t, generator.generateCFile())

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	require.NoError(t, err)

	cStr := string(cContent)

	// Verify C file uses decimal values for octal constants
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("FILE_PERM", 493, CONST_CS | CONST_PERSISTENT);`, "C file does not contain FILE_PERM registration with decimal value 493")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("OTHER_PERM", 420, CONST_CS | CONST_PERSISTENT);`, "C file does not contain OTHER_PERM registration with decimal value 420")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("REGULAR_INT", 42, CONST_CS | CONST_PERSISTENT);`, "C file does not contain REGULAR_INT registration with value 42")

	// Test header file generation
	require.NoError(t, generator.generateHeaderFile())

	hPath := filepath.Join(generator.BuildDir, generator.BaseName+".h")
	hContent, err := os.ReadFile(hPath)
	require.NoError(t, err)

	hStr := string(hContent)

	// Verify header file uses decimal values for octal constants in #define
	assert.Contains(t, hStr, "#define FILE_PERM 493", "Header file does not contain FILE_PERM #define with decimal value 493")
	assert.Contains(t, hStr, "#define OTHER_PERM 420", "Header file does not contain OTHER_PERM #define with decimal value 420")
	assert.Contains(t, hStr, "#define REGULAR_INT 42", "Header file does not contain REGULAR_INT #define with value 42")
}
