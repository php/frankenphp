#include "snitch/snitch.hpp"
#include "test_watcher/test_watcher.hpp"
#include "wtr/watcher.hpp"
#include <iostream>
#include <tuple>
#include <vector>

/* Test that files are scanned */
TEST_CASE("Event Targets", "[file][dir][accuracy][not-perf]")
{
  using namespace wtr::watcher;
  using namespace wtr::test_watcher;

  check_event_lists_eq(watch_gather("Event Targets", 10));
};
