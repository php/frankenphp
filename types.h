#ifndef TYPES_H
#define TYPES_H

#include <Zend/zend.h>
#include <Zend/zend_API.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_hash.h>
#include <Zend/zend_types.h>

zval *get_ht_packed_data(HashTable *, uint32_t index);
Bucket *get_ht_bucket(HashTable *);

void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent);

zend_array *__zend_new_array__(uint32_t size);

zend_array *zend_hash_bulk_insert(
	zend_array *arr, size_t num_entries,
	char *key1, char *key2, char *key3,
	size_t key_len1, size_t key_len2, size_t key_len3,
	zval *val1, zval *val2, zval *val3
);

#endif
