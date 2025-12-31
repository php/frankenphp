package extgen

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var constRegex = regexp.MustCompile(`//\s*export_php:const$`)
var classConstRegex = regexp.MustCompile(`//\s*export_php:classconst\s+(\w+)$`)
var constDeclRegex = regexp.MustCompile(`const\s+(\w+)\s*=\s*(.+)`)

type ConstantParser struct{}

func (cp *ConstantParser) parse(filename string) (constants []phpConstant, err error) {
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

	lineNumber := 0
	expectConstDecl := false
	expectClassConstDecl := false
	currentClassName := ""
	currentConstantValue := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if constRegex.MatchString(line) {
			expectConstDecl = true
			expectClassConstDecl = false
			currentClassName = ""

			continue
		}

		if matches := classConstRegex.FindStringSubmatch(line); len(matches) == 2 {
			expectClassConstDecl = true
			expectConstDecl = false
			currentClassName = matches[1]

			continue
		}

		if (expectConstDecl || expectClassConstDecl) && strings.HasPrefix(line, "const ") {
			matches := constDeclRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				name := matches[1]
				value := strings.TrimSpace(matches[2])

				constant := phpConstant{
					Name:       name,
					Value:      value,
					IsIota:     value == "iota",
					lineNumber: lineNumber,
					ClassName:  currentClassName,
				}

				constant.PhpType = determineConstantType(value)

				if constant.IsIota {
					// affect a default value because user didn't give one
					constant.Value = fmt.Sprintf("%d", currentConstantValue)
					constant.PhpType = phpInt
					currentConstantValue++
				}

				constants = append(constants, constant)
			} else {
				return nil, fmt.Errorf("invalid constant declaration at line %d: %s", lineNumber, line)
			}
			expectConstDecl = false
			expectClassConstDecl = false
		} else if (expectConstDecl || expectClassConstDecl) && !strings.HasPrefix(line, "//") && line != "" {
			// we expected a const declaration but found something else, reset
			expectConstDecl = false
			expectClassConstDecl = false
			currentClassName = ""
		}
	}

	return constants, scanner.Err()
}

// determineConstantType analyzes the value and determines its type
func determineConstantType(value string) phpType {
	value = strings.TrimSpace(value)

	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, "`") && strings.HasSuffix(value, "`")) {
		return phpString
	}

	if value == "true" || value == "false" {
		return phpBool
	}

	// check for integer literals, including hex, octal, binary
	if _, err := strconv.ParseInt(value, 0, 64); err == nil {
		return phpInt
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return phpFloat
	}

	return phpInt
}
