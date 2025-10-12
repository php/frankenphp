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

		convertedString := GoString(PHPString(originalString, false))

		assert.Equal(t, originalString, convertedString, "string -> zend_string -> string should yield an equal string")
	})
}

func TestPHPMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalMap := map[string]any{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		convertedMap := GoMap(PHPMap(originalMap))

		assert.Equal(t, originalMap, convertedMap, "associative array should be equal after conversion")
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

		convertedArray := GoAssociativeArray(PHPAssociativeArray(originalArray))

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []any{"bar1", "bar2"}

		convertedSlice := GoPackedArray(PHPPackedArray(originalSlice))

		assert.Equal(t, originalSlice, convertedSlice, "slice should be equal after conversion")
	})
}

func TestPHPPackedArrayToGoMap(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []any{"bar1", "bar2"}
		expectedMap := map[string]any{
			"0": "bar1",
			"1": "bar2",
		}

		convertedMap := GoMap(PHPPackedArray(originalSlice))

		assert.Equal(t, expectedMap, convertedMap, "convert a packed to an associative array")
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

		convertedSlice := GoPackedArray(PHPAssociativeArray(originalArray))

		assert.Equal(t, expectedSlice, convertedSlice, "convert an associative array to a slice")
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

		convertedArray := GoMap(PHPMap(originalArray))

		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}

func TestPHPObject(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalObject := Object{
			ClassName: "stdClass",
			Props: map[string]any{
				"prop1": "value1",
				"prop2": 42,
			},
		}

		convertedObject := GoObject(PHPObject(originalObject))

		assert.Equal(t, originalObject, convertedObject, "nested mixed array should be equal after conversion")
	})
}
