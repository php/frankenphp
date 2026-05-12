package extgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
)

var phpFuncRegex = regexp.MustCompile(`//\s*export_php:function\s+([^{}\n]+)(?:\s*{\s*})?`)

var warnOut io.Writer = os.Stdout

func warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(warnOut, format, args...)
}

type FuncParser struct{}

func (fp *FuncParser) parse(filename string) ([]phpFunction, error) {
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
	var functions []phpFunction
	consumed := make(map[int]bool)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil {
			continue
		}

		directive, directiveLine := findDirective(funcDecl.Doc, fset, phpFuncRegex)
		if directive == "" {
			continue
		}
		consumed[directiveLine] = true

		phpFunc, err := fp.parseSignature(directive)
		if err != nil {
			warnf("Warning: Error parsing signature '%s': %v\n", directive, err)
			continue
		}

		if err := validator.validateFunction(*phpFunc); err != nil {
			warnf("Warning: Invalid function '%s': %v\n", phpFunc.Name, err)
			continue
		}

		if err := validator.validateTypes(*phpFunc); err != nil {
			warnf("Warning: Function '%s' uses unsupported types: %v\n", phpFunc.Name, err)
			continue
		}

		phpFunc.lineNumber = directiveLine
		phpFunc.GoFunction = extractNodeSource(src, fset, funcDecl)

		if err := validator.validateGoFunctionSignatureWithOptions(*phpFunc, false); err != nil {
			warnf("Warning: Go function signature mismatch for %q: %v\n", phpFunc.Name, err)
			continue
		}

		functions = append(functions, *phpFunc)
	}

	if err := checkOrphanDirectives(file, fset, phpFuncRegex, consumed, "//export_php:function"); err != nil {
		return nil, err
	}

	return functions, nil
}

func (fp *FuncParser) parseSignature(signature string) (*phpFunction, error) {
	name, params, returnType, nullable, err := parseSignatureParams(signature)
	if err != nil {
		return nil, err
	}

	return &phpFunction{
		Name:             name,
		Signature:        signature,
		Params:           params,
		ReturnType:       phpType(returnType),
		IsReturnNullable: nullable,
	}, nil
}
