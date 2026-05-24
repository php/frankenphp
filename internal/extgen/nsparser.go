package extgen

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type NamespaceParser struct{}

var namespaceRegex = regexp.MustCompile(`//\s*export_php:namespace\s+(.+)`)

func (np *NamespaceParser) parse(filename string) (foundNamespace string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	defer func() {
		err = errors.Join(err, file.Close())
	}()

	var lineNumber, foundLineNumber int

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if matches := namespaceRegex.FindStringSubmatch(line); matches != nil {
			namespace := strings.TrimSpace(matches[1])
			if foundNamespace != "" {
				return "", fmt.Errorf("multiple namespace declarations found: first at line %d, second at line %d", foundLineNumber, lineNumber)
			}

			foundNamespace = namespace
			foundLineNumber = lineNumber
		}
	}

	return foundNamespace, scanner.Err()
}
