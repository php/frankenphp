#pragma once

#include "snitch/snitch.hpp"
#include "test_watcher/constant.hpp"
#include "wtr/watcher.hpp"
#include <cmath>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <numeric>
#include <string>
#include <thread>
#include <unordered_set>
#include <vector>

namespace wtr {
inline namespace watcher {
struct event_without_time {
  decltype(::wtr::event::path_name) path_name;
  decltype(::wtr::event::path_type) path_type;
  decltype(::wtr::event::effect_type) effect_type;
  auto operator==(event_without_time const& other) const -> bool = default;
  auto operator!=(event_without_time const& other) const -> bool = default;
};

template<class T>
inline auto to(::wtr::watcher::event_without_time const& from) noexcept -> T;

template<>
inline auto to<std::string>(
  ::wtr::watcher::event_without_time const& from) noexcept -> std::string
{
  using wtr::to, std::string;
  auto&& fields = "\n  path_name:    " + to<string>(from.path_name)
                + "\n  path_type:    " + to<string>(from.path_type)
                + "\n  effect_type:  " + to<string>(from.effect_type) + "\n";
  return "{" + fields + "}";
};
}
}

template<>
struct std::hash<wtr::watcher::event_without_time> {
  inline auto operator()(
    wtr::watcher::event_without_time const& ev) const noexcept -> std::size_t
  {
    return std::hash<decltype(ev.path_name.string())>{}(ev.path_name.string())
#ifdef _WIN32
         ^ std::hash<int>{}((int)ev.path_type)
         ^ std::hash<int>{}((int)ev.effect_type);
#else
         ^ std::hash<decltype(ev.path_type)>{}(ev.path_type)
         ^ std::hash<decltype(ev.effect_type)>{}(ev.effect_type);
#endif
  }
};

namespace wtr {
namespace test_watcher {

inline bool str_eq(char const* a, char const* b)
{
  return std::strcmp(a, b) == 0;
}

/* Make Events
   Mirror what the Watcher should see
   Half are creation events
   Half are destruction */
inline auto mk_events(
  std::filesystem::path const& base_path,
  auto const& path_count,
  std::vector<wtr::event>* event_list,
  unsigned long options = mk_events_options) -> void
{
  using namespace wtr::watcher;
  using namespace std::chrono_literals;
  using std::iota, std::abs, std::filesystem::exists;

  /* - The path count must be even
     - The path count must be greater than 1 */
  REQUIRE(path_count % 2 == 0);
  REQUIRE(path_count > 1);

  std::vector<int> path_indices(path_count);

  if (options & mk_events_reverse)
    iota(path_indices.rbegin(), path_indices.rend(), -path_count / 2);
  else
    iota(path_indices.begin(), path_indices.end(), -path_count / 2);

  for (auto& i : path_indices) {
    std::this_thread::sleep_for(16ms);
    auto const path = base_path / std::to_string(i < 0 ? abs(i) - 1 : abs(i));
    if ((options & mk_events_reverse) ? i >= 0 : i < 0) {
      event_list->push_back(wtr::event{
        path,
        wtr::event::effect_type::create,
        wtr::event::path_type::file});
      auto _ = std::ofstream{path};
    }
    else {
      event_list->push_back(wtr::event{
        path,
        wtr::event::effect_type::destroy,
        wtr::event::path_type::file});
      std::filesystem::remove(path);
    }
  }

  if (options & mk_events_die_after)
    event_list->push_back(wtr::event{
      std::string("s/self/die@").append(base_path.string()),
      wtr::event::effect_type::destroy,
      wtr::event::path_type::watcher});
}

inline auto mk_revents(
  auto const& watch_path,
  auto const& path_count,
  auto const& event_list,
  unsigned long options = mk_events_options)
{
  return mk_events(
    watch_path,
    path_count,
    event_list,
    options |= mk_events_reverse);
}

inline auto
check_event_lists_eq(auto const& event_sent_list, auto const& event_recv_list)
{
  auto const max_i = event_sent_list.size() > event_recv_list.size()
                     ? event_recv_list.size()
                     : event_sent_list.size();
  for (size_t i = 0; i < max_i; ++i) {
    if (event_sent_list[i].path_type != wtr::event::path_type::watcher) {
      if (event_sent_list[i].path_name != event_recv_list[i].path_name)
        std::cerr << "Bad .path_name field... (" << event_sent_list[i].path_name
                  << " vs " << event_recv_list[i].path_name << "):"
                  << "\n Sent:\n " << event_sent_list[i] << "\n Received:\n "
                  << event_recv_list[i] << "\n";
      if (event_sent_list[i].effect_type != event_recv_list[i].effect_type)
        std::cerr << "Bad .effect_type field... ("
                  << event_sent_list[i].effect_type << " vs "
                  << event_recv_list[i].effect_type << "):"
                  << "\n Sent:\n " << event_sent_list[i] << "\n Received:\n "
                  << event_recv_list[i] << "\n";
      if (event_sent_list[i].path_type != event_recv_list[i].path_type)
        std::cerr << "Bad .path_type field... (" << event_sent_list[i].path_type
                  << " vs " << event_recv_list[i].path_type << "):"
                  << "\n Sent:\n " << event_sent_list[i] << "\n Received:\n "
                  << event_recv_list[i] << "\n";
      CHECK(event_sent_list[i].path_name == event_recv_list[i].path_name);
      CHECK(event_sent_list[i].effect_type == event_recv_list[i].effect_type);
      CHECK(event_sent_list[i].path_type == event_recv_list[i].path_type);
    }
  }

  CHECK(event_sent_list.size() == event_recv_list.size());
}

// Same as above, but uses a `std::set` to check for event equality
// *regardless of position in the container*
inline auto check_event_lists_set_eq(
  auto const& event_sent_list,
  auto const& event_recv_list)
{
  std::unordered_set<wtr::watcher::event_without_time> sent_set;
  for (auto const& event : event_sent_list)
    sent_set.insert(
      event_without_time{event.path_name, event.path_type, event.effect_type});
  std::unordered_set<wtr::watcher::event_without_time> recv_set;
  for (auto const& event : event_recv_list)
    recv_set.insert(
      event_without_time{event.path_name, event.path_type, event.effect_type});
  for (auto const& event : sent_set) {
    auto has = recv_set.contains(event);
    CHECK(has);
#if 0
    if (! has)
      std::cerr << "Missing event in recv list: " << to_string(event) << "\n";
#else
    if (! has)
      std::cerr << "Missing event in recv list: " << to<std::string>(event)
                << "\n";
#endif
  }
  for (auto const& event : recv_set) {
    auto has = sent_set.contains(event);
    CHECK(has);
#if 0
    if (! has)
      std::cerr << "Missing event in sent list: " << to_string(event) << "\n";
#else
    if (! has)
      std::cerr << "Missing event in sent list: " << to<std::string>(event)
                << "\n";
#endif
  }
  CHECK(sent_set == recv_set);
}

inline auto check_event_lists_eq(auto const& lists)
{
  auto [event_sent_list, event_recv_list] = lists;
  check_event_lists_eq(event_sent_list, event_recv_list);
}

inline auto check_event_lists_set_eq(auto const& lists)
{
  auto [event_sent_list, event_recv_list] = lists;
  check_event_lists_set_eq(event_sent_list, event_recv_list);
}

} /* namespace test_watcher */
} /* namespace wtr */
