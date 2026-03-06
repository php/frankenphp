// header.go
package extgen

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/extension.h.tpl
var hFileContent string

type HeaderGenerator struct {
	generator *Generator
}

type TemplateData struct {
	BaseName    string
	HeaderGuard string
	Constants   []phpConstant
	Classes     []phpClass
}

func (hg *HeaderGenerator) generate() error {
	filename := filepath.Join(hg.generator.BuildDir, hg.generator.BaseName+".h")
	content, err := hg.buildContent()
	if err != nil {
		return err
	}

	return writeFile(filename, content)
}

func (hg *HeaderGenerator) buildContent() (string, error) {
	headerGuard := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}

		return '_'
	}, hg.generator.BaseName)

	headerGuard = strings.ToUpper(headerGuard) + "_H"

	tmpl, err := template.New("header").Parse(hFileContent)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, TemplateData{
		BaseName:    hg.generator.BaseName,
		HeaderGuard: headerGuard,
		Constants:   hg.generator.Constants,
		Classes:     hg.generator.Classes,
	})

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
