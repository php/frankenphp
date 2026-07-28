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
