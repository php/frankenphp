#include <pthread.h>
#include <signal.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern char *go_frankenphp_cli_init(void);
extern void go_frankenphp_caddy_main(void);
extern int frankenphp_execute_script_cli_native(int argc, char **argv);

typedef void (*go_initializer)(int, char **, char **);
extern go_initializer __frankenphp_go_init_start[];
extern go_initializer __frankenphp_go_init_end[];

static sigset_t original_mask;

/* Run before the Go c-archive constructor. In library mode Go honors these
 * inherited masks on its own threads, including SIGTERM and SIGINT. */
__attribute__((constructor(101))) static void prepare_signals(void) {
  sigset_t signals;
  sigemptyset(&signals);
  sigaddset(&signals, SIGHUP);
  sigaddset(&signals, SIGINT);
  sigaddset(&signals, SIGQUIT);
  sigaddset(&signals, SIGTERM);
  sigaddset(&signals, SIGUSR1);
  sigaddset(&signals, SIGUSR2);
  sigaddset(&signals, SIGALRM);
  if (pthread_sigmask(SIG_BLOCK, &signals, &original_mask) != 0) {
    static const char message[] = "frankenphp: cannot initialize signal mask\n";
    (void)write(STDERR_FILENO, message, sizeof(message) - 1);
    _exit(1);
  }
}

int main(int argc, char **argv, char **envp) {
  /* musl does not pass argc/argv to ELF constructors, but Go requires them.
   * The linker keeps Go's initializer out of the automatic constructor list.
   * All other C/C++ constructors have already run when we enter main. */
  for (go_initializer *init = __frankenphp_go_init_start;
       init < __frankenphp_go_init_end; init++) {
    (*init)(argc, argv, envp);
  }

  if (argc > 1 && strcmp(argv[1], "php-cli") == 0) {
    /* This cgo call waits for all Go package initializers, preserving extension
     * registration and embedded-app extraction without entering Caddy's CLI. */
    char *script = go_frankenphp_cli_init();
    char **php_argv = malloc((size_t)argc * sizeof(*php_argv));
    if (php_argv == NULL) {
      free(script);
      return 1;
    }
    php_argv[0] = argv[0];
    for (int i = 2; i < argc; i++) {
      php_argv[i - 1] = argv[i];
    }
    php_argv[argc - 1] = NULL;
    if (script != NULL) {
      php_argv[1] = script;
    }
    if (pthread_sigmask(SIG_SETMASK, &original_mask, NULL) != 0) {
      free(php_argv);
      free(script);
      return 1;
    }
    int status = frankenphp_execute_script_cli_native(argc - 1, php_argv);
    free(php_argv);
    free(script);
    return status;
  }

  if (pthread_sigmask(SIG_SETMASK, &original_mask, NULL) != 0) {
    return 1;
  }
  go_frankenphp_caddy_main();
  return 0;
}
