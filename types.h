#ifndef TYPES_H
#define TYPES_H

#include <zend.h>
#include <zend_hash.h>
#include <zend_API.h>
#include <zend_types.h>
#include <zend_alloc.h>

zval* get_ht_packed_data(HashTable*, uint32_t index);
Bucket* get_ht_bucket_data(HashTable*, uint32_t index);

static inline void* emalloc_wrapper(size_t size) {
    return emalloc(size);
}

static inline void zend_hash_init_wrapper(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor, bool persistent) {
    zend_hash_init(ht, nSize, null, pDestructor, persistent);
}

#endif
