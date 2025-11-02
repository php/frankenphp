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

void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent) {
  zend_hash_init(ht, nSize, NULL, pDestructor, persistent);
}

zend_array *__zend_new_array__(uint32_t size) { return zend_new_array(size); }

zend_array *zend_hash_bulk_insert(
	zend_array *arr, size_t num_entries,
	char *key1, char *key2, char *key3,
	size_t key_len1, size_t key_len2, size_t key_len3,
	zval *val1, zval *val2, zval *val3
) {
  if (!arr){
    arr = zend_new_array(num_entries);
  }

  if (key1){ zend_hash_str_update(arr, key1, key_len1, val1); }
  if (key2){ zend_hash_str_update(arr, key2, key_len2, val2); }
  if (key3){ zend_hash_str_update(arr, key3, key_len3, val3); }

  return arr;
}
