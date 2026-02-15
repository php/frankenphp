#ifndef _FRANKENPHP_STRINGS_H
#define _FRANKENPHP_STRINGS_H

/**
 * Cached interned strings for memory and performance benefits
 * Add more hard-coded strings here if needed
 */
typedef struct frankenphp_interned_strings_t {
  zend_string *remote_addr;
  zend_string *remote_host;
  zend_string *remote_port;
  zend_string *document_root;
  zend_string *path_info;
  zend_string *php_self;
  zend_string *document_uri;
  zend_string *script_filename;
  zend_string *script_name;
  zend_string *https;
  zend_string *ssl_protocol;
  zend_string *request_scheme;
  zend_string *server_name;
  zend_string *server_port;
  zend_string *content_length;
  zend_string *server_protocol;
  zend_string *http_host;
  zend_string *request_uri;
  zend_string *ssl_cipher;
  zend_string *server_software;
  zend_string *server_software_str;
  zend_string *gateway_interface;
  zend_string *gateway_interface_str;
  zend_string *auth_type;
  zend_string *remote_ident;
  zend_string *content_type;
  zend_string *path_translated;
  zend_string *query_string;
  zend_string *remote_user;
  zend_string *request_method;
} frankenphp_interned_strings_t;

zend_string *frankenphp_init_persistent_string(const char *string, size_t len) {
  /* persistent strings will be ignored by the GC at the end of a request */
  zend_string *z_string = zend_string_init(string, len, 1);
  zend_string_hash_val(z_string);

  /* interned strings will not be ref counted by the GC */
  GC_ADD_FLAGS(z_string, IS_STR_INTERNED);

  return z_string;
}

#define FRANKENPHP_INTERNED_STR(str)                                             \
  frankenphp_init_persistent_string(str, sizeof(str) - 1)

static frankenphp_interned_strings_t frankenphp_init_interned_strings() {
  return (frankenphp_interned_strings_t){
      .remote_addr = FRANKENPHP_INTERNED_STR("REMOTE_ADDR"),
      .remote_host = FRANKENPHP_INTERNED_STR("REMOTE_HOST"),
      .remote_port = FRANKENPHP_INTERNED_STR("REMOTE_PORT"),
      .document_root = FRANKENPHP_INTERNED_STR("DOCUMENT_ROOT"),
      .path_info = FRANKENPHP_INTERNED_STR("PATH_INFO"),
      .php_self = FRANKENPHP_INTERNED_STR("PHP_SELF"),
      .document_uri = FRANKENPHP_INTERNED_STR("DOCUMENT_URI"),
      .script_filename = FRANKENPHP_INTERNED_STR("SCRIPT_FILENAME"),
      .script_name = FRANKENPHP_INTERNED_STR("SCRIPT_NAME"),
      .https = FRANKENPHP_INTERNED_STR("HTTPS"),
      .ssl_protocol = FRANKENPHP_INTERNED_STR("SSL_PROTOCOL"),
      .request_scheme = FRANKENPHP_INTERNED_STR("REQUEST_SCHEME"),
      .server_name = FRANKENPHP_INTERNED_STR("SERVER_NAME"),
      .server_port = FRANKENPHP_INTERNED_STR("SERVER_PORT"),
      .content_length = FRANKENPHP_INTERNED_STR("CONTENT_LENGTH"),
      .server_protocol = FRANKENPHP_INTERNED_STR("SERVER_PROTOCOL"),
      .http_host = FRANKENPHP_INTERNED_STR("HTTP_HOST"),
      .request_uri = FRANKENPHP_INTERNED_STR("REQUEST_URI"),
      .ssl_cipher = FRANKENPHP_INTERNED_STR("SSL_CIPHER"),
      .server_software = FRANKENPHP_INTERNED_STR("SERVER_SOFTWARE"),
      .server_software_str = FRANKENPHP_INTERNED_STR("FrankenPHP"),
      .gateway_interface = FRANKENPHP_INTERNED_STR("GATEWAY_INTERFACE"),
      .gateway_interface_str = FRANKENPHP_INTERNED_STR("CGI/1.1"),
      .auth_type = FRANKENPHP_INTERNED_STR("AUTH_TYPE"),
      .remote_ident = FRANKENPHP_INTERNED_STR("REMOTE_IDENT"),
      .content_type = FRANKENPHP_INTERNED_STR("CONTENT_TYPE"),
      .path_translated = FRANKENPHP_INTERNED_STR("PATH_TRANSLATED"),
      .query_string = FRANKENPHP_INTERNED_STR("QUERY_STRING"),
      .remote_user = FRANKENPHP_INTERNED_STR("REMOTE_USER"),
      .request_method = FRANKENPHP_INTERNED_STR("REQUEST_METHOD"),
  };
}

#undef FRANKENPHP_INTERNED_STR

#endif
