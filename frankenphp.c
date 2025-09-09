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
static const char *MODULES_TO_RELOAD[] = {"filter", "session", NULL};

frankenphp_version frankenphp_get_version() {
  return (frankenphp_version){
      PHP_MAJOR_VERSION,           PHP_MINOR_VERSION, PHP_RELEASE_VERSION,
      PHP_EXTRA_VERSION,           PHP_VERSION,       PHP_VERSION_ID,
      TOSTRING(FRANKENPHP_VERSION)};
}

frankenphp_config frankenphp_get_config() {
  return (frankenphp_config){
      frankenphp_get_version(),
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
__thread uintptr_t thread_index;
__thread bool is_worker_thread = false;
__thread zval *os_environment = NULL;

static void frankenphp_update_request_context() {
  /* the server context is stored on the go side, still SG(server_context) needs
   * to not be NULL */
  SG(server_context) = (void *)1;
  /* status It is not reset by zend engine, set it to 200. */
  SG(sapi_headers).http_response_code = 200;

  is_worker_thread = go_update_request_info(thread_index, &SG(request_info));
}

static void frankenphp_free_request_context() {
  if (SG(request_info).cookie_data != NULL) {
    free(SG(request_info).cookie_data);
    SG(request_info).cookie_data = NULL;
  }

  /* freed via thread.Unpin() */
  SG(request_info).auth_password = NULL;
  SG(request_info).auth_user = NULL;
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
  }
  zend_end_try();

  zend_auto_global *auto_global;
  zend_string *_env = ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_ENV);
  zend_string *_server = ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_SERVER);
  ZEND_HASH_MAP_FOREACH_PTR(CG(auto_globals), auto_global) {
    if (auto_global->name == _env) {
      /* skip $_ENV */
    } else if (auto_global->name == _server) {
      /* always reimport $_SERVER  */
      auto_global->armed = auto_global->auto_global_callback(auto_global->name);
    } else if (auto_global->jit) {
      /* globals with jit are: $_SERVER, $_ENV, $_REQUEST, $GLOBALS,
       * jit will only trigger on script parsing and therefore behaves
       * differently in worker mode. We will skip all jit globals
       */
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

/* Adapted from php_request_shutdown */
static void frankenphp_worker_request_shutdown() {
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

  /* Shutdown output layer (send the set HTTP headers, cleanup output handlers,
   * etc.) */
  zend_try { php_output_deactivate(); }
  zend_end_try();

  /* SAPI related shutdown (free stuff) */
  frankenphp_free_request_context();
  zend_try { sapi_deactivate(); }
  zend_end_try();

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

PHPAPI void get_full_env(zval *track_vars_array) {
  go_getfullenv(thread_index, track_vars_array);
}

void frankenphp_add_assoc_str_ex(zval *track_vars_array, char *key,
                                 size_t keylen, zend_string *val) {
  add_assoc_str_ex(track_vars_array, key, keylen, val);
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

  if (go_putenv(thread_index, setting, (int)setting_len)) {
    RETURN_TRUE;
  } else {
    RETURN_FALSE;
  }
} /* }}} */

/* {{{ Call go's getenv to prevent race conditions */
PHP_FUNCTION(frankenphp_getenv) {
  char *name = NULL;
  size_t name_len = 0;
  bool local_only = 0;

  ZEND_PARSE_PARAMETERS_START(0, 2)
  Z_PARAM_OPTIONAL
  Z_PARAM_STRING_OR_NULL(name, name_len)
  Z_PARAM_BOOL(local_only)
  ZEND_PARSE_PARAMETERS_END();

  if (!name) {
    array_init(return_value);
    get_full_env(return_value);

    return;
  }

  struct go_getenv_return result = go_getenv(thread_index, name);

  if (result.r0) {
    // Return the single environment variable as a string
    RETVAL_STR(result.r1);
  } else {
    // Environment variable does not exist
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

  if (!is_worker_thread) {
    /* not a worker, throw an error */
    zend_throw_exception(
        spl_ce_RuntimeException,
        "frankenphp_handle_request() called while not in worker mode", 0);
    RETURN_THROWS();
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /* Disable timeouts while waiting for a request to handle */
  zend_unset_timeout();
#endif

  bool has_request = go_frankenphp_worker_handle_request_start(thread_index);
  if (frankenphp_worker_request_startup() == FAILURE
      /* Shutting down */
      || !has_request) {
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
  fci.size = sizeof fci;
  fci.retval = &retval;
  if (zend_call_function(&fci, &fcc) == SUCCESS) {
    zval_ptr_dtor(&retval);
  }

  /*
   * If an exception occurred, print the message to the client before
   * closing the connection and bailout.
   */
  if (EG(exception) && !zend_is_unwind_exit(EG(exception)) &&
      !zend_is_graceful_exit(EG(exception))) {
    zend_exception_error(EG(exception), E_ERROR);
    zend_bailout();
  }

  frankenphp_worker_request_shutdown();
  go_frankenphp_finish_worker_request(thread_index);

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

PHP_MINIT_FUNCTION(frankenphp) {
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

static void frankenphp_request_shutdown() {
  frankenphp_free_request_context();
  php_request_shutdown((void *)0);
}

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
void frankenphp_register_trusted_var(zend_string *z_key, char *value,
                                     size_t val_len, HashTable *ht) {
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

void frankenphp_register_single(zend_string *z_key, char *value, size_t val_len,
                                zval *track_vars_array) {
  HashTable *ht = Z_ARRVAL_P(track_vars_array);
  frankenphp_register_trusted_var(z_key, value, val_len, ht);
}

/* Register known $_SERVER variables in bulk to avoid cgo overhead */
void frankenphp_register_bulk(
    zval *track_vars_array, ht_key_value_pair remote_addr,
    ht_key_value_pair remote_host, ht_key_value_pair remote_port,
    ht_key_value_pair document_root, ht_key_value_pair path_info,
    ht_key_value_pair php_self, ht_key_value_pair document_uri,
    ht_key_value_pair script_filename, ht_key_value_pair script_name,
    ht_key_value_pair https, ht_key_value_pair ssl_protocol,
    ht_key_value_pair request_scheme, ht_key_value_pair server_name,
    ht_key_value_pair server_port, ht_key_value_pair content_length,
    ht_key_value_pair gateway_interface, ht_key_value_pair server_protocol,
    ht_key_value_pair server_software, ht_key_value_pair http_host,
    ht_key_value_pair auth_type, ht_key_value_pair remote_ident,
    ht_key_value_pair request_uri, ht_key_value_pair ssl_cipher) {
  HashTable *ht = Z_ARRVAL_P(track_vars_array);
  frankenphp_register_trusted_var(remote_addr.key, remote_addr.val,
                                  remote_addr.val_len, ht);
  frankenphp_register_trusted_var(remote_host.key, remote_host.val,
                                  remote_host.val_len, ht);
  frankenphp_register_trusted_var(remote_port.key, remote_port.val,
                                  remote_port.val_len, ht);
  frankenphp_register_trusted_var(document_root.key, document_root.val,
                                  document_root.val_len, ht);
  frankenphp_register_trusted_var(path_info.key, path_info.val,
                                  path_info.val_len, ht);
  frankenphp_register_trusted_var(php_self.key, php_self.val, php_self.val_len,
                                  ht);
  frankenphp_register_trusted_var(document_uri.key, document_uri.val,
                                  document_uri.val_len, ht);
  frankenphp_register_trusted_var(script_filename.key, script_filename.val,
                                  script_filename.val_len, ht);
  frankenphp_register_trusted_var(script_name.key, script_name.val,
                                  script_name.val_len, ht);
  frankenphp_register_trusted_var(https.key, https.val, https.val_len, ht);
  frankenphp_register_trusted_var(ssl_protocol.key, ssl_protocol.val,
                                  ssl_protocol.val_len, ht);
  frankenphp_register_trusted_var(ssl_cipher.key, ssl_cipher.val,
                                  ssl_cipher.val_len, ht);
  frankenphp_register_trusted_var(request_scheme.key, request_scheme.val,
                                  request_scheme.val_len, ht);
  frankenphp_register_trusted_var(server_name.key, server_name.val,
                                  server_name.val_len, ht);
  frankenphp_register_trusted_var(server_port.key, server_port.val,
                                  server_port.val_len, ht);
  frankenphp_register_trusted_var(content_length.key, content_length.val,
                                  content_length.val_len, ht);
  frankenphp_register_trusted_var(gateway_interface.key, gateway_interface.val,
                                  gateway_interface.val_len, ht);
  frankenphp_register_trusted_var(server_protocol.key, server_protocol.val,
                                  server_protocol.val_len, ht);
  frankenphp_register_trusted_var(server_software.key, server_software.val,
                                  server_software.val_len, ht);
  frankenphp_register_trusted_var(http_host.key, http_host.val,
                                  http_host.val_len, ht);
  frankenphp_register_trusted_var(auth_type.key, auth_type.val,
                                  auth_type.val_len, ht);
  frankenphp_register_trusted_var(remote_ident.key, remote_ident.val,
                                  remote_ident.val_len, ht);
  frankenphp_register_trusted_var(request_uri.key, request_uri.val,
                                  request_uri.val_len, ht);
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

static void
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

void frankenphp_register_variables_from_request_info(
    zval *track_vars_array, zend_string *content_type,
    zend_string *path_translated, zend_string *query_string,
    zend_string *auth_user, zend_string *request_method) {
  frankenphp_register_variable_from_request_info(
      content_type, (char *)SG(request_info).content_type, true,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      path_translated, (char *)SG(request_info).path_translated, false,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      query_string, SG(request_info).query_string, true, track_vars_array);
  frankenphp_register_variable_from_request_info(
      auth_user, (char *)SG(request_info).auth_user, false, track_vars_array);
  frankenphp_register_variable_from_request_info(
      request_method, (char *)SG(request_info).request_method, false,
      track_vars_array);
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

  /* In CGI mode, we consider the environment to be a part of the server
   * variables.
   */

  /* in non-worker mode we import the os environment regularly */
  if (!is_worker_thread) {
    get_full_env(track_vars_array);
    // php_import_environment_variables(track_vars_array);
    go_register_variables(thread_index, track_vars_array);
    return;
  }

  /* In worker mode we cache the os environment */
  if (os_environment == NULL) {
    os_environment = malloc(sizeof(zval));
    if (os_environment == NULL) {
      php_error(E_ERROR, "Failed to allocate memory for os_environment");

      return;
    }
    array_init(os_environment);
    get_full_env(os_environment);
    // php_import_environment_variables(os_environment);
  }
  zend_hash_copy(Z_ARR_P(track_vars_array), Z_ARR_P(os_environment),
                 (copy_ctor_func_t)zval_add_ref);

  go_register_variables(thread_index, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int) {
  go_log((char *)message, syslog_type_int);
}

static char *frankenphp_getenv(const char *name, size_t name_len) {
  struct go_getenv_return result = go_getenv(thread_index, (char *)name);

  if (result.r0) {
    return result.r1->val;
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

static void *php_thread(void *arg) {
  thread_index = (uintptr_t)arg;
  char thread_name[16] = {0};
  snprintf(thread_name, 16, "php-%" PRIxPTR, thread_index);
  set_thread_name(thread_name);

#ifdef ZTS
  /* initial resource fetch */
  (void)ts_resource(0);
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

  // loop until Go signals to stop
  char *scriptName = NULL;
  while ((scriptName = go_frankenphp_before_script_execution(thread_index))) {
    go_frankenphp_after_script_execution(thread_index,
                                         frankenphp_execute_script(scriptName));
  }

#ifdef ZTS
  ts_free_thread();
#endif

  go_frankenphp_on_thread_shutdown(thread_index);

  return NULL;
}

static void *php_main(void *arg) {
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

  frankenphp_sapi_module.startup(&frankenphp_sapi_module);

  /* check if a default filter is set in php.ini and only filter if
   * it is, this is deprecated and will be removed in PHP 9 */
  char *default_filter;
  cfg_get_string("filter.default", &default_filter);
  should_filter_var = default_filter != NULL;

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

  frankenphp_free_request_context();
  php_request_shutdown((void *)0);

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

  zend_first_try {
    EG(exit_status) = 0;
    php_execute_script(&file_handle);
    status = EG(exit_status);
  }
  zend_catch { status = EG(exit_status); }
  zend_end_try();

  // free the cached os environment before shutting down the script
  if (os_environment != NULL) {
    zval_ptr_dtor(os_environment);
    free(os_environment);
    os_environment = NULL;
  }

  zend_destroy_file_handle(&file_handle);

  frankenphp_request_shutdown();

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

  cli_register_file_handles(false);
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

static zend_module_entry *modules = NULL;
static int modules_len = 0;
static int (*original_php_register_internal_extensions_func)(void) = NULL;

PHPAPI int register_internal_extensions(void) {
  if (original_php_register_internal_extensions_func != NULL &&
      original_php_register_internal_extensions_func() != SUCCESS) {
    return FAILURE;
  }

  for (int i = 0; i < modules_len; i++) {
    if (zend_register_internal_module(&modules[i]) == NULL) {
      return FAILURE;
    }
  }

  modules = NULL;
  modules_len = 0;

  return SUCCESS;
}

void register_extensions(zend_module_entry *m, int len) {
  modules = m;
  modules_len = len;

  original_php_register_internal_extensions_func =
      php_register_internal_extensions_func;
  php_register_internal_extensions_func = register_internal_extensions;
}

/* EXPERIMENTAL */
PHP_FUNCTION(frankenphp_info) {
  if (zend_parse_parameters_none() == FAILURE) {
    RETURN_THROWS();
  }

  zval *result = go_frankenphp_info(thread_index);

  RETURN_ARR(Z_ARR_P(result));
}