#include "extension.h"
#include <php.h>
#include <zend_exceptions.h>

#include "_cgo_export.h"

#ifndef PHP_WIN32
#include <signal.h>

static const int test_signals[] = {SIGSEGV, SIGVTALRM};
#define TEST_SIGNALS_COUNT (sizeof(test_signals) / sizeof(test_signals[0]))

static struct sigaction original_actions[TEST_SIGNALS_COUNT];
static struct sigaction installed_actions[TEST_SIGNALS_COUNT];

static void test_handler(int sig, siginfo_t *info, void *context) {
  for (size_t i = 0; i < TEST_SIGNALS_COUNT; i++) {
    if (test_signals[i] == sig) {
      original_actions[i].sa_sigaction(sig, info, context);
    }
  }
}

static PHP_MINIT_FUNCTION(test_signals) {
  for (size_t i = 0; i < TEST_SIGNALS_COUNT; i++) {
    struct sigaction sa = {0};
    sa.sa_sigaction = test_handler;
    sa.sa_flags = SA_SIGINFO | SA_RESTART;
    sigemptyset(&sa.sa_mask);
    sigaddset(&sa.sa_mask, SIGUSR2);
    if (sigaction(test_signals[i], &sa, &original_actions[i]) != 0 ||
        sigaction(test_signals[i], NULL, &installed_actions[i]) != 0) {
      return FAILURE;
    }
  }

  return SUCCESS;
}

static PHP_MSHUTDOWN_FUNCTION(test_signals) {
  for (size_t i = 0; i < TEST_SIGNALS_COUNT; i++) {
    if (sigaction(test_signals[i], &original_actions[i], NULL) != 0) {
      return FAILURE;
    }
  }

  return SUCCESS;
}
#endif

int test_signal_handler(void) {
#ifndef PHP_WIN32
  for (size_t i = 0; i < TEST_SIGNALS_COUNT; i++) {
    struct sigaction sa;
    if (sigaction(test_signals[i], NULL, &sa) != 0 ||
        sa.sa_sigaction != installed_actions[i].sa_sigaction ||
        sa.sa_flags != (installed_actions[i].sa_flags | SA_ONSTACK)) {
      return FAILURE;
    }

    for (int sig = 1; sig < NSIG; sig++) {
      if (sigismember(&sa.sa_mask, sig) !=
          sigismember(&installed_actions[i].sa_mask, sig)) {
        return FAILURE;
      }
    }
  }
#endif

  return SUCCESS;
}

zend_module_entry module1_entry = {STANDARD_MODULE_HEADER,
                                   "ext1",
                                   NULL, /* Functions */
#ifndef PHP_WIN32
                                   PHP_MINIT(test_signals),
                                   PHP_MSHUTDOWN(test_signals),
#else
                                   NULL, /* MINIT */
                                   NULL, /* MSHUTDOWN */
#endif
                                   NULL, /* RINIT */
                                   NULL, /* RSHUTDOWN */
                                   NULL, /* MINFO */
                                   "0.1.0",
                                   STANDARD_MODULE_PROPERTIES};

zend_module_entry module2_entry = {STANDARD_MODULE_HEADER,
                                   "ext2",
                                   NULL, /* Functions */
                                   NULL, /* MINIT */
                                   NULL, /* MSHUTDOWN */
                                   NULL, /* RINIT */
                                   NULL, /* RSHUTDOWN */
                                   NULL, /* MINFO */
                                   "0.1.0",
                                   STANDARD_MODULE_PROPERTIES};
