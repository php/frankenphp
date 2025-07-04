package extgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var functionNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var parameterNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var classNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var propNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type Validator struct{}

func (v *Validator) validateFunction(fn phpFunction) error {
	if fn.Name == "" {
		return fmt.Errorf("function name cannot be empty")
	}

	if !functionNameRegex.MatchString(fn.Name) {
		return fmt.Errorf("invalid function name: %s", fn.Name)
	}

	for i, param := range fn.Params {
		if err := v.validateParameter(param); err != nil {
			return fmt.Errorf("parameter %d (%s): %w", i, param.Name, err)
		}
	}

	if err := v.validateReturnType(fn.ReturnType); err != nil {
		return fmt.Errorf("return type: %w", err)
	}

	return nil
}

func (v *Validator) validateParameter(param phpParameter) error {
	if param.Name == "" {
		return fmt.Errorf("parameter name cannot be empty")
	}

	if !parameterNameRegex.MatchString(param.Name) {
		return fmt.Errorf("invalid parameter name: %s", param.Name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(param.PhpType, validTypes) {
		return fmt.Errorf("invalid parameter type: %s", param.PhpType)
	}

	return nil
}

func (v *Validator) validateReturnType(returnType string) error {
	validReturnTypes := []string{"void", "string", "int", "float", "bool", "array", "object", "mixed", "null", "true", "false"}
	if !v.isValidType(returnType, validReturnTypes) {
		return fmt.Errorf("invalid return type: %s", returnType)
	}
	return nil
}

func (v *Validator) validateClass(class phpClass) error {
	if class.Name == "" {
		return fmt.Errorf("class name cannot be empty")
	}

	if !classNameRegex.MatchString(class.Name) {
		return fmt.Errorf("invalid class name: %s", class.Name)
	}

	for i, prop := range class.Properties {
		if err := v.validateClassProperty(prop); err != nil {
			return fmt.Errorf("property %d (%s): %w", i, prop.Name, err)
		}
	}

	return nil
}

func (v *Validator) validateClassProperty(prop phpClassProperty) error {
	if prop.Name == "" {
		return fmt.Errorf("property name cannot be empty")
	}

	if !propNameRegex.MatchString(prop.Name) {
		return fmt.Errorf("invalid property name: %s", prop.Name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(prop.PhpType, validTypes) {
		return fmt.Errorf("invalid property type: %s", prop.PhpType)
	}

	return nil
}

func (v *Validator) isValidType(typeStr string, validTypes []string) bool {
	for _, valid := range validTypes {
		if typeStr == valid {
			return true
		}
	}
	return false
}

// validateScalarTypes checks if PHP signature contains only supported scalar types
func (v *Validator) validateScalarTypes(fn phpFunction) error {
	supportedTypes := []string{"string", "int", "float", "bool", "array"}

	for i, param := range fn.Params {
		if !v.isScalarType(param.PhpType, supportedTypes) {
			return fmt.Errorf("parameter %d (%s) has unsupported type '%s'. Only scalar types (string, int, float, bool, array) and their nullable variants are supported", i+1, param.Name, param.PhpType)
		}
	}

	if fn.ReturnType != "void" && !v.isScalarType(fn.ReturnType, supportedTypes) {
		return fmt.Errorf("return type '%s' is not supported. Only scalar types (string, int, float, bool, array), void, and their nullable variants are supported", fn.ReturnType)
	}

	return nil
}

func (v *Validator) isScalarType(phpType string, supportedTypes []string) bool {
	for _, supported := range supportedTypes {
		if phpType == supported {
			return true
		}
	}
	return false
}

// validateGoFunctionSignatureWithOptions validates with option for method vs function
func (v *Validator) validateGoFunctionSignatureWithOptions(phpFunc phpFunction, isMethod bool) error {
	if phpFunc.GoFunction == "" {
		return fmt.Errorf("no Go function found for PHP function '%s'", phpFunc.Name)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", "package main\n"+phpFunc.GoFunction, 0)
	if err != nil {
		return fmt.Errorf("failed to parse Go function: %w", err)
	}

	var goFunc *ast.FuncDecl
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			goFunc = funcDecl
			break
		}
	}

	if goFunc == nil {
		return fmt.Errorf("no function declaration found in Go function")
	}

	goParamCount := 0
	if goFunc.Type.Params != nil {
		goParamCount = len(goFunc.Type.Params.List)
	}

	hasReceiver := goFunc.Recv != nil && len(goFunc.Recv.List) > 0
	paramOffset := 0
	effectiveGoParamCount := goParamCount

	if hasReceiver {
		paramOffset = 0
		effectiveGoParamCount = goParamCount
	} else if isMethod && goParamCount > 0 {
		// this is a method-like function, first parameter should be the struct
		paramOffset = 1
		effectiveGoParamCount = goParamCount - 1
	}

	if len(phpFunc.Params) != effectiveGoParamCount {
		return fmt.Errorf("parameter count mismatch: PHP function has %d parameters but Go function has %d", len(phpFunc.Params), effectiveGoParamCount)
	}

	if goFunc.Type.Params != nil && len(phpFunc.Params) > 0 {
		for i, phpParam := range phpFunc.Params {
			goParamIndex := i + paramOffset

			if goParamIndex >= len(goFunc.Type.Params.List) {
				break
			}

			goParam := goFunc.Type.Params.List[goParamIndex]
			expectedGoType := v.phpTypeToGoType(phpParam.PhpType, phpParam.IsNullable)
			actualGoType := v.goTypeToString(goParam.Type)

			if !v.isCompatibleGoType(expectedGoType, actualGoType) {
				return fmt.Errorf("parameter %d type mismatch: PHP '%s' requires Go type '%s' but found '%s'", i+1, phpParam.PhpType, expectedGoType, actualGoType)
			}
		}
	}

	expectedGoReturnType := v.phpReturnTypeToGoType(phpFunc.ReturnType, phpFunc.IsReturnNullable)
	actualGoReturnType := v.goReturnTypeToString(goFunc.Type.Results)

	if !v.isCompatibleGoType(expectedGoReturnType, actualGoReturnType) {
		return fmt.Errorf("return type mismatch: PHP '%s' requires Go return type '%s' but found '%s'", phpFunc.ReturnType, expectedGoReturnType, actualGoReturnType)
	}

	return nil
}

func (v *Validator) phpTypeToGoType(phpType string, isNullable bool) string {
	var baseType string
	switch phpType {
	case "string":
		baseType = "*C.zend_string"
	case "int":
		baseType = "int64"
	case "float":
		baseType = "float64"
	case "bool":
		baseType = "bool"
	case "array":
		baseType = "*C.zval"
	default:
		baseType = "interface{}"
	}

	if isNullable && phpType != "string" && phpType != "array" {
		return "*" + baseType
	}

	return baseType
}

// isCompatibleGoType checks if the actual Go type is compatible with the expected type.
func (v *Validator) isCompatibleGoType(expectedType, actualType string) bool {
	if expectedType == actualType {
		return true
	}

	switch expectedType {
	case "int64":
		return actualType == "int"
	case "*int64":
		return actualType == "*int"
	case "*float64":
		return actualType == "*float32"
	}

	return false
}

func (v *Validator) phpReturnTypeToGoType(phpReturnType string, isNullable bool) string {
	switch phpReturnType {
	case "void":
		return ""
	case "string":
		return "unsafe.Pointer"
	case "int":
		return "int64"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	case "array":
		return "unsafe.Pointer"
	default:
		return "interface{}"
	}
}

func (v *Validator) goTypeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + v.goTypeToString(t.X)
	case *ast.SelectorExpr:
		return v.goTypeToString(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
}

func (v *Validator) goReturnTypeToString(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}

	if len(results.List) == 1 {
		return v.goTypeToString(results.List[0].Type)
	}

	var types []string
	for _, field := range results.List {
		types = append(types, v.goTypeToString(field.Type))
	}
	return "(" + strings.Join(types, ", ") + ")"
}
