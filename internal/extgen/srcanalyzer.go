package extgen

import (
	"fmt"
	"go/parser"
	"go/token"
)

type SourceAnalyzer struct{}

func (sa *SourceAnalyzer) analyze(filename string) (packageName string, err error) {
	node, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("parsing file: %w", err)
	}

	return node.Name.Name, nil
}
