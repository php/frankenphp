#include "types.h"
#include <Zend/zend_smart_str.h>

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
zend_string *__zval_serialize__(zend_object *obj) {
  // find serialize in global function table and call it
  zval zv;
  ZVAL_OBJ(&zv, obj);
  zval func;
  ZVAL_STRING(&func, "serialize");
  zval retval;
  zval params[1];
  params[0] = zv;
  if (call_user_function(EG(function_table), NULL, &func, &retval, 1, params) != SUCCESS) {
	zval_ptr_dtor(&func);
	return NULL;
  }
  zval_ptr_dtor(&func);

  // pemalloc the return value
  zend_string *result = zend_string_dup(Z_STR(retval), 1);
  zval_ptr_dtor(&retval);

  return result;
}

void __zval_unserialize__(zval *retval, zend_string *str) {
  // find unserialize in global function table and call it
  zval func;
  ZVAL_STRING(&func, "unserialize");
  zval params[1];

  ZVAL_STR(&params[0], str);
  if (call_user_function(EG(function_table), NULL, &func, retval, 1, params) != SUCCESS) {
	ZVAL_NULL(retval);
  }
  zval_ptr_dtor(&func);
  zend_string_release(str);
}

zval *__init_zval__() {
  zval *zv = (zval *)emalloc(sizeof(zval));
  return zv;
}

zend_object *__php_object_init__(
    zval *zv,
    const char *class_name,
    size_t class_name_len,
    zend_class_entry *ce // optional: pass NULL to look up by name
) {
    if (!ce) {
        zend_string *name = zend_string_init_interned(class_name, class_name_len, 1);
        ce = zend_lookup_class(name);
        if (!ce) {
            return NULL;
        }
    }

    object_init_ex(zv, ce);

    return Z_OBJ_P(zv);
}