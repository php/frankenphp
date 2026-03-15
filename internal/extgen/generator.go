package extgen

import (
	"fmt"
	"os"
)

type Generator struct {
	BaseName   string
	SourceFile string
	BuildDir   string
	Functions  []phpFunction
	Classes    []phpClass
	Constants  []phpConstant
	Namespace  string
}

// EXPERIMENTAL
func (g *Generator) Generate() error {
	if err := g.setupBuildDirectory(); err != nil {
		return fmt.Errorf("setup build directory: %w", err)
	}
	if err := g.parseSource(); err != nil {
		return fmt.Errorf("parse source: %w", err)
	}

	if len(g.Functions) == 0 && len(g.Classes) == 0 && len(g.Constants) == 0 {
		return fmt.Errorf("no PHP functions, classes, or constants found in source file")
	}

	generators := []struct {
		name string
		fn   func() error
	}{
		{"stub file", g.generateStubFile},
		{"arginfo", g.generateArginfo},
		{"header file", g.generateHeaderFile},
		{"C file", g.generateCFile},
		{"Go file", g.generateGoFile},
		{"documentation", g.generateDocumentation},
	}

	for _, gen := range generators {
		if err := gen.fn(); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) setupBuildDirectory() error {
	return os.MkdirAll(g.BuildDir, 0755)
}

func (g *Generator) parseSource() error {
	parser := SourceParser{}

	functions, err := parser.ParseFunctions(g.SourceFile)
	if err != nil {
		return fmt.Errorf("parsing functions: %w", err)
	}
	g.Functions = functions

	classes, err := parser.ParseClasses(g.SourceFile)
	if err != nil {
		return fmt.Errorf("parsing classes: %w", err)
	}
	g.Classes = classes

	constants, err := parser.ParseConstants(g.SourceFile)
	if err != nil {
		return fmt.Errorf("parsing constants: %w", err)
	}
	g.Constants = constants

	ns, err := parser.ParseNamespace(g.SourceFile)
	if err != nil {
		return fmt.Errorf("parsing namespace: %w", err)
	}
	g.Namespace = ns

	return nil
}

func (g *Generator) generateStubFile() error {
	generator := StubGenerator{g}
	if err := generator.generate(); err != nil {
		return &GeneratorError{"stub generation", "failed to generate stub file", err}
	}

	return nil
}

func (g *Generator) generateArginfo() error {
	generator := arginfoGenerator{generator: g}
	if err := generator.generate(); err != nil {
		return &GeneratorError{"arginfo generation", "failed to generate arginfo", err}
	}

	return nil
}

func (g *Generator) generateHeaderFile() error {
	generator := HeaderGenerator{g}
	if err := generator.generate(); err != nil {
		return &GeneratorError{"header generation", "failed to generate header file", err}
	}

	return nil
}

func (g *Generator) generateCFile() error {
	generator := cFileGenerator{g}
	if err := generator.generate(); err != nil {
		return &GeneratorError{"C file generation", "failed to generate C file", err}
	}

	return nil
}

func (g *Generator) generateGoFile() error {
	generator := GoFileGenerator{g}
	if err := generator.generate(); err != nil {
		return &GeneratorError{"Go file generation", "failed to generate Go file", err}
	}

	return nil
}

func (g *Generator) generateDocumentation() error {
	docGen := DocumentationGenerator{g}
	if err := docGen.generate(); err != nil {
		return &GeneratorError{"documentation generation", "failed to generate documentation", err}
	}

	return nil
}
