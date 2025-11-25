//go:build !nowatcher

package watcher

// #cgo LDFLAGS: -lwatcher-c -lstdc++
// #include <stdint.h>
// #include <stdlib.h>
// #include "watcher.h"
import "C"
import (
	"log/slog"
	"path/filepath"
	"runtime/cgo"
	"strings"
	"time"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/fastabs"
)

type patternGroup struct {
	callback callbackFunc
}

type pattern struct {
	patternGroup *patternGroup
	value        string
	parsedValues []string
	events       chan eventHolder
	failureCount int
	watcher      C.uintptr_t
	h            cgo.Handle
}

func (p *pattern) startSession() error {
	p.h = cgo.NewHandle(p)
	cDir := C.CString(p.value)
	defer C.free(unsafe.Pointer(cDir))

	p.watcher = C.start_new_watcher(cDir, C.uintptr_t(p.h))
	if p.watcher != 0 {
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "watching", slog.String("pattern", p.value))
		}

		return nil
	}

	if globalLogger.Enabled(globalCtx, slog.LevelError) {
		globalLogger.LogAttrs(globalCtx, slog.LevelError, "couldn't start watching", slog.String("pattern", p.value))
	}

	p.h.Delete()

	return ErrUnableToStartWatching
}

// this method prepares the pattern struct (aka /path/*pattern)
func (p *pattern) parse() error {
	// first we clean the value
	absPattern, err := fastabs.FastAbs(p.value)
	if err != nil {
		return err
	}

	p.value = absPattern

	// then we split the value to determine where the directory ends and the value starts
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

	// now we split the value according to the recursive '**' syntax
	p.parsedValues = strings.Split(patternWithoutDir, "**")
	for i, pp := range p.parsedValues {
		p.parsedValues[i] = strings.Trim(pp, string(filepath.Separator))
	}

	// finally, we remove the trailing separator and add leading separator
	p.value = string(filepath.Separator) + strings.Trim(p.value, string(filepath.Separator))

	return nil
}

func (p *pattern) allowReload(event *Event) bool {
	if !isValidEventType(event.EffectType) || !isValidPathType(event) {
		return false
	}

	return isValidPattern(event, p.value, p.parsedValues)
}

func (p *pattern) handle(event *Event) {
	// If the watcher prematurely sends the die@ event, retry watching
	if event.PathType == PathTypeWatcher && strings.HasPrefix(event.PathName, "e/self/die@") && watcherIsActive.Load() {
		p.retryWatching()

		return
	}

	if p.allowReload(event) {
		p.events <- eventHolder{p.patternGroup, event}
	}
}

func (p *pattern) stop() {
	if C.stop_watcher(p.watcher) == 0 && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "couldn't close the watcher")
	}

	p.h.Delete()
}

func isValidEventType(effectType EffectType) bool {
	return effectType <= EffectTypeDestroy
}

func isValidPathType(event *Event) bool {
	if event.PathType == PathTypeWatcher && globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "special e-dant/watcher event", slog.Any("event", event))
	}

	return event.PathType <= PathTypeHardLink
}

// some editors create temporary files and never actually modify the original file
// so we need to also check Event.AssociatedPathName
// see https://github.com/php/frankenphp/issues/1375
func isValidPattern(event *Event, dir string, patterns []string) bool {
	fileName := event.AssociatedPathName
	if fileName == "" {
		fileName = event.PathName
	}

	// first we remove the value from the file name
	if !strings.HasPrefix(fileName, dir) {
		return false
	}

	// remove the file name and separator from the filename
	fileNameWithoutDir := strings.TrimPrefix(strings.TrimPrefix(fileName, dir), string(filepath.Separator))

	// if the value has size 1 we can match it directly against the filename
	if len(patterns) == 1 {
		return matchBracketPattern(patterns[0], fileNameWithoutDir)
	}

	return matchPatterns(patterns, fileNameWithoutDir)
}

func matchPatterns(patterns []string, fileName string) bool {
	partsToMatch := strings.Split(fileName, string(filepath.Separator))
	cursor := 0

	// if there are multiple parsedValues due to '**' we need to match them individually
	for i, pattern := range patterns {
		patternSize := strings.Count(pattern, string(filepath.Separator)) + 1

		// if we are at the last value we will start matching from the end of the filename
		if i == len(patterns)-1 {
			cursor = len(partsToMatch) - patternSize
		}

		// the cursor will move through the fileName until the value matches
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

// we also check for the following bracket syntax: /value/*.{php,twig,yaml}
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
		if globalLogger.Enabled(globalCtx, slog.LevelError) {
			globalLogger.LogAttrs(globalCtx, slog.LevelError, "failed to match filename", slog.String("file", fileName), slog.Any("error", err))
		}

		return false
	}

	return patternMatches
}

//export go_handle_file_watcher_event
func go_handle_file_watcher_event(event C.struct_wtr_watcher_event, handle C.uintptr_t) {
	p := cgo.Handle(handle).Value().(*pattern)

	e := &Event{
		EffectTime:         time.Unix(int64(event.effect_time)/1000000000, int64(event.effect_time)%1000000000),
		PathName:           C.GoString(event.path_name),
		AssociatedPathName: C.GoString(event.associated_path_name),
		EffectType:         EffectType(event.effect_type),
		PathType:           PathType(event.path_type),
	}

	p.handle(e)
}
