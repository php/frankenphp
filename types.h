#ifndef TYPES_H
#define TYPES_H

#include <zend.h>
#include <zend_API.h>
#include <zend_alloc.h>
#include <zend_hash.h>
#include <zend_types.h>

zval *get_ht_packed_data(HashTable *, uint32_t index);
Bucket *get_ht_bucket_data(HashTable *, uint32_t index);

void *__emalloc__(size_t size);
void __efree__(void *ptr);
void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent);

int __zend_is_callable__(zval *cb);
int __call_user_function__(zval *function_name, zval *retval,
                           uint32_t param_count, zval params[]);

void __zval_long__(zval *z, zend_long l);
void __zval_double__(zval *z, double d);
void __zval_string__(zval *z, const char *s);
void __zval_bool__(zval *z, bool b);
void __zval_null__(zval *z);

#endif
