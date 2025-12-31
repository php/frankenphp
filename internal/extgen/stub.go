package extgen

import (
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/stub.php.tpl
var templateContent string

type StubGenerator struct {
	Generator *Generator
}

func (sg *StubGenerator) generate() error {
	filename := filepath.Join(sg.Generator.BuildDir, sg.Generator.BaseName+".stub.php")
	content, err := sg.buildContent()
	if err != nil {
		return err
	}

	return writeFile(filename, content)
}

func (sg *StubGenerator) buildContent() (string, error) {
	tmpl, err := template.New("stub.php.tpl").Funcs(template.FuncMap{
		"phpType": getPhpTypeAnnotation,
	}).Parse(templateContent)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, sg.Generator); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getPhpTypeAnnotation converts phpType to PHP type annotation
func getPhpTypeAnnotation(t phpType) string {
	switch t {
	case phpString:
		return "string"
	case phpBool:
		return "bool"
	case phpFloat:
		return "float"
	case phpInt:
		return "int"
	case phpArray:
		return "array"
	default:
		return "int"
	}
}
