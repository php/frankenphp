#include "extension.h"
#include "_cgo_export.h"

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_native_cli_test, 0, 0, IS_LONG,
                                        0)
ZEND_END_ARG_INFO()

PHP_FUNCTION(frankenphp_native_test) {
  ZEND_PARSE_PARAMETERS_NONE();
  RETURN_LONG(go_frankenphp_native_test());
}

static const zend_function_entry native_cli_test_functions[] = {
    PHP_FE(frankenphp_native_test, arginfo_native_cli_test) PHP_FE_END};

zend_module_entry native_cli_test_module = {STANDARD_MODULE_HEADER,
                                            "native_cli_test",
                                            native_cli_test_functions,
                                            NULL,
                                            NULL,
                                            NULL,
                                            NULL,
                                            NULL,
                                            "1.0.0",
                                            STANDARD_MODULE_PROPERTIES};
