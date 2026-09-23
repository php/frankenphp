package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"sync"
	"unsafe"
)

type phpinfoEntry struct {
	key, value string
}

var (
	phpinfoMu      sync.Mutex
	phpinfoEntries []phpinfoEntry

	phpinfoDirty   = true
	phpinfoBuildMu sync.Mutex
	phpinfoCurrent *phpinfoTable
	phpinfoRetired []*phpinfoTable
)

// one pinned generation of the phpinfo tables, held at a fixed address while
// C code may still be reading it
type phpinfoTable struct {
	pinner           runtime.Pinner
	entries, modules **C.char
	readers          int
	retired          bool
}

func (t *phpinfoTable) matches(entries, modules **C.char) bool {
	return t.entries == entries && t.modules == modules
}

// AddPHPInfoEntry adds an entry to the frankenphp section of phpinfo().
func AddPHPInfoEntry(key, value string) {
	phpinfoMu.Lock()
	defer phpinfoMu.Unlock()
	phpinfoEntries = append(phpinfoEntries, phpinfoEntry{key, value})
	phpinfoDirty = true
}

func collectPHPInfoEntries(buildInfo *debug.BuildInfo) (entries, modules []phpinfoEntry) {
	phpinfoMu.Lock()
	entries = slices.Clone(phpinfoEntries)
	phpinfoMu.Unlock()

	if buildInfo == nil {
		return entries, nil
	}

	entries = append(entries, phpinfoEntry{"Go", buildInfo.GoVersion})
	modules = buildGoModuleEntries(buildInfo)
	moduleAliases := map[string]string{
		"github.com/dunglas/mercure":       "dunglas/mercure",
		"github.com/e-dant/watcher":        "e-dant/watcher",
		"github.com/dunglas/caddy-cbrotli": "dunglas/caddy-cbrotli",
	}
	for _, module := range modules {
		if alias, ok := moduleAliases[module.key]; ok {
			entries = append(entries, phpinfoEntry{alias, module.value})
		}
	}
	return entries, modules
}

func buildGoModuleEntries(buildInfo *debug.BuildInfo) []phpinfoEntry {
	entries := make([]phpinfoEntry, 0, len(buildInfo.Deps)+1)
	if buildInfo.Main.Path != "" {
		entries = append(entries, phpinfoEntry{buildInfo.Main.Path, goModuleVersion(&buildInfo.Main)})
	}
	for _, dep := range buildInfo.Deps {
		entries = append(entries, phpinfoEntry{dep.Path, goModuleVersion(dep)})
	}
	return entries
}

func goModuleVersion(module *debug.Module) string {
	if module.Replace == nil {
		return module.Version
	}

	if module.Replace.Version == "" {
		// Replaced by a local directory
		return module.Replace.Path
	}

	return module.Replace.Path + " " + module.Replace.Version
}

// The caller must hand the borrowed tables back to
// go_frankenphp_release_phpinfo, bailout included, or they stay pinned.
//
//export go_frankenphp_collect_phpinfo
func go_frankenphp_collect_phpinfo() (**C.char, **C.char) {
	phpinfoBuildMu.Lock()
	defer phpinfoBuildMu.Unlock()

	phpinfoMu.Lock()
	dirty := phpinfoDirty
	phpinfoDirty = false
	phpinfoMu.Unlock()

	if !dirty && phpinfoCurrent != nil {
		phpinfoCurrent.readers++
		return phpinfoCurrent.entries, phpinfoCurrent.modules
	}

	buildInfo, _ := debug.ReadBuildInfo()
	entries, modules := collectPHPInfoEntries(buildInfo)

	table := new(phpinfoTable)
	table.entries = pinPHPInfoEntries(entries, &table.pinner)
	table.modules = pinPHPInfoEntries(modules, &table.pinner)
	table.readers = 1

	// readers of the previous table may still be printing it
	retirePHPInfoTableLocked(phpinfoCurrent)
	phpinfoCurrent = table

	return table.entries, table.modules
}

// go_frankenphp_release_phpinfo ends a collect borrow; the table is unpinned
// once retired and its last reader is done.
//
//export go_frankenphp_release_phpinfo
func go_frankenphp_release_phpinfo(entries, modules **C.char) {
	phpinfoBuildMu.Lock()
	defer phpinfoBuildMu.Unlock()

	table := phpinfoCurrent
	if table == nil || !table.matches(entries, modules) {
		table = nil
		for _, t := range phpinfoRetired {
			if t.matches(entries, modules) {
				table = t
				break
			}
		}
	}
	if table == nil {
		return
	}

	if table.readers > 0 {
		table.readers--
	}
	unpinPHPInfoTableLocked(table)
}

func retirePHPInfoTableLocked(table *phpinfoTable) {
	if table == nil || table.retired {
		return
	}

	table.retired = true
	phpinfoRetired = append(phpinfoRetired, table)
	unpinPHPInfoTableLocked(table)
}

func unpinPHPInfoTableLocked(table *phpinfoTable) {
	if !table.retired || table.readers > 0 {
		return
	}

	table.pinner.Unpin()
	phpinfoRetired = slices.DeleteFunc(phpinfoRetired, func(t *phpinfoTable) bool {
		return t == table
	})
}

// pinPHPInfoEntries returns a sorted, NUL-terminated key/value array for C.
func pinPHPInfoEntries(entries []phpinfoEntry, pinner *runtime.Pinner) **C.char {
	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	arr := make([]*C.char, 2*len(entries)+1)
	for i, e := range entries {
		for j, s := range []string{e.key, e.value} {
			data := unsafe.StringData(s + "\x00")
			pinner.Pin(data)
			arr[2*i+j] = (*C.char)(unsafe.Pointer(data))
		}
	}
	pinner.Pin(&arr[0])
	return &arr[0]
}
