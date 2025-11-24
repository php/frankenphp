//go:build !nowatcher

package watcher

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

type watcher struct {
	patternGroup   *patternGroup
	name           string
	parsedPatterns []string
	events         chan eventHolder
	failureCount   int
	watcher        C.uintptr_t
}

func (p *watcher) startSession() error {
	handle := cgo.NewHandle(p)
	cDir := C.CString(p.name)
	defer C.free(unsafe.Pointer(cDir))

	p.watcher = C.start_new_watcher(cDir, C.uintptr_t(handle))
	if p.watcher != 0 {
		if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
			globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "watching", slog.String("watcher", p.name), slog.Any("groups", p.parsedPatterns))
		}

		return nil
	}

	if globalLogger.Enabled(globalCtx, slog.LevelError) {
		globalLogger.LogAttrs(globalCtx, slog.LevelError, "couldn't start watching", slog.String("watcher", p.name))
	}

	return ErrUnableToStartWatching
}

// this method prepares the watcher struct (aka /name/*watcher)
func (p *watcher) parse() error {
	// first we clean the name
	absPattern, err := fastabs.FastAbs(p.name)
	if err != nil {
		return err
	}

	p.name = absPattern

	// then we split the name to determine where the directory ends and the name starts
	splitPattern := strings.Split(absPattern, string(filepath.Separator))
	patternWithoutDir := ""
	for i, part := range splitPattern {
		isFilename := i == len(splitPattern)-1 && strings.Contains(part, ".")
		isGlobCharacter := strings.ContainsAny(part, "[*?{")

		if isFilename || isGlobCharacter {
			patternWithoutDir = filepath.Join(splitPattern[i:]...)
			p.name = filepath.Join(splitPattern[:i]...)

			break
		}
	}

	// now we split the name according to the recursive '**' syntax
	p.parsedPatterns = strings.Split(patternWithoutDir, "**")
	for i, pp := range p.parsedPatterns {
		p.parsedPatterns[i] = strings.Trim(pp, string(filepath.Separator))
	}

	// finally, we remove the trailing separator and add leading separator
	p.name = string(filepath.Separator) + strings.Trim(p.name, string(filepath.Separator))

	return nil
}

func (p *watcher) allowReload(event *Event) bool {
	if !isValidEventType(event.EffectType) || !isValidPathType(event) {
		return false
	}

	return isValidPattern(event, p.name, p.parsedPatterns)
}

func (p *watcher) handle(event *Event) {
	// If the globalWatcher prematurely sends the die@ event, retry watching
	if event.PathType == PathTypeWatcher && strings.HasPrefix(event.PathName, "e/self/die@") && watcherIsActive.Load() {
		p.retryWatching()

		return
	}

	if p.allowReload(event) {
		p.events <- eventHolder{p.patternGroup, event}
	}
}

func (p *watcher) stop() {
	if C.stop_watcher(p.watcher) == 0 && globalLogger.Enabled(globalCtx, slog.LevelWarn) {
		globalLogger.LogAttrs(globalCtx, slog.LevelWarn, "couldn't close the globalWatcher")
	}
}

func isValidEventType(effectType EffectType) bool {
	return effectType <= EffectTypeDestroy
}

func isValidPathType(event *Event) bool {
	if event.PathType == PathTypeWatcher && globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "special edant/globalWatcher event", slog.Any("event", event))
	}

	return event.PathType <= PathTypeHardLink
}

// some editors create temporary files and never actually modify the original file
// so we need to also check the associated watcher of an event
// see https://github.com/php/frankenphp/issues/1375
func isValidPattern(event *Event, dir string, patterns []string) bool {
	fileName := event.AssociatedPathName
	if fileName == "" {
		fileName = event.PathName
	}

	// first we remove the name from the name
	if !strings.HasPrefix(fileName, dir) {
		return false
	}

	// remove the name and separator from the filename
	fileNameWithoutDir := strings.TrimPrefix(strings.TrimPrefix(fileName, dir), string(filepath.Separator))

	// if the name has size 1 we can match it directly against the filename
	if len(patterns) == 1 {
		return matchBracketPattern(patterns[0], fileNameWithoutDir)
	}

	return matchPatterns(patterns, fileNameWithoutDir)
}

func matchPatterns(patterns []string, fileName string) bool {
	partsToMatch := strings.Split(fileName, string(filepath.Separator))
	cursor := 0

	// if there are multiple parsedPatterns due to '**' we need to match them individually
	for i, pattern := range patterns {
		patternSize := strings.Count(pattern, string(filepath.Separator)) + 1

		// if we are at the last name we will start matching from the end of the filename
		if i == len(patterns)-1 {
			cursor = len(partsToMatch) - patternSize
		}

		// the cursor will move through the fileName until the name matches
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

// we also check for the following bracket syntax: /name/*.{php,twig,yaml}
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
	p := cgo.Handle(handle).Value().(*watcher)

	e := &Event{
		time.Unix(int64(event.effect_time)/1000000000, int64(event.effect_time)%1000000000),
		C.GoString(event.path_name),
		C.GoString(event.associated_path_name),
		EffectType(event.path_type),
		PathType(event.effect_type),
	}

	p.handle(e)
}
