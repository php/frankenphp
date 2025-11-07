// Package frankenphp embeds PHP in Go projects and provides a SAPI for net/http.
//
// This is the core of the [FrankenPHP app server], and can be used in any Go program.
//
// [FrankenPHP app server]: https://frankenphp.dev
package frankenphp

// Use PHP includes corresponding to your PHP installation by running:
//
//   export CGO_CFLAGS=$(php-config --includes)
//   export CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)"
//
// We also set these flags for hardening: https://github.com/docker-library/php/blob/master/8.2/bookworm/zts/Dockerfile#L57-L59

// #include <stdlib.h>
// #include <stdint.h>
// #include <php_variables.h>
// #include <zend_llist.h>
// #include <SAPI.h>
// #include "frankenphp.h"
import "C"
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
	// debug on Linux
	//_ "github.com/ianlancetaylor/cgosymbolizer"
)

type contextKeyStruct struct{}

var contextKey = contextKeyStruct{}

var (
	ErrInvalidRequest         = errors.New("not a FrankenPHP request")
	ErrAlreadyStarted         = errors.New("FrankenPHP is already started")
	ErrInvalidPHPVersion      = errors.New("FrankenPHP is only compatible with PHP 8.2+")
	ErrMainThreadCreation     = errors.New("error creating the main thread")
	ErrRequestContextCreation = errors.New("error during request context creation")
	ErrScriptExecution        = errors.New("error during PHP script execution")
	ErrNotRunning             = errors.New("FrankenPHP is not running. For proper configuration visit: https://frankenphp.dev/docs/config/#caddyfile-config")

	isRunning        bool
	onServerShutdown []func()

	loggerMu sync.RWMutex
	logger   *slog.Logger

	metrics Metrics = nullMetrics{}

	maxWaitTime time.Duration
)

type syslogLevel int

const (
	emerg   syslogLevel = iota // system is unusable
	alert                      // action must be taken immediately
	crit                       // critical conditions
	err                        // error conditions
	warning                    // warning conditions
	notice                     // normal but significant condition
	info                       // informational
	debug                      // debug-level messages
)

func (l syslogLevel) String() string {
	switch l {
	case emerg:
		return "emerg"
	case alert:
		return "alert"
	case crit:
		return "crit"
	case err:
		return "err"
	case warning:
		return "warning"
	case notice:
		return "notice"
	case debug:
		return "debug"
	default:
		return "info"
	}
}

type PHPVersion struct {
	MajorVersion   int
	MinorVersion   int
	ReleaseVersion int
	ExtraVersion   string
	Version        string
	VersionID      int
}

type PHPConfig struct {
	Version                PHPVersion
	ZTS                    bool
	ZendSignals            bool
	ZendMaxExecutionTimers bool
}

// Version returns infos about the PHP version.
func Version() PHPVersion {
	cVersion := C.frankenphp_get_version()

	return PHPVersion{
		int(cVersion.major_version),
		int(cVersion.minor_version),
		int(cVersion.release_version),
		C.GoString(cVersion.extra_version),
		C.GoString(cVersion.version),
		int(cVersion.version_id),
	}
}

func Config() PHPConfig {
	cConfig := C.frankenphp_get_config()

	return PHPConfig{
		Version:                Version(),
		ZTS:                    bool(cConfig.zts),
		ZendSignals:            bool(cConfig.zend_signals),
		ZendMaxExecutionTimers: bool(cConfig.zend_max_execution_timers),
	}
}

func calculateMaxThreads(opt *opt) (numWorkers int, _ error) {
	maxProcs := runtime.GOMAXPROCS(0) * 2
	maxThreadsFromWorkers := 0

	for i, w := range opt.workers {
		if w.num <= 0 {
			// https://github.com/php/frankenphp/issues/126
			opt.workers[i].num = maxProcs
		}
		metrics.TotalWorkers(w.name, w.num)

		numWorkers += opt.workers[i].num

		if w.maxThreads > 0 {
			if w.maxThreads < w.num {
				return 0, fmt.Errorf("worker max_threads (%d) must be greater or equal to worker num (%d) (%q)", w.maxThreads, w.num, w.fileName)
			}
			if w.maxThreads > opt.maxThreads && opt.maxThreads > 0 {
				return 0, fmt.Errorf("worker max_threads (%d) cannot be greater than total max_threads (%d) (%q)", w.maxThreads, opt.maxThreads, w.fileName)
			}
			maxThreadsFromWorkers += w.maxThreads - w.num
		}
	}

	numThreadsIsSet := opt.numThreads > 0
	maxThreadsIsSet := opt.maxThreads != 0
	maxThreadsIsAuto := opt.maxThreads < 0 // maxthreads < 0 signifies auto mode (see phpmaintread.go)

	// if max_threads is only defined in workers, scale up to the sum of all worker max_threads
	if !maxThreadsIsSet && maxThreadsFromWorkers > 0 {
		maxThreadsIsSet = true
		if numThreadsIsSet {
			opt.maxThreads = opt.numThreads + maxThreadsFromWorkers
		} else {
			opt.maxThreads = numWorkers + 1 + maxThreadsFromWorkers
		}
	}

	if numThreadsIsSet && !maxThreadsIsSet {
		opt.maxThreads = opt.numThreads
		if opt.numThreads <= numWorkers {
			return 0, fmt.Errorf("num_threads (%d) must be greater than the number of worker threads (%d)", opt.numThreads, numWorkers)
		}

		return numWorkers, nil
	}

	if maxThreadsIsSet && !numThreadsIsSet {
		opt.numThreads = numWorkers + 1
		if !maxThreadsIsAuto && opt.numThreads > opt.maxThreads {
			return 0, fmt.Errorf("max_threads (%d) must be greater than the number of worker threads (%d)", opt.maxThreads, numWorkers)
		}

		return numWorkers, nil
	}

	if !maxThreadsIsSet && !numThreadsIsSet {
		if numWorkers >= maxProcs {
			// Start at least as many threads as workers, and keep a free thread to handle requests in non-worker mode
			opt.numThreads = numWorkers + 1
		} else {
			opt.numThreads = maxProcs
		}
		opt.maxThreads = opt.numThreads

		return numWorkers, nil
	}

	// both num_threads and max_threads are set
	if opt.numThreads <= numWorkers {
		return 0, fmt.Errorf("num_threads (%d) must be greater than the number of worker threads (%d)", opt.numThreads, numWorkers)
	}

	if !maxThreadsIsAuto && opt.maxThreads < opt.numThreads {
		return 0, fmt.Errorf("max_threads (%d) must be greater than or equal to num_threads (%d)", opt.maxThreads, opt.numThreads)
	}

	return numWorkers, nil
}

// Init starts the PHP runtime and the configured workers.
func Init(options ...Option) error {
	if isRunning {
		return ErrAlreadyStarted
	}
	isRunning = true

	// Ignore all SIGPIPE signals to prevent weird issues with systemd: https://github.com/php/frankenphp/issues/1020
	// Docker/Moby has a similar hack: https://github.com/moby/moby/blob/d828b032a87606ae34267e349bf7f7ccb1f6495a/cmd/dockerd/docker.go#L87-L90
	signal.Ignore(syscall.SIGPIPE)

	registerExtensions()

	// add registered external workers
	for _, ew := range extensionWorkers {
		options = append(options, WithWorkers(ew.name, ew.fileName, ew.num, ew.options...))
	}

	opt := &opt{}
	for _, o := range options {
		if err := o(opt); err != nil {
			return err
		}
	}

	if opt.logger == nil {
		// set a default logger
		// to disable logging, set the logger to slog.New(slog.NewTextHandler(io.Discard, nil))
		opt.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	loggerMu.Lock()
	logger = opt.logger
	loggerMu.Unlock()

	if opt.metrics != nil {
		metrics = opt.metrics
	}

	maxWaitTime = opt.maxWaitTime

	workerThreadCount, err := calculateMaxThreads(opt)
	if err != nil {
		return err
	}

	metrics.TotalThreads(opt.numThreads)

	config := Config()

	if config.Version.MajorVersion < 8 || (config.Version.MajorVersion == 8 && config.Version.MinorVersion < 2) {
		return ErrInvalidPHPVersion
	}

	if config.ZTS {
		if !config.ZendMaxExecutionTimers && runtime.GOOS == "linux" {
			logger.Warn(`Zend Max Execution Timers are not enabled, timeouts (e.g. "max_execution_time") are disabled, recompile PHP with the "--enable-zend-max-execution-timers" configuration option to fix this issue`)
		}
	} else {
		opt.numThreads = 1
		logger.Warn(`ZTS is not enabled, only 1 thread will be available, recompile PHP using the "--enable-zts" configuration option or performance will be degraded`)
	}

	mainThread, err := initPHPThreads(opt.numThreads, opt.maxThreads, opt.phpIni)
	if err != nil {
		return err
	}

	regularRequestChan = make(chan *frankenPHPContext, opt.numThreads-workerThreadCount)
	regularThreads = make([]*phpThread, 0, opt.numThreads-workerThreadCount)
	for i := 0; i < opt.numThreads-workerThreadCount; i++ {
		convertToRegularThread(getInactivePHPThread())
	}

	if err := initWorkers(opt.workers); err != nil {
		return err
	}

	initAutoScaling(mainThread)

	ctx := context.Background()
	logger.LogAttrs(ctx, slog.LevelInfo, "FrankenPHP started 🐘", slog.String("php_version", Version().Version), slog.Int("num_threads", mainThread.numThreads), slog.Int("max_threads", mainThread.maxThreads))
	if EmbeddedAppPath != "" {
		logger.LogAttrs(ctx, slog.LevelInfo, "embedded PHP app 📦", slog.String("path", EmbeddedAppPath))
	}

	// register the startup/shutdown hooks (mainly useful for extensions)
	onServerShutdown = nil
	for _, w := range opt.workers {
		if w.onServerStartup != nil {
			w.onServerStartup()
		}
		if w.onServerShutdown != nil {
			onServerShutdown = append(onServerShutdown, w.onServerShutdown)
		}
	}

	return nil
}

// Shutdown stops the workers and the PHP runtime.
func Shutdown() {
	if !isRunning {
		return
	}

	// call the shutdown hooks (mainly useful for extensions)
	for _, fn := range onServerShutdown {
		fn()
	}

	drainWatcher()
	drainAutoScaling()
	drainPHPThreads()

	metrics.Shutdown()

	// Remove the installed app
	if EmbeddedAppPath != "" {
		_ = os.RemoveAll(EmbeddedAppPath)
	}

	isRunning = false
	logger.Debug("FrankenPHP shut down")
}

// ServeHTTP executes a PHP script according to the given context.
func ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) error {
	if !isRunning {
		return ErrNotRunning
	}

	fc, ok := fromContext(request.Context())
	if !ok {
		return ErrInvalidRequest
	}

	fc.responseWriter = responseWriter

	if !fc.validate() {
		return nil
	}

	// Detect if a worker is available to handle this request
	if fc.worker != nil {
		fc.worker.handleRequest(fc)

		return nil
	}

	// If no worker was available, send the request to non-worker threads
	handleRequestWithRegularPHPThreads(fc)
	return nil
}

//export go_ub_write
func go_ub_write(threadIndex C.uintptr_t, cBuf *C.char, length C.int) (C.size_t, C.bool) {
	fc := phpThreads[threadIndex].getRequestContext()

	if fc.isDone {
		return 0, C.bool(true)
	}

	var writer io.Writer
	if fc.responseWriter == nil {
		var b bytes.Buffer
		// log the output of the worker
		writer = &b
	} else {
		writer = fc.responseWriter
	}

	i, e := writer.Write(unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), length))
	if e != nil {
		fc.logger.LogAttrs(context.Background(), slog.LevelWarn, "write error", slog.Any("error", e))
	}

	if fc.responseWriter == nil {
		// probably starting a worker script, log the output
		fc.logger.Info(writer.(*bytes.Buffer).String())
	}

	return C.size_t(i), C.bool(fc.clientHasClosed())
}

//export go_apache_request_headers
func go_apache_request_headers(threadIndex C.uintptr_t) (*C.go_string, C.size_t) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()

	if fc.responseWriter == nil {
		// worker mode, not handling a request

		logger.LogAttrs(context.Background(), slog.LevelDebug, "apache_request_headers() called in non-HTTP context", slog.String("worker", fc.worker.name))

		return nil, 0
	}

	headers := make([]C.go_string, 0, len(fc.request.Header)*2)

	for field, val := range fc.request.Header {
		fd := unsafe.StringData(field)
		thread.Pin(fd)

		cv := strings.Join(val, ", ")
		vd := unsafe.StringData(cv)
		thread.Pin(vd)

		headers = append(
			headers,
			C.go_string{C.size_t(len(field)), (*C.char)(unsafe.Pointer(fd))},
			C.go_string{C.size_t(len(cv)), (*C.char)(unsafe.Pointer(vd))},
		)
	}

	sd := unsafe.SliceData(headers)
	thread.Pin(sd)

	return sd, C.size_t(len(fc.request.Header))
}

func addHeader(fc *frankenPHPContext, cString *C.char, length C.int) {
	key, val := splitRawHeader(cString, int(length))
	if key == "" {
		fc.logger.LogAttrs(context.Background(), slog.LevelDebug, "invalid header", slog.String("header", C.GoStringN(cString, length)))
		return
	}
	fc.responseWriter.Header().Add(key, val)
}

// split the raw header coming from C with minimal allocations
func splitRawHeader(rawHeader *C.char, length int) (string, string) {
	buf := unsafe.Slice((*byte)(unsafe.Pointer(rawHeader)), length)

	// Search for the colon in 'Header-Key: value'
	var i int
	for i = 0; i < length; i++ {
		if buf[i] == ':' {
			break
		}
	}

	if i == length {
		return "", "" // No colon found, invalid header
	}

	headerKey := C.GoStringN(rawHeader, C.int(i))

	// skip whitespaces after the colon
	j := i + 1
	for j < length && buf[j] == ' ' {
		j++
	}

	// anything left is the header value
	valuePtr := (*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(rawHeader)) + uintptr(j)))
	headerValue := C.GoStringN(valuePtr, C.int(length-j))

	return headerKey, headerValue
}

//export go_write_headers
func go_write_headers(threadIndex C.uintptr_t, status C.int, headers *C.zend_llist) C.bool {
	fc := phpThreads[threadIndex].getRequestContext()

	if fc == nil {
		return C.bool(false)
	}

	if fc.isDone {
		return C.bool(false)
	}

	if fc.responseWriter == nil {
		// probably starting a worker script, pretend we wrote headers so PHP still calls ub_write
		return C.bool(true)
	}

	current := headers.head
	for current != nil {
		h := (*C.sapi_header_struct)(unsafe.Pointer(&(current.data)))

		addHeader(fc, h.header, C.int(h.header_len))
		current = current.next
	}

	goStatus := int(status)

	// go panics on invalid status code
	// https://github.com/golang/go/blob/9b8742f2e79438b9442afa4c0a0139d3937ea33f/src/net/http/server.go#L1162
	if goStatus < 100 || goStatus > 999 {
		logger.Warn(fmt.Sprintf("Invalid response status code %v", goStatus))
		goStatus = 500
	}

	fc.responseWriter.WriteHeader(goStatus)

	if goStatus >= 100 && goStatus < 200 {
		// Clear headers, it's not automatically done by ResponseWriter.WriteHeader() for 1xx responses
		h := fc.responseWriter.Header()
		for k := range h {
			delete(h, k)
		}
	}

	return C.bool(true)
}

//export go_sapi_flush
func go_sapi_flush(threadIndex C.uintptr_t) bool {
	fc := phpThreads[threadIndex].getRequestContext()
	if fc == nil || fc.responseWriter == nil {
		return false
	}

	if fc.clientHasClosed() && !fc.isDone {
		return true
	}

	if err := http.NewResponseController(fc.responseWriter).Flush(); err != nil {
		logger.LogAttrs(context.Background(), slog.LevelWarn, "the current responseWriter is not a flusher, if you are not using a custom build, please report this issue", slog.Any("error", err))
	}

	return false
}

//export go_read_post
func go_read_post(threadIndex C.uintptr_t, cBuf *C.char, countBytes C.size_t) (readBytes C.size_t) {
	fc := phpThreads[threadIndex].getRequestContext()

	if fc.responseWriter == nil {
		return 0
	}

	p := unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), countBytes)
	var err error
	for readBytes < countBytes && err == nil {
		var n int
		n, err = fc.request.Body.Read(p[readBytes:])
		readBytes += C.size_t(n)
	}

	return
}

//export go_read_cookies
func go_read_cookies(threadIndex C.uintptr_t) *C.char {
	request := phpThreads[threadIndex].getRequestContext().request
	if request == nil {
		return nil
	}

	cookie := strings.Join(request.Header.Values("Cookie"), "; ")
	if cookie == "" {
		return nil
	}

	// remove potential null bytes
	cookie = strings.ReplaceAll(cookie, "\x00", "")

	// freed in frankenphp_free_request_context()
	return C.CString(cookie)
}

//export go_log
func go_log(message *C.char, level C.int) {
	m := C.GoString(message)

	var le syslogLevel
	if level < C.int(emerg) || level > C.int(debug) {
		le = info
	} else {
		le = syslogLevel(level)
	}

	switch le {
	case emerg, alert, crit, err:
		logger.LogAttrs(context.Background(), slog.LevelError, m, slog.String("syslog_level", syslogLevel(level).String()))

	case warning:
		logger.LogAttrs(context.Background(), slog.LevelWarn, m, slog.String("syslog_level", syslogLevel(level).String()))
	case debug:
		logger.LogAttrs(context.Background(), slog.LevelDebug, m, slog.String("syslog_level", syslogLevel(level).String()))

	default:
		logger.LogAttrs(context.Background(), slog.LevelInfo, m, slog.String("syslog_level", syslogLevel(level).String()))
	}
}

//export go_is_context_done
func go_is_context_done(threadIndex C.uintptr_t) C.bool {
	return C.bool(phpThreads[threadIndex].getRequestContext().isDone)
}

// ExecuteScriptCLI executes the PHP script passed as parameter.
// It returns the exit status code of the script.
func ExecuteScriptCLI(script string, args []string) int {
	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	argc, argv := convertArgs(args)
	defer freeArgs(argv)

	return int(C.frankenphp_execute_script_cli(cScript, argc, (**C.char)(unsafe.Pointer(&argv[0])), false))
}

func ExecutePHPCode(phpCode string) int {
	// Ensure extensions are registered before CLI execution
	registerExtensions()

	cCode := C.CString(phpCode)
	defer C.free(unsafe.Pointer(cCode))
	return int(C.frankenphp_execute_script_cli(cCode, 0, nil, true))
}

func convertArgs(args []string) (C.int, []*C.char) {
	argc := C.int(len(args))
	argv := make([]*C.char, argc)
	for i, arg := range args {
		argv[i] = C.CString(arg)
	}
	return argc, argv
}

func freeArgs(argv []*C.char) {
	for _, arg := range argv {
		C.free(unsafe.Pointer(arg))
	}
}

func timeoutChan(timeout time.Duration) <-chan time.Time {
	if timeout == 0 {
		return nil
	}

	return time.After(timeout)
}
