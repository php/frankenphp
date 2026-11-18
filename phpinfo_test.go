package frankenphp

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// rebuilds must not accumulate pinned tables: every borrow is released
func TestPHPInfoTablesAreRetiredAfterRelease(t *testing.T) {
	for i := 0; i < 500; i++ {
		AddPHPInfoEntry("retired-key", strings.Repeat("v", 1024))
		entries, modules := go_frankenphp_collect_phpinfo()
		go_frankenphp_release_phpinfo(entries, modules)
	}

	phpinfoBuildMu.Lock()
	current, retired, readers := phpinfoCurrent, len(phpinfoRetired), -1
	if current != nil {
		readers = current.readers
	}
	phpinfoBuildMu.Unlock()

	assert.NotNil(t, current)
	assert.Empty(t, retired, "every retired generation must have been unpinned")
	assert.Zero(t, readers, "every borrow must have been released")
}

// reader accounting must survive concurrent rebuilds
func TestPHPInfoConcurrentBorrowRelease(t *testing.T) {
	entries, modules := go_frankenphp_collect_phpinfo()
	go_frankenphp_release_phpinfo(entries, modules)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				entries, modules := go_frankenphp_collect_phpinfo()
				go_frankenphp_release_phpinfo(entries, modules)
			}
		}()
	}
	for i := 0; i < 100; i++ {
		AddPHPInfoEntry("concurrent-rebuild", "during")
	}
	wg.Wait()

	phpinfoBuildMu.Lock()
	defer phpinfoBuildMu.Unlock()

	assert.Empty(t, phpinfoRetired)
	if phpinfoCurrent != nil {
		assert.Zero(t, phpinfoCurrent.readers)
	}
}

// without retirement, 2000 cycles pin 140+ MB
func TestPHPInfoLateRegistrationDoesNotAccumulatePinnedTables(t *testing.T) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	value := strings.Repeat("v", 1024)
	for i := 0; i < 2000; i++ {
		AddPHPInfoEntry(fmt.Sprintf("heap-key-%d", i), value)
		entries, modules := go_frankenphp_collect_phpinfo()
		go_frankenphp_release_phpinfo(entries, modules)
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	assert.Less(t, after.HeapAlloc-before.HeapAlloc, uint64(32<<20))
}
