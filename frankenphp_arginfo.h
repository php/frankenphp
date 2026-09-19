/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: 406f78307eb4ba5dabdb9e163d60afa49268446c */

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

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_get_vars, 0, 1, IS_ARRAY, 0)
	ZEND_ARG_TYPE_INFO(0, name, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_INFO_EX(arginfo_class_FrankenPHP_WorkerHandle___construct, 0, 0, 0)
ZEND_END_ARG_INFO()

#define arginfo_class_FrankenPHP_WorkerHandle_tick arginfo_frankenphp_finish_request

#define arginfo_class_FrankenPHP_WorkerHandle_getStream arginfo_class_FrankenPHP_WorkerHandle___construct

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_class_FrankenPHP_WorkerHandle_setVars, 0, 1, IS_VOID, 0)
	ZEND_ARG_TYPE_INFO(0, vars, IS_ARRAY, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_OBJ_INFO_EX(arginfo_class_FrankenPHP_WorkerHandle_receive, 0, 0, FrankenPHP\\ReceivedTaskHandle, 1)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_INFO_EX(arginfo_class_FrankenPHP_SentTaskHandle___construct, 0, 0, 2)
	ZEND_ARG_TYPE_INFO(0, worker, IS_STRING, 0)
	ZEND_ARG_TYPE_INFO(0, payload, IS_ARRAY, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, timeout, IS_DOUBLE, 1, "30.0")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_class_FrankenPHP_SentTaskHandle_read, 0, 0, IS_ARRAY, 1)
ZEND_END_ARG_INFO()

#define arginfo_class_FrankenPHP_SentTaskHandle_getStream arginfo_class_FrankenPHP_WorkerHandle___construct

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_class_FrankenPHP_SentTaskHandle_abandon, 0, 0, IS_VOID, 0)
ZEND_END_ARG_INFO()

#define arginfo_class_FrankenPHP_ReceivedTaskHandle___construct arginfo_class_FrankenPHP_WorkerHandle___construct

#define arginfo_class_FrankenPHP_ReceivedTaskHandle_getPayload arginfo_frankenphp_request_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_class_FrankenPHP_ReceivedTaskHandle_update, 0, 1, IS_VOID, 0)
	ZEND_ARG_TYPE_INFO(0, data, IS_ARRAY, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_class_FrankenPHP_ReceivedTaskHandle_complete, 0, 0, IS_VOID, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, data, IS_ARRAY, 1, "null")
ZEND_END_ARG_INFO()

#define arginfo_class_FrankenPHP_ReceivedTaskHandle_getStream arginfo_class_FrankenPHP_WorkerHandle___construct

ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);
ZEND_FUNCTION(frankenphp_request_headers);
ZEND_FUNCTION(frankenphp_response_headers);
ZEND_FUNCTION(mercure_publish);
ZEND_FUNCTION(frankenphp_log);
ZEND_FUNCTION(frankenphp_get_vars);
ZEND_METHOD(FrankenPHP_WorkerHandle, __construct);
ZEND_METHOD(FrankenPHP_WorkerHandle, tick);
ZEND_METHOD(FrankenPHP_WorkerHandle, getStream);
ZEND_METHOD(FrankenPHP_WorkerHandle, setVars);
ZEND_METHOD(FrankenPHP_WorkerHandle, receive);
ZEND_METHOD(FrankenPHP_SentTaskHandle, __construct);
ZEND_METHOD(FrankenPHP_SentTaskHandle, read);
ZEND_METHOD(FrankenPHP_SentTaskHandle, getStream);
ZEND_METHOD(FrankenPHP_SentTaskHandle, abandon);
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, __construct);
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, getPayload);
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, update);
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, complete);
ZEND_METHOD(FrankenPHP_ReceivedTaskHandle, getStream);

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
	ZEND_FE(frankenphp_get_vars, arginfo_frankenphp_get_vars)
	ZEND_FE_END
};

static const zend_function_entry class_FrankenPHP_WorkerHandle_methods[] = {
	ZEND_ME(FrankenPHP_WorkerHandle, __construct, arginfo_class_FrankenPHP_WorkerHandle___construct, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_WorkerHandle, tick, arginfo_class_FrankenPHP_WorkerHandle_tick, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_WorkerHandle, getStream, arginfo_class_FrankenPHP_WorkerHandle_getStream, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_WorkerHandle, setVars, arginfo_class_FrankenPHP_WorkerHandle_setVars, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_WorkerHandle, receive, arginfo_class_FrankenPHP_WorkerHandle_receive, ZEND_ACC_PUBLIC)
	ZEND_FE_END
};

static const zend_function_entry class_FrankenPHP_SentTaskHandle_methods[] = {
	ZEND_ME(FrankenPHP_SentTaskHandle, __construct, arginfo_class_FrankenPHP_SentTaskHandle___construct, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_SentTaskHandle, read, arginfo_class_FrankenPHP_SentTaskHandle_read, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_SentTaskHandle, getStream, arginfo_class_FrankenPHP_SentTaskHandle_getStream, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_SentTaskHandle, abandon, arginfo_class_FrankenPHP_SentTaskHandle_abandon, ZEND_ACC_PUBLIC)
	ZEND_FE_END
};

static const zend_function_entry class_FrankenPHP_ReceivedTaskHandle_methods[] = {
	ZEND_ME(FrankenPHP_ReceivedTaskHandle, __construct, arginfo_class_FrankenPHP_ReceivedTaskHandle___construct, ZEND_ACC_PRIVATE)
	ZEND_ME(FrankenPHP_ReceivedTaskHandle, getPayload, arginfo_class_FrankenPHP_ReceivedTaskHandle_getPayload, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_ReceivedTaskHandle, update, arginfo_class_FrankenPHP_ReceivedTaskHandle_update, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_ReceivedTaskHandle, complete, arginfo_class_FrankenPHP_ReceivedTaskHandle_complete, ZEND_ACC_PUBLIC)
	ZEND_ME(FrankenPHP_ReceivedTaskHandle, getStream, arginfo_class_FrankenPHP_ReceivedTaskHandle_getStream, ZEND_ACC_PUBLIC)
	ZEND_FE_END
};

static void register_frankenphp_symbols(int module_number)
{
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_DEBUG", -4, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_INFO", 0, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_WARN", 4, CONST_PERSISTENT);
	REGISTER_LONG_CONSTANT("FRANKENPHP_LOG_LEVEL_ERROR", 8, CONST_PERSISTENT);
}

static zend_class_entry *register_class_FrankenPHP_WorkerHandle(void)
{
	zend_class_entry ce, *class_entry;

	INIT_NS_CLASS_ENTRY(ce, "FrankenPHP", "WorkerHandle", class_FrankenPHP_WorkerHandle_methods);
	class_entry = zend_register_internal_class_with_flags(&ce, NULL, ZEND_ACC_FINAL);

	return class_entry;
}

static zend_class_entry *register_class_FrankenPHP_SentTaskHandle(void)
{
	zend_class_entry ce, *class_entry;

	INIT_NS_CLASS_ENTRY(ce, "FrankenPHP", "SentTaskHandle", class_FrankenPHP_SentTaskHandle_methods);
	class_entry = zend_register_internal_class_with_flags(&ce, NULL, ZEND_ACC_FINAL|ZEND_ACC_NO_DYNAMIC_PROPERTIES|ZEND_ACC_NOT_SERIALIZABLE);

	return class_entry;
}

static zend_class_entry *register_class_FrankenPHP_ReceivedTaskHandle(void)
{
	zend_class_entry ce, *class_entry;

	INIT_NS_CLASS_ENTRY(ce, "FrankenPHP", "ReceivedTaskHandle", class_FrankenPHP_ReceivedTaskHandle_methods);
	class_entry = zend_register_internal_class_with_flags(&ce, NULL, ZEND_ACC_FINAL|ZEND_ACC_NO_DYNAMIC_PROPERTIES|ZEND_ACC_NOT_SERIALIZABLE);

	return class_entry;
}
