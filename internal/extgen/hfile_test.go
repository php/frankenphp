package extgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
	}

	headerGen := HeaderGenerator{generator}
	require.NoError(t, headerGen.generate())

	expectedFile := filepath.Join(tmpDir, "test_extension.h")
	require.FileExists(t, expectedFile)

	content, err := readFile(expectedFile)
	require.NoError(t, err)

	testHeaderBasicStructure(t, content, "test_extension")
	testHeaderIncludeGuards(t, content, "TEST_EXTENSION_H")
}

func TestHeaderGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name     string
		baseName string
		contains []string
	}{
		{
			name:     "simple extension",
			baseName: "simple",
			contains: []string{
				"#ifndef _SIMPLE_H",
				"#define _SIMPLE_H",
				"#include <php.h>",
				"extern zend_module_entry simple_module_entry;",
				"#endif",
			},
		},
		{
			name:     "extension with hyphens",
			baseName: "my-extension",
			contains: []string{
				"#ifndef _MY_EXTENSION_H",
				"#define _MY_EXTENSION_H",
				"#endif",
			},
		},
		{
			name:     "extension with underscores",
			baseName: "my_extension_name",
			contains: []string{
				"#ifndef _MY_EXTENSION_NAME_H",
				"#define _MY_EXTENSION_NAME_H",
				"#endif",
			},
		},
		{
			name:     "complex extension name",
			baseName: "complex.name-with_symbols",
			contains: []string{
				"#ifndef _COMPLEX_NAME_WITH_SYMBOLS_H",
				"#define _COMPLEX_NAME_WITH_SYMBOLS_H",
				"#endif",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{BaseName: tt.baseName}
			headerGen := HeaderGenerator{generator}
			content, err := headerGen.buildContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated header content should contain '%s'", expected)
			}
		})
	}
}

func TestHeaderGenerator_HeaderGuardGeneration(t *testing.T) {
	tests := []struct {
		baseName      string
		expectedGuard string
	}{
		{"simple", "_SIMPLE_H"},
		{"my-extension", "_MY_EXTENSION_H"},
		{"complex.name", "_COMPLEX_NAME_H"},
		{"under_score", "_UNDER_SCORE_H"},
		{"MixedCase", "_MIXEDCASE_H"},
		{"123numeric", "_123NUMERIC_H"},
		{"special!@#chars", "_SPECIAL___CHARS_H"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			generator := &Generator{BaseName: tt.baseName}
			headerGen := HeaderGenerator{generator}
			content, err := headerGen.buildContent()
			require.NoError(t, err)

			expectedIfndef := "#ifndef " + tt.expectedGuard
			expectedDefine := "#define " + tt.expectedGuard

			assert.Contains(t, content, expectedIfndef, "Expected #ifndef %s, but not found in content", tt.expectedGuard)
			assert.Contains(t, content, expectedDefine, "Expected #define %s, but not found in content", tt.expectedGuard)
		})
	}
}

func TestHeaderGenerator_BasicStructure(t *testing.T) {
	generator := &Generator{BaseName: "structtest"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	require.NoError(t, err)

	expectedElements := []string{
		"#include <php.h>",
		"extern zend_module_entry structtest_module_entry;",
	}

	for _, element := range expectedElements {
		assert.Contains(t, content, element, "Header should contain: %s", element)
	}
}

func TestHeaderGenerator_CompleteStructure(t *testing.T) {
	generator := &Generator{BaseName: "complete_test"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	require.NoError(t, err)

	lines := strings.Split(content, "\n")

	assert.GreaterOrEqual(t, len(lines), 5, "Header file should have multiple lines")

	var foundIfndef, foundDefine, foundEndif bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#ifndef") && !foundIfndef {
			foundIfndef = true
		} else if strings.HasPrefix(line, "#define") && foundIfndef && !foundDefine {
			foundDefine = true
		} else if line == "#endif" {
			foundEndif = true
		}
	}

	assert.True(t, foundIfndef, "Header should start with #ifndef guard")
	assert.True(t, foundDefine, "Header should have #define after #ifndef")
	assert.True(t, foundEndif, "Header should end with #endif")
}

func TestHeaderGenerator_ErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
	}

	headerGen := HeaderGenerator{generator}
	err := headerGen.generate()
	assert.Error(t, err, "Expected error when writing to invalid directory")
}

func TestHeaderGenerator_EmptyBaseName(t *testing.T) {
	generator := &Generator{BaseName: ""}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	require.NoError(t, err)

	assert.Contains(t, content, "#ifndef __H", "Header with empty basename should have __H guard")
	assert.Contains(t, content, "#define __H", "Header with empty basename should have __H define")
}

func TestHeaderGenerator_ContentValidation(t *testing.T) {
	generator := &Generator{BaseName: "validation_test"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(content, "#ifndef"), "Header should have exactly one #ifndef")
	assert.Equal(t, 1, strings.Count(content, "#define"), "Header should have exactly one #define")
	assert.Equal(t, 1, strings.Count(content, "#endif"), "Header should have exactly one #endif")
	assert.False(t, strings.Contains(content, "{{") || strings.Contains(content, "}}"), "Generated header contains unresolved template syntax")
}

func TestHeaderGenerator_SpecialCharacterHandling(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "NORMAL"},
		{"with-hyphens", "WITH_HYPHENS"},
		{"with.dots", "WITH_DOTS"},
		{"with_underscores", "WITH_UNDERSCORES"},
		{"MixedCASE", "MIXEDCASE"},
		{"123numbers", "123NUMBERS"},
		{"special!@#$%", "SPECIAL_____"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			generator := &Generator{BaseName: tt.input}
			headerGen := HeaderGenerator{generator}
			content, err := headerGen.buildContent()
			require.NoError(t, err)

			expectedGuard := "_" + tt.expected + "_H"
			expectedIfndef := "#ifndef " + expectedGuard
			expectedDefine := "#define " + expectedGuard

			assert.Contains(t, content, expectedIfndef, "Expected #ifndef %s for input %s", expectedGuard, tt.input)
			assert.Contains(t, content, expectedDefine, "Expected #define %s for input %s", expectedGuard, tt.input)
		})
	}
}

func TestHeaderGenerator_TemplateErrorHandling(t *testing.T) {
	generator := &Generator{BaseName: "error_test"}
	headerGen := HeaderGenerator{generator}

	_, err := headerGen.buildContent()
	assert.NoError(t, err, "buildContent() should not fail with valid template")
}

func TestHeaderGenerator_GuardConsistency(t *testing.T) {
	baseName := "test_consistency"
	generator := &Generator{BaseName: baseName}
	headerGen := HeaderGenerator{generator}

	content1, err := headerGen.buildContent()
	require.NoError(t, err, "First buildContent() failed: %v", err)

	content2, err := headerGen.buildContent()
	require.NoError(t, err, "Second buildContent() failed: %v", err)

	assert.Equal(t, content1, content2, "Multiple calls to buildContent() should produce identical results")
}

func TestHeaderGenerator_MinimalContent(t *testing.T) {
	generator := &Generator{BaseName: "minimal"}
	headerGen := HeaderGenerator{generator}
	content, err := headerGen.buildContent()
	require.NoError(t, err)

	essentialElements := []string{
		"#ifndef _MINIMAL_H",
		"#define _MINIMAL_H",
		"#include <php.h>",
		"extern zend_module_entry minimal_module_entry;",
		"#endif",
	}

	for _, element := range essentialElements {
		assert.Contains(t, content, element, "Minimal header should contain: %s", element)
	}
}

func testHeaderBasicStructure(t *testing.T, content, baseName string) {
	headerGuard := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}

		return '_'
	}, baseName)
	headerGuard = strings.ToUpper(headerGuard) + "_H"

	requiredElements := []string{
		"#ifndef _" + headerGuard,
		"#define _" + headerGuard,
		"#include <php.h>",
		"extern zend_module_entry test_extension_module_entry;",
		"#endif",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "Header file should contain: %s", element)
	}
}

func testHeaderIncludeGuards(t *testing.T, content, expectedGuard string) {
	expectedIfndef := "#ifndef _" + expectedGuard
	expectedDefine := "#define _" + expectedGuard

	assert.Contains(t, content, expectedIfndef, "Header should contain: %s", expectedIfndef)
	assert.Contains(t, content, expectedDefine, "Header should contain: %s", expectedDefine)
	assert.Contains(t, content, "#endif", "Header should end with #endif")

	ifndefPos := strings.Index(content, expectedIfndef)
	definePos := strings.Index(content, expectedDefine)

	assert.Less(t, ifndefPos, definePos, "#ifndef should come before #define")

	endifPos := strings.LastIndex(content, "#endif")
	assert.NotEqual(t, -1, endifPos, "Header should end with #endif")
	assert.Greater(t, endifPos, definePos, "#endif should come after #define")
}
