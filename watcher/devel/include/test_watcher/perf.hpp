#pragma once

#include "test_watcher/filesystem.hpp"
#include "wtr/watcher.hpp"
#include <algorithm>
#include <array>
#include <chrono>
#include <cstdio>
#include <fstream>
#include <memory>
#include <tuple>
#include <utility>
#include <vector>

// clang-format off

namespace wtr {
namespace test_watcher {

inline auto do_nothing() -> void {
#ifdef _WIN32
  unsigned volatile char _;
  _ = 0;
#else
  asm volatile ("nop");
#endif
};

inline auto median(auto const& values) -> double
{
  auto sorted = values;
  std::sort(sorted.begin(), sorted.end());
  auto size = sorted.size();
  if (size == 0)
    return 0;
  else if (size == 1)
    return sorted[0];
  else {
    // Conversions are easier this way, unfortunately.
    auto left = size % 2 == 0 ? size / 2 : (size / 2 - 1);
    return (sorted[left] + sorted[size / 2]) / 2.0;
  }
}

struct PerfCfg {
  int watch_count;
  int event_count;
};

struct PerfResult {
  struct PerfCfg cfg;
  std::chrono::nanoseconds time_taken_total;
  std::chrono::nanoseconds time_taken_fsops;
  std::chrono::nanoseconds time_taken_watch;
  std::chrono::nanoseconds clock_overhead;
  unsigned nth;
};

struct Range {
  int start{1};
  int stop{1};
  int step{1};
};

struct RangePair {
  Range watcher_range;
  Range event_range;
};

struct TimeParts {
  long long seconds;
  long long milliseconds;
  long long microseconds;
  long long nanoseconds;
};

inline auto time_parts(std::chrono::nanoseconds ns) -> TimeParts
{
  return {
    static_cast<long long>(std::chrono::duration_cast<std::chrono::seconds>(ns).count()),
    static_cast<long long>(std::chrono::duration_cast<std::chrono::milliseconds>(ns).count()),
    static_cast<long long>(std::chrono::duration_cast<std::chrono::microseconds>(ns).count()),
    static_cast<long long>(ns.count())
  };
};

inline auto ftime(char* buf, unsigned long long len, std::chrono::nanoseconds tnanos) {
  auto [ss, ms, us, ns] = time_parts(tnanos);
  auto b = ss > 0
    ? snprintf(buf, len, "%lld s", ss)
    : ms > 0
      ? snprintf(buf, len, "%lld ms", ms)
      : us > 0
        ? snprintf(buf, len, "%lld us", us)
        : snprintf(buf, len, "%lld ns", ns);
  return b;
}

inline auto show_results(auto res) -> void
{
  char buf[2048 * 80]{0};
  char* p = buf;
  auto colw = 20;
  auto idx = [&](char* p)
  { return sizeof(buf) - (p - buf); };
  auto inc = [&](char* p, int n)
  { return p + n < buf + sizeof(buf) ? p + n : 0; };
  auto fill = [&](char* p, int n) {
    auto np = p;
    while (n --> 0)
      np = inc(np, snprintf(np, idx(np), " "));
    return np;
  };
  auto incnfill = [&](char* p, int n)
  { return fill(inc(p, n), colw - n > 0 ? colw - n : 0); };
  auto hdrs =
  { "Total Time", "Total Fs Ops Time", "Total Watch Time", "Fs Ops Time/Event", "Watch Time/Event", "Clock Overhead", "Watchers", "Events" };
  for (auto hdr : hdrs)
    p = incnfill(p, snprintf(p, idx(p), "%s", hdr));
  p = inc(p, snprintf(p, idx(p), "\n"));
  for (auto r : res) {
    p = incnfill(p, ftime(p, idx(p), r.time_taken_total));
    p = incnfill(p, ftime(p, idx(p), r.time_taken_fsops));
    p = incnfill(p, ftime(p, idx(p), r.time_taken_watch));
    p = incnfill(p, ftime(p, idx(p), r.time_taken_fsops / r.cfg.watch_count));
    p = incnfill(p, ftime(p, idx(p), r.time_taken_watch / r.cfg.event_count));
    p = incnfill(p, ftime(p, idx(p), r.clock_overhead));
    p = incnfill(p, snprintf(p, idx(p), "%i", r.cfg.watch_count));
    p = incnfill(p, snprintf(p, idx(p), "%i", r.cfg.event_count));
    p = inc(p, snprintf(p, idx(p), "\n"));
  }
  printf("%s", buf);
};

inline auto now()
{
  using namespace std::chrono;
  return duration_cast<nanoseconds>(
      system_clock{}.now().time_since_epoch());
};

inline auto since = [](auto start)
{
  return now() - start;
};

template<PerfCfg cfg>
class Perf{
public:
  inline auto concurrent_watchers() -> PerfResult
  {
    using namespace std;
    using namespace wtr;

    auto const tmpdir = test_watcher::make_local_tmp_dir();

    if (! filesystem::exists(tmpdir))
      filesystem::create_directory(tmpdir);

    /*  Try to approximate the overhead of
        observing the time and a bit of
        addition by doing what we do in
        the ofstream loop, but without the
        actual filesystem ops.
        The longer this loop, the more
        reliable this figure probably is. */
    auto clock_overhead = 0ns;
    {
      constexpr int n = 1000;
      for (int i = 0; i < n; ++i) {
        auto start = now();
        do_nothing();
        clock_overhead += since(start);
      }
      clock_overhead /= n;
    }

    /*  Forces entry into the closure when the
        watcher decides to call it. Compilers
        might otherwise optimize a function
        with an empty body away. We use this
        to measure the accumulated time taken
        by the watcher between witnessing an
        event and sending it through to us.
    */
    auto cb = [](auto) { do_nothing(); };

    /*  For now, we're not measuring the time
        taken for the de/construction of the
        watchers or the set-up/tear-down time
        of the directory tree.
        Open question if we should measure
        the time taken for de/construction
        of these watchers. If we do want that,
        we should take care not to measure the
        allocation overhead. */
    auto watchers = array<unique_ptr<watch>, cfg.watch_count>{};
    for (int i = 0; i < cfg.watch_count; ++i)
      watchers.at(i) = std::move(make_unique<watch>(tmpdir, cb));

    auto start = now();
    auto time_taken_fsops = chrono::nanoseconds{0};
    for (int i = 0; i < cfg.event_count; ++i) {
      auto start_fsops = now();
      auto _ = ofstream{tmpdir / to_string(i)};  // touch
      time_taken_fsops += since(start_fsops);
    }
    auto time_taken_total = since(start);
    time_taken_fsops -= clock_overhead * cfg.event_count;
    auto time_taken_watch = time_taken_total - time_taken_fsops;

    if (filesystem::exists(tmpdir))
      filesystem::remove_all(tmpdir);

    return {
      cfg,
      time_taken_total,
      time_taken_fsops,
      time_taken_watch,
      clock_overhead,
      [] { static unsigned nth = 0; return nth++; }(),
    };
  };
};

template<RangePair Rp>
auto perf_range() -> std::vector<PerfResult>
{
  auto res = std::vector<PerfResult>{};

  // Until the end ...
  if constexpr (
      Rp.watcher_range.start + Rp.watcher_range.step <= Rp.watcher_range.stop
      && Rp.event_range.start + Rp.event_range.step <= Rp.event_range.stop)
  {
    // Run this ...
    auto head =
      Perf<PerfCfg{
        .watch_count=Rp.watcher_range.start,
        .event_count=Rp.event_range.start}>{}
      .concurrent_watchers();

    // And the rest ...
    auto tail =
      perf_range<
        RangePair{
          .watcher_range = Range{
            .start=Rp.watcher_range.start + Rp.watcher_range.step,
            .stop=Rp.watcher_range.stop,
            .step=Rp.watcher_range.step},
          .event_range = Range{
            .start=Rp.event_range.start + Rp.event_range.step,
            .stop=Rp.event_range.stop,
            .step=Rp.event_range.step}}
      >();

    res.push_back(head);
    res.insert(res.end(), tail.begin(), tail.end());
  }

  return res;
};

inline auto vec_cat(auto... vs) -> std::vector<PerfResult>
{
  auto res = std::vector<PerfResult>{};
  (res.insert(res.end(), vs.begin(), vs.end()), ...);
  return res;
};

inline auto watch_times_of(auto const& res) -> std::vector<long long>
{
  auto watch_time = std::vector<long long>{};
  for (auto r : res)
    watch_time.push_back(r.time_taken_watch.count());
  return watch_time;
}

inline auto fsops_times_of(auto const& res) -> std::vector<long long>
{
  auto fsops_time = std::vector<long long>{};
  for (auto r : res)
    fsops_time.push_back(r.time_taken_fsops.count());
  return fsops_time;
}

}  /*  namespace test_watcher */
}  /*  namespace wtr */
