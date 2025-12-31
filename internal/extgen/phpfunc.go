package extgen

import (
	"fmt"
	"strings"
)

type PHPFuncGenerator struct {
	paramParser *ParameterParser
	namespace   string
}

func (pfg *PHPFuncGenerator) generate(fn phpFunction) string {
	var builder strings.Builder

	paramInfo := pfg.paramParser.analyzeParameters(fn.Params)

	funcName := NamespacedName(pfg.namespace, fn.Name)
	builder.WriteString(fmt.Sprintf("PHP_FUNCTION(%s)\n{\n", funcName))

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

	if fn.ReturnType == phpVoid {
		return fmt.Sprintf("    %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == phpString {
		return fmt.Sprintf("    zend_string *result = %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == phpArray {
		return fmt.Sprintf("    zend_array *result = %s(%s);", fn.Name, callParams)
	}

	if fn.ReturnType == phpMixed {
		return fmt.Sprintf("    zval *result = %s(%s);", fn.Name, callParams)
	}

	return fmt.Sprintf("    %s result = %s(%s);", pfg.getCReturnType(fn.ReturnType), fn.Name, callParams)
}

func (pfg *PHPFuncGenerator) getCReturnType(returnType phpType) string {
	switch returnType {
	case phpInt:
		return "long"
	case phpFloat:
		return "double"
	case phpBool:
		return "int"
	default:
		return "void"
	}
}

func (pfg *PHPFuncGenerator) generateReturnCode(returnType phpType) string {
	switch returnType {
	case phpString:
		return `    if (result) {
        RETURN_STR(result);
    }

	RETURN_EMPTY_STRING();`
	case phpInt:
		return `    RETURN_LONG(result);`
	case phpFloat:
		return `    RETURN_DOUBLE(result);`
	case phpBool:
		return `    RETURN_BOOL(result);`
	case phpArray:
		return `    if (result) {
        RETURN_ARR(result);
    }

	RETURN_EMPTY_ARRAY();`
	default:
		return ""
	}
}
