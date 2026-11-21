#define _GNU_SOURCE
#include <errno.h>
#include <pthread.h>
#include <signal.h>
#include <stdatomic.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "_cgo_export.h"
#include "drain.h"

static int notify_pipe[2] = {-1, -1};
static int release_pipe[2] = {-1, -1};
static __thread int php_thread_idx = -1;
static atomic_int crashed;

static int g_crash_idx, g_mode, g_own_altstack;

/* Called by the Go runtime (sigfwdgo) when the fault is not in Go code. */
static void crash_handler(int sig, siginfo_t *info, void *ctx) {
  (void)info;
  (void)ctx;
  int idx = php_thread_idx;
  int expected = 0;

  if (idx >= 0 && atomic_compare_exchange_strong(&crashed, &expected, 1)) {
    int32_t msg = idx;
    if (write(notify_pipe[1], &msg, sizeof(msg)) == sizeof(msg)) {
      char c;
      /* park until Go is done draining */
      while (read(release_pipe[0], &c, 1) < 0 && errno == EINTR) {
      }
    }
  }

  /* default action, then return: the faulting instruction runs again and
   * the process dies by the original signal, with the original stack */
  struct sigaction sa;
  memset(&sa, 0, sizeof(sa));
  sa.sa_handler = SIG_DFL;
  sigaction(sig, &sa, NULL);
}

/* runs before the Go runtime starts, so that Go saves the handler and
 * forwards to it the faults that happen outside Go code */
__attribute__((constructor)) static void install_crash_handler(void) {
  if (pipe(notify_pipe) != 0 || pipe(release_pipe) != 0) {
    abort();
  }
  if (getenv("DRAIN") && strcmp(getenv("DRAIN"), "0") == 0) {
    return;
  }
  struct sigaction sa;
  memset(&sa, 0, sizeof(sa));
  sa.sa_sigaction = crash_handler;
  sa.sa_flags = SA_SIGINFO | SA_ONSTACK;
  sigfillset(&sa.sa_mask);
  sigaction(SIGSEGV, &sa, NULL);
  sigaction(SIGBUS, &sa, NULL);
}

int notify_fd(void) { return notify_pipe[0]; }

void release_crashed(void) {
  char c = 1;
  if (write(release_pipe[1], &c, 1) != 1) {
    abort();
  }
}

__attribute__((noinline)) void fault_in_c(void) {
  volatile int *p = NULL;
  *p = 42;
}

__attribute__((noinline)) static int overflow(volatile int depth) {
  volatile char buf[4096];
  buf[0] = (char)depth;
  return overflow(depth + 1) + buf[0];
}

static void *php_thread(void *arg) {
  int idx = (int)(intptr_t)arg;
  php_thread_idx = idx;

  if (g_own_altstack) {
    stack_t ss;
    ss.ss_sp = malloc(64 * 1024);
    ss.ss_size = 64 * 1024;
    ss.ss_flags = 0;
    sigaltstack(&ss, NULL);
  }

  usleep(idx * 70000);
  for (int req = 0;; req++) {
    if (!go_request_start(idx, req)) {
      break;
    }
    usleep(150000);
    if (idx == g_crash_idx && req == 1) {
      switch (g_mode) {
      case 0: {
        volatile int *p = NULL;
        *p = 42;
        break;
      }
      case 1:
        overflow(0);
        break;
      case 2:
        go_nil_deref();
        break;
      case 3:
        go_fault_via_c();
        break;
      }
    }
    usleep(150000);
    go_request_done(idx, req);
  }
  go_thread_exit(idx);

  return NULL;
}

void start_threads(int n, int crash_idx, int mode, int own_altstack) {
  g_crash_idx = crash_idx;
  g_mode = mode;
  g_own_altstack = own_altstack;
  for (int i = 0; i < n; i++) {
    pthread_t t;
    pthread_create(&t, NULL, php_thread, (void *)(intptr_t)i);
    pthread_detach(t);
  }
}
