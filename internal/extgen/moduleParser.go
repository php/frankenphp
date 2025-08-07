package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var phpModuleParser = regexp.MustCompile(`//\s*export_php:module\s+(init|shutdown)`)

// phpModule represents a PHP module with optional init and shutdown functions
type phpModule struct {
	InitFunc     string // Name of the init function
	InitCode     string // Code of the init function
	ShutdownFunc string // Name of the shutdown function
	ShutdownCode string // Code of the shutdown function
}

// ModuleParser parses PHP module directives from Go source files
type ModuleParser struct{}

// parse parses the source file for PHP module directives
func (mp *ModuleParser) parse(filename string) (module *phpModule, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		e := file.Close()
		if err == nil {
			err = e
		}
	}()

	scanner := bufio.NewScanner(file)
	module = &phpModule{}
	var currentDirective string
	var lineNumber int

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if matches := phpModuleParser.FindStringSubmatch(line); matches != nil {
			directiveType := matches[1]
			currentDirective = directiveType
			continue
		}

		// If we have a current directive and encounter a non-comment line
		// that doesn't start with "func ", reset the current directive
		if currentDirective != "" && (line == "" || (!strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "func "))) {
			currentDirective = ""
			continue
		}

		if currentDirective != "" && strings.HasPrefix(line, "func ") {
			funcName, funcCode, err := mp.extractGoFunction(scanner, line)
			if err != nil {
				return nil, fmt.Errorf("extracting Go function at line %d: %w", lineNumber, err)
			}

			switch currentDirective {
			case "init":
				module.InitFunc = funcName
				module.InitCode = funcCode
			case "shutdown":
				module.ShutdownFunc = funcName
				module.ShutdownCode = funcCode
			}

			currentDirective = ""
		}
	}

	// If we found no module functions, return nil
	if module.InitFunc == "" && module.ShutdownFunc == "" {
		return nil, nil
	}

	return module, scanner.Err()
}

// extractGoFunction extracts the function name and code from a function declaration
func (mp *ModuleParser) extractGoFunction(scanner *bufio.Scanner, firstLine string) (string, string, error) {
	// Extract function name from the first line
	funcNameRegex := regexp.MustCompile(`func\s+([a-zA-Z0-9_]+)`)
	matches := funcNameRegex.FindStringSubmatch(firstLine)
	if len(matches) < 2 {
		return "", "", fmt.Errorf("could not extract function name from line: %s", firstLine)
	}
	funcName := matches[1]

	// Collect the function code
	goFunc := firstLine + "\n"
	braceCount := 0

	// Count opening braces in the first line
	for _, char := range firstLine {
		if char == '{' {
			braceCount++
		}
	}

	// Continue reading until we find the matching closing brace
	for braceCount > 0 && scanner.Scan() {
		line := scanner.Text()
		goFunc += line + "\n"

		for _, char := range line {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
			}
		}

		if braceCount == 0 {
			break
		}
	}

	return funcName, goFunc, nil
}
