package extgen

import (
	"fmt"
	"regexp"
	"strings"
)

// Shared patterns for both function and method signatures.
var (
	signaturePattern = regexp.MustCompile(`(\w+)\s*\(([^)]*)\)\s*:\s*(\??[\w|]+)`)
	paramPattern     = regexp.MustCompile(`(\??[\w|]+)\s+\$?(\w+)`)
)

// parseSignatureParams splits a "name(params): returnType" signature into its parts.
// Returns name, slice of parameters, return type (without leading "?") and whether it was nullable.
func parseSignatureParams(signature string) (name string, params []phpParameter, returnType string, nullable bool, err error) {
	matches := signaturePattern.FindStringSubmatch(signature)
	if len(matches) != 4 {
		return "", nil, "", false, fmt.Errorf("invalid signature format")
	}

	name = matches[1]
	paramsStr := strings.TrimSpace(matches[2])
	returnTypeStr := strings.TrimSpace(matches[3])

	nullable = strings.HasPrefix(returnTypeStr, "?")
	returnType = strings.TrimPrefix(returnTypeStr, "?")

	if paramsStr != "" {
		for part := range strings.SplitSeq(paramsStr, ",") {
			param, perr := parseParameter(strings.TrimSpace(part))
			if perr != nil {
				return "", nil, "", false, fmt.Errorf("parsing parameter '%s': %w", part, perr)
			}
			params = append(params, param)
		}
	}

	return name, params, returnType, nullable, nil
}

// parseParameter parses a single PHP parameter declaration like "?int $name = 42".
func parseParameter(paramStr string) (phpParameter, error) {
	parts := strings.SplitN(paramStr, "=", 2)
	typePart := strings.TrimSpace(parts[0])

	param := phpParameter{HasDefault: len(parts) > 1}
	if param.HasDefault {
		param.DefaultValue = sanitizeDefaultValue(strings.TrimSpace(parts[1]))
	}

	matches := paramPattern.FindStringSubmatch(typePart)
	if len(matches) < 3 {
		return phpParameter{}, fmt.Errorf("invalid parameter format: %s", paramStr)
	}

	typeStr := strings.TrimSpace(matches[1])
	param.Name = strings.TrimSpace(matches[2])
	param.IsNullable = strings.HasPrefix(typeStr, "?")
	param.PhpType = phpType(strings.TrimPrefix(typeStr, "?"))

	return param, nil
}

// sanitizeDefaultValue normalizes a PHP default value literal: keeps array literals,
// preserves "null", and strips surrounding quotes for scalar strings.
func sanitizeDefaultValue(value string) string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return value
	}
	if strings.EqualFold(value, "null") {
		return "null"
	}
	return strings.Trim(value, `'"`)
}
