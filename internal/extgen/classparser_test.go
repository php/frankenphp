package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single class",
			input: `package main

//export_php:class User
type UserStruct struct {
	name string
	Age  int
}`,
			expected: 1,
		},
		{
			name: "multiple classes",
			input: `package main

//export_php:class User
type UserStruct struct {
	name string
	Age  int
}

//export_php:class Product
type ProductStruct struct {
	Title string
	Price float64
}`,
			expected: 2,
		},
		{
			name: "no php classes",
			input: `package main

type RegularStruct struct {
	Data string
}`,
			expected: 0,
		},
		{
			name: "class with nullable fields",
			input: `package main

//export_php:class OptionalData
type OptionalStruct struct {
	Required string
	Optional *string
	Count    *int
}`,
			expected: 1,
		},
		{
			name: "class with methods",
			input: `package main

//export_php:class User
type UserStruct struct {
	name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := classParser{}
			classes, err := parser.parse(fileName)
			require.NoError(t, err)

			assert.Len(t, classes, tt.expected, "parse() got wrong number of classes")

			if tt.name == "single class" && len(classes) > 0 {
				class := classes[0]
				assert.Equal(t, "User", class.Name, "Expected class name 'User'")
				assert.Equal(t, "UserStruct", class.GoStruct, "Expected Go struct 'UserStruct'")
				assert.Len(t, class.Properties, 2, "Expected 2 properties")
			}

			if tt.name == "class with nullable fields" && len(classes) > 0 {
				class := classes[0]
				if len(class.Properties) >= 3 {
					assert.False(t, class.Properties[0].IsNullable, "Required field should not be nullable")
					assert.True(t, class.Properties[1].IsNullable, "Optional field should be nullable")
					assert.True(t, class.Properties[2].IsNullable, "Count field should be nullable")
				}
			}
		})
	}
}

func TestClassMethods(t *testing.T) {
	var input = []byte(`package main

//export_php:class User
type UserStruct struct {
	name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) unsafe.Pointer {
	return nil
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int64) {
	u.Age = int(age)
}

//export_php:method User::getInfo(string $prefix = "User"): string
func GetUserInfo(u UserStruct, prefix *C.zend_string) unsafe.Pointer {
	return nil
}`)

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, input, 0644))

	parser := classParser{}
	classes, err := parser.parse(fileName)
	require.NoError(t, err)

	require.Len(t, classes, 1, "Expected 1 class")

	class := classes[0]
	require.Len(t, class.Methods, 3, "Expected 3 methods")

	getName := class.Methods[0]
	assert.Equal(t, "getName", getName.Name, "Expected method name 'getName'")
	assert.Equal(t, phpString, getName.ReturnType, "Expected return type 'string'")
	assert.Empty(t, getName.Params, "Expected 0 params")
	assert.Equal(t, "User", getName.ClassName, "Expected class name 'User'")

	setAge := class.Methods[1]
	assert.Equal(t, "setAge", setAge.Name, "Expected method name 'setAge'")
	assert.Equal(t, phpVoid, setAge.ReturnType, "Expected return type 'void'")
	require.Len(t, setAge.Params, 1, "Expected 1 param")

	param := setAge.Params[0]
	assert.Equal(t, "age", param.Name, "Expected param name 'age'")
	assert.Equal(t, phpInt, param.PhpType, "Expected param type 'int'")
	assert.False(t, param.IsNullable, "Expected param to not be nullable")
	assert.False(t, param.HasDefault, "Expected param to not have default value")

	getInfo := class.Methods[2]
	assert.Equal(t, "getInfo", getInfo.Name, "Expected method name 'getInfo'")
	assert.Equal(t, phpString, getInfo.ReturnType, "Expected return type 'string'")
	require.Len(t, getInfo.Params, 1, "Expected 1 param")

	param = getInfo.Params[0]
	assert.Equal(t, "prefix", param.Name, "Expected param name 'prefix'")
	assert.Equal(t, phpString, param.PhpType, "Expected param type 'string'")
	assert.True(t, param.HasDefault, "Expected param to have default value")
	assert.Equal(t, "User", param.DefaultValue, "Expected default value 'User'")
}

func TestMethodParameterParsing(t *testing.T) {
	tests := []struct {
		name          string
		paramStr      string
		expectedParam phpParameter
		expectError   bool
	}{
		{
			name:     "simple int parameter",
			paramStr: "int $age",
			expectedParam: phpParameter{
				Name:       "age",
				PhpType:    phpInt,
				IsNullable: false,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "nullable string parameter",
			paramStr: "?string $name",
			expectedParam: phpParameter{
				Name:       "name",
				PhpType:    phpString,
				IsNullable: true,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "parameter with default value",
			paramStr: `string $prefix = "default"`,
			expectedParam: phpParameter{
				Name:         "prefix",
				PhpType:      phpString,
				IsNullable:   false,
				HasDefault:   true,
				DefaultValue: "default",
			},
			expectError: false,
		},
		{
			name:     "nullable parameter with default null",
			paramStr: "?int $count = null",
			expectedParam: phpParameter{
				Name:         "count",
				PhpType:      phpInt,
				IsNullable:   true,
				HasDefault:   true,
				DefaultValue: "null",
			},
			expectError: false,
		},
		{
			name:        "invalid parameter format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := classParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseMethodParameter(tt.paramStr)

			if tt.expectError {
				assert.Error(t, err, "Expected error for parameter '%s', but got none", tt.paramStr)
				return
			}

			require.NoError(t, err, "parseMethodParameter(%s) error", tt.paramStr)

			assert.Equal(t, tt.expectedParam.Name, param.Name, "Expected name '%s'", tt.expectedParam.Name)
			assert.Equal(t, tt.expectedParam.PhpType, param.PhpType, "Expected type '%s'", tt.expectedParam.PhpType)
			assert.Equal(t, tt.expectedParam.IsNullable, param.IsNullable, "Expected isNullable %v", tt.expectedParam.IsNullable)
			assert.Equal(t, tt.expectedParam.HasDefault, param.HasDefault, "Expected hasDefault %v", tt.expectedParam.HasDefault)
			assert.Equal(t, tt.expectedParam.DefaultValue, param.DefaultValue, "Expected defaultValue '%s'", tt.expectedParam.DefaultValue)
		})
	}
}

func TestGoTypeToPHPType(t *testing.T) {
	tests := []struct {
		goType   string
		expected phpType
	}{
		{"string", phpString},
		{"*string", phpString},
		{"int", phpInt},
		{"int64", phpInt},
		{"*int", phpInt},
		{"float64", phpFloat},
		{"*float32", phpFloat},
		{"bool", phpBool},
		{"*bool", phpBool},
		{"[]string", phpArray},
		{"map[string]int", phpArray},
		{"*[]int", phpArray},
		{"any", phpMixed},
		{"CustomType", phpMixed},
	}

	parser := classParser{}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := parser.goTypeToPHPType(tt.goType)
			assert.Equal(t, tt.expected, result, "goTypeToPHPType(%s) = %s, want %s", tt.goType, result, tt.expected)
		})
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []phpType
	}{
		{
			name: "basic types",
			input: `package main

//export_php:class TestClass
type TestStruct struct {
	StringField string
	IntField    int
	FloatField  float64
	BoolField   bool
}`,
			expected: []phpType{phpString, phpInt, phpFloat, phpBool},
		},
		{
			name: "pointer types",
			input: `package main

//export_php:class NullableClass
type NullableStruct struct {
	NullableString *string
	NullableInt    *int
	NullableFloat  *float64
	NullableBool   *bool
}`,
			expected: []phpType{phpString, phpInt, phpFloat, phpBool},
		},
		{
			name: "collection types",
			input: `package main

//export_php:class CollectionClass
type CollectionStruct struct {
	StringSlice []string
	IntMap      map[string]int
	MixedSlice  []any
}`,
			expected: []phpType{phpArray, phpArray, phpArray},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0o644))

			parser := classParser{}
			classes, err := parser.parse(fileName)
			require.NoError(t, err)

			require.Len(t, classes, 1, "Expected 1 class")

			class := classes[0]
			require.Len(t, class.Properties, len(tt.expected), "Expected %d properties", len(tt.expected))

			for i, expectedType := range tt.expected {
				assert.Equal(t, expectedType, class.Properties[i].PhpType, "Property %d: expected type %s", i, expectedType)
			}
		})
	}
}

func TestClassParserUnsupportedTypes(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedClasses int
		expectedMethods int
		hasWarning      bool
	}{
		{
			name: "method with array parameter should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::arrayMethod(array $data): string
func (tc *TestClass) arrayMethod(data any) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method with object parameter should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::objectMethod(object $obj): string
func (tc *TestClass) objectMethod(obj any) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method with mixed parameter should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::mixedMethod(mixed $value): string
func (tc *TestClass) mixedMethod(value any) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method with array return type should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::arrayReturn(string $name): array
func (tc *TestClass) arrayReturn(name *C.zend_string) any {
	return []string{"result"}
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method with object return type should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::objectReturn(string $name): object
func (tc *TestClass) objectReturn(name *C.zend_string) any {
	return map[string]any{"key": "value"}
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "valid scalar types should pass",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::validMethod(string $name, int $count, float $rate, bool $active): string
func validMethod(tc *TestClass, name *C.zend_string, count int64, rate float64, active bool) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 1,
			hasWarning:      false,
		},
		{
			name: "valid void return should pass",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::voidMethod(string $message): void
func voidMethod(tc *TestClass, message *C.zend_string) {
	// Do something
}`,
			expectedClasses: 1,
			expectedMethods: 1,
			hasWarning:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &classParser{}
			classes, err := parser.parse(fileName)
			require.NoError(t, err)

			assert.Len(t, classes, tt.expectedClasses, "parse() got wrong number of classes")
			if len(classes) > 0 {
				assert.Len(t, classes[0].Methods, tt.expectedMethods, "parse() got wrong number of methods")
			}
		})
	}
}

func TestClassParserGoTypeMismatch(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedClasses int
		expectedMethods int
		hasWarning      bool
	}{
		{
			name: "method parameter count mismatch should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::countMismatch(string $name, int $count): string
func (tc *TestClass) countMismatch(name *C.zend_string) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method parameter type mismatch should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::typeMismatch(string $name, int $count): string
func (tc *TestClass) typeMismatch(name *C.zend_string, count string) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "method return type mismatch should be rejected",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::returnMismatch(string $name): int
func (tc *TestClass) returnMismatch(name *C.zend_string) string {
	return ""
}`,
			expectedClasses: 1,
			expectedMethods: 0,
			hasWarning:      true,
		},
		{
			name: "valid matching types should pass",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::validMatch(string $name, int $count): string
func validMatch(tc *TestClass, name *C.zend_string, count int64) unsafe.Pointer {
	return nil
}`,
			expectedClasses: 1,
			expectedMethods: 1,
			hasWarning:      false,
		},
		{
			name: "valid bool types should pass",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::validBool(bool $flag): bool
func validBool(tc *TestClass, flag bool) bool {
	return flag
}`,
			expectedClasses: 1,
			expectedMethods: 1,
			hasWarning:      false,
		},
		{
			name: "valid float types should pass",
			input: `package main

//export_php:class TestClass
type TestClass struct {
	Name string
}

//export_php:method TestClass::validFloat(float $value): float
func validFloat(tc *TestClass, value float64) float64 {
	return value
}`,
			expectedClasses: 1,
			expectedMethods: 1,
			hasWarning:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &classParser{}
			classes, err := parser.parse(fileName)
			require.NoError(t, err)

			assert.Len(t, classes, tt.expectedClasses, "parse() got wrong number of classes")
			if len(classes) > 0 {
				assert.Len(t, classes[0].Methods, tt.expectedMethods, "parse() got wrong number of methods")
			}
		})
	}
}
