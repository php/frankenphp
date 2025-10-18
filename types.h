#ifndef TYPES_H
#define TYPES_H

#include <Zend/zend.h>
#include <Zend/zend_API.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_hash.h>
#include <Zend/zend_types.h>

zval *get_ht_packed_data(HashTable *, uint32_t index);
Bucket *get_ht_bucket(HashTable *);

void *__emalloc__(size_t size);
void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent);

void __zval_null__(zval *zv);
void __zval_bool__(zval *zv, bool val);
void __zval_long__(zval *zv, zend_long val);
void __zval_double__(zval *zv, double val);
void __zval_string__(zval *zv, zend_string *str);
void __zval_empty_string__(zval *zv);
void __zval_arr__(zval *zv, zend_array *arr);
zend_array *__zend_new_array__(uint32_t size);

bool is_internal_class(zend_class_entry *entry);
zend_string *__zval_serialize__(zend_object *obj);
void __zval_unserialize__(zval *retval, zend_string *str);
zval *__init_zval__();
zend_object *__php_object_init__(
    zval *zv,
    const char *class_name,
    size_t class_name_len,
    zend_class_entry *ce
);

#endif
