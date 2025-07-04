package extgen

import (
	"bytes"
	_ "embed"
	"fmt"
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
	Imports           []string
	Constants         []phpConstant
	InternalFunctions []string
	Functions         []phpFunction
	Classes           []phpClass
}

func (gg *GoFileGenerator) generate() error {
	filename := filepath.Join(gg.generator.BuildDir, gg.generator.BaseName+".go")
	content, err := gg.buildContent()
	if err != nil {
		return fmt.Errorf("building Go file content: %w", err)
	}

	return WriteFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	imports, internalFunctions, err := sourceAnalyzer.analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	filteredImports := make([]string, 0, len(imports))
	for _, imp := range imports {
		if imp != `"C"` {
			filteredImports = append(filteredImports, imp)
		}
	}

	classes := make([]phpClass, len(gg.generator.Classes))
	copy(classes, gg.generator.Classes)
	for i, class := range classes {
		for j, method := range class.Methods {
			classes[i].Methods[j].Wrapper = gg.generateMethodWrapper(method, class)
		}
	}

	templateContent, err := gg.getTemplateContent(goTemplateData{
		PackageName:       SanitizePackageName(gg.generator.BaseName),
		BaseName:          gg.generator.BaseName,
		Imports:           filteredImports,
		Constants:         gg.generator.Constants,
		InternalFunctions: internalFunctions,
		Functions:         gg.generator.Functions,
		Classes:           classes,
	})

	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return templateContent, nil
}

func (gg *GoFileGenerator) getTemplateContent(data goTemplateData) (string, error) {
	tmpl := template.Must(template.New("gofile").Funcs(sprig.FuncMap()).Parse(goFileContent))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (gg *GoFileGenerator) generateMethodWrapper(method phpClassMethod, class phpClass) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("func %s_wrapper(handle C.uintptr_t", method.Name))

	for _, param := range method.Params {
		if param.PhpType == "string" {
			builder.WriteString(fmt.Sprintf(", %s *C.zend_string", param.Name))
			continue
		}

		if param.PhpType == "array" {
			builder.WriteString(fmt.Sprintf(", %s *C.zval", param.Name))
			continue
		}

		goType := gg.phpTypeToGoType(param.PhpType)
		if param.IsNullable {
			goType = "*" + goType
		}
		builder.WriteString(fmt.Sprintf(", %s %s", param.Name, goType))
	}

	if method.ReturnType != "void" {
		if method.ReturnType == "string" || method.ReturnType == "array" {
			builder.WriteString(") unsafe.Pointer {\n")
		} else {
			goReturnType := gg.phpTypeToGoType(method.ReturnType)
			builder.WriteString(fmt.Sprintf(") %s {\n", goReturnType))
		}
	} else {
		builder.WriteString(") {\n")
	}

	builder.WriteString("	obj := getGoObject(handle)\n")
	builder.WriteString("	if obj == nil {\n")
	if method.ReturnType != "void" {
		if method.ReturnType == "string" || method.ReturnType == "array" {
			builder.WriteString("		return nil\n")
		} else {
			builder.WriteString(fmt.Sprintf("		var zero %s\n", gg.phpTypeToGoType(method.ReturnType)))
			builder.WriteString("		return zero\n")
		}
	} else {
		builder.WriteString("		return\n")
	}
	builder.WriteString("	}\n")
	builder.WriteString(fmt.Sprintf("	structObj := obj.(*%s)\n", class.GoStruct))

	builder.WriteString("	")
	if method.ReturnType != "void" {
		builder.WriteString("return ")
	}

	builder.WriteString(fmt.Sprintf("structObj.%s(", gg.goMethodName(method.Name)))

	for i, param := range method.Params {
		if i > 0 {
			builder.WriteString(", ")
		}

		builder.WriteString(param.Name)
	}

	builder.WriteString(`)
}`)

	return builder.String()
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

func (gg *GoFileGenerator) phpTypeToGoType(phpType string) string {
	typeMap := map[string]string{
		"string": "string",
		"int":    "int64",
		"float":  "float64",
		"bool":   "bool",
		"array":  "*frankenphp.Array",
		"mixed":  "interface{}",
		"void":   "",
	}

	if goType, exists := typeMap[phpType]; exists {
		return goType
	}

	return "interface{}"
}

func (gg *GoFileGenerator) goMethodName(phpMethodName string) string {
	if len(phpMethodName) == 0 {
		return phpMethodName
	}

	return strings.ToUpper(phpMethodName[:1]) + phpMethodName[1:]
}
