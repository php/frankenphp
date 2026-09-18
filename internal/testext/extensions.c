#include "extension.h"
#include <php.h>
#include <zend_exceptions.h>

#include "_cgo_export.h"

#ifndef PHP_WIN32
#include <signal.h>

static struct sigaction original_sigsegv_action;
static struct sigaction installed_sigsegv_action;

static void test_sigsegv_handler(int sig, siginfo_t *info, void *context) {
  original_sigsegv_action.sa_sigaction(sig, info, context);
}

static PHP_MINIT_FUNCTION(test_signals) {
  struct sigaction sa = {0};
  sa.sa_sigaction = test_sigsegv_handler;
  sa.sa_flags = SA_SIGINFO | SA_RESTART;
  sigemptyset(&sa.sa_mask);
  sigaddset(&sa.sa_mask, SIGUSR2);
  if (sigaction(SIGSEGV, &sa, &original_sigsegv_action) != 0 ||
      sigaction(SIGSEGV, NULL, &installed_sigsegv_action) != 0) {
    return FAILURE;
  }

  return SUCCESS;
}

static PHP_MSHUTDOWN_FUNCTION(test_signals) {
  return sigaction(SIGSEGV, &original_sigsegv_action, NULL) == 0 ? SUCCESS
                                                                 : FAILURE;
}
#endif

int test_signal_handler(void) {
#ifndef PHP_WIN32
  struct sigaction sa;
  if (sigaction(SIGSEGV, NULL, &sa) != 0 ||
      sa.sa_sigaction != installed_sigsegv_action.sa_sigaction ||
      sa.sa_flags != (installed_sigsegv_action.sa_flags | SA_ONSTACK)) {
    return FAILURE;
  }

  for (int sig = 1; sig < NSIG; sig++) {
    if (sigismember(&sa.sa_mask, sig) !=
        sigismember(&installed_sigsegv_action.sa_mask, sig)) {
      return FAILURE;
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
