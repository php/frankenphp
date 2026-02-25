package extgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStubGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
		Functions: []phpFunction{
			{
				Name:      "greet",
				Signature: "greet(string $name): string",
				Params: []phpParameter{
					{Name: "name", PhpType: phpString},
				},
				ReturnType: phpString,
			},
			{
				Name:      "calculate",
				Signature: "calculate(int $a, int $b): int",
				Params: []phpParameter{
					{Name: "a", PhpType: phpInt},
					{Name: "b", PhpType: phpInt},
				},
				ReturnType: phpInt,
			},
		},
		Classes: []phpClass{
			{
				Name:     "User",
				GoStruct: "UserStruct",
			},
		},
		Constants: []phpConstant{
			{
				Name:    "GLOBAL_CONST",
				Value:   "42",
				PhpType: phpInt,
			},
			{
				Name:      "USER_STATUS_ACTIVE",
				Value:     "1",
				PhpType:   phpInt,
				ClassName: "User",
			},
		},
	}

	stubGen := StubGenerator{generator}
	assert.NoError(t, stubGen.generate(), "generate() failed")

	expectedFile := filepath.Join(tmpDir, "test_extension.stub.php")
	assert.FileExists(t, expectedFile, "Expected stub file was not created: %s", expectedFile)

	content, err := readFile(expectedFile)
	assert.NoError(t, err, "Failed to read generated stub file")

	testStubBasicStructure(t, content)
	testStubFunctions(t, content, generator.Functions)
	testStubClasses(t, content, generator.Classes)
	testStubConstants(t, content, generator.Constants)
}

func TestStubGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name      string
		functions []phpFunction
		classes   []phpClass
		constants []phpConstant
		contains  []string
	}{
		{
			name:      "empty extension",
			functions: []phpFunction{},
			classes:   []phpClass{},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
			},
		},
		{
			name: "functions only",
			functions: []phpFunction{
				{
					Name:      "testFunc",
					Signature: "testFunc(string $param): bool",
				},
			},
			classes:   []phpClass{},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"function testFunc(string $param): bool {}",
			},
		},
		{
			name:      "classes only",
			functions: []phpFunction{},
			classes: []phpClass{
				{
					Name: "TestClass",
				},
			},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"class TestClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name:      "constants only",
			functions: []phpFunction{},
			classes:   []phpClass{},
			constants: []phpConstant{
				{
					Name:    "GLOBAL_CONST",
					Value:   `"test"`,
					PhpType: phpString,
				},
			},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				`const GLOBAL_CONST = "test";`,
			},
		},
		{
			name: "functions and classes",
			functions: []phpFunction{
				{
					Name:      "process",
					Signature: "process(array $data): array",
				},
			},
			classes: []phpClass{
				{
					Name: "Result",
				},
			},
			constants: []phpConstant{},
			contains: []string{
				"function process(array $data): array {}",
				"class Result {",
				"public function __construct() {}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Functions: tt.functions,
				Classes:   tt.classes,
				Constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated stub content should contain '%s'", expected)
			}
		})
	}
}

func TestStubGenerator_FunctionSignatures(t *testing.T) {
	tests := []struct {
		name     string
		function phpFunction
		expected string
	}{
		{
			name: "simple function",
			function: phpFunction{
				Name:      "test",
				Signature: "test(): void",
			},
			expected: "function test(): void {}",
		},
		{
			name: "function with parameters",
			function: phpFunction{
				Name:      "greet",
				Signature: "greet(string $name): string",
			},
			expected: "function greet(string $name): string {}",
		},
		{
			name: "function with nullable return",
			function: phpFunction{
				Name:      "findUser",
				Signature: "findUser(int $id): ?object",
			},
			expected: "function findUser(int $id): ?object {}",
		},
		{
			name: "complex function signature",
			function: phpFunction{
				Name:      "process",
				Signature: "process(array $data, ?string $prefix = null, bool $strict = false): ?array",
			},
			expected: "function process(array $data, ?string $prefix = null, bool $strict = false): ?array {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Functions: []phpFunction{tt.function},
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")
			assert.Contains(t, content, tt.expected, "Generated content should contain function signature: %s", tt.expected)
		})
	}
}

func TestStubGenerator_ClassGeneration(t *testing.T) {
	tests := []struct {
		name     string
		class    phpClass
		contains []string
	}{
		{
			name: "simple class",
			class: phpClass{
				Name: "SimpleClass",
			},
			contains: []string{
				"class SimpleClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name: "class with no properties",
			class: phpClass{
				Name: "EmptyClass",
			},
			contains: []string{
				"class EmptyClass {",
				"public function __construct() {}",
				"}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Classes: []phpClass{tt.class},
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated content should contain '%s'", expected)
			}
		})
	}
}

func TestStubGenerator_MultipleItems(t *testing.T) {
	functions := []phpFunction{
		{
			Name:      "func1",
			Signature: "func1(): void",
		},
		{
			Name:      "func2",
			Signature: "func2(string $param): bool",
		},
		{
			Name:      "func3",
			Signature: "func3(int $a, int $b): int",
		},
	}

	classes := []phpClass{
		{
			Name: "Class1",
		},
		{
			Name: "Class2",
		},
	}

	generator := &Generator{
		Functions: functions,
		Classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	for _, fn := range functions {
		expectedFunc := "function " + fn.Name
		assert.Contains(t, content, expectedFunc, "Should contain function: %s", expectedFunc)
	}

	for _, class := range classes {
		expectedClass := "class " + class.Name
		assert.Contains(t, content, expectedClass, "Should contain class: %s", expectedClass)
	}

	funcPos := strings.Index(content, "function func1")
	classPos := strings.Index(content, "class Class1")

	assert.NotEqual(t, -1, funcPos, "functions should be present")
	assert.NotEqual(t, -1, classPos, "classes should be present")
	assert.Less(t, funcPos, classPos, "functions should appear before classes in the stub file")
}

func TestStubGenerator_ErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
		Functions: []phpFunction{
			{Name: "test", Signature: "test(): void"},
		},
	}

	stubGen := StubGenerator{generator}
	err := stubGen.generate()
	assert.Error(t, err, "Expected error when writing to invalid directory")
}

func TestStubGenerator_EmptyContent(t *testing.T) {
	generator := &Generator{
		Functions: []phpFunction{},
		Classes:   []phpClass{},
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	expectedMinimal := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, expected := range expectedMinimal {
		assert.Contains(t, content, expected, "Even empty content should contain: %s", expected)
	}

	assert.NotContains(t, content, "function ", "Empty stub should not contain function declarations")
	assert.NotContains(t, content, "class ", "Empty stub should not contain class declarations")
}

func TestStubGenerator_PHPSyntaxValidation(t *testing.T) {
	functions := []phpFunction{
		{
			Name:      "complexFunc",
			Signature: "complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object",
		},
	}

	classes := []phpClass{
		{
			Name: "ComplexClass",
		},
	}

	generator := &Generator{
		Functions: functions,
		Classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	syntaxChecks := []struct {
		element string
		reason  string
	}{
		{"<?php", "should start with PHP opening tag"},
		{"{", "should contain opening braces"},
		{"}", "should contain closing braces"},
		{"public", "should use proper visibility"},
		{"function", "should contain function keyword"},
		{"class", "should contain class keyword"},
	}

	for _, check := range syntaxChecks {
		assert.Contains(t, content, check.element, "Generated PHP %s", check.reason)
	}

	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	assert.Equal(t, openBraces, closeBraces, "Unbalanced braces in PHP: %d open, %d close", openBraces, closeBraces)

	assert.Contains(t, content, "function complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object {}", "Complex function signature should be preserved exactly")
}

func TestStubGenerator_ClassConstants(t *testing.T) {
	tests := []struct {
		name      string
		classes   []phpClass
		constants []phpConstant
		contains  []string
	}{
		{
			name: "class with constants",
			classes: []phpClass{
				{Name: "MyClass"},
			},
			constants: []phpConstant{
				{
					Name:      "STATUS_ACTIVE",
					Value:     "1",
					PhpType:   phpInt,
					ClassName: "MyClass",
				},
				{
					Name:      "STATUS_INACTIVE",
					Value:     "0",
					PhpType:   phpInt,
					ClassName: "MyClass",
				},
			},
			contains: []string{
				"class MyClass {",
				"public const STATUS_ACTIVE = 1;",
				"public const STATUS_INACTIVE = 0;",
				"public function __construct() {}",
			},
		},
		{
			name: "class with iota constants",
			classes: []phpClass{
				{Name: "StatusClass"},
			},
			constants: []phpConstant{
				{
					Name:      "FIRST",
					Value:     "0",
					PhpType:   phpInt,
					IsIota:    true,
					ClassName: "StatusClass",
				},
				{
					Name:      "SECOND",
					Value:     "1",
					PhpType:   phpInt,
					IsIota:    true,
					ClassName: "StatusClass",
				},
			},
			contains: []string{
				"class StatusClass {",
				"public const FIRST = UNKNOWN;",
				"public const SECOND = UNKNOWN;",
				"@cvalue FIRST",
				"@cvalue SECOND",
			},
		},
		{
			name: "global and class constants",
			classes: []phpClass{
				{Name: "TestClass"},
			},
			constants: []phpConstant{
				{
					Name:    "GLOBAL_CONST",
					Value:   `"global"`,
					PhpType: phpString,
				},
				{
					Name:      "CLASS_CONST",
					Value:     "42",
					PhpType:   phpInt,
					ClassName: "TestClass",
				},
			},
			contains: []string{
				`const GLOBAL_CONST = "global";`,
				"class TestClass {",
				"public const CLASS_CONST = 42;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				Classes:   tt.classes,
				Constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected)
			}
		})
	}
}

func TestStubGenerator_FileStructure(t *testing.T) {
	generator := &Generator{
		Functions: []phpFunction{
			{Name: "testFunc", Signature: "testFunc(): void"},
		},
		Classes: []phpClass{
			{
				Name: "TestClass",
			},
		},
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	lines := strings.Split(content, "\n")

	assert.GreaterOrEqual(t, len(lines), 3, "Stub file should have multiple lines")
	assert.Equal(t, "<?php", strings.TrimSpace(lines[0]), "First line should be <?php opening tag")

	foundGenerateDirective := false
	for _, line := range lines {
		if strings.Contains(line, "@generate-class-entries") {
			foundGenerateDirective = true
			break
		}
	}

	assert.True(t, foundGenerateDirective, "Should contain @generate-class-entries directive")
	assert.Contains(t, strings.Join(lines, "\n"), "\n\n", "Should have proper spacing between sections")
}

func testStubBasicStructure(t *testing.T, content string) {
	requiredElements := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "Stub file should contain: %s", element)
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		assert.Equal(t, "<?php", strings.TrimSpace(lines[0]), "Stub file should start with <?php")
	}
}

func testStubFunctions(t *testing.T, content string, functions []phpFunction) {
	for _, fn := range functions {
		expectedFunc := "function " + fn.Signature + " {}"
		assert.Contains(t, content, expectedFunc, "Stub should contain function: %s", expectedFunc)
	}
}

func testStubClasses(t *testing.T, content string, classes []phpClass) {
	for _, class := range classes {
		expectedClass := "class " + class.Name + " {"
		assert.Contains(t, content, expectedClass, "Stub should contain class: %s", expectedClass)

		expectedConstructor := "public function __construct() {}"
		assert.Contains(t, content, expectedConstructor, "Class %s should have constructor", class.Name)

		assert.Contains(t, content, "}", "Class %s should be properly closed", class.Name)
	}
}

func testStubConstants(t *testing.T, content string, constants []phpConstant) {
	for _, constant := range constants {
		if constant.ClassName == "" {
			if constant.IsIota {
				expectedConst := "const " + constant.Name + " = UNKNOWN;"
				assert.Contains(t, content, expectedConst, "Stub should contain iota constant: %s", expectedConst)
			} else {
				expectedConst := "const " + constant.Name + " = " + constant.Value + ";"
				assert.Contains(t, content, expectedConst, "Stub should contain constant: %s", expectedConst)
			}

			continue
		}
		if constant.IsIota {
			expectedConst := "public const " + constant.Name + " = UNKNOWN;"
			assert.Contains(t, content, expectedConst, "Stub should contain class iota constant: %s", expectedConst)
		} else {
			expectedConst := "public const " + constant.Name + " = " + constant.Value + ";"
			assert.Contains(t, content, expectedConst, "Stub should contain class constant: %s", expectedConst)
		}
	}
}
