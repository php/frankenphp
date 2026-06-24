#include "snitch/snitch.hpp"
#include "test_watcher/test_watcher.hpp"
#include "wtr/watcher.hpp"
#include <stdio.h>

// clang-format off

/*  We test that the overhead of watching a filesystem is unencumbering.

    IOW, our results should be bound to how fast we can perform io on
    the filesystem, not by how fast the watcher can witness events.

    We want to offload most of the unrelated work onto compile-time
    computations so that we can accurately measure a "common" watcher
    path/callback setup. Things like:
      Allocating, storing and resizing vectors of watchers.
      Complicated iteration logic.
    We use some fancy templates for that, so beware of the (fatal)
    compiler error when testing many watchers or events:
      template instantiation depth exceeds <some number around 1k>
*/

TEST_CASE("Performance", "[concurrent][file][perf]")
{
  using namespace wtr::test_watcher;

  fprintf(stderr, "Performance\n");

  //  Warming up the cache
  perf_range<RangePair{{1,3,1},{100,500,100}}>();

  auto res = vec_cat(
    perf_range<RangePair{
        .watcher_range=  { .start=1,    .stop=1,     .step=0    },
        .event_range=    { .start=100,  .stop=1000,  .step=100  }}>(),
    perf_range<RangePair{
        .watcher_range=  { .start=1,    .stop=1,     .step=0    },
        .event_range=    { .start=1000, .stop=10000, .step=1000 }}>(),
    perf_range<RangePair{
        .watcher_range=  { .start=5,    .stop=30,    .step=5    },
        .event_range=    { .start=1000, .stop=1000,  .step=0    }}>()
  );

  show_results(res);

  CHECK(median(watch_times_of(res)) < median(fsops_times_of(res)));
};

// clang-format on
