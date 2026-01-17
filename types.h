#ifndef TYPES_H
#define TYPES_H

#include "frankenphp.h"
#include <Zend/zend.h>
#include <Zend/zend_API.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_hash.h>

zval *get_ht_packed_data(HashTable *, uint32_t index);
Bucket *get_ht_bucket_data(HashTable *, uint32_t index);

void *__emalloc__(size_t size);
void __efree__(void *ptr);
void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent);

int __zend_is_callable__(zval *cb);
int __call_user_function__(zval *function_name, zval *retval,
                           uint32_t param_count, zval params[]);

void __zval_null__(zval *zv);
void __zval_bool__(zval *zv, bool val);
void __zval_long__(zval *zv, zend_long val);
void __zval_double__(zval *zv, double val);
void __zval_string__(zval *zv, zend_string *str);
void __zval_empty_string__(zval *zv);
void __zval_arr__(zval *zv, zend_array *arr);
zend_array *__zend_new_array__(uint32_t size);

#endif
