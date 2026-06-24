#include "snitch/snitch.hpp"
#include "test_watcher/test_watcher.hpp"
#include "wtr/watcher.hpp"
#include <chrono>
#include <filesystem>
#include <future>
#include <iostream>
#include <mutex>
#include <string>
#include <tuple>
#include <vector>

/* Test that files are scanned */
TEST_CASE("Simple", "[dir][file][simple][not-perf]")
{
  namespace fs = std::filesystem;
  using namespace std::chrono_literals;
  using namespace wtr::watcher;
  using namespace wtr::test_watcher;

  static constexpr auto path_count = 3;
  static constexpr auto title = "Simple";
  static auto verbose = is_verbose();
  auto const tmpdir = make_local_tmp_dir();
  auto event_recv_list = std::vector<event>{};
  auto event_recv_list_mtx = std::mutex{};
  auto event_sent_list = std::vector<event>{};
  auto watch_path_list = std::vector<std::string>{};

  std::cerr << title << std::endl;

  REQUIRE(fs::exists(tmpdir) || fs::create_directory(tmpdir));

  /*  This sleep is hiding a bug on darwin which picks
      up events slightly *before* we start watching.
      I'm ok with that bit of wiggle-room. */
  std::this_thread::sleep_for(10ms);

  event_sent_list.push_back(
    {std::string("s/self/live@").append(tmpdir.string()),
     event::effect_type::create,
     event::path_type::watcher});

  auto watcher = watch(
    tmpdir,
    [&event_recv_list_mtx, &event_recv_list](event const& ev)
    {
#ifdef _WIN32
      /*  Windows counts all events in a directory as *also*
          `modify` events *on* the directory. So, we ignore
          those for consistency with the other tests. */
      if (
        ev.path_type == wtr::event::path_type::dir
        && ev.effect_type == wtr::event::effect_type::modify)
        return;
#endif
      auto _ = std::scoped_lock{event_recv_list_mtx};
      if (verbose) std::cerr << ev << std::endl;
      event_recv_list.push_back(ev);
    });

  /*  See the note in the readme about the ready state */
  std::this_thread::sleep_for(100ms);

  for (int i = 0; i < path_count; ++i) {
    auto const new_dir_path = tmpdir / ("new_dir" + std::to_string(i));
    fs::create_directory(new_dir_path);
    REQUIRE(fs::is_directory(new_dir_path));

    event_sent_list.push_back(
      {new_dir_path, event::effect_type::create, event::path_type::dir});

    auto const new_file_path =
      tmpdir / ("new_file" + std::to_string(i) + ".txt");
    std::ofstream(new_file_path).close();
    REQUIRE(fs::is_regular_file(new_file_path));

    event_sent_list.push_back(
      {new_file_path, event::effect_type::create, event::path_type::file});
  }

  /*  And give the watchers some room to await the events */
  for (int i = 0;; i++) {
    std::this_thread::sleep_for(10ms);
    auto _ = std::scoped_lock<std::mutex>{event_recv_list_mtx};
    if (event_recv_list.size() == event_sent_list.size()) break;
    if (i > 1000) REQUIRE(! "Timeout: Waited more than one second for results");
  }

  event_sent_list.push_back(
    {std::string("s/self/die@").append(tmpdir.string()),
     event::effect_type::destroy,
     event::path_type::watcher});

  /*  @todo This close operation should probably try to flush
      any pending events out there... Can we do that? */
  REQUIRE(watcher.close() == true);

  REQUIRE(! fs::exists(tmpdir) || fs::remove_all(tmpdir));

  if (verbose) {
    std::cerr << "event_sent_list:" << std::endl;
    for (auto const& ev : event_sent_list) std::cerr << ev << std::endl;
  }

  check_event_lists_eq(event_sent_list, event_recv_list);
};
