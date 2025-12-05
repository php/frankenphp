package watcher

import (
	"time"
)

type (
	EffectType int8
	PathType   int8
)

func (e EffectType) MarshalJSON() ([]byte, error) {
	switch e {
	case EffectTypeRename:
		return []byte(`"Rename"`), nil
	case EffectTypeModify:
		return []byte(`"Modify"`), nil
	case EffectTypeCreate:
		return []byte(`"Create"`), nil
	case EffectTypeDestroy:
		return []byte(`"Destroy"`), nil
	case EffectTypeOwner:
		return []byte(`"Owner"`), nil
	case EffectTypeOther:
		return []byte(`"Other"`), nil
	}

	panic("unreachable")
}

func (e PathType) MarshalJSON() ([]byte, error) {
	switch e {
	case PathTypeDir:
		return []byte(`"Dir"`), nil
	case PathTypeFile:
		return []byte(`"File"`), nil
	case PathTypeHardLink:
		return []byte(`"HardLink"`), nil
	case PathTypeSymLink:
		return []byte(`"SymLink"`), nil
	case PathTypeWatcher:
		return []byte(`"Watcher"`), nil
	case PathTypeOther:
		return []byte(`"Other"`), nil
	}

	panic("unreachable")
}

type Event struct {
	EffectTime         time.Time  `json:"effectTime"`
	PathName           string     `json:"pathName"`
	AssociatedPathName string     `json:"associatedPathName,omitempty"`
	EffectType         EffectType `json:"effectType"`
	PathType           PathType   `json:"pathType"`
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
