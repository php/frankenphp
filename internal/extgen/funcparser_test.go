package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single function",
			input: `package main

//export_php:function testFunc(string $name): string
func testFunc(name *C.zend_string) unsafe.Pointer {
	return String("Hello " + CStringToGoString(name))
}`,
			expected: 1,
		},
		{
			name: "multiple functions",
			input: `package main

//export_php:function func1(int $a): int
func func1(a int64) int64 {
	return a * 2
}

//export_php:function func2(string $b): string  
func func2(b *C.zend_string) unsafe.Pointer {
	return String("processed: " + CStringToGoString(b))
}`,
			expected: 2,
		},
		{
			name: "no php functions",
			input: `package main

func regularFunc() {
	// Just a regular Go function
}`,
			expected: 0,
		},
		{
			name: "mixed functions",
			input: `package main

//export_php:function phpFunc(string $data): string
func phpFunc(data *C.zend_string) unsafe.Pointer {
	return String("PHP: " + CStringToGoString(data))
}

func internalFunc() {
	// Internal function without export_php comment
}

//export_php:function anotherPhpFunc(int $num): int
func anotherPhpFunc(num int64) int64 {
	return num * 10
}`,
			expected: 2,
		},
		{
			name: "wrong args syntax",
			input: `package main

//export_php function phpFunc(data string): string
func phpFunc(data *C.zend_string) unsafe.Pointer {
	return String("PHP: " + CStringToGoString(data))
}`,
			expected: 0,
		},
		{
			name: "decoupled function names",
			input: `package main

//export_php:function my_php_function(string $name): string
func myGoFunction(name *C.zend_string) unsafe.Pointer {
	return String("Hello " + CStringToGoString(name))
}

//export_php:function another_php_func(int $num): int
func someOtherGoName(num int64) int64 {
	return num * 5
}`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &FuncParser{}
			functions, err := parser.parse(fileName)
			require.NoError(t, err)
			assert.Len(t, functions, tt.expected, "parse() got wrong number of functions")

			if tt.name == "single function" && len(functions) > 0 {
				fn := functions[0]
				assert.Equal(t, "testFunc", fn.Name, "Expected function name 'testFunc'")
				assert.Equal(t, phpString, fn.ReturnType, "Expected return type 'string'")
				assert.Len(t, fn.Params, 1, "Expected 1 parameter")
				if len(fn.Params) > 0 {
					assert.Equal(t, "name", fn.Params[0].Name, "Expected parameter name 'name'")
				}
			}

			if tt.name == "decoupled function names" && len(functions) >= 2 {
				fn1 := functions[0]
				assert.Equal(t, "my_php_function", fn1.Name, "Expected PHP function name 'my_php_function'")
				fn2 := functions[1]
				assert.Equal(t, "another_php_func", fn2.Name, "Expected PHP function name 'another_php_func'")
			}
		})
	}
}

func TestSignatureParsing(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		expectError bool
		funcName    string
		paramCount  int
		returnType  phpType
		nullable    bool
	}{
		{
			name:       "simple function",
			signature:  "test(name string): string",
			funcName:   "test",
			paramCount: 1,
			returnType: phpString,
			nullable:   false,
		},
		{
			name:       "nullable return",
			signature:  "test(id int): ?string",
			funcName:   "test",
			paramCount: 1,
			returnType: phpString,
			nullable:   true,
		},
		{
			name:       "multiple params",
			signature:  "calculate(a int, b float, name string): float",
			funcName:   "calculate",
			paramCount: 3,
			returnType: phpFloat,
			nullable:   false,
		},
		{
			name:       "no parameters",
			signature:  "getValue(): int",
			funcName:   "getValue",
			paramCount: 0,
			returnType: phpInt,
			nullable:   false,
		},
		{
			name:       "nullable parameters",
			signature:  "process(?string data, ?int count): bool",
			funcName:   "process",
			paramCount: 2,
			returnType: phpBool,
			nullable:   false,
		},
		{
			name:        "invalid signature",
			signature:   "invalid syntax here",
			expectError: true,
		},
		{
			name:        "missing return type",
			signature:   "test(name string)",
			expectError: true,
		},
	}

	parser := &FuncParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := parser.parseSignature(tt.signature)

			if tt.expectError {
				assert.Error(t, err, "parseSignature() expected error but got none")
				return
			}

			assert.NoError(t, err, "parseSignature() unexpected error")
			assert.Equal(t, tt.funcName, fn.Name, "parseSignature() name mismatch")
			assert.Len(t, fn.Params, tt.paramCount, "parseSignature() param count mismatch")
			assert.Equal(t, tt.returnType, fn.ReturnType, "parseSignature() return type mismatch")
			assert.Equal(t, tt.nullable, fn.IsReturnNullable, "parseSignature() nullable mismatch")

			if tt.name == "nullable parameters" {
				if len(fn.Params) >= 2 {
					assert.True(t, fn.Params[0].IsNullable, "First parameter should be nullable")
					assert.True(t, fn.Params[1].IsNullable, "Second parameter should be nullable")
				}
			}
		})
	}
}

func TestParameterParsing(t *testing.T) {
	tests := []struct {
		name             string
		paramStr         string
		expectedName     string
		expectedType     phpType
		expectedNullable bool
		expectedDefault  string
		hasDefault       bool
		expectError      bool
	}{
		{
			name:         "simple string param",
			paramStr:     "string name",
			expectedName: "name",
			expectedType: phpString,
		},
		{
			name:             "nullable int param",
			paramStr:         "?int count",
			expectedName:     "count",
			expectedType:     phpInt,
			expectedNullable: true,
		},
		{
			name:            "param with default",
			paramStr:        "string message = 'hello'",
			expectedName:    "message",
			expectedType:    phpString,
			expectedDefault: "hello",
			hasDefault:      true,
		},
		{
			name:            "int with default",
			paramStr:        "int limit = 10",
			expectedName:    "limit",
			expectedType:    phpInt,
			expectedDefault: "10",
			hasDefault:      true,
		},
		{
			name:             "nullable with default",
			paramStr:         "?string data = null",
			expectedName:     "data",
			expectedType:     phpString,
			expectedNullable: true,
			expectedDefault:  "null",
			hasDefault:       true,
		},
		{
			name:        "invalid format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := &FuncParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseParameter(tt.paramStr)

			if tt.expectError {
				assert.Error(t, err, "parseParameter() expected error but got none")

				return
			}

			assert.NoError(t, err, "parseParameter() unexpected error")
			assert.Equal(t, tt.expectedName, param.Name, "parseParameter() name mismatch")
			assert.Equal(t, tt.expectedType, param.PhpType, "parseParameter() type mismatch")
			assert.Equal(t, tt.expectedNullable, param.IsNullable, "parseParameter() nullable mismatch")
			assert.Equal(t, tt.hasDefault, param.HasDefault, "parseParameter() hasDefault mismatch")

			if tt.hasDefault {
				assert.Equal(t, tt.expectedDefault, param.DefaultValue, "parseParameter() defaultValue mismatch")
			}
		})
	}
}

func TestFunctionParserUnsupportedTypes(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   int
		hasWarning bool
	}{
		{
			name: "function with array parameter should be rejected",
			input: `package main

//export_php:function arrayFunc(array $data): string
func arrayFunc(data any) unsafe.Pointer {
	return String("processed")
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "function with object parameter should be rejected",
			input: `package main

//export_php:function objectFunc(object $obj): string
func objectFunc(obj any) unsafe.Pointer {
	return String("processed")
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "function with mixed parameter should be rejected",
			input: `package main

//export_php:function mixedFunc(mixed $value): string
func mixedFunc(value any) unsafe.Pointer {
	return String("processed")
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "function with array return type should be rejected",
			input: `package main

//export_php:function arrayReturnFunc(string $name): array
func arrayReturnFunc(name *C.zend_string) any {
	return []string{"result"}
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "function with object return type should be rejected",
			input: `package main

//export_php:function objectReturnFunc(string $name): object
func objectReturnFunc(name *C.zend_string) any {
	return map[string]any{"key": "value"}
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "valid scalar types should pass",
			input: `package main

//export_php:function validFunc(string $name, int $count, float $rate, bool $active): string
func validFunc(name *C.zend_string, count int64, rate float64, active bool) unsafe.Pointer {
	return nil
}`,
			expected:   1,
			hasWarning: false,
		},
		{
			name: "valid void return should pass",
			input: `package main

//export_php:function voidFunc(string $message): void
func voidFunc(message *C.zend_string) {
	// Do something
}`,
			expected:   1,
			hasWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &FuncParser{}
			functions, err := parser.parse(tmpFile)
			require.NoError(t, err)

			assert.Len(t, functions, tt.expected, "parse() got wrong number of functions")
		})
	}
}

func TestFunctionParserGoTypeMismatch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   int
		hasWarning bool
	}{
		{
			name: "parameter count mismatch should be rejected",
			input: `package main

//export_php:function countMismatch(string $name, int $count): string
func countMismatch(name *C.zend_string) unsafe.Pointer {
	return nil
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "parameter type mismatch should be rejected",
			input: `package main

//export_php:function typeMismatch(string $name, int $count): string
func typeMismatch(name *C.zend_string, count string) unsafe.Pointer {
	return nil
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "return type mismatch should be rejected",
			input: `package main

//export_php:function returnMismatch(string $name): int
func returnMismatch(name *C.zend_string) string {
	return ""
}`,
			expected:   0,
			hasWarning: true,
		},
		{
			name: "valid matching types should pass",
			input: `package main

//export_php:function validMatch(string $name, int $count): string
func validMatch(name *C.zend_string, count int64) unsafe.Pointer {
	return nil
}`,
			expected:   1,
			hasWarning: false,
		},
		{
			name: "valid bool types should pass",
			input: `package main

//export_php:function validBool(bool $flag): bool
func validBool(flag bool) bool {
	return flag
}`,
			expected:   1,
			hasWarning: false,
		},
		{
			name: "valid float types should pass",
			input: `package main

//export_php:function validFloat(float $value): float
func validFloat(value float64) float64 {
	return value
}`,
			expected:   1,
			hasWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &FuncParser{}
			functions, err := parser.parse(fileName)
			require.NoError(t, err)

			assert.Len(t, functions, tt.expected, "parse() got wrong number of functions")
		})
	}
}
