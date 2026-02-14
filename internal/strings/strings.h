#ifndef _FRANKENPHP_STRINGS_H
#define _FRANKENPHP_STRINGS_H

/**
 * Cached interned strings for memory and performance benefits
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

#define S(str) frankenphp_init_persistent_string(str, sizeof(str) - 1)

static frankenphp_interned_strings_t frankenphp_init_interned_strings() {
  return (frankenphp_interned_strings_t){
    .remote_addr = S("REMOTE_ADDR"),
    .remote_host = S("REMOTE_HOST"),
    .remote_port = S("REMOTE_PORT"),
    .document_root = S("DOCUMENT_ROOT"),
    .path_info = S("PATH_INFO"),
    .php_self = S("PHP_SELF"),
    .document_uri = S("DOCUMENT_URI"),
    .script_filename = S("SCRIPT_FILENAME"),
    .script_name = S("SCRIPT_NAME"),
    .https = S("HTTPS"),
    .ssl_protocol = S("SSL_PROTOCOL"),
    .request_scheme = S("REQUEST_SCHEME"),
    .server_name = S("SERVER_NAME"),
    .server_port = S("SERVER_PORT"),
    .content_length = S("CONTENT_LENGTH"),
    .server_protocol = S("SERVER_PROTOCOL"),
    .http_host = S("HTTP_HOST"),
    .request_uri = S("REQUEST_URI"),
    .ssl_cipher = S("SSL_CIPHER"),
    .server_software = S("SERVER_SOFTWARE"),
    .server_software_str = S("FrankenPHP"),
    .gateway_interface = S("GATEWAY_INTERFACE"),
    .gateway_interface_str = S("CGI/1.1"),
    .auth_type = S("AUTH_TYPE"),
    .remote_ident = S("REMOTE_IDENT"),
    .content_type = S("CONTENT_TYPE"),
    .path_translated = S("PATH_TRANSLATED"),
    .query_string = S("QUERY_STRING"),
    .remote_user = S("REMOTE_USER"),
    .request_method = S("REQUEST_METHOD"),
  };
}

#undef S

#endif
