package extgen

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// findDirective searches a comment group for a line matching re and returns the
// first capture group (typically the directive payload) along with the comment's
// source line number. Returns "" when no comment matches.
func findDirective(group *ast.CommentGroup, fset *token.FileSet, re *regexp.Regexp) (string, int) {
	if group == nil {
		return "", 0
	}
	for _, comment := range group.List {
		if matches := re.FindStringSubmatch(comment.Text); matches != nil {
			return strings.TrimSpace(matches[1]), fset.Position(comment.Pos()).Line
		}
	}
	return "", 0
}

// extractNodeSource returns the verbatim source text covered by node in src.
func extractNodeSource(src []byte, fset *token.FileSet, node ast.Node) string {
	start := fset.Position(node.Pos()).Offset
	end := fset.Position(node.End()).Offset
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

// flattenParamTypes expands a parameter list into one entry per parameter.
// A single ast.Field can declare several parameters at once ("func f(a, b int)"),
// so the field list length is not the parameter count.
func flattenParamTypes(params *ast.FieldList) []ast.Expr {
	if params == nil {
		return nil
	}

	var types []ast.Expr
	for _, field := range params.List {
		for range max(len(field.Names), 1) {
			types = append(types, field.Type)
		}
	}

	return types
}

// checkOrphanDirectives returns an error for the first comment that matches re
// but whose source line was not consumed by a declaration.
func checkOrphanDirectives(file *ast.File, fset *token.FileSet, re *regexp.Regexp, consumed map[int]bool, directiveLabel string) error {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !re.MatchString(comment.Text) {
				continue
			}
			line := fset.Position(comment.Pos()).Line
			if !consumed[line] {
				return fmt.Errorf("%s directive at line %d is not followed by a function declaration", directiveLabel, line)
			}
		}
	}
	return nil
}
