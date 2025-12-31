package extgen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPHPFuncGenerator_NamespacedFunctions(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		function  phpFunction
		expected  string
	}{
		{
			name:      "no namespace",
			namespace: "",
			function:  phpFunction{Name: "test_func", ReturnType: "int"},
			expected:  "PHP_FUNCTION(test_func)",
		},
		{
			name:      "single level namespace",
			namespace: "MyNamespace",
			function:  phpFunction{Name: "test_func", ReturnType: "int"},
			expected:  "PHP_FUNCTION(MyNamespace_test_func)",
		},
		{
			name:      "multi level namespace",
			namespace: `Go\Extension`,
			function:  phpFunction{Name: "multiply", ReturnType: "int"},
			expected:  "PHP_FUNCTION(Go_Extension_multiply)",
		},
		{
			name:      "deep namespace",
			namespace: `My\Deep\Nested\Namespace`,
			function:  phpFunction{Name: "is_even", ReturnType: "bool"},
			expected:  "PHP_FUNCTION(My_Deep_Nested_Namespace_is_even)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := PHPFuncGenerator{
				paramParser: &ParameterParser{},
				namespace:   tt.namespace,
			}

			result := generator.generate(tt.function)

			require.Contains(t, result, tt.expected, "Expected to find %q in generated PHP code, but didn't.\nGenerated:\n%s", tt.expected, result)
		})
	}
}

func TestGetNamespacedFunctionName(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		functionName string
		expected     string
	}{
		{
			name:         "no namespace",
			namespace:    "",
			functionName: "test_func",
			expected:     "test_func",
		},
		{
			name:         "single level namespace",
			namespace:    "MyNamespace",
			functionName: "test_func",
			expected:     "MyNamespace_test_func",
		},
		{
			name:         "multi level namespace",
			namespace:    `Go\Extension`,
			functionName: "multiply",
			expected:     "Go_Extension_multiply",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NamespacedName(tt.namespace, tt.functionName)

			require.Equal(t, tt.expected, result, "Expected %q, got %q", tt.expected, result)
		})
	}
}

func TestCFileWithNamespacedPHPFunctions(t *testing.T) {
	generator := &Generator{
		BaseName:  "test_extension",
		Namespace: `Go\Extension`,
		Functions: []phpFunction{
			{
				Name:       "multiply",
				ReturnType: "int",
				Params: []phpParameter{
					{Name: "a", PhpType: "int"},
					{Name: "b", PhpType: "int"},
				},
			},
			{
				Name:       "is_even",
				ReturnType: "bool",
				Params: []phpParameter{
					{Name: "num", PhpType: "int"},
				},
			},
		},
		Classes: []phpClass{
			{
				Name:     "MySuperClass",
				GoStruct: "MySuperClass",
				Methods: []phpClassMethod{
					{
						Name:       "getName",
						PhpName:    "getName",
						ReturnType: "string",
						ClassName:  "MySuperClass",
					},
				},
			},
		},
		BuildDir: t.TempDir(),
	}

	cFileGen := cFileGenerator{generator: generator}
	content, err := cFileGen.buildContent()
	require.NoError(t, err, "error generating C file")

	expectedFunctions := []string{
		"PHP_FUNCTION(Go_Extension_multiply)",
		"PHP_FUNCTION(Go_Extension_is_even)",
	}

	for _, expected := range expectedFunctions {
		require.Contains(t, content, expected, "Expected to find %q in C file content", expected)
	}

	expectedMethods := []string{
		"PHP_METHOD(Go_Extension_MySuperClass, __construct)",
		"PHP_METHOD(Go_Extension_MySuperClass, getName)",
	}

	for _, expected := range expectedMethods {
		require.Contains(t, content, expected, "Expected to find %q in C file content", expected)
	}

	oldDeclarations := []string{
		"PHP_FUNCTION(multiply)",
		"PHP_FUNCTION(is_even)",
		"PHP_METHOD(MySuperClass, __construct)",
		"PHP_METHOD(MySuperClass, getName)",
	}

	for _, old := range oldDeclarations {
		require.NotContains(t, content, old, "Did not expect to find old declaration %q in C file content", old)
	}
}
