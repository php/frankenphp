package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var phpFuncRegex = regexp.MustCompile(`//\s*export_php:function\s+([^{}\n]+)(?:\s*{\s*})?`)

type FuncParser struct{}

func (fp *FuncParser) parse(filename string) (functions []phpFunction, err error) {
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
	var currentPHPFunc *phpFunction
	validator := Validator{}

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if matches := phpFuncRegex.FindStringSubmatch(line); matches != nil {
			signature := strings.TrimSpace(matches[1])
			phpFunc, err := fp.parseSignature(signature)
			if err != nil {
				fmt.Printf("Warning: Error parsing signature '%s': %v\n", signature, err)

				continue
			}

			if err := validator.validateFunction(*phpFunc); err != nil {
				fmt.Printf("Warning: Invalid function '%s': %v\n", phpFunc.Name, err)

				continue
			}

			if err := validator.validateTypes(*phpFunc); err != nil {
				fmt.Printf("Warning: Function '%s' uses unsupported types: %v\n", phpFunc.Name, err)

				continue
			}

			phpFunc.lineNumber = lineNumber
			currentPHPFunc = phpFunc
		}

		if currentPHPFunc != nil && strings.HasPrefix(line, "func ") {
			goFunc, err := fp.extractGoFunction(scanner, line)
			if err != nil {
				return nil, fmt.Errorf("extracting Go function: %w", err)
			}

			currentPHPFunc.GoFunction = goFunc

			if err := validator.validateGoFunctionSignatureWithOptions(*currentPHPFunc, false); err != nil {
				fmt.Printf("Warning: Go function signature mismatch for %q: %v\n", currentPHPFunc.Name, err)
				currentPHPFunc = nil

				continue
			}

			functions = append(functions, *currentPHPFunc)
			currentPHPFunc = nil
		}
	}

	if currentPHPFunc != nil {
		return nil, fmt.Errorf("//export_php function directive at line %d is not followed by a function declaration", currentPHPFunc.lineNumber)
	}

	return functions, scanner.Err()
}

func (fp *FuncParser) extractGoFunction(scanner *bufio.Scanner, firstLine string) (string, error) {
	goFunc := firstLine + "\n"
	braceCount := 1

	for scanner.Scan() {
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

	return goFunc, nil
}

func (fp *FuncParser) parseSignature(signature string) (*phpFunction, error) {
	name, params, returnType, nullable, err := parseSignatureParams(signature)
	if err != nil {
		return nil, err
	}

	return &phpFunction{
		Name:             name,
		Signature:        signature,
		Params:           params,
		ReturnType:       phpType(returnType),
		IsReturnNullable: nullable,
	}, nil
}
