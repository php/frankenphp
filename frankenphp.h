#ifndef _FRANKENPHP_H
#define _FRANKENPHP_H

#ifdef _WIN32
// Define this to prevent windows.h from including legacy winsock.h
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif

// Explicitly include Winsock2 BEFORE windows.h
#include <windows.h>
#include <winerror.h>
#include <winsock2.h>
#include <ws2tcpip.h>

// Fix for missing IntSafe functions (LongLongAdd) when building with Clang
#ifdef __clang__
#ifndef INTSAFE_E_ARITHMETIC_OVERFLOW
#define INTSAFE_E_ARITHMETIC_OVERFLOW ((HRESULT)0x80070216L)
#endif

#ifndef LongLongAdd
static inline HRESULT LongLongAdd(LONGLONG llAugend, LONGLONG llAddend,
                                  LONGLONG *pllResult) {
  if (__builtin_add_overflow(llAugend, llAddend, pllResult)) {
    return INTSAFE_E_ARITHMETIC_OVERFLOW;
  }
  return S_OK;
}
#endif

#ifndef LongLongSub
static inline HRESULT LongLongSub(LONGLONG llMinuend, LONGLONG llSubtrahend,
                                  LONGLONG *pllResult) {
  if (__builtin_sub_overflow(llMinuend, llSubtrahend, pllResult)) {
    return INTSAFE_E_ARITHMETIC_OVERFLOW;
  }
  return S_OK;
}
#endif
#endif
#endif

#include <Zend/zend_modules.h>
#include <Zend/zend_types.h>
#include <stdbool.h>
#include <stdint.h>

#ifndef PHP_WIN32
#include <pthread.h>
#include <signal.h>
#endif

/* Platform capabilities for the force-kill primitive; declared in the
 * header so Go (via CGo) gets the correct struct layout too. */
#if !defined(PHP_WIN32) && defined(SIGRTMIN)
#define FRANKENPHP_HAS_KILL_SIGNAL 1
#define FRANKENPHP_KILL_SIGNAL (SIGRTMIN + 3)
#endif

typedef struct {
  zend_atomic_bool *vm_interrupt;
  zend_atomic_bool *timed_out;
#ifdef FRANKENPHP_HAS_KILL_SIGNAL
  pthread_t tid;
#elif defined(PHP_WIN32)
  HANDLE thread_handle;
#endif
} force_kill_slot;

#ifndef FRANKENPHP_VERSION
#define FRANKENPHP_VERSION dev
#endif
#define STRINGIFY(x) #x
#define TOSTRING(x) STRINGIFY(x)

typedef struct go_string {
  size_t len;
  char *data;
} go_string;

typedef struct frankenphp_server_vars {
  size_t total_num_vars;
  char *remote_addr;
  size_t remote_addr_len;
  char *remote_host;
  size_t remote_host_len;
  char *remote_port;
  size_t remote_port_len;
  char *document_root;
  size_t document_root_len;
  char *path_info;
  size_t path_info_len;
  char *php_self;
  size_t php_self_len;
  char *document_uri;
  size_t document_uri_len;
  char *script_filename;
  size_t script_filename_len;
  char *script_name;
  size_t script_name_len;
  char *server_name;
  size_t server_name_len;
  char *server_port;
  size_t server_port_len;
  char *content_length;
  size_t content_length_len;
  char *server_protocol;
  size_t server_protocol_len;
  char *http_host;
  size_t http_host_len;
  char *request_uri;
  size_t request_uri_len;
  char *ssl_cipher;
  size_t ssl_cipher_len;
  zend_string *request_scheme;
  zend_string *ssl_protocol;
  zend_string *https;
} frankenphp_server_vars;

/**
 * Cached interned strings for memory and performance benefits
 * Add more hard-coded strings here if needed
 */
#define FRANKENPHP_INTERNED_STRINGS_LIST(X)                                    \
  X(remote_addr, "REMOTE_ADDR")                                                \
  X(remote_host, "REMOTE_HOST")                                                \
  X(remote_port, "REMOTE_PORT")                                                \
  X(document_root, "DOCUMENT_ROOT")                                            \
  X(path_info, "PATH_INFO")                                                    \
  X(php_self, "PHP_SELF")                                                      \
  X(document_uri, "DOCUMENT_URI")                                              \
  X(script_filename, "SCRIPT_FILENAME")                                        \
  X(script_name, "SCRIPT_NAME")                                                \
  X(https, "HTTPS")                                                            \
  X(httpsLowercase, "https")                                                   \
  X(httpLowercase, "http")                                                     \
  X(ssl_protocol, "SSL_PROTOCOL")                                              \
  X(request_scheme, "REQUEST_SCHEME")                                          \
  X(server_name, "SERVER_NAME")                                                \
  X(server_port, "SERVER_PORT")                                                \
  X(content_length, "CONTENT_LENGTH")                                          \
  X(server_protocol, "SERVER_PROTOCOL")                                        \
  X(http_host, "HTTP_HOST")                                                    \
  X(request_uri, "REQUEST_URI")                                                \
  X(ssl_cipher, "SSL_CIPHER")                                                  \
  X(server_software, "SERVER_SOFTWARE")                                        \
  X(frankenphp, "FrankenPHP")                                                  \
  X(gateway_interface, "GATEWAY_INTERFACE")                                    \
  X(cgi11, "CGI/1.1")                                                          \
  X(auth_type, "AUTH_TYPE")                                                    \
  X(remote_ident, "REMOTE_IDENT")                                              \
  X(content_type, "CONTENT_TYPE")                                              \
  X(path_translated, "PATH_TRANSLATED")                                        \
  X(query_string, "QUERY_STRING")                                              \
  X(remote_user, "REMOTE_USER")                                                \
  X(request_method, "REQUEST_METHOD")                                          \
  X(tls1, "TLSv1")                                                             \
  X(tls11, "TLSv1.1")                                                          \
  X(tls12, "TLSv1.2")                                                          \
  X(tls13, "TLSv1.3")                                                          \
  X(on, "on")                                                                  \
  X(empty, "")

typedef struct frankenphp_interned_strings_t {
#define F_DEFINE_STRUCT_FIELD(name, str) zend_string *name;
  FRANKENPHP_INTERNED_STRINGS_LIST(F_DEFINE_STRUCT_FIELD)
#undef F_DEFINE_STRUCT_FIELD
} frankenphp_interned_strings_t;

extern frankenphp_interned_strings_t frankenphp_strings;

typedef struct frankenphp_version {
  unsigned char major_version;
  unsigned char minor_version;
  unsigned char release_version;
  const char *extra_version;
  const char *version;
  unsigned long version_id;
} frankenphp_version;
frankenphp_version frankenphp_get_version();

typedef struct frankenphp_config {
  bool zts;
  bool zend_signals;
  bool zend_max_execution_timers;
} frankenphp_config;
frankenphp_config frankenphp_get_config();

int frankenphp_new_main_thread(int num_threads);
bool frankenphp_new_php_thread(uintptr_t thread_index);

bool frankenphp_shutdown_dummy_request(void);
void frankenphp_update_local_thread_context(bool is_worker);

int frankenphp_execute_script_cli(char *script, int argc, char **argv,
                                  bool eval);

void frankenphp_register_known_variable(zend_string *z_key, char *value,
                                        size_t val_len, zval *track_vars_array);
void frankenphp_register_variable_safe(char *key, char *var, size_t val_len,
                                       zval *track_vars_array);
void frankenphp_register_server_vars(zval *track_vars_array,
                                     frankenphp_server_vars vars);

zend_string *frankenphp_init_persistent_string(const char *string, size_t len);
int frankenphp_reset_opcache(void);
int frankenphp_get_current_memory_limit();

typedef struct {
  size_t last_memory_usage;
} frankenphp_thread_metrics;

void frankenphp_init_thread_metrics(int max_threads);
void frankenphp_destroy_thread_metrics(void);
size_t frankenphp_get_thread_memory_usage(uintptr_t thread_index);

/* Best-effort force-kill primitives. The slot is populated by each PHP
 * thread at boot (an internal helper calls back into Go via
 * go_frankenphp_store_force_kill_slot) and lives in the Go-side phpThread.
 * force_kill_thread interrupts the Zend VM at the next opcode boundary;
 * on POSIX it also delivers SIGRTMIN+3 to the target thread, on Windows
 * it calls CancelSynchronousIo + QueueUserAPC. release_thread drops any
 * OS-owned resource tied to the slot (currently the Windows thread
 * handle). */
void frankenphp_force_kill_thread(force_kill_slot slot);
void frankenphp_release_thread_for_kill(force_kill_slot slot);

void register_extensions(zend_module_entry **m, int len);

#endif
