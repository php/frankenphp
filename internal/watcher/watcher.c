// clang-format off
//go:build !nowatcher
// clang-format on
#include "_cgo_export.h"
#include "wtr/watcher-c.h"

void handle_event(struct wtr_watcher_event event, void *_ctx) {
  go_handle_file_watcher_event(event, (uintptr_t)_ctx);
}

uintptr_t start_new_watcher(char const *const path, uintptr_t _ctx) {
  void *watcher = wtr_watcher_open(path, handle_event, (void *)_ctx);

  if (watcher == NULL) {
    return 0;
  }

  return (uintptr_t)watcher;
}

int stop_watcher(uintptr_t watcher) {
  if (!wtr_watcher_close((void *)watcher)) {
    return 0;
  }

  return 1;
}
