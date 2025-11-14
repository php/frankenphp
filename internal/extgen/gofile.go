package extgen

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
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

	if _, err := os.Stat(filename); err == nil {
		backupFilename := filename + ".bak"
		if err := os.Rename(filename, backupFilename); err != nil {
			return fmt.Errorf("backing up existing Go file: %w", err)
		}

		gg.generator.SourceFile = backupFilename
	}

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
		if imp != `"C"` && imp != `"unsafe"` && imp != `"github.com/dunglas/frankenphp"` && imp != `"runtime/cgo"` {
			filteredImports = append(filteredImports, imp)
		}
	}

	classes := make([]phpClass, len(gg.generator.Classes))
	copy(classes, gg.generator.Classes)

	if len(classes) > 0 {
		hasCgo := false
		for _, imp := range imports {
			if imp == `"runtime/cgo"` {
				hasCgo = true
				break
			}
		}
		if !hasCgo {
			filteredImports = append(filteredImports, `"runtime/cgo"`)
		}
	}

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

var phpToGoTypeMap = map[phpType]string{
	phpString: "string",
	phpInt:    "int64",
	phpFloat:  "float64",
	phpBool:   "bool",
	phpArray:  "*frankenphp.Array",
	phpMixed:  "any",
	phpVoid:   "",
}

func (gg *GoFileGenerator) phpTypeToGoType(phpT phpType) string {
	if goType, exists := phpToGoTypeMap[phpT]; exists {
		return goType
	}

	return "any"
}
