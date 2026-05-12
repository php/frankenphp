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
		name   string
		input  string
		expect int
		assert func(t *testing.T, constants []phpConstant)
	}{
		{
			name: "single constant",
			input: `package main

//export_php:const
const MyConstant = "test_value"`,
			expect: 1,
			assert: func(t *testing.T, cs []phpConstant) {
				c := cs[0]
				assert.Equal(t, "MyConstant", c.Name)
				assert.Equal(t, `"test_value"`, c.Value)
				assert.Equal(t, phpString, c.PhpType)
				assert.False(t, c.IsIota)
			},
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
			expect: 3,
			assert: func(t *testing.T, cs []phpConstant) {
				names := []string{"FirstConstant", "SecondConstant", "ThirdConstant"}
				values := []string{`"first"`, "42", "true"}
				types := []phpType{phpString, phpInt, phpBool}
				for i, c := range cs {
					assert.Equal(t, names[i], c.Name)
					assert.Equal(t, values[i], c.Value)
					assert.Equal(t, types[i], c.PhpType)
				}
			},
		},
		{
			name: "iota constant",
			input: `package main

//export_php:const
const IotaConstant = iota`,
			expect: 1,
			assert: func(t *testing.T, cs []phpConstant) {
				c := cs[0]
				assert.Equal(t, "IotaConstant", c.Name)
				assert.True(t, c.IsIota)
				assert.Equal(t, "0", c.Value)
			},
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
			expect: 3,
		},
		{
			name: "no php constants",
			input: `package main

const RegularConstant = "not exported"

func someFunction() {
	// Just regular code
}`,
			expect: 0,
		},
		{
			name: "constant with complex value",
			input: `package main

//export_php:const
const ComplexConstant = "string with spaces and symbols !@#$%"`,
			expect: 1,
		},
		{
			name: "directive without constant",
			input: `package main

//export_php:const
var notAConstant = "this is a variable"`,
			expect: 0,
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
			expect: 2,
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
			expect: 3,
		},
		{
			name: "boolean constants",
			input: `package main

//export_php:const
const TrueConstant = true

//export_php:const
const FalseConstant = false`,
			expect: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			constants, err := parser.parse(tmpFile)
			require.NoError(t, err)
			require.Len(t, constants, tt.expect)

			if tt.assert != nil {
				tt.assert(t, constants)
			}
		})
	}
}

func TestConstantParserErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "invalid constant declaration",
			input: `package main

//export_php:const
const = "missing name"`,
		},
		{
			name: "malformed constant",
			input: `package main

//export_php:const
const InvalidSyntax`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			_, err := parser.parse(tmpFile)
			assert.Error(t, err, "Expected error but got none")
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

func TestConstantParserConstBlock(t *testing.T) {
	input := `package main

const (
	// export_php:const
	STATUS_PENDING = iota

	// export_php:const
	STATUS_PROCESSING

	// export_php:const
	STATUS_COMPLETED
)`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 3, "Expected 3 constants")

	expectedNames := []string{"STATUS_PENDING", "STATUS_PROCESSING", "STATUS_COMPLETED"}
	expectedValues := []string{"0", "1", "2"}

	for i, c := range constants {
		assert.Equal(t, expectedNames[i], c.Name, "Expected constant %d name to be '%s'", i, expectedNames[i])
		assert.True(t, c.IsIota, "Expected constant %d to be iota", i)
		assert.Equal(t, expectedValues[i], c.Value, "Expected constant %d value to be '%s'", i, expectedValues[i])
		assert.Equal(t, phpInt, c.PhpType, "Expected constant %d to be phpInt type", i)
	}
}

func TestConstantParserConstBlockWithBlockLevelDirective(t *testing.T) {
	input := `package main

// export_php:const
const (
	STATUS_PENDING = iota
	STATUS_PROCESSING
	STATUS_COMPLETED
)`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 3, "Expected 3 constants")

	expectedNames := []string{"STATUS_PENDING", "STATUS_PROCESSING", "STATUS_COMPLETED"}
	expectedValues := []string{"0", "1", "2"}

	for i, c := range constants {
		assert.Equal(t, expectedNames[i], c.Name, "Expected constant %d name to be '%s'", i, expectedNames[i])
		assert.True(t, c.IsIota, "Expected constant %d to be iota", i)
		assert.Equal(t, expectedValues[i], c.Value, "Expected constant %d value to be '%s'", i, expectedValues[i])
		assert.Equal(t, phpInt, c.PhpType, "Expected constant %d to be phpInt type", i)
	}
}

func TestConstantParserMixedConstBlockAndIndividual(t *testing.T) {
	input := `package main

// export_php:const
const INDIVIDUAL = 42

const (
	// export_php:const
	BLOCK_ONE = iota

	// export_php:const
	BLOCK_TWO
)

// export_php:const
const ANOTHER_INDIVIDUAL = "test"`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 4, "Expected 4 constants")

	assert.Equal(t, "INDIVIDUAL", constants[0].Name)
	assert.Equal(t, "42", constants[0].Value)
	assert.Equal(t, phpInt, constants[0].PhpType)

	assert.Equal(t, "BLOCK_ONE", constants[1].Name)
	assert.Equal(t, "0", constants[1].Value)
	assert.True(t, constants[1].IsIota)

	assert.Equal(t, "BLOCK_TWO", constants[2].Name)
	assert.Equal(t, "1", constants[2].Value)
	assert.True(t, constants[2].IsIota)

	assert.Equal(t, "ANOTHER_INDIVIDUAL", constants[3].Name)
	assert.Equal(t, `"test"`, constants[3].Value)
	assert.Equal(t, phpString, constants[3].PhpType)
}

func TestConstantParserIotaRestartsBetweenBlocks(t *testing.T) {
	input := `package main

// export_php:const
const (
	A = iota
	B
	C
)

// export_php:const
const (
	X = iota
	Y
)`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err)

	require.Len(t, constants, 5)
	assert.Equal(t, "0", constants[0].Value, "A should be 0")
	assert.Equal(t, "1", constants[1].Value, "B should be 1")
	assert.Equal(t, "2", constants[2].Value, "C should be 2")
	assert.Equal(t, "0", constants[3].Value, "X should restart at 0 in new block")
	assert.Equal(t, "1", constants[4].Value, "Y should be 1")
}

func TestConstantParserClassConstBlock(t *testing.T) {
	input := `package main

// export_php:classconst Config
const (
	MODE_DEBUG = 1
	MODE_PRODUCTION = 2
	MODE_TEST = 3
)`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 3, "Expected 3 class constants")

	expectedNames := []string{"MODE_DEBUG", "MODE_PRODUCTION", "MODE_TEST"}
	expectedValues := []string{"1", "2", "3"}

	for i, c := range constants {
		assert.Equal(t, expectedNames[i], c.Name, "Expected constant %d name to be '%s'", i, expectedNames[i])
		assert.Equal(t, "Config", c.ClassName, "Expected constant %d to belong to Config class", i)
		assert.Equal(t, expectedValues[i], c.Value, "Expected constant %d value to be '%s'", i, expectedValues[i])
		assert.Equal(t, phpInt, c.PhpType, "Expected constant %d to be phpInt type", i)
	}
}

func TestConstantParserClassConstBlockWithIota(t *testing.T) {
	input := `package main

// export_php:classconst Status
const (
	STATUS_PENDING = iota
	STATUS_ACTIVE
	STATUS_COMPLETED
)`

	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(fileName, []byte(input), 0644))

	parser := &ConstantParser{}
	constants, err := parser.parse(fileName)
	assert.NoError(t, err, "parse() error")

	assert.Len(t, constants, 3, "Expected 3 class constants")

	expectedNames := []string{"STATUS_PENDING", "STATUS_ACTIVE", "STATUS_COMPLETED"}
	expectedValues := []string{"0", "1", "2"}

	for i, c := range constants {
		assert.Equal(t, expectedNames[i], c.Name, "Expected constant %d name to be '%s'", i, expectedNames[i])
		assert.Equal(t, "Status", c.ClassName, "Expected constant %d to belong to Status class", i)
		assert.True(t, c.IsIota, "Expected constant %d to be iota", i)
		assert.Equal(t, expectedValues[i], c.Value, "Expected constant %d value to be '%s'", i, expectedValues[i])
		assert.Equal(t, phpInt, c.PhpType, "Expected constant %d to be phpInt type", i)
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
		name   string
		input  string
		expect int
		assert func(t *testing.T, constants []phpConstant)
	}{
		{
			name: "single class constant",
			input: `package main

//export_php:classconst MyClass
const STATUS_ACTIVE = 1`,
			expect: 1,
			assert: func(t *testing.T, cs []phpConstant) {
				c := cs[0]
				assert.Equal(t, "STATUS_ACTIVE", c.Name)
				assert.Equal(t, "MyClass", c.ClassName)
				assert.Equal(t, "1", c.Value)
				assert.Equal(t, phpInt, c.PhpType)
			},
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
			expect: 3,
			assert: func(t *testing.T, cs []phpConstant) {
				classes := []string{"User", "User", "Order"}
				names := []string{"STATUS_ACTIVE", "STATUS_INACTIVE", "STATE_PENDING"}
				values := []string{`"active"`, `"inactive"`, "0"}
				for i, c := range cs {
					assert.Equal(t, classes[i], c.ClassName)
					assert.Equal(t, names[i], c.Name)
					assert.Equal(t, values[i], c.Value)
				}
			},
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
			expect: 3,
			assert: func(t *testing.T, cs []phpConstant) {
				assert.Empty(t, cs[0].ClassName, "First constant should be global")
				assert.Equal(t, "MyClass", cs[1].ClassName)
				assert.Empty(t, cs[2].ClassName, "Third constant should be global")
			},
		},
		{
			name: "class constant with iota",
			input: `package main

//export_php:classconst Status
const FIRST = iota

//export_php:classconst Status
const SECOND = iota`,
			expect: 2,
		},
		{
			name: "invalid class constant directive",
			input: `package main

//export_php:classconst
const INVALID = "missing class name"`,
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(tmpFile, []byte(tt.input), 0644))

			parser := &ConstantParser{}
			constants, err := parser.parse(tmpFile)
			require.NoError(t, err)
			require.Len(t, constants, tt.expect)

			if tt.assert != nil {
				tt.assert(t, constants)
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
