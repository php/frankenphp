#include "types.h"
#include <Zend/zend_smart_str.h>

zval *get_ht_packed_data(HashTable *ht, uint32_t index) {
  if (ht->u.flags & HASH_FLAG_PACKED) {
    return &ht->arPacked[index];
  }
  return NULL;
}

Bucket *get_ht_bucket_data(HashTable *ht, uint32_t index) {
  if (!(ht->u.flags & HASH_FLAG_PACKED)) {
    return &ht->arData[index];
  }
  return NULL;
}

void *__emalloc__(size_t size) { return emalloc(size); }

void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent) {
  zend_hash_init(ht, nSize, NULL, pDestructor, persistent);
}

void __zval_null__(zval *zv) { ZVAL_NULL(zv); }

void __zval_bool__(zval *zv, bool val) { ZVAL_BOOL(zv, val); }

void __zval_long__(zval *zv, zend_long val) { ZVAL_LONG(zv, val); }

void __zval_double__(zval *zv, double val) { ZVAL_DOUBLE(zv, val); }

void __zval_string__(zval *zv, zend_string *str) { ZVAL_STR(zv, str); }

void __zval_empty_string__(zval *zv) { ZVAL_EMPTY_STRING(zv); }

void __zval_arr__(zval *zv, zend_array *arr) { ZVAL_ARR(zv, arr); }

zend_array *__zend_new_array__(uint32_t size) { return zend_new_array(size); }

bool is_internal_class(zend_class_entry *entry) {
	return entry->create_object != NULL;
}

//serialize
char *__zval_serialize__(zend_object *obj) {
  // find serialize in global function table and call it
  zval zv;
  ZVAL_OBJ(&zv, obj);
  zval func;
  ZVAL_STRING(&func, "serialize");
  zval retval;
  zval params[1];
  smart_str buf = {0};
  params[0] = zv;
  if (call_user_function(EG(function_table), NULL, &func, &retval, 1, params) != SUCCESS) {
	ZVAL_NULL(&retval);
  }
  zval_ptr_dtor(&func);

  zend_string *result = smart_str_extract(&buf);
  return ZSTR_VAL(result);
}

zval *__zval_unserialize__(zval *retval, const char *buf, size_t buf_len) {
  // find unserialize in global function table and call it
  zval func;
  ZVAL_STRING(&func, "unserialize");
  zval params[1];
  ZVAL_STRINGL(&params[0], buf, buf_len);
  if (call_user_function(EG(function_table), NULL, &func, &retval, 1, params) != SUCCESS) {
	ZVAL_NULL(&retval);
  }
  zval_ptr_dtor(&func);
  zval_ptr_dtor(&params[0]);

  zval *result = emalloc(sizeof(zval));
  *result = retval;
  return result;
}
