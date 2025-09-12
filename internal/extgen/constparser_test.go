package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstantParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single constant",
			input: `package main

//export_php:const
const MyConstant = "test_value"`,
			expected: 1,
		},
		{
			name: "multiple constants",
			input: `package main

//export_php:const
const FirstConstant = "first"

//export_php:const
const SecondConstant = 42

//export_php:const
const ThirdConstant = true`,
			expected: 3,
		},
		{
			name: "iota constant",
			input: `package main

//export_php:const
const IotaConstant = iota`,
			expected: 1,
		},
		{
			name: "mixed constants and iota",
			input: `package main

//export_php:const
const StringConst = "hello"

//export_php:const
const IotaConst = iota

//export_php:const
const IntConst = 123`,
			expected: 3,
		},
		{
			name: "no php constants",
			input: `package main

const RegularConstant = "not exported"

func someFunction() {
	// Just regular code
}`,
			expected: 0,
		},
		{
			name: "constant with complex value",
			input: `package main

//export_php:const
const ComplexConstant = "string with spaces and symbols !@#$%"`,
			expected: 1,
		},
		{
			name: "directive without constant",
			input: `package main

//export_php:const
var notAConstant = "this is a variable"`,
			expected: 0,
		},
		{
			name: "mixed export and non-export constants",
			input: `package main

const RegularConst = "regular"

//export_php:const
const ExportedConst = "exported"

const AnotherRegular = 456

//export_php:const
const AnotherExported = 789`,
			expected: 2,
		},
		{
			name: "numeric constants",
			input: `package main

//export_php:const
const IntConstant = 42

//export_php:const
const FloatConstant = 3.14

//export_php:const
const HexConstant = 0xFF`,
			expected: 3,
		},
		{
			name: "boolean constants",
			input: `package main

//export_php:const
const TrueConstant = true

//export_php:const
const FalseConstant = false`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			constants, err := parser.parse(tmpFile)
			assert.NoError(t, err, "parse() error")

			assert.Len(t, constants, tt.expected, "parse() got wrong number of constants")

			if tt.name == "single constant" && len(constants) > 0 {
				c := constants[0]
				assert.Equal(t, "MyConstant", c.Name, "Expected constant name 'MyConstant'")
				assert.Equal(t, `"test_value"`, c.Value, `Expected constant value '"test_value"'`)
				assert.Equal(t, phpString, c.PhpType, "Expected constant type 'string'")
				assert.False(t, c.IsIota, "Expected isIota to be false for string constant")
			}

			if tt.name == "iota constant" && len(constants) > 0 {
				c := constants[0]
				assert.Equal(t, "IotaConstant", c.Name, "Expected constant name 'IotaConstant'")
				assert.True(t, c.IsIota, "Expected isIota to be true")
				assert.Equal(t, "0", c.Value, "Expected iota constant value to be '0'")
			}

			if tt.name == "multiple constants" && len(constants) == 3 {
				expectedNames := []string{"FirstConstant", "SecondConstant", "ThirdConstant"}
				expectedValues := []string{`"first"`, "42", "true"}
				expectedTypes := []phpType{phpString, phpInt, phpBool}

				for i, c := range constants {
					assert.Equal(t, expectedNames[i], c.Name, "Expected constant name '%s'", expectedNames[i])
					assert.Equal(t, expectedValues[i], c.Value, "Expected constant value '%s'", expectedValues[i])
					assert.Equal(t, expectedTypes[i], c.PhpType, "Expected constant type '%s'", expectedTypes[i])
				}
			}
		})
	}
}

func TestConstantParserErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name: "invalid constant declaration",
			input: `package main

//export_php:const
const = "missing name"`,
			expectError: true,
		},
		{
			name: "malformed constant",
			input: `package main

//export_php:const
const InvalidSyntax`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			_, err := parser.parse(tmpFile)
			require.NotNil(t, err)

			if tt.expectError {
				assert.Error(t, err, "Expected error but got none")

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConstantParserIotaSequence(t *testing.T) {
	input := `package main

//export_php:const
const FirstIota = iota

//export_php:const  
const SecondIota = iota

//export_php:const
const ThirdIota = iota`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 3, "Expected 3 constants")

	expectedValues := []string{"0", "1", "2"}
	for i, c := range constants {
		assert.True(t, c.IsIota, "Expected constant %d to be iota", i)
		assert.Equal(t, expectedValues[i], c.Value, "Expected constant %d value to be '%s'", i, expectedValues[i])
	}
}

func TestConstantParserTypeDetection(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType phpType
	}{
		{"string with double quotes", `"hello world"`, phpString},
		{"string with backticks", "`hello world`", phpString},
		{"boolean true", "true", phpBool},
		{"boolean false", "false", phpBool},
		{"integer", "42", phpInt},
		{"negative integer", "-42", phpInt},
		{"hex integer", "0xFF", phpInt},
		{"octal integer", "0755", phpInt},
		{"go octal integer", "0o755", phpInt},
		{"binary integer", "0b1010", phpInt},
		{"float", "3.14", phpFloat},
		{"negative float", "-3.14", phpFloat},
		{"scientific notation", "1e10", phpFloat},
		{"unknown type", "someFunction()", phpInt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineConstantType(tt.value)
			assert.Equal(t, tt.expectedType, result, "determineConstantType(%s) expected %s", tt.value, tt.expectedType)
		})
	}
}

func TestConstantParserClassConstants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single class constant",
			input: `package main

//export_php:classconst MyClass
const STATUS_ACTIVE = 1`,
			expected: 1,
		},
		{
			name: "multiple class constants",
			input: `package main

//export_php:classconst User
const STATUS_ACTIVE = "active"

//export_php:classconst User
const STATUS_INACTIVE = "inactive"

//export_php:classconst Order
const STATE_PENDING = 0`,
			expected: 3,
		},
		{
			name: "mixed global and class constants",
			input: `package main

//export_php:const
const GLOBAL_CONST = "global"

//export_php:classconst MyClass
const CLASS_CONST = 42

//export_php:const
const ANOTHER_GLOBAL = true`,
			expected: 3,
		},
		{
			name: "class constant with iota",
			input: `package main

//export_php:classconst Status
const FIRST = iota

//export_php:classconst Status
const SECOND = iota`,
			expected: 2,
		},
		{
			name: "invalid class constant directive",
			input: `package main

//export_php:classconst
const INVALID = "missing class name"`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			constants, err := parser.parse(tmpFile)
			assert.NoError(t, err, "parse() error")

			assert.Len(t, constants, tt.expected, "parse() got wrong number of constants")

			if tt.name == "single class constant" && len(constants) > 0 {
				c := constants[0]
				assert.Equal(t, "STATUS_ACTIVE", c.Name, "Expected constant name 'STATUS_ACTIVE'")
				assert.Equal(t, "MyClass", c.ClassName, "Expected class name 'MyClass'")
				assert.Equal(t, "1", c.Value, "Expected constant value '1'")
				assert.Equal(t, phpInt, c.PhpType, "Expected constant type 'int'")
			}

			if tt.name == "multiple class constants" && len(constants) == 3 {
				expectedClasses := []string{"User", "User", "Order"}
				expectedNames := []string{"STATUS_ACTIVE", "STATUS_INACTIVE", "STATE_PENDING"}
				expectedValues := []string{`"active"`, `"inactive"`, "0"}

				for i, c := range constants {
					assert.Equal(t, expectedClasses[i], c.ClassName, "Expected class name '%s'", expectedClasses[i])
					assert.Equal(t, expectedNames[i], c.Name, "Expected constant name '%s'", expectedNames[i])
					assert.Equal(t, expectedValues[i], c.Value, "Expected constant value '%s'", expectedValues[i])
				}
			}

			if tt.name == "mixed global and class constants" && len(constants) == 3 {
				assert.Empty(t, constants[0].ClassName, "First constant should be global")
				assert.Equal(t, "MyClass", constants[1].ClassName, "Second constant should belong to MyClass")
				assert.Empty(t, constants[2].ClassName, "Third constant should be global")
			}
		})
	}
}

func TestConstantParserRegexMatch(t *testing.T) {
	testCases := []struct {
		line     string
		expected bool
	}{
		{"//export_php:const", true},
		{"// export_php:const", true},
		{"//  export_php:const", true},
		{"//export_php:const ", false}, // should not match with trailing content
		{"//export_php", false},
		{"//export_php:function", false},
		{"//export_php:class", false},
		{"// some other comment", false},
	}

	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			matches := constRegex.MatchString(tc.line)
			assert.Equal(t, tc.expected, matches, "Expected regex match for line '%s'", tc.line)
		})
	}
}

func TestConstantParserClassConstRegex(t *testing.T) {
	testCases := []struct {
		line        string
		shouldMatch bool
		className   string
	}{
		{"//export_php:classconst MyClass", true, "MyClass"},
		{"// export_php:classconst User", true, "User"},
		{"//  export_php:classconst  Status", true, "Status"},
		{"//export_php:classconst Order123", true, "Order123"},
		{"//export_php:classconst", false, ""},
		{"//export_php:classconst ", false, ""},
		{"//export_php:classconst MyClass extra", false, ""},
		{"//export_php:const", false, ""},
		{"//export_php:function", false, ""},
		{"// some other comment", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			matches := classConstRegex.FindStringSubmatch(tc.line)

			if tc.shouldMatch {
				assert.Len(t, matches, 2, "Expected 2 matches for line '%s'", tc.line)
				if len(matches) != 2 {
					return
				}
				assert.Equal(t, tc.className, matches[1], "Expected class name '%s'", tc.className)
			} else {
				assert.Empty(t, matches, "Expected no matches for line '%s'", tc.line)
			}
		})
	}
}

func TestConstantParserDeclRegex(t *testing.T) {
	testCases := []struct {
		line        string
		shouldMatch bool
		name        string
		value       string
	}{
		{`const MyConst = "value"`, true, "MyConst", `"value"`},
		{"const IntConst = 42", true, "IntConst", "42"},
		{"const BoolConst = true", true, "BoolConst", "true"},
		{"const IotaConst = iota", true, "IotaConst", "iota"},
		{"const ComplexValue = someFunction()", true, "ComplexValue", "someFunction()"},
		{`const SpacedName = "with spaces"`, true, "SpacedName", `"with spaces"`},
		{`var notAConst = "value"`, false, "", ""},
		{"const", false, "", ""},
		{"const =", false, "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.line, func(t *testing.T) {
			matches := constDeclRegex.FindStringSubmatch(tc.line)

			if tc.shouldMatch {
				assert.Len(t, matches, 3, "Expected 3 matches for line '%s'", tc.line)
				if len(matches) != 3 {
					return
				}
				assert.Equal(t, tc.name, matches[1], "Expected name '%s'", tc.name)
				assert.Equal(t, tc.value, matches[2], "Expected value '%s'", tc.value)
			} else {
				assert.Empty(t, matches, "Expected no matches for line '%s'", tc.line)
			}
		})
	}
}

func TestPHPConstantCValue(t *testing.T) {
	tests := []struct {
		name     string
		constant phpConstant
		expected string
	}{
		{
			name: "octal notation 0o35",
			constant: phpConstant{
				Name:    "OctalConst",
				Value:   "0o35",
				PhpType: phpInt,
			},
			expected: "29", // 0o35 = 29 in decimal
		},
		{
			name: "octal notation 0o755",
			constant: phpConstant{
				Name:    "OctalPerm",
				Value:   "0o755",
				PhpType: phpInt,
			},
			expected: "493", // 0o755 = 493 in decimal
		},
		{
			name: "regular integer",
			constant: phpConstant{
				Name:    "RegularInt",
				Value:   "42",
				PhpType: phpInt,
			},
			expected: "42",
		},
		{
			name: "hex integer",
			constant: phpConstant{
				Name:    "HexInt",
				Value:   "0xFF",
				PhpType: phpInt,
			},
			expected: "0xFF", // hex should remain unchanged
		},
		{
			name: "string constant",
			constant: phpConstant{
				Name:    "StringConst",
				Value:   `"hello"`,
				PhpType: phpString,
			},
			expected: `"hello"`, // strings should remain unchanged
		},
		{
			name: "boolean constant",
			constant: phpConstant{
				Name:    "BoolConst",
				Value:   "true",
				PhpType: phpBool,
			},
			expected: "true", // booleans should remain unchanged
		},
		{
			name: "float constant",
			constant: phpConstant{
				Name:    "FloatConst",
				Value:   "3.14",
				PhpType: phpFloat,
			},
			expected: "3.14", // floats should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.constant.CValue()
			assert.Equal(t, tt.expected, result, "CValue() expected %s", tt.expected)
		})
	}
}
