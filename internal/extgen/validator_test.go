package extgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFunction(t *testing.T) {
	tests := []struct {
		name        string
		function    phpFunction
		expectError bool
	}{
		{
			name: "valid function",
			function: phpFunction{
				Name:       "validFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "param1", PhpType: phpString},
					{Name: "param2", PhpType: phpInt},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with nullable return",
			function: phpFunction{
				Name:             "nullableReturn",
				ReturnType:       phpString,
				IsReturnNullable: true,
				Params: []phpParameter{
					{Name: "data", PhpType: phpArray},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with array parameter",
			function: phpFunction{
				Name:       "arrayFunction",
				ReturnType: phpArray,
				Params: []phpParameter{
					{Name: "items", PhpType: phpArray},
					{Name: "filter", PhpType: phpString},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with nullable array parameter",
			function: phpFunction{
				Name:       "nullableArrayFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "items", PhpType: phpArray, IsNullable: true},
					{Name: "name", PhpType: phpString},
				},
			},
			expectError: false,
		},
		{
			name: "empty function name",
			function: phpFunction{
				Name:       "",
				ReturnType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid function name - starts with number",
			function: phpFunction{
				Name:       "123invalid",
				ReturnType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid function name - contains special chars",
			function: phpFunction{
				Name:       "invalid-name",
				ReturnType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			function: phpFunction{
				Name:       "validName",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "123invalid", PhpType: phpString},
				},
			},
			expectError: true,
		},
		{
			name: "empty parameter name",
			function: phpFunction{
				Name:       "validName",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "", PhpType: phpString},
				},
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateFunction(tt.function)

			if tt.expectError {
				assert.Error(t, err, "validateFunction() should return an error for function %s", tt.function.Name)
			} else {
				assert.NoError(t, err, "validateFunction() should not return an error for function %s", tt.function.Name)
			}
		})
	}
}

func TestValidateReturnType(t *testing.T) {
	tests := []struct {
		name        string
		returnType  string
		expectError bool
	}{
		{
			name:        "valid string type",
			returnType:  "string",
			expectError: false,
		},
		{
			name:        "valid int type",
			returnType:  "int",
			expectError: false,
		},
		{
			name:        "valid array type",
			returnType:  "array",
			expectError: false,
		},
		{
			name:        "valid bool type",
			returnType:  "bool",
			expectError: false,
		},
		{
			name:        "valid float type",
			returnType:  "float",
			expectError: false,
		},
		{
			name:        "valid void type",
			returnType:  "void",
			expectError: false,
		},
		{
			name:        "invalid return type",
			returnType:  "invalidType",
			expectError: true,
		},
		{
			name:        "empty return type",
			returnType:  "",
			expectError: true,
		},
		{
			name:        "case sensitive - String should be invalid",
			returnType:  "String",
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateReturnType(phpType(tt.returnType))

			if tt.expectError {
				assert.Error(t, err, "validateReturnType(%s) should return an error", tt.returnType)
			} else {
				assert.NoError(t, err, "validateReturnType(%s) should not return an error", tt.returnType)
			}
		})
	}
}

func TestValidateClassProperty(t *testing.T) {
	tests := []struct {
		name        string
		prop        phpClassProperty
		expectError bool
	}{
		{
			name: "valid property",
			prop: phpClassProperty{
				Name:    "validProperty",
				PhpType: phpString,
				GoType:  "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable property",
			prop: phpClassProperty{
				Name:       "nullableProperty",
				PhpType:    phpInt,
				GoType:     "*int",
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "empty property name",
			prop: phpClassProperty{
				Name:    "",
				PhpType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid property name",
			prop: phpClassProperty{
				Name:    "123invalid",
				PhpType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid property type",
			prop: phpClassProperty{
				Name:    "validName",
				PhpType: phpType("invalidType"),
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateClassProperty(tt.prop)

			if tt.expectError {
				assert.Error(t, err, "validateClassProperty() should return an error")
			} else {
				assert.NoError(t, err, "validateClassProperty() should not return an error")
			}
		})
	}
}

func TestValidateParameter(t *testing.T) {
	tests := []struct {
		name        string
		param       phpParameter
		expectError bool
	}{
		{
			name: "valid string parameter",
			param: phpParameter{
				Name:    "validParam",
				PhpType: phpString,
			},
			expectError: false,
		},
		{
			name: "valid nullable parameter",
			param: phpParameter{
				Name:       "nullableParam",
				PhpType:    phpInt,
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "valid parameter with default",
			param: phpParameter{
				Name:         "defaultParam",
				PhpType:      phpString,
				HasDefault:   true,
				DefaultValue: "hello",
			},
			expectError: false,
		},
		{
			name: "valid array parameter",
			param: phpParameter{
				Name:    "arrayParam",
				PhpType: phpArray,
			},
			expectError: false,
		},
		{
			name: "valid nullable array parameter",
			param: phpParameter{
				Name:       "nullableArrayParam",
				PhpType:    phpArray,
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "empty parameter name",
			param: phpParameter{
				Name:    "",
				PhpType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			param: phpParameter{
				Name:    "123invalid",
				PhpType: phpString,
			},
			expectError: true,
		},
		{
			name: "invalid parameter type",
			param: phpParameter{
				Name:    "validName",
				PhpType: phpType("invalidType"),
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateParameter(tt.param)

			if tt.expectError {
				assert.Error(t, err, "validateParameter() should return an error")
			} else {
				assert.NoError(t, err, "validateParameter() should not return an error")
			}
		})
	}
}

func TestValidateClass(t *testing.T) {
	tests := []struct {
		name        string
		class       phpClass
		expectError bool
	}{
		{
			name: "valid class",
			class: phpClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []phpClassProperty{
					{Name: "name", PhpType: phpString},
					{Name: "age", PhpType: phpInt},
				},
			},
			expectError: false,
		},
		{
			name: "valid class with nullable properties",
			class: phpClass{
				Name:     "NullableClass",
				GoStruct: "NullableStruct",
				Properties: []phpClassProperty{
					{Name: "required", PhpType: phpString, IsNullable: false},
					{Name: "optional", PhpType: phpString, IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "empty class name",
			class: phpClass{
				Name:     "",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid class name",
			class: phpClass{
				Name:     "123InvalidClass",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid property",
			class: phpClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []phpClassProperty{
					{Name: "123invalid", PhpType: phpString},
				},
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateClass(tt.class)

			if tt.expectError {
				assert.Error(t, err, "validateClass() should return an error")
			} else {
				assert.NoError(t, err, "validateClass() should not return an error")
			}
		})
	}
}

func TestValidateTypes(t *testing.T) {
	tests := []struct {
		name        string
		function    phpFunction
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid scalar parameters only",
			function: phpFunction{
				Name:       "validFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "stringParam", PhpType: phpString},
					{Name: "intParam", PhpType: phpInt},
					{Name: "floatParam", PhpType: phpFloat},
					{Name: "boolParam", PhpType: phpBool},
				},
			},
			expectError: false,
		},
		{
			name: "valid nullable scalar parameters",
			function: phpFunction{
				Name:       "nullableFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "stringParam", PhpType: phpString, IsNullable: true},
					{Name: "intParam", PhpType: phpInt, IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "valid void return type",
			function: phpFunction{
				Name:       "voidFunction",
				ReturnType: phpVoid,
				Params: []phpParameter{
					{Name: "stringParam", PhpType: phpString},
				},
			},
			expectError: false,
		},
		{
			name: "valid array parameter and return",
			function: phpFunction{
				Name:       "arrayFunction",
				ReturnType: phpArray,
				Params: []phpParameter{
					{Name: "arrayParam", PhpType: phpArray},
					{Name: "stringParam", PhpType: phpString},
				},
			},
			expectError: false,
		},
		{
			name: "valid nullable array parameter",
			function: phpFunction{
				Name:       "nullableArrayFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "arrayParam", PhpType: phpArray, IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "invalid object parameter",
			function: phpFunction{
				Name:       "objectFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "objectParam", PhpType: phpObject},
				},
			},
			expectError: true,
			errorMsg:    `parameter 1 "objectParam" has unsupported type "object"`,
		},
		{
			name: "invalid object return type",
			function: phpFunction{
				Name:       "objectReturnFunction",
				ReturnType: phpObject,
				Params: []phpParameter{
					{Name: "stringParam", PhpType: phpString},
				},
			},
			expectError: true,
			errorMsg:    `return type "object" is not supported`,
		},
		{
			name: "mixed scalar and invalid parameters",
			function: phpFunction{
				Name:       "mixedFunction",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "validParam", PhpType: phpString},
					{Name: "invalidParam", PhpType: phpObject},
					{Name: "anotherValidParam", PhpType: phpInt},
				},
			},
			expectError: true,
			errorMsg:    `parameter 2 "invalidParam" has unsupported type "object"`,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTypes(tt.function)

			if tt.expectError {
				assert.Error(t, err, "validateTypes() should return an error for function %s", tt.function.Name)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "validateTypes() should not return an error for function %s", tt.function.Name)
			}
		})
	}
}

func TestValidateGoFunctionSignature(t *testing.T) {
	tests := []struct {
		name        string
		phpFunc     phpFunction
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid Go function signature",
			phpFunc: phpFunction{
				Name:       "testFunc",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "name", PhpType: phpString},
					{Name: "count", PhpType: phpInt},
				},
				GoFunction: `func testFunc(name *C.zend_string, count int64) unsafe.Pointer {
	return nil
}`,
			},
			expectError: false,
		},
		{
			name: "valid void return type",
			phpFunc: phpFunction{
				Name:       "voidFunc",
				ReturnType: phpVoid,
				Params: []phpParameter{
					{Name: "message", PhpType: phpString},
				},
				GoFunction: `func voidFunc(message *C.zend_string) {
	// Do something
}`,
			},
			expectError: false,
		},
		{
			name: "no Go function provided",
			phpFunc: phpFunction{
				Name:       "noGoFunc",
				ReturnType: phpString,
				Params:     []phpParameter{},
				GoFunction: "",
			},
			expectError: true,
			errorMsg:    "no Go function found",
		},
		{
			name: "parameter count mismatch",
			phpFunc: phpFunction{
				Name:       "countMismatch",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "param1", PhpType: phpString},
					{Name: "param2", PhpType: phpInt},
				},
				GoFunction: `func countMismatch(param1 *C.zend_string) unsafe.Pointer {
	return nil
}`,
			},
			expectError: true,
			errorMsg:    "parameter count mismatch: PHP function has 2 parameters but Go function has 1",
		},
		{
			name: "parameter type mismatch",
			phpFunc: phpFunction{
				Name:       "typeMismatch",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "name", PhpType: phpString},
					{Name: "count", PhpType: phpInt},
				},
				GoFunction: `func typeMismatch(name *C.zend_string, count string) unsafe.Pointer {
	return nil
}`,
			},
			expectError: true,
			errorMsg:    `parameter 2 type mismatch: PHP "int" requires Go type "int64" but found "string"`,
		},
		{
			name: "return type mismatch",
			phpFunc: phpFunction{
				Name:       "returnMismatch",
				ReturnType: phpInt,
				Params: []phpParameter{
					{Name: "value", PhpType: phpString},
				},
				GoFunction: `func returnMismatch(value *C.zend_string) string {
	return ""
}`,
			},
			expectError: true,
			errorMsg:    `return type mismatch: PHP "int" requires Go return type "int64" but found "string"`,
		},
		{
			name: "valid bool parameter and return",
			phpFunc: phpFunction{
				Name:       "boolFunc",
				ReturnType: phpBool,
				Params: []phpParameter{
					{Name: "flag", PhpType: phpBool},
				},
				GoFunction: `func boolFunc(flag bool) bool {
	return flag
}`,
			},
			expectError: false,
		},
		{
			name: "valid float parameter and return",
			phpFunc: phpFunction{
				Name:       "floatFunc",
				ReturnType: phpFloat,
				Params: []phpParameter{
					{Name: "value", PhpType: phpFloat},
				},
				GoFunction: `func floatFunc(value float64) float64 {
	return value * 2.0
}`,
			},
			expectError: false,
		},
		{
			name: "valid array parameter and return",
			phpFunc: phpFunction{
				Name:       "arrayFunc",
				ReturnType: phpArray,
				Params: []phpParameter{
					{Name: "items", PhpType: phpArray},
				},
				GoFunction: `func arrayFunc(items *C.zend_array) unsafe.Pointer {
	return nil
}`,
			},
			expectError: false,
		},
		{
			name: "valid nullable array parameter",
			phpFunc: phpFunction{
				Name:       "nullableArrayFunc",
				ReturnType: phpString,
				Params: []phpParameter{
					{Name: "items", PhpType: phpArray, IsNullable: true},
					{Name: "name", PhpType: phpString},
				},
				GoFunction: `func nullableArrayFunc(items *C.zend_array, name *C.zend_string) unsafe.Pointer {
	return nil
}`,
			},
			expectError: false,
		},
		{
			name: "mixed array and scalar parameters",
			phpFunc: phpFunction{
				Name:       "mixedFunc",
				ReturnType: phpArray,
				Params: []phpParameter{
					{Name: "data", PhpType: phpArray},
					{Name: "filter", PhpType: phpString},
					{Name: "limit", PhpType: phpInt},
				},
				GoFunction: `func mixedFunc(data *C.zend_array, filter *C.zend_string, limit int64) unsafe.Pointer {
	return nil
}`,
			},
			expectError: false,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGoFunctionSignatureWithOptions(tt.phpFunc, false)

			if tt.expectError {
				assert.Error(t, err, "validateGoFunctionSignature() should return an error for function %s", tt.phpFunc.Name)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "validateGoFunctionSignature() should not return an error for function %s", tt.phpFunc.Name)
			}
		})
	}
}

func TestPhpTypeToGoType(t *testing.T) {
	tests := []struct {
		phpType    string
		isNullable bool
		expected   string
	}{
		{"string", false, "*C.zend_string"},
		{"string", true, "*C.zend_string"},
		{"int", false, "int64"},
		{"int", true, "*int64"},
		{"float", false, "float64"},
		{"float", true, "*float64"},
		{"bool", false, "bool"},
		{"bool", true, "*bool"},
		{"array", false, "*C.zend_array"},
		{"array", true, "*C.zend_array"},
		{"unknown", false, "any"},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.phpType, func(t *testing.T) {
			result := validator.phpTypeToGoType(phpType(tt.phpType), tt.isNullable)
			assert.Equal(t, tt.expected, result, "phpTypeToGoType(%s, %v) should return %s", tt.phpType, tt.isNullable, tt.expected)
		})
	}
}

func TestPhpReturnTypeToGoType(t *testing.T) {
	tests := []struct {
		phpReturnType string
		expected      string
	}{
		{"void", ""},
		{"void", ""},
		{"string", "unsafe.Pointer"},
		{"string", "unsafe.Pointer"},
		{"int", "int64"},
		{"int", "int64"},
		{"float", "float64"},
		{"float", "float64"},
		{"bool", "bool"},
		{"bool", "bool"},
		{"array", "unsafe.Pointer"},
		{"array", "unsafe.Pointer"},
		{"unknown", "any"},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.phpReturnType, func(t *testing.T) {
			result := validator.phpReturnTypeToGoType(phpType(tt.phpReturnType))
			assert.Equal(t, tt.expected, result, "phpReturnTypeToGoType(%s) should return %s", tt.phpReturnType, tt.expected)
		})
	}
}
