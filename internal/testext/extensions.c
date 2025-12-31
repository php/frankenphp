#include <php.h>
#include <zend_exceptions.h>

#include "_cgo_export.h"

zend_module_entry module1_entry = {STANDARD_MODULE_HEADER,
                                   "ext1",
                                   NULL, /* Functions */
                                   NULL, /* MINIT */
                                   NULL, /* MSHUTDOWN */
                                   NULL, /* RINIT */
                                   NULL, /* RSHUTDOWN */
                                   NULL, /* MINFO */
                                   "0.1.0",
                                   STANDARD_MODULE_PROPERTIES};

zend_module_entry module2_entry = {STANDARD_MODULE_HEADER,
                                   "ext2",
                                   NULL, /* Functions */
                                   NULL, /* MINIT */
                                   NULL, /* MSHUTDOWN */
                                   NULL, /* RINIT */
                                   NULL, /* RSHUTDOWN */
                                   NULL, /* MINFO */
                                   "0.1.0",
                                   STANDARD_MODULE_PROPERTIES};
