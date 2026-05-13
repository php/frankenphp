package extgen

import (
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
				fmt.Fprintf(os.Stderr, "Warning: Invalid class '%s': %v\n", class.Name, err)
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
		return "any"
	}
}

var goToPhpTypeMap = map[string]phpType{
	"string": phpString,
	"int":    phpInt, "int64": phpInt, "int32": phpInt, "int16": phpInt, "int8": phpInt,
	"uint": phpInt, "uint64": phpInt, "uint32": phpInt, "uint16": phpInt, "uint8": phpInt,
	"float64": phpFloat, "float32": phpFloat,
	"bool": phpBool,
	"any":  phpMixed,
}

func (cp *classParser) goTypeToPHPType(goType string) phpType {
	goType = strings.TrimPrefix(goType, "*")

	if phpType, exists := goToPhpTypeMap[goType]; exists {
		return phpType
	}

	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") {
		return phpArray
	}

	return phpMixed
}

func (cp *classParser) parseMethods(filename string) ([]phpClassMethod, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	validator := Validator{}
	var methods []phpClassMethod
	consumed := make(map[int]bool)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		directive, directiveLine := findDirective(funcDecl.Doc, fset, phpMethodRegex)
		if directive == "" {
			continue
		}
		rawMatch := phpMethodRegex.FindStringSubmatch(findMatchingComment(funcDecl.Doc, phpMethodRegex))
		if len(rawMatch) != 3 {
			continue
		}
		className := strings.TrimSpace(rawMatch[1])
		signature := strings.TrimSpace(rawMatch[2])
		consumed[directiveLine] = true

		method, err := cp.parseMethodSignature(className, signature)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error parsing method signature %q: %v\n", signature, err)
			continue
		}

		phpFunc := phpFunction{
			Name:             method.Name,
			Signature:        method.Signature,
			Params:           method.Params,
			ReturnType:       method.ReturnType,
			IsReturnNullable: method.isReturnNullable,
		}
		if err := validator.validateTypes(phpFunc); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Method \"%s::%s\" uses unsupported types: %v\n", className, method.Name, err)
			continue
		}

		method.lineNumber = directiveLine
		method.GoFunction = extractNodeSource(src, fset, funcDecl)

		phpFunc.GoFunction = method.GoFunction
		if err := validator.validateGoFunctionSignatureWithOptions(phpFunc, true); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Go method signature mismatch for '%s::%s': %v\n", method.ClassName, method.Name, err)
			continue
		}

		methods = append(methods, *method)
	}

	if err := checkOrphanDirectives(file, fset, phpMethodRegex, consumed, "//export_php:method"); err != nil {
		return nil, err
	}

	return methods, nil
}

// findMatchingComment returns the raw comment text whose line matches re.
func findMatchingComment(group *ast.CommentGroup, re *regexp.Regexp) string {
	if group == nil {
		return ""
	}
	for _, comment := range group.List {
		if re.MatchString(comment.Text) {
			return comment.Text
		}
	}
	return ""
}

func (cp *classParser) parseMethodSignature(className, signature string) (*phpClassMethod, error) {
	name, params, returnType, nullable, err := parseSignatureParams(signature)
	if err != nil {
		return nil, err
	}

	return &phpClassMethod{
		Name:             name,
		PhpName:          name,
		ClassName:        className,
		Signature:        signature,
		Params:           params,
		ReturnType:       phpType(returnType),
		isReturnNullable: nullable,
	}, nil
}
