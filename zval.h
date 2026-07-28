/* zval.h - Deep-copy zval trees to and from persistent memory.
 *
 * Provides a small, self-contained toolkit for moving zval trees across
 * thread boundaries. The supported shape is a whitelist: scalars, arrays,
 * and enums. Everything else is rejected by persistent_zval_validate so
 * callers can fail fast before allocating.
 *
 * Fast paths:
 *   - Interned strings: shared memory, no copy.
 *   - Opcache-immutable arrays: shared pointer, no copy, no free.
 *
 * Included by frankenphp.c; not a standalone compilation unit. */

#ifndef _ZVAL_H
#define _ZVAL_H

#include <Zend/zend_enum.h>

/* Enum payload stored in persistent memory: the class name + case name
 * are kept as persistent zend_strings and the case object is re-resolved
 * via zend_lookup_class + zend_enum_get_case_cstr on each read. */
typedef struct {
  zend_string *class_name;
  zend_string *case_name;
} persistent_zval_enum_t;

/* Whitelist check: only scalars, arrays of allowed values, and enum
 * instances pass. Returns false for objects other than enums, resources,
 * closures, references, etc. */
static bool persistent_zval_validate(zval *z) {
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
    /* Opcache-immutable arrays are compile-time constants in shared
     * memory; their leaves are guaranteed scalars or further immutable
     * arrays. The copy/free paths below already trust this flag, so a
     * recursive walk here would just be cycles. */
    if ((GC_FLAGS(Z_ARRVAL_P(z)) & IS_ARRAY_IMMUTABLE) != 0)
      return true;
    zval *val;
    ZEND_HASH_FOREACH_VAL(Z_ARRVAL_P(z), val) {
      if (!persistent_zval_validate(val))
        return false;
    }
    ZEND_HASH_FOREACH_END();
    return true;
  }
  default:
    return false;
  }
}

/* Deep-copy a zval from request memory into persistent (pemalloc) memory.
 * Callers must have already passed persistent_zval_validate on src.
 *
 * Storage convention for enums: dst becomes IS_PTR holding a
 * persistent_zval_enum_t. This is an internal representation; the caller
 * should never expose a persistent zval to PHP directly, only via
 * persistent_zval_to_request. */
static void persistent_zval_persist(zval *dst, zval *src) {
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
      ZVAL_STR(dst, s); /* interned strings live process-wide */
    } else {
      ZVAL_NEW_STR(dst, zend_string_init(ZSTR_VAL(s), ZSTR_LEN(s), 1));
    }
    break;
  }
  case IS_OBJECT: {
    /* Must be an enum (validated earlier in persistent_zval_validate). */
    zend_class_entry *ce = Z_OBJCE_P(src);
    persistent_zval_enum_t *e = pemalloc(sizeof(*e), 1);
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
    if ((GC_FLAGS(src_ht) & IS_ARRAY_IMMUTABLE) != 0) {
      /* Opcache-immutable arrays live for the process lifetime and are
       * safe to share across threads by pointer. Zero-copy, zero-free. */
      ZVAL_ARR(dst, src_ht);
      break;
    }
    HashTable *dst_ht = pemalloc(sizeof(HashTable), 1);
    zend_hash_init(dst_ht, zend_hash_num_elements(src_ht), NULL, NULL, 1);
    ZVAL_ARR(dst, dst_ht);

    zend_string *key;
    zend_ulong idx;
    zval *val;
    ZEND_HASH_FOREACH_KEY_VAL(src_ht, idx, key, val) {
      zval pval;
      persistent_zval_persist(&pval, val);
      if (key) {
        if (ZSTR_IS_INTERNED(key)) {
          zend_hash_add_new(dst_ht, key, &pval);
        } else {
          zend_string *pkey = zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 1);
          /* Iteration guarantees the source key has its hash set.
           * Propagating it lets zend_hash_add_new skip the re-hash. */
          ZSTR_H(pkey) = ZSTR_H(key);
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
    /* Unreachable: persistent_zval_validate is the gatekeeper. */
    ZEND_UNREACHABLE();
  }
}

/* Deep-free a persistent zval tree. Idempotent on scalars. Skips
 * interned strings and immutable arrays (they are borrowed, not owned). */
static void persistent_zval_free(zval *z) {
  switch (Z_TYPE_P(z)) {
  case IS_STRING:
    if (!ZSTR_IS_INTERNED(Z_STR_P(z))) {
      zend_string_free(Z_STR_P(z));
    }
    break;
  case IS_PTR: {
    persistent_zval_enum_t *e = (persistent_zval_enum_t *)Z_PTR_P(z);
    if (!ZSTR_IS_INTERNED(e->class_name))
      zend_string_free(e->class_name);
    if (!ZSTR_IS_INTERNED(e->case_name))
      zend_string_free(e->case_name);
    pefree(e, 1);
    break;
  }
  case IS_ARRAY: {
    HashTable *ht = Z_ARRVAL_P(z);
    if ((GC_FLAGS(ht) & IS_ARRAY_IMMUTABLE) != 0) {
      /* Borrowed from opcache, do not touch. */
      break;
    }
    zval *val;
    ZEND_HASH_FOREACH_VAL(ht, val) { persistent_zval_free(val); }
    ZEND_HASH_FOREACH_END();
    zend_hash_destroy(ht);
    pefree(ht, 1);
    break;
  }
  default:
    break;
  }
}

/* Deep-copy a persistent zval tree back into request memory. Enums are
 * resolved from their class+case names on each call. If the enum class
 * or case can't be found in the current thread's class table, an
 * exception is thrown and dst is set to IS_NULL. */
static void persistent_zval_to_request(zval *dst, zval *src) {
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
    persistent_zval_enum_t *e = (persistent_zval_enum_t *)Z_PTR_P(src);
    zend_class_entry *ce = zend_lookup_class(e->class_name);
    if (EG(exception)) {
      /* Autoloader threw; let that exception propagate untouched. */
      ZVAL_NULL(dst);
      break;
    }
    if (!ce || !(ce->ce_flags & ZEND_ACC_ENUM)) {
      zend_throw_exception_ex(spl_ce_LogicException, 0,
                              "persistent_zval: enum class \"%s\" not found",
                              ZSTR_VAL(e->class_name));
      ZVAL_NULL(dst);
      break;
    }
    zend_object *enum_obj = zend_enum_get_case_cstr(ce, ZSTR_VAL(e->case_name));
    if (!enum_obj) {
      zend_throw_exception_ex(spl_ce_LogicException, 0,
                              "persistent_zval: enum case \"%s::%s\" not found",
                              ZSTR_VAL(e->class_name), ZSTR_VAL(e->case_name));
      ZVAL_NULL(dst);
      break;
    }
    ZVAL_OBJ_COPY(dst, enum_obj);
    break;
  }
  case IS_ARRAY: {
    HashTable *src_ht = Z_ARRVAL_P(src);
    if ((GC_FLAGS(src_ht) & IS_ARRAY_IMMUTABLE) != 0) {
      /* Zero-copy: immutable arrays are safe to expose directly. */
      ZVAL_ARR(dst, src_ht);
      break;
    }
    array_init_size(dst, zend_hash_num_elements(src_ht));
    HashTable *dst_ht = Z_ARRVAL_P(dst);

    zend_string *key;
    zend_ulong idx;
    zval *val;
    ZEND_HASH_FOREACH_KEY_VAL(src_ht, idx, key, val) {
      zval rval;
      persistent_zval_to_request(&rval, val);
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
    /* Unreachable: only types produced by persistent_zval_persist land here. */
    ZEND_UNREACHABLE();
  }
}

#endif /* _ZVAL_H */
