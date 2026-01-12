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
  zend_string *remote_addr_key;
  char *remote_addr_val;
  size_t remote_addr_len;

  zend_string *remote_host_key;
  char *remote_host_val;
  size_t remote_host_len;

  zend_string *remote_port_key;
  char *remote_port_val;
  size_t remote_port_len;

  zend_string *document_root_key;
  char *document_root_val;
  size_t document_root_len;

  zend_string *path_info_key;
  char *path_info_val;
  size_t path_info_len;

  zend_string *php_self_key;
  char *php_self_val;
  size_t php_self_len;

  zend_string *document_uri_key;
  char *document_uri_val;
  size_t document_uri_len;

  zend_string *script_filename_key;
  char *script_filename_val;
  size_t script_filename_len;

  zend_string *script_name_key;
  char *script_name_val;
  size_t script_name_len;

  zend_string *https_key;
  char *https_val;
  size_t https_len;

  zend_string *ssl_protocol_key;
  char *ssl_protocol_val;
  size_t ssl_protocol_len;

  zend_string *request_scheme_key;
  char *request_scheme_val;
  size_t request_scheme_len;

  zend_string *server_name_key;
  char *server_name_val;
  size_t server_name_len;

  zend_string *server_port_key;
  char *server_port_val;
  size_t server_port_len;

  zend_string *content_length_key;
  char *content_length_val;
  size_t content_length_len;

  zend_string *gateway_interface_key;
  zend_string *gateway_interface_str;

  zend_string *server_protocol_key;
  char *server_protocol_val;
  size_t server_protocol_len;

  zend_string *server_software_key;
  zend_string *server_software_str;

  zend_string *http_host_key;
  char *http_host_val;
  size_t http_host_len;

  zend_string *auth_type_key;
  char *auth_type_val;
  size_t auth_type_len;

  zend_string *remote_ident_key;
  char *remote_ident_val;
  size_t remote_ident_len;

  zend_string *request_uri_key;
  char *request_uri_val;
  size_t request_uri_len;

  zend_string *ssl_cipher_key;
  char *ssl_cipher_val;
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

void frankenphp_register_variables_from_request_info(
    zval *track_vars_array, zend_string *content_type,
    zend_string *path_translated, zend_string *query_string,
    zend_string *auth_user, zend_string *request_method);
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
