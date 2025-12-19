#include "types.h"

zval *get_ht_packed_data(HashTable *ht, uint32_t index) {
  if (ht->u.flags & HASH_FLAG_PACKED) {
    return ht->arPacked;
  }
  return NULL;
}

Bucket *get_ht_bucket(HashTable *ht) {
  if (!(ht->u.flags & HASH_FLAG_PACKED)) {
    return ht->arData;
  }
  return NULL;
}

void *__emalloc__(size_t size) { return emalloc(size); }

void __efree__(void *ptr) { efree(ptr); }

void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent) {
  zend_hash_init(ht, nSize, NULL, pDestructor, persistent);
}

zend_array *__zend_new_array__(uint32_t size) { return zend_new_array(size); }

/* Returns existing interned string or creates a new zend_string. */
zend_string *__zend_string_init_existing_interned__(const char *str,
                                                    size_t size,
                                                    bool permanent) {
  return zend_string_init(str, size, permanent);
  // TODO: use this once it's possible to test the behavior
  // return zend_string_init_existing_interned(str, size, permanent);
}

zend_array *zend_hash_bulk_insert(zend_array *arr, size_t num_entries,
                                  size_t bulk_size, char *key1, char *key2,
                                  char *key3, char *key4, size_t key_len1,
                                  size_t key_len2, size_t key_len3,
                                  size_t key_len4, zval *val1, zval *val2,
                                  zval *val3, zval *val4) {
  if (!arr) {
    arr = zend_new_array(num_entries);
  }

  zend_hash_str_update(arr, key1, key_len1, val1);
  if (bulk_size < 1) { return arr; }
  zend_hash_str_update(arr, key2, key_len2, val2);
  if (bulk_size < 2) { return arr; }
  zend_hash_str_update(arr, key3, key_len3, val3);
  if (bulk_size < 3) { return arr; }
  zend_hash_str_update(arr, key4, key_len4, val4);

  return arr;
}

zend_array *zend_hash_bulk_next_index_insert(zend_array *arr,
                                             size_t num_entries,
                                             size_t bulk_size, zval *val1,
                                             zval *val2, zval *val3,
                                             zval *val4) {
  if (!arr) {
    arr = zend_new_array(num_entries);
  }

  zend_hash_next_index_insert(arr, val1);
  if (bulk_size < 1) { return arr; }
  zend_hash_next_index_insert(arr, val2);
  if (bulk_size < 2) { return arr; }
  zend_hash_next_index_insert(arr, val3);
  if (bulk_size < 3) { return arr; }
  zend_hash_next_index_insert(arr, val4);

  return arr;
}

int __zend_is_callable__(zval *cb) { return zend_is_callable(cb, 0, NULL); }

int __call_user_function__(zval *function_name, zval *retval,
                           uint32_t param_count, zval params[]) {
  return call_user_function(CG(function_table), NULL, function_name, retval,
                            param_count, params);
}
