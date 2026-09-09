package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"runtime"
	"runtime/debug"
	"sort"
	"unsafe"
)

type phpinfoEntry struct {
	key, value string
}

var (
	phpinfoEntries  []phpinfoEntry
	goModuleEntries []phpinfoEntry
	phpinfoPinner   runtime.Pinner
)

// Report the Go toolchain and Go module versions.
// The list is verbose, so it's displayed in a collapsed section.
func init() {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	AddPHPInfoEntry("go", buildInfo.GoVersion)

	goModuleEntries = buildGoModuleEntries(buildInfo)
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

// goModuleVersion returns the version of the given module, taking "replace"
// directives into account.
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

// AddPHPInfoEntry adds an entry to the frankenphp section of phpinfo().
// Call it during package initialization before Init.
func AddPHPInfoEntry(key, value string) {
	phpinfoEntries = append(phpinfoEntries, phpinfoEntry{key, value})
}

// AddPHPInfoModule adds a component's Go module version to the frankenphp section
// of phpinfo(). Call it during package initialization before Init.
func AddPHPInfoModule(key string, module *debug.Module) {
	AddPHPInfoEntry(key, goModuleVersion(module))
}

func initPHPInfoEntries() {
	// Replace the previous runtime's tables before PHP starts using them.
	C.frankenphp_phpinfo_entries = nil
	C.frankenphp_go_modules = nil
	phpinfoPinner.Unpin()
	C.frankenphp_phpinfo_entries = pinPHPInfoEntries(phpinfoEntries, &phpinfoPinner)
	C.frankenphp_go_modules = pinPHPInfoEntries(goModuleEntries, &phpinfoPinner)
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
