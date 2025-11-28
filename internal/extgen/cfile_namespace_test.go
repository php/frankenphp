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

func TestCFileGenerationWithNamespacedConstants(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		constants []phpConstant
		contains  []string
	}{
		{
			name:      "integer constant with namespace",
			namespace: `Go\Extension`,
			constants: []phpConstant{
				{Name: "TEST_INT", Value: "42", PhpType: phpInt},
			},
			contains: []string{
				`REGISTER_NS_LONG_CONSTANT("Go\\Extension", "TEST_INT", 42, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "string constant with namespace",
			namespace: `Go\Extension`,
			constants: []phpConstant{
				{Name: "TEST_STRING", Value: `"hello"`, PhpType: phpString},
			},
			contains: []string{
				`REGISTER_NS_STRING_CONSTANT("Go\\Extension", "TEST_STRING", "hello", CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "float constant with namespace",
			namespace: `Go\Extension`,
			constants: []phpConstant{
				{Name: "TEST_FLOAT", Value: "3.14", PhpType: phpFloat},
			},
			contains: []string{
				`REGISTER_NS_DOUBLE_CONSTANT("Go\\Extension", "TEST_FLOAT", 3.14, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "bool constant with namespace",
			namespace: `Go\Extension`,
			constants: []phpConstant{
				{Name: "TEST_BOOL", Value: "true", PhpType: phpBool},
			},
			contains: []string{
				`REGISTER_NS_LONG_CONSTANT("Go\\Extension", "TEST_BOOL", ZEND_TRUE, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "iota constant with namespace",
			namespace: `Go\Extension`,
			constants: []phpConstant{
				{Name: "STATUS_OK", Value: "STATUS_OK", PhpType: phpInt, IsIota: true},
			},
			contains: []string{
				`REGISTER_NS_LONG_CONSTANT("Go\\Extension", "STATUS_OK", STATUS_OK, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "multiple constants with deep namespace",
			namespace: `My\Deep\Namespace`,
			constants: []phpConstant{
				{Name: "CONST_INT", Value: "100", PhpType: phpInt},
				{Name: "CONST_STR", Value: `"value"`, PhpType: phpString},
				{Name: "CONST_FLOAT", Value: "1.5", PhpType: phpFloat},
			},
			contains: []string{
				`REGISTER_NS_LONG_CONSTANT("My\\Deep\\Namespace", "CONST_INT", 100, CONST_CS | CONST_PERSISTENT);`,
				`REGISTER_NS_STRING_CONSTANT("My\\Deep\\Namespace", "CONST_STR", "value", CONST_CS | CONST_PERSISTENT);`,
				`REGISTER_NS_DOUBLE_CONSTANT("My\\Deep\\Namespace", "CONST_FLOAT", 1.5, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "single level namespace",
			namespace: `TestNamespace`,
			constants: []phpConstant{
				{Name: "MY_CONST", Value: "999", PhpType: phpInt},
			},
			contains: []string{
				`REGISTER_NS_LONG_CONSTANT("TestNamespace", "MY_CONST", 999, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "namespace with trailing backslash",
			namespace: `TestIntegration\Extension`,
			constants: []phpConstant{
				{Name: "VERSION", Value: `"1.0.0"`, PhpType: phpString},
			},
			contains: []string{
				`REGISTER_NS_STRING_CONSTANT("TestIntegration\\Extension", "VERSION", "1.0.0", CONST_CS | CONST_PERSISTENT);`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  "test_ext",
				Namespace: tt.namespace,
				Constants: tt.constants,
			}

			cGen := cFileGenerator{generator: generator}
			content, err := cGen.buildContent()
			require.NoError(t, err, "Failed to build C file content")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated C content should contain '%s'", expected)
			}
		})
	}
}

func TestCFileGenerationWithoutNamespacedConstants(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		constants []phpConstant
		contains  []string
	}{
		{
			name:      "integer constant without namespace",
			namespace: "",
			constants: []phpConstant{
				{Name: "GLOBAL_INT", Value: "42", PhpType: phpInt},
			},
			contains: []string{
				`REGISTER_LONG_CONSTANT("GLOBAL_INT", 42, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "string constant without namespace",
			namespace: "",
			constants: []phpConstant{
				{Name: "GLOBAL_STRING", Value: `"test"`, PhpType: phpString},
			},
			contains: []string{
				`REGISTER_STRING_CONSTANT("GLOBAL_STRING", "test", CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "float constant without namespace",
			namespace: "",
			constants: []phpConstant{
				{Name: "GLOBAL_FLOAT", Value: "2.71", PhpType: phpFloat},
			},
			contains: []string{
				`REGISTER_DOUBLE_CONSTANT("GLOBAL_FLOAT", 2.71, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "bool constant without namespace",
			namespace: "",
			constants: []phpConstant{
				{Name: "GLOBAL_BOOL", Value: "false", PhpType: phpBool},
			},
			contains: []string{
				`REGISTER_LONG_CONSTANT("GLOBAL_BOOL", ZEND_FALSE, CONST_CS | CONST_PERSISTENT);`,
			},
		},
		{
			name:      "iota constant without namespace",
			namespace: "",
			constants: []phpConstant{
				{Name: "ERROR_CODE", Value: "ERROR_CODE", PhpType: phpInt, IsIota: true},
			},
			contains: []string{
				`REGISTER_LONG_CONSTANT("ERROR_CODE", ERROR_CODE, CONST_CS | CONST_PERSISTENT);`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  "test_ext",
				Namespace: tt.namespace,
				Constants: tt.constants,
			}

			cGen := cFileGenerator{generator: generator}
			content, err := cGen.buildContent()
			require.NoError(t, err, "Failed to build C file content")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated C content should contain '%s'", expected)
			}

			assert.NotContains(t, content, "REGISTER_NS_", "Content should NOT contain namespaced constant macros when namespace is empty")
		})
	}
}

func TestCFileTemplateFunctionMapCString(t *testing.T) {
	generator := &Generator{
		BaseName:  "test_ext",
		Namespace: `My\Namespace\Test`,
		Constants: []phpConstant{
			{Name: "MY_CONST", Value: "123", PhpType: phpInt},
		},
	}

	cGen := cFileGenerator{generator: generator}
	content, err := cGen.getTemplateContent()
	require.NoError(t, err, "Failed to get template content")

	assert.Contains(t, content, `"My\\Namespace\\Test"`, "Template should properly escape namespace backslashes using cString filter")
	assert.NotContains(t, content, `"My\Namespace\Test"`, "Template should not contain unescaped namespace (single backslashes)")
}
