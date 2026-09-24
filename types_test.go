package frankenphp

import (
	"log/slog"
	"runtime"
	"runtime/debug"
	"slices"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execute the function on a PHP thread directly
// this is necessary if tests make use of PHP's internal allocation
func testOnDummyPHPThread(t *testing.T, test func()) {
	t.Helper()

	globalLogger = slog.Default()
	_, err := initPHPThreads(1, 1, 0, nil) // boot 1 thread
	assert.NoError(t, err)
	handler := convertToTaskThread(phpThreads[0])

	task := newTask(test)
	handler.execute(task)
	task.waitForCompletion()

	drainPHPThreads()
}

func TestGoString(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalString := "Hello, World!"

		phpString := PHPString(originalString, false)
		defer zendStringRelease(phpString)

		assert.Equal(t, originalString, GoString(phpString), "string -> zend_string -> string should yield an equal string")
	})
}

func TestPHPMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalMap := map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		phpArray := PHPMap(originalMap)
		defer zendHashDestroy(phpArray)
		convertedMap, err := GoMap[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalMap, convertedMap, "associative array should be equal after conversion")
	})
}

func TestOrderedPHPAssociativeArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray[string]{
			Map: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo2", "foo1"},
		}

		phpArray := PHPAssociativeArray(originalArray)
		defer zendHashDestroy(phpArray)
		convertedArray, err := GoAssociativeArray[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []string{"bar1", "bar2"}

		phpArray := PHPPackedArray(originalSlice)
		defer zendHashDestroy(phpArray)
		convertedSlice, err := GoPackedArray[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalSlice, convertedSlice, "slice should be equal after conversion")
	})
}

func TestPHPPackedArrayToGoMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []string{"bar1", "bar2"}
		expectedMap := map[string]string{
			"0": "bar1",
			"1": "bar2",
		}

		phpArray := PHPPackedArray(originalSlice)
		defer zendHashDestroy(phpArray)
		convertedMap, err := GoMap[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, expectedMap, convertedMap, "convert a packed to an associative array")
	})
}

func TestPHPAssociativeArrayToPacked(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray[string]{
			Map: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo1", "foo2"},
		}
		expectedSlice := []string{"bar1", "bar2"}

		phpArray := PHPAssociativeArray(originalArray)
		defer zendHashDestroy(phpArray)
		convertedSlice, err := GoPackedArray[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, expectedSlice, convertedSlice, "convert an associative array to a slice")
	})
}

func TestNestedMixedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := map[string]any{
			"string":      "value",
			"int":         int64(123),
			"float":       1.2,
			"true":        true,
			"false":       false,
			"nil":         nil,
			"packedArray": []any{"bar1", "bar2"},
			"associativeArray": AssociativeArray[any]{
				Map:   map[string]any{"foo1": "bar1", "foo2": "bar2"},
				Order: []string{"foo2", "foo1"},
			},
		}

		phpArray := PHPMap(originalArray)
		defer zendHashDestroy(phpArray)
		convertedArray, err := GoMap[any](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}

func TestPinPHPInfoEntries(t *testing.T) {
	var pinner runtime.Pinner
	defer pinner.Unpin()

	require.Nil(t, pinPHPInfoEntries(nil, &pinner))
	entries := []phpinfoEntry{{"z", "last"}, {"a", ""}, {"", "first"}}
	ptr := pinPHPInfoEntries(entries, &pinner)
	runtime.GC()

	arr := unsafe.Slice(ptr, 2*len(entries)+1)
	for i, want := range []string{"", "first", "a", "", "z", "last"} {
		got := unsafe.Slice((*byte)(unsafe.Pointer(arr[i])), len(want)+1)
		assert.Equal(t, want+"\x00", string(got))
	}
	assert.Nil(t, arr[len(arr)-1])
}

func TestBuildGoModuleEntries(t *testing.T) {
	deps := []*debug.Module{
		{Path: "example.com/dependency", Version: "v1.2.3"},
		{
			Path:    "example.com/replaced",
			Version: "v1.0.0",
			Replace: &debug.Module{Path: "example.com/fork", Version: "v1.4.0"},
		},
		{
			Path:    "example.com/local",
			Version: "v1.0.0",
			Replace: &debug.Module{Path: "../local"},
		},
	}
	wantDeps := []phpinfoEntry{
		{"example.com/dependency", "v1.2.3"},
		{"example.com/replaced", "example.com/fork v1.4.0"},
		{"example.com/local", "../local"},
	}

	for _, tt := range []struct {
		name string
		info debug.BuildInfo
		want []phpinfoEntry
	}{
		{
			name: "direct caddy main",
			info: debug.BuildInfo{
				Main: debug.Module{Path: "github.com/dunglas/frankenphp/caddy", Version: "v1.12.7"},
			},
			want: []phpinfoEntry{{"github.com/dunglas/frankenphp/caddy", "v1.12.7"}},
		},
		{
			name: "development main",
			info: debug.BuildInfo{Main: debug.Module{Path: "caddy", Version: "(devel)"}},
			want: []phpinfoEntry{{"caddy", "(devel)"}},
		},
		{
			name: "main without version",
			info: debug.BuildInfo{Main: debug.Module{Path: "example.com/app"}},
			want: []phpinfoEntry{{"example.com/app", ""}},
		},
		{
			name: "main absent",
		},
		{
			name: "generated command",
			info: debug.BuildInfo{Path: "command-line-arguments"},
		},
		{
			name: "main without path",
			info: debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
		},
		{
			name: "replaced main",
			info: debug.BuildInfo{Main: debug.Module{
				Path:    "example.com/app",
				Version: "v1.0.0",
				Replace: &debug.Module{Path: "example.com/app-fork", Version: "v1.1.0"},
			}},
			want: []phpinfoEntry{{"example.com/app", "example.com/app-fork v1.1.0"}},
		},
		{
			name: "locally replaced main",
			info: debug.BuildInfo{Main: debug.Module{
				Path:    "example.com/app",
				Version: "v1.0.0",
				Replace: &debug.Module{Path: "../app"},
			}},
			want: []phpinfoEntry{{"example.com/app", "../app"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildGoModuleEntries(&tt.info); !slices.Equal(got, tt.want) {
				t.Fatalf("without dependencies: got %v, want %v", got, tt.want)
			}

			tt.info.Deps = deps
			want := append(slices.Clone(tt.want), wantDeps...)
			if got := buildGoModuleEntries(&tt.info); !slices.Equal(got, want) {
				t.Fatalf("with dependencies: got %v, want %v", got, want)
			}
		})
	}
}

func TestCollectPHPInfoEntries(t *testing.T) {
	// Keep this test serial and isolate registrations from the runtime tests.
	previousEntries := phpinfoEntries
	phpinfoEntries = nil
	t.Cleanup(func() { phpinfoEntries = previousEntries })

	const key, path = "dunglas/mercure", "github.com/dunglas/mercure"
	AddPHPInfoEntry("custom", "value")

	entries, modules := collectPHPInfoEntries(nil)
	require.Equal(t, []phpinfoEntry{{"custom", "value"}}, entries)
	require.Empty(t, modules)

	entries, modules = collectPHPInfoEntries(&debug.BuildInfo{GoVersion: "go1.26.0"})
	require.Equal(t, []phpinfoEntry{{"custom", "value"}, {"Go", "go1.26.0"}}, entries)
	require.Empty(t, modules)

	for _, tt := range []struct {
		name    string
		replace *debug.Module
		want    string
	}{
		{name: "unreplaced", want: "v1.2.3"},
		{name: "same module version replacement", replace: &debug.Module{Path: path, Version: "v1.2.4"}, want: path + " v1.2.4"},
		{name: "fork version replacement", replace: &debug.Module{Path: "example.com/fork", Version: "v2.0.0"}, want: "example.com/fork v2.0.0"},
		{name: "local path replacement", replace: &debug.Module{Path: "../local-component"}, want: "../local-component"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entries, modules := collectPHPInfoEntries(&debug.BuildInfo{
				GoVersion: "go1.26.0",
				Main:      debug.Module{Path: "github.com/e-dant/watcher", Version: "v3.0.0"},
				Deps: []*debug.Module{
					{Path: path, Version: "v1.2.3", Replace: tt.replace},
					{Path: "example.com/other", Version: "v4.0.0"},
					{Path: "github.com/dunglas/caddy-cbrotli", Version: "v1.0.0"},
				},
			})
			require.Equal(t, []phpinfoEntry{
				{"custom", "value"}, {"Go", "go1.26.0"}, {"e-dant/watcher", "v3.0.0"}, {key, tt.want},
				{"dunglas/caddy-cbrotli", "v1.0.0"},
			}, entries)
			require.Equal(t, []phpinfoEntry{
				{"github.com/e-dant/watcher", "v3.0.0"}, {path, tt.want}, {"example.com/other", "v4.0.0"},
				{"github.com/dunglas/caddy-cbrotli", "v1.0.0"},
			}, modules)
		})
	}
}
