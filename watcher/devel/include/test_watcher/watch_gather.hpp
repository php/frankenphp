#pragma once

#include "snitch/snitch.hpp"
#include "test_watcher/constant.hpp"
#include "test_watcher/event.hpp"
#include "test_watcher/filesystem.hpp"
#include "test_watcher/is_verbose.hpp"
#include "wtr/watcher.hpp"
#include <cassert>
#include <chrono>
#include <filesystem>
#include <functional>
#include <iostream>
#include <memory>
#include <mutex>
#include <thread>
#include <tuple>
#include <utility>
#include <vector>

namespace wtr {
namespace test_watcher {

auto watch_gather(
  auto const& title = "test",
  int path_count = 10,
  int concurrency_level = 1)
{
  namespace fs = std::filesystem;
  using namespace std::chrono_literals;

  auto verbose = is_verbose();

  auto event_recv_list = std::vector<wtr::event>{};
  auto event_recv_list_mtx = std::mutex{};

  auto event_sent_list = std::vector<wtr::event>{};
  auto watch_path_list = std::vector<fs::path>{};

  auto const tmpdir = make_local_tmp_dir();

  std::cerr << title << std::endl;

  {
    auto lifetimes = std::vector<std::unique_ptr<wtr::watch>>{};

    /*  Setup the test directory */
    {
      REQUIRE(fs::exists(tmpdir) || fs::create_directory(tmpdir));

      for (auto i = 0; i < concurrency_level; i++) {
        auto p = tmpdir / std::to_string(i);
        watch_path_list.emplace_back(p);
        REQUIRE(fs::create_directory(p));
      }
    }

#ifdef __APPLE__
    /*  Darwin might pick up events beforehand if we don't
        sleep this off. I think this is technically a bug,
        but a good kind of bug. At least, not detrimental to
        the user. */
    std::this_thread::sleep_for(100ms);
#endif

    /*  Watch Paths */
    {
      for (auto const& p : watch_path_list) {
        REQUIRE(fs::exists(p));

        auto cb = [&](wtr::event const& ev)
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
          for (auto const& p : watch_path_list)
            if (ev.path_name == p) return;
          event_recv_list.emplace_back(ev);
        };

        lifetimes.emplace_back(std::make_unique<wtr::watch>(p, cb));
      }
    }

    /*  See the note in the readme about ready state */
    std::this_thread::sleep_for(16ms);

    /*  Create Filesystem Events */
    {
      for (auto const& p : watch_path_list)
        event_sent_list.emplace_back(wtr::event{
          std::string("s/self/live@").append(p.string()),
          wtr::event::effect_type::create,
          wtr::event::path_type::watcher});

      for (auto const& p : watch_path_list)
        mk_events(p, path_count, &event_sent_list);

      for (auto const& p : watch_path_list) REQUIRE(fs::exists(p));

      for (int i = 0;; i++) {
        auto timeout = 1000 + (concurrency_level * 10);
        std::this_thread::sleep_for(10ms);
        auto _ = std::scoped_lock<std::mutex>{event_recv_list_mtx};
        if (event_recv_list.size() == event_sent_list.size()) break;
        if (i > timeout) REQUIRE(! "Timeout expired waiting for results");
      }

      for (auto const& p : watch_path_list)
        event_sent_list.emplace_back(wtr::event{
          std::string("s/self/die@").append(p.string()),
          wtr::event::effect_type::destroy,
          wtr::event::path_type::watcher});
    }
  }

  /*  Clean */
  REQUIRE(! fs::exists(tmpdir) || fs::remove_all(tmpdir));

  return std::make_pair(std::move(event_sent_list), std::move(event_recv_list));
};

} /* namespace test_watcher */
} /* namespace wtr */
