package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentationGenerator_Generate(t *testing.T) {
	tests := []struct {
		name        string
		generator   *Generator
		expectError bool
	}{
		{
			name: "simple extension with functions",
			generator: &Generator{
				BaseName: "testextension",
				BuildDir: "",
				Functions: []phpFunction{
					{
						Name:       "greet",
						ReturnType: phpString,
						Params: []phpParameter{
							{Name: "name", PhpType: phpString},
						},
						Signature: "greet(string $name): string",
					},
				},
				Classes: []phpClass{},
			},
			expectError: false,
		},
		{
			name: "extension with classes",
			generator: &Generator{
				BaseName:  "classextension",
				BuildDir:  "",
				Functions: []phpFunction{},
				Classes: []phpClass{
					{
						Name: "TestClass",
						Properties: []phpClassProperty{
							{Name: "name", PhpType: phpString},
							{Name: "count", PhpType: phpInt, IsNullable: true},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "extension with both functions and classes",
			generator: &Generator{
				BaseName: "fullextension",
				BuildDir: "",
				Functions: []phpFunction{
					{
						Name:             "calculate",
						ReturnType:       phpInt,
						IsReturnNullable: true,
						Params: []phpParameter{
							{Name: "base", PhpType: phpInt},
							{Name: "multiplier", PhpType: phpInt, HasDefault: true, DefaultValue: "2", IsNullable: true},
						},
						Signature: "calculate(int $base, ?int $multiplier = 2): ?int",
					},
				},
				Classes: []phpClass{
					{
						Name: "Calculator",
						Properties: []phpClassProperty{
							{Name: "precision", PhpType: phpInt},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty extension",
			generator: &Generator{
				BaseName:  "emptyextension",
				BuildDir:  "",
				Functions: []phpFunction{},
				Classes:   []phpClass{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tt.generator.BuildDir = tempDir

			docGen := &DocumentationGenerator{
				generator: tt.generator,
			}

			err := docGen.generate()

			if tt.expectError {
				assert.Error(t, err, "generate() expected error but got none")
				return
			}

			assert.NoError(t, err, "generate() unexpected error")

			readmePath := filepath.Join(tempDir, "README.md")
			require.FileExists(t, readmePath)

			content, err := os.ReadFile(readmePath)
			require.NoError(t, err, "Failed to read generated README.md")

			contentStr := string(content)

			assert.Contains(t, contentStr, "# "+tt.generator.BaseName+" Extension", "README should contain extension title")
			assert.Contains(t, contentStr, "Auto-generated PHP extension from Go code.", "README should contain description")

			if len(tt.generator.Functions) > 0 {
				assert.Contains(t, contentStr, "## Functions", "README should contain functions section when functions exist")

				for _, fn := range tt.generator.Functions {
					assert.Contains(t, contentStr, "### "+fn.Name, "README should contain function %s", fn.Name)
					assert.Contains(t, contentStr, fn.Signature, "README should contain function signature for %s", fn.Name)
				}
			}

			if len(tt.generator.Classes) > 0 {
				assert.Contains(t, contentStr, "## Classes", "README should contain classes section when classes exist")

				for _, class := range tt.generator.Classes {
					assert.Contains(t, contentStr, "### "+class.Name, "README should contain class %s", class.Name)
				}
			}
		})
	}
}

func TestDocumentationGenerator_GenerateMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		generator   *Generator
		contains    []string
		notContains []string
	}{
		{
			name: "function with parameters",
			generator: &Generator{
				BaseName: "testextension",
				Functions: []phpFunction{
					{
						Name:       "processData",
						ReturnType: phpArray,
						Params: []phpParameter{
							{Name: "data", PhpType: phpString},
							{Name: "options", PhpType: phpArray, IsNullable: true},
							{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "10"},
						},
						Signature: "processData(string $data, ?array $options, int $count = 10): array",
					},
				},
				Classes: []phpClass{},
			},
			contains: []string{
				"# testextension Extension",
				"## Functions",
				"### processData",
				"**Parameters:**",
				"- `data` (string)",
				"- `options` (array) (nullable)",
				"- `count` (int) (default: 10)",
				"**Returns:** array",
			},
		},
		{
			name: "nullable return type",
			generator: &Generator{
				BaseName: "nullableext",
				Functions: []phpFunction{
					{
						Name:             "maybeGetValue",
						ReturnType:       phpString,
						IsReturnNullable: true,
						Params:           []phpParameter{},
						Signature:        "maybeGetValue(): ?string",
					},
				},
				Classes: []phpClass{},
			},
			contains: []string{
				"**Returns:** string (nullable)",
			},
		},
		{
			name: "class with properties",
			generator: &Generator{
				BaseName:  "classext",
				Functions: []phpFunction{},
				Classes: []phpClass{
					{
						Name: "DataProcessor",
						Properties: []phpClassProperty{
							{Name: "name", PhpType: phpString},
							{Name: "config", PhpType: phpArray, IsNullable: true},
							{Name: "enabled", PhpType: phpBool},
						},
					},
				},
			},
			contains: []string{
				"## Classes",
				"### DataProcessor",
				"**Properties:**",
				"- `name`: string",
				"- `config`: array (nullable)",
				"- `enabled`: bool",
			},
		},
		{
			name: "extension with no functions or classes",
			generator: &Generator{
				BaseName:  "emptyext",
				Functions: []phpFunction{},
				Classes:   []phpClass{},
			},
			contains: []string{
				"# emptyext Extension",
				"Auto-generated PHP extension from Go code.",
			},
			notContains: []string{
				"## Functions",
				"## Classes",
			},
		},
		{
			name: "function with no parameters",
			generator: &Generator{
				BaseName: "noparamext",
				Functions: []phpFunction{
					{
						Name:       "getCurrentTime",
						ReturnType: phpInt,
						Params:     []phpParameter{},
						Signature:  "getCurrentTime(): int",
					},
				},
				Classes: []phpClass{},
			},
			contains: []string{
				"### getCurrentTime",
				"**Returns:** int",
			},
			notContains: []string{
				"**Parameters:**",
			},
		},
		{
			name: "class with no properties",
			generator: &Generator{
				BaseName:  "nopropsext",
				Functions: []phpFunction{},
				Classes: []phpClass{
					{
						Name:       "EmptyClass",
						Properties: []phpClassProperty{},
					},
				},
			},
			contains: []string{
				"### EmptyClass",
			},
			notContains: []string{
				"**Properties:**",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docGen := &DocumentationGenerator{
				generator: tt.generator,
			}

			result, err := docGen.generateMarkdown()
			if !assert.NoError(t, err, "generateMarkdown() unexpected error") {
				return
			}

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "generateMarkdown() should contain '%s'", expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, result, notExpected, "generateMarkdown() should NOT contain '%s'", notExpected)
			}
		})
	}
}

func TestDocumentationGenerator_Generate_InvalidDirectory(t *testing.T) {
	generator := &Generator{
		BaseName:  "test",
		BuildDir:  "/nonexistent/directory",
		Functions: []phpFunction{},
		Classes:   []phpClass{},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	err := docGen.generate()
	assert.Error(t, err, "generate() expected error for invalid directory but got none")
}

func TestDocumentationGenerator_TemplateError(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		Functions: []phpFunction{
			{
				Name:       "test",
				ReturnType: phpString,
				Signature:  "test(): string",
			},
		},
		Classes: []phpClass{},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	result, err := docGen.generateMarkdown()
	assert.NoError(t, err, "generateMarkdown() unexpected error")
	assert.NotEmpty(t, result, "generateMarkdown() returned empty result")
}

func BenchmarkDocumentationGenerator_GenerateMarkdown(b *testing.B) {
	generator := &Generator{
		BaseName: "benchext",
		Functions: []phpFunction{
			{
				Name:       "function1",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "param1", PhpType: phpString},
					{Name: "param2", PhpType: phpInt, HasDefault: true, DefaultValue: "0"},
				},
				Signature: "function1(string $param1, int $param2 = 0): string",
			},
			{
				Name:             "function2",
				ReturnType:       phpArray,
				IsReturnNullable: true,
				Params: []phpParameter{
					{Name: "data", PhpType: phpArray, IsNullable: true},
				},
				Signature: "function2(?array $data): ?array",
			},
		},
		Classes: []phpClass{
			{
				Name: "TestClass",
				Properties: []phpClassProperty{
					{Name: "prop1", PhpType: phpString},
					{Name: "prop2", PhpType: phpInt, IsNullable: true},
				},
			},
		},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	for b.Loop() {
		_, err := docGen.generateMarkdown()
		assert.NoError(b, err)
	}
}
