#include "types.h"

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

void __efree__(void *ptr) { efree(ptr); }

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

int __zend_is_callable__(zval *cb) { return zend_is_callable(cb, 0, NULL); }

int __call_user_function__(zval *function_name, zval *retval,
                           uint32_t param_count, zval params[]) {
  return call_user_function(CG(function_table), NULL, function_name, retval,
                            param_count, params);
}
