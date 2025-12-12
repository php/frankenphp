//go:build !nowatcher

package watcher

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/dunglas/frankenphp/internal/fastabs"
	"github.com/e-dant/watcher/watcher-go"
)

type pattern struct {
	patternGroup *PatternGroup
	value        string
	parsedValues []string
	events       chan eventHolder
	failureCount int

	watcher *watcher.Watcher
}

func (p *pattern) startSession() {
	p.watcher = watcher.NewWatcher(p.value, p.handle)

	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "watching", slog.String("pattern", p.value))
	}
}

// this method prepares the pattern struct (aka /path/*pattern)
func (p *pattern) parse() (err error) {
	// first we clean the value
	absPattern, err := fastabs.FastAbs(p.value)
	if err != nil {
		return err
	}

	p.value = absPattern

	// then we split the pattern to determine where the directory ends and the pattern starts
	splitPattern := strings.Split(absPattern, string(filepath.Separator))
	patternWithoutDir := ""
	for i, part := range splitPattern {
		isFilename := i == len(splitPattern)-1 && strings.Contains(part, ".")
		isGlobCharacter := strings.ContainsAny(part, "[*?{")

		if isFilename || isGlobCharacter {
			patternWithoutDir = filepath.Join(splitPattern[i:]...)
			p.value = filepath.Join(splitPattern[:i]...)

			break
		}
	}

	// now we split the pattern according to the recursive '**' syntax
	p.parsedValues = strings.Split(patternWithoutDir, "**")
	for i, pp := range p.parsedValues {
		p.parsedValues[i] = strings.Trim(pp, string(filepath.Separator))
	}

	// remove the trailing separator and add leading separator
	p.value = string(filepath.Separator) + strings.Trim(p.value, string(filepath.Separator))

	// try to canonicalize the path
	canonicalPattern, err := filepath.EvalSymlinks(p.value)
	if err == nil {
		p.value = canonicalPattern
	}

	return nil
}

func (p *pattern) allowReload(event *watcher.Event) bool {
	if !isValidEventType(event.EffectType) || !isValidPathType(event) {
		return false
	}

	// some editors create temporary files and never actually modify the original file
	// so we need to also check Event.AssociatedPathName
	// see https://github.com/php/frankenphp/issues/1375
	return p.isValidPattern(event.PathName) || p.isValidPattern(event.AssociatedPathName)
}

func (p *pattern) handle(event *watcher.Event) {
	// If the watcher prematurely sends the die@ event, retry watching
	if event.PathType == watcher.PathTypeWatcher && strings.HasPrefix(event.PathName, "e/self/die@") && watcherIsActive.Load() {
		p.retryWatching()

		return
	}

	if p.allowReload(event) {
		p.events <- eventHolder{p.patternGroup, event}
	}
}

func (p *pattern) stop() {
	p.watcher.Close()
}

func isValidEventType(effectType watcher.EffectType) bool {
	return effectType <= watcher.EffectTypeDestroy
}

func isValidPathType(event *watcher.Event) bool {
	if event.PathType == watcher.PathTypeWatcher && globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "special e-dant/watcher event", slog.Any("event", event))
	}

	return event.PathType <= watcher.PathTypeHardLink
}

func (p *pattern) isValidPattern(fileName string) bool {
	if fileName == "" {
		return false
	}

	// first we remove the dir from the file name
	if !strings.HasPrefix(fileName, p.value) {
		return false
	}

	// remove the directory path and separator from the filename
	fileNameWithoutDir := strings.TrimPrefix(strings.TrimPrefix(fileName, p.value), string(filepath.Separator))

	// if the pattern has size 1 we can match it directly against the filename
	if len(p.parsedValues) == 1 {
		return matchCurlyBracePattern(p.parsedValues[0], fileNameWithoutDir)
	}

	return p.matchPatterns(fileNameWithoutDir)
}

func (p *pattern) matchPatterns(fileName string) bool {
	partsToMatch := strings.Split(fileName, string(filepath.Separator))
	cursor := 0

	// if there are multiple parsedValues due to '**' we need to match them individually
	for i, pattern := range p.parsedValues {
		patternSize := strings.Count(pattern, string(filepath.Separator)) + 1

		// if we are at the last pattern we will start matching from the end of the filename
		if i == len(p.parsedValues)-1 {
			cursor = len(partsToMatch) - patternSize

			if cursor < 0 {
				return false
			}
		}

		// the cursor will move through the fileName until the pattern matches
		for j := cursor; j < len(partsToMatch); j++ {
			if j+patternSize > len(partsToMatch) {
				return false
			}

			cursor = j
			subPattern := strings.Join(partsToMatch[j:j+patternSize], string(filepath.Separator))

			if matchCurlyBracePattern(pattern, subPattern) {
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

// we also check for the following syntax: /path/*.{php,twig,yaml}
func matchCurlyBracePattern(pattern string, fileName string) bool {
	for _, subPattern := range expandCurlyBraces(pattern) {
		if matchPattern(subPattern, fileName) {
			return true
		}
	}

	return false
}

// {dir1,dir2}/path -> []string{"dir1/path", "dir2/path"}
func expandCurlyBraces(s string) []string {
	start := strings.Index(s, "{")
	if start == -1 {
		return []string{s}
	}
	end := strings.Index(s[start:], "}")
	if end == -1 {
		return []string{s}
	}

	j := start + end
	var out []string
	for _, opt := range strings.Split(s[start+1:j], ",") {
		subPattern := s[:start] + opt + s[j+1:]
		allSubPatterns := expandCurlyBraces(subPattern)
		out = append(out, allSubPatterns...)
	}

	return out
}

func matchPattern(pattern string, fileName string) bool {
	if pattern == "" {
		return true
	}

	patternMatches, err := filepath.Match(pattern, fileName)

	if err != nil {
		if globalLogger.Enabled(globalCtx, slog.LevelError) {
			globalLogger.LogAttrs(globalCtx, slog.LevelError, "failed to match filename", slog.String("file", fileName), slog.Any("error", err))
		}

		return false
	}

	return patternMatches
}
