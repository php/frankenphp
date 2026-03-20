/* bg_worker_vars.h - Persistent zval helpers for background worker variables.
 *
 * Handles validation, deep-copy to/from persistent memory, immutable array
 * detection, interned string optimization, and enum serialization.
 *
 * Included by frankenphp.c - not a standalone compilation unit. */

#ifndef BG_WORKER_VARS_H
#define BG_WORKER_VARS_H

typedef struct {
  zend_string *class_name;
  zend_string *case_name;
} bg_worker_enum_t;

/* Forward declarations */
static void bg_worker_free_persistent_zval(zval *z);

static void bg_worker_request_copy_zval(zval *dst, zval *src);

/* Check if a HashTable is an opcache immutable array - safe to share
 * across threads without copying. */
static bool bg_worker_is_immutable(HashTable *ht) {
  return (GC_FLAGS(ht) & IS_ARRAY_IMMUTABLE) != 0;
}

/* Free a stored vars pointer only if it's a persistent copy (not immutable). */
static void bg_worker_free_stored_vars(void *ptr) {
  if (ptr != NULL) {
    HashTable *ht = (HashTable *)ptr;
    if (!bg_worker_is_immutable(ht)) {
      zval z;
      ZVAL_ARR(&z, ht);
      bg_worker_free_persistent_zval(&z);
    }
  }
}

/* Copy or reference a stored vars pointer to request memory.
 * Immutable arrays are returned as zero-copy references. */
static void bg_worker_read_stored_vars(zval *dst, void *ptr) {
  HashTable *ht = (HashTable *)ptr;
  if (bg_worker_is_immutable(ht)) {
    ZVAL_ARR(dst, ht); /* zero-copy: immutable = safe to share */
  } else {
    zval src;
    ZVAL_ARR(&src, ht);
    bg_worker_request_copy_zval(dst, &src);
  }
}

/* Validate that a zval tree contains only scalars, arrays, and enums */
static bool bg_worker_validate_zval(zval *z) {
  switch (Z_TYPE_P(z)) {
  case IS_NULL:
  case IS_FALSE:
  case IS_TRUE:
  case IS_LONG:
  case IS_DOUBLE:
  case IS_STRING:
    return true;
  case IS_OBJECT:
    return (Z_OBJCE_P(z)->ce_flags & ZEND_ACC_ENUM) != 0;
  case IS_ARRAY: {
    zval *val;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(z), val) {
      if (!bg_worker_validate_zval(val))
        return false;
    }
    ZEND_HASH_FOREACH_END();
    return true;
  }
  default:
    return false;
  }
}

/* Deep-copy a zval into persistent memory */
static void bg_worker_persist_zval(zval *dst, zval *src) {
  switch (Z_TYPE_P(src)) {
  case IS_NULL:
  case IS_FALSE:
  case IS_TRUE:
    ZVAL_COPY_VALUE(dst, src);
    break;
  case IS_LONG:
    ZVAL_LONG(dst, Z_LVAL_P(src));
    break;
  case IS_DOUBLE:
    ZVAL_DOUBLE(dst, Z_DVAL_P(src));
    break;
  case IS_STRING: {
    zend_string *s = Z_STR_P(src);
    if (ZSTR_IS_INTERNED(s)) {
      ZVAL_STR(dst, s); /* interned = shared memory, no copy needed */
    } else {
      ZVAL_NEW_STR(dst, zend_string_init(ZSTR_VAL(s), ZSTR_LEN(s), 1));
    }
    break;
  }
  case IS_OBJECT: {
    /* Must be an enum (validated earlier) */
    zend_class_entry *ce = Z_OBJCE_P(src);
    bg_worker_enum_t *e = pemalloc(sizeof(bg_worker_enum_t), 1);
    e->class_name =
        ZSTR_IS_INTERNED(ce->name)
            ? ce->name
            : zend_string_init(ZSTR_VAL(ce->name), ZSTR_LEN(ce->name), 1);
    zval *case_name_zval = zend_enum_fetch_case_name(Z_OBJ_P(src));
    zend_string *case_str = Z_STR_P(case_name_zval);
    e->case_name =
        ZSTR_IS_INTERNED(case_str)
            ? case_str
            : zend_string_init(ZSTR_VAL(case_str), ZSTR_LEN(case_str), 1);
    ZVAL_PTR(dst, e);
    break;
  }
  case IS_ARRAY: {
    HashTable *src_ht = Z_ARRVAL_P(src);
    HashTable *dst_ht = pemalloc(sizeof(HashTable), 1);
    zend_hash_init(dst_ht, zend_hash_num_elements(src_ht), NULL, NULL, 1);
    ZVAL_ARR(dst, dst_ht);

    zend_string *key;
    zend_ulong idx;
    zval *val;
    ZEND_HASH_FOREACH_KEY_VAL(src_ht, idx, key, val) {
      zval pval;
      bg_worker_persist_zval(&pval, val);
      if (key) {
        if (ZSTR_IS_INTERNED(key)) {
          zend_hash_add_new(dst_ht, key, &pval);
        } else {
          zend_string *pkey = zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 1);
          zend_hash_add_new(dst_ht, pkey, &pval);
          zend_string_release(pkey);
        }
      } else {
        zend_hash_index_add_new(dst_ht, idx, &pval);
      }
    }
    ZEND_HASH_FOREACH_END();
    break;
  }
  default:
    ZVAL_NULL(dst);
    break;
  }
}

/* Deep-free a persistent zval tree */
static void bg_worker_free_persistent_zval(zval *z) {
  switch (Z_TYPE_P(z)) {
  case IS_STRING:
    if (!ZSTR_IS_INTERNED(Z_STR_P(z))) {
      zend_string_free(Z_STR_P(z));
    }
    break;
  case IS_PTR: {
    bg_worker_enum_t *e = (bg_worker_enum_t *)Z_PTR_P(z);
    if (!ZSTR_IS_INTERNED(e->class_name))
      zend_string_free(e->class_name);
    if (!ZSTR_IS_INTERNED(e->case_name))
      zend_string_free(e->case_name);
    pefree(e, 1);
    break;
  }
  case IS_ARRAY: {
    zval *val;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(z), val) {
      bg_worker_free_persistent_zval(val);
    }
    ZEND_HASH_FOREACH_END();
    zend_hash_destroy(Z_ARRVAL_P(z));
    pefree(Z_ARRVAL_P(z), 1);
    break;
  }
  default:
    break;
  }
}

/* Deep-copy a persistent zval tree into request memory */
static void bg_worker_request_copy_zval(zval *dst, zval *src) {
  switch (Z_TYPE_P(src)) {
  case IS_NULL:
  case IS_FALSE:
  case IS_TRUE:
    ZVAL_COPY_VALUE(dst, src);
    break;
  case IS_LONG:
    ZVAL_LONG(dst, Z_LVAL_P(src));
    break;
  case IS_DOUBLE:
    ZVAL_DOUBLE(dst, Z_DVAL_P(src));
    break;
  case IS_STRING:
    if (ZSTR_IS_INTERNED(Z_STR_P(src))) {
      ZVAL_STR(dst, Z_STR_P(src));
    } else {
      ZVAL_STRINGL(dst, Z_STRVAL_P(src), Z_STRLEN_P(src));
    }
    break;
  case IS_PTR: {
    bg_worker_enum_t *e = (bg_worker_enum_t *)Z_PTR_P(src);
    zend_class_entry *ce = zend_lookup_class(e->class_name);
    if (!ce || !(ce->ce_flags & ZEND_ACC_ENUM)) {
      zend_throw_exception_ex(spl_ce_LogicException, 0,
                              "Background worker enum class \"%s\" not found",
                              ZSTR_VAL(e->class_name));
      ZVAL_NULL(dst);
      break;
    }
    zend_object *enum_obj = zend_enum_get_case_cstr(ce, ZSTR_VAL(e->case_name));
    if (!enum_obj) {
      zend_throw_exception_ex(
          spl_ce_LogicException, 0,
          "Background worker enum case \"%s::%s\" not found",
          ZSTR_VAL(e->class_name), ZSTR_VAL(e->case_name));
      ZVAL_NULL(dst);
      break;
    }
    ZVAL_OBJ_COPY(dst, enum_obj);
    break;
  }
  case IS_ARRAY: {
    HashTable *src_ht = Z_ARRVAL_P(src);
    array_init_size(dst, zend_hash_num_elements(src_ht));
    HashTable *dst_ht = Z_ARRVAL_P(dst);

    zend_string *key;
    zend_ulong idx;
    zval *val;
    ZEND_HASH_FOREACH_KEY_VAL(src_ht, idx, key, val) {
      zval rval;
      bg_worker_request_copy_zval(&rval, val);
      if (EG(exception)) {
        zval_ptr_dtor(&rval);
        break;
      }
      if (key) {
        if (ZSTR_IS_INTERNED(key)) {
          zend_hash_add_new(dst_ht, key, &rval);
        } else {
          zend_string *rkey = zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 0);
          ZSTR_H(rkey) = ZSTR_H(key);
          zend_hash_add_new(dst_ht, rkey, &rval);
          zend_string_release(rkey);
        }
      } else {
        zend_hash_index_add_new(dst_ht, idx, &rval);
      }
    }
    ZEND_HASH_FOREACH_END();
    break;
  }
  default:
    ZVAL_NULL(dst);
    break;
  }
}

/* Move a persistent zval tree into request memory, freeing persistent data
 * in one pass. Combines bg_worker_request_copy_zval +
 * bg_worker_free_persistent_zval. */
static void bg_worker_move_zval(zval *dst, zval *src) {
  switch (Z_TYPE_P(src)) {
  case IS_NULL:
  case IS_FALSE:
  case IS_TRUE:
    ZVAL_COPY_VALUE(dst, src);
    break;
  case IS_LONG:
    ZVAL_LONG(dst, Z_LVAL_P(src));
    break;
  case IS_DOUBLE:
    ZVAL_DOUBLE(dst, Z_DVAL_P(src));
    break;
  case IS_STRING:
    if (ZSTR_IS_INTERNED(Z_STR_P(src))) {
      ZVAL_STR(dst, Z_STR_P(src)); /* zero-copy, no free */
    } else {
      ZVAL_STRINGL(dst, Z_STRVAL_P(src), Z_STRLEN_P(src));
      zend_string_free(Z_STR_P(src));
    }
    break;
  case IS_PTR: {
    bg_worker_enum_t *e = (bg_worker_enum_t *)Z_PTR_P(src);
    zend_class_entry *ce = zend_lookup_class(e->class_name);
    if (!ce || !(ce->ce_flags & ZEND_ACC_ENUM)) {
      zend_throw_exception_ex(spl_ce_LogicException, 0,
                              "Background worker enum class \"%s\" not found",
                              ZSTR_VAL(e->class_name));
      ZVAL_NULL(dst);
    } else {
      zend_object *enum_obj =
          zend_enum_get_case_cstr(ce, ZSTR_VAL(e->case_name));
      if (!enum_obj) {
        zend_throw_exception_ex(
            spl_ce_LogicException, 0,
            "Background worker enum case \"%s::%s\" not found",
            ZSTR_VAL(e->class_name), ZSTR_VAL(e->case_name));
        ZVAL_NULL(dst);
      } else {
        ZVAL_OBJ_COPY(dst, enum_obj);
      }
    }
    if (!ZSTR_IS_INTERNED(e->class_name))
      zend_string_free(e->class_name);
    if (!ZSTR_IS_INTERNED(e->case_name))
      zend_string_free(e->case_name);
    pefree(e, 1);
    break;
  }
  case IS_ARRAY: {
    HashTable *src_ht = Z_ARRVAL_P(src);
    array_init_size(dst, zend_hash_num_elements(src_ht));
    HashTable *dst_ht = Z_ARRVAL_P(dst);

    zend_string *key;
    zend_ulong idx;
    zval *val;
    bool move_failed = false;
    ZEND_HASH_FOREACH_KEY_VAL(src_ht, idx, key, val) {
      if (move_failed) {
        /* Free remaining persistent entries that were not moved */
        bg_worker_free_persistent_zval(val);
        continue;
      }
      zval rval;
      bg_worker_move_zval(&rval, val);
      if (EG(exception)) {
        zval_ptr_dtor(&rval);
        move_failed = true;
        continue;
      }
      if (key) {
        if (ZSTR_IS_INTERNED(key)) {
          zend_hash_add_new(dst_ht, key, &rval);
        } else {
          zend_string *rkey =
              zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 0);
          ZSTR_H(rkey) = ZSTR_H(key);
          zend_hash_add_new(dst_ht, rkey, &rval);
          zend_string_release(rkey);
        }
      } else {
        zend_hash_index_add_new(dst_ht, idx, &rval);
      }
    }
    ZEND_HASH_FOREACH_END();
    zend_hash_destroy(src_ht);
    pefree(src_ht, 1);
    break;
  }
  default:
    ZVAL_NULL(dst);
    break;
  }
}

#endif /* BG_WORKER_VARS_H */
