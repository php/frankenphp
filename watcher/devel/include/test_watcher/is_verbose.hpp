#pragma once

#include <cstdlib>

#ifdef _WIN32
#include <stdlib.h>  // _dupenv_s
#endif

namespace wtr {
namespace test_watcher {

inline auto is_verbose() -> bool
{
#ifndef _WIN32
  return std::getenv("VERBOSE") != nullptr;
#else
  // WTF, Windows?
  char* buf = nullptr;
  if (_dupenv_s(&buf, 0, "VERBOSE") > 0 && buf != nullptr) {
    free(buf);
    return true;
  }
  else {
    return false;
  }
#endif
}

}  // namespace test_watcher
}  // namespace wtr
