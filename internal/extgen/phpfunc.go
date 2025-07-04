package extgen

import (
	"fmt"
	"strings"
)

type PHPFuncGenerator struct {
	paramParser *ParameterParser
}

func (pfg *PHPFuncGenerator) generate(fn phpFunction) string {
	var builder strings.Builder

	paramInfo := pfg.paramParser.analyzeParameters(fn.Params)

	builder.WriteString(fmt.Sprintf("PHP_FUNCTION(%s)\n{\n", fn.Name))

	if decl := pfg.paramParser.generateParamDeclarations(fn.Params); decl != "" {
		builder.WriteString(decl + "\n")
	}

	builder.WriteString(pfg.paramParser.generateParamParsing(fn.Params, paramInfo.RequiredCount) + "\n")

	builder.WriteString(pfg.generateGoCall(fn) + "\n")

	if returnCode := pfg.generateReturnCode(fn.ReturnType); returnCode != "" {
		builder.WriteString(returnCode + "\n")
	}

	builder.WriteString("}\n\n")

	return builder.String()
}

func (pfg *PHPFuncGenerator) generateGoCall(fn phpFunction) string {
	callParams := pfg.paramParser.generateGoCallParams(fn.Params)

	if fn.ReturnType == "void" {
		return fmt.Sprintf("    %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == "string" {
		return fmt.Sprintf("    zend_string *result = %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == "array" {
		return fmt.Sprintf("    zend_array *result = %s(%s);", fn.Name, callParams)
	}

	return fmt.Sprintf("    %s result = %s(%s);", pfg.getCReturnType(fn.ReturnType), fn.Name, callParams)
}

func (pfg *PHPFuncGenerator) getCReturnType(returnType string) string {
	switch returnType {
	case "string":
		return "zend_string*"
	case "int":
		return "long"
	case "float":
		return "double"
	case "bool":
		return "int"
	case "array":
		return "zend_array*"
	default:
		return "void"
	}
}

func (pfg *PHPFuncGenerator) generateReturnCode(returnType string) string {
	switch returnType {
	case "string":
		return `    if (result) {
        RETURN_STR(result);
    } else {
        RETURN_EMPTY_STRING();
    }`
	case "int":
		return `    RETURN_LONG(result);`
	case "float":
		return `    RETURN_DOUBLE(result);`
	case "bool":
		return `    RETURN_BOOL(result);`
	case "array":
		return `    if (result) {
        RETURN_ARR(result);
    } else {
        array_init(return_value);
    }`
	default:
		return ""
	}
}
