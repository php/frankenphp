package frankenphp

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zaptest"
)

// execute the function on a PHP thread directly
// this is necessary if tests make use of PHP's internal allocation
func testOnDummyPHPThread(t *testing.T, cb func()) {
	t.Helper()
	logger = slog.New(zapslog.NewHandler(zaptest.NewLogger(t).Core()))
	assert.NoError(t, Init(
		WithWorkers("tw", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithNumThreads(2),
		WithLogger(logger),
	))
	defer Shutdown()

	_, err := executeOnPHPThread(cb, "tw")
	assert.NoError(t, err)
}

// executeOnPHPThread executes the callback func() directly on a task worker thread
// useful for testing purposes when dealing with PHP allocations
func executeOnPHPThread(callback func(), taskWorkerName string) (*pendingTask, error) {
	tw := getTaskWorkerByName(taskWorkerName)
	if tw == nil {
		return nil, errors.New("no task worker found with name " + taskWorkerName)
	}

	pt := &pendingTask{callback: callback}
	err := pt.dispatch(tw)

	return pt, err
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
		defer zvalPtrDtor(phpArray)
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
		defer zvalPtrDtor(phpArray)
		convertedArray, err := GoAssociativeArray[string](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []string{"bar1", "bar2"}

		phpArray := PHPPackedArray(originalSlice)
		defer zvalPtrDtor(phpArray)
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
		defer zvalPtrDtor(phpArray)
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
		defer zvalPtrDtor(phpArray)
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
		defer zvalPtrDtor(phpArray)
		convertedArray, err := GoMap[any](phpArray)
		require.NoError(t, err)

		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}
