package watcher

import (
	"encoding/json"
	"time"
)

type (
	EffectType int8
	PathType   int8
)

func (e EffectType) MarshalJSON() ([]byte, error) {
	var s string
	switch e {
	case EffectTypeRename:
		s = "rename"
	case EffectTypeModify:
		s = "modify"
	case EffectTypeCreate:
		s = "create"
	case EffectTypeDestroy:
		s = "destroy"
	case EffectTypeOwner:
		s = "owner"
	case EffectTypeOther:
		s = "other"
	}

	return json.Marshal(s)
}

func (e PathType) MarshalJSON() ([]byte, error) {
	var s string
	switch e {
	case PathTypeDir:
		s = "dir"
	case PathTypeFile:
		s = "file"
	case PathTypeHardLink:
		s = "hard_link"
	case PathTypeSymLink:
		s = "sym_link"
	case PathTypeWatcher:
		s = "watcher"
	case PathTypeOther:
		s = "other"
	}

	return json.Marshal(s)
}

type Event struct {
	EffectTime         time.Time
	PathName           string
	AssociatedPathName string `json:",omitempty"`
	EffectType         EffectType
	PathType           PathType
}

const (
	EffectTypeRename EffectType = iota
	EffectTypeModify
	EffectTypeCreate
	EffectTypeDestroy
	EffectTypeOwner
	EffectTypeOther
)

const (
	PathTypeDir PathType = iota
	PathTypeFile
	PathTypeHardLink
	PathTypeSymLink
	PathTypeWatcher
	PathTypeOther
)

type callbackFunc func(events []*Event)

type PatternGroup struct {
	Patterns []string
	Callback callbackFunc
}
