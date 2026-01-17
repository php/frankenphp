package frankenphp

import (
	"io"
	"math/rand/v2"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dunglas/frankenphp/internal/state"
	"github.com/stretchr/testify/assert"
)

var testDataPath, _ = filepath.Abs("./testdata")

func setupGlobals(t *testing.T) {
	t.Helper()

	t.Cleanup(Shutdown)

	resetGlobals()
}

func TestStartAndStopTheMainThreadWithOneInactiveThread(t *testing.T) {
	_, err := initPHPThreads(1, 1, nil) // boot 1 thread
	assert.NoError(t, err)

	assert.Len(t, phpThreads, 1)
	assert.Equal(t, 0, phpThreads[0].threadIndex)
	assert.True(t, phpThreads[0].state.Is(state.Inactive))

	drainPHPThreads()

	assert.Nil(t, phpThreads)
}

func TestTransitionRegularThreadToWorkerThread(t *testing.T) {
	setupGlobals(t)

	_, err := initPHPThreads(1, 1, nil)
	assert.NoError(t, err)

	// transition to regular thread
	convertToRegularThread(phpThreads[0])
	assert.IsType(t, &regularThread{}, phpThreads[0].handler)

	// transition to worker thread
	worker := getDummyWorker(t, "transition-worker-1.php")
	convertToWorkerThread(phpThreads[0], worker)
	assert.IsType(t, &workerThread{}, phpThreads[0].handler)
	assert.Len(t, worker.threads, 1)

	// transition back to inactive thread
	convertToInactiveThread(phpThreads[0])
	assert.IsType(t, &inactiveThread{}, phpThreads[0].handler)
	assert.Len(t, worker.threads, 0)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestTransitionAThreadBetween2DifferentWorkers(t *testing.T) {
	setupGlobals(t)

	_, err := initPHPThreads(1, 1, nil)
	assert.NoError(t, err)
	firstWorker := getDummyWorker(t, "transition-worker-1.php")
	secondWorker := getDummyWorker(t, "transition-worker-2.php")

	// convert to first worker thread
	convertToWorkerThread(phpThreads[0], firstWorker)
	firstHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, firstWorker, firstHandler.worker)
	assert.Len(t, firstWorker.threads, 1)
	assert.Len(t, secondWorker.threads, 0)

	// convert to second worker thread
	convertToWorkerThread(phpThreads[0], secondWorker)
	secondHandler := phpThreads[0].handler.(*workerThread)
	assert.Same(t, secondWorker, secondHandler.worker)
	assert.Len(t, firstWorker.threads, 0)
	assert.Len(t, secondWorker.threads, 1)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

// try all possible handler transitions
// takes around 200ms and is supposed to force race conditions
func TestTransitionThreadsWhileDoingRequests(t *testing.T) {
	t.Cleanup(Shutdown)

	var (
		isDone atomic.Bool
		wg     sync.WaitGroup
	)

	numThreads := 10
	numRequestsPerThread := 100
	worker1Path := filepath.Join(testDataPath, "transition-worker-1.php")
	worker1Name := "worker-1"
	worker2Path := filepath.Join(testDataPath, "transition-worker-2.php")
	worker2Name := "worker-2"

	assert.NoError(t, Init(
		WithNumThreads(numThreads),
		WithWorkers(worker1Name, worker1Path, 1,
			WithWorkerEnv(map[string]string{"ENV1": "foo"}),
			WithWorkerWatchMode([]string{}),
			WithWorkerMaxFailures(0),
		),
		WithWorkers(worker2Name, worker2Path, 1,
			WithWorkerEnv(map[string]string{"ENV1": "foo"}),
			WithWorkerWatchMode([]string{}),
			WithWorkerMaxFailures(0),
		),
	))

	// try all possible permutations of transition, transition every ms
	transitions := allPossibleTransitions(worker1Path, worker2Path)
	for i := range numThreads {
		go func(thread *phpThread, start int) {
			for {
				for j := start; j < len(transitions); j++ {
					if isDone.Load() {
						return
					}
					transitions[j](thread)
					time.Sleep(time.Millisecond)
				}
				start = 0
			}
		}(phpThreads[i], i)
	}

	// randomly do requests to the 3 endpoints
	wg.Add(numThreads)
	for i := range numThreads {
		go func(i int) {
			for range numRequestsPerThread {
				switch rand.IntN(3) {
				case 0:
					assertRequestBody(t, "http://localhost/transition-worker-1.php", "Hello from worker 1")
				case 1:
					assertRequestBody(t, "http://localhost/transition-worker-2.php", "Hello from worker 2")
				case 2:
					assertRequestBody(t, "http://localhost/transition-regular.php", "Hello from regular thread")
				}
			}
			wg.Done()
		}(i)
	}

	// we are finished as soon as all 1000 requests are done
	wg.Wait()
	isDone.Store(true)
}

func TestFinishBootingAWorkerScript(t *testing.T) {
	setupGlobals(t)

	_, err := initPHPThreads(1, 1, nil)
	assert.NoError(t, err)

	// boot the worker
	worker := getDummyWorker(t, "transition-worker-1.php")
	convertToWorkerThread(phpThreads[0], worker)
	phpThreads[0].state.WaitFor(state.Ready)

	assert.NotNil(t, phpThreads[0].handler.(*workerThread).dummyContext)
	assert.Nil(t, phpThreads[0].handler.(*workerThread).workerContext)
	assert.False(
		t,
		phpThreads[0].handler.(*workerThread).isBootingScript,
		"isBootingScript should be false after the worker thread is ready",
	)

	drainPHPThreads()
	assert.Nil(t, phpThreads)
}

func TestReturnAnErrorIf2WorkersHaveTheSameFileName(t *testing.T) {
	workers = []*worker{}
	w, err1 := newWorker(workerOpt{fileName: testDataPath + "/index.php"})
	workers = append(workers, w)
	_, err2 := newWorker(workerOpt{fileName: testDataPath + "/index.php"})

	assert.NoError(t, err1)
	assert.Error(t, err2, "two workers cannot have the same filename")
}

func TestReturnAnErrorIf2ModuleWorkersHaveTheSameName(t *testing.T) {
	workers = []*worker{}
	w, err1 := newWorker(workerOpt{fileName: testDataPath + "/index.php", name: "workername"})
	workers = append(workers, w)
	_, err2 := newWorker(workerOpt{fileName: testDataPath + "/hello.php", name: "workername"})

	assert.NoError(t, err1)
	assert.Error(t, err2, "two workers cannot have the same name")
}

func getDummyWorker(t *testing.T, fileName string) *worker {
	t.Helper()

	if workers == nil {
		workers = []*worker{}
	}

	worker, _ := newWorker(workerOpt{
		fileName: testDataPath + "/" + fileName,
		num:      1,
	})
	workers = append(workers, worker)

	return worker
}

func assertRequestBody(t *testing.T, url string, expected string) {
	r := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	req, err := NewRequestWithContext(r, WithRequestDocumentRoot(testDataPath, false))
	assert.NoError(t, err)
	err = ServeHTTP(w, req)
	assert.NoError(t, err)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, expected, string(body))
}

// create a mix of possible transitions of workers and regular threads
func allPossibleTransitions(worker1Path string, worker2Path string) []func(*phpThread) {
	return []func(*phpThread){
		convertToRegularThread,
		func(thread *phpThread) { thread.shutdown() },
		func(thread *phpThread) {
			if thread.state.Is(state.Reserved) {
				thread.boot()
			}
		},
		func(thread *phpThread) { convertToWorkerThread(thread, getWorkerByPath(worker1Path)) },
		convertToInactiveThread,
		func(thread *phpThread) { convertToWorkerThread(thread, getWorkerByPath(worker2Path)) },
		convertToInactiveThread,
	}
}

func TestCorrectThreadCalculation(t *testing.T) {
	maxProcs := runtime.GOMAXPROCS(0) * 2
	oneWorkerThread := []workerOpt{{num: 1}}

	// default values
	testThreadCalculation(t, maxProcs, maxProcs, &opt{})
	testThreadCalculation(t, maxProcs, maxProcs, &opt{workers: oneWorkerThread})

	// num_threads is set
	testThreadCalculation(t, 1, 1, &opt{numThreads: 1})
	testThreadCalculation(t, 2, 2, &opt{numThreads: 2, workers: oneWorkerThread})

	// max_threads is set
	testThreadCalculation(t, 1, 10, &opt{maxThreads: 10})
	testThreadCalculation(t, 2, 10, &opt{maxThreads: 10, workers: oneWorkerThread})
	testThreadCalculation(t, 5, 10, &opt{numThreads: 5, maxThreads: 10, workers: oneWorkerThread})

	// automatic max_threads
	testThreadCalculation(t, 1, -1, &opt{maxThreads: -1})
	testThreadCalculation(t, 2, -1, &opt{maxThreads: -1, workers: oneWorkerThread})
	testThreadCalculation(t, 2, -1, &opt{numThreads: 2, maxThreads: -1})

	// max_threads should be thread minimum + sum of worker max_threads
	testThreadCalculation(t, 2, 6, &opt{workers: []workerOpt{{num: 1, maxThreads: 5}}})
	testThreadCalculation(t, 6, 9, &opt{workers: []workerOpt{{num: 1, maxThreads: 4}, {num: 4, maxThreads: 4}}})
	testThreadCalculation(t, 10, 14, &opt{numThreads: 10, workers: []workerOpt{{num: 1, maxThreads: 4}, {num: 3, maxThreads: 4}}})

	// max_threads should remain equal to overall max_threads
	testThreadCalculation(t, 2, 5, &opt{maxThreads: 5, workers: []workerOpt{{num: 1, maxThreads: 3}}})
	testThreadCalculation(t, 3, 5, &opt{maxThreads: 5, workers: []workerOpt{{num: 1, maxThreads: 4}, {num: 1, maxThreads: 4}}})

	// not enough num threads
	testThreadCalculationError(t, &opt{numThreads: 1, workers: oneWorkerThread})
	testThreadCalculationError(t, &opt{numThreads: 1, maxThreads: 1, workers: oneWorkerThread})

	// not enough max_threads
	testThreadCalculationError(t, &opt{numThreads: 2, maxThreads: 1})
	testThreadCalculationError(t, &opt{maxThreads: 1, workers: oneWorkerThread})

	// worker max_threads is bigger than overall max_threads
	testThreadCalculationError(t, &opt{maxThreads: 5, workers: []workerOpt{{num: 1, maxThreads: 10}}})

	// worker max_threads is smaller than num_threads
	testThreadCalculationError(t, &opt{workers: []workerOpt{{num: 3, maxThreads: 2}}})
}

func testThreadCalculation(t *testing.T, expectedNumThreads int, expectedMaxThreads int, o *opt) {
	t.Helper()

	_, err := calculateMaxThreads(o)
	assert.NoError(t, err, "no error should be returned")
	assert.Equal(t, expectedNumThreads, o.numThreads, "num_threads must be correct")
	assert.Equal(t, expectedMaxThreads, o.maxThreads, "max_threads must be correct")
}

func testThreadCalculationError(t *testing.T, o *opt) {
	t.Helper()

	_, err := calculateMaxThreads(o)
	assert.Error(t, err, "configuration must error")
}
