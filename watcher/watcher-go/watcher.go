// Package watcher is a filesystem watcher. Simple, efficient and friendly.
package watcher

/*
#cgo unix LDFLAGS: -lwatcher-c
#cgo windows LDFLAGS: -llibwatcher-c
#include <stdlib.h>
#include <stdint.h>
#include <wtr/watcher-c.h>

extern void go_callback(struct wtr_watcher_event event, void *_ctx);

static inline void* open(char const *const path, uintptr_t _ctx) {
		return wtr_watcher_open(path, go_callback, (void *) _ctx);
}
*/
import "C"
import (
	"runtime/cgo"
	"time"
	"unsafe"
)

type (
	EffectType int8
	PathType   int8
)

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

type Event struct {
	EffectTime         time.Time  `json:"effect_time"`
	PathName           string     `json:"path_name"`
	AssociatedPathName string     `json:"associated_path_name,omitempty"`
	EffectType         EffectType `json:"effect_type"`
	PathType           PathType   `json:"path_type"`
}

type Watcher struct {
	handle   cgo.Handle
	callback func(*Event)
	cWatcher unsafe.Pointer
}

func NewWatcher(path string, callback func(*Event)) *Watcher {
	w := &Watcher{
		callback: callback,
	}

	h := cgo.NewHandle(w)
	w.handle = h

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	w.cWatcher = C.open(cPath, C.uintptr_t(h))

	return w
}

func (w *Watcher) Close() {
	C.wtr_watcher_close(w.cWatcher)
	w.handle.Delete()
}

//export go_callback
func go_callback(event C.struct_wtr_watcher_event, ctx unsafe.Pointer) {
	w := cgo.Handle(ctx).Value().(*Watcher)

	e := &Event{
		EffectTime:         time.Unix(int64(event.effect_time)/1000000000, int64(event.effect_time)%1000000000),
		PathName:           C.GoString(event.path_name),
		AssociatedPathName: C.GoString(event.associated_path_name),
		EffectType:         EffectType(event.effect_type),
		PathType:           PathType(event.path_type),
	}

	w.callback(e)
}

func (e EffectType) MarshalJSON() ([]byte, error) {
	switch e {
	case EffectTypeRename:
		return []byte(`"rename"`), nil
	case EffectTypeModify:
		return []byte(`"modify"`), nil
	case EffectTypeCreate:
		return []byte(`"create"`), nil
	case EffectTypeDestroy:
		return []byte(`"destroy"`), nil
	case EffectTypeOwner:
		return []byte(`"owner"`), nil
	case EffectTypeOther:
		return []byte(`"other"`), nil
	}

	panic("unreachable")
}

func (e PathType) MarshalJSON() ([]byte, error) {
	switch e {
	case PathTypeDir:
		return []byte(`"dir"`), nil
	case PathTypeFile:
		return []byte(`"file"`), nil
	case PathTypeHardLink:
		return []byte(`"hard_link"`), nil
	case PathTypeSymLink:
		return []byte(`"sym_link"`), nil
	case PathTypeWatcher:
		return []byte(`"watcher"`), nil
	case PathTypeOther:
		return []byte(`"other"`), nil
	}

	panic("unreachable")
}
