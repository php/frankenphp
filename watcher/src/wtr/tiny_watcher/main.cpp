// clang-format off
#include "wtr/watcher.hpp"  // Or wherever yours is
#include <iostream>
#include <string>

using namespace std;
using namespace wtr;

// The event type, and every field within it, has
// string conversions and stream operators. All
// kinds of strings -- Narrow, wide and weird ones.
// If we don't want particular formatting, we can
// json-serialize and show the event like this:
//   some_stream << event
// Here, we'll apply our own formatting.
auto show(event e) {
  auto s = [](auto a) { return to<string>(a); };
  cout << s(e.effect_type) + ' '
        + s(e.path_type)   + ' '
        + s(e.path_name)
        + (e.associated ? " -> " + s(e.associated->path_name) : "")
       << endl;
}

auto main() -> int {
  // Watch the current directory asynchronously,
  // calling the provided function for each event.
  auto watcher = watch(".", show);

  // Do some work. (We'll just wait for a newline.)
  (void)getchar();

  // The watcher would close itself around here,
  // though we can check and close it ourselves:
  return watcher.close() ? 0 : 1;
}
