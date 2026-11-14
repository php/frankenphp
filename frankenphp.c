#include "frankenphp.h"
#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <Zend/zend_smart_str.h>
#include <errno.h>
#include <ext/date/php_date.h>
#include <ext/spl/spl_exceptions.h>
#include <ext/standard/basic_functions.h>
#include <ext/standard/head.h>
#include <ext/standard/info.h>
#include <ext/standard/php_fopen_wrappers.h>
#ifdef HAVE_PHP_SESSION
#include <ext/session/php_session.h>
#endif
#include <inttypes.h>
#include <php.h>
#ifdef PHP_WIN32
#include <config.w32.h>
#include <win32/sockets.h>
#else
#include <php_config.h>
#endif
#include <main/php_streams.h>
#include <php_ini.h>
#include <php_main.h>
#include <php_network.h>
#include <php_output.h>
#include <php_variables.h>
#include <php_version.h>
#include <pthread.h>
#include <sapi/embed/php_embed.h>
#include <signal.h>
#include <stdatomic.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#ifndef PHP_WIN32
#include <fcntl.h>
#include <unistd.h>
#endif
#if defined(__linux__)
#include <sys/prctl.h>
#elif defined(__FreeBSD__)
#include <pthread_np.h>
#include <sys/procctl.h>
#elif defined(__OpenBSD__)
#include <pthread_np.h>
#endif
#if defined(__APPLE__) || defined(__OpenBSD__) || defined(__NetBSD__) ||       \
    defined(__DragonFly__)
#define FRANKENPHP_KQUEUE_PARENT_DEATH 1
#include <sys/event.h>
#include <sys/types.h>
#endif

#if PHP_VERSION_ID >= 80600
#include <sapi/cli/cli.h>
#endif
#include "emulate_php_cli.h"

#include "_cgo_export.h"
#include "frankenphp_arginfo.h"
#include "zval.h"

#if defined(PHP_WIN32) && defined(ZTS)
ZEND_TSRMLS_CACHE_DEFINE()
#endif

/**
 * The list of modules to reload on each request. If an external module
 * requires to be reloaded between requests, it is possible to hook on
 * `sapi_module.activate` and `sapi_module.deactivate`.
 *
 * @see https://github.com/DataDog/dd-trace-php/pull/3169 for an example
 */
static const char *MODULES_TO_RELOAD[] = {
    "filter",
#ifndef HAVE_PHP_SESSION
    /* When the session extension is not visible at build time (e.g. it is
     * loaded as a shared module), frankenphp_reset_session_state() is
     * compiled out. Reset the session module through the registry instead so
     * PS(id) is cleared between worker requests and a stale id cannot bleed
     * across concurrent clients. See GHSA-v3ph-cgqh-r8p5.
     *
     * Unlike frankenphp_reset_session_state(), the module RSHUTDOWN frees
     * save handlers registered from the worker bootstrap; that is an
     * acceptable trade-off for a build where the extension isn't linked. */
    "session",
#endif
    NULL};

frankenphp_version frankenphp_get_version() {
  return (frankenphp_version){
      PHP_MAJOR_VERSION, PHP_MINOR_VERSION, PHP_RELEASE_VERSION,
      PHP_EXTRA_VERSION, PHP_VERSION,       PHP_VERSION_ID,
  };
}

frankenphp_config frankenphp_get_config() {
  return (frankenphp_config){
#ifdef ZTS
      true,
#else
      false,
#endif
#ifdef ZEND_SIGNALS
      true,
#else
      false,
#endif
#ifdef ZEND_MAX_EXECUTION_TIMERS
      true,
#else
      false,
#endif
  };
}

bool should_filter_var = 0;
bool original_user_abort_setting = 0;
frankenphp_interned_strings_t frankenphp_strings = {0};
HashTable *main_thread_env = NULL;

#if defined(__ELF__) && defined(__GNUC__)
#define THREAD_LOCAL __thread __attribute__((tls_model("local-exec")))
#else
#define THREAD_LOCAL __thread
#endif

static THREAD_LOCAL uintptr_t thread_index;
static THREAD_LOCAL bool is_worker_thread = false;
static THREAD_LOCAL bool is_background_worker = false;
/* Stop socket pair of a background worker thread: [0] is the script's end,
 * exposed via WorkerHandle::getStream(); [1] is transferred to the Go
 * side, which closes it to signal a drain. */
static THREAD_LOCAL php_socket_t worker_stop_socks[2] = {SOCK_ERR, SOCK_ERR};
/* set by the first WorkerHandle::tick() of the current run, the ready
 * point of a background worker */
static THREAD_LOCAL bool worker_ticked = false;
/* name of the background worker the thread runs, for WorkerHandle::getName() */
static THREAD_LOCAL zend_string *worker_name = NULL;
/* putenv() overrides layered over the other env tables, an IS_NULL entry
 * unsets the variable */
static THREAD_LOCAL HashTable *sandboxed_env = NULL;
/* prepared_env holds entries from php(_server)'s `env KEY VAL`, exposed to
 * getenv() and merged into $_ENV when 'E' is in variables_order. Separate from
 * putenv() so those don't leak into $_ENV. Released after each worker request
 * so its entries don't leak into the next one. */
static THREAD_LOCAL HashTable *prepared_env = NULL;
/* prepared_env of the worker script boot, kept for all its requests */
static THREAD_LOCAL HashTable *worker_env = NULL;

/* Published via SG(server_context) so ext-parallel children, which inherit
 * the parent's SG(server_context), can route SAPI callbacks back to the
 * parent's thread_index instead of their zero-initialized TLS. */
typedef struct {
  uintptr_t thread_index;
} frankenphp_server_ctx;
static THREAD_LOCAL frankenphp_server_ctx frankenphp_local_server_ctx;

uintptr_t frankenphp_thread_index(void) {
  frankenphp_server_ctx *ctx = (frankenphp_server_ctx *)SG(server_context);
  /* Fall back to the OS thread's own TLS before
   * frankenphp_update_request_context(). */
  return ctx == NULL ? thread_index : ctx->thread_index;
}

/* Whether the calling thread is one FrankenPHP did not start, a child of
 * ext-parallel for instance: it runs with the server context of its parent,
 * and so with its index, while what is thread-local here is its own. */
static bool frankenphp_is_foreign_thread(void) {
  return SG(server_context) != NULL &&
         SG(server_context) != &frankenphp_local_server_ctx;
}

#ifndef PHP_WIN32
static bool is_forked_child = false;
static pid_t fork_parent_pid = 0;

static void frankenphp_fork_prepare(void) { fork_parent_pid = getpid(); }

static void frankenphp_mark_fork_child(void) { is_forked_child = true; }

#if defined(FRANKENPHP_KQUEUE_PARENT_DEATH)
/* Watcher thread for platforms without a kernel parent-death signal.
 * Blocks in kevent() until the parent exits, then force-kills this child. */
static void *frankenphp_parent_death_watcher(void *arg) {
  pid_t ppid = (pid_t)(intptr_t)arg;
  int kq = kqueue();
  if (kq < 0) {
    _exit(1);
  }
  struct kevent kev;
  EV_SET(&kev, ppid, EVFILT_PROC, EV_ADD | EV_ONESHOT, NOTE_EXIT, 0, NULL);
  if (kevent(kq, &kev, 1, NULL, 0, NULL) < 0) {
    _exit(1);
  }
  struct kevent event;
  while (kevent(kq, NULL, 0, &event, 1, NULL) < 0 && errno == EINTR)
    ;
  _exit(1);
}
#endif

static void frankenphp_fork_child(void) {
  // a fork of a fork (e.g. a daemonizer) must outlive its short-lived parent
  if (is_forked_child) {
    return;
  }
  frankenphp_mark_fork_child();
#if defined(__linux__)
  // if the parent process dies between fork() and this prctl()
  if (prctl(PR_SET_PDEATHSIG, SIGKILL, 0, 0, 0) != 0 ||
      getppid() != fork_parent_pid) {
    _exit(1);
  }
#elif defined(__FreeBSD__)
  /* FreeBSD analogue of PR_SET_PDEATHSIG - except with
   * parent process death, rather than parent thread death. */
  int sig = SIGKILL;
  if (procctl(P_PID, 0, PROC_PDEATHSIG_CTL, &sig) != 0 ||
      getppid() != fork_parent_pid) {
    _exit(1);
  }
#elif defined(FRANKENPHP_KQUEUE_PARENT_DEATH)
  /* No kernel parent-death signal available; spawn a watcher that
   * blocks on EVFILT_PROC/NOTE_EXIT. */
  if (getppid() != fork_parent_pid) {
    _exit(1);
  }
  pthread_t watcher;
  if (pthread_create(&watcher, NULL, frankenphp_parent_death_watcher,
                     (void *)(intptr_t)fork_parent_pid) != 0) {
    _exit(1);
  }
  pthread_detach(watcher);
#endif
}

static void frankenphp_register_atfork(void) {
  pthread_atfork(frankenphp_fork_prepare, NULL, frankenphp_fork_child);
}

static void frankenphp_register_cli_atfork(void) {
  pthread_atfork(NULL, NULL, frankenphp_mark_fork_child);
}

/* pcntl signals delivered to a Go M segfault on PCNTL_G (no TSRM there)
 * Block these in a constructor so Go's schedinit captures
 * the mask and every M inherits it; execute_script_cli unblocks on its own
 * pthread. Caddy's `signal.Notify` keeps working via `runtime.ensureSigM`.
 * Limited to async-notify signals: Go's minitSignalMask re-unblocks anything
 * flagged _SigKill/_SigThrow/_SigUnblock on every M anyway. */
static void frankenphp_fill_cli_signal_set(sigset_t *s) {
  sigemptyset(s);
#ifdef SIGUSR1
  sigaddset(s, SIGUSR1);
#endif
#ifdef SIGUSR2
  sigaddset(s, SIGUSR2);
#endif
#ifdef SIGALRM
  sigaddset(s, SIGALRM);
#endif
}

__attribute__((constructor)) static void frankenphp_libpreinit(void) {
  sigset_t set;
  frankenphp_fill_cli_signal_set(&set);
  /* Single-threaded at this point (constructors run before Go's runtime),
   * so sigprocmask is sufficient and portable. */
  sigprocmask(SIG_BLOCK, &set, NULL);
}
#endif

/* Best-effort force-kill for stuck PHP threads.
 *
 * Each thread captures &EG(vm_interrupt) / &EG(timed_out) at boot and
 * hands them to Go via go_frankenphp_store_force_kill_slot. To kill,
 * Go passes the slot back to frankenphp_force_kill_thread, which stores
 * true into both bools (the VM bails through zend_timeout() at the next
 * opcode boundary) and then wakes any in-flight syscall:
 *   - Linux/FreeBSD: pthread_kill(SIGRTMIN+3) -> EINTR.
 *   - Windows: CancelSynchronousIo + QueueUserAPC for alertable I/O +
 *     SleepEx. Non-alertable Sleep (including PHP's usleep) stays stuck.
 *   - macOS: atomic-bool only; busy loops bail, blocking syscalls don't.
 *
 * Reserved signal: SIGRTMIN+3. PHP's pcntl_signal(SIGRTMIN+3, ...)
 * clobbers it. glibc NPTL reserves SIGRTMIN..SIGRTMIN+2; embedders with
 * their own Go signal usage may need to patch this constant.
 *
 * The slot lives Go-side on phpThread; the C side has no global table.
 * The signal handler is installed once via pthread_once. */
#ifdef PHP_WIN32
static void CALLBACK frankenphp_noop_apc(ULONG_PTR param) { (void)param; }
#endif

#ifdef FRANKENPHP_HAS_KILL_SIGNAL
/* No-op: delivery itself is what unblocks the syscall via EINTR. */
static void frankenphp_kill_signal_handler(int sig) { (void)sig; }

static pthread_once_t kill_signal_handler_installed = PTHREAD_ONCE_INIT;
/* Set to true only after sigaction() succeeds. force_kill_thread skips
 * pthread_kill when this is false, so a sigaction failure (invalid
 * signal number, exhausted handler slots, etc.) can't deliver the
 * signal with its default action (process termination). */
static atomic_bool kill_signal_handler_active;
static void install_kill_signal_handler(void) {
  /* No SA_RESTART so syscalls return EINTR rather than being restarted.
   * SA_ONSTACK guards against an accidental process-level delivery to a
   * Go-managed thread, where Go requires the alternate signal stack. */
  struct sigaction sa;
  memset(&sa, 0, sizeof(sa));
  sa.sa_handler = frankenphp_kill_signal_handler;
  sigemptyset(&sa.sa_mask);
  sa.sa_flags = SA_ONSTACK;
  if (sigaction(FRANKENPHP_KILL_SIGNAL, &sa, NULL) == 0) {
    atomic_store(&kill_signal_handler_active, true);
  }
}
#endif

/* Must run on the PHP thread itself: EG() resolves to its own TSRM
 * context and pthread_self() captures the right tid. */
static void frankenphp_register_thread_for_kill(uintptr_t idx) {
  force_kill_slot slot;
  memset(&slot, 0, sizeof(slot));
  slot.vm_interrupt = &EG(vm_interrupt);
  slot.timed_out = &EG(timed_out);
#ifdef FRANKENPHP_HAS_KILL_SIGNAL
  slot.tid = pthread_self();
  pthread_once(&kill_signal_handler_installed, install_kill_signal_handler);
#elif defined(PHP_WIN32)
  if (!DuplicateHandle(GetCurrentProcess(), GetCurrentThread(),
                       GetCurrentProcess(), &slot.thread_handle, 0, FALSE,
                       DUPLICATE_SAME_ACCESS)) {
    /* On failure, force_kill falls back to atomic-bool only. */
    slot.thread_handle = NULL;
  }
#endif
  go_frankenphp_store_force_kill_slot(idx, slot);
}

void frankenphp_force_kill_thread(force_kill_slot slot) {
  if (slot.vm_interrupt == NULL) {
    /* Boot aborted before the slot was published. */
    return;
  }

  /* Atomic stores first: by the time the thread wakes (signal-driven or
   * natural) the VM sees them and bails through zend_timeout(). */
  frankenphp_atomic_bool_store(slot.timed_out, true);
  frankenphp_atomic_bool_store(slot.vm_interrupt, true);

#ifdef FRANKENPHP_HAS_KILL_SIGNAL
  /* ESRCH (thread already exited) / EINVAL are both benign here.
   * Skip if sigaction() failed at install time: delivering an unhandled
   * SIGRTMIN+3 would terminate the process. */
  if (atomic_load(&kill_signal_handler_active)) {
    pthread_kill(slot.tid, FRANKENPHP_KILL_SIGNAL);
  }
#elif defined(PHP_WIN32)
  if (slot.thread_handle != NULL) {
    CancelSynchronousIo(slot.thread_handle);
    QueueUserAPC((PAPCFUNC)frankenphp_noop_apc, slot.thread_handle, 0);
  }
#endif
}

/* CloseHandle on Windows; no-op on POSIX. */
void frankenphp_release_thread_for_kill(force_kill_slot slot) {
#ifdef PHP_WIN32
  if (slot.thread_handle != NULL) {
    CloseHandle(slot.thread_handle);
  }
#else
  (void)slot;
#endif
}

/* Milliseconds on a clock that never goes back, for the deadlines of the
 * waits. */
static int64_t frankenphp_now_ms(void) {
#ifdef PHP_WIN32
  return (int64_t)GetTickCount64();
#else
  struct timespec now;
  clock_gettime(CLOCK_MONOTONIC, &now);

  return (int64_t)now.tv_sec * 1000 + now.tv_nsec / 1000000;
#endif
}

/* A thread that waits does so in slices of this long, so that a force-kill no
 * signal carries to it is noticed at the next one. */
#define FRANKENPHP_WAIT_SLICE_MS 1000

/* Socket pairs of background workers: the handle of a thread, see
 * WorkerHandle::getStream(). One end is exposed to a PHP script as a stream,
 * the other is held by the Go side, which writes wake-ups to it and closes it
 * to land EOF on the script's end. A pair rather than a pipe because on
 * Windows php_select() only really waits on sockets: before 8.5 it reports
 * any other handle as always ready. */
static void frankenphp_sock_close(php_socket_t s) {
  if (s == SOCK_ERR) {
    return;
  }
  /* php_network.h maps closesocket to close outside Windows */
  closesocket(s);
}

/* keep the pair out of processes the script may spawn: a child holding the Go
 * end would keep the script's end from ever reaching EOF */
static void frankenphp_sock_no_inherit(php_socket_t s) {
#ifdef PHP_WIN32
  SetHandleInformation((HANDLE)s, HANDLE_FLAG_INHERIT, 0);
#else
  fcntl(s, F_SETFD, FD_CLOEXEC);
#endif
}

/* Opens a pair: [0] for the script, [1] for the Go side. The Go side's end
 * never blocks: its writes are wake-ups, a full buffer means the peer has
 * plenty of unread ones already. */
#ifdef PHP_WIN32
/* Windows has no socketpair() and PHP's emulation is not one: it binds a
 * listener to INADDR_ANY, reachable off the machine, and hands back whichever
 * connection arrives first. The pair is formed by hand instead, the way
 * libevent and Tor do it: loopback alone, and a connection is kept only when
 * it comes from the socket we connected with. */
#define FRANKENPHP_SOCK_PAIR_TRIES 8

#ifndef WSA_FLAG_NO_HANDLE_INHERIT
#define WSA_FLAG_NO_HANDLE_INHERIT 0x80
#endif

/* Not inheritable from the start: a process another thread spawns before
 * SetHandleInformation() would keep a copy, and delay the EOF of the drain
 * until it exits. Overlapped, as socket() makes them. */
static SOCKET frankenphp_sock_win32(void) {
  return WSASocketW(AF_INET, SOCK_STREAM, IPPROTO_TCP, NULL, 0,
                    WSA_FLAG_OVERLAPPED | WSA_FLAG_NO_HANDLE_INHERIT);
}

static int frankenphp_sock_pair_win32(php_socket_t socks[2]) {
  SOCKET listener = INVALID_SOCKET, client = INVALID_SOCKET,
         accepted = INVALID_SOCKET;
  struct sockaddr_in addr, local, peer;
  int addr_len = sizeof(addr), local_len = sizeof(local), peer_len,
      exclusive = 1;

  socks[0] = SOCK_ERR;
  socks[1] = SOCK_ERR;

  listener = frankenphp_sock_win32();
  if (listener == INVALID_SOCKET) {
    return -1;
  }

  /* nobody else may take this port from under the listener */
  setsockopt(listener, SOL_SOCKET, SO_EXCLUSIVEADDRUSE,
             (const char *)&exclusive, sizeof(exclusive));

  memset(&addr, 0, sizeof(addr));
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
  addr.sin_port = 0;

  if (bind(listener, (struct sockaddr *)&addr, sizeof(addr)) != 0 ||
      getsockname(listener, (struct sockaddr *)&addr, &addr_len) != 0 ||
      listen(listener, 1) != 0) {
    goto error;
  }

  client = frankenphp_sock_win32();
  if (client == INVALID_SOCKET ||
      connect(client, (struct sockaddr *)&addr, sizeof(addr)) != 0 ||
      getsockname(client, (struct sockaddr *)&local, &local_len) != 0) {
    goto error;
  }

  for (int attempt = 0; attempt < FRANKENPHP_SOCK_PAIR_TRIES; attempt++) {
    memset(&peer, 0, sizeof(peer));
    peer_len = sizeof(peer);
    accepted = accept(listener, (struct sockaddr *)&peer, &peer_len);
    if (accepted == INVALID_SOCKET) {
      goto error;
    }
    if (peer.sin_port == local.sin_port &&
        peer.sin_addr.s_addr == local.sin_addr.s_addr) {
      closesocket(listener);
      socks[0] = accepted;
      socks[1] = client;

      return 0;
    }

    /* somebody else got in: drop them and wait for our own connection */
    closesocket(accepted);
    accepted = INVALID_SOCKET;
  }

error:
  if (listener != INVALID_SOCKET) {
    closesocket(listener);
  }
  if (client != INVALID_SOCKET) {
    closesocket(client);
  }
  if (accepted != INVALID_SOCKET) {
    closesocket(accepted);
  }

  return -1;
}
#endif

static int frankenphp_sock_pair_open(php_socket_t socks[2]) {
#ifdef PHP_WIN32
  if (frankenphp_sock_pair_win32(socks) != 0) {
    return -1;
  }
#else
#ifdef SOCK_CLOEXEC
  int type = SOCK_STREAM | SOCK_CLOEXEC;
#else
  int type = SOCK_STREAM;
#endif
  if (socketpair(AF_UNIX, type, 0, socks) != 0) {
    socks[0] = SOCK_ERR;
    socks[1] = SOCK_ERR;

    return -1;
  }
#endif
  /* redundant where SOCK_CLOEXEC applied; a fork()ed child (pcntl) still
   * inherits both ends and delays the EOF until it exits */
  frankenphp_sock_no_inherit(socks[0]);
  frankenphp_sock_no_inherit(socks[1]);
  php_set_sock_blocking(socks[1], 0);
#ifdef PHP_WIN32
  /* loopback TCP: a byte-sized wake-up must not wait for Nagle and the
   * delayed ACK */
  int nodelay = 1;
  setsockopt(socks[0], IPPROTO_TCP, TCP_NODELAY, (const char *)&nodelay,
             sizeof(nodelay));
  setsockopt(socks[1], IPPROTO_TCP, TCP_NODELAY, (const char *)&nodelay,
             sizeof(nodelay));
#endif
#ifdef SO_NOSIGPIPE
  /* a wake-up to a closed peer must fail, not raise (MSG_NOSIGNAL elsewhere) */
  int one = 1;
  setsockopt(socks[1], SOL_SOCKET, SO_NOSIGPIPE, &one, sizeof(one));
#endif

  return 0;
}

/* best-effort wake-up on the Go side's end of a pair */
static void frankenphp_sock_send(php_socket_t s, const char *buf, size_t len) {
#ifdef MSG_NOSIGNAL
  int flags = MSG_NOSIGNAL;
#else
  int flags = 0;
#endif
  (void)send(s, buf, (int)len, flags);
}

static void frankenphp_worker_close_stop_socks(void) {
  for (int i = 0; i < 2; i++) {
    frankenphp_sock_close(worker_stop_socks[i]);
    worker_stop_socks[i] = SOCK_ERR;
  }
}

/* Resets the background worker state of the calling thread. The streams of
 * WorkerHandle::getStream() do not own the socket, and request shutdown
 * destroyed the handles they belong to. */
static void frankenphp_reset_background_worker(void) {
  is_background_worker = false;
  worker_ticked = false;
  if (worker_name != NULL) {
    zend_string_release_ex(worker_name, 1);
    worker_name = NULL;
  }
  frankenphp_worker_close_stop_socks();
}

/* Marks the calling thread as the background worker of that name, opens its
 * stop socket pair and transfers the Go side's end to the caller, clearing the
 * TLS slot so a later recycle won't double-close it. Returns -1 if the pair
 * could not be created. max_execution_time stays armed until the first
 * tick(), see there. */

intptr_t frankenphp_set_background_worker_and_get_stop_sock(const char *name,
                                                            size_t name_len) {
  frankenphp_reset_background_worker();
  is_background_worker = true;
  worker_name = zend_string_init(name, name_len, 1);

  if (frankenphp_sock_pair_open(worker_stop_socks) != 0) {
    return -1;
  }

  /* One wake-up right away, so a script that hands its handle to an event
   * loop ticks on its own: readiness then means the loop serviced the handle
   * once. Nothing to do on failure, the script has to tick by itself. */
  frankenphp_sock_send(worker_stop_socks[1], "\n", 1);

  intptr_t s = (intptr_t)worker_stop_socks[1];
  worker_stop_socks[1] = SOCK_ERR;

  return s;
}

/* Closes the Go side's end of a pair, which lands as EOF on the script's
 * end so its stream_select() or blocking read returns promptly. */
void frankenphp_close_sock(intptr_t s) {
  if (s < 0) {
    return;
  }
  frankenphp_sock_close((php_socket_t)s);
}

/* Closes the Go side's end of a stop socket pair, which lands as EOF on the
 * script's end so its wait returns promptly. Closing alone only does while
 * no other process holds a copy of it, and a pcntl_fork() child inherits
 * every descriptor of the process, the pairs of the other threads included:
 * shutting the write direction down sends the FIN regardless. */
void frankenphp_worker_close_stop_sock(intptr_t s) {
  if (s < 0) {
    return;
  }
#ifdef PHP_WIN32
  shutdown((php_socket_t)s, SD_SEND);
#else
  shutdown((php_socket_t)s, SHUT_WR);
#endif
  frankenphp_sock_close((php_socket_t)s);
}

/* Wakes a background worker thread with a line on its handle, one per task sent
 * to the worker. The line says something may be pending, the script finds out
 * what by polling, so its content is no part of the contract. */
void frankenphp_worker_signal_task(intptr_t s) {
  frankenphp_sock_send((php_socket_t)s, "\n", 1);
}

/* Mailboxes: what a PHP thread sleeps on when it waits for the other threads
 * without a descriptor, one per thread index. A mailbox is a counter the wakers
 * bump: the owner reads it, checks what it waits for, and sleeps only while
 * the counter did not move since, so a wake-up between the check and the
 * sleep is never lost, and one that finds the owner awake costs no syscall.
 * Wake-ups say "look again", never what happened: the owner may get one that
 * was meant for an earlier wait and goes back to sleep then. On 64-bit Linux
 * the counter is a futex; elsewhere a condition variable, or an event on
 * Windows, stands behind it: the futex syscall of the 32-bit ports takes a
 * timeout whose layout depends on the libc. */
#if defined(__linux__) && defined(__LP64__) &&                                 \
    !defined(FRANKENPHP_MAILBOX_NO_FUTEX)
#include <sys/syscall.h>
#define FRANKENPHP_MAILBOX_FUTEX 1
/* FUTEX_WAIT_PRIVATE and FUTEX_WAKE_PRIVATE of linux/futex.h, which comes
 * with the kernel headers rather than with the libc */
#define FRANKENPHP_FUTEX_WAIT (0 | 128)
#define FRANKENPHP_FUTEX_WAKE (1 | 128)
#endif

typedef struct {
  _Atomic uint32_t seq;
  /* set while the owner sleeps or is about to: wakers skip the syscall
   * otherwise */
  _Atomic uint32_t sleeping;
  /* when the last waker found the owner asleep, see frankenphp_wake_stamp():
   * the owner measures its wake-ups with it. 32 bits, as wide as the atomics
   * of every port go without a library */
  _Atomic uint32_t woken_at;
#if defined(PHP_WIN32)
  HANDLE event;
#elif !defined(FRANKENPHP_MAILBOX_FUTEX)
  pthread_mutex_t mu;
  pthread_cond_t cond;
#endif
  /* keeps the counters of two threads on different cache lines */
  char pad[64];
} frankenphp_mailbox;

static frankenphp_mailbox *mailboxes = NULL;
static int mailboxes_len = 0;

/* What a thread waits for on its mailbox: an answer to what it sent, or work
 * to do. */
enum {
  FRANKENPHP_WAIT_ANSWER,
  FRANKENPHP_WAIT_WORK,
  FRANKENPHP_WAIT_KINDS,
};

/* Watching before sleeping: a thread put to sleep costs both sides a syscall
 * and the sleeper the time of its wake-up, tens of microseconds on most
 * machines, so a thread first watches its mailbox for a moment. What comes
 * meanwhile then costs neither, and a busy worker passes tasks around without
 * a syscall. The watch keeps a core busy, so it is bounded twice, by what each
 * thread measures of itself:
 * - it lasts twice the time the thread's own wake-ups take, which it knows
 *   from the wakers' timestamps: a watch that ends on a sleep then costs about
 *   what the sleep does, never much more, and it outlasts the wake-up of a
 *   peer the thread just woke, so one sleeper does not put the other to sleep;
 * - it has to pay off: each thread keeps a score per kind of wait, up for a
 *   watch that ended on a wake-up and down twice as fast for one that ended
 *   on a sleep. At zero the thread sleeps at once, and watches again every
 *   FRANKENPHP_WATCH_PROBE waits to notice when things speed up. */
#define FRANKENPHP_WATCH_SCORE_MAX 4
#define FRANKENPHP_WATCH_PROBE 16
/* the wake-up time assumed before the first one was measured, and the bounds
 * of the watch whatever was measured, in ns */
#define FRANKENPHP_WAKE_DEFAULT_NS 25000
#define FRANKENPHP_WATCH_MIN_NS 10000
#define FRANKENPHP_WATCH_MAX_NS 200000

#ifndef PHP_WIN32
typedef struct {
  uint8_t score;
  uint8_t skipped;
} frankenphp_watch;

/* a new thread watches: its first waits tell */
static THREAD_LOCAL frankenphp_watch watches[FRANKENPHP_WAIT_KINDS] = {{2, 0},
                                                                       {2, 0}};
/* how long the wake-ups of the thread take, a moving average in ns */
static THREAD_LOCAL int64_t wake_ns = FRANKENPHP_WAKE_DEFAULT_NS;
/* watching only helps when another core can run what the thread waits for */
static bool mailboxes_watch = false;
#endif

void frankenphp_init_mailboxes(int max_threads, bool watch) {
#ifndef PHP_WIN32
  mailboxes_watch = watch;
#else
  /* no watch there yet: a wait sleeps at once */
  (void)watch;
#endif
  mailboxes = calloc(max_threads, sizeof(frankenphp_mailbox));
  mailboxes_len = mailboxes == NULL ? 0 : max_threads;
  for (int i = 0; i < mailboxes_len; i++) {
#if defined(PHP_WIN32)
    mailboxes[i].event = CreateEvent(NULL, FALSE, FALSE, NULL);
#elif !defined(FRANKENPHP_MAILBOX_FUTEX)
    pthread_mutex_init(&mailboxes[i].mu, NULL);
#ifdef __APPLE__
    /* no clock to pick there, the wait takes a duration instead */
    pthread_cond_init(&mailboxes[i].cond, NULL);
#else
    /* timed waits on a clock that a change of the wall time does not move */
    pthread_condattr_t attr;
    pthread_condattr_init(&attr);
    pthread_condattr_setclock(&attr, CLOCK_MONOTONIC);
    pthread_cond_init(&mailboxes[i].cond, &attr);
    pthread_condattr_destroy(&attr);
#endif
#endif
  }
}

void frankenphp_destroy_mailboxes(void) {
  for (int i = 0; i < mailboxes_len; i++) {
#if defined(PHP_WIN32)
    if (mailboxes[i].event != NULL) {
      CloseHandle(mailboxes[i].event);
    }
#elif !defined(FRANKENPHP_MAILBOX_FUTEX)
    pthread_cond_destroy(&mailboxes[i].cond);
    pthread_mutex_destroy(&mailboxes[i].mu);
#endif
  }
  free(mailboxes);
  mailboxes = NULL;
  mailboxes_len = 0;
}

#ifndef PHP_WIN32
static int64_t frankenphp_now_ns(void) {
  struct timespec now;
  clock_gettime(CLOCK_MONOTONIC, &now);

  return (int64_t)now.tv_sec * 1000000000 + now.tv_nsec;
}

/* The time of a wake-up in microseconds, wrapping around and never 0, which
 * says that no waker left one. */
static uint32_t frankenphp_wake_stamp(void) {
  return (uint32_t)(frankenphp_now_ns() / 1000) | 1;
}
#endif

/* Wakes the thread of that index if it sleeps on its mailbox, and makes its
 * next sleep return at once if it is about to. */
void frankenphp_mailbox_wake(uintptr_t index) {
  if (index >= (uintptr_t)mailboxes_len) {
    return;
  }
  frankenphp_mailbox *mb = &mailboxes[index];

#if defined(FRANKENPHP_MAILBOX_FUTEX)
  atomic_fetch_add(&mb->seq, 1);
  if (atomic_load(&mb->sleeping)) {
    atomic_store(&mb->woken_at, frankenphp_wake_stamp());
    syscall(SYS_futex, &mb->seq, FRANKENPHP_FUTEX_WAKE, 1, NULL, NULL, 0);
  }
#elif defined(PHP_WIN32)
  atomic_fetch_add(&mb->seq, 1);
  if (atomic_load(&mb->sleeping)) {
    SetEvent(mb->event);
  }
#else
  /* under the mutex: the owner cannot check the counter and miss the signal */
  pthread_mutex_lock(&mb->mu);
  atomic_fetch_add(&mb->seq, 1);
  bool sleeping = atomic_load(&mb->sleeping);
  if (sleeping) {
    atomic_store(&mb->woken_at, frankenphp_wake_stamp());
  }
  pthread_mutex_unlock(&mb->mu);
  if (sleeping) {
    pthread_cond_signal(&mb->cond);
  }
#endif
}

/* The counter of the calling thread's mailbox, to read before checking what
 * the thread waits for and to hand to frankenphp_mailbox_wait(). */
static uint32_t frankenphp_mailbox_seq(void) {
  return atomic_load(&mailboxes[thread_index].seq);
}

#ifndef PHP_WIN32
static inline void frankenphp_cpu_relax(void) {
#if defined(__x86_64__) || defined(__i386__)
  __builtin_ia32_pause();
#elif defined(__aarch64__)
  __asm__ __volatile__("yield");
#endif
}

/* Watches the mailbox instead of sleeping on it, see above:
 * true once its counter moved on from seq, false when the thread has to
 * sleep, because the watch ended without that or because watching stopped
 * paying off for this kind of wait. */
static bool frankenphp_mailbox_watch(frankenphp_mailbox *mb, uint32_t seq,
                                     int kind) {
  frankenphp_watch *watch = &watches[kind];
  if (watch->score == 0 && ++watch->skipped < FRANKENPHP_WATCH_PROBE) {
    return false;
  }
  watch->skipped = 0;

  int64_t budget = 2 * wake_ns;
  if (budget < FRANKENPHP_WATCH_MIN_NS) {
    budget = FRANKENPHP_WATCH_MIN_NS;
  } else if (budget > FRANKENPHP_WATCH_MAX_NS) {
    budget = FRANKENPHP_WATCH_MAX_NS;
  }
  int64_t until = frankenphp_now_ns() + budget;
  do {
    for (int i = 0; i < 32; i++) {
      if (atomic_load(&mb->seq) != seq) {
        if (watch->score < FRANKENPHP_WATCH_SCORE_MAX) {
          watch->score++;
        }

        return true;
      }
      frankenphp_cpu_relax();
    }
  } while (frankenphp_now_ns() < until);
  watch->score = watch->score > 2 ? watch->score - 2 : 0;

  return false;
}
#endif

/* Waits until the counter of the calling thread's mailbox moves on from seq,
 * watching it first when that pays off for this kind of wait, sleeping on it
 * then: 1 once it did, 0 after timeout_ms without it, forever when negative,
 * -1 when max_execution_time or a force-kill interrupted the wait, for the VM
 * to bail out once the method returns. */
static int frankenphp_mailbox_wait(uint32_t seq, int timeout_ms, int kind) {
  frankenphp_mailbox *mb = &mailboxes[thread_index];

#ifndef PHP_WIN32
  if (mailboxes_watch && timeout_ms != 0 &&
      frankenphp_mailbox_watch(mb, seq, kind)) {
    return 1;
  }
#endif

  int64_t deadline = timeout_ms < 0 ? -1 : frankenphp_now_ms() + timeout_ms;
  int result;
#ifndef PHP_WIN32
  bool woken = false;
#endif

  atomic_store(&mb->sleeping, 1);
#if !defined(FRANKENPHP_MAILBOX_FUTEX) && !defined(PHP_WIN32)
  pthread_mutex_lock(&mb->mu);
#endif
  for (;;) {
    if (atomic_load(&mb->seq) != seq) {
      result = 1;
      break;
    }
    if (zend_atomic_bool_load(&EG(timed_out))) {
      result = -1;
      break;
    }
    int64_t remaining = -1;
    if (deadline >= 0) {
      remaining = deadline - frankenphp_now_ms();
      if (remaining <= 0) {
        result = 0;
        break;
      }
    }

    int64_t slice = FRANKENPHP_WAIT_SLICE_MS;
    if (remaining >= 0 && remaining < slice) {
      slice = remaining;
    }
#if defined(FRANKENPHP_MAILBOX_FUTEX)
    /* always with a timeout: the kernel then never restarts the wait after
     * a signal, whatever the flags of its handler, so the signals of
     * max_execution_time and of the force-kill both land here as EINTR */
    struct timespec rel = {slice / 1000, (slice % 1000) * 1000000};
    woken = syscall(SYS_futex, &mb->seq, FRANKENPHP_FUTEX_WAIT, seq, &rel, NULL,
                    0) == 0;
#elif defined(PHP_WIN32)
    /* alertable, for the APC of the force-kill */
    WaitForSingleObjectEx(mb->event, (DWORD)slice, TRUE);
#elif defined(__APPLE__)
    struct timespec rel = {slice / 1000, (slice % 1000) * 1000000};
    woken = pthread_cond_timedwait_relative_np(&mb->cond, &mb->mu, &rel) == 0;
#else
    struct timespec until;
    clock_gettime(CLOCK_MONOTONIC, &until);
    until.tv_sec += slice / 1000;
    until.tv_nsec += (slice % 1000) * 1000000;
    if (until.tv_nsec >= 1000000000) {
      until.tv_sec++;
      until.tv_nsec -= 1000000000;
    }
    woken = pthread_cond_timedwait(&mb->cond, &mb->mu, &until) == 0;
#endif
#ifndef PHP_WIN32
    /* out of an actual sleep: how long this wake-up took tells how long the
     * next watch is worth */
    uint32_t at = woken ? atomic_exchange(&mb->woken_at, 0) : 0;
    if (at != 0) {
      /* in microseconds, a millisecond at most: beyond it something else than
       * a wake-up held the thread */
      uint32_t took = frankenphp_wake_stamp() - at;
      if (took > 1000) {
        took = 1000;
      }
      wake_ns = (7 * wake_ns + (int64_t)took * 1000) / 8;
    }
#endif
    /* woken, timed out, interrupted or none of it: the loop tells */
  }
#if !defined(FRANKENPHP_MAILBOX_FUTEX) && !defined(PHP_WIN32)
  pthread_mutex_unlock(&mb->mu);
#endif
  atomic_store(&mb->sleeping, 0);

  return result;
}

/* Task channels: one descriptor per side of a task, the sender's [0] and the
 * receiver's [1], each waited on by its stream and signaled by the other side
 * through the Go side. On Linux they are eventfds, elsewhere the ends of a
 * socket pair, which is what Windows's php_select() waits on. A pair is made
 * for the task that needs one and closed with it: a poller may hold a
 * descriptor past the end of its task, and would get the signals of the next
 * one from a pair handed over. The Go side signals a descriptor once, and
 * takes the signal back when nothing is left to read there, see
 * senderReadableLocked(). */
#if defined(__linux__) && !defined(FRANKENPHP_TASK_CHAN_NO_EVENTFD)
#include <sys/eventfd.h>
#define FRANKENPHP_TASK_CHAN_EVENTFD 1
#endif

int frankenphp_task_chan_open(intptr_t fds[2]) {
#ifdef FRANKENPHP_TASK_CHAN_EVENTFD
  int a = eventfd(0, EFD_CLOEXEC | EFD_NONBLOCK);
  if (a < 0) {
    return -1;
  }
  int b = eventfd(0, EFD_CLOEXEC | EFD_NONBLOCK);
  if (b < 0) {
    close(a);

    return -1;
  }
  fds[0] = a;
  fds[1] = b;
#else
  php_socket_t pair[2];
  if (frankenphp_sock_pair_open(pair) != 0) {
    return -1;
  }
  php_set_sock_blocking(pair[0], 0);
  fds[0] = (intptr_t)pair[0];
  fds[1] = (intptr_t)pair[1];
#endif

  return 0;
}

/* Wakes the side waiting on fds[side]. */
void frankenphp_task_chan_signal(intptr_t fd0, intptr_t fd1, int side) {
#ifdef FRANKENPHP_TASK_CHAN_EVENTFD
  uint64_t one = 1;
  (void)!write((int)(side ? fd1 : fd0), &one, sizeof(one));
#else
  /* a byte on one end lands on the other */
  frankenphp_sock_send((php_socket_t)(side ? fd0 : fd1), "1", 1);
#endif
}

/* Takes back the signal written for the side waiting on fd. */
void frankenphp_task_chan_consume(intptr_t fd) {
#ifdef FRANKENPHP_TASK_CHAN_EVENTFD
  uint64_t v;
  (void)!read((int)fd, &v, sizeof(v));
#else
  /* the byte was sent, and may still be on its way where the pair is a
   * loopback connection: a moment at most */
  for (;;) {
    char b;
    int n = recv((php_socket_t)fd, &b, 1, 0);
    if (n >= 0) {
      return;
    }
    int err = php_socket_errno();
    if (err != EINTR && !PHP_IS_TRANSIENT_ERROR(err)) {
      return;
    }
    php_pollfd_for_ms((php_socket_t)fd, PHP_POLLREADABLE,
                      FRANKENPHP_WAIT_SLICE_MS);
  }
#endif
}

void frankenphp_update_local_thread_context(bool is_worker) {
  /* A thread that ran a background worker can be recycled into an HTTP or a
   * request thread: reset the TLS so the handle rejects callers again. */
  if (is_background_worker) {
    frankenphp_reset_background_worker();
  }

  is_worker_thread = is_worker;

  /* workers should keep running if the user aborts the connection */
  PG(ignore_user_abort) = is_worker ? 1 : original_user_abort_setting;
}

static void frankenphp_update_request_context() {
  /* the server context is stored on the go side, still SG(server_context) needs
   * to not be NULL */
  frankenphp_local_server_ctx.thread_index = thread_index;
  SG(server_context) = &frankenphp_local_server_ctx;
  /* status It is not reset by zend engine, set it to 200. */
  SG(sapi_headers).http_response_code = 200;

  char *authorization_header =
      go_update_request_info(thread_index, &SG(request_info));

  /* let PHP handle basic auth */
  php_handle_auth_data(authorization_header);
}

static void frankenphp_free_request_context() {
  if (SG(request_info).cookie_data != NULL) {
    free(SG(request_info).cookie_data);
    SG(request_info).cookie_data = NULL;
  }

  /* freed via thread.Unpin() */
  SG(request_info).request_method = NULL;
  SG(request_info).query_string = NULL;
  SG(request_info).content_type = NULL;
  SG(request_info).path_translated = NULL;
  SG(request_info).request_uri = NULL;
}

static bool frankenphp_release_super_globals_once() {
  bool released = zend_hash_str_del(&EG(symbol_table), "_SESSION",
                                    sizeof("_SESSION") - 1) == SUCCESS;

  zend_auto_global *auto_global;
  zend_string *_env = ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_ENV);
  ZEND_HASH_MAP_FOREACH_PTR(CG(auto_globals), auto_global) {
    zval *value;
    if (auto_global->name == _env || !auto_global->auto_global_callback ||
        !(value = zend_hash_find(&EG(symbol_table), auto_global->name))) {
      continue;
    }

    zval *array = value;
    ZVAL_DEREF(array);
    if (Z_TYPE_P(array) == IS_ARRAY &&
        zend_hash_num_elements(Z_ARRVAL_P(array)) == 0) {
      continue;
    }

    /* frankenphp_reset_super_globals() only reimports an existing $_REQUEST */
    zval garbage;
    ZVAL_COPY_VALUE(&garbage, value);
    ZVAL_EMPTY_ARRAY(value);
    zval_ptr_dtor(&garbage);
    released = true;
  }
  ZEND_HASH_FOREACH_END();

  return released;
}

/* Destructors must run before the next client's output is active */
static void frankenphp_release_super_globals() {
  /* a destructor can put data back into a superglobal already released */
  while (frankenphp_release_super_globals_once()) {
  }
}

/* reset all 'auto globals' in worker mode except of $_ENV
 * see: php_hash_environment() */
static void frankenphp_reset_super_globals() {
  zend_try {
    /* only $_FILES needs to be flushed explicitly
     * $_GET, $_POST, $_COOKIE and $_SERVER are flushed on reimport
     * $_ENV is not flushed
     * for more info see: php_startup_auto_globals()
     */
    zval *files = &PG(http_globals)[TRACK_VARS_FILES];
    zval_ptr_dtor_nogc(files);
    memset(files, 0, sizeof(*files));
  }
  zend_end_try();

  zend_auto_global *auto_global;
  zend_string *_env = ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_ENV);
  zend_string *_server = ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_SERVER);
  ZEND_HASH_MAP_FOREACH_PTR(CG(auto_globals), auto_global) {
    if (auto_global->name == _env) {
      /* skip $_ENV */
    } else if (auto_global->name == _server) {
      /* always reimport $_SERVER */
      auto_global->armed = auto_global->auto_global_callback(auto_global->name);
    } else if (auto_global->jit) {
      /* JIT globals ($_REQUEST, $GLOBALS) need special handling:
       * - $GLOBALS will always be handled by the application, we skip it
       * For $_REQUEST:
       * - If in symbol_table: re-initialize with current request data
       * - If not: do nothing, it may be armed by jit later */
      if (auto_global->name == ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_REQUEST) &&
          zend_hash_exists(&EG(symbol_table), auto_global->name)) {
        auto_global->armed =
            auto_global->auto_global_callback(auto_global->name);
      }
    } else if (auto_global->auto_global_callback) {
      /* $_GET, $_POST, $_COOKIE, $_FILES are reimported here */
      auto_global->armed = auto_global->auto_global_callback(auto_global->name);
    } else {
      /* $_SESSION will land here (not an http_global) */
      auto_global->armed = 0;
    }
  }
  ZEND_HASH_FOREACH_END();
}

/* Mirrors php_stream_input_t from ext/standard/php_fopen_wrapper.c */
typedef struct {
  php_stream *body;
  zend_off_t position;
} frankenphp_stream_input;

/* Matches the body by address: userland may already have closed and freed it.
 * A php://input stream kept by the application reads it through a raw
 * pointer, the body then lives as long as the worker script. */
static void frankenphp_release_request_body(php_stream *request_body) {
  zend_resource *body_res = NULL;
  zend_resource *res;
  ZEND_HASH_FOREACH_PTR(&EG(regular_list), res) {
    if (res->type != php_file_le_stream()) {
      continue;
    }
    php_stream *stream = (php_stream *)res->ptr;
    if (stream == request_body) {
      body_res = res;
    } else if (stream->wrapper == &php_stream_php_wrapper &&
               strcmp(stream->ops->label, "Input") == 0 &&
               ((frankenphp_stream_input *)stream->abstract)->body ==
                   request_body) {
      return;
    }
  }
  ZEND_HASH_FOREACH_END();

  if (body_res != NULL) {
    zend_list_delete(body_res);
  }
}

#ifdef HAVE_PHP_SESSION
/* Reset session state between worker requests, preserving user handlers.
 * Based on php_rshutdown_session_globals() + php_rinit_session_globals(). */
static void frankenphp_reset_session_state(void) {
  if (PS(session_status) == php_session_active) {
    php_session_flush(1);
  }

  if (!Z_ISUNDEF(PS(http_session_vars))) {
    zval_ptr_dtor(&PS(http_session_vars));
    ZVAL_UNDEF(&PS(http_session_vars));
  }

  if (PS(mod_data) || PS(mod_user_implemented)) {
    zend_try { PS(mod)->s_close(&PS(mod_data)); }
    zend_end_try();
  }

  if (PS(id)) {
    zend_string_release_ex(PS(id), 0);
    PS(id) = NULL;
  }

  if (PS(session_vars)) {
    zend_string_release_ex(PS(session_vars), 0);
    PS(session_vars) = NULL;
  }

  /* PS(mod_user_class_name) and PS(mod_user_names) are preserved */

#if PHP_VERSION_ID >= 80300
  if (PS(session_started_filename)) {
    zend_string_release(PS(session_started_filename));
    PS(session_started_filename) = NULL;
    PS(session_started_lineno) = 0;
  }
#endif

  PS(session_status) = php_session_none;
  PS(in_save_handler) = 0;
  PS(set_handler) = 0;
  PS(mod_data) = NULL;
  PS(mod_user_is_open) = 0;
  PS(define_sid) = 1;
}
#endif

/* Adapted from the url_scanner RSHUTDOWN, which isn't exported */
static void frankenphp_reset_url_scanner(url_adapt_state_ex_t *ctx) {
  smart_str_free(&ctx->result);
  smart_str_free(&ctx->buf);
  smart_str_free(&ctx->tag);
  smart_str_free(&ctx->arg);
  smart_str_free(&ctx->attr_val);
  smart_str_free(&ctx->form_app);
  smart_str_free(&ctx->url_app);
  ctx->active = 0;
  ctx->tag_type = 0;
  ctx->attr_type = 0;
}

/* Response state of the standard extension, freed by its RSHUTDOWN */
static void frankenphp_reset_basic_state(void) {
  /* output_add_rewrite_var() and session.use_trans_sid */
  frankenphp_reset_url_scanner(&BG(url_adapt_session_ex));
  frankenphp_reset_url_scanner(&BG(url_adapt_output_ex));
}

static frankenphp_thread_metrics *thread_metrics = NULL;

/* Adapted from php_request_shutdown */
static void frankenphp_worker_request_shutdown() {
  __atomic_store_n(&thread_metrics[thread_index].last_memory_usage,
                   zend_memory_usage(0), __ATOMIC_RELAXED);

  /* Flush all output buffers */
  zend_try { php_output_end_all(); }
  zend_end_try();

  const char **module_name;
  zend_module_entry *module;
  for (module_name = MODULES_TO_RELOAD; *module_name; module_name++) {
    if ((module = zend_hash_str_find_ptr(&module_registry, *module_name,
                                         strlen(*module_name)))) {
      module->request_shutdown_func(module->type, module->module_number);
    }
  }

#ifdef HAVE_PHP_SESSION
  frankenphp_reset_session_state();
#endif

  /* Shutdown output layer (send the set HTTP headers, cleanup output handlers,
   * etc.) */
  zend_try { php_output_deactivate(); }
  zend_end_try();
  frankenphp_reset_basic_state();

  /* sapi_deactivate() drops the request body stream without closing it, it
   * would otherwise stay open until the worker script ends */
  php_stream *request_body = SG(request_info).request_body;

  /* SAPI related shutdown (free stuff) */
  zend_try { sapi_deactivate(); }
  zend_end_try();
#if PHP_VERSION_ID < 80600
  /* sapi_deactivate() frees these without clearing them (fixed in PHP 8.6),
   * php_request_shutdown() would free them again when the worker script ends */
  SG(request_info).content_type_dup = NULL;
  SG(request_info).current_user = NULL;
  SG(request_info).current_user_length = 0;
#endif
  if (request_body != NULL) {
    frankenphp_release_request_body(request_body);
  }
  frankenphp_free_request_context();

  if (prepared_env != NULL) {
    zend_hash_release(prepared_env);
    prepared_env = NULL;
  }

  zend_set_memory_limit(PG(memory_limit));
}

// shutdown the dummy request that starts the worker script
bool frankenphp_shutdown_dummy_request(void) {
  if (SG(server_context) == NULL) {
    return false;
  }

  worker_env = prepared_env;
  prepared_env = NULL;
  frankenphp_worker_request_shutdown();

  return true;
}

/* Copies worker_env then prepared_env into ht, the latter wins */
static void frankenphp_copy_prepared_env(HashTable *ht) {
  if (worker_env != NULL) {
    zend_hash_copy(ht, worker_env, (copy_ctor_func_t)zval_add_ref);
  }
  if (prepared_env != NULL) {
    zend_hash_copy(ht, prepared_env, (copy_ctor_func_t)zval_add_ref);
  }
}

void get_full_env(zval *track_vars_array) {
  HashTable *ht = Z_ARR_P(track_vars_array);
  zend_hash_extend(ht, zend_hash_num_elements(main_thread_env), 0);
  zend_hash_copy(ht, main_thread_env, NULL);

  /* $_ENV outlives worker requests, it must not capture the env of the
   * request that first used it */
  HashTable *env = worker_env != NULL ? worker_env : prepared_env;
  if (env != NULL) {
    zend_hash_copy(ht, env, (copy_ctor_func_t)zval_add_ref);
  }
}

/* Adapted from php_request_startup() */
static int frankenphp_worker_request_startup() {
  int retval = SUCCESS;

  /* Use the default error handler while parsing the worker request */
  zval orig_user_error_handler;
  ZVAL_COPY_VALUE(&orig_user_error_handler, &EG(user_error_handler));
  ZVAL_UNDEF(&EG(user_error_handler));

  frankenphp_update_request_context();

  zend_try {
    php_output_activate();

    /* initialize global variables */
    PG(header_is_being_sent) = 0;
    PG(connection_status) = PHP_CONNECTION_NORMAL;

    /* Keep the current execution context */
    sapi_activate();

#ifdef ZEND_MAX_EXECUTION_TIMERS
    if (PG(max_input_time) == -1) {
      zend_set_timeout(EG(timeout_seconds), 1);
    } else {
      zend_set_timeout(PG(max_input_time), 1);
    }
#endif

    if (PG(expose_php)) {
      sapi_add_header(SAPI_PHP_VERSION_HEADER,
                      sizeof(SAPI_PHP_VERSION_HEADER) - 1, 1);
    }

    /* PG(output_handler) is a char* before PHP 8.6, a zend_string* since */
#if PHP_VERSION_ID < 80600
    if (PG(output_handler) && PG(output_handler)[0]) {
      zval oh;

      ZVAL_STRING(&oh, PG(output_handler));
#else
    if (PG(output_handler) && ZSTR_LEN(PG(output_handler))) {
      zval oh;

      ZVAL_STR(&oh, zend_string_dup(PG(output_handler), false));
#endif
      php_output_start_user(&oh, 0, PHP_OUTPUT_HANDLER_STDFLAGS);
      zval_ptr_dtor(&oh);
    } else if (PG(output_buffering)) {
      php_output_start_user(NULL,
                            PG(output_buffering) > 1 ? PG(output_buffering) : 0,
                            PHP_OUTPUT_HANDLER_STDFLAGS);
    } else if (PG(implicit_flush)) {
      php_output_set_implicit_flush(1);
    }

    frankenphp_reset_super_globals();

    const char **module_name;
    zend_module_entry *module;
    for (module_name = MODULES_TO_RELOAD; *module_name; module_name++) {
      if ((module = zend_hash_str_find_ptr(&module_registry, *module_name,
                                           strlen(*module_name))) &&
          module->request_startup_func) {
        module->request_startup_func(module->type, module->module_number);
      }
    }
  }
  zend_catch { retval = FAILURE; }
  zend_end_try();

  /* Restore the original user error handler */
  ZVAL_COPY_VALUE(&EG(user_error_handler), &orig_user_error_handler);

  SG(sapi_started) = 1;

  return retval;
}

PHP_FUNCTION(frankenphp_finish_request) { /* {{{ */
  ZEND_PARSE_PARAMETERS_NONE();

  uintptr_t idx = frankenphp_thread_index();
  if (go_is_context_done(idx)) {
    RETURN_FALSE;
  }

  php_output_end_all();
  php_header();

  go_frankenphp_finish_php_request(idx);

  RETURN_TRUE;
} /* }}} */

/* {{{ Sandboxed putenv() that never mutates the process-global OS environment.
 * Writes land only in the thread-local sandboxed_env, so a secret passed to
 * putenv() by one request cannot leak to the Go/Caddy side, to child processes
 * (proc_open, exec, ...), or to other requests. */
PHP_FUNCTION(frankenphp_putenv) {
  char *setting;
  size_t setting_len;

  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_STRING(setting, setting_len)
  ZEND_PARSE_PARAMETERS_END();

  if (setting_len == 0 || setting[0] == '=') {
    zend_argument_value_error(1, "must have a valid syntax");
    RETURN_THROWS();
  }

  if (sandboxed_env == NULL) {
    sandboxed_env = zend_new_array(0);
  }

  /* cut at null byte to stay consistent with regular putenv */
  char *null_pos = memchr(setting, '\0', setting_len);
  if (null_pos != NULL) {
    setting_len = null_pos - setting;
  }

  /* cut the string at the first '=' */
  char *eq_pos = memchr(setting, '=', setting_len);

  /* no '=' found, delete the variable */
  if (eq_pos == NULL) {
    zval unset;
    ZVAL_NULL(&unset);
    zend_hash_str_update(sandboxed_env, setting, setting_len, &unset);

    RETURN_TRUE;
  }

  size_t name_len = eq_pos - setting;
  size_t value_len =
      (setting_len > name_len + 1) ? (setting_len - name_len - 1) : 0;
  zval val = {0};
  ZVAL_STRINGL(&val, eq_pos + 1, value_len);
  zend_hash_str_update(sandboxed_env, setting, name_len, &val);

  RETURN_TRUE;
} /* }}} */

/* getenv() lookup, the first layer holding the name wins. Returns an IS_NULL
 * zval for a variable unset by putenv(). */
static inline zval *frankenphp_lookup_env(const char *name, size_t name_len) {
  HashTable *layers[] = {sandboxed_env, prepared_env, worker_env,
                         main_thread_env};
  for (size_t i = 0; i < sizeof(layers) / sizeof(layers[0]); i++) {
    if (layers[i] != NULL) {
      zval *env_val = zend_hash_str_find(layers[i], name, name_len);
      if (env_val != NULL) {
        return env_val;
      }
    }
  }

  return NULL;
}

/* Returns a fresh copy of the full environment, merging the layers above. */
static inline HashTable *frankenphp_dup_env(void) {
  HashTable *env = zend_array_dup(main_thread_env);
  frankenphp_copy_prepared_env(env);

  if (sandboxed_env != NULL) {
    zend_string *key;
    zval *val;
    ZEND_HASH_FOREACH_STR_KEY_VAL(sandboxed_env, key, val) {
      if (Z_TYPE_P(val) == IS_NULL) {
        zend_hash_del(env, key);
      } else {
        Z_TRY_ADDREF_P(val);
        zend_hash_update(env, key, val);
      }
    }
    ZEND_HASH_FOREACH_END();
  }

  return env;
}

/* {{{ Get the env from the sandboxed environment */
PHP_FUNCTION(frankenphp_getenv) {
  zend_string *name = NULL;
  bool local_only = 0;

  ZEND_PARSE_PARAMETERS_START(0, 2)
  Z_PARAM_OPTIONAL
  Z_PARAM_STR_OR_NULL(name)
  Z_PARAM_BOOL(local_only)
  ZEND_PARSE_PARAMETERS_END();

  if (!name) {
    RETURN_ARR(frankenphp_dup_env());
    return;
  }

  zval *env_val = frankenphp_lookup_env(ZSTR_VAL(name), ZSTR_LEN(name));
  if (env_val && Z_TYPE_P(env_val) == IS_STRING) {
    zend_string *str = Z_STR_P(env_val);
    zend_string_addref(str);
    RETVAL_STR(str);
  } else {
    RETVAL_FALSE;
  }
} /* }}} */

/* {{{ Fetch all HTTP request headers */
PHP_FUNCTION(frankenphp_request_headers) {
  ZEND_PARSE_PARAMETERS_NONE();

  struct go_apache_request_headers_return headers =
      go_apache_request_headers(frankenphp_thread_index());

  array_init_size(return_value, headers.r1);

  for (size_t i = 0; i < headers.r1; i++) {
    go_string key = headers.r0[i * 2];
    go_string val = headers.r0[(i * 2) + 1];

    add_assoc_stringl_ex(return_value, key.data, key.len, val.data, val.len);
  }
}
/* }}} */

/* add_response_header and apache_response_headers are copied from
 * https://github.com/php/php-src/blob/master/sapi/cli/php_cli_server.c
 * Copyright (c) The PHP Group
 * Licensed under The PHP License
 * Original authors: Moriyoshi Koizumi <moriyoshi@php.net> and Xinchen Hui
 * <laruence@php.net>
 */
static void add_response_header(sapi_header_struct *h,
                                zval *return_value) /* {{{ */
{
  if (h->header_len > 0) {
    char *s;
    size_t len = 0;
    ALLOCA_FLAG(use_heap)

    char *p = strchr(h->header, ':');
    if (NULL != p) {
      len = p - h->header;
    }
    if (len > 0) {
      while (len != 0 &&
             (h->header[len - 1] == ' ' || h->header[len - 1] == '\t')) {
        len--;
      }
      if (len) {
        s = do_alloca(len + 1, use_heap);
        memcpy(s, h->header, len);
        s[len] = 0;
        do {
          p++;
        } while (*p == ' ' || *p == '\t');
        add_assoc_stringl_ex(return_value, s, len, p,
                             h->header_len - (p - h->header));
        free_alloca(s, use_heap);
      }
    }
  }
}
/* }}} */

PHP_FUNCTION(frankenphp_response_headers) /* {{{ */
{
  ZEND_PARSE_PARAMETERS_NONE();

  array_init(return_value);
  zend_llist_apply_with_argument(
      &SG(sapi_headers).headers,
      (llist_apply_with_arg_func_t)add_response_header, return_value);
}
/* }}} */

PHP_FUNCTION(frankenphp_handle_request) {
  zend_fcall_info fci;
  zend_fcall_info_cache fcc;

  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_FUNC(fci, fcc)
  ZEND_PARSE_PARAMETERS_END();

  if (!is_worker_thread) {
    /* not a worker, throw an error */
    zend_throw_exception(
        spl_ce_RuntimeException,
        "frankenphp_handle_request() called while not in worker mode", 0);
    RETURN_THROWS();
  }

  if (is_background_worker) {
    /* background workers never receive HTTP requests */
    zend_throw_exception(
        spl_ce_RuntimeException,
        "frankenphp_handle_request() cannot be called from a background worker",
        0);
    RETURN_THROWS();
  }

  frankenphp_release_super_globals();
  if (EG(exception)) {
    RETURN_THROWS();
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /* Disable timeouts while waiting for a request to handle */
  zend_unset_timeout();
#endif

  zval handler_parameters = {0};
  bool has_request = go_frankenphp_worker_handle_request_start(
      thread_index, &handler_parameters);
  /* Startup also runs when shutting down, to keep sapi_activate() and
   * sapi_deactivate() balanced for the code after the worker loop */
  if (frankenphp_worker_request_startup() == FAILURE) {
    /* The engine can't run userland code after a caught bailout, end the
     * script so the pending request gets closed */
    zend_bailout();
  }

  if (!has_request) {
    /* Shutting down */
    RETURN_FALSE;
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /*
   * Reset default timeout
   */
  if (PG(max_input_time) != -1) {
#if PHP_VERSION_ID < 80600
    zend_set_timeout(INI_INT("max_execution_time"), 0);
#else
    zend_set_timeout(zend_ini_long_literal("max_execution_time"), 0);
#endif
  }
#endif

  /* Call the PHP func passed to frankenphp_handle_request() */
  zval retval = {0};
  zval *callback_ret = NULL;

  fci.size = sizeof fci;
  fci.retval = &retval;
  fci.params = Z_ISUNDEF(handler_parameters) ? NULL : &handler_parameters;
  fci.param_count = Z_ISUNDEF(handler_parameters) ? 0 : 1;

  /* a body over the limit was rejected while parsing the request */
  if (EXPECTED(!go_is_context_done(thread_index)) &&
      zend_call_function(&fci, &fcc) == SUCCESS && Z_TYPE(retval) != IS_UNDEF) {
    callback_ret = &retval;

    /* pass NULL instead of the NULL zval as return value */
    if (Z_TYPE(retval) == IS_NULL) {
      callback_ret = NULL;
    }
  }

  /*
   * If an exception occurred, print the message to the client before
   * closing the connection.
   */
  if (EG(exception)) {
    if (!zend_is_unwind_exit(EG(exception)) &&
        !zend_is_graceful_exit(EG(exception))) {
      zend_exception_error(EG(exception), E_ERROR);
    } else {
      /* exit() will jump directly to after php_execute_script */
      zend_bailout();
    }
  }

#ifndef PHP_WIN32
  if (UNEXPECTED(is_forked_child)) {
    _exit(EG(exit_status));
  }
#endif

  frankenphp_worker_request_shutdown();
  go_frankenphp_finish_worker_request(thread_index, callback_ret);
  zval_ptr_dtor(&handler_parameters);
  if (callback_ret != NULL) {
    zval_ptr_dtor(&retval);
  }

  RETURN_TRUE;
}

PHP_FUNCTION(headers_send) {
  zend_long response_code = 200;

  ZEND_PARSE_PARAMETERS_START(0, 1)
  Z_PARAM_OPTIONAL
  Z_PARAM_LONG(response_code)
  ZEND_PARSE_PARAMETERS_END();

  int previous_status_code = SG(sapi_headers).http_response_code;
  SG(sapi_headers).http_response_code = response_code;

  if (response_code >= 100 && response_code < 200) {
    int ret = sapi_module.send_headers(&SG(sapi_headers));
    SG(sapi_headers).http_response_code = previous_status_code;

    RETURN_LONG(ret);
  }

  RETURN_LONG(sapi_send_headers());
}

PHP_FUNCTION(mercure_publish) {
  zval *topics;
  zend_string *data = NULL, *id = NULL, *type = NULL;
  zend_bool private = 0;
  zend_long retry = 0;
  bool retry_is_null = 1;

  ZEND_PARSE_PARAMETERS_START(1, 6)
  Z_PARAM_ZVAL(topics)
  Z_PARAM_OPTIONAL
  Z_PARAM_STR_OR_NULL(data)
  Z_PARAM_BOOL(private)
  Z_PARAM_STR_OR_NULL(id)
  Z_PARAM_STR_OR_NULL(type)
  Z_PARAM_LONG_OR_NULL(retry, retry_is_null)
  ZEND_PARSE_PARAMETERS_END();

  if (Z_TYPE_P(topics) != IS_ARRAY && Z_TYPE_P(topics) != IS_STRING) {
    zend_argument_type_error(1, "must be of type array|string");
    RETURN_THROWS();
  }

  if (Z_TYPE_P(topics) == IS_ARRAY) {
    zval *topic;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(topics), topic) {
      ZVAL_DEREF(topic);
      if (Z_TYPE_P(topic) != IS_STRING) {
        zend_argument_type_error(1, "must only contain strings, %s given",
                                 zend_zval_type_name(topic));
        RETURN_THROWS();
      }
    }
    ZEND_HASH_FOREACH_END();
  }

  /* The protocol only allows digits in the "retry" field. */
  if (retry < 0) {
    zend_argument_value_error(6, "must be greater than or equal to 0");
    RETURN_THROWS();
  }

  struct go_mercure_publish_return result = go_mercure_publish(
      frankenphp_thread_index(), topics, data, private, id, type, retry);

  switch (result.r2) {
  case FRANKENPHP_MERCURE_OK:
    RETURN_STR(result.r0);
  case FRANKENPHP_MERCURE_NO_HUB:
    zend_throw_exception(spl_ce_RuntimeException, "No Mercure hub configured",
                         0);
    RETURN_THROWS();
  case FRANKENPHP_MERCURE_INVALID_UPDATE:
    zend_argument_value_error(result.r3, "%s", result.r1);
    free(result.r1);
    RETURN_THROWS();
  case FRANKENPHP_MERCURE_PUBLISH_FAILED:
    zend_throw_exception_ex(spl_ce_RuntimeException, 0, "Publish failed: %s",
                            result.r1);
    free(result.r1);
    RETURN_THROWS();
  case FRANKENPHP_MERCURE_UNSUPPORTED:
    zend_throw_exception(spl_ce_RuntimeException,
                         "FrankenPHP not built with Mercure support", 0);
    RETURN_THROWS();
  }
}

PHP_FUNCTION(frankenphp_log) {
  zend_string *message = NULL;
  zend_long level = 0;
  zval *context = NULL;

  ZEND_PARSE_PARAMETERS_START(1, 3)
  Z_PARAM_STR(message)
  Z_PARAM_OPTIONAL
  Z_PARAM_LONG(level)
  Z_PARAM_ARRAY(context)
  ZEND_PARSE_PARAMETERS_END();

  char *ret = NULL;
  ret = go_log_attrs(frankenphp_thread_index(), message, level, context);
  if (ret != NULL) {
    zend_throw_exception(spl_ce_RuntimeException, ret, 0);
    free(ret);
    RETURN_THROWS();
  }
}

#if FRANKENPHP_OPCACHE_RESTART_HOOK
static void frankenphp_opcache_restart_hook(int reason) {
  go_opcache_restart_scheduled(reason);
}
#endif

/* Handles of FrankenPHP: objects a script waits on. On PHP 8.6 they implement
 * Io\Poll\Handle, so an Io\Poll\Context waits on them next to the script's
 * own handles; the interface is a marker, the contract is the three hooks
 * below, and the descriptor never reaches userland. Before 8.6 the same hooks
 * carry the object's lifetime, in the layout the poll API expects. */
#if PHP_VERSION_ID >= 80600
#include <php_poll.h>
typedef php_poll_handle_object frankenphp_handle_obj;
typedef php_poll_handle_ops frankenphp_handle_ops;
#define FRANKENPHP_HANDLE_OF(zobj) PHP_POLL_HANDLE_OBJ_FROM_ZOBJ(zobj)
#else
typedef struct frankenphp_handle_obj frankenphp_handle_obj;
typedef struct {
  php_socket_t (*get_fd)(frankenphp_handle_obj *handle);
  int (*is_valid)(frankenphp_handle_obj *handle);
  void (*cleanup)(frankenphp_handle_obj *handle);
} frankenphp_handle_ops;
struct frankenphp_handle_obj {
  frankenphp_handle_ops *ops;
  void *handle_data;
  zend_object std;
};
#define FRANKENPHP_HANDLE_OF(zobj)                                             \
  ((frankenphp_handle_obj *)((char *)(zobj) -                                  \
                             offsetof(frankenphp_handle_obj, std)))
#endif

/* The state a handle carries, NULL when it needs none. */
#define FRANKENPHP_HANDLE_DATA(zthis)                                          \
  (FRANKENPHP_HANDLE_OF(Z_OBJ_P(zthis))->handle_data)

static zend_object_handlers frankenphp_handle_obj_handlers;

static zend_object *frankenphp_handle_obj_create(zend_class_entry *ce,
                                                 frankenphp_handle_ops *ops) {
#if PHP_VERSION_ID >= 80600
  frankenphp_handle_obj *obj =
      php_poll_handle_object_create(sizeof(*obj), ce, ops);
#else
  frankenphp_handle_obj *obj = zend_object_alloc(sizeof(*obj), ce);

  zend_object_std_init(&obj->std, ce);
  object_properties_init(&obj->std, ce);
  obj->ops = ops;
  obj->handle_data = NULL;
#endif
  obj->std.handlers = &frankenphp_handle_obj_handlers;

  return &obj->std;
}

static void frankenphp_handle_obj_free(zend_object *object) {
  frankenphp_handle_obj *obj = FRANKENPHP_HANDLE_OF(object);

  if (obj->ops != NULL && obj->ops->cleanup != NULL) {
    obj->ops->cleanup(obj);
  }
  zend_object_std_dtor(&obj->std);
}

/* Declares Io\Poll\Handle on a handle class: the interface accepts internal
 * classes only. PHP 8.6 brings it with the poll API, below it FrankenPHP
 * declares it, and the classmap-autoloaded declaration of
 * symfony/polyfill-io-poll then never loads. */
static void frankenphp_handle_implements_poll(zend_class_entry *ce) {
  zend_class_entry *poll_handle_ce = zend_hash_str_find_ptr(
      CG(class_table), "io\\poll\\handle", sizeof("io\\poll\\handle") - 1);

#if PHP_VERSION_ID < 80600
  if (poll_handle_ce == NULL) {
    zend_class_entry poll_handle;
    INIT_NS_CLASS_ENTRY(poll_handle, "Io\\Poll", "Handle", NULL);
    poll_handle_ce = zend_register_internal_interface(&poll_handle);
  }
#endif

  if (poll_handle_ce != NULL) {
    zend_class_implements(ce, 1, poll_handle_ce);
  }
}

/* The socket of the current background worker, invalid outside one and
 * once the thread let it go: is_valid() then reports what tick() does. */
static php_socket_t
frankenphp_worker_handle_get_fd(frankenphp_handle_obj *handle) {
  (void)handle;

  return is_background_worker ? worker_stop_socks[0] : SOCK_ERR;
}

static int frankenphp_worker_handle_is_valid(frankenphp_handle_obj *handle) {
  return frankenphp_worker_handle_get_fd(handle) != SOCK_ERR;
}

/* The stream a script took from this handle, so asking twice hands out the
 * same one and a loop does not grow the resource list of a run that never
 * ends. On the handle rather than the thread, so it goes away with it. */
static void frankenphp_worker_handle_cleanup(frankenphp_handle_obj *handle) {
  if (handle->handle_data == NULL) {
    return;
  }

  zend_list_delete(handle->handle_data);
  handle->handle_data = NULL;
}

static frankenphp_handle_ops frankenphp_worker_handle_poll_ops = {
    .get_fd = frankenphp_worker_handle_get_fd,
    .is_valid = frankenphp_worker_handle_is_valid,
    .cleanup = frankenphp_worker_handle_cleanup,
};

static zend_object *frankenphp_worker_handle_new(zend_class_entry *ce) {
  return frankenphp_handle_obj_create(ce, &frankenphp_worker_handle_poll_ops);
}

/* Ops of the streams returned by WorkerHandle::getStream(): the socket ops,
 * except that closing a stream leaves the socket alone, and that a read which
 * would block before the first tick() throws. The socket belongs to the
 * thread, every handle of a run shares it, and it is closed at the next run
 * setup or on thread exit. Initialized in MINIT. */
static php_stream_ops frankenphp_worker_handle_ops;

static int frankenphp_worker_handle_close(php_stream *stream,
                                          int close_handle) {
  (void)close_handle;

  /* free the stream data only, never the shared socket */
  return php_stream_socket_ops.close(stream, 0);
}

/* Before the first tick of a run, the wake-up sent at start is all the handle
 * gets until the drain: once a read took it, the next one would park the run
 * short of its ready point, and the start with it, for good. */
static ssize_t frankenphp_worker_handle_read(php_stream *stream, char *buf,
                                             size_t count) {
  php_netstream_data_t *sock = (php_netstream_data_t *)stream->abstract;

  if (!worker_ticked && sock->is_blocked &&
      php_pollfd_for_ms(sock->socket, PHP_POLLREADABLE, 0) <= 0) {
    zend_throw_exception(spl_ce_LogicException,
                         "FrankenPHP\\WorkerHandle: reading the stream would "
                         "block before the first tick()",
                         0);
    return -1;
  }

  return php_stream_socket_ops.read(stream, buf, count);
}

/* The constructor is not a gate: unserialize() and Reflection make an
 * instance without it, so every method checks the thread it runs on. */
static bool frankenphp_worker_handle_usable(void) {
  if (is_background_worker && worker_stop_socks[0] != SOCK_ERR) {
    return true;
  }

  zend_throw_exception(
      spl_ce_LogicException,
      "FrankenPHP\\WorkerHandle can only be used from a background worker", 0);

  return false;
}

/* The handle of the current background worker: the stream to wait on and the
 * tick that reports the drain. Constructing it outside a background worker
 * throws, a run may create as many as it likes. */
ZEND_METHOD(FrankenPHP_WorkerHandle, __construct) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!is_background_worker) {
    zend_throw_exception(
        spl_ce_LogicException,
        "FrankenPHP\\WorkerHandle can only be created from a background worker",
        0);
    RETURN_THROWS();
  }
}

ZEND_METHOD(FrankenPHP_WorkerHandle, getName) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }

  /* the TLS copy is persistent and goes with the run: return a request one */
  RETURN_STRINGL(ZSTR_VAL(worker_name), ZSTR_LEN(worker_name));
}

ZEND_METHOD(FrankenPHP_WorkerHandle, getStream) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }

  /* The same resource until the script closes it, so asking for it in a loop
   * does not grow the resource list of a run that never ends. The stream does
   * not own the socket (see frankenphp_worker_handle_ops), so closing one
   * leaves the streams of the other handles alone. */
  frankenphp_handle_obj *handle = FRANKENPHP_HANDLE_OF(Z_OBJ_P(ZEND_THIS));
  if (handle->handle_data != NULL) {
    zend_resource *cached = handle->handle_data;
    if (cached->type == php_file_le_stream()) {
      GC_ADDREF(cached);
      RETURN_RES(cached);
    }
    /* closed by the script: drop the handle's ref */
    zend_list_delete(cached);
    handle->handle_data = NULL;
  }

  php_stream *stream =
      php_stream_sock_open_from_socket(worker_stop_socks[0], NULL);
  if (stream == NULL) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to create a stream over the stop socket", 0);
    RETURN_THROWS();
  }

  /* a blocking read is a valid way to park: wait without the
   * default_socket_timeout wake-ups */
  struct timeval no_timeout = {-1, 0};
  php_stream_set_option(stream, PHP_STREAM_OPTION_READ_TIMEOUT, 0, &no_timeout);

  stream->ops = &frankenphp_worker_handle_ops;

  php_stream_to_zval(stream, return_value);
  handle->handle_data = Z_RES_P(return_value);
  GC_ADDREF(Z_RES_P(return_value));
}

/* Consumes the wake-ups written on the handle, without blocking: false once
 * FrankenPHP drained the worker, which closes its end of the socket, or when
 * the socket is broken and carries no drain anymore. */
static bool frankenphp_worker_handle_consume(void) {
  char buf[64];
  for (;;) {
    if (php_pollfd_for_ms(worker_stop_socks[0], PHP_POLLREADABLE, 0) <= 0) {
      /* nothing pending, or a transient poll error: still running */
      return true;
    }

#ifdef PHP_WIN32
    int n = recv(worker_stop_socks[0], buf, (int)sizeof(buf), 0);
#else
    ssize_t n = recv(worker_stop_socks[0], buf, sizeof(buf), 0);
#endif
    if (n == 0) {
      return false;
    }
    if (n < 0) {
      int err = php_socket_errno();

      return err == EINTR || PHP_IS_TRANSIENT_ERROR(err);
    }
  }
}

/* The ready point of a background worker and its liveness check, the analog
 * of frankenphp_handle_request(): the first call of a run reports the worker
 * ready, and every call returns false once FrankenPHP drains it. It never
 * hands out work, and never waits unless it is given a timeout: a script
 * with more to wait on waits on its handle, or on the stream of getStream(),
 * and calls this when it is readable. Whatever the runtime wrote there to
 * wake the script up is consumed here, so the protocol stays private. */
ZEND_METHOD(FrankenPHP_WorkerHandle, tick) {
  double timeout = 0;
  bool timeout_is_null = false;
  ZEND_PARSE_PARAMETERS_START(0, 1)
  Z_PARAM_OPTIONAL
  Z_PARAM_DOUBLE_OR_NULL(timeout, timeout_is_null)
  ZEND_PARSE_PARAMETERS_END();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }
  if (!timeout_is_null && (zend_isnan(timeout) || timeout < 0)) {
    zend_argument_value_error(1, "must be greater than or equal to 0");
    RETURN_THROWS();
  }

  if (!worker_ticked) {
    /* refused while a declared vars namespace has no snapshot of this run */
    char *error =
        go_frankenphp_background_worker_ready(frankenphp_thread_index());
    if (error != NULL) {
      zend_throw_exception(spl_ce_LogicException, error, 0);
      free(error);
      RETURN_THROWS();
    }
    worker_ticked = true;
    /* The bootstrap ran under max_execution_time like any request, the loop
     * that starts now has no time limit, like the CLI. Nothing re-arms the
     * timer past this point. */
    zend_unset_timeout();
  }

  /* 0 never waits: the script waits on its handle, and the thread parks
   * there. Otherwise the wait happens here, on the mailbox of the thread:
   * null, or past what a duration holds, waits until FrankenPHP needs the
   * script */
  bool wait = timeout_is_null || timeout > 0;
  int timeout_ms = -1;
  if (!timeout_is_null && timeout * 1000 < (double)INT_MAX) {
    /* a wait shorter than the clock's grain still waits */
    timeout_ms = timeout > 0 && timeout < 0.001 ? 1 : (int)(timeout * 1000);
  }
  int64_t deadline = -1;
  for (;;) {
    /* read before asking, so a task sent right after the answer is not slept
     * through */
    uint32_t seq = frankenphp_mailbox_seq();

    /* what was written on the handle was for a script that waits there, and
     * does not cut a wait in here short: its end is the drain */
    if (!frankenphp_worker_handle_consume()) {
      RETURN_FALSE;
    }

    /* the thread parks where it is about to wait, unless tasks are queued:
     * the script then finds its handle readable, and a wait in here is over,
     * see go_frankenphp_background_worker_park. The Go side also knows of a
     * drain before the socket tells */
    int parked = go_frankenphp_background_worker_park(
        frankenphp_thread_index(),
        wait ? FRANKENPHP_PARK_ON_MAILBOX : FRANKENPHP_PARK_ON_HANDLE);
    if (parked == FRANKENPHP_PARK_DRAINED) {
      RETURN_FALSE;
    }
    if (!wait || parked == FRANKENPHP_PARK_READY) {
      RETURN_TRUE;
    }

    int remaining = -1;
    if (timeout_ms >= 0) {
      int64_t now = frankenphp_now_ms();
      if (deadline < 0) {
        deadline = now + timeout_ms;
      }
      remaining = deadline > now ? (int)(deadline - now) : 0;
    }
    if (remaining == 0 ||
        frankenphp_mailbox_wait(seq, remaining, FRANKENPHP_WAIT_WORK) <= 0) {
      /* out of time, or interrupted, and the VM bails out once the method
       * returns: the thread leaves its park */
      RETURN_BOOL(go_frankenphp_background_worker_park(
                      frankenphp_thread_index(), FRANKENPHP_PARK_LEAVE) !=
                  FRANKENPHP_PARK_DRAINED);
    }
    /* woken, for a task or for the drain: the loop tells which */
  }
}

/* Shared vars of background workers, see WorkerHandle::setVars() and
 * frankenphp_get_vars(): the persistent tables live in slots owned by the
 * Go side, which copies and frees them through these helpers. */
void frankenphp_vars_to_request(zval *return_value, HashTable *table) {
  zval persistent;
  ZVAL_ARR(&persistent, table);

  /* the writer checked the tree against its own memory_limit, not ours */
  size_t size;
  if (!persistent_zval_fits_request(&persistent, &size)) {
    ZVAL_NULL(return_value);
    zend_throw_exception_ex(spl_ce_RuntimeException, 0,
                            "FrankenPHP: the values to copy take %zu bytes, "
                            "more than what is left of memory_limit",
                            size);

    return;
  }

  persistent_zval_to_request(return_value, &persistent);
}

void frankenphp_vars_free(HashTable *table) {
  zval persistent;
  ZVAL_ARR(&persistent, table);
  persistent_zval_free(&persistent);
}

/* Lets go of a reference on a snapshot. Its slot holds one and so does each
 * reader while it copies the table, on a count only touched atomically: the
 * last to let go frees the snapshot, with no lock and no call into the Go
 * side. */
void frankenphp_vars_release(HashTable *table) {
  if (__atomic_sub_fetch(&table->gc.refcount, 1, __ATOMIC_SEQ_CST) == 0) {
    frankenphp_vars_free(table);
  }
}

ZEND_METHOD(FrankenPHP_WorkerHandle, getNamespaces) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }

  go_frankenphp_worker_namespaces(frankenphp_thread_index(), return_value);
}

ZEND_METHOD(FrankenPHP_WorkerHandle, setVars) {
  zend_string *namespace;
  zval *vars;
  ZEND_PARSE_PARAMETERS_START(2, 2)
  Z_PARAM_STR(namespace)
  Z_PARAM_ARRAY(vars)
  ZEND_PARSE_PARAMETERS_END();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }

  /* validate the whole tree first: persist and free recurse without a
   * guard of their own */
  const char *invalid = persistent_zval_invalid(vars);
  if (invalid != NULL) {
    zend_value_error("FrankenPHP\\WorkerHandle::setVars(): %s", invalid);
    RETURN_THROWS();
  }

  zval persistent;
  persistent_zval_persist(&persistent, vars);

  HashTable *old = NULL;
  char *error =
      go_frankenphp_set_vars(frankenphp_thread_index(), ZSTR_VAL(namespace),
                             ZSTR_LEN(namespace), Z_ARRVAL(persistent), &old);
  if (error != NULL) {
    frankenphp_vars_free(Z_ARRVAL(persistent));
    zend_throw_exception(spl_ce_LogicException, error, 0);
    free(error);
    RETURN_THROWS();
  }
  if (old != NULL) {
    frankenphp_vars_release(old);
  }
}

PHP_FUNCTION(frankenphp_get_vars) {
  zend_string *namespace;
  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_STR(namespace)
  ZEND_PARSE_PARAMETERS_END();

  HashTable *table = NULL;
  struct go_frankenphp_get_vars_return found =
      go_frankenphp_get_vars(frankenphp_thread_index(), ZSTR_VAL(namespace),
                             ZSTR_LEN(namespace), &table);
  if (found.r0 != NULL) {
    zend_throw_exception(found.r1 ? spl_ce_LogicException
                                  : spl_ce_RuntimeException,
                         found.r0, 0);
    free(found.r0);
    RETURN_THROWS();
  }

  /* the copy may bail out, past memory_limit or in an autoloader an enum
   * triggers: the snapshot is let go of whatever happens, so its publisher
   * can replace and free it */
  bool bailed_out = false;
  zend_try { frankenphp_vars_to_request(return_value, table); }
  zend_catch { bailed_out = true; }
  zend_end_try();

  frankenphp_vars_release(table);
  if (bailed_out) {
    zend_bailout();
  }
}

/* Tasks, see SentTaskHandle: the sender hands a persistent copy of the payload
 * to the Go side, which queues it for the background worker receiving its
 * namespace and wakes one of its threads; WorkerHandle::receive() dequeues it
 * there. Updates flow back the same way. Each side keeps the state of its end
 * on its handle, nothing travels on the descriptor: it is what a wait returns
 * on and what the Go side signals, which holds the state the methods report. */
typedef struct {
  uintptr_t task;
  /* the side's descriptor, see frankenphp_task_chan_open: -1 until the script
   * waits through stream_select() or a poll context, see frankenphp_task_fd */
  intptr_t fd;
  bool sender;
  bool closed;        /* this side ended the task */
  zend_resource *res; /* the stream of getStream(), NULL until a script asks */
  zval payload;       /* the receiver's side only */
  zval namespace;     /* the receiver's side only */
} frankenphp_task_data;

static zend_class_entry *frankenphp_sent_task_ce;
static zend_class_entry *frankenphp_received_task_ce;

/* Ends the task from this side, once: the sender abandons it, the receiver
 * completes it through complete() only, before the script ends, with the
 * result when it gave one. abort(), dropping the handle or ending the script
 * aborts the task instead, so a sender never takes a task let go of for a
 * completed one. The Go side learns it before signaling the other side, which
 * then finds it. */
static bool frankenphp_task_settle(frankenphp_task_data *data, bool completed,
                                   HashTable *result) {
  if (data->closed || data->task == 0) {
    return false;
  }
  data->closed = true;

  if (data->sender) {
    go_frankenphp_task_sender_close(data->task);

    return true;
  }

  /* whether the sender was still there to learn it */
  return go_frankenphp_task_receiver_close(
      data->task, !completed || (EG(flags) & EG_FLAGS_IN_SHUTDOWN) != 0,
      result);
}

/* The stream a script took for this side, NULL when it took none or
 * closed it. */
static php_stream *frankenphp_task_data_stream(frankenphp_task_data *data) {
  if (data->res == NULL || data->res->ptr == NULL ||
      data->res->type != php_file_le_stream()) {
    return NULL;
  }

  return data->res->ptr;
}

/* the data goes through the methods of the handles */
static ssize_t frankenphp_task_stream_read(php_stream *stream, char *buf,
                                           size_t count) {
  (void)buf;
  (void)count;
  frankenphp_task_data *data = stream->abstract;
  if (data->closed || go_frankenphp_task_side_gone(data->task, data->sender)) {
    stream->eof = 1;
  }

  return -1;
}

/* Closing the stream leaves the task alone, only the methods of its handle
 * end it: getStream() builds another one. The state is the handle's, which
 * closes the stream it handed out before it lets go of it. */
static int frankenphp_task_stream_close(php_stream *stream, int close_handle) {
  (void)stream;
  (void)close_handle;

  return 0;
}

/* The descriptor of this side, made the first time the script waits on the
 * task next to something else: until then its waits go through the mailbox
 * of the thread and the task has none. -1 when it could not be made. */
static intptr_t frankenphp_task_fd(frankenphp_task_data *data) {
  if (data->fd < 0) {
    data->fd = go_frankenphp_task_fd(data->task, data->sender);
  }

  return data->fd;
}

static int frankenphp_task_stream_cast(php_stream *stream, int castas,
                                       void **ret) {
  /* for a poller of any kind: one that asks again before each wait, as
   * stream_select() does, or one that keeps the descriptor, as an event loop
   * extension or a StreamPollHandle does */
  if (castas != PHP_STREAM_AS_FD_FOR_SELECT
#ifdef PHP_STREAM_AS_FD_FOR_POLL
      && castas != PHP_STREAM_AS_FD_FOR_POLL
#endif
  ) {
    return FAILURE;
  }
  frankenphp_task_data *data = stream->abstract;
  if (data->closed) {
    return FAILURE;
  }
  if (frankenphp_task_fd(data) < 0) {
    return FAILURE;
  }
  if (ret != NULL) {
    *(php_socket_t *)ret = (php_socket_t)data->fd;
  }

  return SUCCESS;
}

/* feof() is all the stream answers: it has no timeout, a wait that must end
 * goes through stream_select() or a poll context, which take one */
static int frankenphp_task_stream_set_option(php_stream *stream, int option,
                                             int value, void *ptrparam) {
  (void)value;
  (void)ptrparam;
  frankenphp_task_data *data = stream->abstract;
  if (option != PHP_STREAM_OPTION_CHECK_LIVENESS) {
    return PHP_STREAM_OPTION_RETURN_NOTIMPL;
  }

  /* the other side ended the task */
  return data->closed || go_frankenphp_task_side_gone(data->task, data->sender)
             ? PHP_STREAM_OPTION_RETURN_ERR
             : PHP_STREAM_OPTION_RETURN_OK;
}

/* no write: the stream is not writable */
static const php_stream_ops frankenphp_task_stream_ops = {
    NULL,
    frankenphp_task_stream_read,
    frankenphp_task_stream_close,
    NULL,
    "FrankenPHP task",
    NULL,
    frankenphp_task_stream_cast,
    NULL,
    frankenphp_task_stream_set_option,
};

/* Waits for the next event of the sender's side: false when
 * max_execution_time or a force-kill interrupted the wait, for the VM to bail
 * out once the method returns. On the mailbox of the thread, seq being its
 * counter read before the Go side was asked for the event; on the side's
 * descriptor once the script took it, without consuming the signal, and other
 * interruptions are retried there, like PHP's own stream code does. */
static bool frankenphp_task_wait(frankenphp_task_data *data, uint32_t seq) {
  if (data->fd < 0) {
    return frankenphp_mailbox_wait(seq, -1, FRANKENPHP_WAIT_ANSWER) >= 0;
  }

  /* in slices, like the sleep on a mailbox: a force-kill that no signal
   * carries to this thread, or one that landed right before the wait, is
   * noticed at the next one */
  while (!zend_atomic_bool_load(&EG(timed_out))) {
    int n = php_pollfd_for_ms((php_socket_t)data->fd, PHP_POLLREADABLE,
                              FRANKENPHP_WAIT_SLICE_MS);
    if (n > 0 || (n < 0 && php_socket_errno() != EINTR)) {
      return true;
    }
  }

  return false;
}

/* Handles of a task: the state above belongs to the handle of its side. On PHP
 * 8.6 they are Io\Poll\Handle, so a context waits on the descriptor and no
 * stream is built at all. */
static frankenphp_task_data *frankenphp_task_data_of(zend_object *object) {
  return FRANKENPHP_HANDLE_OF(object)->handle_data;
}

#define FRANKENPHP_TASK_DATA(zthis) frankenphp_task_data_of(Z_OBJ_P(zthis))

/* The state of a live task, NULL once this side ended it: complete(), abort()
 * and abandon() do, and every method but getStream() throws from there on. */
static frankenphp_task_data *frankenphp_task_obj_open(zval *zthis) {
  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(zthis);
  if (data == NULL || data->task == 0 || data->closed) {
    zend_throw_exception(spl_ce_LogicException, "the task is over", 0);

    return NULL;
  }

  return data;
}

/* The descriptor a task waits on, the same one its stream reports to
 * stream_select(); invalid once this side ended the task, which is what
 * the poll context and read() both report. */
static php_socket_t frankenphp_task_get_fd(frankenphp_handle_obj *handle) {
  frankenphp_task_data *data = handle->handle_data;
  if (data == NULL || data->task == 0 || data->closed ||
      frankenphp_task_fd(data) < 0) {
    return SOCK_ERR;
  }

  return (php_socket_t)data->fd;
}

static int frankenphp_task_is_valid(frankenphp_handle_obj *handle) {
  frankenphp_task_data *data = handle->handle_data;

  return data != NULL && data->task != 0 && !data->closed;
}

static void frankenphp_task_cleanup(frankenphp_handle_obj *handle) {
  frankenphp_task_data *data = handle->handle_data;
  if (data == NULL) {
    return;
  }
  handle->handle_data = NULL;

  frankenphp_task_settle(data, false, NULL);
  if (data->res != NULL) {
    zend_resource *res = data->res;
    data->res = NULL;
    /* the task went with the handle, so does a stream the script kept: its
     * descriptor, its state and the Go side of the task are gone */
    if (res->type == php_file_le_stream()) {
      zend_list_close(res);
    }
    zend_list_delete(res);
  }
  zval_ptr_dtor(&data->payload);
  zval_ptr_dtor(&data->namespace);
  efree(data);
}

static frankenphp_handle_ops frankenphp_task_poll_ops = {
    .get_fd = frankenphp_task_get_fd,
    .is_valid = frankenphp_task_is_valid,
    .cleanup = frankenphp_task_cleanup,
};

static zend_object *frankenphp_task_handle_new(zend_class_entry *ce) {
  zend_object *object =
      frankenphp_handle_obj_create(ce, &frankenphp_task_poll_ops);
  frankenphp_task_data *data = ecalloc(1, sizeof(*data));

  data->fd = -1;
  ZVAL_UNDEF(&data->payload);
  ZVAL_UNDEF(&data->namespace);
  FRANKENPHP_HANDLE_OF(object)->handle_data = data;

  return object;
}

/* The handle takes the task the Go side just handed out. */
static void frankenphp_task_obj_take(zval *zthis, uintptr_t task, bool sender) {
  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(zthis);

  data->task = task;
  data->sender = sender;
}

/* Builds the stream of this side on demand, so a script waiting through a poll
 * context allocates none, and hands the resource over: it may select on it and
 * close it, the next call then builds another one. Past the end of the task
 * the last one comes back closed, as the poll hooks report an invalid
 * descriptor there, so a waiter learns the task is over from is_resource(). */
static void frankenphp_task_obj_get_stream(zval *zthis, zval *return_value) {
  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(zthis);

  if (data != NULL && data->task != 0 && !data->closed &&
      frankenphp_task_data_stream(data) == NULL) {
    if (data->res != NULL) {
      /* the script closed the one it had */
      zend_list_delete(data->res);
    }
    php_stream *stream =
        php_stream_alloc(&frankenphp_task_stream_ops, data, NULL, "r");

    /* the handle keeps the reference the registration holds */
    data->res = stream->res;
  }

  if (data == NULL || data->res == NULL) {
    zend_throw_exception(spl_ce_LogicException, "the task is over", 0);
    RETURN_THROWS();
  }

  GC_ADDREF(data->res);
  RETURN_RES(data->res);
}

/* Ends the task from this side, with the result of a task being completed
 * when there is one, and closes the stream when the script took one, so its
 * stream_select() and feof() report the end. False when the task was over
 * already, or the other side gone. */
static bool frankenphp_task_obj_close(zval *zthis, bool completed,
                                      HashTable *result) {
  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(zthis);
  if (data == NULL || data->closed) {
    return false;
  }

  bool present = frankenphp_task_settle(data, completed, result);
  if (data->res != NULL && data->res->type == php_file_le_stream()) {
    zend_list_close(data->res);
  }

  return present;
}

ZEND_METHOD(FrankenPHP_SentTaskHandle, __construct) {
  zend_string *namespace;
  zval *payload;
  double pickup_timeout = 30.0;
  bool pickup_timeout_is_null = false;
  ZEND_PARSE_PARAMETERS_START(2, 3)
  Z_PARAM_STR(namespace)
  Z_PARAM_ARRAY(payload)
  Z_PARAM_OPTIONAL
  Z_PARAM_DOUBLE_OR_NULL(pickup_timeout, pickup_timeout_is_null)
  ZEND_PARSE_PARAMETERS_END();

  /* the waits of a task go through the mailbox of its thread, and the wake-ups
   * to the index the thread calls in with: a thread that borrows the index of
   * another would sleep on a mailbox nobody wakes */
  if (frankenphp_is_foreign_thread()) {
    zend_throw_exception(spl_ce_LogicException,
                         "FrankenPHP\\SentTaskHandle cannot be used from a "
                         "thread FrankenPHP did not start",
                         0);
    RETURN_THROWS();
  }

  /* a handle sends one task: a second construction would orphan the first */
  frankenphp_task_data *current = FRANKENPHP_TASK_DATA(ZEND_THIS);
  if (current == NULL || current->task != 0 || current->closed) {
    zend_throw_error(
        NULL,
        "FrankenPHP\\SentTaskHandle::__construct() cannot be called twice");
    RETURN_THROWS();
  }

  if (!pickup_timeout_is_null &&
      (zend_isnan(pickup_timeout) || pickup_timeout < 0)) {
    zend_argument_value_error(3, "must be greater than or equal to 0");
    RETURN_THROWS();
  }
  /* past what a duration holds, infinity included, waits forever like null */
  int timeout_ms = -1;
  if (!pickup_timeout_is_null && pickup_timeout * 1000 < (double)INT_MAX) {
    timeout_ms = (int)(pickup_timeout * 1000);
  }
  const char *invalid = persistent_zval_invalid(payload);
  if (invalid != NULL) {
    zend_value_error("FrankenPHP\\SentTaskHandle::__construct(): payload %s",
                     invalid);
    RETURN_THROWS();
  }

  zval persistent;
  persistent_zval_persist(&persistent, payload);

  /* the Go side owns the payload from here on, it frees it on failure, and
   * watches the time a thread has to pick the task up */
  struct go_frankenphp_send_task_return task = go_frankenphp_send_task(
      frankenphp_thread_index(), ZSTR_VAL(namespace), ZSTR_LEN(namespace),
      Z_ARRVAL(persistent), timeout_ms);
  if (task.r1 != NULL) {
    /* a namespace nobody receives is a mistake, a failure to set the task up
     * is not */
    zend_throw_exception(
        task.r2 ? spl_ce_LogicException : spl_ce_RuntimeException, task.r1, 0);
    free(task.r1);
    RETURN_THROWS();
  }

  /* the task is the handle's from here on: a bailout below, or a throw, settles
   * it rather than leaving it in the queue */
  frankenphp_task_obj_take(ZEND_THIS, task.r0, true);
  /* queued: read() waits for the pickup along with the first update */
}

ZEND_METHOD(FrankenPHP_SentTaskHandle, read) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_data *data = frankenphp_task_obj_open(ZEND_THIS);
  if (data == NULL) {
    RETURN_THROWS();
  }

  for (;;) {
    /* read before asking, so an event landing right after the answer is not
     * slept through */
    uint32_t seq = frankenphp_mailbox_seq();
    struct go_frankenphp_read_task_return update =
        go_frankenphp_read_task(data->task);
    switch (update.r1) {
    case FRANKENPHP_TASK_READ_UPDATE:
      zend_try { frankenphp_vars_to_request(return_value, update.r0); }
      zend_catch {
        frankenphp_vars_free(update.r0);
        zend_bailout();
      }
      zend_end_try();
      frankenphp_vars_free(update.r0);
      return;
    case FRANKENPHP_TASK_READ_OVER:
      php_stream *stream = frankenphp_task_data_stream(data);
      if (stream != NULL) {
        /* a script waiting on it with stream_select() or feof() sees the
         * end of the task there too */
        stream->eof = 1;
      }
      if (update.r2 == NULL) {
        RETURN_NULL();
      }
      /* the task failed: the worker let go of it without completing it, or no
       * thread will pick it up, on a drain, the shutdown, or because none did
       * in time. It left the queue then, so what the sender is told stands */
      zend_throw_exception(spl_ce_RuntimeException, update.r2, 0);
      free(update.r2);
      RETURN_THROWS();
    default:
      /* nothing yet: wait for the next event. The Go side watches what a
       * queued task waits for */
      if (!frankenphp_task_wait(data, seq)) {
        zend_throw_exception(spl_ce_RuntimeException,
                             "FrankenPHP\\SentTaskHandle::read(): the wait "
                             "was interrupted",
                             0);
        RETURN_THROWS();
      }
      /* the loop tells what the wait ended on */
    }
  }
}

ZEND_METHOD(FrankenPHP_SentTaskHandle, getStream) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_obj_get_stream(ZEND_THIS, return_value);
}

ZEND_METHOD(FrankenPHP_SentTaskHandle, abandon) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_obj_close(ZEND_THIS, false, NULL);
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, __construct) {
  zend_throw_error(NULL, "Cannot directly construct FrankenPHP\\"
                         "ReceivedTaskHandle, use WorkerHandle::receive()");
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, getPayload) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(ZEND_THIS);
  /* the payload outlives complete(), only an instance made behind the
   * private constructor's back has none */
  if (Z_ISUNDEF(data->payload)) {
    zend_throw_error(NULL, "FrankenPHP\\ReceivedTaskHandle was not handed "
                           "out by WorkerHandle::receive()");
    RETURN_THROWS();
  }

  RETURN_COPY(&data->payload);
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, getNamespace) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_data *data = FRANKENPHP_TASK_DATA(ZEND_THIS);
  if (Z_ISUNDEF(data->namespace)) {
    zend_throw_error(NULL, "FrankenPHP\\ReceivedTaskHandle was not handed "
                           "out by WorkerHandle::receive()");
    RETURN_THROWS();
  }

  RETURN_COPY(&data->namespace);
}

/* The persistent copy of an update of a live task, which the Go side owns
 * once it got it and frees when the update goes nowhere: NULL with an
 * exception when the task is over or the values cannot be copied. */
static HashTable *frankenphp_task_obj_persist(zval *zthis, zval *data,
                                              frankenphp_task_data **task) {
  *task = frankenphp_task_obj_open(zthis);
  if (*task == NULL) {
    return NULL;
  }
  const char *invalid = persistent_zval_invalid(data);
  if (invalid != NULL) {
    zend_value_error("FrankenPHP\\ReceivedTaskHandle: %s", invalid);
    return NULL;
  }

  zval persistent;
  persistent_zval_persist(&persistent, data);

  return Z_ARRVAL(persistent);
}

/* Queues an update for the sender, which reads it with read(): false when
 * the sender gave up, or stopped reading while the worker drains, the update
 * goes nowhere then. A sender leaving is no mistake of the worker, so it is
 * no exception either. */
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, update) {
  zval *data;
  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_ARRAY(data)
  ZEND_PARSE_PARAMETERS_END();

  frankenphp_task_data *task;
  HashTable *update = frankenphp_task_obj_persist(ZEND_THIS, data, &task);
  if (update == NULL) {
    RETURN_THROWS();
  }

  RETURN_BOOL(go_frankenphp_update_task(task->task, update));
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, complete) {
  zval *data = NULL;
  ZEND_PARSE_PARAMETERS_START(0, 1)
  Z_PARAM_OPTIONAL
  Z_PARAM_ARRAY_OR_NULL(data)
  ZEND_PARSE_PARAMETERS_END();

  /* the result goes with the end of the task, in one step: a sender that
   * leaves once it has it left a completed task */
  HashTable *result = NULL;
  if (data != NULL) {
    frankenphp_task_data *task;
    result = frankenphp_task_obj_persist(ZEND_THIS, data, &task);
    if (result == NULL) {
      RETURN_THROWS();
    }
  }

  RETURN_BOOL(frankenphp_task_obj_close(ZEND_THIS, true, result));
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, abort) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_obj_close(ZEND_THIS, false, NULL);
}

ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, getStream) {
  ZEND_PARSE_PARAMETERS_NONE();

  frankenphp_task_obj_get_stream(ZEND_THIS, return_value);
}

ZEND_METHOD(FrankenPHP_WorkerHandle, receive) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!frankenphp_worker_handle_usable()) {
    RETURN_THROWS();
  }

  /* never waits: tick() does, or the script on its handle */
  struct go_frankenphp_receive_task_return task =
      go_frankenphp_receive_task(frankenphp_thread_index());
  if (task.r0 == 0) {
    RETURN_NULL();
  }

  /* the task is this thread's from here on: a bailout while copying its
   * namespace or its payload, or while making the handle, must not leave it
   * open, the sender would wait on it forever. Which is why the namespace
   * comes as C memory to copy here, not as a string made inside the call into
   * the Go side, where a bailout would have nothing to end the task */
  zval namespace, payload;
  ZVAL_UNDEF(&namespace);
  ZVAL_UNDEF(&payload);
  zend_try {
    ZVAL_STRINGL(&namespace, task.r2, task.r3);
    frankenphp_vars_to_request(&payload, task.r1);
    if (!EG(exception)) {
      object_init_ex(return_value, frankenphp_received_task_ce);
    }
  }
  zend_catch {
    free(task.r2);
    frankenphp_vars_free(task.r1);
    zval_ptr_dtor(&namespace);
    go_frankenphp_task_receiver_close(task.r0, true, NULL);
    zend_bailout();
  }
  zend_end_try();
  free(task.r2);
  frankenphp_vars_free(task.r1);

  if (EG(exception)) {
    /* an enum of the payload does not resolve here: the task cannot be
     * processed, the sender is told so */
    zval_ptr_dtor(&payload);
    zval_ptr_dtor(&namespace);
    go_frankenphp_task_receiver_close(task.r0, true, NULL);
    RETURN_THROWS();
  }

  frankenphp_task_obj_take(return_value, task.r0, false);
  ZVAL_COPY_VALUE(&FRANKENPHP_TASK_DATA(return_value)->payload, &payload);
  ZVAL_COPY_VALUE(&FRANKENPHP_TASK_DATA(return_value)->namespace, &namespace);
}

/* {{{ thread-safe opcache reset */
PHP_FUNCTION(frankenphp_opcache_reset) {
  go_schedule_opcache_reset(frankenphp_thread_index());

  RETVAL_TRUE;
} /* }}} */

/* Try to override opcache_reset if opcache is loaded.
 * instead of resetting opcache, reboot all threads */
static void frankenphp_override_opcache_reset(void) {
  zend_function *func = zend_hash_str_find_ptr(
      CG(function_table), "opcache_reset", sizeof("opcache_reset") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION &&
      ((zend_internal_function *)func)->handler !=
          ZEND_FN(frankenphp_opcache_reset)) {
    ((zend_internal_function *)func)->handler =
        ZEND_FN(frankenphp_opcache_reset);
  }
}

#ifdef FRANKENPHP_TEST
/* Test-only entry point that exercises zval.h end-to-end:
 * validate -> persist (request -> persistent memory) ->
 * to_request (persistent -> fresh request memory) -> free persistent copy.
 * Compiled only when FRANKENPHP_TEST is defined; never registered
 * in production builds. */
ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(
    arginfo_frankenphp_test_persist_roundtrip, 0, 1, IS_MIXED, 0)
ZEND_ARG_TYPE_INFO(0, value, IS_MIXED, 0)
ZEND_END_ARG_INFO()

PHP_FUNCTION(frankenphp_test_persist_roundtrip) {
  zval *input;
  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_ZVAL(input)
  ZEND_PARSE_PARAMETERS_END();

  if (persistent_zval_invalid(input) != NULL) {
    zend_throw_exception(spl_ce_LogicException,
                         "persistent_zval: value not supported (only "
                         "scalars, arrays, and enums are allowed, nested "
                         "no deeper than PERSISTENT_ZVAL_MAX_DEPTH)",
                         0);
    RETURN_THROWS();
  }

  zval persistent;
  persistent_zval_persist(&persistent, input);
  persistent_zval_to_request(return_value, &persistent);
  persistent_zval_free(&persistent);
}

static const zend_function_entry frankenphp_test_hook_functions[] = {
    PHP_FE(frankenphp_test_persist_roundtrip,
           arginfo_frankenphp_test_persist_roundtrip) PHP_FE_END};
#endif

static zend_class_entry *frankenphp_ce_request_body_too_large_exception;

PHP_MINIT_FUNCTION(frankenphp) {
  frankenphp_worker_handle_ops = php_stream_socket_ops;
  frankenphp_worker_handle_ops.label = "FrankenPHP worker handle";
  frankenphp_worker_handle_ops.close = frankenphp_worker_handle_close;
  frankenphp_worker_handle_ops.read = frankenphp_worker_handle_read;

  frankenphp_handle_obj_handlers = std_object_handlers;
  frankenphp_handle_obj_handlers.offset = offsetof(frankenphp_handle_obj, std);
  frankenphp_handle_obj_handlers.free_obj = frankenphp_handle_obj_free;
  frankenphp_handle_obj_handlers.clone_obj = NULL;

  register_frankenphp_symbols(module_number);
  frankenphp_ce_request_body_too_large_exception =
      register_class_FrankenPHP_RequestBodyTooLargeException(zend_ce_exception);

  zend_class_entry *worker_handle_ce = register_class_FrankenPHP_WorkerHandle();
  worker_handle_ce->create_object = frankenphp_worker_handle_new;
  frankenphp_handle_implements_poll(worker_handle_ce);

  frankenphp_sent_task_ce = register_class_FrankenPHP_SentTaskHandle();
  frankenphp_sent_task_ce->create_object = frankenphp_task_handle_new;
  frankenphp_handle_implements_poll(frankenphp_sent_task_ce);

  frankenphp_received_task_ce = register_class_FrankenPHP_ReceivedTaskHandle();
  frankenphp_received_task_ce->create_object = frankenphp_task_handle_new;
  frankenphp_handle_implements_poll(frankenphp_received_task_ce);
#ifndef PHP_WIN32
  /* MINIT runs once per ZTS thread — guard the atfork registration */
  static pthread_once_t atfork_once = PTHREAD_ONCE_INIT;
  pthread_once(&atfork_once, frankenphp_register_atfork);
#endif

#ifdef FRANKENPHP_TEST
  if (zend_register_functions(NULL, frankenphp_test_hook_functions, NULL,
                              MODULE_PERSISTENT) == FAILURE) {
    return FAILURE;
  }
#endif

  zend_function *func;

  // Override putenv
  func = zend_hash_str_find_ptr(CG(function_table), "putenv",
                                sizeof("putenv") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION) {
    ((zend_internal_function *)func)->handler = ZEND_FN(frankenphp_putenv);
  } else {
    php_error(E_WARNING, "Failed to find built-in putenv function");
  }

  // Override getenv
  func = zend_hash_str_find_ptr(CG(function_table), "getenv",
                                sizeof("getenv") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION) {
    ((zend_internal_function *)func)->handler = ZEND_FN(frankenphp_getenv);
  } else {
    php_error(E_WARNING, "Failed to find built-in getenv function");
  }

  // Override opcache_reset (may not be available yet if opcache loads as a
  // shared extension in PHP 8.4 and below)
  frankenphp_override_opcache_reset();

  return SUCCESS;
}

static void frankenphp_print_info_rows(const char **entries) {
  for (int i = 0; entries[i] != NULL; i += 2) {
    php_info_print_table_row(2, entries[i], entries[i + 1]);
  }
}

PHP_MINFO_FUNCTION(frankenphp) {
#ifndef PHP_WIN32
  if (UNEXPECTED(is_forked_child)) {
    return;
  }
#endif

  struct go_frankenphp_collect_phpinfo_return data =
      go_frankenphp_collect_phpinfo();
  const char **entries = (const char **)data.r0;
  const char **modules = (const char **)data.r1;
  bool bailed_out = false;

  zend_try {
    /* no Go in here: printing may bailout, and a bailout must never
     * unwind a Go frame */
    php_info_print_table_start();
    php_info_print_table_row(2, "FrankenPHP", TOSTRING(FRANKENPHP_VERSION));
    if (entries) {
      frankenphp_print_info_rows(entries);
    }
    php_info_print_table_end();

    if (modules != NULL) {
      /* the Go module list is long: collapse it in HTML */
      if (sapi_module.phpinfo_as_text) {
        php_info_print_table_start();
        php_info_print_table_header(1, "Go modules");
        php_info_print_table_end();
      } else {
        php_printf("<details><summary style=\"cursor: pointer\">Go "
                   "modules</summary>\n");
      }

      php_info_print_table_start();
      php_info_print_table_header(2, "Module", "Version");
      frankenphp_print_info_rows(modules);
      php_info_print_table_end();

      if (!sapi_module.phpinfo_as_text) {
        php_printf("</details>\n");
      }
    }
  }
  zend_catch { bailed_out = true; }
  zend_end_try();

  /* the bailout is caught: safe to hand the tables back to Go */
  go_frankenphp_release_phpinfo(data.r0, data.r1);

  if (bailed_out) {
    /* re-raise the caught bailout */
    zend_bailout();
  }
}

static zend_module_entry frankenphp_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp",
    ext_functions,         /* function table */
    PHP_MINIT(frankenphp), /* initialization */
    NULL,                  /* shutdown */
    NULL,                  /* request initialization */
    NULL,                  /* request shutdown */
    PHP_MINFO(frankenphp), /* information */
    TOSTRING(FRANKENPHP_VERSION),
    STANDARD_MODULE_PROPERTIES};

static const zend_function_entry frankenphp_cli_functions[] = {
    ZEND_FE(frankenphp_log, arginfo_frankenphp_log) ZEND_FE_END};

static PHP_MINIT_FUNCTION(frankenphp_cli) {
  register_frankenphp_symbols(module_number);

  return SUCCESS;
}

/* same phpinfo section in CLI, without the server functions and hooks */
static zend_module_entry frankenphp_cli_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp-cli",
    frankenphp_cli_functions,  /* function table */
    PHP_MINIT(frankenphp_cli), /* initialization */
    NULL,                      /* shutdown */
    NULL,                      /* request initialization */
    NULL,                      /* request shutdown */
    PHP_MINFO(frankenphp),     /* information */
    TOSTRING(FRANKENPHP_VERSION),
    STANDARD_MODULE_PROPERTIES};

static int frankenphp_startup(sapi_module_struct *sapi_module) {
  php_import_environment_variables = get_full_env;

  int result = php_module_startup(sapi_module, &frankenphp_module);
#ifndef PHP_WIN32
  if (result == SUCCESS) {
    for (int sig = 1; sig < NSIG; sig++) {
      struct sigaction sa;
      if (sigaction(sig, NULL, &sa) == 0 && sa.sa_handler != SIG_DFL &&
          sa.sa_handler != SIG_IGN && !(sa.sa_flags & SA_ONSTACK)) {
        sa.sa_flags |= SA_ONSTACK;
        sigaction(sig, &sa, NULL);
      }
    }
  }
#endif
#if PHP_VERSION_ID < 80500
  if (result == SUCCESS) {
    /* Override opcache here again if loaded as a shared extension
     * (php 8.4 and under) */
    frankenphp_override_opcache_reset();
  }
#endif

  return result;
}

static int frankenphp_deactivate(void) { return SUCCESS; }

#ifdef ZTS
/* ext/date's post-deactivate hook frees last_errors, which its GINIT leaves
 * uninitialized, even when a failed startup skipped its RINIT
 * TODO: remove when only PHP versions with the fix are supported (8.4.28+,
 * 8.5.13+), see https://github.com/php/php-src/pull/25127 */
static void frankenphp_init_date_globals(void) {
  zend_module_entry *date =
      zend_hash_str_find_ptr(&module_registry, ZEND_STRL("date"));
  zend_date_globals *globals = ts_resource(*date->globals_id_ptr);
  globals->last_errors = NULL;
}
#endif

static void frankenphp_handle_aborted_connection(void) {
  /* a bailout would fail php_request_startup(), let the script's first write
   * notice the abort instead */
  if (PG(during_request_startup)) {
    return;
  }

  php_handle_aborted_connection();
}

static size_t frankenphp_ub_write(const char *str, size_t str_length) {
#ifndef PHP_WIN32
  if (UNEXPECTED(is_forked_child)) {
    return 0;
  }
#endif

  struct go_ub_write_return result =
      go_ub_write(frankenphp_thread_index(), (char *)str, str_length);

  if (result.r1) {
    frankenphp_handle_aborted_connection();
  }

  return result.r0;
}

static int frankenphp_send_headers(sapi_headers_struct *sapi_headers) {
#ifndef PHP_WIN32
  if (UNEXPECTED(is_forked_child)) {
    return SAPI_HEADER_SEND_FAILED;
  }
#endif

  if (SG(request_info).no_headers == 1) {
    return SAPI_HEADER_SENT_SUCCESSFULLY;
  }

  /* Use the response code PHP already parsed; reparsing http_status_line with
   * a fixed +9 offset read out of bounds for lines shorter than 9 bytes, e.g.
   * header("HTTP/"). */
  int status = SG(sapi_headers).http_response_code;
  if (!status) {
    status = 200;
  }

  bool success = go_write_headers(frankenphp_thread_index(), status,
                                  &sapi_headers->headers);
  if (success) {
    return SAPI_HEADER_SENT_SUCCESSFULLY;
  }

  return SAPI_HEADER_SEND_FAILED;
}

static void frankenphp_sapi_flush(void *server_context) {
#ifndef PHP_WIN32
  if (UNEXPECTED(is_forked_child)) {
    return;
  }
#endif

  sapi_send_headers();
  if (go_sapi_flush(frankenphp_thread_index())) {
    frankenphp_handle_aborted_connection();
  }
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes) {
  struct go_read_post_return result =
      go_read_post(frankenphp_thread_index(), buffer, count_bytes);

  /* php://input can't report read errors, throw so user code doesn't take
   * the truncated body for a complete one. Worker request startup and
   * shutdown run in frankenphp_handle_request()'s frame, not in user code. */
  zend_execute_data *ex = EG(current_execute_data);
  if (result.r1 && ex != NULL &&
      !(ex->func != NULL && ex->func->type == ZEND_INTERNAL_FUNCTION &&
        ex->func->internal_function.handler ==
            ZEND_FN(frankenphp_handle_request))) {
    zend_throw_exception(frankenphp_ce_request_body_too_large_exception,
                         "Request body too large", 0);
  }

  return result.r0;
}

static char *frankenphp_read_cookies(void) {
  return go_read_cookies(frankenphp_thread_index());
}

/* all variables with well defined keys can safely be registered like this */
static inline void frankenphp_register_trusted_var(zend_string *z_key,
                                                   char *value, size_t val_len,
                                                   HashTable *ht) {
  if (value == NULL) {
    zval empty;
    ZVAL_EMPTY_STRING(&empty);
    zend_hash_update_ind(ht, z_key, &empty);
    return;
  }
  size_t new_val_len = val_len;

  if (!should_filter_var ||
      sapi_module.input_filter(PARSE_SERVER, ZSTR_VAL(z_key), &value,
                               new_val_len, &new_val_len)) {
    zval z_value;
    ZVAL_STRINGL_FAST(&z_value, value, new_val_len);
    zend_hash_update_ind(ht, z_key, &z_value);
  }
}

/* Register known $_SERVER variables in bulk to avoid cgo overhead */
void frankenphp_register_server_vars(zval *track_vars_array,
                                     frankenphp_server_vars vars) {
  HashTable *ht = Z_ARRVAL_P(track_vars_array);
  zend_hash_extend(ht, vars.total_num_vars, 0);
  zend_hash_copy(ht, main_thread_env, NULL);

  // update values with variable strings
#define FRANKENPHP_REGISTER_VAR(name)                                          \
  frankenphp_register_trusted_var(frankenphp_strings.name, vars.name,          \
                                  vars.name##_len, ht)

  FRANKENPHP_REGISTER_VAR(remote_addr);
  FRANKENPHP_REGISTER_VAR(remote_host);
  FRANKENPHP_REGISTER_VAR(remote_port);
  FRANKENPHP_REGISTER_VAR(document_root);
  FRANKENPHP_REGISTER_VAR(path_info);
  if (vars.path_translated_len != 0) {
    FRANKENPHP_REGISTER_VAR(path_translated);
  }
  FRANKENPHP_REGISTER_VAR(php_self);
  FRANKENPHP_REGISTER_VAR(document_uri);
  FRANKENPHP_REGISTER_VAR(script_filename);
  FRANKENPHP_REGISTER_VAR(script_name);
  FRANKENPHP_REGISTER_VAR(ssl_cipher);
  FRANKENPHP_REGISTER_VAR(server_name);
  FRANKENPHP_REGISTER_VAR(server_port);
  if (vars.server_addr_len != 0) {
    FRANKENPHP_REGISTER_VAR(server_addr);
  }
  FRANKENPHP_REGISTER_VAR(content_length);
  FRANKENPHP_REGISTER_VAR(server_protocol);
  FRANKENPHP_REGISTER_VAR(http_host);
  FRANKENPHP_REGISTER_VAR(request_uri);

#undef FRANKENPHP_REGISTER_VAR

  /* update values with hard-coded zend_strings */
  zval zv;
  ZVAL_STR(&zv, frankenphp_strings.cgi11);
  zend_hash_update_ind(ht, frankenphp_strings.gateway_interface, &zv);
  ZVAL_STR(&zv, frankenphp_strings.frankenphp);
  zend_hash_update_ind(ht, frankenphp_strings.server_software, &zv);
  ZVAL_STR(&zv, vars.request_scheme);
  zend_hash_update_ind(ht, frankenphp_strings.request_scheme, &zv);
  ZVAL_STR(&zv, vars.ssl_protocol);
  zend_hash_update_ind(ht, frankenphp_strings.ssl_protocol, &zv);
  ZVAL_STR(&zv, vars.https);
  zend_hash_update_ind(ht, frankenphp_strings.https, &zv);

  /* update values with always empty strings */
  ZVAL_EMPTY_STRING(&zv);
  zend_hash_update_ind(ht, frankenphp_strings.auth_type, &zv);
  zend_hash_update_ind(ht, frankenphp_strings.remote_ident, &zv);
}

/** Create an immutable zend_string that lasts for the whole process **/
zend_string *frankenphp_init_persistent_string(const char *string, size_t len) {
  /* persistent strings will be ignored by the GC at the end of a request */
  zend_string *z_string = zend_string_init(string, len, 1);
  zend_string_hash_val(z_string);

  /* interned strings will not be ref counted by the GC */
  GC_ADD_FLAGS(z_string, IS_STR_INTERNED);

  return z_string;
}

/* initialize all hard-coded zend_strings once per process */
static void frankenphp_init_interned_strings(void) {
  if (frankenphp_strings.remote_addr != NULL) {
    return; /* already initialized */
  }

#define F_INITIALIZE_FIELD(name, str)                                          \
  frankenphp_strings.name =                                                    \
      frankenphp_init_persistent_string(str, sizeof(str) - 1);

  FRANKENPHP_INTERNED_STRINGS_LIST(F_INITIALIZE_FIELD)
#undef F_INITIALIZE_FIELD
}

/* Register variables from SG(request_info) into $_SERVER */
static inline void
frankenphp_register_variable_from_request_info(zend_string *zKey, char *value,
                                               bool must_be_present,
                                               zval *track_vars_array) {
  if (value != NULL) {
    frankenphp_register_trusted_var(zKey, value, strlen(value),
                                    Z_ARRVAL_P(track_vars_array));
  } else if (must_be_present) {
    frankenphp_register_trusted_var(zKey, NULL, 0,
                                    Z_ARRVAL_P(track_vars_array));
  }
}

static void
frankenphp_register_variables_from_request_info(zval *track_vars_array) {
  frankenphp_register_variable_from_request_info(
      frankenphp_strings.content_type, (char *)SG(request_info).content_type,
      true, track_vars_array);
  frankenphp_register_variable_from_request_info(
      frankenphp_strings.query_string, SG(request_info).query_string, true,
      track_vars_array);
  frankenphp_register_variable_from_request_info(frankenphp_strings.remote_user,
                                                 SG(request_info).auth_user,
                                                 false, track_vars_array);
  frankenphp_register_variable_from_request_info(
      frankenphp_strings.request_method,
      (char *)SG(request_info).request_method, false, track_vars_array);
}

/* Only hard-coded keys may be registered this way */
void frankenphp_register_known_variable(zend_string *z_key, char *value,
                                        size_t val_len,
                                        zval *track_vars_array) {
  frankenphp_register_trusted_var(z_key, value, val_len,
                                  Z_ARRVAL_P(track_vars_array));
}

/* variables with user-defined keys must be registered safely
 * see: php_variables.c -> php_register_variable_ex (#1106) */
void frankenphp_register_variable_safe(char *key, char *val, size_t val_len,
                                       zval *track_vars_array) {
  if (key == NULL) {
    return;
  }
  if (val == NULL) {
    val = "";
  }
  size_t new_val_len = val_len;
  if (!should_filter_var ||
      sapi_module.input_filter(PARSE_SERVER, key, &val, new_val_len,
                               &new_val_len)) {
    php_register_variable_safe(key, val, new_val_len, track_vars_array);
  }
}

static void frankenphp_register_variables(zval *track_vars_array) {
  /* https://www.php.net/manual/en/reserved.variables.server.php */

  /* In CGI mode, the environment is part of the $_SERVER variables.
   * $_SERVER and $_ENV should only contain values from the original
   * environment, not values added though putenv
   */
  /* import environment and CGI variables from the request context in go */
  bool has_prepared_env =
      go_register_server_variables(frankenphp_thread_index(), track_vars_array);

  /* Some variables are already present in SG(request_info) */
  frankenphp_register_variables_from_request_info(track_vars_array);

  /* The prepared environment is registered last and can overwrite any value */
  if (has_prepared_env) {
    frankenphp_copy_prepared_env(Z_ARRVAL_P(track_vars_array));
  }
}

static void frankenphp_log_message(const char *message, int syslog_type_int) {
  go_log(frankenphp_thread_index(), (char *)message, syslog_type_int);
}

static char *frankenphp_getenv(const char *name, size_t name_len) {
  zval *env_val = frankenphp_lookup_env(name, name_len);
  if (env_val && Z_TYPE_P(env_val) == IS_STRING) {
    zend_string *str = Z_STR_P(env_val);
    return ZSTR_VAL(str);
  }

  return NULL;
}

/* php.ini files and php_ini directives override these defaults. */
static void frankenphp_ini_defaults(HashTable *configuration_hash) {
  if (go_is_hot_reload_enabled()) {
    /* Otherwise, a page reloaded right after a change may run the old code. */
    zval tmp;
    ZVAL_NEW_STR(&tmp, zend_string_init("0", sizeof("0") - 1, 1));
    zend_hash_str_update(configuration_hash, "opcache.revalidate_freq",
                         sizeof("opcache.revalidate_freq") - 1, &tmp);
  }
}

sapi_module_struct frankenphp_sapi_module = {
    "frankenphp", /* name */
    "FrankenPHP", /* pretty name */

    frankenphp_startup,          /* startup */
    php_module_shutdown_wrapper, /* shutdown */

    NULL,                  /* activate */
    frankenphp_deactivate, /* deactivate */

    frankenphp_ub_write,   /* unbuffered write */
    frankenphp_sapi_flush, /* flush */
    NULL,                  /* get uid */
    frankenphp_getenv,     /* getenv */

    php_error, /* error handler */

    NULL,                    /* header handler */
    frankenphp_send_headers, /* send headers handler */
    NULL,                    /* send header handler */

    frankenphp_read_post,    /* read POST data */
    frankenphp_read_cookies, /* read Cookies */

    frankenphp_register_variables, /* register server variables */
    frankenphp_log_message,        /* Log message */
    NULL,                          /* Get request time */
#if PHP_VERSION_ID < 80700
    NULL, /* Child terminate */
#endif

    STANDARD_SAPI_MODULE_PROPERTIES};

/* Sets thread name for profiling and debugging.
 *
 * Adapted from https://github.com/Pithikos/C-Thread-Pool
 * Copyright: Johan Hanssen Seferidis
 * License: MIT
 */
static void set_thread_name(char *thread_name) {
#ifdef __linux__
  /* Use prctl instead to prevent using _GNU_SOURCE flag and implicit
   * declaration */
  prctl(PR_SET_NAME, thread_name);
#elif defined(__APPLE__) && defined(__MACH__)
  pthread_setname_np(thread_name);
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
  pthread_set_name_np(pthread_self(), thread_name);
#endif
}

/* Like php-fpm, answer a script that can't be opened instead of compiling it */
static bool open_primary_script(zend_file_handle *file_handle) {
  bool orig_display_errors = PG(display_errors);
  PG(display_errors) = 0;
  zend_result result = zend_stream_open(file_handle);
  PG(display_errors) = orig_display_errors;
  if (EXPECTED(result == SUCCESS)) {
    return true;
  }

  int open_errno = errno;
  zend_try {
    if (open_errno == EACCES) {
      SG(sapi_headers).http_response_code = 403;
      PUTS("Access denied.\n");
    } else {
      SG(sapi_headers).http_response_code = 404;
      PUTS("No input file specified.\n");
    }
  }
  zend_catch {}
  zend_end_try();

  /* a worker whose script disappeared must count as failing */
  EG(exit_status) = 255;
  return false;
}

static inline void reset_sandboxed_environment() {
  if (sandboxed_env != NULL) {
    zend_hash_release(sandboxed_env);
    sandboxed_env = NULL;
  }
  if (prepared_env != NULL) {
    zend_hash_release(prepared_env);
    prepared_env = NULL;
  }
  if (worker_env != NULL) {
    zend_hash_release(worker_env);
    worker_env = NULL;
  }
}

/* Adds a key/value pair to the per-thread prepared environment, exposing
 *  env vars from the php(_server) directive to getenv() and $_ENV. */
void frankenphp_add_to_prepared_env(char *name, size_t name_len, char *val,
                                    size_t val_len, size_t size) {
  if (prepared_env == NULL) {
    prepared_env = zend_new_array(size);
  }
  zval zv = {0};
  ZVAL_STRINGL(&zv, val, val_len);
  zend_hash_str_update(prepared_env, name, name_len, &zv);
}

#if defined(ZTS) && PHP_VERSION_ID < 80700
/* Destroys the persistent resources of the thread (pfsockopen(), persistent
 * PDO connections...) while the thread and its modules are still intact.
 * Since PHP 8.4 (php-src 5941cdaa), ts_free_thread() no longer does it:
 * zend_shutdown() destroys the lists of the threads still registered in TSRM,
 * which a thread that exits earlier is not, so every thread restart (reboot,
 * max_requests) leaked them. Up to PHP 8.3, ts_free_thread() destroys the
 * list itself, but a reboot with a persistent stream then aborts with a
 * corrupted heap. PHP 8.7 destroys the list in ts_free_thread() again
 * (php-src 0a98b979), the same way as here. */
static void frankenphp_destroy_thread_persistent_list(void) {
  dtor_func_t destructor = EG(persistent_list).pDestructor;
  volatile bool completed = false;

  do {
    zend_try {
      zend_hash_graceful_reverse_destroy(&EG(persistent_list));
      completed = true;
    }
    zend_end_try();
  } while (!completed);

  /* An empty list again, so that destroying it a second time is harmless */
  zend_hash_init(&EG(persistent_list), 8, NULL, destructor, 1);
}
#endif

static void *php_thread(void *arg) {
  thread_index = (uintptr_t)arg;
  char thread_name[16] = {0};
  snprintf(thread_name, 16, "php-%" PRIxPTR, thread_index);
  set_thread_name(thread_name);

#ifdef FRANKENPHP_HAS_KILL_SIGNAL
  /* The spawning Go-managed M may block realtime signals, which the
   * new pthread inherits. Unblock FRANKENPHP_KILL_SIGNAL here so
   * force-kill deliveries are not silently dropped. */
  sigset_t unblock;
  sigemptyset(&unblock);
  sigaddset(&unblock, FRANKENPHP_KILL_SIGNAL);
  pthread_sigmask(SIG_UNBLOCK, &unblock, NULL);
#endif

  /* Initial allocation of all global PHP memory for this thread */
#ifdef ZTS
  (void)ts_resource(0);
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
  frankenphp_init_date_globals();
#endif

  /* Publish this thread's force-kill slot to Go so the graceful-drain
   * grace period can wake it from a busy PHP loop or blocking syscall. */
  frankenphp_register_thread_for_kill(thread_index);

  bool thread_is_healthy = true;
  bool has_attempted_shutdown = false;

  /* Main loop of the PHP thread, execute a PHP script and repeat until Go
   * signals to stop */
  zend_first_try {
    char *scriptName = NULL;
    while ((scriptName = go_frankenphp_before_script_execution(thread_index))) {
      has_attempted_shutdown = false;

      frankenphp_update_request_context();

      if (UNEXPECTED(php_request_startup() == FAILURE)) {
        /* only the request failed, the thread can serve the next one */
        has_attempted_shutdown = true;
        if (!SG(headers_sent)) {
          SG(sapi_headers).http_response_code = 500;
        }
        reset_sandboxed_environment();
        php_request_shutdown((void *)0);
        sandboxed_env = NULL;
        frankenphp_free_request_context();
        go_frankenphp_after_script_execution(thread_index, EG(exit_status));
        continue;
      }

#if PHP_VERSION_ID < 80500
      /* Override opcache here again if loaded as a shared extension
       * (php 8.4 and under) */
      frankenphp_override_opcache_reset();
#endif

      zend_file_handle file_handle;
      zend_stream_init_filename(&file_handle, scriptName);

      file_handle.primary_script = 1;
      EG(exit_status) = 0;

      /* Execute the PHP script, potential bailout to zend_catch. A body over
       * the limit was rejected while parsing the request. */
      if (EXPECTED(!go_is_context_done(thread_index)) &&
          EXPECTED(open_primary_script(&file_handle))) {
        php_execute_script(&file_handle);
      }
#ifndef PHP_WIN32
      if (UNEXPECTED(is_forked_child)) {
        _exit(EG(exit_status));
      }
#endif
      zend_destroy_file_handle(&file_handle);
      reset_sandboxed_environment();

      /* Update the last memory usage for metrics */
      __atomic_store_n(&thread_metrics[thread_index].last_memory_usage,
                       zend_memory_usage(0), __ATOMIC_RELAXED);

      has_attempted_shutdown = true;

#ifdef HAVE_PHP_SESSION
      /* A bailout inside a user save handler skips the cleanup that clears
       * PS(in_save_handler); RSHUTDOWN's recursion guard then refuses to run
       * the handler's close() and any resource it holds leaks
       * (https://github.com/php/frankenphp/issues/2368). See
       * https://github.com/php/php-src/blob/900797e54fb8d21a761205e5788b9275dc1c7c0e/ext/session/mod_user.c#L29
       * fpm doesn't run into this because it kills timed out processes, which
       * releases all resources:
       * https://github.com/php/php-src/blob/900797e54fb8d21a761205e5788b9275dc1c7c0e/sapi/fpm/fpm/fpm_request.c#L276
       */
      PS(in_save_handler) = false;
#endif

      /* shutdown the request, potential bailout to zend_catch */
      php_request_shutdown((void *)0);
      /* a putenv() from a shutdown function or destructor created a new
       * sandbox on the request heap, which php_request_shutdown() just freed */
      sandboxed_env = NULL;
      frankenphp_free_request_context();
      go_frankenphp_after_script_execution(thread_index, EG(exit_status));
    }
  }
  zend_catch {
#ifndef PHP_WIN32
    if (UNEXPECTED(is_forked_child)) {
      _exit(EG(exit_status));
    }
#endif

    /* Critical failure from php_execute_script or php_request_shutdown, mark
     * the thread as unhealthy */
    thread_is_healthy = false;
    if (!has_attempted_shutdown) {
      /* php_request_shutdown() was not called, force a shutdown now */
      reset_sandboxed_environment();
      zend_try { php_request_shutdown((void *)0); }
      zend_catch {}
      zend_end_try();
      sandboxed_env = NULL;
    }

    /* Log the last error message, it must be cleared to prevent a crash when
     * freeing execution globals */
    if (PG(last_error_message)) {
      go_log_attrs(thread_index, PG(last_error_message), 8, NULL);
      PG(last_error_message) = NULL;
      PG(last_error_file) = NULL;
    }
    frankenphp_free_request_context();
    go_frankenphp_after_script_execution(thread_index, EG(exit_status));
  }
  zend_end_try();

  /* frankenphp_update_local_thread_context() only releases the stop socket on
   * recycle: close it here too, so it does not outlive the thread on shutdown,
   * reboot or an unhealthy exit. */
  if (is_background_worker) {
    frankenphp_reset_background_worker();
  }

  /* Must precede ts_free_thread: that frees the TSRM storage backing
   * the slot's &EG() pointers. Clearing first means any concurrent
   * force-kill either ran before us or sees a zero slot. */
  go_frankenphp_clear_force_kill_slot(thread_index);

  /* free all global PHP memory reserved for this thread */
#ifdef ZTS
#if PHP_VERSION_ID < 80700
  frankenphp_destroy_thread_persistent_list();
#endif
  ts_free_thread();
#endif

  /* Thread is healthy, signal to Go that the thread has shut down */
  if (thread_is_healthy) {
    go_frankenphp_on_thread_shutdown(thread_index);
    return NULL;
  }

  go_log(thread_index, "Restarting unhealthy thread", LOG_WARNING);

  if (!frankenphp_new_php_thread(thread_index)) {
    /* probably unreachable */
    go_log(thread_index, "Failed to restart an unhealthy thread", LOG_ERR);
  }

  return NULL;
}

static void *php_main(void *arg) {
#ifndef ZEND_WIN32
  /*
   * SIGPIPE must be masked in non-Go threads:
   * https://pkg.go.dev/os/signal#hdr-Go_programs_that_use_cgo_or_SWIG
   */
  sigset_t set;
  sigemptyset(&set);
  sigaddset(&set, SIGPIPE);

  if (pthread_sigmask(SIG_BLOCK, &set, NULL) != 0) {
    perror("failed to block SIGPIPE");
    exit(EXIT_FAILURE);
  }
#endif

  set_thread_name("php-main");

#ifdef ZTS
#if (PHP_VERSION_ID >= 80300)
  php_tsrm_startup_ex((intptr_t)arg);
#else
  php_tsrm_startup();
#endif
/*tsrm_error_set(TSRM_ERROR_LEVEL_INFO, NULL);*/
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

  sapi_startup(&frankenphp_sapi_module);

  /* TODO: adapted from https://github.com/php/php-src/pull/16958, remove when
   * merged. */
#ifdef PHP_WIN32
  {
    const DWORD flags = GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                        GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT;
    HMODULE module;
    /* Use a larger buffer to support long module paths on Windows. */
    wchar_t filename[32768];
    if (GetModuleHandleExW(flags, (LPCWSTR)&frankenphp_sapi_module, &module)) {
      const DWORD filename_capacity = (DWORD)_countof(filename);
      DWORD len = GetModuleFileNameW(module, filename, filename_capacity);
      if (len > 0 && len < filename_capacity) {
        wchar_t *slash = wcsrchr(filename, L'\\');
        if (slash) {
          *slash = L'\0';
          if (!SetDllDirectoryW(filename)) {
            fprintf(stderr, "Warning: SetDllDirectoryW failed (error %lu)\n",
                    GetLastError());
          }
        }
      }
    }
  }
#endif

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /* overwrite php.ini with custom user settings */
  char *php_ini_overrides = go_get_custom_php_ini(false);
#else
  /* overwrite php.ini with custom user settings and disable
   * max_execution_timers */
  char *php_ini_overrides = go_get_custom_php_ini(true);
#endif

  if (php_ini_overrides != NULL) {
    frankenphp_sapi_module.ini_entries = php_ini_overrides;
  }

  frankenphp_sapi_module.ini_defaults = frankenphp_ini_defaults;

  frankenphp_init_interned_strings();

  /* take a snapshot of the environment for sandboxing */
  if (main_thread_env == NULL) {
    main_thread_env = pemalloc(sizeof(HashTable), 1);
    zend_hash_init(main_thread_env, 8, NULL, NULL, 1);
    go_init_os_env(main_thread_env);
  }

  frankenphp_sapi_module.startup(&frankenphp_sapi_module);

#if FRANKENPHP_OPCACHE_RESTART_HOOK
  /* Report the opcache restarts that opcache schedules on its own */
  zend_accel_schedule_restart_hook = frankenphp_opcache_restart_hook;
#endif

  /* check if a default filter is set in php.ini and only filter if
   * it is, this is deprecated and will be removed in PHP 9 */
  char *default_filter;
  cfg_get_string("filter.default", &default_filter);
  should_filter_var = default_filter != NULL;
  original_user_abort_setting = PG(ignore_user_abort);

  go_frankenphp_main_thread_is_ready();

  /* channel closed, shutdown gracefully. drainPHPThreads has already
   * waited for every PHP thread to exit (state.Done), so SAPI/TSRM
   * teardown here is safe. */
  frankenphp_sapi_module.shutdown(&frankenphp_sapi_module);

  sapi_shutdown();
#ifdef ZTS
  tsrm_shutdown();
#endif

  if (frankenphp_sapi_module.ini_entries) {
    free((char *)frankenphp_sapi_module.ini_entries);
    frankenphp_sapi_module.ini_entries = NULL;
  }

  go_frankenphp_shutdown_main_thread();

  return NULL;
}

int frankenphp_new_main_thread(int num_threads) {
  pthread_t thread;

  if (pthread_create(&thread, NULL, &php_main, (void *)(intptr_t)num_threads) !=
      0) {
    return -1;
  }

  return pthread_detach(thread);
}

bool frankenphp_new_php_thread(uintptr_t thread_index) {
  pthread_t thread;
  if (pthread_create(&thread, NULL, &php_thread, (void *)thread_index) != 0) {
    return false;
  }
  pthread_detach(thread);
  return true;
}

/* }}} */

static void *execute_script_cli(void *arg) {
  cli_exec_args_t *args = (cli_exec_args_t *)arg;

#ifndef PHP_WIN32
  sigset_t cli_signals;
  frankenphp_fill_cli_signal_set(&cli_signals);
  pthread_sigmask(SIG_UNBLOCK, &cli_signals, NULL);
#endif

#if PHP_VERSION_ID >= 80600
  return (void *)(intptr_t)do_php_cli(args->argc, args->argv);
#else
  return (void *)(intptr_t)emulate_script_cli(args);
#endif
}

static int (*previous_php_register_internal_extensions_func)(void) = NULL;

/* the CLI SAPIs take no extra modules: hook their module startup */
static int register_frankenphp_module(void) {
  if (previous_php_register_internal_extensions_func() != SUCCESS) {
    return FAILURE;
  }

#ifndef PHP_WIN32
  /* mark only: a CLI child must survive its PHP thread's exit */
  static pthread_once_t cli_atfork_once = PTHREAD_ONCE_INIT;
  pthread_once(&cli_atfork_once, frankenphp_register_cli_atfork);
#endif

  return zend_register_internal_module(&frankenphp_cli_module) == NULL
             ? FAILURE
             : SUCCESS;
}

int frankenphp_execute_script_cli(char *script, int argc, char **argv,
                                  bool eval) {
  pthread_t thread;
  int err;
  void *exit_status;

  cli_exec_args_t args = {
      .script = script, .argc = argc, .argv = argv, .eval = eval};

  /* a failed join leaves the hook installed: don't save it as its own
   * predecessor */
  if (php_register_internal_extensions_func != register_frankenphp_module) {
    previous_php_register_internal_extensions_func =
        php_register_internal_extensions_func;
  }
  php_register_internal_extensions_func = register_frankenphp_module;

  /*
   * Start the script in a dedicated thread to prevent conflicts between Go and
   * PHP signal handlers
   */
  err = pthread_create(&thread, NULL, execute_script_cli, &args);
  if (err != 0) {
    php_register_internal_extensions_func =
        previous_php_register_internal_extensions_func;
    return err;
  }

  err = pthread_join(thread, &exit_status);
  if (err != 0) {
    /* the CLI thread may still be inside the hook */
    return err;
  }

  php_register_internal_extensions_func =
      previous_php_register_internal_extensions_func;
  return (intptr_t)exit_status;
}

int frankenphp_get_current_memory_limit() { return PG(memory_limit); }

void frankenphp_init_thread_metrics(int max_threads) {
  thread_metrics = calloc(max_threads, sizeof(frankenphp_thread_metrics));
}

void frankenphp_destroy_thread_metrics(void) {
  free(thread_metrics);
  thread_metrics = NULL;
}

size_t frankenphp_get_thread_memory_usage(uintptr_t thread_index) {
  return __atomic_load_n(&thread_metrics[thread_index].last_memory_usage,
                         __ATOMIC_RELAXED);
}

static zend_module_entry **modules = NULL;
static int modules_len = 0;
static int (*original_php_register_internal_extensions_func)(void) = NULL;

int register_internal_extensions(void) {
  if (original_php_register_internal_extensions_func != NULL &&
      original_php_register_internal_extensions_func() != SUCCESS) {
    return FAILURE;
  }

  /* modules is kept for the process lifetime: every php_module_startup()
   * (main thread reboots, a new Init()) registers the modules again */
  for (int i = 0; i < modules_len; i++) {
    if (zend_register_internal_module(modules[i]) == NULL) {
      return FAILURE;
    }
  }

  return SUCCESS;
}

void register_extensions(zend_module_entry **m, int len) {
  /* install the hook only once, or it would become its own predecessor */
  if (modules != NULL || len <= 0) {
    return;
  }

  modules = pemalloc(len * sizeof(*modules), 1);
  memcpy(modules, m, len * sizeof(*modules));
  modules_len = len;

  original_php_register_internal_extensions_func =
      php_register_internal_extensions_func;
  php_register_internal_extensions_func = register_internal_extensions;
}
