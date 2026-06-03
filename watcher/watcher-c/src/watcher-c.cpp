#include <cstdint>
#include <cstdio>
#include "wtr/watcher-c.h"
#include "wtr/watcher.hpp"

#ifdef _WIN32
#define PATH_BUF_LEN 4096
#include <windows.h>
#include <stringapiset.h>
#endif

extern "C" {

#ifdef _WIN32
/*  - utf16_buf
      Must be null-terminated, which, with std::filesystem, it is.
    - utf8_buf
      Points to a null-terminated buffer, if conversion is successful, else null.
    - utf8_buf_len
      Just here conventionally. This buffer should be exactly 4096, also conventionally.
*/
static int utf16_to_utf8(wchar_t const* utf16_buf, char* utf8_buf, int utf8_buf_len) {
  if (!utf16_buf || !utf8_buf) return -1;
  static int const utf16_buf_len = -1;
  int wrote = WideCharToMultiByte(CP_UTF8, 0, utf16_buf, utf16_buf_len, utf8_buf, utf8_buf_len, NULL, NULL);
  if (wrote <= 0) {
    utf8_buf[0] = 0;
    return -1;
  } else {
    utf8_buf[wrote] = 0;
    return wrote;
  }
}
#endif

void* wtr_watcher_open(
  char const* const path,
  wtr_watcher_callback callback,
  void* context)
{
  auto wrapped_callback = [callback, context](wtr::watcher::event ev_owned)
  {
    wtr_watcher_event ev_view = {};
#ifdef _WIN32
    char path_name[PATH_BUF_LEN] = {0};
    char associated_path_name[PATH_BUF_LEN] = {0};
    int wp = utf16_to_utf8(ev_owned.path_name.c_str(), path_name, PATH_BUF_LEN);
    if (wp <= 0) return;
    ev_view.path_name = path_name;
    if (ev_owned.associated) {
      int wa = utf16_to_utf8(ev_owned.associated->path_name.c_str(), associated_path_name, PATH_BUF_LEN);
      if (wa <= 0) return;
      ev_view.associated_path_name = associated_path_name;
    }
#else
    ev_view.path_name = ev_owned.path_name.c_str();
    if (ev_owned.associated) {
      ev_view.associated_path_name = ev_owned.associated->path_name.c_str();
    }
#endif
    ev_view.effect_type = (int8_t)ev_owned.effect_type;
    ev_view.path_type = (int8_t)ev_owned.path_type;
    ev_view.effect_time = ev_owned.effect_time;
    callback(ev_view, context);
  };
  return (void*)new wtr::watcher::watch(path, wrapped_callback);
}

bool wtr_watcher_close(void* watcher)
{
  if (! watcher) return false;
  if (! ((wtr::watcher::watch*)watcher)->close()) return false;
  delete (wtr::watcher::watch*)watcher;
  return true;
}

}  // extern "C"
