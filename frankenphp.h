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

typedef struct ht_key_value_pair {
  zend_string *key;
  char *val;
  size_t val_len;
} ht_key_value_pair;

typedef struct php_variable {
  const char *var;
  size_t data_len;
  char *data;
} php_variable;

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
  frankenphp_version version;
  bool zts;
  bool zend_signals;
  bool zend_max_execution_timers;
} frankenphp_config;
frankenphp_config frankenphp_get_config();

int frankenphp_new_main_thread(int num_threads);
bool frankenphp_new_php_thread(uintptr_t thread_index);

bool frankenphp_shutdown_dummy_request(void);
int frankenphp_execute_script(char *file_name);

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
void frankenphp_add_assoc_str_ex(zval *track_vars_array, char *key,
                                 size_t keylen, zend_string *val);

void frankenphp_register_single(zend_string *z_key, char *value, size_t val_len,
                                zval *track_vars_array);
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
    ht_key_value_pair request_uri, ht_key_value_pair ssl_cipher);

void register_extensions(zend_module_entry *m, int len);

#endif
