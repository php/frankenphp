package extgen

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
)

var phpClassRegex = regexp.MustCompile(`//\s*export_php:class\s+(\w+)`)
var phpMethodRegex = regexp.MustCompile(`//\s*export_php:method\s+(\w+)::([^{}\n]+)(?:\s*{\s*})?`)
var methodSignatureRegex = regexp.MustCompile(`(\w+)\s*\(([^)]*)\)\s*:\s*(\??[\w|]+)`)
var methodParamTypeNameRegex = regexp.MustCompile(`(\??[\w|]+)\s+\$?(\w+)`)

type exportDirective struct {
	line      int
	className string
}

type classParser struct{}

func (cp *classParser) Parse(filename string) ([]phpClass, error) {
	return cp.parse(filename)
}

func (cp *classParser) parse(filename string) (classes []phpClass, err error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	validator := Validator{}

	exportDirectives := cp.collectExportDirectives(node, fset)
	methods, err := cp.parseMethods(filename)
	if err != nil {
		return nil, fmt.Errorf("parsing methods: %w", err)
	}

	// match structs to directives
	matchedDirectives := make(map[int]bool)

	var genDecl *ast.GenDecl
	var ok bool
	for _, decl := range node.Decls {
		if genDecl, ok = decl.(*ast.GenDecl); !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			var typeSpec *ast.TypeSpec
			if typeSpec, ok = spec.(*ast.TypeSpec); !ok {
				continue
			}

			var structType *ast.StructType
			if structType, ok = typeSpec.Type.(*ast.StructType); !ok {
				continue
			}

			var phpCl string
			var directiveLine int
			if phpCl, directiveLine = cp.extractPHPClassCommentWithLine(genDecl.Doc, fset); phpCl == "" {
				continue
			}

			matchedDirectives[directiveLine] = true

			class := phpClass{
				Name:     phpCl,
				GoStruct: typeSpec.Name.Name,
			}

			class.Properties = cp.parseStructFields(structType.Fields.List)

			// associate methods with this class
			for _, method := range methods {
				if method.ClassName == phpCl {
					class.Methods = append(class.Methods, method)
				}
			}

			if err := validator.validateClass(class); err != nil {
				fmt.Printf("Warning: Invalid class '%s': %v\n", class.Name, err)
				continue
			}

			classes = append(classes, class)
		}
	}

	for _, directive := range exportDirectives {
		if !matchedDirectives[directive.line] {
			return nil, fmt.Errorf("//export_php class directive at line %d is not followed by a struct declaration", directive.line)
		}
	}

	return classes, nil
}

func (cp *classParser) collectExportDirectives(node *ast.File, fset *token.FileSet) []exportDirective {
	var directives []exportDirective

	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			if matches := phpClassRegex.FindStringSubmatch(comment.Text); matches != nil {
				pos := fset.Position(comment.Pos())
				directives = append(directives, exportDirective{
					line:      pos.Line,
					className: matches[1],
				})
			}
		}
	}

	return directives
}

func (cp *classParser) extractPHPClassCommentWithLine(commentGroup *ast.CommentGroup, fset *token.FileSet) (string, int) {
	if commentGroup == nil {
		return "", 0
	}

	for _, comment := range commentGroup.List {
		if matches := phpClassRegex.FindStringSubmatch(comment.Text); matches != nil {
			pos := fset.Position(comment.Pos())
			return matches[1], pos.Line
		}
	}

	return "", 0
}

func (cp *classParser) parseStructFields(fields []*ast.Field) []phpClassProperty {
	var properties []phpClassProperty

	for _, field := range fields {
		for _, name := range field.Names {
			prop := cp.parseStructField(name.Name, field)
			properties = append(properties, prop)
		}
	}

	return properties
}

func (cp *classParser) parseStructField(fieldName string, field *ast.Field) phpClassProperty {
	prop := phpClassProperty{Name: fieldName}

	// check if field is a pointer (nullable)
	if starExpr, isPointer := field.Type.(*ast.StarExpr); isPointer {
		prop.IsNullable = true
		prop.GoType = cp.typeToString(starExpr.X)
	} else {
		prop.IsNullable = false
		prop.GoType = cp.typeToString(field.Type)
	}

	prop.PhpType = cp.goTypeToPHPType(prop.GoType)

	return prop
}

func (cp *classParser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + cp.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + cp.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + cp.typeToString(t.Key) + "]" + cp.typeToString(t.Value)
	default:
		return "interface{}"
	}
}

func (cp *classParser) goTypeToPHPType(goType string) phpType {
	goType = strings.TrimPrefix(goType, "*")

	typeMap := map[string]phpType{
		"string": phpString,
		"int":    phpInt, "int64": phpInt, "int32": phpInt, "int16": phpInt, "int8": phpInt,
		"uint": phpInt, "uint64": phpInt, "uint32": phpInt, "uint16": phpInt, "uint8": phpInt,
		"float64": phpFloat, "float32": phpFloat,
		"bool": phpBool,
	}

	if phpType, exists := typeMap[goType]; exists {
		return phpType
	}

	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") {
		return phpArray
	}

	return phpMixed
}

func (cp *classParser) parseMethods(filename string) (methods []phpClassMethod, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		e := file.Close()
		if err != nil {
			err = e
		}
	}()

	scanner := bufio.NewScanner(file)
	var currentMethod *phpClassMethod

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if matches := phpMethodRegex.FindStringSubmatch(line); matches != nil {
			className := strings.TrimSpace(matches[1])
			signature := strings.TrimSpace(matches[2])

			method, err := cp.parseMethodSignature(className, signature)
			if err != nil {
				fmt.Printf("Warning: Error parsing method signature %q: %v\n", signature, err)

				continue
			}

			validator := Validator{}
			phpFunc := phpFunction{
				Name:             method.Name,
				Signature:        method.Signature,
				Params:           method.Params,
				ReturnType:       method.ReturnType,
				IsReturnNullable: method.isReturnNullable,
			}

			if err := validator.validateScalarTypes(phpFunc); err != nil {
				fmt.Printf("Warning: Method \"%s::%s\" uses unsupported types: %v\n", className, method.Name, err)

				continue
			}

			method.lineNumber = lineNumber
			currentMethod = method
		}

		if currentMethod != nil && strings.HasPrefix(line, "func ") {
			goFunc, err := cp.extractGoMethodFunction(scanner, line)
			if err != nil {
				return nil, fmt.Errorf("extracting Go method function: %w", err)
			}

			currentMethod.GoFunction = goFunc

			validator := Validator{}
			phpFunc := phpFunction{
				Name:             currentMethod.Name,
				Signature:        currentMethod.Signature,
				GoFunction:       currentMethod.GoFunction,
				Params:           currentMethod.Params,
				ReturnType:       currentMethod.ReturnType,
				IsReturnNullable: currentMethod.isReturnNullable,
			}

			if err := validator.validateGoFunctionSignatureWithOptions(phpFunc, true); err != nil {
				fmt.Printf("Warning: Go method signature mismatch for '%s::%s': %v\n", currentMethod.ClassName, currentMethod.Name, err)
				currentMethod = nil
				continue
			}

			methods = append(methods, *currentMethod)
			currentMethod = nil
		}
	}

	if currentMethod != nil {
		return nil, fmt.Errorf("//export_php:method directive at line %d is not followed by a function declaration", currentMethod.lineNumber)
	}

	return methods, scanner.Err()
}

func (cp *classParser) parseMethodSignature(className, signature string) (*phpClassMethod, error) {
	matches := methodSignatureRegex.FindStringSubmatch(signature)

	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid method signature format")
	}

	methodName := matches[1]
	paramsStr := strings.TrimSpace(matches[2])
	returnTypeStr := strings.TrimSpace(matches[3])

	isReturnNullable := strings.HasPrefix(returnTypeStr, "?")
	returnType := strings.TrimPrefix(returnTypeStr, "?")

	var params []phpParameter
	if paramsStr != "" {
		paramParts := strings.SplitSeq(paramsStr, ",")
		for part := range paramParts {
			param, err := cp.parseMethodParameter(strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("parsing parameter '%s': %w", part, err)
			}

			params = append(params, param)
		}
	}

	return &phpClassMethod{
		Name:             methodName,
		PhpName:          methodName,
		ClassName:        className,
		Signature:        signature,
		Params:           params,
		ReturnType:       phpType(returnType),
		isReturnNullable: isReturnNullable,
	}, nil
}

func (cp *classParser) parseMethodParameter(paramStr string) (phpParameter, error) {
	parts := strings.Split(paramStr, "=")
	typePart := strings.TrimSpace(parts[0])

	param := phpParameter{HasDefault: len(parts) > 1}

	if param.HasDefault {
		param.DefaultValue = cp.sanitizeDefaultValue(strings.TrimSpace(parts[1]))
	}

	matches := methodParamTypeNameRegex.FindStringSubmatch(typePart)

	if len(matches) < 3 {
		return phpParameter{}, fmt.Errorf("invalid parameter format: %s", paramStr)
	}

	typeStr := strings.TrimSpace(matches[1])
	param.Name = strings.TrimSpace(matches[2])
	param.IsNullable = strings.HasPrefix(typeStr, "?")
	param.PhpType = phpType(strings.TrimPrefix(typeStr, "?"))

	return param, nil
}

func (cp *classParser) sanitizeDefaultValue(value string) string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return value
	}

	if strings.ToLower(value) == "null" {
		return "null"
	}

	return strings.Trim(value, `'"`)
}

func (cp *classParser) extractGoMethodFunction(scanner *bufio.Scanner, firstLine string) (string, error) {
	goFunc := firstLine + "\n"
	braceCount := 1

	for scanner.Scan() {
		line := scanner.Text()
		goFunc += line + "\n"

		for _, char := range line {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
			}
		}

		if braceCount == 0 {
			break
		}
	}

	return goFunc, nil
}
