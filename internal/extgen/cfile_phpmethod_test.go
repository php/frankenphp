package extgen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCFile_NamespacedPHPMethods(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		classes   []phpClass
		expected  []string
	}{
		{
			name:      "no namespace - regular PHP_METHOD",
			namespace: "",
			classes: []phpClass{
				{
					Name:     "TestClass",
					GoStruct: "TestClass",
					Methods: []phpClassMethod{
						{Name: "testMethod", PhpName: "testMethod", ClassName: "TestClass"},
					},
				},
			},
			expected: []string{
				"PHP_METHOD(TestClass, __construct)",
				"PHP_METHOD(TestClass, testMethod)",
			},
		},
		{
			name:      "single level namespace",
			namespace: "MyNamespace",
			classes: []phpClass{
				{
					Name:     "TestClass",
					GoStruct: "TestClass",
					Methods: []phpClassMethod{
						{Name: "testMethod", PhpName: "testMethod", ClassName: "TestClass"},
					},
				},
			},
			expected: []string{
				"PHP_METHOD(MyNamespace_TestClass, __construct)",
				"PHP_METHOD(MyNamespace_TestClass, testMethod)",
			},
		},
		{
			name:      "multi level namespace",
			namespace: `Go\Extension`,
			classes: []phpClass{
				{
					Name:     "MySuperClass",
					GoStruct: "MySuperClass",
					Methods: []phpClassMethod{
						{Name: "getName", PhpName: "getName", ClassName: "MySuperClass"},
						{Name: "setName", PhpName: "setName", ClassName: "MySuperClass"},
					},
				},
			},
			expected: []string{
				"PHP_METHOD(Go_Extension_MySuperClass, __construct)",
				"PHP_METHOD(Go_Extension_MySuperClass, getName)",
				"PHP_METHOD(Go_Extension_MySuperClass, setName)",
			},
		},
		{
			name:      "multiple classes with namespace",
			namespace: `Go\Extension`,
			classes: []phpClass{
				{
					Name:     "ClassA",
					GoStruct: "ClassA",
					Methods: []phpClassMethod{
						{Name: "methodA", PhpName: "methodA", ClassName: "ClassA"},
					},
				},
				{
					Name:     "ClassB",
					GoStruct: "ClassB",
					Methods: []phpClassMethod{
						{Name: "methodB", PhpName: "methodB", ClassName: "ClassB"},
					},
				},
			},
			expected: []string{
				"PHP_METHOD(Go_Extension_ClassA, __construct)",
				"PHP_METHOD(Go_Extension_ClassA, methodA)",
				"PHP_METHOD(Go_Extension_ClassB, __construct)",
				"PHP_METHOD(Go_Extension_ClassB, methodB)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  "test_extension",
				Namespace: tt.namespace,
				Classes:   tt.classes,
				BuildDir:  t.TempDir(),
			}

			cFileGen := cFileGenerator{generator: generator}
			content, err := cFileGen.getTemplateContent()
			require.NoError(t, err, "error generating C template content: %v", err)

			for _, expected := range tt.expected {
				require.Contains(t, content, expected, "Expected to find %q in C template content", expected)
			}

			if tt.namespace != "" {
				for _, class := range tt.classes {
					oldConstructor := "PHP_METHOD(" + class.Name + ", __construct)"
					require.NotContains(t, content, oldConstructor, "Did not expect to find old constructor declaration %q in namespaced content", oldConstructor)

					for _, method := range class.Methods {
						oldMethod := "PHP_METHOD(" + class.Name + ", " + method.PhpName + ")"
						require.NotContains(t, content, oldMethod, "Did not expect to find old method declaration %q in namespaced content", oldMethod)
					}
				}
			}
		})
	}
}

func TestCFile_PHP_METHOD_Integration(t *testing.T) {
	generator := &Generator{
		BaseName:  "test_extension",
		Namespace: `Go\Extension`,
		Functions: []phpFunction{
			{Name: "testFunc", ReturnType: "void"},
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
					{
						Name:       "setName",
						PhpName:    "setName",
						ReturnType: "void",
						ClassName:  "MySuperClass",
						Params: []phpParameter{
							{Name: "name", PhpType: "string"},
						},
					},
				},
			},
		},
		BuildDir: t.TempDir(),
	}

	cFileGen := cFileGenerator{generator: generator}
	fullContent, err := cFileGen.buildContent()
	require.NoError(t, err, "error generating full C file: %v", err)

	expectedDeclarations := []string{
		"PHP_FUNCTION(Go_Extension_testFunc)",
		"PHP_METHOD(Go_Extension_MySuperClass, __construct)",
		"PHP_METHOD(Go_Extension_MySuperClass, getName)",
		"PHP_METHOD(Go_Extension_MySuperClass, setName)",
	}

	for _, expected := range expectedDeclarations {
		require.Contains(t, fullContent, expected, "Expected to find %q in full C file content", expected)
	}

	oldDeclarations := []string{
		"PHP_FUNCTION(testFunc)",
		"PHP_METHOD(MySuperClass, __construct)",
		"PHP_METHOD(MySuperClass, getName)",
		"PHP_METHOD(MySuperClass, setName)",
	}

	for _, old := range oldDeclarations {
		require.NotContains(t, fullContent, old, "Did not expect to find old declaration %q in full C file content", old)
	}
}

func TestCFile_ClassMethodStringReturn(t *testing.T) {
	generator := &Generator{
		BaseName: "test_extension",
		Classes: []phpClass{
			{
				Name:     "TestClass",
				GoStruct: "TestClass",
				Methods: []phpClassMethod{
					{
						Name:       "getString",
						PhpName:    "getString",
						ReturnType: "string",
						ClassName:  "TestClass",
					},
				},
			},
		},
		BuildDir: t.TempDir(),
	}

	cFileGen := cFileGenerator{generator: generator}
	content, err := cFileGen.getTemplateContent()
	require.NoError(t, err)

	require.Contains(t, content, "if (result)", "Expected NULL check for string return")
	require.Contains(t, content, "RETURN_STR(result)", "Expected RETURN_STR macro")
	require.Contains(t, content, "RETURN_EMPTY_STRING()", "Expected RETURN_EMPTY_STRING fallback")
}

func TestCFile_HeaderIncludesNotCommentedOut(t *testing.T) {
	generator := &Generator{
		BaseName: "test_extension",
		Classes: []phpClass{
			{
				Name:     "Foo",
				GoStruct: "Foo",
				Methods:  []phpClassMethod{{Name: "m", PhpName: "m", ReturnType: phpInt, ClassName: "Foo"}},
			},
		},
		BuildDir: t.TempDir(),
	}

	cFileGen := cFileGenerator{generator: generator}
	content, err := cFileGen.getTemplateContent()
	require.NoError(t, err)

	require.Contains(t, content, "\n#include <php.h>", "#include <php.h> must start on its own line, not be glued to the trailing // comment")
	require.NotContains(t, content, "going forward.#include", "header preamble must not be glued to the first #include")
}

func TestCFile_ClassMethodParamCastsByParamType(t *testing.T) {
	tests := []struct {
		name        string
		returnType  phpType
		params      []phpParameter
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:       "int return with string param does not cast string param",
			returnType: phpInt,
			params:     []phpParameter{{Name: "name", PhpType: phpString}},
			wantContain: []string{
				"getVarInt_wrapper(intern->go_handle, name);",
			},
			wantAbsent: []string{
				"(long)name",
			},
		},
		{
			name:       "float return with string param does not cast string param",
			returnType: phpFloat,
			params:     []phpParameter{{Name: "name", PhpType: phpString}},
			wantContain: []string{
				"getVarInt_wrapper(intern->go_handle, name);",
			},
			wantAbsent: []string{
				"(double)name",
			},
		},
		{
			name:       "bool return with string param does not cast string param",
			returnType: phpBool,
			params:     []phpParameter{{Name: "name", PhpType: phpString}},
			wantContain: []string{
				"getVarInt_wrapper(intern->go_handle, name);",
			},
			wantAbsent: []string{
				"(int)name",
			},
		},
		{
			name:       "string return with int param casts int param to long",
			returnType: phpString,
			params:     []phpParameter{{Name: "count", PhpType: phpInt}},
			wantContain: []string{
				"getVarInt_wrapper(intern->go_handle, (long)count);",
			},
		},
		{
			name:       "int return with mixed params casts each by its own type",
			returnType: phpInt,
			params: []phpParameter{
				{Name: "name", PhpType: phpString},
				{Name: "count", PhpType: phpInt},
				{Name: "ratio", PhpType: phpFloat},
				{Name: "enabled", PhpType: phpBool},
			},
			wantContain: []string{
				"getVarInt_wrapper(intern->go_handle, name, (long)count, (double)ratio, (int)enabled);",
			},
			wantAbsent: []string{
				"(long)name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName: "test_extension",
				Classes: []phpClass{
					{
						Name:     "Scriptling",
						GoStruct: "Scriptling",
						Methods: []phpClassMethod{
							{
								Name:       "getVarInt",
								PhpName:    "getVarInt",
								ReturnType: tt.returnType,
								ClassName:  "Scriptling",
								Params:     tt.params,
							},
						},
					},
				},
				BuildDir: t.TempDir(),
			}

			cFileGen := cFileGenerator{generator: generator}
			content, err := cFileGen.getTemplateContent()
			require.NoError(t, err)

			for _, want := range tt.wantContain {
				require.Contains(t, content, want)
			}
			for _, absent := range tt.wantAbsent {
				require.NotContains(t, content, absent)
			}
		})
	}
}
