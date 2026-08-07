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
	packageName, err := sourceAnalyzer.analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	templateContent, err := gg.getTemplateContent(goTemplateData{
		PackageName:       packageName,
		BaseName:          gg.generator.BaseName,
		SanitizedBaseName: SanitizePackageName(gg.generator.BaseName),
		Functions:         gg.generator.Functions,
		Classes:           gg.generator.Classes,
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
	// Reuse the validator's mapping so the signatures the generator emits cannot
	// drift from the ones it accepts.
	validator := &Validator{}
	funcMap["goParamType"] = validator.phpTypeToGoType
	funcMap["goReturnType"] = validator.phpReturnTypeToGoType
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

// extractGoFunctionName extracts the Go function or method name from a Go
// function signature string.
func extractGoFunctionName(goFunction string) string {
	idx := strings.Index(goFunction, "func ")
	if idx == -1 {
		return ""
	}

	rest := strings.TrimLeft(goFunction[idx+len("func "):], " \t")
	if strings.HasPrefix(rest, "(") {
		// method: skip the receiver so the name, not the receiver, is returned
		closing := strings.IndexByte(rest, ')')
		if closing == -1 {
			return ""
		}
		rest = rest[closing+1:]
	}

	end := strings.IndexByte(rest, '(')
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(rest[:end])
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
