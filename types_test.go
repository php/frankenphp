package frankenphp

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
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

	task, err := executeOnPHPThread(cb, "tw")
	assert.NoError(t, err)

	task.WaitForCompletion()
}

// executeOnPHPThread executes the callback func() directly on a task worker thread
// Currently only used in tests
func executeOnPHPThread(callback func(), taskWorkerName string) (*PendingTask, error) {
	tw := getTaskWorkerByName(taskWorkerName)
	if tw == nil {
		return nil, errors.New("no task worker found with name " + taskWorkerName)
	}

	pt := &PendingTask{callback: callback}
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
		originalMap := map[string]any{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		phpArray := PHPMap(originalMap)
		defer zendHashDestroy(phpArray)

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

		phpArray := PHPAssociativeArray(originalArray)
		defer zendHashDestroy(phpArray)

		assert.Equal(t, originalArray, GoAssociativeArray(phpArray), "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalSlice := []any{"bar1", "bar2"}

		phpArray := PHPPackedArray(originalSlice)
		defer zendHashDestroy(phpArray)

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

		phpArray := PHPPackedArray(originalSlice)
		defer zendHashDestroy(phpArray)

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

		phpArray := PHPAssociativeArray(originalArray)
		defer zendHashDestroy(phpArray)

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

		phpArray := PHPMap(originalArray)
		defer zendHashDestroy(phpArray)

		assert.Equal(t, originalArray, GoMap(phpArray), "nested mixed array should be equal after conversion")
	})
}

func TestPHPObject(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalObject := Object{
			ClassName: "stdClass",
			Props: map[string]any{
				"prop1": "value1",
				"prop2": int64(42),
			},
		}

		phpObject := PHPObject(originalObject)
		defer zvalPtrDtor(phpObject)

		convertedObject := GoObject(phpObject)
		assert.Equal(t, originalObject.ClassName, convertedObject.ClassName, "object class should be equal after conversion")
		assert.Equal(t, originalObject.Props, convertedObject.Props, "object props should be equal after conversion")
	})
}

func benchOnPHPThread(b *testing.B, count int, cb func()) {
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	assert.NoError(b, Init(
		WithWorkers("tw", "./testdata/tasks/task-worker.php", 1, AsTaskWorker(true, 0)),
		WithNumThreads(2),
		WithLogger(logger),
	))
	defer Shutdown()
	task, err := executeOnPHPThread(func() {
		for i := 0; i < count; i++ {
			cb()
		}
	}, "tw")
	assert.NoError(b, err)
	task.WaitForCompletion()
}

func BenchmarkBool(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpBool := PHPValue(true)
		_ = GoValue(phpBool)
	})
}

func BenchmarkInt(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpInt := PHPValue(int64(42))
		_ = GoValue(phpInt)
	})
}

func BenchmarkFloat(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		phpFloat := PHPValue(3.14)
		_ = GoValue(phpFloat)
	})
}

func BenchmarkString(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		message := "Hello, World!"
		phpString := PHPString(message, false)
		_ = GoString(phpString)
		zendStringRelease(phpString)
	})
}

func BenchmarkMap(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		originalMap := map[string]any{
			"foo1": "bar1",
			"foo2": int64(2),
			"foo3": true,
			"foo4": 3.14,
			"foo5": nil,
		}

		phpArray := PHPMap(originalMap)
		_ = GoMap(phpArray)
		zendHashDestroy(phpArray)
	})
}

func BenchmarkOrderedAssociativeArray(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		originalArray := AssociativeArray{
			Map: map[string]any{
				"foo1": "bar1",
				"foo2": int64(2),
				"foo3": true,
				"foo4": 3.14,
				"foo5": nil,
			},
			Order: []string{"foo3", "foo1", "foo4", "foo2", "foo5"},
		}

		phpArray := PHPAssociativeArray(originalArray)
		_ = GoAssociativeArray(phpArray)
		zendHashDestroy(phpArray)
	})
}

func BenchmarkSlice(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		originalSlice := []any{"bar1", "bar2", "bar3", "bar4", "bar5"}

		phpArray := PHPPackedArray(originalSlice)
		_ = GoPackedArray(phpArray)
		zendHashDestroy(phpArray)
	})
}

func BenchmarkObject(b *testing.B) {
	benchOnPHPThread(b, b.N, func() {
		originalObject := Object{
			ClassName: "stdClass",
			Props: map[string]any{
				"prop1": "value1",
				"prop2": int64(42),
				"prop3": true,
				"prop4": 3.14,
				"prop5": nil,
			},
		}

		phpObject := PHPObject(originalObject)
		_ = GoObject(phpObject)
		zvalPtrDtor(phpObject)
	})
}
