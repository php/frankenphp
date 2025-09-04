package extgen

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
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
	Variables         []string
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

	return writeFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	imports, variables, internalFunctions, err := sourceAnalyzer.analyze(gg.generator.SourceFile)
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

	templateContent, err := gg.getTemplateContent(goTemplateData{
		PackageName:       SanitizePackageName(gg.generator.BaseName),
		BaseName:          gg.generator.BaseName,
		Imports:           filteredImports,
		Constants:         gg.generator.Constants,
		Variables:         variables,
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
	funcMap := sprig.FuncMap()
	funcMap["phpTypeToGoType"] = gg.phpTypeToGoType
	funcMap["isStringOrArray"] = func(t phpType) bool {
		return t == phpString || t == phpArray
	}
	funcMap["isVoid"] = func(t phpType) bool {
		return t == phpVoid
	}

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

func (gg *GoFileGenerator) phpTypeToGoType(phpT phpType) string {
	typeMap := map[phpType]string{
		phpString: "string",
		phpInt:    "int64",
		phpFloat:  "float64",
		phpBool:   "bool",
		phpArray:  "*frankenphp.Array",
		phpMixed:  "interface{}",
		phpVoid:   "",
	}

	if goType, exists := typeMap[phpT]; exists {
		return goType
	}

	return "interface{}"
}
