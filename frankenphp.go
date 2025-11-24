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

var (
	ErrInvalidRequest     = errors.New("not a FrankenPHP request")
	ErrAlreadyStarted     = errors.New("FrankenPHP is already started")
	ErrInvalidPHPVersion  = errors.New("FrankenPHP is only compatible with PHP 8.2+")
	ErrMainThreadCreation = errors.New("error creating the main thread")
	ErrScriptExecution    = errors.New("error during PHP script execution")
	ErrNotRunning         = errors.New("FrankenPHP is not running. For proper configuration visit: https://frankenphp.dev/docs/config/#caddyfile-config")

	ErrInvalidRequestPath         = ErrRejected{"invalid request path", http.StatusBadRequest}
	ErrInvalidContentLengthHeader = ErrRejected{"invalid Content-Length header", http.StatusBadRequest}
	ErrMaxWaitTimeExceeded        = ErrRejected{"maximum request handling time exceeded", http.StatusServiceUnavailable}

	contextKey   = contextKeyStruct{}
	serverHeader = []string{"FrankenPHP"}

	isRunning        bool
	onServerShutdown []func()

	// Set default values to make Shutdown() idempotent
	globalMu     sync.Mutex
	globalCtx    = context.Background()
	globalLogger = slog.Default()

	metrics Metrics = nullMetrics{}

	maxWaitTime time.Duration
)

type ErrRejected struct {
	message string
	status  int
}

func (e ErrRejected) Error() string {
	return e.message
}

type syslogLevel int

const (
	syslogLevelEmerg  syslogLevel = iota // system is unusable
	syslogLevelAlert                     // action must be taken immediately
	syslogLevelCrit                      // critical conditions
	syslogLevelErr                       // error conditions
	syslogLevelWarn                      // warning conditions
	syslogLevelNotice                    // normal but significant condition
	syslogLevelInfo                      // informational
	syslogLevelDebug                     // debug-level messages
)

func (l syslogLevel) String() string {
	switch l {
	case syslogLevelEmerg:
		return "emerg"
	case syslogLevelAlert:
		return "alert"
	case syslogLevelCrit:
		return "crit"
	case syslogLevelErr:
		return "err"
	case syslogLevelWarn:
		return "warning"
	case syslogLevelNotice:
		return "notice"
	case syslogLevelDebug:
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

	opt := &opt{}
	for _, o := range options {
		if err := o(opt); err != nil {
			Shutdown()
			return err
		}
	}

	globalMu.Lock()

	if opt.ctx != nil {
		globalCtx = opt.ctx
		opt.ctx = nil
	}

	if opt.logger != nil {
		globalLogger = opt.logger
		opt.logger = nil
	}

	globalMu.Unlock()

	if opt.metrics != nil {
		metrics = opt.metrics
	}

	maxWaitTime = opt.maxWaitTime

	workerThreadCount, err := calculateMaxThreads(opt)
	if err != nil {
		Shutdown()
		return err
	}

	metrics.TotalThreads(opt.numThreads)

	config := Config()

	if config.Version.MajorVersion < 8 || (config.Version.MajorVersion == 8 && config.Version.MinorVersion < 2) {
		Shutdown()
		return ErrInvalidPHPVersion
	}

	if config.ZTS {
		if !config.ZendMaxExecutionTimers && runtime.GOOS == "linux" {
			if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
				globalLogger.LogAttrs(globalCtx, slog.LevelWarn, `Zend Max Execution Timers are not enabled, timeouts (e.g. "max_execution_time") are disabled, recompile PHP with the "--enable-zend-max-execution-timers" configuration option to fix this issue`)
			}
		}
	} else {
		opt.numThreads = 1

		if globalLogger.Enabled(globalCtx, slog.LevelWarn) {
			globalLogger.LogAttrs(globalCtx, slog.LevelWarn, `ZTS is not enabled, only 1 thread will be available, recompile PHP using the "--enable-zts" configuration option or performance will be degraded`)
		}
	}

	mainThread, err := initPHPThreads(opt.numThreads, opt.maxThreads, opt.phpIni)
	if err != nil {
		Shutdown()
		return err
	}

	regularRequestChan = make(chan contextHolder)
	regularThreads = make([]*phpThread, 0, opt.numThreads-workerThreadCount)
	for i := 0; i < opt.numThreads-workerThreadCount; i++ {
		convertToRegularThread(getInactivePHPThread())
	}

	if err := initWorkers(opt.workers); err != nil {
		Shutdown()

		return err
	}

	if err := initWatchers(opt); err != nil {
		Shutdown()
		return err
	}

	initAutoScaling(mainThread)

	if globalLogger.Enabled(globalCtx, slog.LevelInfo) {
		globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "FrankenPHP started 🐘", slog.String("php_version", Version().Version), slog.Int("num_threads", mainThread.numThreads), slog.Int("max_threads", mainThread.maxThreads))

		if EmbeddedAppPath != "" {
			globalLogger.LogAttrs(globalCtx, slog.LevelInfo, "embedded PHP app 📦", slog.String("path", EmbeddedAppPath))
		}
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

	drainWatchers()
	drainAutoScaling()
	drainPHPThreads()

	metrics.Shutdown()

	// Remove the installed app
	if EmbeddedAppPath != "" {
		_ = os.RemoveAll(EmbeddedAppPath)
	}

	isRunning = false
	if globalLogger.Enabled(globalCtx, slog.LevelDebug) {
		globalLogger.LogAttrs(globalCtx, slog.LevelDebug, "FrankenPHP shut down")
	}

	resetGlobals()
}

// ServeHTTP executes a PHP script according to the given context.
func ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) error {
	h := responseWriter.Header()
	if h["Server"] == nil {
		h["Server"] = serverHeader
	}

	if !isRunning {
		return ErrNotRunning
	}

	ctx := request.Context()
	fc, ok := fromContext(ctx)

	ch := contextHolder{ctx, fc}

	if !ok {
		return ErrInvalidRequest
	}

	fc.responseWriter = responseWriter

	if err := fc.validate(); err != nil {
		return err
	}

	// Detect if a worker is available to handle this request
	if fc.worker != nil {
		return fc.worker.handleRequest(ch)
	}

	// If no worker was available, send the request to non-worker threads
	return handleRequestWithRegularPHPThreads(ch)
}

//export go_ub_write
func go_ub_write(threadIndex C.uintptr_t, cBuf *C.char, length C.int) (C.size_t, C.bool) {
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()

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

	var ctx context.Context

	i, e := writer.Write(unsafe.Slice((*byte)(unsafe.Pointer(cBuf)), length))
	if e != nil {
		ctx = thread.context()

		if fc.logger.Enabled(ctx, slog.LevelWarn) {
			fc.logger.LogAttrs(ctx, slog.LevelWarn, "write error", slog.Any("error", e))
		}
	}

	if fc.responseWriter == nil {
		// probably starting a worker script, log the output

		if ctx == nil {
			ctx = thread.context()
		}

		if fc.logger.Enabled(ctx, slog.LevelInfo) {
			fc.logger.LogAttrs(ctx, slog.LevelInfo, writer.(*bytes.Buffer).String())
		}
	}

	return C.size_t(i), C.bool(fc.clientHasClosed())
}

//export go_apache_request_headers
func go_apache_request_headers(threadIndex C.uintptr_t) (*C.go_string, C.size_t) {
	thread := phpThreads[threadIndex]
	ctx := thread.context()
	fc := thread.frankenPHPContext()

	if fc.responseWriter == nil {
		// worker mode, not handling a request

		if globalLogger.Enabled(ctx, slog.LevelDebug) {
			globalLogger.LogAttrs(ctx, slog.LevelDebug, "apache_request_headers() called in non-HTTP context", slog.String("worker", fc.worker.name))
		}

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

func addHeader(ctx context.Context, fc *frankenPHPContext, cString *C.char, length C.int) {
	key, val := splitRawHeader(cString, int(length))
	if key == "" {
		if fc.logger.Enabled(ctx, slog.LevelDebug) {
			fc.logger.LogAttrs(ctx, slog.LevelDebug, "invalid header", slog.String("header", C.GoStringN(cString, length)))
		}

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
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()
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

		addHeader(thread.context(), fc, h.header, C.int(h.header_len))
		current = current.next
	}

	goStatus := int(status)

	// go panics on invalid status code
	// https://github.com/golang/go/blob/9b8742f2e79438b9442afa4c0a0139d3937ea33f/src/net/http/server.go#L1162
	if goStatus < 100 || goStatus > 999 {
		ctx := thread.context()

		if globalLogger.Enabled(ctx, slog.LevelWarn) {
			globalLogger.LogAttrs(ctx, slog.LevelWarn, "Invalid response status code", slog.Int("status_code", goStatus))
		}

		goStatus = 500
	}

	fc.responseWriter.WriteHeader(goStatus)

	if goStatus < 200 {
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
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()
	if fc == nil {
		return false
	}

	if fc.responseWriter == nil {
		return false
	}

	if fc.clientHasClosed() && !fc.isDone {
		return true
	}

	if err := http.NewResponseController(fc.responseWriter).Flush(); err != nil {
		ctx := thread.context()

		if globalLogger.Enabled(ctx, slog.LevelWarn) {
			globalLogger.LogAttrs(ctx, slog.LevelWarn, "the current responseWriter is not a flusher, if you are not using a custom build, please report this issue", slog.Any("error", err))
		}
	}

	return false
}

//export go_read_post
func go_read_post(threadIndex C.uintptr_t, cBuf *C.char, countBytes C.size_t) (readBytes C.size_t) {
	fc := phpThreads[threadIndex].frankenPHPContext()

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
	request := phpThreads[threadIndex].frankenPHPContext().request
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
func go_log(threadIndex C.uintptr_t, message *C.char, level C.int) {
	ctx := phpThreads[threadIndex].context()

	m := C.GoString(message)
	le := syslogLevelInfo

	if level >= C.int(syslogLevelEmerg) && level <= C.int(syslogLevelDebug) {
		le = syslogLevel(level)
	}

	switch le {
	case syslogLevelEmerg, syslogLevelAlert, syslogLevelCrit, syslogLevelErr:
		if globalLogger.Enabled(ctx, slog.LevelError) {
			globalLogger.LogAttrs(ctx, slog.LevelError, m, slog.String("syslog_level", syslogLevel(level).String()))
		}

	case syslogLevelWarn:
		if globalLogger.Enabled(ctx, slog.LevelWarn) {
			globalLogger.LogAttrs(ctx, slog.LevelWarn, m, slog.String("syslog_level", syslogLevel(level).String()))
		}
	case syslogLevelDebug:
		if globalLogger.Enabled(ctx, slog.LevelDebug) {
			globalLogger.LogAttrs(ctx, slog.LevelDebug, m, slog.String("syslog_level", syslogLevel(level).String()))
		}

	default:
		if globalLogger.Enabled(ctx, slog.LevelInfo) {
			globalLogger.LogAttrs(ctx, slog.LevelInfo, m, slog.String("syslog_level", syslogLevel(level).String()))
		}
	}
}

//export go_log_attrs
func go_log_attrs(threadIndex C.uintptr_t, message *C.zend_string, level C.zend_long, cattrs *C.zval) *C.char {
	var attrs map[string]any

	if cattrs == nil {
		attrs = nil
	} else {
		var err error
		if attrs, err = GoMap[any](unsafe.Pointer(cattrs)); err != nil {
			// NOTE: return value is already formatted for a PHP exception message.
			return C.CString("Failed to log message: converting attrs: " + err.Error())
		}
	}

	ctx := phpThreads[threadIndex].context()

	if globalLogger.Enabled(ctx, slog.Level(level)) {
		globalLogger.LogAttrs(ctx,
			slog.Level(level),
			GoString(unsafe.Pointer(message)),
			mapToAttr(attrs)...)
	}

	return nil
}

func mapToAttr(input map[string]any) []slog.Attr {
	out := make([]slog.Attr, 0, len(input))

	for key, val := range input {
		out = append(out, slog.Any(key, val))
	}

	return out
}

//export go_is_context_done
func go_is_context_done(threadIndex C.uintptr_t) C.bool {
	return C.bool(phpThreads[threadIndex].frankenPHPContext().isDone)
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

func resetGlobals() {
	globalMu.Lock()
	globalCtx = context.Background()
	globalLogger = slog.Default()
	workers = nil
	watcherIsEnabled = false
	globalMu.Unlock()
}
