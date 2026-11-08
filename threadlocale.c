/* setlocale() changes the locale of the whole process: a request sees the
 * locale of requests running at the same time, and concurrent calls can crash
 * the process because setlocale() isn't thread-safe. FrankenPHP replaces it
 * with a version that gives each thread its own locale with the POSIX
 * newlocale() and uselocale() functions, which the locale-dependent C
 * functions (strftime(), strcoll(), localeconv(), printf()...) use instead of
 * the process locale. */

#ifndef _WIN32

#include "threadlocale.h"

#include <php.h>

#include <Zend/zend_smart_str.h>
#include <ext/standard/basic_functions.h>
#include <locale.h>
#include <pthread.h>
#include <string.h>
#ifdef __APPLE__
#include <xlocale.h>
#endif

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) ||       \
    defined(__OpenBSD__) || defined(__DragonFly__)
#define BSD_COMPOSITE_NAMES
#endif

typedef struct {
  int category;
  int mask;
  const char *name;
} locale_category;

/* In the order of the composite name setlocale(LC_ALL, NULL) returns when the
 * categories have different locales: "C/C.UTF-8/C/C/C/C" on BSD and macOS,
 * "LC_CTYPE=C.UTF-8;LC_NUMERIC=C;..." on glibc and "C.UTF-8;C;C;C;C;C" on
 * musl */
static const locale_category categories[] = {
#ifdef BSD_COMPOSITE_NAMES
    {LC_COLLATE, LC_COLLATE_MASK, "LC_COLLATE"},
    {LC_CTYPE, LC_CTYPE_MASK, "LC_CTYPE"},
    {LC_MONETARY, LC_MONETARY_MASK, "LC_MONETARY"},
    {LC_NUMERIC, LC_NUMERIC_MASK, "LC_NUMERIC"},
    {LC_TIME, LC_TIME_MASK, "LC_TIME"},
    {LC_MESSAGES, LC_MESSAGES_MASK, "LC_MESSAGES"},
#else
    {LC_CTYPE, LC_CTYPE_MASK, "LC_CTYPE"},
    {LC_NUMERIC, LC_NUMERIC_MASK, "LC_NUMERIC"},
    {LC_TIME, LC_TIME_MASK, "LC_TIME"},
    {LC_COLLATE, LC_COLLATE_MASK, "LC_COLLATE"},
    {LC_MONETARY, LC_MONETARY_MASK, "LC_MONETARY"},
    {LC_MESSAGES, LC_MESSAGES_MASK, "LC_MESSAGES"},
#ifdef __GLIBC__
    {LC_PAPER, LC_PAPER_MASK, "LC_PAPER"},
    {LC_NAME, LC_NAME_MASK, "LC_NAME"},
    {LC_ADDRESS, LC_ADDRESS_MASK, "LC_ADDRESS"},
    {LC_TELEPHONE, LC_TELEPHONE_MASK, "LC_TELEPHONE"},
    {LC_MEASUREMENT, LC_MEASUREMENT_MASK, "LC_MEASUREMENT"},
    {LC_IDENTIFICATION, LC_IDENTIFICATION_MASK, "LC_IDENTIFICATION"},
#endif
#endif
};

#define CATEGORY_COUNT (sizeof(categories) / sizeof(categories[0]))

#ifdef BSD_COMPOSITE_NAMES
#define COMPOSITE_SEPARATOR "/"
#else
#define COMPOSITE_SEPARATOR ";"
#endif

/* A locale object with the locale name of each category. They are shared by
 * all threads and never freed: threads may still use them, a process only
 * needs a few, and freelocale() crashes on macOS while another thread creates
 * a locale. */
typedef struct shared_locale {
  locale_t locale;
  struct shared_locale *next;
  char *names[CATEGORY_COUNT];
} shared_locale;

static shared_locale *shared_locales = NULL;
static pthread_mutex_t shared_locales_mutex = PTHREAD_MUTEX_INITIALIZER;

/* Locale of the current thread, NULL for the process locale */
static __thread shared_locale *thread_locale = NULL;
/* Locale the worker script set before handling requests */
static __thread shared_locale *worker_locale = NULL;

static int get_category_index(zend_long category) {
  for (size_t i = 0; i < CATEGORY_COUNT; i++) {
    if (categories[i].category == category) {
      return (int)i;
    }
  }

  return -1;
}

static void get_thread_names(const char **names) {
  for (size_t i = 0; i < CATEGORY_COUNT; i++) {
    names[i] = thread_locale != NULL ? thread_locale->names[i]
                                     : setlocale(categories[i].category, NULL);
  }
}

static bool are_names_equal(const char **names) {
  for (size_t i = 1; i < CATEGORY_COUNT; i++) {
    if (strcmp(names[i], names[0]) != 0) {
      return false;
    }
  }

  return true;
}

/* The name setlocale() returns for a category or LC_ALL */
static zend_string *get_name(zend_long category, const char **names) {
  if (category != LC_ALL) {
    const char *name = names[get_category_index(category)];
    return zend_string_init(name, strlen(name), 0);
  }

  if (are_names_equal(names)) {
    return zend_string_init(names[0], strlen(names[0]), 0);
  }

  smart_str composite = {0};
  for (size_t i = 0; i < CATEGORY_COUNT; i++) {
    if (i > 0) {
      smart_str_appends(&composite, COMPOSITE_SEPARATOR);
    }
#ifdef __GLIBC__
    smart_str_appends(&composite, categories[i].name);
    smart_str_appendc(&composite, '=');
#endif
    smart_str_appends(&composite, names[i]);
  }

  return smart_str_extract(&composite);
}

/* Splits a composite name returned by setlocale(LC_ALL, NULL) into names,
 * which point into the modified composite string */
static bool parse_composite_name(char *composite, const char **names) {
  size_t count = 0;
  char *last = NULL;

  for (char *part = strtok_r(composite, COMPOSITE_SEPARATOR, &last);
       part != NULL; part = strtok_r(NULL, COMPOSITE_SEPARATOR, &last)) {
#ifdef __GLIBC__
    char *name = strchr(part, '=');
    if (name == NULL) {
      return false;
    }
    *name = '\0';

    size_t i = 0;
    while (i < CATEGORY_COUNT && strcmp(categories[i].name, part) != 0) {
      i++;
    }
    if (i == CATEGORY_COUNT) {
      return false;
    }
    names[i] = name + 1;
    count++;
#else
    if (count == CATEGORY_COUNT) {
      return false;
    }
    names[count++] = part;
#endif
  }

  return count == CATEGORY_COUNT;
}

/* The locale setlocale() uses for "", from the LC_ALL, LC_* and LANG
 * environment variables */
static const char *get_environment_name(size_t index) {
  const char *variables[] = {"LC_ALL", categories[index].name, "LANG"};

  for (size_t i = 0; i < sizeof(variables) / sizeof(variables[0]); i++) {
    const char *value = getenv(variables[i]);
    if (value != NULL && value[0] != '\0') {
      return value;
    }
  }

  return "C";
}

/* The name the C library's setlocale() returns for a locale */
static const char *get_canonical_name(const char *name) {
#ifndef BSD_COMPOSITE_NAMES
  /* glibc and musl name the POSIX locale "C" */
  if (strcmp(name, "POSIX") == 0) {
    return "C";
  }
#endif

  return name;
}

static locale_t new_locale(const char **names) {
  if (are_names_equal(names)) {
    return newlocale(LC_ALL_MASK, names[0], (locale_t)0);
  }

  locale_t locale = newlocale(LC_ALL_MASK, "C", (locale_t)0);
  for (size_t i = 0; locale != (locale_t)0 && i < CATEGORY_COUNT; i++) {
    if (strcmp(names[i], "C") == 0) {
      continue;
    }

    /* On success, newlocale() reuses or frees locale */
    locale_t with_category = newlocale(categories[i].mask, names[i], locale);
    if (with_category == (locale_t)0) {
      freelocale(locale);
    }
    locale = with_category;
  }

  return locale;
}

static shared_locale *find_shared_locale(const char **names) {
  for (shared_locale *shared =
           __atomic_load_n(&shared_locales, __ATOMIC_ACQUIRE);
       shared != NULL; shared = shared->next) {
    size_t i = 0;
    while (i < CATEGORY_COUNT && strcmp(shared->names[i], names[i]) == 0) {
      i++;
    }
    if (i == CATEGORY_COUNT) {
      return shared;
    }
  }

  return NULL;
}

/* Returns NULL if a locale isn't available */
static shared_locale *get_shared_locale(const char **names) {
  shared_locale *shared = find_shared_locale(names);
  if (shared != NULL) {
    return shared;
  }

  /* Locales are created one at a time, concurrent newlocale() and
   * freelocale() calls crash on macOS */
  pthread_mutex_lock(&shared_locales_mutex);
  shared = find_shared_locale(names);
  if (shared == NULL) {
    locale_t locale = new_locale(names);
    if (locale != (locale_t)0) {
      shared = pemalloc(sizeof(shared_locale), 1);
      shared->locale = locale;
      for (size_t i = 0; i < CATEGORY_COUNT; i++) {
        shared->names[i] = pestrdup(names[i], 1);
      }
      shared->next = shared_locales;
      __atomic_store_n(&shared_locales, shared, __ATOMIC_RELEASE);
    }
  }
  pthread_mutex_unlock(&shared_locales_mutex);

  return shared;
}

/* Switches the current thread to a locale, NULL for the process locale */
static void use_locale(shared_locale *shared) {
  if (shared == thread_locale) {
    return;
  }

  int ctype = get_category_index(LC_CTYPE);
  const char *previous_ctype =
      thread_locale != NULL ? thread_locale->names[ctype] : NULL;
  const char *ctype_name = shared != NULL ? shared->names[ctype] : NULL;

  uselocale(shared != NULL ? shared->locale : LC_GLOBAL_LOCALE);
  thread_locale = shared;

  if (previous_ctype == ctype_name ||
      (previous_ctype != NULL && ctype_name != NULL &&
       strcmp(previous_ctype, ctype_name) == 0)) {
    return;
  }

  /* Update the LC_CTYPE dependent state like setlocale() does, PCRE uses
   * BG(ctype_string) to select its character tables */
  zend_update_current_locale();
  if (BG(ctype_string) != NULL) {
    zend_string_release_ex(BG(ctype_string), 0);
    BG(ctype_string) = NULL;
  }
  if (ctype_name != NULL && strcmp(ctype_name, "C") != 0) {
    BG(ctype_string) = zend_string_init(ctype_name, strlen(ctype_name), 0);
  }
}

static zend_string *try_setlocale_str(zend_long category, zend_string *loc) {
  int index = category == LC_ALL ? -1 : get_category_index(category);
  if (category != LC_ALL && index < 0) {
    return NULL;
  }

  const char *names[CATEGORY_COUNT];
  get_thread_names(names);

  if (zend_string_equals_literal(loc, "0")) {
    return get_name(category, names);
  }

  if (ZSTR_LEN(loc) >= 255) {
    php_error_docref(NULL, E_WARNING, "Specified locale name is too long");
    return NULL;
  }

  char composite[255];
  if (ZSTR_LEN(loc) == 0) {
    for (size_t i = 0; i < CATEGORY_COUNT; i++) {
      if (category == LC_ALL || (int)i == index) {
        names[i] = get_environment_name(i);
      }
    }
  } else if (category == LC_ALL &&
             strstr(ZSTR_VAL(loc), COMPOSITE_SEPARATOR) != NULL) {
    memcpy(composite, ZSTR_VAL(loc), ZSTR_LEN(loc) + 1);
    if (!parse_composite_name(composite, names)) {
      return NULL;
    }
  } else if (category == LC_ALL) {
    for (size_t i = 0; i < CATEGORY_COUNT; i++) {
      names[i] = ZSTR_VAL(loc);
    }
  } else {
    names[index] = ZSTR_VAL(loc);
  }

  for (size_t i = 0; i < CATEGORY_COUNT; i++) {
    names[i] = get_canonical_name(names[i]);
  }

  shared_locale *shared = get_shared_locale(names);
  if (shared == NULL) {
    return NULL;
  }

  use_locale(shared);

  return get_name(category, (const char **)shared->names);
}

static zend_string *try_setlocale_zval(zend_long category, zval *loc_zv) {
  zend_string *tmp_loc_str;
  zend_string *loc_str = zval_try_get_tmp_string(loc_zv, &tmp_loc_str);
  if (UNEXPECTED(loc_str == NULL)) {
    return NULL;
  }
#if PHP_VERSION_ID >= 80600
  if (zend_str_has_nul_byte(loc_str)) {
    zend_argument_value_error(2, "must not contain any null bytes");
    zend_tmp_string_release(tmp_loc_str);
    return NULL;
  }
#endif
  zend_string *result = try_setlocale_str(category, loc_str);
  zend_tmp_string_release(tmp_loc_str);

  return result;
}

#if PHP_VERSION_ID >= 80500
/* Since PHP 8.5, setlocale() checks the type of all arguments before trying
 * a locale, and since PHP 8.6, it rejects null bytes and an array after the
 * first locale */
static bool parse_setlocale_args(zval *args, uint32_t num_args,
                                 zend_string **strings) {
  for (uint32_t i = 0; i < num_args; i++) {
#if PHP_VERSION_ID >= 80600
    if (Z_TYPE(args[i]) == IS_ARRAY) {
      if (UNEXPECTED(i != 0)) {
        zend_wrong_parameter_type_error(i + 2, Z_EXPECTED_STRING_OR_NULL,
                                        &args[i]);
        return false;
      }
      if (UNEXPECTED(num_args > 1)) {
        zend_argument_count_error("setlocale() expects exactly 2 arguments "
                                  "when argument #2 ($locales) is an array, "
                                  "%d given",
                                  (int)num_args + 1);
        return false;
      }
      break;
    }
    if (UNEXPECTED(
            !zend_parse_arg_path_str(&args[i], &strings[i], true, i + 2))) {
      zend_wrong_parameter_type_error(
          i + 2,
          Z_TYPE(args[i]) == IS_STRING
              ? Z_EXPECTED_PATH
              : (i == 0 ? Z_EXPECTED_ARRAY_OR_STRING_OR_NULL
                        : Z_EXPECTED_STRING_OR_NULL),
          &args[i]);
      return false;
    }
#else
    if (UNEXPECTED(Z_TYPE(args[i]) != IS_ARRAY &&
                   !zend_parse_arg_str(&args[i], &strings[i], true, i + 2))) {
      zend_wrong_parameter_type_error(i + 2, Z_EXPECTED_ARRAY_OR_STRING_OR_NULL,
                                      &args[i]);
      return false;
    }
#endif
  }

  return true;
}
#endif

/* Returns the first locale of the arguments that is available */
static zend_string *try_setlocale_args(zend_long category, zval *args,
                                       uint32_t num_args,
                                       zend_string **strings) {
  for (uint32_t i = 0; i < num_args; i++) {
    zend_string *result;
    if (Z_TYPE(args[i]) == IS_ARRAY) {
      zval *elem;
      ZEND_HASH_FOREACH_VAL(Z_ARRVAL(args[i]), elem) {
        result = try_setlocale_zval(category, elem);
        if (EG(exception) || result != NULL) {
          return result;
        }
      }
      ZEND_HASH_FOREACH_END();
      continue;
    }
#if PHP_VERSION_ID >= 80500
    result = try_setlocale_str(category, Z_ISNULL(args[i]) ? ZSTR_EMPTY_ALLOC()
                                                           : strings[i]);
#else
    (void)strings;
    result = try_setlocale_zval(category, &args[i]);
#endif
    if (EG(exception) || result != NULL) {
      return result;
    }
  }

  return NULL;
}

/* Same as PHP_FUNCTION(setlocale) in ext/standard/string.c, including the
 * argument handling of each PHP version */
static PHP_FUNCTION(frankenphp_setlocale) {
  zend_long category;
  zval *args = NULL;
  uint32_t num_args;

  ZEND_PARSE_PARAMETERS_START(2, -1)
  Z_PARAM_LONG(category)
  Z_PARAM_VARIADIC('+', args, num_args)
  ZEND_PARSE_PARAMETERS_END();

#if PHP_VERSION_ID >= 80500
  ALLOCA_FLAG(use_heap);
  zend_string **strings = do_alloca(sizeof(zend_string *) * num_args, use_heap);
  zend_string *result =
      parse_setlocale_args(args, num_args, strings)
          ? try_setlocale_args(category, args, num_args, strings)
          : NULL;
  free_alloca(strings, use_heap);
#else
  zend_string *result = try_setlocale_args(category, args, num_args, NULL);
#endif

  if (EG(exception)) {
    RETURN_THROWS();
  }
  if (result == NULL) {
    RETURN_FALSE;
  }
  RETURN_STR(result);
}

void frankenphp_override_setlocale(void) {
  zend_function *func = zend_hash_str_find_ptr(CG(function_table), "setlocale",
                                               sizeof("setlocale") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION) {
    ((zend_internal_function *)func)->handler = ZEND_FN(frankenphp_setlocale);
  } else {
    php_error(E_WARNING, "Failed to find built-in setlocale function");
  }
}

void frankenphp_save_worker_locale(void) { worker_locale = thread_locale; }

void frankenphp_restore_worker_locale(void) { use_locale(worker_locale); }

void frankenphp_reset_thread_locale(void) {
  worker_locale = NULL;
  use_locale(NULL);
}

#else

/* PHP keeps the locale per thread on Windows, so setlocale() isn't replaced
 * there: only the locale the worker script set before handling requests is
 * restored after each request */

#include "threadlocale.h"

#include <php.h>

#include <ext/standard/basic_functions.h>
#include <locale.h>
#include <string.h>

/* Name of the locale the worker script set before handling requests */
static __thread char *worker_locale = NULL;

void frankenphp_save_worker_locale(void) {
  const char *name = setlocale(LC_ALL, NULL);
  if (worker_locale != NULL) {
    pefree(worker_locale, 1);
  }
  worker_locale = name != NULL ? pestrdup(name, 1) : NULL;
}

void frankenphp_restore_worker_locale(void) {
  if (worker_locale == NULL) {
    return;
  }

  const char *name = setlocale(LC_ALL, NULL);
  if (name != NULL && strcmp(name, worker_locale) == 0) {
    return;
  }

  name = setlocale(LC_ALL, worker_locale);
  if (name == NULL) {
    return;
  }

  /* Update the LC_CTYPE dependent state like setlocale() does */
  zend_update_current_locale();
  if (BG(ctype_string) != NULL) {
    zend_string_release_ex(BG(ctype_string), 0);
    BG(ctype_string) = NULL;
  }
  if (strcmp(name, "C") != 0) {
    BG(ctype_string) = zend_string_init(name, strlen(name), 0);
  }
}

void frankenphp_reset_thread_locale(void) {
  if (worker_locale != NULL) {
    pefree(worker_locale, 1);
    worker_locale = NULL;
  }
}

#endif
