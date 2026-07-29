#include "extension.h"
#include <php.h>
#include <zend_exceptions.h>

#include "_cgo_export.h"

ZEND_BEGIN_MODULE_GLOBALS(ext2)
zend_long minit_count;
ZEND_END_MODULE_GLOBALS(ext2)

ZEND_DECLARE_MODULE_GLOBALS(ext2)

#ifdef ZTS
#define EXT2_G(v) ZEND_TSRMG(ext2_globals_id, zend_ext2_globals *, v)
#else
#define EXT2_G(v) (ext2_globals.v)
#endif

static PHP_GINIT_FUNCTION(ext2) { ext2_globals->minit_count = 0; }

PHP_MINIT_FUNCTION(ext2) {
  EXT2_G(minit_count)++;
  REGISTER_LONG_CONSTANT("EXT2_MINIT_COUNT", EXT2_G(minit_count),
                         CONST_CS | CONST_PERSISTENT);

  return SUCCESS;
}

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
                                   NULL,            /* Functions */
                                   PHP_MINIT(ext2), /* MINIT */
                                   NULL,            /* MSHUTDOWN */
                                   NULL,            /* RINIT */
                                   NULL,            /* RSHUTDOWN */
                                   NULL,            /* MINFO */
                                   "0.1.0",
                                   PHP_MODULE_GLOBALS(ext2),
                                   PHP_GINIT(ext2),
                                   NULL,
                                   NULL,
                                   STANDARD_MODULE_PROPERTIES_EX};
