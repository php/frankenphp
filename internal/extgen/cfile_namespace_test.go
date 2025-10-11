package extgen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespacedClassName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		className string
		expected  string
	}{
		{
			name:      "no namespace",
			namespace: "",
			className: "MySuperClass",
			expected:  "MySuperClass",
		},
		{
			name:      "single level namespace",
			namespace: "MyNamespace",
			className: "MySuperClass",
			expected:  "MyNamespace_MySuperClass",
		},
		{
			name:      "multi level namespace",
			namespace: `Go\Extension`,
			className: "MySuperClass",
			expected:  "Go_Extension_MySuperClass",
		},
		{
			name:      "deep namespace",
			namespace: `My\Deep\Nested\Namespace`,
			className: "TestClass",
			expected:  "My_Deep_Nested_Namespace_TestClass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NamespacedName(tt.namespace, tt.className)
			require.Equal(t, tt.expected, result, "expected %q, got %q", tt.expected, result)
		})
	}
}

func TestCFileGenerationWithNamespace(t *testing.T) {
	content := `package main

//export_php:namespace Go\Extension

//export_php:class MySuperClass
type MySuperClass struct{}

//export_php:method MySuperClass test(): string
func (m *MySuperClass) Test() string {
	return "test"
}
`

	tmpfile, err := os.CreateTemp("", "test_cfile_namespace_*.go")
	require.NoError(t, err, "Failed to create temp file")
	defer func() {
		err := os.Remove(tmpfile.Name())
		assert.NoError(t, err, "Failed to remove temp file: %v", err)
	}()

	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err, "Failed to write to temp file")

	err = tmpfile.Close()
	require.NoError(t, err, "Failed to close temp file")

	generator := &Generator{
		BaseName:   "test_extension",
		SourceFile: tmpfile.Name(),
		BuildDir:   t.TempDir(),
		Namespace:  `Go\Extension`,
		Classes: []phpClass{
			{
				Name:     "MySuperClass",
				GoStruct: "MySuperClass",
				Methods: []phpClassMethod{
					{
						Name:       "test",
						PhpName:    "test",
						Signature:  "test(): string",
						ReturnType: "string",
						ClassName:  "MySuperClass",
					},
				},
			},
		},
	}

	cFileGen := cFileGenerator{generator: generator}
	contentResult, err := cFileGen.getTemplateContent()
	require.NoError(t, err, "error generating C file")

	expectedCall := "register_class_Go_Extension_MySuperClass()"
	require.Contains(t, contentResult, expectedCall, "C file should contain the standard function call")

	oldCall := "register_class_MySuperClass()"
	require.NotContains(t, contentResult, oldCall, "C file should not contain old non-namespaced call")
}

func TestCFileGenerationWithoutNamespace(t *testing.T) {
	generator := &Generator{
		BaseName:  "test_extension",
		BuildDir:  t.TempDir(),
		Namespace: "",
		Classes: []phpClass{
			{
				Name:     "MySuperClass",
				GoStruct: "MySuperClass",
			},
		},
	}

	cFileGen := cFileGenerator{generator: generator}
	contentResult, err := cFileGen.getTemplateContent()
	require.NoError(t, err, "error generating C file")

	expectedCall := "register_class_MySuperClass()"
	require.Contains(t, contentResult, expectedCall, "C file should not contain the standard function call")
}
