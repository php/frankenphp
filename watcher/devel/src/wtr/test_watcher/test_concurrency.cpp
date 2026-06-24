#include "snitch/snitch.hpp"
#include "test_watcher/event.hpp"
#include "test_watcher/test_watcher.hpp"
#include "wtr/watcher.hpp"
#include <iostream>
#include <tuple>
#include <utility>

/* Test that files are scanned */
TEST_CASE(
  "Concurrency",
  "[concurrent][file][dir][accuracy][not-perf]")
{
  using namespace wtr::watcher;
  using namespace wtr::test_watcher;

  auto event_count_per_watcher = 10;
  auto concurrency_level = 8;

  check_event_lists_set_eq(watch_gather(
    "Concurrency",
    event_count_per_watcher,
    concurrency_level));
};
