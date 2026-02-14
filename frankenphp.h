#ifndef _FRANKENPHP_H
#define _FRANKENPHP_H

#include <Zend/zend_modules.h>
#include <Zend/zend_types.h>
#include <stdbool.h>
#include <stdint.h>

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
  char *https;
  size_t https_len;
  char *ssl_protocol;
  size_t ssl_protocol_len;
  char *request_scheme;
  size_t request_scheme_len;
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
} frankenphp_server_vars;

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
int frankenphp_execute_script(char *file_name);
void frankenphp_update_local_thread_context(bool is_worker);

int frankenphp_execute_script_cli(char *script, int argc, char **argv,
                                  bool eval);
void frankenphp_register_variable_safe(char *key, char *var, size_t val_len,
                                       zval *track_vars_array);
zend_string *frankenphp_init_persistent_string(const char *string, size_t len);
int frankenphp_reset_opcache(void);
int frankenphp_get_current_memory_limit();

void frankenphp_register_single(zend_string *z_key, char *value, size_t val_len,
                                zval *track_vars_array);
void frankenphp_register_bulk(zval *track_vars_array,
                              frankenphp_server_vars vars);

void register_extensions(zend_module_entry **m, int len);

#endif
