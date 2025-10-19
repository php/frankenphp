#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <Zend/zend_types.h>
#include <errno.h>
#include <ext/spl/spl_exceptions.h>
#include <ext/standard/head.h>
#include <inttypes.h>
#include <php.h>
#include <php_config.h>
#include <php_ini.h>
#include <php_main.h>
#include <php_output.h>
#include <php_variables.h>
#include <php_version.h>
#include <pthread.h>
#include <sapi/embed/php_embed.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#if defined(__linux__)
#include <sys/prctl.h>
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
#include <pthread_np.h>
#endif

typedef struct {
  char *script;
  int argc;
  char **argv;
  bool eval;
} cli_exec_args_t;
cli_exec_args_t *cli_args;

/* Function declaration to avoid implicit declaration error */
void register_server_variable_filtered(const char *key, char **val, size_t *val_len, zval *track_vars_array);

/*
 * CLI code is adapted from
 * https://github.com/php/php-src/blob/master/sapi/cli/php_cli.c Copyright (c)
 * The PHP Group Licensed under The PHP License Original uthors: Edin Kadribasic
 * <edink@php.net>, Marcus Boerger <helly@php.net> and Johannes Schlueter
 * <johannes@php.net> Parts based on CGI SAPI Module by Rasmus Lerdorf, Stig
 * Bakken and Zeev Suraski
 */
static void cli_register_file_handles(bool no_close) /* {{{ */
{
  php_stream *s_in, *s_out, *s_err;
  php_stream_context *sc_in = NULL, *sc_out = NULL, *sc_err = NULL;
  zend_constant ic, oc, ec;

  s_in = php_stream_open_wrapper_ex("php://stdin", "rb", 0, NULL, sc_in);
  s_out = php_stream_open_wrapper_ex("php://stdout", "wb", 0, NULL, sc_out);
  s_err = php_stream_open_wrapper_ex("php://stderr", "wb", 0, NULL, sc_err);

  if (s_in == NULL || s_out == NULL || s_err == NULL) {
    if (s_in)
      php_stream_close(s_in);
    if (s_out)
      php_stream_close(s_out);
    if (s_err)
      php_stream_close(s_err);
    return;
  }

  if (no_close) {
    s_in->flags |= PHP_STREAM_FLAG_NO_CLOSE;
    s_out->flags |= PHP_STREAM_FLAG_NO_CLOSE;
    s_err->flags |= PHP_STREAM_FLAG_NO_CLOSE;
  }

  /*s_in_process = s_in;*/

  php_stream_to_zval(s_in, &ic.value);
  php_stream_to_zval(s_out, &oc.value);
  php_stream_to_zval(s_err, &ec.value);

  ZEND_CONSTANT_SET_FLAGS(&ic, CONST_CS, 0);
  ic.name = zend_string_init_interned("STDIN", sizeof("STDIN") - 1, 0);
  zend_register_constant(&ic);

  ZEND_CONSTANT_SET_FLAGS(&oc, CONST_CS, 0);
  oc.name = zend_string_init_interned("STDOUT", sizeof("STDOUT") - 1, 0);
  zend_register_constant(&oc);

  ZEND_CONSTANT_SET_FLAGS(&ec, CONST_CS, 0);
  ec.name = zend_string_init_interned("STDERR", sizeof("STDERR") - 1, 0);
  zend_register_constant(&ec);
}
/* }}} */

static void sapi_cli_register_variables(zval *track_vars_array) /* {{{ */
{
  size_t len = strlen(cli_args->script);
  char *docroot = "";

  /*
   * In CGI mode, we consider the environment to be a part of the server
   * variables
   */
  php_import_environment_variables(track_vars_array);

  /* Build the special-case PHP_SELF variable for the CLI version */
  register_server_variable_filtered("PHP_SELF", &cli_args->script, &len,
                                    track_vars_array);
  register_server_variable_filtered("SCRIPT_NAME", &cli_args->script, &len,
                                    track_vars_array);

  /* filenames are empty for stdin */
  register_server_variable_filtered("SCRIPT_FILENAME", &cli_args->script, &len,
                                    track_vars_array);
  register_server_variable_filtered("PATH_TRANSLATED", &cli_args->script, &len,
                                    track_vars_array);

  /* just make it available */
  len = 0U;
  register_server_variable_filtered("DOCUMENT_ROOT", &docroot, &len,
                                    track_vars_array);
}
/* }}} */

void *emulate_script_cli(void *arg) {
  void *exit_status;
  cli_exec_args_t* args = arg;
  cli_args = args;
  bool eval = args->eval;

  /*
   * The SAPI name "cli" is hardcoded into too many programs... let's usurp it.
   */
  php_embed_module.name = "cli";
  php_embed_module.pretty_name = "PHP CLI embedded in FrankenPHP";
  php_embed_module.register_server_variables = sapi_cli_register_variables;

  php_embed_init(cli_args->argc, cli_args->argv);

  cli_register_file_handles(false);
  zend_first_try {
    if (eval) {
      /* evaluate the cli_args->script as literal PHP code (php-cli -r "...") */
      zend_eval_string_ex(cli_args->script, NULL, "Command line code", 1);
    } else {
      zend_file_handle file_handle;
      zend_stream_init_filename(&file_handle, cli_args->script);

      CG(skip_shebang) = 1;
      php_execute_script(&file_handle);
    }
  }
  zend_end_try();

  exit_status = (void *)(intptr_t)EG(exit_status);

  php_embed_shutdown();

  return exit_status;
}
