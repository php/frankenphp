package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var phpFuncRegex = regexp.MustCompile(`//\s*export_php:function\s+([^{}\n]+)(?:\s*{\s*})?`)
var signatureRegex = regexp.MustCompile(`(\w+)\s*\(([^)]*)\)\s*:\s*(\??[\w|]+)`)
var typeNameRegex = regexp.MustCompile(`(\??[\w|]+)\s+\$?(\w+)`)

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

			if err := validator.validateScalarTypes(*phpFunc); err != nil {
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
	matches := signatureRegex.FindStringSubmatch(signature)

	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid signature format")
	}

	name := matches[1]
	paramsStr := strings.TrimSpace(matches[2])
	returnTypeStr := strings.TrimSpace(matches[3])

	isReturnNullable := strings.HasPrefix(returnTypeStr, "?")
	returnType := strings.TrimPrefix(returnTypeStr, "?")

	var params []phpParameter
	if paramsStr != "" {
		paramParts := strings.SplitSeq(paramsStr, ",")
		for part := range paramParts {
			param, err := fp.parseParameter(strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("parsing parameter '%s': %w", part, err)
			}
			params = append(params, param)
		}
	}

	return &phpFunction{
		Name:             name,
		Signature:        signature,
		Params:           params,
		ReturnType:       phpType(returnType),
		IsReturnNullable: isReturnNullable,
	}, nil
}

func (fp *FuncParser) parseParameter(paramStr string) (phpParameter, error) {
	parts := strings.Split(paramStr, "=")
	typePart := strings.TrimSpace(parts[0])

	param := phpParameter{HasDefault: len(parts) > 1}

	if param.HasDefault {
		param.DefaultValue = fp.sanitizeDefaultValue(strings.TrimSpace(parts[1]))
	}

	matches := typeNameRegex.FindStringSubmatch(typePart)

	if len(matches) < 3 {
		return phpParameter{}, fmt.Errorf("invalid parameter format: %s", paramStr)
	}

	typeStr := strings.TrimSpace(matches[1])
	param.Name = strings.TrimSpace(matches[2])
	param.IsNullable = strings.HasPrefix(typeStr, "?")
	param.PhpType = phpType(strings.TrimPrefix(typeStr, "?"))

	return param, nil
}

func (fp *FuncParser) sanitizeDefaultValue(value string) string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return value
	}
	if strings.ToLower(value) == "null" {
		return "null"
	}

	return strings.Trim(value, `'"`)
}
