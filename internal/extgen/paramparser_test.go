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
				{Name: "name", PhpType: phpString, HasDefault: false},
				{Name: "count", PhpType: phpInt, HasDefault: false},
			},
			expected: ParameterInfo{
				RequiredCount: 2,
				TotalCount:    2,
			},
		},
		{
			name: "mixed required and optional parameters",
			params: []phpParameter{
				{Name: "name", PhpType: phpString, HasDefault: false},
				{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", PhpType: phpBool, HasDefault: true, DefaultValue: "true"},
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
				{Name: "message", PhpType: phpString, HasDefault: false},
			},
			expected: "    zend_string *message = NULL;",
		},
		{
			name: "nullable string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: phpString, HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *message = NULL;\n    zend_bool message_is_null = 0;",
		},
		{
			name: "int parameter with default",
			params: []phpParameter{
				{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "42"},
			},
			expected: "    zend_long count = 42;",
		},
		{
			name: "nullable int parameter",
			params: []phpParameter{
				{Name: "count", PhpType: phpInt, HasDefault: false, IsNullable: true},
			},
			expected: "    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
		{
			name: "bool parameter with true default",
			params: []phpParameter{
				{Name: "enabled", PhpType: phpBool, HasDefault: true, DefaultValue: "true"},
			},
			expected: "    zend_bool enabled = 1;",
		},
		{
			name: "nullable bool parameter",
			params: []phpParameter{
				{Name: "enabled", PhpType: phpBool, HasDefault: false, IsNullable: true},
			},
			expected: "    zend_bool enabled = 0;\n    zend_bool enabled_is_null = 0;",
		},
		{
			name: "float parameter",
			params: []phpParameter{
				{Name: "ratio", PhpType: phpFloat, HasDefault: false},
			},
			expected: "    double ratio = 0.0;",
		},
		{
			name: "nullable float parameter",
			params: []phpParameter{
				{Name: "ratio", PhpType: phpFloat, HasDefault: false, IsNullable: true},
			},
			expected: "    double ratio = 0.0;\n    zend_bool ratio_is_null = 0;",
		},
		{
			name: "multiple parameters",
			params: []phpParameter{
				{Name: "name", PhpType: phpString, HasDefault: false},
				{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "10"},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 10;",
		},
		{
			name: "mixed nullable and non-nullable parameters",
			params: []phpParameter{
				{Name: "name", PhpType: phpString, HasDefault: false, IsNullable: false},
				{Name: "count", PhpType: phpInt, HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
		{
			name: "array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: phpArray, HasDefault: false},
			},
			expected: "    zval *items = NULL;",
		},
		{
			name: "nullable array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: phpArray, HasDefault: false, IsNullable: true},
			},
			expected: "    zval *items = NULL;",
		},
		{
			name: "mixed types with array",
			params: []phpParameter{
				{Name: "name", PhpType: phpString, HasDefault: false},
				{Name: "items", PhpType: phpArray, HasDefault: false},
				{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "5"},
			},
			expected: "    zend_string *name = NULL;\n    zval *items = NULL;\n    zend_long count = 5;",
		},
		{
			name: "mixed parameter",
			params: []phpParameter{
				{Name: "m", PhpType: phpMixed, HasDefault: false},
			},
			expected: "    zval *m = NULL;",
		},
		{
			name: "nullable mixed parameter",
			params: []phpParameter{
				{Name: "m", PhpType: phpMixed, HasDefault: false, IsNullable: true},
			},
			expected: "    zval *m = NULL;",
		},
		{
			name: "callable parameter",
			params: []phpParameter{
				{Name: "callback", PhpType: phpCallable, HasDefault: false},
			},
			expected: "    zval *callback_callback;",
		},
		{
			name: "nullable callable parameter",
			params: []phpParameter{
				{Name: "callback", PhpType: phpCallable, HasDefault: false, IsNullable: true},
			},
			expected: "    zval *callback_callback;",
		},
		{
			name: "mixed types with callable",
			params: []phpParameter{
				{Name: "data", PhpType: phpArray, HasDefault: false},
				{Name: "callback", PhpType: phpCallable, HasDefault: false},
				{Name: "options", PhpType: phpInt, HasDefault: true, DefaultValue: "0"},
			},
			expected: "    zval *data = NULL;\n    zval *callback_callback;\n    zend_long options = 0;",
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
				{Name: "message", PhpType: phpString, HasDefault: false},
			},
			requiredCount: 1,
			expected: `    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STR(message)
    ZEND_PARSE_PARAMETERS_END();`,
		},
		{
			name: "mixed required and optional parameters",
			params: []phpParameter{
				{Name: "name", PhpType: phpString, HasDefault: false},
				{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", PhpType: phpBool, HasDefault: true, DefaultValue: "true"},
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
				{Name: "message", PhpType: phpString},
			},
			expected: "message",
		},
		{
			name: "multiple parameters of different types",
			params: []phpParameter{
				{Name: "name", PhpType: phpString},
				{Name: "count", PhpType: phpInt},
				{Name: "ratio", PhpType: phpFloat},
				{Name: "enabled", PhpType: phpBool},
			},
			expected: "name, (long) count, (double) ratio, (int) enabled",
		},
		{
			name: "array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: phpArray},
			},
			expected: "items",
		},
		{
			name: "nullable array parameter",
			params: []phpParameter{
				{Name: "items", PhpType: phpArray, IsNullable: true},
			},
			expected: "items",
		},
		{
			name: "mixed parameters with array",
			params: []phpParameter{
				{Name: "name", PhpType: phpString},
				{Name: "items", PhpType: phpArray},
				{Name: "count", PhpType: phpInt},
			},
			expected: "name, items, (long) count",
		},
		{
			name: "callable parameter",
			params: []phpParameter{
				{Name: "callback", PhpType: "callable"},
			},
			expected: "callback_callback",
		},
		{
			name: "nullable callable parameter",
			params: []phpParameter{
				{Name: "callback", PhpType: "callable", IsNullable: true},
			},
			expected: "callback_callback",
		},
		{
			name: "mixed parameters with callable",
			params: []phpParameter{
				{Name: "data", PhpType: "array"},
				{Name: "callback", PhpType: "callable"},
				{Name: "limit", PhpType: "int"},
			},
			expected: "data, callback_callback, (long) limit",
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
			param:    phpParameter{Name: "message", PhpType: phpString},
			expected: "\n        Z_PARAM_STR(message)",
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: phpString, IsNullable: true},
			expected: "\n        Z_PARAM_STR_OR_NULL(message, message_is_null)",
		},
		{
			name:     "int parameter",
			param:    phpParameter{Name: "count", PhpType: phpInt},
			expected: "\n        Z_PARAM_LONG(count)",
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: phpInt, IsNullable: true},
			expected: "\n        Z_PARAM_LONG_OR_NULL(count, count_is_null)",
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat},
			expected: "\n        Z_PARAM_DOUBLE(ratio)",
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat, IsNullable: true},
			expected: "\n        Z_PARAM_DOUBLE_OR_NULL(ratio, ratio_is_null)",
		},
		{
			name:     "bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: phpBool},
			expected: "\n        Z_PARAM_BOOL(enabled)",
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: phpBool, IsNullable: true},
			expected: "\n        Z_PARAM_BOOL_OR_NULL(enabled, enabled_is_null)",
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray},
			expected: "\n        Z_PARAM_ARRAY(items)",
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray, IsNullable: true},
			expected: "\n        Z_PARAM_ARRAY_OR_NULL(items)",
		},
		{
			name:     "mixed parameter",
			param:    phpParameter{Name: "m", PhpType: phpMixed},
			expected: "\n        Z_PARAM_ZVAL(m)",
		},
		{
			name:     "nullable mixed parameter",
			param:    phpParameter{Name: "m", PhpType: phpMixed, IsNullable: true},
			expected: "\n        Z_PARAM_ZVAL_OR_NULL(m)",
		},
		{
			name:     "callable parameter",
			param:    phpParameter{Name: "callback", PhpType: phpCallable},
			expected: "\n        Z_PARAM_ZVAL(callback_callback)",
		},
		{
			name:     "nullable callable parameter",
			param:    phpParameter{Name: "callback", PhpType: phpCallable, IsNullable: true},
			expected: "\n        Z_PARAM_ZVAL_OR_NULL(callback_callback)",
		},
		{
			name:     "unknown type",
			param:    phpParameter{Name: "unknown", PhpType: phpType("unknown")},
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
			param:    phpParameter{Name: "count", PhpType: phpInt, HasDefault: false},
			fallback: "0",
			expected: "0",
		},
		{
			name:     "parameter with default value",
			param:    phpParameter{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "42"},
			fallback: "0",
			expected: "42",
		},
		{
			name:     "parameter with empty default value",
			param:    phpParameter{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: ""},
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
			param:    phpParameter{Name: "message", PhpType: phpString},
			expected: "message",
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: phpString, IsNullable: true},
			expected: "message_is_null ? NULL : message",
		},
		{
			name:     "int parameter",
			param:    phpParameter{Name: "count", PhpType: phpInt},
			expected: "(long) count",
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: phpInt, IsNullable: true},
			expected: "count_is_null ? NULL : &count",
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat},
			expected: "(double) ratio",
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat, IsNullable: true},
			expected: "ratio_is_null ? NULL : &ratio",
		},
		{
			name:     "bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: phpBool},
			expected: "(int) enabled",
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: phpBool, IsNullable: true},
			expected: "enabled_is_null ? NULL : &enabled",
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray},
			expected: "items",
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray, IsNullable: true},
			expected: "items",
		},
		{
			name:     "callable parameter",
			param:    phpParameter{Name: "callback", PhpType: "callable"},
			expected: "callback_callback",
		},
		{
			name:     "nullable callable parameter",
			param:    phpParameter{Name: "callback", PhpType: "callable", IsNullable: true},
			expected: "callback_callback",
		},
		{
			name:     "unknown type",
			param:    phpParameter{Name: "unknown", PhpType: phpType("unknown")},
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
			param:    phpParameter{Name: "message", PhpType: phpString, HasDefault: false},
			expected: []string{"zend_string *message = NULL;"},
		},
		{
			name:     "nullable string parameter",
			param:    phpParameter{Name: "message", PhpType: phpString, HasDefault: false, IsNullable: true},
			expected: []string{"zend_string *message = NULL;", "zend_bool message_is_null = 0;"},
		},
		{
			name:     "int parameter with default",
			param:    phpParameter{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "42"},
			expected: []string{"zend_long count = 42;"},
		},
		{
			name:     "nullable int parameter",
			param:    phpParameter{Name: "count", PhpType: phpInt, HasDefault: false, IsNullable: true},
			expected: []string{"zend_long count = 0;", "zend_bool count_is_null = 0;"},
		},
		{
			name:     "bool parameter with true default",
			param:    phpParameter{Name: "enabled", PhpType: phpBool, HasDefault: true, DefaultValue: "true"},
			expected: []string{"zend_bool enabled = 1;"},
		},
		{
			name:     "nullable bool parameter",
			param:    phpParameter{Name: "enabled", PhpType: phpBool, HasDefault: false, IsNullable: true},
			expected: []string{"zend_bool enabled = 0;", "zend_bool enabled_is_null = 0;"},
		},
		{
			name:     "bool parameter with false default",
			param:    phpParameter{Name: "disabled", PhpType: phpBool, HasDefault: true, DefaultValue: "false"},
			expected: []string{"zend_bool disabled = false;"},
		},
		{
			name:     "float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat, HasDefault: false},
			expected: []string{"double ratio = 0.0;"},
		},
		{
			name:     "nullable float parameter",
			param:    phpParameter{Name: "ratio", PhpType: phpFloat, HasDefault: false, IsNullable: true},
			expected: []string{"double ratio = 0.0;", "zend_bool ratio_is_null = 0;"},
		},
		{
			name:     "array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray, HasDefault: false},
			expected: []string{"zval *items = NULL;"},
		},
		{
			name:     "nullable array parameter",
			param:    phpParameter{Name: "items", PhpType: phpArray, HasDefault: false, IsNullable: true},
			expected: []string{"zval *items = NULL;"},
		},
		{
			name:     "callable parameter",
			param:    phpParameter{Name: "callback", PhpType: "callable", HasDefault: false},
			expected: []string{"zval *callback_callback;"},
		},
		{
			name:     "nullable callable parameter",
			param:    phpParameter{Name: "callback", PhpType: "callable", HasDefault: false, IsNullable: true},
			expected: []string{"zval *callback_callback;"},
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
		{Name: "name", PhpType: phpString, HasDefault: false},
		{Name: "count", PhpType: phpInt, HasDefault: true, DefaultValue: "10"},
		{Name: "enabled", PhpType: phpBool, HasDefault: true, DefaultValue: "true"},
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
