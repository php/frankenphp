package extgen

import (
	"fmt"
	"strings"
)

type ParameterParser struct{}

type ParameterInfo struct {
	RequiredCount int
	TotalCount    int
}

func (pp *ParameterParser) analyzeParameters(params []phpParameter) ParameterInfo {
	info := ParameterInfo{TotalCount: len(params)}

	for _, param := range params {
		if !param.HasDefault {
			info.RequiredCount++
		}
	}

	return info
}

func (pp *ParameterParser) generateParamDeclarations(params []phpParameter) string {
	if len(params) == 0 {
		return ""
	}

	var declarations []string

	for _, param := range params {
		declarations = append(declarations, pp.generateSingleParamDeclaration(param)...)
	}

	return "    " + strings.Join(declarations, "\n    ")
}

func (pp *ParameterParser) generateSingleParamDeclaration(param phpParameter) []string {
	var decls []string

	switch param.PhpType {
	case phpString:
		decls = append(decls, fmt.Sprintf("zend_string *%s = NULL;", param.Name))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}
	case phpInt:
		defaultVal := pp.getDefaultValue(param, "0")
		decls = append(decls, fmt.Sprintf("zend_long %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}
	case phpFloat:
		defaultVal := pp.getDefaultValue(param, "0.0")
		decls = append(decls, fmt.Sprintf("double %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}
	case phpBool:
		defaultVal := pp.getDefaultValue(param, "0")
		if param.HasDefault && param.DefaultValue == "true" {
			defaultVal = "1"
		}
		decls = append(decls, fmt.Sprintf("zend_bool %s = %s;", param.Name, defaultVal))
		if param.IsNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.Name))
		}
	case phpArray, phpMixed:
		decls = append(decls, fmt.Sprintf("zval *%s = NULL;", param.Name))
	}

	return decls
}

func (pp *ParameterParser) getDefaultValue(param phpParameter, fallback string) string {
	if !param.HasDefault || param.DefaultValue == "" {
		return fallback
	}
	return param.DefaultValue
}

func (pp *ParameterParser) generateParamParsing(params []phpParameter, requiredCount int) string {
	if len(params) == 0 {
		return `    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }`
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("    ZEND_PARSE_PARAMETERS_START(%d, %d)", requiredCount, len(params)))

	optionalStarted := false
	for _, param := range params {
		if param.HasDefault && !optionalStarted {
			builder.WriteString("\n        Z_PARAM_OPTIONAL")
			optionalStarted = true
		}

		builder.WriteString(pp.generateParamParsingMacro(param))
	}

	builder.WriteString("\n    ZEND_PARSE_PARAMETERS_END();")
	return builder.String()
}

func (pp *ParameterParser) generateParamParsingMacro(param phpParameter) string {
	if param.IsNullable {
		switch param.PhpType {
		case phpString:
			return fmt.Sprintf("\n        Z_PARAM_STR_OR_NULL(%s, %s_is_null)", param.Name, param.Name)
		case phpInt:
			return fmt.Sprintf("\n        Z_PARAM_LONG_OR_NULL(%s, %s_is_null)", param.Name, param.Name)
		case phpFloat:
			return fmt.Sprintf("\n        Z_PARAM_DOUBLE_OR_NULL(%s, %s_is_null)", param.Name, param.Name)
		case phpBool:
			return fmt.Sprintf("\n        Z_PARAM_BOOL_OR_NULL(%s, %s_is_null)", param.Name, param.Name)
		case phpArray:
			return fmt.Sprintf("\n        Z_PARAM_ARRAY_OR_NULL(%s)", param.Name)
		case phpMixed:
			return fmt.Sprintf("\n        Z_PARAM_ZVAL_OR_NULL(%s)", param.Name)
		default:
			return ""
		}
	} else {
		switch param.PhpType {
		case phpString:
			return fmt.Sprintf("\n        Z_PARAM_STR(%s)", param.Name)
		case phpInt:
			return fmt.Sprintf("\n        Z_PARAM_LONG(%s)", param.Name)
		case phpFloat:
			return fmt.Sprintf("\n        Z_PARAM_DOUBLE(%s)", param.Name)
		case phpBool:
			return fmt.Sprintf("\n        Z_PARAM_BOOL(%s)", param.Name)
		case phpArray:
			return fmt.Sprintf("\n        Z_PARAM_ARRAY(%s)", param.Name)
		case phpMixed:
			return fmt.Sprintf("\n        Z_PARAM_ZVAL(%s)", param.Name)
		default:
			return ""
		}
	}
}

func (pp *ParameterParser) generateGoCallParams(params []phpParameter) string {
	if len(params) == 0 {
		return ""
	}

	var goParams []string
	for _, param := range params {
		goParams = append(goParams, pp.generateSingleGoCallParam(param))
	}

	return strings.Join(goParams, ", ")
}

func (pp *ParameterParser) generateSingleGoCallParam(param phpParameter) string {
	if param.IsNullable {
		switch param.PhpType {
		case phpString:
			return fmt.Sprintf("%s_is_null ? NULL : %s", param.Name, param.Name)
		case phpInt:
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.Name, param.Name)
		case phpFloat:
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.Name, param.Name)
		case phpBool:
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.Name, param.Name)
		default:
			return param.Name
		}
	}

	switch param.PhpType {
	case phpInt:
		return fmt.Sprintf("(long) %s", param.Name)
	case phpFloat:
		return fmt.Sprintf("(double) %s", param.Name)
	case phpBool:
		return fmt.Sprintf("(int) %s", param.Name)
	default:
		return param.Name
	}
}
