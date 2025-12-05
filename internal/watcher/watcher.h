#include <stdint.h>
#include <stdlib.h>
#include <wtr/watcher-c.h>

uintptr_t start_new_watcher(char const *const path, uintptr_t _ctx);

int stop_watcher(uintptr_t watcher);
