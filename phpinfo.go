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
)

// AddPHPInfoEntry adds an entry to the frankenphp section of phpinfo().
func AddPHPInfoEntry(key, value string) {
	phpinfoMu.Lock()
	defer phpinfoMu.Unlock()
	phpinfoEntries = append(phpinfoEntries, phpinfoEntry{key, value})
}

func collectPHPInfoEntries(buildInfo *debug.BuildInfo) (entries, modules []phpinfoEntry) {
	phpinfoMu.Lock()
	entries = slices.Clone(phpinfoEntries)
	phpinfoMu.Unlock()

	if buildInfo == nil {
		return entries, nil
	}

	entries = append(entries, phpinfoEntry{"go", buildInfo.GoVersion})
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

//export go_frankenphp_phpinfo
func go_frankenphp_phpinfo() {
	buildInfo, _ := debug.ReadBuildInfo()
	entries, modules := collectPHPInfoEntries(buildInfo)

	// PHP consumes these tables synchronously on the calling PHP thread.
	// Each call owns its pins, so concurrent phpinfo() calls share no C pointers.
	var pinner runtime.Pinner
	defer pinner.Unpin()
	C.frankenphp_print_phpinfo(pinPHPInfoEntries(entries, &pinner), pinPHPInfoEntries(modules, &pinner))
}

// pinPHPInfoEntries sorts entries and pins a null-terminated array of key, value
// pointers, along with the null-terminated strings they point to.
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
