#ifndef _FRANKENPHP_STRINGS_H
#define _FRANKENPHP_STRINGS_H

#include <Zend/zend.h>
#include <Zend/zend_types.h>
#include <php.h>
#include <php_config.h>

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
  zend_string *auth_user;
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

frankenphp_interned_strings_t frankenphp_init_interned_strings() {
    frankenphp_interned_strings_t interned_strings = {0};
	interned_strings.remote_addr = frankenphp_init_persistent_string("REMOTE_ADDR", sizeof("REMOTE_ADDR") - 1);
	interned_strings.remote_host = frankenphp_init_persistent_string("REMOTE_HOST", sizeof("REMOTE_HOST") - 1);
	interned_strings.remote_port = frankenphp_init_persistent_string("REMOTE_PORT", sizeof("REMOTE_PORT") - 1);
	interned_strings.document_root = frankenphp_init_persistent_string("DOCUMENT_ROOT", sizeof("DOCUMENT_ROOT") - 1);
	interned_strings.path_info = frankenphp_init_persistent_string("PATH_INFO", sizeof("PATH_INFO") - 1);
	interned_strings.php_self = frankenphp_init_persistent_string("PHP_SELF", sizeof("PHP_SELF") - 1);
	interned_strings.document_uri = frankenphp_init_persistent_string("DOCUMENT_URI", sizeof("DOCUMENT_URI") - 1);
	interned_strings.script_filename = frankenphp_init_persistent_string("SCRIPT_FILENAME", sizeof("SCRIPT_FILENAME") - 1);
	interned_strings.script_name = frankenphp_init_persistent_string("SCRIPT_NAME", sizeof("SCRIPT_NAME") - 1);
	interned_strings.https = frankenphp_init_persistent_string("HTTPS", sizeof("HTTPS") - 1);
	interned_strings.ssl_protocol = frankenphp_init_persistent_string("SSL_PROTOCOL", sizeof("SSL_PROTOCOL") - 1);
	interned_strings.request_scheme = frankenphp_init_persistent_string("REQUEST_SCHEME", sizeof("REQUEST_SCHEME") - 1);
	interned_strings.server_name = frankenphp_init_persistent_string("SERVER_NAME", sizeof("SERVER_NAME") - 1);
	interned_strings.server_port = frankenphp_init_persistent_string("SERVER_PORT", sizeof("SERVER_PORT") - 1);
	interned_strings.content_length = frankenphp_init_persistent_string("CONTENT_LENGTH", sizeof("CONTENT_LENGTH") - 1);
	interned_strings.server_protocol = frankenphp_init_persistent_string("SERVER_PROTOCOL", sizeof("SERVER_PROTOCOL") - 1);
	interned_strings.http_host = frankenphp_init_persistent_string("HTTP_HOST", sizeof("HTTP_HOST") - 1);
	interned_strings.request_uri = frankenphp_init_persistent_string("REQUEST_URI", sizeof("REQUEST_URI") - 1);
	interned_strings.ssl_cipher = frankenphp_init_persistent_string("SSL_CIPHER", sizeof("SSL_CIPHER") - 1);
	interned_strings.server_software = frankenphp_init_persistent_string("SERVER_SOFTWARE", sizeof("SERVER_SOFTWARE") - 1);
	interned_strings.server_software_str = frankenphp_init_persistent_string("FrankenPHP", sizeof("FrankenPHP") - 1);
	interned_strings.gateway_interface = frankenphp_init_persistent_string("GATEWAY_INTERFACE", sizeof("GATEWAY_INTERFACE") - 1);
	interned_strings.gateway_interface_str = frankenphp_init_persistent_string("CGI/1.1", sizeof("CGI/1.1") - 1);
	interned_strings.auth_type = frankenphp_init_persistent_string("AUTH_TYPE", sizeof("AUTH_TYPE") - 1);
	interned_strings.remote_ident = frankenphp_init_persistent_string("REMOTE_IDENT", sizeof("REMOTE_IDENT") - 1);
	interned_strings.content_type = frankenphp_init_persistent_string("CONTENT_TYPE", sizeof("CONTENT_TYPE") - 1);
	interned_strings.path_translated = frankenphp_init_persistent_string("PATH_TRANSLATED", sizeof("PATH_TRANSLATED") - 1);
	interned_strings.query_string = frankenphp_init_persistent_string("QUERY_STRING", sizeof("QUERY_STRING") - 1);
	interned_strings.auth_user = frankenphp_init_persistent_string("AUTH_USER", sizeof("AUTH_USER") - 1);
	interned_strings.request_method = frankenphp_init_persistent_string("REQUEST_METHOD", sizeof("REQUEST_METHOD") - 1);

	return interned_strings;
}

#endif
