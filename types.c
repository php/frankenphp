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

int __zend_is_callable__(zval *cb) { return zend_is_callable(cb, 0, NULL); }

int __call_user_function__(zval *function_name, zval *retval,
                           uint32_t param_count, zval params[]) {
  return call_user_function(CG(function_table), NULL, function_name, retval,
                            param_count, params);
}

void __zval_long__(zval *z, zend_long l) { ZVAL_LONG(z, l); }

void __zval_double__(zval *z, double d) { ZVAL_DOUBLE(z, d); }

void __zval_string__(zval *z, const char *s) { ZVAL_STRING(z, s); }

void __zval_bool__(zval *z, bool b) { ZVAL_BOOL(z, b); }

void __zval_null__(zval *z) { ZVAL_NULL(z); }
