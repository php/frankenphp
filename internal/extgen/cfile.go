package extgen

import (
	"github.com/Masterminds/sprig/v3"

	"bytes"
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/extension.c.tpl
var cFileContent string

type cFileGenerator struct {
	generator *Generator
}

type cTemplateData struct {
	BaseName  string
	Functions []phpFunction
	Classes   []phpClass
	Constants []phpConstant
	Namespace string
}

func (cg *cFileGenerator) generate() error {
	filename := filepath.Join(cg.generator.BuildDir, cg.generator.BaseName+".c")
	content, err := cg.buildContent()
	if err != nil {
		return err
	}

	return writeFile(filename, content)
}

func (cg *cFileGenerator) buildContent() (string, error) {
	var builder strings.Builder

	templateContent, err := cg.getTemplateContent()
	if err != nil {
		return "", err
	}
	builder.WriteString(templateContent)

	for _, fn := range cg.generator.Functions {
		fnGen := PHPFuncGenerator{
			paramParser: &ParameterParser{},
			namespace:   cg.generator.Namespace,
		}
		builder.WriteString(fnGen.generate(fn))
	}

	return builder.String(), nil
}

func (cg *cFileGenerator) getTemplateContent() (string, error) {
	funcMap := sprig.FuncMap()
	funcMap["namespacedClassName"] = NamespacedName
	funcMap["cString"] = escapeCString

	tmpl := template.Must(template.New("cfile").Funcs(funcMap).Parse(cFileContent))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cTemplateData{
		BaseName:  cg.generator.BaseName,
		Functions: cg.generator.Functions,
		Classes:   cg.generator.Classes,
		Constants: cg.generator.Constants,
		Namespace: cg.generator.Namespace,
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// escapeCString escapes backslashes for C string literals
func escapeCString(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}
