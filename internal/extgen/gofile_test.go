package extgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoFileGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import (
	"fmt"
	"strings"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: greet(name string): string
func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}

//export_php: calculate(a int, b int): int
func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}

func internalHelper(data string) string {
	return strings.ToUpper(data)
}

func anotherHelper() {
	fmt.Println("Internal helper")
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []phpFunction{
			{
				Name:       "greet",
				ReturnType: phpString,
				GoFunction: `func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}`,
			},
			{
				Name:       "calculate",
				ReturnType: phpInt,
				GoFunction: `func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}`,
			},
		},
	}

	goGen := GoFileGenerator{generator}
	require.NoError(t, goGen.generate())

	sourceStillExists := filepath.Join(tmpDir, "test.go")
	require.FileExists(t, sourceStillExists)
	sourceStillContent, err := readFile(sourceStillExists)
	require.NoError(t, err)
	assert.Equal(t, sourceContent, sourceStillContent, "Source file should not be modified")

	generatedFile := filepath.Join(tmpDir, "test_generated.go")
	require.FileExists(t, generatedFile)

	generatedContent, err := readFile(generatedFile)
	require.NoError(t, err)

	testGeneratedFileBasicStructure(t, generatedContent, "main", "test")
	testGeneratedFileWrappers(t, generatedContent, generator.Functions)
}

func TestGoFileGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name        string
		baseName    string
		sourceFile  string
		functions   []phpFunction
		classes     []phpClass
		contains    []string
		notContains []string
	}{
		{
			name:     "simple extension",
			baseName: "simple",
			sourceFile: createTempSourceFile(t, `package main

//export_php: test(): void
func test() {
	// simple function
}`),
			functions: []phpFunction{
				{
					Name:       "test",
					ReturnType: phpVoid,
					GoFunction: "func test() {\n\t// simple function\n}",
				},
			},
			contains: []string{
				"package main",
				`#include "simple.h"`,
				`import "C"`,
				"func init()",
				"frankenphp.RegisterExtension(",
				"//export go_test",
				"func go_test()",
				"test()", // wrapper calls original function
			},
		},
		{
			name:     "extension with complex imports",
			baseName: "complex",
			sourceFile: createTempSourceFile(t, `package main

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: process(data string): string
func process(data *go_string) *go_value {
	return types.String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`),
			functions: []phpFunction{
				{
					Name:       "process",
					ReturnType: phpString,
					GoFunction: `func process(data *go_string) *go_value {
	return String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`,
				},
			},
			contains: []string{
				"package main",
				"//export go_process",
				`"C"`,
				"process(", // wrapper calls original function
			},
		},
		{
			name:     "extension with internal functions",
			baseName: "internal",
			sourceFile: createTempSourceFile(t, `package main

//export_php: publicFunc(): void
func publicFunc() {}

func internalFunc1() string {
	return "internal"
}

func internalFunc2(data string) {
	// process data internally
}`),
			functions: []phpFunction{
				{
					Name:       "publicFunc",
					ReturnType: phpVoid,
					GoFunction: "func publicFunc() {}",
				},
			},
			contains: []string{
				"//export go_publicFunc",
				"func go_publicFunc()",
				"publicFunc()", // wrapper calls original function
			},
			notContains: []string{
				"func internalFunc1() string",
				"func internalFunc2(data string)",
			},
		},
		{
			name:     "runtime/cgo blank import without classes",
			baseName: "no_classes",
			sourceFile: createTempSourceFile(t, `package main

//export_php: getValue(): string
func getValue() string {
	return "test"
}`),
			functions: []phpFunction{
				{
					Name:       "getValue",
					ReturnType: phpString,
					GoFunction: `func getValue() string {
	return "test"
}`,
				},
			},
			classes: nil,
			contains: []string{
				`_ "runtime/cgo"`,
				"func init()",
				"frankenphp.RegisterExtension(",
			},
			notContains: []string{
				"cgo.NewHandle",
				"registerGoObject",
				"getGoObject",
				"removeGoObject",
			},
		},
		{
			name:     "runtime/cgo normal import with classes",
			baseName: "with_classes",
			sourceFile: createTempSourceFile(t, `package main

//export_php:class TestClass
type TestStruct struct {
	value string
}

//export_php:method TestClass::getValue(): string
func (ts *TestStruct) GetValue() string {
	return ts.value
}`),
			functions: []phpFunction{},
			classes: []phpClass{
				{
					Name:     "TestClass",
					GoStruct: "TestStruct",
					Methods: []phpClassMethod{
						{
							Name:       "GetValue",
							ReturnType: phpString,
						},
					},
				},
			},
			contains: []string{
				`"runtime/cgo"`,
				"cgo.NewHandle",
				"func registerGoObject",
				"func getGoObject",
			},
			notContains: []string{
				`_ "runtime/cgo"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: tt.sourceFile,
				Functions:  tt.functions,
				Classes:    tt.classes,
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated Go content should contain %q", expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, content, notExpected, "Generated Go content should NOT contain %q", notExpected)
			}
		})
	}
}

func TestGoFileGenerator_PackageNameSanitization(t *testing.T) {
	tests := []struct {
		baseName        string
		expectedPackage string
	}{
		{"simple", "main"},
		{"my-extension", "main"},
		{"ext.with.dots", "main"},
		{"123invalid", "main"},
		{"valid_name", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			sourceFile := createTempSourceFile(t, "package main\n//export_php: test(): void\nfunc test() {}")

			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: sourceFile,
				Functions: []phpFunction{
					{Name: "test", ReturnType: phpVoid, GoFunction: "func test() {}"},
				},
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			require.NoError(t, err)

			expectedPackage := "package " + tt.expectedPackage
			assert.Contains(t, content, expectedPackage, "Generated content should contain '%s'", expectedPackage)
		})
	}
}

func TestGoFileGenerator_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		expectErr  bool
	}{
		{
			name:       "nonexistent file",
			sourceFile: "/nonexistent/file.go",
			expectErr:  true,
		},
		{
			name:       "invalid Go syntax",
			sourceFile: createTempSourceFile(t, "invalid go syntax here"),
			expectErr:  true,
		},
		{
			name:       "valid file",
			sourceFile: createTempSourceFile(t, "package main\nfunc test() {}"),
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:   "test",
				SourceFile: tt.sourceFile,
			}

			goGen := GoFileGenerator{generator}
			_, err := goGen.buildContent()

			if tt.expectErr {
				assert.Error(t, err, "Expected error but got none")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestGoFileGenerator_ComplexScenario(t *testing.T) {
	sourceContent := `package example

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: processData(input string, options array): array
func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return types.Array([]any{processed})
}

//export_php: validateInput(data string): bool
func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return types.Bool(isValid)
}

func internalProcess(data string) string {
	return strings.ToUpper(data)
}

func validateFormat(input string) bool {
	return !strings.Contains(input, "invalid")
}

func jsonHelper(data any) ([]byte, error) {
	return json.Marshal(data)
}

func debugPrint(msg string) {
	fmt.Printf("DEBUG: %s\n", msg)
}`

	sourceFile := createTempSourceFile(t, sourceContent)

	functions := []phpFunction{
		{
			Name:       "processData",
			ReturnType: phpArray,
			GoFunction: `func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return Array([]any{processed})
}`,
		},
		{
			Name:       "validateInput",
			ReturnType: phpBool,
			GoFunction: `func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return Bool(isValid)
}`,
		},
	}

	generator := &Generator{
		BaseName:   "complex-example",
		SourceFile: sourceFile,
		Functions:  functions,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)
	assert.Contains(t, content, "package example", "Package name should match source package")

	for _, fn := range functions {
		exportDirective := "//export go_" + fn.Name
		assert.Contains(t, content, exportDirective, "Generated content should contain export directive: %s", exportDirective)
	}
}

func TestGoFileGenerator_MethodWrapperWithNullableParams(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "fmt"

//export_php:class TestClass
type TestStruct struct {
	name string
}

//export_php:method TestClass::processData(string $name, ?int $count, ?bool $enabled): string
func (ts *TestStruct) ProcessData(name string, count *int64, enabled *bool) string {
	result := fmt.Sprintf("name=%s", name)
	if count != nil {
		result += fmt.Sprintf(", count=%d", *count)
	}
	if enabled != nil {
		result += fmt.Sprintf(", enabled=%t", *enabled)
	}
	return result
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	methods := []phpClassMethod{
		{
			Name:       "ProcessData",
			PhpName:    "processData",
			ClassName:  "TestClass",
			Signature:  "processData(string $name, ?int $count, ?bool $enabled): string",
			ReturnType: phpString,
			Params: []phpParameter{
				{Name: "name", PhpType: phpString, IsNullable: false},
				{Name: "count", PhpType: phpInt, IsNullable: true},
				{Name: "enabled", PhpType: phpBool, IsNullable: true},
			},
			GoFunction: `func (ts *TestStruct) ProcessData(name string, count *int64, enabled *bool) string {
	result := fmt.Sprintf("name=%s", name)
	if count != nil {
		result += fmt.Sprintf(", count=%d", *count)
	}
	if enabled != nil {
		result += fmt.Sprintf(", enabled=%t", *enabled)
	}
	return result
}`,
		},
	}

	classes := []phpClass{
		{
			Name:     "TestClass",
			GoStruct: "TestStruct",
			Methods:  methods,
		},
	}

	generator := &Generator{
		BaseName:   "nullable_test",
		SourceFile: sourceFile,
		Classes:    classes,
		BuildDir:   tmpDir,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedWrapperSignature := "func ProcessData_wrapper(handle C.uintptr_t, name *C.zend_string, count *int64, enabled *bool)"
	assert.Contains(t, content, expectedWrapperSignature, "Generated content should contain wrapper with nullable pointer types: %s", expectedWrapperSignature)

	expectedCall := "structObj.ProcessData(name, count, enabled)"
	assert.Contains(t, content, expectedCall, "Generated content should contain correct method call: %s", expectedCall)

	exportDirective := "//export ProcessData_wrapper"
	assert.Contains(t, content, exportDirective, "Generated content should contain export directive: %s", exportDirective)
}

func TestGoFileGenerator_MethodWrapperWithArrayParams(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "fmt"

//export_php:class ArrayClass
type ArrayStruct struct {
	data []any
}

//export_php:method ArrayClass::processArray(array $items): array
func (as *ArrayStruct) ProcessArray(items frankenphp.AssociativeArray) frankenphp.AssociativeArray {
	result := frankenphp.AssociativeArray{}
	for key, value := range items.Map {
		result.Set("processed_"+key, value)
	}
	return result
}

//export_php:method ArrayClass::filterData(array $data, string $filter): array
func (as *ArrayStruct) FilterData(data frankenphp.AssociativeArray, filter string) frankenphp.AssociativeArray {
	result := frankenphp.AssociativeArray{}
	// Filter logic here
	return result
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	methods := []phpClassMethod{
		{
			Name:       "ProcessArray",
			PhpName:    "processArray",
			ClassName:  "ArrayClass",
			Signature:  "processArray(array $items): array",
			ReturnType: phpArray,
			Params: []phpParameter{
				{Name: "items", PhpType: phpArray, IsNullable: false},
			},
			GoFunction: `func (as *ArrayStruct) ProcessArray(items frankenphp.AssociativeArray) frankenphp.AssociativeArray {
	result := frankenphp.AssociativeArray{}
	for key, value := range items.Entries() {
		result.Set("processed_"+key, value)
	}
	return result
}`,
		},
		{
			Name:       "FilterData",
			PhpName:    "filterData",
			ClassName:  "ArrayClass",
			Signature:  "filterData(array $data, string $filter): array",
			ReturnType: phpArray,
			Params: []phpParameter{
				{Name: "data", PhpType: phpArray, IsNullable: false},
				{Name: "filter", PhpType: phpString, IsNullable: false},
			},
			GoFunction: `func (as *ArrayStruct) FilterData(data frankenphp.AssociativeArray, filter string) frankenphp.AssociativeArray {
	result := frankenphp.AssociativeArray{}
	return result
}`,
		},
	}

	classes := []phpClass{
		{
			Name:     "ArrayClass",
			GoStruct: "ArrayStruct",
			Methods:  methods,
		},
	}

	generator := &Generator{
		BaseName:   "array_test",
		SourceFile: sourceFile,
		Classes:    classes,
		BuildDir:   tmpDir,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedArrayWrapperSignature := "func ProcessArray_wrapper(handle C.uintptr_t, items *C.zval) unsafe.Pointer"
	assert.Contains(t, content, expectedArrayWrapperSignature, "Generated content should contain array wrapper signature: %s", expectedArrayWrapperSignature)

	expectedMixedWrapperSignature := "func FilterData_wrapper(handle C.uintptr_t, data *C.zval, filter *C.zend_string) unsafe.Pointer"
	assert.Contains(t, content, expectedMixedWrapperSignature, "Generated content should contain mixed wrapper signature: %s", expectedMixedWrapperSignature)

	expectedArrayCall := "structObj.ProcessArray(items)"
	assert.Contains(t, content, expectedArrayCall, "Generated content should contain array method call: %s", expectedArrayCall)

	expectedMixedCall := "structObj.FilterData(data, filter)"
	assert.Contains(t, content, expectedMixedCall, "Generated content should contain mixed method call: %s", expectedMixedCall)

	assert.Contains(t, content, "//export ProcessArray_wrapper", "Generated content should contain ProcessArray export directive")
	assert.Contains(t, content, "//export FilterData_wrapper", "Generated content should contain FilterData export directive")
}

func TestGoFileGenerator_Idempotency(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import (
	"fmt"
	"strings"
)

//export_php: greet(name string): string
func greet(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}

func internalHelper(data string) string {
	return strings.ToUpper(data)
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []phpFunction{
			{
				Name:       "greet",
				ReturnType: phpString,
				GoFunction: `func greet(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}`,
			},
		},
	}

	goGen := GoFileGenerator{generator}
	require.NoError(t, goGen.generate(), "First generation should succeed")

	generatedFile := filepath.Join(tmpDir, "test_generated.go")
	require.FileExists(t, generatedFile, "Generated file should exist after first run")

	firstRunContent, err := os.ReadFile(generatedFile)
	require.NoError(t, err)
	firstRunSourceContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	require.NoError(t, goGen.generate(), "Second generation should succeed")

	secondRunContent, err := os.ReadFile(generatedFile)
	require.NoError(t, err)
	secondRunSourceContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	assert.True(t, bytes.Equal(firstRunContent, secondRunContent), "Generated file content should be identical between runs")
	assert.True(t, bytes.Equal(firstRunSourceContent, secondRunSourceContent), "Source file should remain unchanged after both runs")
	assert.Equal(t, sourceContent, string(secondRunSourceContent), "Source file content should match original")
}

func TestGoFileGenerator_HeaderComments(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

//export_php: test(): void
func test() {
	// simple function
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []phpFunction{
			{
				Name:       "test",
				ReturnType: phpVoid,
				GoFunction: "func test() {\n\t// simple function\n}",
			},
		},
	}

	goGen := GoFileGenerator{generator}
	require.NoError(t, goGen.generate())

	generatedFile := filepath.Join(tmpDir, "test_generated.go")
	require.FileExists(t, generatedFile)

	assertContainsHeaderComment(t, generatedFile)
}

func TestExtractGoFunctionName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple function",
			input:    "func test() {}",
			expected: "test",
		},
		{
			name:     "function with params",
			input:    "func calculate(a int, b int) int {}",
			expected: "calculate",
		},
		{
			name:     "function with complex params",
			input:    "func process(data *go_string, opts *go_nullable) *go_value {}",
			expected: "process",
		},
		{
			name:     "function with whitespace",
			input:    "func  spacedName  () {}",
			expected: "spacedName",
		},
		{
			name:     "no func keyword",
			input:    "test() {}",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGoFunctionName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGoFunctionSignatureParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no parameters",
			input:    "func test() {}",
			expected: "",
		},
		{
			name:     "single parameter",
			input:    "func test(name string) {}",
			expected: "name string",
		},
		{
			name:     "multiple parameters",
			input:    "func test(a int, b string, c bool) {}",
			expected: "a int, b string, c bool",
		},
		{
			name:     "pointer parameters",
			input:    "func test(data *go_string) {}",
			expected: "data *go_string",
		},
		{
			name:     "nested parentheses",
			input:    "func test(fn func(int) string) {}",
			expected: "fn func(int) string",
		},
		{
			name:     "variadic parameters",
			input:    "func test(args ...string) {}",
			expected: "args ...string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGoFunctionSignatureParams(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGoFunctionSignatureReturn(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no return type",
			input:    "func test() {}",
			expected: "",
		},
		{
			name:     "single return type",
			input:    "func test() string {}",
			expected: "string",
		},
		{
			name:     "pointer return type",
			input:    "func test() *go_value {}",
			expected: "*go_value",
		},
		{
			name:     "multiple return types",
			input:    "func test() (string, error) {}",
			expected: "(string, error)",
		},
		{
			name:     "named return values",
			input:    "func test() (result string, err error) {}",
			expected: "(result string, err error)",
		},
		{
			name:     "complex return type",
			input:    "func test() unsafe.Pointer {}",
			expected: "unsafe.Pointer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGoFunctionSignatureReturn(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGoFunctionCallParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no parameters",
			input:    "func test() {}",
			expected: "",
		},
		{
			name:     "single parameter",
			input:    "func test(name string) {}",
			expected: "name",
		},
		{
			name:     "multiple parameters",
			input:    "func test(a int, b string, c bool) {}",
			expected: "a, b, c",
		},
		{
			name:     "pointer parameters",
			input:    "func test(data *go_string, opts *go_nullable) {}",
			expected: "data, opts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGoFunctionCallParams(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoFileGenerator_SourceFilePreservation(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "fmt"

//export_php: greet(name string): string
func greet(name *go_string) *go_value {
	return String(fmt.Sprintf("Hello, %s!", CStringToGoString(name)))
}

func internalHelper() {
	fmt.Println("internal")
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	hashBefore := computeFileHash(t, sourceFile)

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []phpFunction{
			{
				Name:       "greet",
				ReturnType: phpString,
				GoFunction: `func greet(name *go_string) *go_value {
	return String(fmt.Sprintf("Hello, %s!", CStringToGoString(name)))
}`,
			},
		},
	}

	goGen := GoFileGenerator{generator}
	require.NoError(t, goGen.generate())

	hashAfter := computeFileHash(t, sourceFile)

	assert.Equal(t, hashBefore, hashAfter, "Source file hash should remain unchanged after generation")

	contentAfter, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, sourceContent, string(contentAfter), "Source file content should be byte-for-byte identical")
}

func TestGoFileGenerator_WrapperParameterForwarding(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "fmt"

//export_php: process(name string, count int): string
func process(name *go_string, count long) *go_value {
	n := CStringToGoString(name)
	return String(fmt.Sprintf("%s: %d", n, count))
}

//export_php: simple(): void
func simple() {}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	functions := []phpFunction{
		{
			Name:       "process",
			ReturnType: phpString,
			GoFunction: `func process(name *go_string, count long) *go_value {
	n := CStringToGoString(name)
	return String(fmt.Sprintf("%s: %d", n, count))
}`,
		},
		{
			Name:       "simple",
			ReturnType: phpVoid,
			GoFunction: "func simple() {}",
		},
	}

	generator := &Generator{
		BaseName:   "wrapper_test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions:  functions,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	assert.Contains(t, content, "//export go_process", "Should have wrapper export directive")
	assert.Contains(t, content, "func go_process(", "Should have wrapper function")
	assert.Contains(t, content, "process(", "Wrapper should call original function")

	assert.Contains(t, content, "//export go_simple", "Should have simple wrapper export directive")
	assert.Contains(t, content, "func go_simple()", "Should have simple wrapper function")
	assert.Contains(t, content, "simple()", "Simple wrapper should call original function")
}

func TestGoFileGenerator_MalformedSource(t *testing.T) {
	tests := []struct {
		name          string
		sourceContent string
		expectError   bool
	}{
		{
			name:          "missing package declaration",
			sourceContent: "func test() {}",
			expectError:   true,
		},
		{
			name:          "syntax error",
			sourceContent: "package main\nfunc test( {}",
			expectError:   true,
		},
		{
			name:          "incomplete function",
			sourceContent: "package main\nfunc test() {",
			expectError:   true,
		},
		{
			name:          "valid minimal source",
			sourceContent: "package main\nfunc test() {}",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			sourceFile := filepath.Join(tmpDir, "test.go")
			require.NoError(t, os.WriteFile(sourceFile, []byte(tt.sourceContent), 0644))

			generator := &Generator{
				BaseName:   "test",
				SourceFile: sourceFile,
				BuildDir:   tmpDir,
			}

			goGen := GoFileGenerator{generator}
			_, err := goGen.buildContent()

			if tt.expectError {
				assert.Error(t, err, "Expected error for malformed source")
			} else {
				assert.NoError(t, err, "Should not error for valid source")
			}

			contentAfter, readErr := os.ReadFile(sourceFile)
			require.NoError(t, readErr)
			assert.Equal(t, tt.sourceContent, string(contentAfter), "Source file should remain unchanged even on error")
		})
	}
}

func TestGoFileGenerator_MethodWrapperWithNullableArrayParams(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

//export_php:class NullableArrayClass
type NullableArrayStruct struct{}

//export_php:method NullableArrayClass::processOptionalArray(?array $items, string $name): string
func (nas *NullableArrayStruct) ProcessOptionalArray(items frankenphp.AssociativeArray, name string) string {
	return fmt.Sprintf("Processing %d items for %s", len(items.Map), name)
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	methods := []phpClassMethod{
		{
			Name:       "ProcessOptionalArray",
			PhpName:    "processOptionalArray",
			ClassName:  "NullableArrayClass",
			Signature:  "processOptionalArray(?array $items, string $name): string",
			ReturnType: phpString,
			Params: []phpParameter{
				{Name: "items", PhpType: phpArray, IsNullable: true},
				{Name: "name", PhpType: phpString, IsNullable: false},
			},
			GoFunction: `func (nas *NullableArrayStruct) ProcessOptionalArray(items frankenphp.AssociativeArray, name string) string {
	return fmt.Sprintf("Processing %d items for %s", len(items.Map), name)
}`,
		},
	}

	classes := []phpClass{
		{
			Name:     "NullableArrayClass",
			GoStruct: "NullableArrayStruct",
			Methods:  methods,
		},
	}

	generator := &Generator{
		BaseName:   "nullable_array_test",
		SourceFile: sourceFile,
		Classes:    classes,
		BuildDir:   tmpDir,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedWrapperSignature := "func ProcessOptionalArray_wrapper(handle C.uintptr_t, items *C.zval, name *C.zend_string) unsafe.Pointer"
	assert.Contains(t, content, expectedWrapperSignature, "Generated content should contain nullable array wrapper signature: %s", expectedWrapperSignature)

	expectedCall := "structObj.ProcessOptionalArray(items, name)"
	assert.Contains(t, content, expectedCall, "Generated content should contain method call: %s", expectedCall)

	assert.Contains(t, content, "//export ProcessOptionalArray_wrapper", "Generated content should contain export directive")
}

func createTempSourceFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.go")

	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

	return tmpFile
}

func TestGoFileGenerator_MethodWrapperWithCallableParams(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "C"

//export_php:class CallableClass
type CallableStruct struct{}

//export_php:method CallableClass::processCallback(callable $callback): string
func (cs *CallableStruct) ProcessCallback(callback *C.zval) string {
	return "processed"
}

//export_php:method CallableClass::processOptionalCallback(?callable $callback): string
func (cs *CallableStruct) ProcessOptionalCallback(callback *C.zval) string {
	return "processed_optional"
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	methods := []phpClassMethod{
		{
			Name:       "ProcessCallback",
			PhpName:    "processCallback",
			ClassName:  "CallableClass",
			Signature:  "processCallback(callable $callback): string",
			ReturnType: phpString,
			Params: []phpParameter{
				{Name: "callback", PhpType: phpCallable, IsNullable: false},
			},
			GoFunction: `func (cs *CallableStruct) ProcessCallback(callback *C.zval) string {
	return "processed"
}`,
		},
		{
			Name:       "ProcessOptionalCallback",
			PhpName:    "processOptionalCallback",
			ClassName:  "CallableClass",
			Signature:  "processOptionalCallback(?callable $callback): string",
			ReturnType: phpString,
			Params: []phpParameter{
				{Name: "callback", PhpType: phpCallable, IsNullable: true},
			},
			GoFunction: `func (cs *CallableStruct) ProcessOptionalCallback(callback *C.zval) string {
	return "processed_optional"
}`,
		},
	}

	classes := []phpClass{
		{
			Name:     "CallableClass",
			GoStruct: "CallableStruct",
			Methods:  methods,
		},
	}

	generator := &Generator{
		BaseName:   "callable_test",
		SourceFile: sourceFile,
		Classes:    classes,
		BuildDir:   tmpDir,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedCallableWrapperSignature := "func ProcessCallback_wrapper(handle C.uintptr_t, callback *C.zval) unsafe.Pointer"
	assert.Contains(t, content, expectedCallableWrapperSignature, "Generated content should contain callable wrapper signature: %s", expectedCallableWrapperSignature)

	expectedOptionalCallableWrapperSignature := "func ProcessOptionalCallback_wrapper(handle C.uintptr_t, callback *C.zval) unsafe.Pointer"
	assert.Contains(t, content, expectedOptionalCallableWrapperSignature, "Generated content should contain optional callable wrapper signature: %s", expectedOptionalCallableWrapperSignature)

	expectedCallableCall := "structObj.ProcessCallback(callback)"
	assert.Contains(t, content, expectedCallableCall, "Generated content should contain callable method call: %s", expectedCallableCall)

	expectedOptionalCallableCall := "structObj.ProcessOptionalCallback(callback)"
	assert.Contains(t, content, expectedOptionalCallableCall, "Generated content should contain optional callable method call: %s", expectedOptionalCallableCall)

	assert.Contains(t, content, "//export ProcessCallback_wrapper", "Generated content should contain ProcessCallback export directive")
	assert.Contains(t, content, "//export ProcessOptionalCallback_wrapper", "Generated content should contain ProcessOptionalCallback export directive")
}

func TestGoFileGenerator_phpTypeToGoType(t *testing.T) {
	generator := &Generator{}
	goGen := GoFileGenerator{generator}

	tests := []struct {
		phpType  phpType
		expected string
	}{
		{phpString, "string"},
		{phpInt, "int64"},
		{phpFloat, "float64"},
		{phpBool, "bool"},
		{phpArray, "*frankenphp.Array"},
		{phpMixed, "any"},
		{phpVoid, ""},
		{phpCallable, "*C.zval"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phpType), func(t *testing.T) {
			result := goGen.phpTypeToGoType(tt.phpType)
			assert.Equal(t, tt.expected, result, "phpTypeToGoType(%s) should return %s", tt.phpType, tt.expected)
		})
	}

	t.Run("unknown_type", func(t *testing.T) {
		unknownType := phpType("unknown")
		result := goGen.phpTypeToGoType(unknownType)
		assert.Equal(t, "any", result, "phpTypeToGoType should fallback to interface{} for unknown types")
	})
}

func testGoFileInternalFunctions(t *testing.T, content string) {
	internalIndicators := []string{
		"func internalHelper",
		"func anotherHelper",
	}

	foundInternal := false
	for _, indicator := range internalIndicators {
		if strings.Contains(content, indicator) {
			foundInternal = true

			break
		}
	}

	if !foundInternal {
		t.Log("No internal functions found (this may be expected)")
	}
}

func testGeneratedFileBasicStructure(t *testing.T, content, expectedPackage, baseName string) {
	requiredElements := []string{
		"package " + expectedPackage,
		"// #include <stdlib.h>",
		`// #include "` + baseName + `.h"`,
		`import "C"`,
		"func init() {",
		"frankenphp.RegisterExtension(",
		"}",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "Generated file should contain: %s", element)
	}

	assert.NotContains(t, content, "func internalHelper", "Generated file should not contain internal functions from source")
	assert.NotContains(t, content, "func anotherHelper", "Generated file should not contain internal functions from source")
}

func testGeneratedFileWrappers(t *testing.T, content string, functions []phpFunction) {
	for _, fn := range functions {
		exportDirective := "//export go_" + fn.Name
		assert.Contains(t, content, exportDirective, "Generated file should contain export directive: %s", exportDirective)

		wrapperFunc := "func go_" + fn.Name + "("
		assert.Contains(t, content, wrapperFunc, "Generated file should contain wrapper function: %s", wrapperFunc)

		funcName := extractGoFunctionName(fn.GoFunction)
		if funcName != "" {
			assert.Contains(t, content, funcName+"(", "Generated wrapper should call original function: %s", funcName)
		}
	}
}

// compareFileContents compares two files byte-by-byte
func compareFileContents(t *testing.T, file1, file2 string) bool {
	content1, err := os.ReadFile(file1)
	require.NoError(t, err)
	content2, err := os.ReadFile(file2)
	require.NoError(t, err)
	return bytes.Equal(content1, content2)
}

// computeFileHash returns SHA256 hash of file
func computeFileHash(t *testing.T, filename string) string {
	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// assertContainsHeaderComment verifies file has autogenerated header
func assertContainsHeaderComment(t *testing.T, filename string) {
	content, err := os.ReadFile(filename)
	require.NoError(t, err)

	headerSection := string(content[:min(len(content), 500)])
	assert.Contains(t, headerSection, "AUTOGENERATED FILE - DO NOT EDIT", "File should contain autogenerated header comment")
	assert.Contains(t, headerSection, "FrankenPHP extension generator", "File should mention FrankenPHP extension generator")
}
