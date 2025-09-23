package frankenphp

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
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

		phpString := PHPString(originalString, false)
		defer zendStringRelease(phpString)

		assert.Equal(t, originalString, GoString(phpString), "string -> zend_string -> string should yield an equal string")
	})
}

func TestPHPMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalMap := map[string]any{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		phpArray := arrayAsZval(PHPMap(originalMap))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, originalMap, GoMap(phpArray), "associative array should be equal after conversion")
	})
}

func TestOrderedPHPAssociativeArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{
			Map: map[string]any{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo2", "foo1"},
		}

		phpArray := arrayAsZval(PHPAssociativeArray(originalArray))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, originalArray, GoAssociativeArray(phpArray), "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []any{"bar1", "bar2"}

		phpArray := arrayAsZval(PHPPackedArray(originalSlice))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, originalSlice, GoPackedArray(phpArray), "slice should be equal after conversion")
	})
}

func TestPHPPackedArrayToGoMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []any{"bar1", "bar2"}
		expectedMap := map[string]any{
			"0": "bar1",
			"1": "bar2",
		}

		phpArray := arrayAsZval(PHPPackedArray(originalSlice))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, expectedMap, GoMap(phpArray), "convert a packed to an associative array")
	})
}

func TestPHPAssociativeArrayToPacked(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{
			Map: map[string]any{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo1", "foo2"},
		}
		expectedSlice := []any{"bar1", "bar2"}

		phpArray := arrayAsZval(PHPAssociativeArray(originalArray))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, expectedSlice, GoPackedArray(phpArray), "convert an associative array to a slice")
	})
}

func TestNestedMixedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := map[string]any{
			"string":      "value",
			"int":         int64(123),
			"float":       float64(1.2),
			"true":        true,
			"false":       false,
			"nil":         nil,
			"packedArray": []any{"bar1", "bar2"},
			"associativeArray": AssociativeArray{
				Map:   map[string]any{"foo1": "bar1", "foo2": "bar2"},
				Order: []string{"foo2", "foo1"},
			},
		}

		phpArray := arrayAsZval(PHPMap(originalArray))
		defer zvalPtrDtor(phpArray)

		assert.Equal(t, originalArray, GoMap(phpArray), "nested mixed array should be equal after conversion")
	})
}
