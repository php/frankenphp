package extgen

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed templates/extension.go.tpl
var goFileContent string

type GoFileGenerator struct {
	generator *Generator
}

type goTemplateData struct {
	PackageName       string
	BaseName          string
	SanitizedBaseName string
	Constants         []phpConstant
	Variables         []string
	InternalFunctions []string
	Functions         []phpFunction
	Classes           []phpClass
}

func (gg *GoFileGenerator) generate() error {
	filename := filepath.Join(gg.generator.BuildDir, gg.generator.BaseName+"_generated.go")

	content, err := gg.buildContent()
	if err != nil {
		return fmt.Errorf("building Go file content: %w", err)
	}

	return writeFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	packageName, variables, internalFunctions, err := sourceAnalyzer.analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	classes := make([]phpClass, len(gg.generator.Classes))
	copy(classes, gg.generator.Classes)

	templateContent, err := gg.getTemplateContent(goTemplateData{
		PackageName:       packageName,
		BaseName:          gg.generator.BaseName,
		SanitizedBaseName: SanitizePackageName(gg.generator.BaseName),
		Constants:         gg.generator.Constants,
		Variables:         variables,
		InternalFunctions: internalFunctions,
		Functions:         gg.generator.Functions,
		Classes:           classes,
	})

	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	fc, err := format.Source([]byte(templateContent))
	if err != nil {
		return "", fmt.Errorf("formatting source: %w", err)
	}

	return string(fc), nil
}

func (gg *GoFileGenerator) getTemplateContent(data goTemplateData) (string, error) {
	funcMap := sprig.FuncMap()
	funcMap["phpTypeToGoType"] = gg.phpTypeToGoType
	funcMap["isStringOrArray"] = func(t phpType) bool {
		return t == phpString || t == phpArray
	}
	funcMap["isVoid"] = func(t phpType) bool {
		return t == phpVoid
	}
	funcMap["extractGoFunctionName"] = extractGoFunctionName
	funcMap["extractGoFunctionSignatureParams"] = extractGoFunctionSignatureParams
	funcMap["extractGoFunctionSignatureReturn"] = extractGoFunctionSignatureReturn
	funcMap["extractGoFunctionCallParams"] = extractGoFunctionCallParams

	tmpl := template.Must(template.New("gofile").Funcs(funcMap).Parse(goFileContent))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type GoMethodSignature struct {
	MethodName string
	Params     []GoParameter
	ReturnType string
}

type GoParameter struct {
	Name string
	Type string
}

var phpToGoTypeMap = map[phpType]string{
	phpString:   "string",
	phpInt:      "int64",
	phpFloat:    "float64",
	phpBool:     "bool",
	phpArray:    "*frankenphp.Array",
	phpMixed:    "any",
	phpVoid:     "",
	phpCallable: "*C.zval",
}

func (gg *GoFileGenerator) phpTypeToGoType(phpT phpType) string {
	if goType, exists := phpToGoTypeMap[phpT]; exists {
		return goType
	}

	return "any"
}

// extractGoFunctionName extracts the Go function name from a Go function signature string.
func extractGoFunctionName(goFunction string) string {
	idx := strings.Index(goFunction, "func ")
	if idx == -1 {
		return ""
	}

	start := idx + len("func ")

	end := start
	for end < len(goFunction) && goFunction[end] != '(' {
		end++
	}

	if end >= len(goFunction) {
		return ""
	}

	return strings.TrimSpace(goFunction[start:end])
}

// extractGoFunctionSignatureParams extracts the parameters from a Go function signature.
func extractGoFunctionSignatureParams(goFunction string) string {
	start := strings.IndexByte(goFunction, '(')
	if start == -1 {
		return ""
	}
	start++

	depth := 1
	end := start
	for end < len(goFunction) && depth > 0 {
		switch goFunction[end] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth > 0 {
			end++
		}
	}

	if end >= len(goFunction) {
		return ""
	}

	return strings.TrimSpace(goFunction[start:end])
}

// extractGoFunctionSignatureReturn extracts the return type from a Go function signature.
func extractGoFunctionSignatureReturn(goFunction string) string {
	start := strings.IndexByte(goFunction, '(')
	if start == -1 {
		return ""
	}

	depth := 1
	pos := start + 1
	for pos < len(goFunction) && depth > 0 {
		switch goFunction[pos] {
		case '(':
			depth++
		case ')':
			depth--
		}
		pos++
	}

	if pos >= len(goFunction) {
		return ""
	}

	end := strings.IndexByte(goFunction[pos:], '{')
	if end == -1 {
		return ""
	}
	end += pos

	returnType := strings.TrimSpace(goFunction[pos:end])
	return returnType
}

// extractGoFunctionCallParams extracts just the parameter names for calling a function.
func extractGoFunctionCallParams(goFunction string) string {
	params := extractGoFunctionSignatureParams(goFunction)
	if params == "" {
		return ""
	}

	var names []string
	parts := strings.Split(params, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 {
			continue
		}

		words := strings.Fields(part)
		if len(words) > 0 {
			names = append(names, words[0])
		}
	}

	var result strings.Builder
	for i, name := range names {
		if i > 0 {
			result.WriteString(", ")
		}

		result.WriteString(name)
	}

	return result.String()
}
