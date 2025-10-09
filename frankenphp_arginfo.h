/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: c5318079b1c5629258a1f4b682c7aaf327588b71 */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_handle_request, 0, 1,
                                        _IS_BOOL, 0)
ZEND_ARG_TYPE_INFO(0, callback, IS_CALLABLE, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_headers_send, 0, 0, IS_LONG, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, status, IS_LONG, 0, "200")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_finish_request, 0, 0,
                                        _IS_BOOL, 0)
ZEND_END_ARG_INFO()

#define arginfo_fastcgi_finish_request arginfo_frankenphp_finish_request

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_request_headers, 0,
                                        0, IS_ARRAY, 0)
ZEND_END_ARG_INFO()

#define arginfo_apache_request_headers arginfo_frankenphp_request_headers

#define arginfo_getallheaders arginfo_frankenphp_request_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_MASK_EX(arginfo_frankenphp_response_headers, 0,
                                        0, MAY_BE_ARRAY | MAY_BE_BOOL)
ZEND_END_ARG_INFO()

#define arginfo_apache_response_headers arginfo_frankenphp_response_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(
    arginfo_frankenphp_set_caddy_placeholder, 0, 2, IS_VOID, 0)
ZEND_ARG_TYPE_INFO(0, key, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, value, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);
ZEND_FUNCTION(frankenphp_request_headers);
ZEND_FUNCTION(frankenphp_response_headers);
ZEND_FUNCTION(frankenphp_set_caddy_placeholder);

// clang-format off
static const zend_function_entry ext_functions[] = {
  ZEND_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
  ZEND_FE(headers_send, arginfo_headers_send)
  ZEND_FE(frankenphp_finish_request, arginfo_frankenphp_finish_request)
  ZEND_FALIAS(fastcgi_finish_request, frankenphp_finish_request, arginfo_fastcgi_finish_request)
  ZEND_FE(frankenphp_request_headers, arginfo_frankenphp_request_headers)
  ZEND_FALIAS(apache_request_headers, frankenphp_request_headers, arginfo_apache_request_headers)
  ZEND_FALIAS(getallheaders, frankenphp_request_headers, arginfo_getallheaders)
  ZEND_FE(frankenphp_response_headers, arginfo_frankenphp_response_headers)
  ZEND_FALIAS(apache_response_headers, frankenphp_response_headers, arginfo_apache_response_headers)
  ZEND_FE(frankenphp_set_caddy_placeholder, arginfo_frankenphp_set_caddy_placeholder)
  ZEND_FE_END
};
// clang-format on
