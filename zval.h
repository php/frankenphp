/* zval.h - Deep-copy zval trees to and from persistent memory.
 *
 * Provides a small, self-contained toolkit for moving zval trees across
 * thread boundaries. The supported shape is a whitelist: scalars, arrays,
 * and enums. Everything else is rejected by persistent_zval_validate so
 * callers can fail fast before allocating. Nesting is also capped there
 * (PERSISTENT_ZVAL_MAX_DEPTH): persist/free/to_request below recurse once
 * per nesting level with no guard of their own, so every caller MUST run
 * persistent_zval_validate first, on the full tree, before calling any of
 * them - it's the only thing standing between attacker-controlled nesting
 * depth and a native stack overflow (crashes the whole process, not just
 * one request).
 *
 * A persistent tree owns all its memory, interned strings and
 * opcache-immutable arrays included: it can outlive an engine reboot, which
 * frees the former, and an opcache restart, which rewinds the latter.
 *
 * Included by frankenphp.c; not a standalone compilation unit. */

#ifndef _ZVAL_H
#define _ZVAL_H

#include <Zend/zend_enum.h>

/* Conservative on purpose: comfortably below the depth that overflows the
 * native stack even under sanitizer builds (larger per-frame redzones), far
 * above any depth a legitimate config/state value would ever need. Matches
 * the same order of magnitude as PHP's own default nesting caps (e.g.
 * json_decode()'s default $depth of 512, Xdebug's max_nesting_level of
 * 256). */
#define PERSISTENT_ZVAL_MAX_DEPTH 256

/* Enum payload stored in persistent memory: the class name + case name
 * are kept as persistent zend_strings and the case object is re-resolved
 * via zend_lookup_class + zend_enum_get_case_cstr on each read. */
typedef struct {
  zend_string *class_name;
  zend_string *case_name;
} persistent_zval_enum_t;

/* Takes bytes out of what the copy of a tree may still use. */
static bool persistent_zval_spend(size_t *left, size_t bytes) {
  if (*left < bytes) {
    return false;
  }
  *left -= bytes;

  return true;
}

/* Whitelist check: only scalars, arrays of allowed values, and enum
 * instances pass, objects other than enums, resources, closures, references
 * etc. do not. Also enforces PERSISTENT_ZVAL_MAX_DEPTH, bailing out before
 * recursing further once hit - see the file header for why this is the only
 * place that's safe to do so - and counts the bytes the copy would take. */
static const char *persistent_zval_check(zval *z, int depth, size_t *left) {
  if (depth > PERSISTENT_ZVAL_MAX_DEPTH) {
    return "values must be nested no deeper than " ZEND_TOSTR(
        PERSISTENT_ZVAL_MAX_DEPTH) " levels";
  }
  switch (Z_TYPE_P(z)) {
  case IS_NULL:
  case IS_FALSE:
  case IS_TRUE:
  case IS_LONG:
  case IS_DOUBLE:
    return NULL;
  case IS_STRING:
    return persistent_zval_spend(left, _ZSTR_STRUCT_SIZE(Z_STRLEN_P(z)))
               ? NULL
               : "values must fit in memory_limit once copied";
  case IS_OBJECT:
    if (!(Z_OBJCE_P(z)->ce_flags & ZEND_ACC_ENUM)) {
      return "values must be null, scalars, arrays or enums";
    }
    return persistent_zval_spend(
               left, sizeof(persistent_zval_enum_t) +
                         _ZSTR_STRUCT_SIZE(ZSTR_LEN(Z_OBJCE_P(z)->name)) +
                         _ZSTR_STRUCT_SIZE(
                             Z_STRLEN_P(zend_enum_fetch_case_name(Z_OBJ_P(z)))))
               ? NULL
               : "values must fit in memory_limit once copied";
  case IS_ARRAY: {
    if (!persistent_zval_spend(left, sizeof(HashTable) +
                                         zend_hash_num_elements(Z_ARRVAL_P(z)) *
                                             sizeof(Bucket))) {
      return "values must fit in memory_limit once copied";
    }
    zend_string *key;
    zval *val;
    ZEND_HASH_FOREACH_STR_KEY_VAL(Z_ARRVAL_P(z), key, val) {
      if (key != NULL &&
          !persistent_zval_spend(left, _ZSTR_STRUCT_SIZE(ZSTR_LEN(key)))) {
        return "values must fit in memory_limit once copied";
      }
      const char *invalid = persistent_zval_check(val, depth + 1, left);
      if (invalid != NULL) {
        return invalid;
      }
    }
    ZEND_HASH_FOREACH_END();
    return NULL;
  }
  default:
    return "values must be null, scalars, arrays or enums";
  }
}

/* Why a tree cannot be persisted, NULL when it can. Its copy is malloc
 * memory, outside the memory_limit of the request making it, and every
 * occurrence of a value shared in the tree gets a copy of its own: a tree
 * whose copy would not fit in that limit is refused, which also bounds the
 * walk of a value shared many times over. */
static const char *persistent_zval_invalid(zval *z) {
  size_t left = PG(memory_limit) > 0 ? (size_t)PG(memory_limit) : SIZE_MAX;

  return persistent_zval_check(z, 0, &left);
}

static bool persistent_zval_validate(zval *z) {
  return persistent_zval_invalid(z) == NULL;
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
    ZVAL_NEW_STR(dst, zend_string_init(ZSTR_VAL(s), ZSTR_LEN(s), 1));
    break;
  }
  case IS_OBJECT: {
    /* Must be an enum (validated earlier in persistent_zval_validate). */
    zend_class_entry *ce = Z_OBJCE_P(src);
    persistent_zval_enum_t *e = pemalloc(sizeof(*e), 1);
    e->class_name = zend_string_init(ZSTR_VAL(ce->name), ZSTR_LEN(ce->name), 1);
    zval *case_name_zval = zend_enum_fetch_case_name(Z_OBJ_P(src));
    zend_string *case_str = Z_STR_P(case_name_zval);
    e->case_name = zend_string_init(ZSTR_VAL(case_str), ZSTR_LEN(case_str), 1);
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
      persistent_zval_persist(&pval, val);
      if (key) {
        zend_string *pkey = zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 1);
        /* Iteration guarantees the source key has its hash set.
         * Propagating it lets zend_hash_add_new skip the re-hash. */
        ZSTR_H(pkey) = ZSTR_H(key);
        zend_hash_add_new(dst_ht, pkey, &pval);
        zend_string_release(pkey);
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

/* Deep-free a persistent zval tree. Idempotent on scalars. */
static void persistent_zval_free(zval *z) {
  switch (Z_TYPE_P(z)) {
  case IS_STRING:
    zend_string_free(Z_STR_P(z));
    break;
  case IS_PTR: {
    persistent_zval_enum_t *e = (persistent_zval_enum_t *)Z_PTR_P(z);
    zend_string_free(e->class_name);
    zend_string_free(e->case_name);
    pefree(e, 1);
    break;
  }
  case IS_ARRAY: {
    HashTable *ht = Z_ARRVAL_P(z);
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
/* The case of an enum by name, NULL when the reader's class has no such case:
 * the engine's own lookup asserts the case exists, which a snapshot written
 * against another version of the class breaks. */
static zend_object *persistent_zval_enum_case(zend_class_entry *ce,
                                              zend_string *name) {
  /* by value: hashing the persistent name would write to a string other
   * threads read */
  zend_class_constant *c = zend_hash_str_find_ptr(
      CE_CONSTANTS_TABLE(ce), ZSTR_VAL(name), ZSTR_LEN(name));
  if (c == NULL || !(ZEND_CLASS_CONST_FLAGS(c) & ZEND_CLASS_CONST_IS_CASE)) {
    return NULL;
  }
  if (Z_TYPE(c->value) == IS_CONSTANT_AST &&
      zval_update_constant_ex(&c->value, c->ce) == FAILURE) {
    return NULL;
  }

  return Z_TYPE(c->value) == IS_OBJECT ? Z_OBJ(c->value) : NULL;
}

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
    ZVAL_STRINGL(dst, Z_STRVAL_P(src), Z_STRLEN_P(src));
    break;
  case IS_PTR: {
    persistent_zval_enum_t *e = (persistent_zval_enum_t *)Z_PTR_P(src);
    /* autoloaders get the name and may keep it: a request copy, since the
     * persistent one is shared with other threads and freed with its tree */
    zend_string *class_name =
        zend_string_init(ZSTR_VAL(e->class_name), ZSTR_LEN(e->class_name), 0);
    zend_class_entry *ce = zend_lookup_class(class_name);
    zend_string_release(class_name);
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
    zend_object *enum_obj = persistent_zval_enum_case(ce, e->case_name);
    if (EG(exception)) {
      ZVAL_NULL(dst);
      break;
    }
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
        zend_string *rkey = zend_string_init(ZSTR_VAL(key), ZSTR_LEN(key), 0);
        ZSTR_H(rkey) = ZSTR_H(key);
        zend_hash_add_new(dst_ht, rkey, &rval);
        zend_string_release(rkey);
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
