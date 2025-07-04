package extgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParameterParser_AnalyzeParameters(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []phpParameter
		expected ParameterInfo
	}{
		{
			name:   "no parameters",
			params: []phpParameter{},
			expected: ParameterInfo{
				RequiredCount: 0,
				TotalCount:    0,
			},
		},
		{
			name: "all required parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false},
				{Name: "count", PhpType: "int", HasDefault: false},
			},
			expected: ParameterInfo{
				RequiredCount: 2,
				TotalCount:    2,
			},
		},
		{
			name: "mixed required and optional parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false},
				{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
			},
			expected: ParameterInfo{
				RequiredCount: 1,
				TotalCount:    3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.analyzeParameters(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamDeclarations(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []phpParameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []phpParameter{},
			expected: "",
		},
		{
			name: "string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string", HasDefault: false},
			},
			expected: "    zend_string *message = NULL;",
		},
		{
			name: "nullable string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *message = NULL;\n    zend_bool message_is_null = 0;",
		},
		{
			name: "int parameter with default",
			params: []phpParameter{
				{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "42"},
			},
			expected: "    zend_long count = 42;",
		},
		{
			name: "nullable int parameter",
			params: []phpParameter{
				{Name: "count", PhpType: "int", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
		{
			name: "bool parameter with true default",
			params: []phpParameter{
				{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
			},
			expected: "    zend_bool enabled = 1;",
		},
		{
			name: "nullable bool parameter",
			params: []phpParameter{
				{Name: "enabled", PhpType: "bool", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_bool enabled = 0;\n    zend_bool enabled_is_null = 0;",
		},
		{
			name: "float parameter",
			params: []phpParameter{
				{Name: "ratio", PhpType: "float", HasDefault: false},
			},
			expected: "    double ratio = 0.0;",
		},
		{
			name: "nullable float parameter",
			params: []phpParameter{
				{Name: "ratio", PhpType: "float", HasDefault: false, IsNullable: true},
			},
			expected: "    double ratio = 0.0;\n    zend_bool ratio_is_null = 0;",
		},
		{
			name: "multiple parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false},
				{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "10"},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 10;",
		},
		{
			name: "mixed nullable and non-nullable parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false, IsNullable: false},
				{Name: "count", PhpType: "int", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
		{
			name: "array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: "array", HasDefault: false},
			},
			expected: "    zval *items = NULL;",
		},
		{
			name: "nullable array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: "array", HasDefault: false, IsNullable: true},
			},
			expected: "    zval *items = NULL;",
		},
		{
			name: "mixed types with array",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false},
				{Name: "items", PhpType: "array", HasDefault: false},
				{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "5"},
			},
			expected: "    zend_string *name = NULL;\n    zval *items = NULL;\n    zend_long count = 5;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamDeclarations(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamParsing(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name          string
		params        []phpParameter
		requiredCount int
		expected      string
	}{
		{
			name:          "no parameters",
			params:        []phpParameter{},
			requiredCount: 0,
			expected: `    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }`,
		},
		{
			name: "single required string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string", HasDefault: false},
			},
			requiredCount: 1,
			expected: `    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STR(message)
    ZEND_PARSE_PARAMETERS_END();`,
		},
		{
			name: "mixed required and optional parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string", HasDefault: false},
				{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
			},
			requiredCount: 1,
			expected: `    ZEND_PARSE_PARAMETERS_START(1, 3)
        Z_PARAM_STR(name)
        Z_PARAM_OPTIONAL
        Z_PARAM_LONG(count)
        Z_PARAM_BOOL(enabled)
    ZEND_PARSE_PARAMETERS_END();`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamParsing(tt.params, tt.requiredCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateGoCallParams(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []phpParameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []phpParameter{},
			expected: "",
		},
		{
			name: "single string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string"},
			},
			expected: "message",
		},
		{
			name: "multiple parameters of different types",
			params: []phpParameter{
				{Name: "name", PhpType: "string"},
				{Name: "count", PhpType: "int"},
				{Name: "ratio", PhpType: "float"},
				{Name: "enabled", PhpType: "bool"},
			},
			expected: "name, (long) count, (double) ratio, (int) enabled",
		},
		{
			name: "array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: "array"},
			},
			expected: "items",
		},
		{
			name: "nullable array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: "array", IsNullable: true},
			},
			expected: "items",
		},
		{
			name: "mixed parameters with array",
			params: []phpParameter{
				{Name: "name", PhpType: "string"},
				{Name: "items", PhpType: "array"},
				{Name: "count", PhpType: "int"},
			},
			expected: "name, items, (long) count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateGoCallParams(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamParsingMacro(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    phpParameter
		expected string
	}{
		{
			name:     "string parameter",
			param:    phpParameter{Name: "message", PhpType: "string"},
			expected: "\n        Z_PARAM_STR(message)",
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: "string", IsNullable: true},
			expected: "\n        Z_PARAM_STR_OR_NULL(message, message_is_null)",
		},
		{
			name:     "int parameter",
			param:    phpParameter{Name: "count", PhpType: "int"},
			expected: "\n        Z_PARAM_LONG(count)",
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: "int", IsNullable: true},
			expected: "\n        Z_PARAM_LONG_OR_NULL(count, count_is_null)",
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float"},
			expected: "\n        Z_PARAM_DOUBLE(ratio)",
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float", IsNullable: true},
			expected: "\n        Z_PARAM_DOUBLE_OR_NULL(ratio, ratio_is_null)",
		},
		{
			name:     "bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: "bool"},
			expected: "\n        Z_PARAM_BOOL(enabled)",
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: "bool", IsNullable: true},
			expected: "\n        Z_PARAM_BOOL_OR_NULL(enabled, enabled_is_null)",
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: "array"},
			expected: "\n        Z_PARAM_ARRAY(items)",
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: "array", IsNullable: true},
			expected: "\n        Z_PARAM_ARRAY_OR_NULL(items)",
		},
		{
			name:     "unknown type",
			param:    phpParameter{Name: "unknown", PhpType: "unknown"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamParsingMacro(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GetDefaultValue(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    phpParameter
		fallback string
		expected string
	}{
		{
			name:     "parameter without default",
			param:    phpParameter{Name: "count", PhpType: "int", HasDefault: false},
			fallback: "0",
			expected: "0",
		},
		{
			name:     "parameter with default value",
			param:    phpParameter{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "42"},
			fallback: "0",
			expected: "42",
		},
		{
			name:     "parameter with empty default value",
			param:    phpParameter{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: ""},
			fallback: "0",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.getDefaultValue(tt.param, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateSingleGoCallParam(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    phpParameter
		expected string
	}{
		{
			name:     "string parameter",
			param:    phpParameter{Name: "message", PhpType: "string"},
			expected: "message",
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: "string", IsNullable: true},
			expected: "message_is_null ? NULL : message",
		},
		{
			name:     "int parameter",
			param:    phpParameter{Name: "count", PhpType: "int"},
			expected: "(long) count",
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: "int", IsNullable: true},
			expected: "count_is_null ? NULL : &count",
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float"},
			expected: "(double) ratio",
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float", IsNullable: true},
			expected: "ratio_is_null ? NULL : &ratio",
		},
		{
			name:     "bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: "bool"},
			expected: "(int) enabled",
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: "bool", IsNullable: true},
			expected: "enabled_is_null ? NULL : &enabled",
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: "array"},
			expected: "items",
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: "array", IsNullable: true},
			expected: "items",
		},
		{
			name:     "unknown type",
			param:    phpParameter{Name: "unknown", PhpType: "unknown"},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateSingleGoCallParam(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateSingleParamDeclaration(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    phpParameter
		expected []string
	}{
		{
			name:     "string parameter",
			param:    phpParameter{Name: "message", PhpType: "string", HasDefault: false},
			expected: []string{"zend_string *message = NULL;"},
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: "string", HasDefault: false, IsNullable: true},
			expected: []string{"zend_string *message = NULL;", "zend_bool message_is_null = 0;"},
		},
		{
			name:     "int parameter with default",
			param:    phpParameter{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "42"},
			expected: []string{"zend_long count = 42;"},
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: "int", HasDefault: false, IsNullable: true},
			expected: []string{"zend_long count = 0;", "zend_bool count_is_null = 0;"},
		},
		{
			name:     "bool parameter with true default",
			param:    phpParameter{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
			expected: []string{"zend_bool enabled = 1;"},
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: "bool", HasDefault: false, IsNullable: true},
			expected: []string{"zend_bool enabled = 0;", "zend_bool enabled_is_null = 0;"},
		},
		{
			name:     "bool parameter with false default",
			param:    phpParameter{Name: "disabled", PhpType: "bool", HasDefault: true, DefaultValue: "false"},
			expected: []string{"zend_bool disabled = false;"},
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float", HasDefault: false},
			expected: []string{"double ratio = 0.0;"},
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: "float", HasDefault: false, IsNullable: true},
			expected: []string{"double ratio = 0.0;", "zend_bool ratio_is_null = 0;"},
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: "array", HasDefault: false},
			expected: []string{"zval *items = NULL;"},
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: "array", HasDefault: false, IsNullable: true},
			expected: []string{"zval *items = NULL;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateSingleParamDeclaration(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_Integration(t *testing.T) {
	pp := &ParameterParser{}

	params := []phpParameter{
		{Name: "name", PhpType: "string", HasDefault: false},
		{Name: "count", PhpType: "int", HasDefault: true, DefaultValue: "10"},
		{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
	}

	info := pp.analyzeParameters(params)
	assert.Equal(t, 1, info.RequiredCount)
	assert.Equal(t, 3, info.TotalCount)

	declarations := pp.generateParamDeclarations(params)
	expectedDeclarations := []string{
		"zend_string *name = NULL;",
		"zend_long count = 10;",
		"zend_bool enabled = 1;",
	}
	for _, expected := range expectedDeclarations {
		assert.Contains(t, declarations, expected)
	}

	parsing := pp.generateParamParsing(params, info.RequiredCount)
	assert.Contains(t, parsing, "ZEND_PARSE_PARAMETERS_START(1, 3)")
	assert.Contains(t, parsing, "Z_PARAM_OPTIONAL")

	goCallParams := pp.generateGoCallParams(params)
	assert.Equal(t, "name, (long) count, (int) enabled", goCallParams)
}
