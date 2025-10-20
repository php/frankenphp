package frankenphp

import (
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
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
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

		convertedString := GoString(PHPString(originalString, false))

		assert.Equal(t, originalString, convertedString, "string -> zend_string -> string should yield an equal string")
	})
}

func TestPHPMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalMap := map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		convertedMap, err := GoMap[string](PHPMap(originalMap))
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

		convertedArray, err := GoAssociativeArray[string](PHPAssociativeArray(originalArray))
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []string{"bar1", "bar2"}

		convertedSlice, err := GoPackedArray[string](PHPPackedArray(originalSlice))
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

		convertedMap, err := GoMap[string](PHPPackedArray(originalSlice))
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

		convertedSlice, err := GoPackedArray[string](PHPAssociativeArray(originalArray))
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

		convertedArray, err := GoMap[any](PHPMap(originalArray))
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}
