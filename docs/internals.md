# Internals

This document explains FrankenPHP's internal architecture, focusing on thread management, the state machine, and the CGO boundary between Go and C/PHP.

## Overview

FrankenPHP embeds the PHP interpreter directly into Go via CGO. Each PHP execution runs on a real POSIX thread (not a goroutine) because PHP's ZTS (Zend Thread Safety) model requires it. Go orchestrates these threads through a state machine, while C handles the PHP SAPI lifecycle.

The main layers are:

1. **Go layer** (top-level `*.go` files such as `frankenphp.go`, `phpthread.go`, `thread*.go`, `scaling.go`): Thread pool management, request routing, auto-scaling
2. **C layer** (`frankenphp.c`, `frankenphp.h`): PHP SAPI implementation, script execution loop, superglobal management
3. **State machine** (`internal/state/`): Synchronization between Go goroutines and C threads

## Thread Types

### Main Thread (`phpmainthread.go`)

The main PHP thread (`phpMainThread`) initializes the PHP runtime:

1. Applies `php.ini` overrides
2. Takes a snapshot of the environment (`main_thread_env`) for sandboxing
3. Starts the PHP SAPI module
4. Signals readiness to the Go side

It stays alive for the lifetime of the server. All other threads are started after it signals `Ready`.

### Regular Threads (`threadregular.go`)

Handle classic one-request-per-invocation PHP scripts. Each request:

1. Receives a request via `requestChan` or the shared `regularRequestChan`
2. Returns the script filename from `beforeScriptExecution()`
3. The C layer executes the PHP script
4. `afterScriptExecution()` closes the request context

### Worker Threads (`threadworker.go`)

Keep a PHP script alive across multiple requests. The PHP script calls `frankenphp_handle_request()` in a loop:

1. `beforeScriptExecution()` returns the worker script filename
2. The C layer starts executing the PHP script
3. The PHP script calls `frankenphp_handle_request()`, which calls `waitForWorkerRequest()` in Go
4. Go blocks until a request arrives, then sets up the request context
5. The PHP callback handles the request
6. `go_frankenphp_finish_worker_request()` cleans up the request context
7. The PHP script loops back to step 3

After the script exits, the worker is restarted immediately if it had reached `frankenphp_handle_request()` at least once (whether the exit was clean or the result of a fatal error). Exponential backoff is only applied to consecutive startup failures, where the script exits before ever reaching `frankenphp_handle_request()`.

## Thread State Machine

Each thread has a `ThreadState` (defined in `internal/state/state.go`) that governs its lifecycle. The state machine uses a `sync.RWMutex` for all state transitions and a channel-based subscriber pattern for blocking waits.

### States

```text
Lifecycle:        Reserved → BootRequested → Booting → Inactive → Ready ⇄ (processing)
                                                                    ↓
Shutdown:                                                     ShuttingDown → Done → Reserved
                                                                    ↑
Restart (admin/watcher):                                      Restarting → Yielding → Ready
                                                                    ↑
ZTS reboot (max_requests):                                    Rebooting → RebootReady → Ready
                                                                    ↑
Handler transition:                                       TransitionRequested → TransitionInProgress → TransitionComplete
```

The full set of states is defined in `internal/state/state.go`:

| State                  | Description                                                                          |
| ---------------------- | ------------------------------------------------------------------------------------ |
| `Reserved`             | Thread slot allocated but not booted. Can be booted on demand.                       |
| `BootRequested`        | Boot has been queued (e.g., by the main thread) but the POSIX thread hasn't started. |
| `Booting`              | Underlying POSIX thread is starting up.                                              |
| `Inactive`             | Thread is alive but has no handler assigned. Minimal memory footprint.               |
| `Ready`                | Thread has a handler and is ready to accept work.                                    |
| `ShuttingDown`         | Thread is shutting down.                                                             |
| `Done`                 | Thread has completely shut down. Transitions back to `Reserved` for potential reuse. |
| `Restarting`           | Worker thread is being restarted (e.g., via admin API or file watcher).              |
| `Yielding`             | Worker thread has yielded control and is waiting to be re-activated.                 |
| `Rebooting`            | Worker thread is exiting the C loop for a full ZTS restart (e.g., `max_requests`).   |
| `RebootReady`          | The C thread has exited and ZTS state is cleaned up, ready to spawn a new C thread.  |
| `TransitionRequested`  | A handler change has been requested from the Go side.                                |
| `TransitionInProgress` | The C thread has acknowledged the transition request.                                |
| `TransitionComplete`   | The Go side has installed the new handler.                                           |

### Key Operations

**`RequestSafeStateChange(nextState)`**: The primary way external goroutines request state changes. It:

- Atomically succeeds from `Ready` or `Inactive` (under mutex)
- Returns `false` immediately from `ShuttingDown`, `Done`, or `Reserved`
- Blocks and retries from any other state, waiting for `Ready`, `Inactive`, or `ShuttingDown`

This guarantees mutual exclusion: only one of `shutdown()`, `setHandler()`, or `drainWorkerThreads()` can succeed at a time on a given thread.

**`WaitFor(states...)`**: Blocks until the thread reaches one of the specified states. Uses a channel-based subscriber pattern so waiters are efficiently notified.

**`Set(nextState)`**: Unconditional state change. Used by the thread itself (from C callbacks) to signal state transitions.

**`CompareAndSwap(compareTo, swapTo)`**: Atomic compare-and-swap. Used for boot initialization.

### Handler Transition Protocol

When a thread needs to change its handler (e.g., from inactive to worker):

```text
Go side (setHandler)           C side (PHP thread)
─────────────────              ─────────────────
RequestSafeStateChange(
  TransitionRequested)
close(drainChan)
                               detects drain
                               Set(TransitionInProgress)
WaitFor(TransitionInProgress)
  → unblocked                  WaitFor(TransitionComplete)
handler = newHandler
drainChan = make(chan struct{})
Set(TransitionComplete)
                                 → unblocked
                               newHandler.beforeScriptExecution()
```

This protocol ensures the handler pointer is never read and written concurrently.

### Worker Restart Protocol

When workers are restarted (e.g., via admin API):

```text
Go side (RestartWorkers)       C side (worker thread)
─────────────────              ─────────────────
RequestSafeStateChange(
  Restarting)
close(drainChan)
                               detects drain in waitForWorkerRequest()
                               returns false → PHP script exits
                               beforeScriptExecution():
                                 state is Restarting →
                                 Set(Yielding)
WaitFor(Yielding)
  → unblocked                    WaitFor(Ready, ShuttingDown)
drainChan = make(chan struct{})
Set(Ready)
                                 → unblocked
                               beforeScriptExecution() recurse:
                                 state is Ready → normal execution
```

## CGO Boundary

### Exported Go Functions

C code calls Go functions via CGO exports. The main callbacks are:

| Function                                    | Called when                                      |
| ------------------------------------------- | ------------------------------------------------ |
| `go_frankenphp_before_script_execution`     | C loop needs the next script to execute          |
| `go_frankenphp_after_script_execution`      | PHP script has finished executing                |
| `go_frankenphp_worker_handle_request_start` | Worker's `frankenphp_handle_request()` is called |
| `go_frankenphp_finish_worker_request`       | Worker request handler has returned              |
| `go_ub_write`                               | PHP produces output (`echo`, etc.)               |
| `go_read_post`                              | PHP reads POST body (`php://input`)              |
| `go_read_cookies`                           | PHP reads cookies                                |
| `go_write_headers`                          | PHP sends response headers                       |
| `go_sapi_flush`                             | PHP flushes output                               |
| `go_log_attrs`                              | PHP logs a structured message                    |

All these functions receive a `threadIndex` parameter identifying the calling thread. This is a thread-local variable in C (`__thread uintptr_t thread_index`) set during thread initialization.

### C Thread Main Loop

Each PHP thread runs `php_thread()` in `frankenphp.c`:

```c
while ((scriptName = go_frankenphp_before_script_execution(thread_index))) {
    php_request_startup();
    php_execute_script(&file_handle);
    php_request_shutdown();
    go_frankenphp_after_script_execution(thread_index, exit_status);
}
```

Bailouts (fatal PHP errors) are caught by `zend_catch`, which marks the thread as unhealthy and forces cleanup.

### Memory Management

- **Go → C strings**: `C.CString()` allocates with `malloc()`. The C side is responsible for freeing (e.g., `frankenphp_free_request_context()` frees cookie data).
- **Go string pinning**: `phpThread` (in `phpthread.go`) embeds Go's [`runtime.Pinner`](https://pkg.go.dev/runtime#Pinner). `thread.Pin()` / `thread.Unpin()` keep Go memory referenced from C alive without copying it. The thread is unpinned after each script execution.
- **PHP memory**: Managed by Zend's memory manager (`emalloc`/`efree`). Automatically freed at request shutdown.

## Auto-Scaling

FrankenPHP can automatically scale the number of PHP threads based on demand (`scaling.go`).

### Configuration

- `num_threads`: Initial number of threads started at boot
- `max_threads`: Maximum number of threads allowed (includes auto-scaled)

### Upscaling

A dedicated goroutine reads from an unbuffered `scaleChan`:

1. A request handler can't find an available thread
2. It sends the request context to `scaleChan`
3. The scaling goroutine checks:
   - Has the request been stalled long enough? (minimum 5ms)
   - Is CPU usage below the threshold? (80%)
   - Is the thread limit reached?
4. If all checks pass, a new thread is booted and assigned

### Downscaling

A separate goroutine periodically checks (every 5s) for idle auto-scaled threads. Threads in `Ready` state idle longer than `maxIdleTime` (default 5s) are converted to `Inactive` (up to 10 per cycle). They are not fully stopped: a code path exists for that, but it is currently disabled because some PECL extensions leak memory and prevent threads from cleanly shutting down.

## Environment Sandboxing

FrankenPHP sandboxes environment variables per-thread:

1. At startup, the main thread snapshots `os.Environ()` into `main_thread_env` (a PHP `HashTable`).
2. `$_SERVER` is built from a copy of `main_thread_env` plus request-specific variables (in `frankenphp_register_server_vars`). It is rebuilt for every request, including each iteration of a worker script.
3. `$_ENV` is populated from the same snapshot through PHP's `php_import_environment_variables` hook. In regular mode this happens once per script execution; in worker mode it happens once when the worker script starts and is **not** rebuilt between worker requests, which is why writes to `$_ENV` leak across requests (see [Worker Mode](worker.md)).
4. `frankenphp_putenv()` / `frankenphp_getenv()` operate on a thread-local `sandboxed_env` initialized lazily from `main_thread_env`, preventing race conditions on the global C environment.
5. `reset_sandboxed_environment()` releases `sandboxed_env` after each PHP script execution. In regular mode that's per request; in worker mode it only runs when the worker script itself exits, so `putenv()` writes are visible to subsequent worker requests on the same thread until the script restarts.

## Request Flow (Regular Mode)

1. HTTP request arrives at Caddy
2. FrankenPHP's Caddy module resolves the PHP script path
3. A `frankenPHPContext` is created with the request and script info
4. The context is sent to an available regular thread via `requestChan`
5. The thread's `beforeScriptExecution()` returns the script filename
6. The C layer executes the PHP script
7. During execution, Go callbacks handle I/O (`go_ub_write`, `go_read_post`, etc.)
8. After execution, `afterScriptExecution()` signals completion
9. The response is sent to the client

## Request Flow (Worker Mode)

1. HTTP request arrives at Caddy
2. FrankenPHP's Caddy module resolves the worker for this request
3. A `frankenPHPContext` is created
4. The context is sent to the worker's `requestChan` or a specific thread's `requestChan`
5. The worker thread's `waitForWorkerRequest()` receives it
6. PHP's `frankenphp_handle_request()` callback is invoked
7. After the callback returns, `go_frankenphp_finish_worker_request()` cleans up
8. The worker loops back to `waitForWorkerRequest()`
