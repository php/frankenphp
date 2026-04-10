#include "frankenphp.h"
#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_enum.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <errno.h>
#include <ext/spl/spl_exceptions.h>
#include <ext/standard/head.h>
#ifdef HAVE_PHP_SESSION
#include <ext/session/php_session.h>
#endif
#include <inttypes.h>
#include <php.h>
#ifdef PHP_WIN32
#include <config.w32.h>
#include <io.h>
#else
#include <php_config.h>
#endif
#include <php_ini.h>
#include <php_main.h>
#include <php_output.h>
#include <php_variables.h>
#include <pthread.h>
#include <sapi/embed/php_embed.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#ifndef ZEND_WIN32
#include <unistd.h>
#endif
#if defined(__linux__)
#include <sys/prctl.h>
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
#include <pthread_np.h>
#endif

#include "_cgo_export.h"
#include "frankenphp_arginfo.h"

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
static const char *MODULES_TO_RELOAD[] = {"filter", NULL};

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

__thread uintptr_t thread_index;
__thread bool is_worker_thread = false;
__thread char *worker_name = NULL;
__thread bool is_background_worker = false;
__thread int worker_stop_fds[2] = {-1, -1};
__thread php_stream *worker_signaling_stream = NULL;
__thread zval last_set_vars_zval; /* IS_UNDEF when unset (TLS zero-init) */
__thread HashTable *sandboxed_env = NULL;

/* Best-effort force-kill for stuck background workers after grace period.
 * - Linux ZTS: arm PHP's per-thread timer -> "max execution time" fatal
 * - Windows: CancelSynchronousIo + QueueUserAPC -> interrupts I/O and sleeps
 * - macOS/other: no-op (threads abandoned, exit when blocking call returns) */
static int force_kill_num_threads = 0;
#ifdef ZEND_MAX_EXECUTION_TIMERS
static timer_t *thread_php_timers = NULL;
static bool *thread_php_timer_saved = NULL;
#elif defined(PHP_WIN32)
static HANDLE *thread_handles = NULL;
static bool *thread_handle_saved = NULL;
static void CALLBACK frankenphp_noop_apc(ULONG_PTR param) { (void)param; }
#endif

void frankenphp_init_force_kill(int num_threads) {
  force_kill_num_threads = num_threads;
#ifdef ZEND_MAX_EXECUTION_TIMERS
  thread_php_timers = calloc(num_threads, sizeof(timer_t));
  thread_php_timer_saved = calloc(num_threads, sizeof(bool));
#elif defined(PHP_WIN32)
  thread_handles = calloc(num_threads, sizeof(HANDLE));
  thread_handle_saved = calloc(num_threads, sizeof(bool));
#endif
}

void frankenphp_save_php_timer(uintptr_t idx) {
  if (idx >= (uintptr_t)force_kill_num_threads) {
    return;
  }
#ifdef ZEND_MAX_EXECUTION_TIMERS
  if (thread_php_timers && EG(pid)) {
    thread_php_timers[idx] = EG(max_execution_timer_timer);
    thread_php_timer_saved[idx] = true;
  }
#elif defined(PHP_WIN32)
  if (thread_handles) {
    DuplicateHandle(GetCurrentProcess(), GetCurrentThread(),
                    GetCurrentProcess(), &thread_handles[idx], 0, FALSE,
                    DUPLICATE_SAME_ACCESS);
    thread_handle_saved[idx] = true;
  }
#endif
  (void)idx;
}

void frankenphp_force_kill_thread(uintptr_t idx) {
  if (idx >= (uintptr_t)force_kill_num_threads) {
    return;
  }
#ifdef ZEND_MAX_EXECUTION_TIMERS
  if (thread_php_timers && thread_php_timer_saved[idx]) {
    struct itimerspec its;
    its.it_value.tv_sec = 0;
    its.it_value.tv_nsec = 1;
    its.it_interval.tv_sec = 0;
    its.it_interval.tv_nsec = 0;
    timer_settime(thread_php_timers[idx], 0, &its, NULL);
  }
#elif defined(PHP_WIN32)
  if (thread_handles && thread_handle_saved[idx]) {
    CancelSynchronousIo(thread_handles[idx]);
    QueueUserAPC((PAPCFUNC)frankenphp_noop_apc, thread_handles[idx], 0);
  }
#endif
  (void)idx;
}

void frankenphp_destroy_force_kill(void) {
#ifdef ZEND_MAX_EXECUTION_TIMERS
  if (thread_php_timers) {
    free(thread_php_timers);
    thread_php_timers = NULL;
  }
  if (thread_php_timer_saved) {
    free(thread_php_timer_saved);
    thread_php_timer_saved = NULL;
  }
#elif defined(PHP_WIN32)
  if (thread_handles) {
    for (int i = 0; i < force_kill_num_threads; i++) {
      if (thread_handle_saved && thread_handle_saved[i]) {
        CloseHandle(thread_handles[i]);
      }
    }
    free(thread_handles);
    thread_handles = NULL;
  }
  if (thread_handle_saved) {
    free(thread_handle_saved);
    thread_handle_saved = NULL;
  }
#endif
  force_kill_num_threads = 0;
}

/* Per-thread cache for get_vars results.
 * Maps worker name (string) -> {version, cached_zval}.
 * When the version matches, the cached zval is returned with a refcount bump,
 * giving the same HashTable pointer -> === comparisons are O(1). */
typedef struct {
  uint64_t version;
  zval value;
} bg_worker_vars_cache_entry;
__thread HashTable *worker_vars_cache = NULL;

static void frankenphp_worker_close_stop_fds(void) {
  if (worker_stop_fds[0] >= 0) {
#ifdef PHP_WIN32
    _close(worker_stop_fds[0]);
#else
    close(worker_stop_fds[0]);
#endif
    worker_stop_fds[0] = -1;
  }

  if (worker_stop_fds[1] >= 0) {
#ifdef PHP_WIN32
    _close(worker_stop_fds[1]);
#else
    close(worker_stop_fds[1]);
#endif
    worker_stop_fds[1] = -1;
  }
}

static int frankenphp_worker_open_stop_pipe(void) {
#ifdef PHP_WIN32
  return _pipe(worker_stop_fds, 4096, _O_BINARY);
#else
  return pipe(worker_stop_fds);
#endif
}

static int frankenphp_worker_dup_fd(int fd) {
#ifdef PHP_WIN32
  return _dup(fd);
#else
  return dup(fd);
#endif
}

void frankenphp_update_local_thread_context(bool is_worker) {
  is_worker_thread = is_worker;

  /* workers should keep running if the user aborts the connection */
  PG(ignore_user_abort) = is_worker ? 1 : original_user_abort_setting;
}

static void bg_worker_vars_cache_dtor(zval *zv) {
  bg_worker_vars_cache_entry *entry = Z_PTR_P(zv);
  zval_ptr_dtor(&entry->value);
  free(entry);
}

static void bg_worker_vars_cache_reset(void) {
  if (worker_vars_cache) {
    zend_hash_destroy(worker_vars_cache);
    free(worker_vars_cache);
    worker_vars_cache = NULL;
  }
}

void frankenphp_set_worker_name(char *name, bool background) {
  free(worker_name);
  if (name) {
    size_t len = strlen(name) + 1;
    worker_name = malloc(len);
    memcpy(worker_name, name, len);
  } else {
    worker_name = NULL;
  }
  is_background_worker = background;
  if (!background) {
    return;
  }
  worker_signaling_stream = NULL;
  if (Z_TYPE(last_set_vars_zval) != IS_UNDEF) {
    zval_ptr_dtor(&last_set_vars_zval);
    ZVAL_UNDEF(&last_set_vars_zval);
  }
  zend_unset_timeout();

  /* Create a pipe for stop signaling */
  frankenphp_worker_close_stop_fds();
  if (frankenphp_worker_open_stop_pipe() != 0) {
    worker_stop_fds[0] = -1;
    worker_stop_fds[1] = -1;
  }
}

int frankenphp_worker_get_stop_fd_write(void) { return worker_stop_fds[1]; }

static int bg_worker_pipe_write_impl(int fd, const char *buf, int len) {
  if (fd < 0) {
    return -1;
  }

#ifdef PHP_WIN32
  return _write(fd, buf, len);
#else
  return (int)write(fd, buf, len);
#endif
}

int frankenphp_worker_write_stop_fd(int fd) {
  return bg_worker_pipe_write_impl(fd, "stop\n", 5);
}

int frankenphp_worker_write_task_signal(int fd) {
  return bg_worker_pipe_write_impl(fd, "task\n", 5);
}

int frankenphp_worker_pipe_nudge(int fd) {
  return bg_worker_pipe_write_impl(fd, "\n", 1);
}

int frankenphp_worker_create_pipe(int *fds) {
#ifdef PHP_WIN32
  return _pipe(fds, 4096, _O_BINARY);
#else
  return pipe(fds);
#endif
}

void frankenphp_worker_close_fd(int fd) {
  if (fd >= 0) {
#ifdef PHP_WIN32
    _close(fd);
#else
    close(fd);
#endif
  }
}

static void frankenphp_update_request_context() {
  /* the server context is stored on the go side, still SG(server_context) needs
   * to not be NULL */
  SG(server_context) = (void *)1;
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

    /* $_SESSION must be explicitly deleted from the symbol table.
     * Unlike other superglobals, $_SESSION is stored in EG(symbol_table)
     * with a reference to PS(http_session_vars). The session RSHUTDOWN
     * only decrements the refcount but doesn't remove it from the symbol
     * table, causing data to leak between requests. */
    zend_hash_str_del(&EG(symbol_table), "_SESSION", sizeof("_SESSION") - 1);
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

/*
 * free php_stream resources that are temporary (php_stream_temp_ops)
 * streams are globally registered in EG(regular_list), see zend_list.c
 * this fixes a leak when reading the body of a request
 */
static void frankenphp_release_temporary_streams() {
  zend_resource *val;
  int stream_type = php_file_le_stream();
  ZEND_HASH_FOREACH_PTR(&EG(regular_list), val) {
    /* verify the resource is a stream */
    if (val->type == stream_type) {
      php_stream *stream = (php_stream *)val->ptr;
      if (stream != NULL && stream->ops == &php_stream_temp_ops &&
          stream->__exposed == 0 && GC_REFCOUNT(val) == 1) {
        ZEND_ASSERT(!stream->is_persistent);
        zend_list_delete(val);
      }
    }
  }
  ZEND_HASH_FOREACH_END();
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

  /* SAPI related shutdown (free stuff) */
  zend_try { sapi_deactivate(); }
  zend_end_try();
  frankenphp_free_request_context();

  zend_set_memory_limit(PG(memory_limit));
}

// shutdown the dummy request that starts the worker script
bool frankenphp_shutdown_dummy_request(void) {
  if (SG(server_context) == NULL) {
    return false;
  }

  frankenphp_worker_request_shutdown();

  return true;
}

void get_full_env(zval *track_vars_array) {
  zend_hash_extend(Z_ARR_P(track_vars_array),
                   zend_hash_num_elements(main_thread_env), 0);
  zend_hash_copy(Z_ARR_P(track_vars_array), main_thread_env, NULL);
}

/* Adapted from php_request_startup() */
static int frankenphp_worker_request_startup() {
  int retval = SUCCESS;

  frankenphp_update_request_context();

  zend_try {
    frankenphp_release_temporary_streams();
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

    if (PG(output_handler) && PG(output_handler)[0]) {
      zval oh;

      ZVAL_STRING(&oh, PG(output_handler));
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

  SG(sapi_started) = 1;

  return retval;
}

PHP_FUNCTION(frankenphp_finish_request) { /* {{{ */
  ZEND_PARSE_PARAMETERS_NONE();

  if (go_is_context_done(thread_index)) {
    RETURN_FALSE;
  }

  php_output_end_all();
  php_header();

  go_frankenphp_finish_php_request(thread_index);

  RETURN_TRUE;
} /* }}} */

/* {{{ Call go's putenv to prevent race conditions */
PHP_FUNCTION(frankenphp_putenv) {
  char *setting;
  size_t setting_len;

  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_STRING(setting, setting_len)
  ZEND_PARSE_PARAMETERS_END();

  // Cast str_len to int (ensure it fits in an int)
  if (setting_len > INT_MAX) {
    php_error(E_WARNING, "String length exceeds maximum integer value");
    RETURN_FALSE;
  }

  if (setting_len == 0 || setting[0] == '=') {
    zend_argument_value_error(1, "must have a valid syntax");
    RETURN_THROWS();
  }

  if (sandboxed_env == NULL) {
    sandboxed_env = zend_array_dup(main_thread_env);
  }

  /* cut at null byte to stay consistent with regular putenv */
  char *null_pos = memchr(setting, '\0', setting_len);
  if (null_pos != NULL) {
    setting_len = null_pos - setting;
  }

  /* cut the string at the first '=' */
  char *eq_pos = memchr(setting, '=', setting_len);
  bool success = true;

  /* no '=' found, delete the variable */
  if (eq_pos == NULL) {
    success = go_putenv(setting, (int)setting_len, NULL, 0);
    if (success) {
      zend_hash_str_del(sandboxed_env, setting, setting_len);
    }

    RETURN_BOOL(success);
  }

  size_t name_len = eq_pos - setting;
  size_t value_len =
      (setting_len > name_len + 1) ? (setting_len - name_len - 1) : 0;
  success = go_putenv(setting, (int)name_len, eq_pos + 1, (int)value_len);
  if (success) {
    zval val = {0};
    ZVAL_STRINGL(&val, eq_pos + 1, value_len);
    zend_hash_str_update(sandboxed_env, setting, name_len, &val);
  }

  RETURN_BOOL(success);
} /* }}} */

/* {{{ Get the env from the sandboxed environment */
PHP_FUNCTION(frankenphp_getenv) {
  zend_string *name = NULL;
  bool local_only = 0;

  ZEND_PARSE_PARAMETERS_START(0, 2)
  Z_PARAM_OPTIONAL
  Z_PARAM_STR_OR_NULL(name)
  Z_PARAM_BOOL(local_only)
  ZEND_PARSE_PARAMETERS_END();

  HashTable *ht = sandboxed_env ? sandboxed_env : main_thread_env;

  if (!name) {
    RETURN_ARR(zend_array_dup(ht));
    return;
  }

  zval *env_val = zend_hash_find(ht, name);
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
      go_apache_request_headers(thread_index);

  array_init_size(return_value, headers.r1);

  for (size_t i = 0; i < headers.r1; i++) {
    go_string key = headers.r0[i * 2];
    go_string val = headers.r0[i * 2 + 1];

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

  if (!is_worker_thread || is_background_worker) {
    zend_throw_exception(
        spl_ce_RuntimeException,
        is_background_worker
            ? "frankenphp_handle_request() cannot be called from a background "
              "worker"
            : "frankenphp_handle_request() called while not in worker mode",
        0);
    RETURN_THROWS();
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /* Disable timeouts while waiting for a request to handle */
  zend_unset_timeout();
#endif

  struct go_frankenphp_worker_handle_request_start_return result =
      go_frankenphp_worker_handle_request_start(thread_index);
  if (frankenphp_worker_request_startup() == FAILURE
      /* Shutting down */
      || !result.r0) {
    RETURN_FALSE;
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /*
   * Reset default timeout
   */
  if (PG(max_input_time) != -1) {
    zend_set_timeout(INI_INT("max_execution_time"), 0);
  }
#endif

  /* Call the PHP func passed to frankenphp_handle_request() */
  zval retval = {0};
  zval *callback_ret = NULL;

  fci.size = sizeof fci;
  fci.retval = &retval;
  fci.params = result.r1;
  fci.param_count = result.r1 == NULL ? 0 : 1;

  if (zend_call_function(&fci, &fcc) == SUCCESS && Z_TYPE(retval) != IS_UNDEF) {
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

  bg_worker_vars_cache_reset();
  frankenphp_worker_request_shutdown();
  go_frankenphp_finish_worker_request(thread_index, callback_ret);
  if (result.r1 != NULL) {
    zval_ptr_dtor(result.r1);
  }
  if (callback_ret != NULL) {
    zval_ptr_dtor(&retval);
  }

  RETURN_TRUE;
}

/* Persistent zval helpers: validation, deep-copy, immutable detection, enums */
#include "bg_worker_vars.h"

/* Go-callable wrapper to free a persistent HashTable (used by task cleanup) */
void frankenphp_worker_free_persistent_ht(void *ptr) {
  bg_worker_free_stored_vars(ptr);
}

PHP_FUNCTION(frankenphp_worker_set_vars) {
  zval *vars_array = NULL;

  ZEND_PARSE_PARAMETERS_START(1, 1);
  Z_PARAM_ARRAY(vars_array);
  ZEND_PARSE_PARAMETERS_END();

  /* Skip if the new value is identical to the last one.
   * Always update the cache so the next comparison can use pointer equality. */
  if (Z_TYPE(last_set_vars_zval) != IS_UNDEF &&
      zend_is_identical(vars_array, &last_set_vars_zval)) {
    zval_ptr_dtor(&last_set_vars_zval);
    ZVAL_COPY(&last_set_vars_zval, vars_array);
    return;
  }

  HashTable *ht = Z_ARRVAL_P(vars_array);

  if (bg_worker_is_immutable(ht)) {
    /* Fast path: immutable arrays are already in shared memory.
     * No validation needed (immutable arrays contain only safe types).
     * No deep-copy needed. */
    void *old = NULL;
    char *error = go_frankenphp_worker_set_vars(thread_index, ht, &old);
    if (error) {
      zend_throw_exception(spl_ce_RuntimeException, error, 0);
      free(error);
      RETURN_THROWS();
    }
    bg_worker_free_stored_vars(old);
  } else {
    /* Slow path: validate, deep-copy to persistent memory */
    zval *val;
    ZEND_HASH_FOREACH_VAL(ht, val) {
      if (!bg_worker_validate_zval(val)) {
        zend_value_error("Values must be null, scalars, arrays, or enums; "
                         "objects and resources are not allowed");
        RETURN_THROWS();
      }
    }
    ZEND_HASH_FOREACH_END();

    zval persistent;
    bg_worker_persist_zval(&persistent, vars_array);

    void *old = NULL;
    char *error =
        go_frankenphp_worker_set_vars(thread_index, Z_ARRVAL(persistent), &old);
    if (error) {
      bg_worker_free_persistent_zval(&persistent);
      zend_throw_exception(spl_ce_RuntimeException, error, 0);
      free(error);
      RETURN_THROWS();
    }
    bg_worker_free_stored_vars(old);
  }

  /* Cache the new value for next comparison */
  if (Z_TYPE(last_set_vars_zval) != IS_UNDEF) {
    zval_ptr_dtor(&last_set_vars_zval);
  }
  ZVAL_COPY(&last_set_vars_zval, vars_array);
}

/* Copy vars from persistent storage into a PHP zval.
 * For count == 1: copies directly into dst.
 * For count > 1: creates a keyed array in dst.
 * Returns true on success, false if an exception occurred. */
bool frankenphp_worker_copy_vars(zval *dst, int count, char **names,
                                 size_t *name_lens, void **ptrs) {
  if (count == 1) {
    if (ptrs[0]) {
      bg_worker_read_stored_vars(dst, ptrs[0]);
    } else {
      array_init(dst);
    }
    return !EG(exception);
  }

  array_init(dst);
  for (int i = 0; i < count; i++) {
    zval worker_vars;
    if (ptrs[i]) {
      bg_worker_read_stored_vars(&worker_vars, ptrs[i]);
      if (EG(exception)) {
        zval_ptr_dtor(&worker_vars);
        return false;
      }
    } else {
      array_init(&worker_vars);
    }
    add_assoc_zval_ex(dst, names[i], name_lens[i], &worker_vars);
  }
  return true;
}

PHP_FUNCTION(frankenphp_worker_get_vars) {
  zval *names = NULL;
  double timeout = 30.0;

  ZEND_PARSE_PARAMETERS_START(1, 2);
  Z_PARAM_ZVAL(names);
  Z_PARAM_OPTIONAL
  Z_PARAM_DOUBLE(timeout);
  ZEND_PARSE_PARAMETERS_END();

  if (timeout < 0) {
    zend_value_error("Timeout must not be negative");
    RETURN_THROWS();
  }
  int timeout_ms = (int)(timeout * 1000);

  if (Z_TYPE_P(names) == IS_STRING) {
    if (Z_STRLEN_P(names) == 0) {
      zend_value_error("Background worker name must not be empty");
      RETURN_THROWS();
    }

    char *name_ptr = Z_STRVAL_P(names);
    size_t name_len_val = Z_STRLEN_P(names);

    /* Check per-request cache */
    uint64_t caller_version = 0;
    uint64_t out_version = 0;
    bg_worker_vars_cache_entry *cached = NULL;
    if (worker_vars_cache) {
      zval *entry_zv =
          zend_hash_str_find(worker_vars_cache, name_ptr, name_len_val);
      if (entry_zv) {
        cached = Z_PTR_P(entry_zv);
        caller_version = cached->version;
      }
    }

    char *error = go_frankenphp_worker_get_vars(
        thread_index, &name_ptr, &name_len_val, 1, timeout_ms, return_value,
        cached ? &caller_version : NULL, &out_version);
    if (error) {
      zend_throw_exception(spl_ce_RuntimeException, error, 0);
      free(error);
      RETURN_THROWS();
    }
    if (EG(exception)) {
      RETURN_THROWS();
    }

    /* Cache hit: Go skipped the copy because version matched */
    if (cached && out_version == caller_version) {
      ZVAL_COPY(return_value, &cached->value);
      return;
    }

    /* Cache miss: store the new result */
    if (!worker_vars_cache) {
      worker_vars_cache = malloc(sizeof(HashTable));
      zend_hash_init(worker_vars_cache, 4, NULL, bg_worker_vars_cache_dtor, 1);
    }
    bg_worker_vars_cache_entry *entry = malloc(sizeof(*entry));
    entry->version = out_version;
    ZVAL_COPY(&entry->value, return_value);
    zval entry_zv;
    ZVAL_PTR(&entry_zv, entry);
    zend_hash_str_update(worker_vars_cache, name_ptr, name_len_val, &entry_zv);

    return;
  }

  if (Z_TYPE_P(names) != IS_ARRAY) {
    zend_type_error("Argument #1 ($name) must be of type string|array, %s "
                    "given",
                    zend_zval_type_name(names));
    RETURN_THROWS();
  }

  HashTable *ht = Z_ARRVAL_P(names);
  zval *val;

  ZEND_HASH_FOREACH_VAL(ht, val) {
    if (Z_TYPE_P(val) != IS_STRING || Z_STRLEN_P(val) == 0) {
      zend_value_error("All background worker names must be non-empty strings");
      RETURN_THROWS();
    }
  }
  ZEND_HASH_FOREACH_END();

  int name_count = zend_hash_num_elements(ht);
  char **name_ptrs = malloc(sizeof(char *) * name_count);
  size_t *name_lens_arr = malloc(sizeof(size_t) * name_count);
  int idx = 0;
  ZEND_HASH_FOREACH_VAL(ht, val) {
    name_ptrs[idx] = Z_STRVAL_P(val);
    name_lens_arr[idx] = Z_STRLEN_P(val);
    idx++;
  }
  ZEND_HASH_FOREACH_END();

  char *error = go_frankenphp_worker_get_vars(
      thread_index, name_ptrs, name_lens_arr, name_count, timeout_ms,
      return_value, NULL, NULL);
  free(name_ptrs);
  free(name_lens_arr);
  if (error) {
    zend_throw_exception(spl_ce_RuntimeException, error, 0);
    free(error);
    RETURN_THROWS();
  }
}

PHP_FUNCTION(frankenphp_worker_get_signaling_stream) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!is_background_worker) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "frankenphp_worker_get_signaling_stream() can only "
                         "be called from a background worker",
                         0);
    RETURN_THROWS();
  }

  /* Return the cached stream if already created */
  if (worker_signaling_stream != NULL) {
    php_stream_to_zval(worker_signaling_stream, return_value);
    GC_ADDREF(Z_COUNTED_P(return_value));
    return;
  }

  if (worker_stop_fds[0] < 0) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to create background worker stop pipe", 0);
    RETURN_THROWS();
  }

  int fd = frankenphp_worker_dup_fd(worker_stop_fds[0]);
  if (fd < 0) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to dup background worker stop fd", 0);
    RETURN_THROWS();
  }

  php_stream *stream = php_stream_fopen_from_fd(fd, "rb", NULL);
  if (!stream) {
    frankenphp_worker_close_fd(fd);
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to create stream from stop fd", 0);
    RETURN_THROWS();
  }

  worker_signaling_stream = stream;
  php_stream_to_zval(stream, return_value);

  /* Keep an extra ref so PHP can't destroy the stream while TLS caches it */
  GC_ADDREF(Z_COUNTED_P(return_value));
}

/* Custom stream ops for background worker task (sender/read side) */
typedef struct {
  int task_id;
  int read_fd;
} bg_worker_task_sender_data;

static ssize_t bg_worker_task_sender_write(php_stream *stream, const char *buf,
                                           size_t count) {
  return -1; /* read-only stream */
}

static ssize_t bg_worker_task_sender_read(php_stream *stream, char *buf,
                                          size_t count) {
  bg_worker_task_sender_data *data =
      (bg_worker_task_sender_data *)stream->abstract;
  if (!data || data->read_fd < 0)
    return -1;
#ifdef PHP_WIN32
  return _read(data->read_fd, buf, (unsigned int)count);
#else
  return read(data->read_fd, buf, count);
#endif
}

static int bg_worker_task_sender_close(php_stream *stream, int close_handle) {
  bg_worker_task_sender_data *data =
      (bg_worker_task_sender_data *)stream->abstract;
  if (data) {
    go_frankenphp_worker_task_cancel(data->task_id);
    frankenphp_worker_close_fd(data->read_fd);
    free(data);
    stream->abstract = NULL;
  }
  return 0;
}

static int bg_worker_task_sender_cast(php_stream *stream, int castas,
                                      void **ret) {
  bg_worker_task_sender_data *data =
      (bg_worker_task_sender_data *)stream->abstract;
  if (!data)
    return FAILURE;

  switch (castas) {
  case PHP_STREAM_AS_FD:
  case PHP_STREAM_AS_SOCKETD:
    if (ret)
      *(int *)ret = data->read_fd;
    return SUCCESS;
  default:
    return FAILURE;
  }
}

static const php_stream_ops bg_worker_task_sender_ops = {
    bg_worker_task_sender_write, /* write */
    bg_worker_task_sender_read,  /* read */
    bg_worker_task_sender_close, /* close */
    NULL,                        /* flush */
    "bg_worker_task_sender",     /* label */
    NULL,                        /* seek */
    bg_worker_task_sender_cast,  /* cast */
    NULL,                        /* stat */
    NULL,                        /* set_option */
};

PHP_FUNCTION(frankenphp_worker_task_send) {
  char *name = NULL;
  size_t name_len = 0;
  zval *payload = NULL;
  double timeout = 30.0;

  ZEND_PARSE_PARAMETERS_START(2, 3);
  Z_PARAM_STRING(name, name_len);
  Z_PARAM_ARRAY(payload);
  Z_PARAM_OPTIONAL;
  Z_PARAM_DOUBLE(timeout);
  ZEND_PARSE_PARAMETERS_END();

  if (name_len == 0) {
    zend_value_error("Background worker name must not be empty");
    RETURN_THROWS();
  }

  HashTable *payload_ht;
  bool payload_immutable = bg_worker_is_immutable(Z_ARRVAL_P(payload));

  if (payload_immutable) {
    payload_ht = Z_ARRVAL_P(payload);
  } else {
    zval *val;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(payload), val) {
      if (!bg_worker_validate_zval(val)) {
        zend_value_error("Task payload values must be null, scalars, arrays, "
                         "or enums; objects and resources are not allowed");
        RETURN_THROWS();
      }
    }
    ZEND_HASH_FOREACH_END();

    zval persistent_payload;
    bg_worker_persist_zval(&persistent_payload, payload);
    payload_ht = Z_ARRVAL(persistent_payload);
  }

  int task_id = -1;
  int read_fd = -1;
  int timeout_ms = (int)(timeout * 1000);
  if (timeout_ms < 0) {
    timeout_ms = 0;
  }
  char *error = go_frankenphp_worker_task_send(
      thread_index, name, name_len, payload_ht, timeout_ms, &task_id, &read_fd);
  if (error) {
    if (!payload_immutable) {
      zval z;
      ZVAL_ARR(&z, payload_ht);
      bg_worker_free_persistent_zval(&z);
    }
    zend_throw_exception(spl_ce_RuntimeException, error, 0);
    free(error);
    RETURN_THROWS();
  }

  /* Create a custom sender stream */
  bg_worker_task_sender_data *sdata =
      malloc(sizeof(bg_worker_task_sender_data));
  sdata->task_id = task_id;
  sdata->read_fd = read_fd;

  php_stream *stream =
      php_stream_alloc(&bg_worker_task_sender_ops, sdata, NULL, "rb");
  if (!stream) {
    free(sdata);
    go_frankenphp_worker_task_cancel(task_id);
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to create task stream", 0);
    RETURN_THROWS();
  }

  zval task_id_zval;
  ZVAL_LONG(&task_id_zval, task_id);
  stream->wrapperdata = task_id_zval;

  php_stream_to_zval(stream, return_value);
}

PHP_FUNCTION(frankenphp_worker_task_read) {
  zval *stream_zval = NULL;

  ZEND_PARSE_PARAMETERS_START(1, 1);
  Z_PARAM_RESOURCE(stream_zval);
  ZEND_PARSE_PARAMETERS_END();

  php_stream *stream = NULL;
  php_stream_from_zval_no_verify(stream, stream_zval);
  if (!stream || stream->ops != &bg_worker_task_sender_ops) {
    zend_value_error(
        "Argument #1 must be a task stream from frankenphp_worker_task_send()");
    RETURN_THROWS();
  }

  if (!stream->abstract) {
    RETURN_NULL(); /* stream already closed */
  }

  int task_id = (int)Z_LVAL(stream->wrapperdata);

  /* Consume the signal byte from the pipe */
  char buf[1];
  if (php_stream_read(stream, buf, 1) <= 0) {
    /* Pipe EOF - drain remaining FIFO items */
    int eof_result = go_frankenphp_worker_task_read(task_id, NULL);
    if (eof_result == -2) {
      zend_throw_exception(spl_ce_RuntimeException,
                           "background worker exited without completing the task",
                           0);
      RETURN_THROWS();
    }
    RETURN_NULL();
  }

  void *data = NULL;
  int result = go_frankenphp_worker_task_read(task_id, &data);
  if (result == -2) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "background worker exited without completing the task", 0);
    RETURN_THROWS();
  }
  if (result < 0 || data == NULL) {
    RETURN_NULL();
  }

  HashTable *data_ht = (HashTable *)data;
  if (bg_worker_is_immutable(data_ht)) {
    ZVAL_ARR(return_value, data_ht);
  } else {
    zval src;
    ZVAL_ARR(&src, data_ht);
    bg_worker_move_zval(return_value, &src);
  }
}

/* Custom stream ops for background worker task (receiver/write side) */
typedef struct {
  int task_id;
  int write_fd;
} bg_worker_task_stream_data;

static ssize_t bg_worker_task_write(php_stream *stream, const char *buf,
                                    size_t count) {
  return -1; /* writing is done via task_update, not stream write */
}

static ssize_t bg_worker_task_read(php_stream *stream, char *buf,
                                   size_t count) {
  return -1; /* write-only stream */
}

static int bg_worker_task_close(php_stream *stream, int close_handle) {
  bg_worker_task_stream_data *data =
      (bg_worker_task_stream_data *)stream->abstract;
  if (data) {
    /* During explicit fclose(): clean close. During request shutdown
     * (exit/crash): skip - afterScriptExecution handles cleanup. */
    if (!(EG(flags) & EG_FLAGS_IN_SHUTDOWN)) {
      go_frankenphp_worker_task_close(thread_index, data->task_id);
    }
    free(data);
    stream->abstract = NULL;
  }
  return 0;
}

static int bg_worker_task_cast(php_stream *stream, int castas, void **ret) {
  bg_worker_task_stream_data *data =
      (bg_worker_task_stream_data *)stream->abstract;
  if (!data)
    return FAILURE;

  switch (castas) {
  case PHP_STREAM_AS_FD:
  case PHP_STREAM_AS_SOCKETD:
    if (ret)
      *(int *)ret = data->write_fd;
    return SUCCESS;
  default:
    return FAILURE;
  }
}

static const php_stream_ops bg_worker_task_stream_ops = {
    bg_worker_task_write, /* write */
    bg_worker_task_read,  /* read */
    bg_worker_task_close, /* close */
    NULL,                 /* flush */
    "bg_worker_task",     /* label */
    NULL,                 /* seek */
    bg_worker_task_cast,  /* cast */
    NULL,                 /* stat */
    NULL,                 /* set_option */
};

PHP_FUNCTION(frankenphp_worker_task_receive) {
  ZEND_PARSE_PARAMETERS_NONE();

  if (!is_background_worker) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "frankenphp_worker_task_receive() can only be called "
                         "from a background worker",
                         0);
    RETURN_THROWS();
  }

  void *payload = NULL;
  int task_id = -1;
  int result =
      go_frankenphp_worker_task_receive(thread_index, &payload, &task_id);
  if (result < 0) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "frankenphp_worker_task_receive() can only be called "
                         "from a background worker",
                         0);
    RETURN_THROWS();
  }
  if (result == 0) {
    RETURN_NULL();
  }

  /* Create a custom task stream (write side) */
  bg_worker_task_stream_data *sdata =
      malloc(sizeof(bg_worker_task_stream_data));
  sdata->task_id = task_id;
  sdata->write_fd = -1; /* set by Go via pipe */

  php_stream *stream =
      php_stream_alloc(&bg_worker_task_stream_ops, sdata, NULL, "wb");
  if (!stream) {
    free(sdata);
    go_frankenphp_worker_task_close(thread_index, task_id);
    zend_throw_exception(spl_ce_RuntimeException,
                         "failed to create task stream", 0);
    RETURN_THROWS();
  }

  /* Store task_id in wrapperdata for task_update */
  zval task_id_zval;
  ZVAL_LONG(&task_id_zval, task_id);
  stream->wrapperdata = task_id_zval;

  /* Return [stream, payload] */
  array_init(return_value);

  zval stream_zval;
  php_stream_to_zval(stream, &stream_zval);
  add_next_index_zval(return_value, &stream_zval);

  zval payload_zval;
  if (payload) {
    HashTable *pht = (HashTable *)payload;
    if (bg_worker_is_immutable(pht)) {
      ZVAL_ARR(&payload_zval, pht);
    } else {
      zval src;
      ZVAL_ARR(&src, pht);
      bg_worker_move_zval(&payload_zval, &src);
    }
  } else {
    array_init(&payload_zval);
  }
  add_next_index_zval(return_value, &payload_zval);
}

PHP_FUNCTION(frankenphp_worker_task_update) {
  zval *stream_zval = NULL;
  zval *data = NULL;

  ZEND_PARSE_PARAMETERS_START(2, 2);
  Z_PARAM_RESOURCE(stream_zval);
  Z_PARAM_ARRAY(data);
  ZEND_PARSE_PARAMETERS_END();

  if (!is_background_worker) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "frankenphp_worker_task_update() can only be called "
                         "from a background worker",
                         0);
    RETURN_THROWS();
  }

  php_stream *stream = NULL;
  php_stream_from_zval_no_verify(stream, stream_zval);
  if (!stream || stream->ops != &bg_worker_task_stream_ops) {
    zend_value_error("Argument #1 must be a task stream from "
                     "frankenphp_worker_task_receive()");
    RETURN_THROWS();
  }

  if (!stream->abstract) {
    zend_throw_exception(spl_ce_RuntimeException,
                         "task stream is already closed", 0);
    RETURN_THROWS();
  }

  int task_id = (int)Z_LVAL(stream->wrapperdata);

  HashTable *update_ht;
  bool update_immutable = bg_worker_is_immutable(Z_ARRVAL_P(data));

  if (update_immutable) {
    update_ht = Z_ARRVAL_P(data);
  } else {
    /* Validate data */
    zval *val;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(data), val) {
      if (!bg_worker_validate_zval(val)) {
        zend_value_error(
            "Task update values must be null, scalars, arrays, or enums");
        RETURN_THROWS();
      }
    }
    ZEND_HASH_FOREACH_END();

    /* Deep-copy to persistent memory */
    zval persistent;
    bg_worker_persist_zval(&persistent, data);
    update_ht = Z_ARRVAL(persistent);
  }

  char *error = go_frankenphp_worker_task_update(task_id, update_ht);
  if (error) {
    if (!update_immutable) {
      zval z;
      ZVAL_ARR(&z, update_ht);
      bg_worker_free_persistent_zval(&z);
    }
    zend_throw_exception(spl_ce_RuntimeException, error, 0);
    free(error);
    RETURN_THROWS();
  }
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

  struct go_mercure_publish_return result =
      go_mercure_publish(thread_index, topics, data, private, id, type, retry);

  switch (result.r1) {
  case 0:
    RETURN_STR(result.r0);
  case 1:
    zend_throw_exception(spl_ce_RuntimeException, "No Mercure hub configured",
                         0);
    RETURN_THROWS();
  case 2:
    zend_throw_exception(spl_ce_RuntimeException, "Publish failed", 0);
    RETURN_THROWS();
  }

  zend_throw_exception(spl_ce_RuntimeException,
                       "FrankenPHP not built with Mercure support", 0);
  RETURN_THROWS();
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
  ret = go_log_attrs(thread_index, message, level, context);
  if (ret != NULL) {
    zend_throw_exception(spl_ce_RuntimeException, ret, 0);
    free(ret);
    RETURN_THROWS();
  }
}

PHP_MINIT_FUNCTION(frankenphp) {
  register_frankenphp_symbols(module_number);

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

  return SUCCESS;
}

static zend_module_entry frankenphp_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp",
    ext_functions,         /* function table */
    PHP_MINIT(frankenphp), /* initialization */
    NULL,                  /* shutdown */
    NULL,                  /* request initialization */
    NULL,                  /* request shutdown */
    NULL,                  /* information */
    TOSTRING(FRANKENPHP_VERSION),
    STANDARD_MODULE_PROPERTIES};

static int frankenphp_startup(sapi_module_struct *sapi_module) {
  php_import_environment_variables = get_full_env;

  return php_module_startup(sapi_module, &frankenphp_module);
}

static int frankenphp_deactivate(void) { return SUCCESS; }

static size_t frankenphp_ub_write(const char *str, size_t str_length) {
  struct go_ub_write_return result =
      go_ub_write(thread_index, (char *)str, str_length);

  if (result.r1) {
    php_handle_aborted_connection();
  }

  return result.r0;
}

static int frankenphp_send_headers(sapi_headers_struct *sapi_headers) {
  if (SG(request_info).no_headers == 1) {
    return SAPI_HEADER_SENT_SUCCESSFULLY;
  }

  int status;

  if (SG(sapi_headers).http_status_line) {
    status = atoi((SG(sapi_headers).http_status_line) + 9);
  } else {
    status = SG(sapi_headers).http_response_code;

    if (!status) {
      status = 200;
    }
  }

  bool success = go_write_headers(thread_index, status, &sapi_headers->headers);
  if (success) {
    return SAPI_HEADER_SENT_SUCCESSFULLY;
  }

  return SAPI_HEADER_SEND_FAILED;
}

static void frankenphp_sapi_flush(void *server_context) {
  sapi_send_headers();
  if (go_sapi_flush(thread_index)) {
    php_handle_aborted_connection();
  }
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes) {
  return go_read_post(thread_index, buffer, count_bytes);
}

static char *frankenphp_read_cookies(void) {
  return go_read_cookies(thread_index);
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
  FRANKENPHP_REGISTER_VAR(php_self);
  FRANKENPHP_REGISTER_VAR(document_uri);
  FRANKENPHP_REGISTER_VAR(script_filename);
  FRANKENPHP_REGISTER_VAR(script_name);
  FRANKENPHP_REGISTER_VAR(ssl_cipher);
  FRANKENPHP_REGISTER_VAR(server_name);
  FRANKENPHP_REGISTER_VAR(server_port);
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
      frankenphp_strings.path_translated,
      (char *)SG(request_info).path_translated, false, track_vars_array);
  frankenphp_register_variable_from_request_info(
      frankenphp_strings.query_string, SG(request_info).query_string, true,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      frankenphp_strings.remote_user, (char *)SG(request_info).auth_user, false,
      track_vars_array);
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

static inline void register_server_variable_filtered(const char *key,
                                                     char **val,
                                                     size_t *val_len,
                                                     zval *track_vars_array) {
  if (sapi_module.input_filter(PARSE_SERVER, key, val, *val_len, val_len)) {
    php_register_variable_safe(key, *val, *val_len, track_vars_array);
  }
}

static void frankenphp_register_variables(zval *track_vars_array) {
  /* https://www.php.net/manual/en/reserved.variables.server.php */

  /* In CGI mode, the environment is part of the $_SERVER variables.
   * $_SERVER and $_ENV should only contain values from the original
   * environment, not values added though putenv
   */
  /* import environment and CGI variables from the request context in go */
  go_register_server_variables(thread_index, track_vars_array);

  /* Some variables are already present in SG(request_info) */
  frankenphp_register_variables_from_request_info(track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int) {
  go_log(thread_index, (char *)message, syslog_type_int);
}

static char *frankenphp_getenv(const char *name, size_t name_len) {
  HashTable *ht = sandboxed_env ? sandboxed_env : main_thread_env;

  zval *env_val = zend_hash_str_find(ht, name, name_len);
  if (env_val && Z_TYPE_P(env_val) == IS_STRING) {
    zend_string *str = Z_STR_P(env_val);
    return ZSTR_VAL(str);
  }

  return NULL;
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
    NULL,                          /* Child terminate */

    STANDARD_SAPI_MODULE_PROPERTIES};

/* Sets thread name for profiling and debugging.
 *
 * Adapted from https://github.com/Pithikos/C-Thread-Pool
 * Copyright: Johan Hanssen Seferidis
 * License: MIT
 */
static void set_thread_name(char *thread_name) {
#if defined(__linux__)
  /* Use prctl instead to prevent using _GNU_SOURCE flag and implicit
   * declaration */
  prctl(PR_SET_NAME, thread_name);
#elif defined(__APPLE__) && defined(__MACH__)
  pthread_setname_np(thread_name);
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
  pthread_set_name_np(pthread_self(), thread_name);
#endif
}

static inline void reset_sandboxed_environment() {
  if (sandboxed_env != NULL) {
    zend_hash_release(sandboxed_env);
    sandboxed_env = NULL;
  }
}

static void *php_thread(void *arg) {
  thread_index = (uintptr_t)arg;
  char thread_name[16] = {0};
  snprintf(thread_name, 16, "php-%" PRIxPTR, thread_index);
  set_thread_name(thread_name);

  /* Initial allocation of all global PHP memory for this thread */
#ifdef ZTS
  (void)ts_resource(0);
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

  /* Save PHP's timer handle for best-effort force-kill after grace period */
  frankenphp_save_php_timer(thread_index);

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
        /* Request startup failed, bail out to zend_catch */
        frankenphp_log_message("Request startup failed, thread is unhealthy",
                               LOG_ERR);
        zend_bailout();
      }

      /* Background worker setup: inject $_SERVER vars, skip shebang, disable
       * timeout */
      if (is_background_worker) {
        CG(skip_shebang) = 1;
        zend_unset_timeout();

        zend_is_auto_global_str("_SERVER", sizeof("_SERVER") - 1);
        zval *server = &PG(http_globals)[TRACK_VARS_SERVER];
        if (server && Z_TYPE_P(server) == IS_ARRAY) {
          if (worker_name != NULL) {
            zval name_zval;
            ZVAL_STRING(&name_zval, worker_name);
            zend_hash_str_update(Z_ARRVAL_P(server), "FRANKENPHP_WORKER_NAME",
                                 sizeof("FRANKENPHP_WORKER_NAME") - 1,
                                 &name_zval);

            zval bg_zval;
            ZVAL_TRUE(&bg_zval);
            zend_hash_str_update(
                Z_ARRVAL_P(server), "FRANKENPHP_WORKER_BACKGROUND",
                sizeof("FRANKENPHP_WORKER_BACKGROUND") - 1, &bg_zval);

            zval argv_array;
            array_init(&argv_array);
            add_next_index_string(&argv_array, scriptName);
            add_next_index_string(&argv_array, worker_name);

            zval argc_zval;
            ZVAL_LONG(&argc_zval, 2);

            zend_hash_str_update(Z_ARRVAL_P(server), "argv", sizeof("argv") - 1,
                                 &argv_array);
            zend_hash_str_update(Z_ARRVAL_P(server), "argc", sizeof("argc") - 1,
                                 &argc_zval);
          }
        }
      }

      zend_file_handle file_handle;
      zend_stream_init_filename(&file_handle, scriptName);

      file_handle.primary_script = 1;
      EG(exit_status) = 0;

      /* Execute the PHP script, potential bailout to zend_catch */
      php_execute_script(&file_handle);
      zend_destroy_file_handle(&file_handle);
      reset_sandboxed_environment();

      /* Update the last memory usage for metrics */
      __atomic_store_n(&thread_metrics[thread_index].last_memory_usage,
                       zend_memory_usage(0), __ATOMIC_RELAXED);

      has_attempted_shutdown = true;

      /* Clean up background worker caches before request shutdown */
      bg_worker_vars_cache_reset();
      if (Z_TYPE(last_set_vars_zval) != IS_UNDEF) {
        zval_ptr_dtor(&last_set_vars_zval);
        ZVAL_UNDEF(&last_set_vars_zval);
      }

      /* shutdown the request, potential bailout to zend_catch */
      php_request_shutdown((void *)0);
      frankenphp_free_request_context();
      go_frankenphp_after_script_execution(thread_index, EG(exit_status));
    }
  }
  zend_catch {
    /* Critical failure from php_execute_script or php_request_shutdown, mark
     * the thread as unhealthy */
    thread_is_healthy = false;
    if (!has_attempted_shutdown) {
      /* php_request_shutdown() was not called, force a shutdown now */
      reset_sandboxed_environment();
      zend_try { php_request_shutdown((void *)0); }
      zend_catch {}
      zend_end_try();
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

  /* free all global PHP memory reserved for this thread */
#ifdef ZTS
  ts_free_thread();
#endif

  frankenphp_worker_close_stop_fds();

  /* Thread is healthy, signal to Go that the thread has shut down */
  if (thread_is_healthy) {
    go_frankenphp_on_thread_shutdown(thread_index);

    return NULL;
  }

  /* Thread is unhealthy, PHP globals might be in a bad state after a bailout,
   * restart the entire thread */
  frankenphp_log_message("Restarting unhealthy thread", LOG_WARNING);

  if (!frankenphp_new_php_thread(thread_index)) {
    /* probably unreachable */
    frankenphp_log_message("Failed to restart an unhealthy thread", LOG_ERR);
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

  frankenphp_init_interned_strings();

  /* take a snapshot of the environment for sandboxing */
  if (main_thread_env == NULL) {
    main_thread_env = pemalloc(sizeof(HashTable), 1);
    zend_hash_init(main_thread_env, 8, NULL, NULL, 1);
    go_init_os_env(main_thread_env);
  }

  frankenphp_sapi_module.startup(&frankenphp_sapi_module);

  /* check if a default filter is set in php.ini and only filter if
   * it is, this is deprecated and will be removed in PHP 9 */
  char *default_filter;
  cfg_get_string("filter.default", &default_filter);
  should_filter_var = default_filter != NULL;
  original_user_abort_setting = PG(ignore_user_abort);

  go_frankenphp_main_thread_is_ready();

  /* channel closed, shutdown gracefully */
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

static int frankenphp_request_startup() {
  frankenphp_update_request_context();
  if (php_request_startup() == SUCCESS) {
    return SUCCESS;
  }

  php_request_shutdown((void *)0);
  frankenphp_free_request_context();

  return FAILURE;
}

int frankenphp_execute_script(char *file_name) {
  if (frankenphp_request_startup() == FAILURE) {

    return FAILURE;
  }

  int status = SUCCESS;

  zend_file_handle file_handle;
  zend_stream_init_filename(&file_handle, file_name);

  file_handle.primary_script = 1;

  if (worker_name != NULL) {
    zend_is_auto_global_str("_SERVER", sizeof("_SERVER") - 1);
    zval *server = &PG(http_globals)[TRACK_VARS_SERVER];
    if (server && Z_TYPE_P(server) == IS_ARRAY) {
      zval name_zval;
      ZVAL_STRING(&name_zval, worker_name);
      zend_hash_str_update(Z_ARRVAL_P(server), "FRANKENPHP_WORKER_NAME",
                           sizeof("FRANKENPHP_WORKER_NAME") - 1, &name_zval);

      zval bg_zval;
      ZVAL_BOOL(&bg_zval, is_background_worker);
      zend_hash_str_update(Z_ARRVAL_P(server), "FRANKENPHP_WORKER_BACKGROUND",
                           sizeof("FRANKENPHP_WORKER_BACKGROUND") - 1,
                           &bg_zval);
    }
  }

  if (is_background_worker) {
    CG(skip_shebang) = 1;

    /* Background workers run indefinitely - disable max_execution_time */
    zend_set_timeout(0, 0);

    zval *server = &PG(http_globals)[TRACK_VARS_SERVER];
    if (server && Z_TYPE_P(server) == IS_ARRAY) {
      zval argv_array;
      array_init(&argv_array);
      add_next_index_string(&argv_array, file_name);
      add_next_index_string(&argv_array, worker_name);

      zval argc_zval;
      ZVAL_LONG(&argc_zval, 2);

      zend_hash_str_update(Z_ARRVAL_P(server), "argv", sizeof("argv") - 1,
                           &argv_array);
      zend_hash_str_update(Z_ARRVAL_P(server), "argc", sizeof("argc") - 1,
                           &argc_zval);
    }
  }

  zend_first_try {
    EG(exit_status) = 0;
    php_execute_script(&file_handle);
    status = EG(exit_status);
  }
  zend_catch { status = EG(exit_status); }
  zend_end_try();

  zend_destroy_file_handle(&file_handle);

  /* Reset the sandboxed environment if it is in use */
  if (sandboxed_env != NULL) {
    zend_hash_release(sandboxed_env);
    sandboxed_env = NULL;
  }

  bg_worker_vars_cache_reset();
  if (Z_TYPE(last_set_vars_zval) != IS_UNDEF) {
    zval_ptr_dtor(&last_set_vars_zval);
    ZVAL_UNDEF(&last_set_vars_zval);
  }
  php_request_shutdown((void *)0);
  frankenphp_free_request_context();

  return status;
}

/* Use global variables to store CLI arguments to prevent useless allocations */
static char *cli_script;
static int cli_argc;
static char **cli_argv;

/*
 * CLI code is adapted from
 * https://github.com/php/php-src/blob/master/sapi/cli/php_cli.c Copyright (c)
 * The PHP Group Licensed under The PHP License Original uthors: Edin Kadribasic
 * <edink@php.net>, Marcus Boerger <helly@php.net> and Johannes Schlueter
 * <johannes@php.net> Parts based on CGI SAPI Module by Rasmus Lerdorf, Stig
 * Bakken and Zeev Suraski
 */
static void cli_register_file_handles(void) {
  php_stream *s_in, *s_out, *s_err;
  php_stream_context *sc_in = NULL, *sc_out = NULL, *sc_err = NULL;
  zend_constant ic, oc, ec;

  s_in = php_stream_open_wrapper_ex("php://stdin", "rb", 0, NULL, sc_in);
  s_out = php_stream_open_wrapper_ex("php://stdout", "wb", 0, NULL, sc_out);
  s_err = php_stream_open_wrapper_ex("php://stderr", "wb", 0, NULL, sc_err);

  /* Release stream resources, but don't free the underlying handles. Othewrise,
   * extensions which write to stderr or company during mshutdown/gshutdown
   * won't have the expected functionality.
   */
  if (s_in)
    s_in->flags |= PHP_STREAM_FLAG_NO_RSCR_DTOR_CLOSE;
  if (s_out)
    s_out->flags |= PHP_STREAM_FLAG_NO_RSCR_DTOR_CLOSE;
  if (s_err)
    s_err->flags |= PHP_STREAM_FLAG_NO_RSCR_DTOR_CLOSE;

  if (s_in == NULL || s_out == NULL || s_err == NULL) {
    if (s_in)
      php_stream_close(s_in);
    if (s_out)
      php_stream_close(s_out);
    if (s_err)
      php_stream_close(s_err);
    return;
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

static void sapi_cli_register_variables(zval *track_vars_array) /* {{{ */
{
  size_t len = strlen(cli_script);
  char *docroot = "";

  /*
   * In CGI mode, we consider the environment to be a part of the server
   * variables
   */
  php_import_environment_variables(track_vars_array);

  /* Build the special-case PHP_SELF variable for the CLI version */
  register_server_variable_filtered("PHP_SELF", &cli_script, &len,
                                    track_vars_array);
  register_server_variable_filtered("SCRIPT_NAME", &cli_script, &len,
                                    track_vars_array);

  /* filenames are empty for stdin */
  register_server_variable_filtered("SCRIPT_FILENAME", &cli_script, &len,
                                    track_vars_array);
  register_server_variable_filtered("PATH_TRANSLATED", &cli_script, &len,
                                    track_vars_array);

  /* just make it available */
  len = 0U;
  register_server_variable_filtered("DOCUMENT_ROOT", &docroot, &len,
                                    track_vars_array);
}
/* }}} */

static void *execute_script_cli(void *arg) {
  void *exit_status;
  bool eval = (bool)arg;

  /*
   * The SAPI name "cli" is hardcoded into too many programs... let's usurp it.
   */
  php_embed_module.name = "cli";
  php_embed_module.pretty_name = "PHP CLI embedded in FrankenPHP";
  php_embed_module.register_server_variables = sapi_cli_register_variables;

  php_embed_init(cli_argc, cli_argv);

  cli_register_file_handles();
  zend_first_try {
    if (eval) {
      /* evaluate the cli_script as literal PHP code (php-cli -r "...") */
      zend_eval_string_ex(cli_script, NULL, "Command line code", 1);
    } else {
      zend_file_handle file_handle;
      zend_stream_init_filename(&file_handle, cli_script);

      CG(skip_shebang) = 1;
      php_execute_script(&file_handle);
    }
  }
  zend_end_try();

  exit_status = (void *)(intptr_t)EG(exit_status);

  php_embed_shutdown();

  return exit_status;
}

int frankenphp_execute_script_cli(char *script, int argc, char **argv,
                                  bool eval) {
  pthread_t thread;
  int err;
  void *exit_status;

  cli_script = script;
  cli_argc = argc;
  cli_argv = argv;

  /*
   * Start the script in a dedicated thread to prevent conflicts between Go and
   * PHP signal handlers
   */
  err = pthread_create(&thread, NULL, execute_script_cli, (void *)eval);
  if (err != 0) {
    return err;
  }

  err = pthread_join(thread, &exit_status);
  if (err != 0) {
    return err;
  }

  return (intptr_t)exit_status;
}

int frankenphp_reset_opcache(void) {
  zend_function *opcache_reset =
      zend_hash_str_find_ptr(CG(function_table), ZEND_STRL("opcache_reset"));
  if (opcache_reset) {
    zend_call_known_function(opcache_reset, NULL, NULL, NULL, 0, NULL, NULL);
  }

  return 0;
}

int frankenphp_get_current_memory_limit() { return PG(memory_limit); }

void frankenphp_init_thread_metrics(int max_threads) {
  thread_metrics = calloc(max_threads, sizeof(frankenphp_thread_metrics));
}

void frankenphp_destroy_thread_metrics(void) {
  free(thread_metrics);
  thread_metrics = NULL;
}

size_t frankenphp_get_thread_memory_usage(uintptr_t idx) {
  return __atomic_load_n(&thread_metrics[idx].last_memory_usage,
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

  for (int i = 0; i < modules_len; i++) {
    if (zend_register_internal_module(modules[i]) == NULL) {
      return FAILURE;
    }
  }

  modules = NULL;
  modules_len = 0;

  return SUCCESS;
}

void register_extensions(zend_module_entry **m, int len) {
  modules = m;
  modules_len = len;

  original_php_register_internal_extensions_func =
      php_register_internal_extensions_func;
  php_register_internal_extensions_func = register_internal_extensions;
}
