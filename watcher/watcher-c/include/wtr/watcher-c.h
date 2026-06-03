#pragma once

#include <stdbool.h>
#include <stdint.h>

#ifdef _WIN32
#define WTR_WATCHER_C_API __declspec(dllexport)
#else
#define WTR_WATCHER_C_API
#endif

#ifdef __cplusplus
extern "C" {
#endif

/*  Represents "what happened" to a path. */
static const int8_t WTR_WATCHER_EFFECT_RENAME = 0;
static const int8_t WTR_WATCHER_EFFECT_MODIFY = 1;
static const int8_t WTR_WATCHER_EFFECT_CREATE = 2;
static const int8_t WTR_WATCHER_EFFECT_DESTROY = 3;
static const int8_t WTR_WATCHER_EFFECT_OWNER = 4;
static const int8_t WTR_WATCHER_EFFECT_OTHER = 5;

/*  Represents "what kind" of path it is. */
static const int8_t WTR_WATCHER_PATH_DIR = 0;
static const int8_t WTR_WATCHER_PATH_FILE = 1;
static const int8_t WTR_WATCHER_PATH_HARD_LINK = 2;
static const int8_t WTR_WATCHER_PATH_SYM_LINK = 3;
static const int8_t WTR_WATCHER_PATH_WATCHER = 4;
static const int8_t WTR_WATCHER_PATH_OTHER = 5;

/*  The `event` object is used to carry information about
    filesystem events to the user through the (user-supplied)
    callback given to `watch`.
    The `event` object will contain the:
      - `path_name`: The path to the event.
      - `path_type`: One of:
        - dir
        - file
        - hard_link
        - sym_link
        - watcher
        - other
      - `effect_type`: One of:
        - rename
        - modify
        - create
        - destroy
        - owner
        - other
      - `effect_time`:
        The time of the event in nanoseconds since epoch.
*/
struct wtr_watcher_event {
  int64_t effect_time;
  char const* path_name;
  char const* associated_path_name;
  int8_t effect_type;
  int8_t path_type;
};

/*  Ensure the user's callback can receive
    events and will return nothing. */
typedef void (* wtr_watcher_callback)(struct wtr_watcher_event event, void* context);

WTR_WATCHER_C_API
void* wtr_watcher_open(char const* const path, wtr_watcher_callback callback, void* context);

WTR_WATCHER_C_API
bool wtr_watcher_close(void* watcher);

#ifdef __cplusplus
}
#endif
