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
	phpinfoModules []phpinfoEntry
	phpinfoPinner  runtime.Pinner
)

// AddPHPInfoEntry adds an entry to the frankenphp section of phpinfo().
func AddPHPInfoEntry(key, value string) {
	phpinfoMu.Lock()
	defer phpinfoMu.Unlock()
	phpinfoEntries = append(phpinfoEntries, phpinfoEntry{key, value})
}

// AddPHPInfoModule adds a component's Go module version to the frankenphp section
// of phpinfo().
func AddPHPInfoModule(key, modulePath string) {
	phpinfoMu.Lock()
	defer phpinfoMu.Unlock()
	phpinfoModules = append(phpinfoModules, phpinfoEntry{key, modulePath})
}

func collectPHPInfoEntries(buildInfo *debug.BuildInfo) (entries, modules []phpinfoEntry) {
	phpinfoMu.Lock()
	entries = slices.Clone(phpinfoEntries)
	components := slices.Clone(phpinfoModules)
	phpinfoMu.Unlock()

	if buildInfo == nil {
		return entries, nil
	}

	entries = append(entries, phpinfoEntry{"go", buildInfo.GoVersion})
	modules = buildGoModuleEntries(buildInfo)
	versions := make(map[string]string, len(modules))
	for _, module := range modules {
		versions[module.key] = module.value
	}
	for _, component := range components {
		if version, ok := versions[component.value]; ok {
			entries = append(entries, phpinfoEntry{component.key, version})
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

func initPHPInfoEntries() {
	buildInfo, _ := debug.ReadBuildInfo()
	entries, modules := collectPHPInfoEntries(buildInfo)

	// Replace the previous runtime's tables before PHP starts using them.
	C.frankenphp_phpinfo_entries = nil
	C.frankenphp_go_modules = nil
	phpinfoPinner.Unpin()
	C.frankenphp_phpinfo_entries = pinPHPInfoEntries(entries, &phpinfoPinner)
	C.frankenphp_go_modules = pinPHPInfoEntries(modules, &phpinfoPinner)
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
