package extgen

import (
	"os"
	"strings"
	"unicode"
)

func writeFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func readFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// NamespacedName converts a namespace and name to a C-compatible format.
// E.g., namespace "Go\Extension" and name "MyClass" become "Go_Extension_MyClass".
// This symbol remains exported, so it's usable in templates.
func NamespacedName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	namespacePart := strings.ReplaceAll(namespace, `\`, "_")
	return namespacePart + "_" + name
}

// EXPERIMENTAL
func SanitizePackageName(name string) string {
	sanitized := strings.ReplaceAll(name, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")

	if len(sanitized) > 0 && !unicode.IsLetter(rune(sanitized[0])) && sanitized[0] != '_' {
		sanitized = "_" + sanitized
	}

	return sanitized
}
