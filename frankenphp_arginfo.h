/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: 60f0d27c04f94d7b24c052e91ef294595a2bc421 */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_handle_request, 0, 1, _IS_BOOL, 0)
	ZEND_ARG_TYPE_INFO(0, callback, IS_CALLABLE, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_headers_send, 0, 0, IS_LONG, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, status, IS_LONG, 0, "200")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_finish_request, 0, 0, _IS_BOOL, 0)
ZEND_END_ARG_INFO()

#define arginfo_fastcgi_finish_request arginfo_frankenphp_finish_request

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_request_headers, 0, 0, IS_ARRAY, 0)
ZEND_END_ARG_INFO()

#define arginfo_apache_request_headers arginfo_frankenphp_request_headers

#define arginfo_getallheaders arginfo_frankenphp_request_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_MASK_EX(arginfo_frankenphp_response_headers, 0, 0, MAY_BE_ARRAY|MAY_BE_BOOL)
ZEND_END_ARG_INFO()

#define arginfo_apache_response_headers arginfo_frankenphp_response_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_mercure_publish, 0, 1, IS_STRING, 0)
	ZEND_ARG_TYPE_MASK(0, topics, MAY_BE_STRING|MAY_BE_ARRAY, NULL)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, data, IS_STRING, 0, "\'\'")
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, private, _IS_BOOL, 0, "false")
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, id, IS_STRING, 1, "null")
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, type, IS_STRING, 1, "null")
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, retry, IS_LONG, 1, "null")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_log, 0, 1, IS_VOID, 0)
	ZEND_ARG_TYPE_INFO(0, message, IS_STRING, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, level, IS_LONG, 0, "0")
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, context, IS_ARRAY, 0, "[]")
ZEND_END_ARG_INFO()


ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);
ZEND_FUNCTION(frankenphp_request_headers);
ZEND_FUNCTION(frankenphp_response_headers);
ZEND_FUNCTION(mercure_publish);
ZEND_FUNCTION(frankenphp_log);


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
	ZEND_FE(mercure_publish, arginfo_mercure_publish)
	ZEND_FE(frankenphp_log, arginfo_frankenphp_log)
	ZEND_FE_END
};

static void register_frankenphp_symbols(int module_number)
{
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_DEBUG", -4, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_INFO", 0, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_WARN", 4, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_ERROR", 8, CONST_PERSISTENT);
}
