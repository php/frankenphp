//go:build !nowatcher

package watcher

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/dunglas/frankenphp/internal/fastabs"
)

type watchPattern struct {
	dir          string
	patterns     []string
	trigger      chan string
	failureCount int
}

func parseFilePatterns(filePatterns []string) ([]*watchPattern, error) {
	watchPatterns := make([]*watchPattern, 0, len(filePatterns))
	for _, filePattern := range filePatterns {
		watchPattern, err := parseFilePattern(filePattern)
		if err != nil {
			return nil, err
		}
		watchPatterns = append(watchPatterns, watchPattern)
	}
	return watchPatterns, nil
}

// this method prepares the watchPattern struct for a single file pattern (aka /path/*pattern)
func parseFilePattern(filePattern string) (*watchPattern, error) {
	w := &watchPattern{}

	// first we clean the pattern
	absPattern, err := fastabs.FastAbs(filePattern)
	if err != nil {
		return nil, err
	}
	w.dir = absPattern

	// then we split the pattern to determine where the directory ends and the pattern starts
	splitPattern := strings.Split(absPattern, string(filepath.Separator))
	patternWithoutDir := ""
	for i, part := range splitPattern {
		isFilename := i == len(splitPattern)-1 && strings.Contains(part, ".")
		isGlobCharacter := strings.ContainsAny(part, "[*?{")
		if isFilename || isGlobCharacter {
			patternWithoutDir = filepath.Join(splitPattern[i:]...)
			w.dir = filepath.Join(splitPattern[:i]...)
			break
		}
	}

	// now we split the pattern according to the recursive '**' syntax
	w.patterns = strings.Split(patternWithoutDir, "**")
	for i, pattern := range w.patterns {
		w.patterns[i] = strings.Trim(pattern, string(filepath.Separator))
	}

	// finally, we remove the trailing separator and add leading separator
	w.dir = string(filepath.Separator) + strings.Trim(w.dir, string(filepath.Separator))

	return w, nil
}

func (watchPattern *watchPattern) allowReload(fileName string, eventType int, pathType int) bool {
	if !isValidEventType(eventType) || !isValidPathType(pathType, fileName) {
		return false
	}

	return isValidPattern(fileName, watchPattern.dir, watchPattern.patterns)
}

// 0:rename,1:modify,2:create,3:destroy,4:owner,5:other,
func isValidEventType(eventType int) bool {
	return eventType <= 3
}

// 0:dir,1:file,2:hard_link,3:sym_link,4:watcher,5:other,
func isValidPathType(pathType int, fileName string) bool {
	if pathType == 4 && logger.Enabled(ctx, slog.LevelDebug) {
		logger.LogAttrs(ctx, slog.LevelDebug, "special edant/watcher event", slog.String("fileName", fileName))
	}

	return pathType <= 2
}

func isValidPattern(fileName string, dir string, patterns []string) bool {
	// first we remove the dir from the pattern
	if !strings.HasPrefix(fileName, dir) {
		return false
	}

	// remove the dir and separator from the filename
	fileNameWithoutDir := strings.TrimPrefix(strings.TrimPrefix(fileName, dir), string(filepath.Separator))

	// if the pattern has size 1 we can match it directly against the filename
	if len(patterns) == 1 {
		return matchBracketPattern(patterns[0], fileNameWithoutDir)
	}

	return matchPatterns(patterns, fileNameWithoutDir)
}

func matchPatterns(patterns []string, fileName string) bool {
	partsToMatch := strings.Split(fileName, string(filepath.Separator))
	cursor := 0

	// if there are multiple patterns due to '**' we need to match them individually
	for i, pattern := range patterns {
		patternSize := strings.Count(pattern, string(filepath.Separator)) + 1

		// if we are at the last pattern we will start matching from the end of the filename
		if i == len(patterns)-1 {
			cursor = len(partsToMatch) - patternSize
		}

		// the cursor will move through the fileName until the pattern matches
		for j := cursor; j < len(partsToMatch); j++ {
			cursor = j
			subPattern := strings.Join(partsToMatch[j:j+patternSize], string(filepath.Separator))
			if matchBracketPattern(pattern, subPattern) {
				cursor = j + patternSize - 1
				break
			}
			if cursor > len(partsToMatch)-patternSize-1 {
				return false
			}
		}
	}

	return true
}

// we also check for the following bracket syntax: /path/*.{php,twig,yaml}
func matchBracketPattern(pattern string, fileName string) bool {
	openingBracket := strings.Index(pattern, "{")
	closingBracket := strings.Index(pattern, "}")

	// if there are no brackets we can match regularly
	if openingBracket == -1 || closingBracket == -1 {
		return matchPattern(pattern, fileName)
	}

	beforeTheBrackets := pattern[:openingBracket]
	betweenTheBrackets := pattern[openingBracket+1 : closingBracket]
	afterTheBrackets := pattern[closingBracket+1:]

	// all bracket entries are checked individually, only one needs to match
	// *.{php,twig,yaml} -> *.php, *.twig, *.yaml
	for pattern := range strings.SplitSeq(betweenTheBrackets, ",") {
		if matchPattern(beforeTheBrackets+pattern+afterTheBrackets, fileName) {
			return true
		}
	}

	return false
}

func matchPattern(pattern string, fileName string) bool {
	if pattern == "" {
		return true
	}

	patternMatches, err := filepath.Match(pattern, fileName)

	if err != nil {
		if logger.Enabled(ctx, slog.LevelError) {
			logger.LogAttrs(ctx, slog.LevelError, "failed to match filename", slog.String("file", fileName), slog.Any("error", err))
		}

		return false
	}

	return patternMatches
}
