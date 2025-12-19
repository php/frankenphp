package frankenphp

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execute the function on a PHP thread directly
// this is necessary if tests make use of PHP's internal allocation
func testOnDummyPHPThread(t *testing.T, test func()) {
	t.Helper()

	globalLogger = slog.Default()
	_, err := initPHPThreads(1, 1, nil) // boot 1 thread
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
		defer zendArrayRelease(phpArray)
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
		defer zendArrayRelease(phpArray)
		convertedArray, err := GoAssociativeArray[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []string{"bar1", "bar2"}

		phpArray := PHPPackedArray(originalSlice)
		defer zendArrayRelease(phpArray)
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
		defer zendArrayRelease(phpArray)
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
		defer zendArrayRelease(phpArray)
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
		defer zendArrayRelease(phpArray)
		convertedArray, err := GoMap[any](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}

func benchOnPHPThread(b *testing.B, count int, cb func()) {
	globalLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := initPHPThreads(1, 1, nil) // boot 1 thread
	assert.NoError(b, err)
	handler := convertToTaskThread(phpThreads[0])

	task := newTask(func() {
		for i := 0; i < count; i++ {
			cb()
		}
	})
	handler.execute(task)
	task.waitForCompletion()

	drainPHPThreads()
}

func BenchmarkBool(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpBool := PHPValue(true)
		_, _ = GoValue[bool](phpBool)
	})
}

func BenchmarkInt(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpInt := PHPValue(int64(42))
		_, _ = GoValue[int64](phpInt)
	})
}

func BenchmarkFloat(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpFloat := PHPValue(3.14)
		_, _ = GoValue[float64](phpFloat)
	})
}

func BenchmarkString(b *testing.B) {
	message := "Hello, World!"
	benchOnPHPThread(b, b.N, func() {
		phpString := PHPString(message, false)
		_ = GoString(phpString)
		zendStringRelease(phpString)
	})
}

func BenchmarkEmptyMap(b *testing.B) {
	originalMap := map[string]any{}
	benchOnPHPThread(b, b.N, func() {
		phpArray := PHPMap(originalMap)
		_, _ = GoMap[any](phpArray)
		zendArrayRelease(phpArray)
	})
}

func BenchmarkMap5Entries(b *testing.B) {
	originalMap := map[string]any{
		"foo1": "bar1",
		"foo2": int64(2),
		"foo3": true,
		"foo4": 3.14,
		"foo5": nil,
	}
	benchOnPHPThread(b, b.N, func() {
		phpArray := PHPMap(originalMap)
		_, _ = GoMap[any](phpArray)
		zendArrayRelease(phpArray)
	})
}

func BenchmarkAssociativeArray5Entries(b *testing.B) {
	originalArray := AssociativeArray[any]{
		Map: map[string]any{
			"foo1": "bar1",
			"foo2": int64(2),
			"foo3": true,
			"foo4": 3.14,
			"foo5": nil,
		},
		Order: []string{"foo3", "foo1", "foo4", "foo2", "foo5"},
	}
	benchOnPHPThread(b, b.N, func() {
		phpArray := PHPAssociativeArray(originalArray)
		_, _ = GoAssociativeArray[any](phpArray)
		zendArrayRelease(phpArray)
	})
}

func BenchmarkSlice5Entries(b *testing.B) {
	originalSlice := []any{"bar1", int64(2), true, 3.14, nil}
	benchOnPHPThread(b, b.N, func() {
		phpArray := PHPPackedArray(originalSlice)
		_, _ = GoPackedArray[any](phpArray)
		zendArrayRelease(phpArray)
	})
}

func BenchmarkMap50Entries(b *testing.B) {
	originalMap := map[string]any{}
	for i := 0; i < 10; i++ {
		originalMap[fmt.Sprintf("foo%d", i*5)] = fmt.Sprintf("val%d", i)
		originalMap[fmt.Sprintf("foo%d", i*5+1)] = "Error" // interned string
		originalMap[fmt.Sprintf("foo%d", i*5+2)] = true
		originalMap[fmt.Sprintf("foo%d", i*5+3)] = 3.12
		originalMap[fmt.Sprintf("foo%d", i*5+4)] = nil
	}
	benchOnPHPThread(b, b.N, func() {
		phpArray := PHPMap(originalMap)
		_, _ = GoMap[any](phpArray)
		zendArrayRelease(phpArray)
	})
}
