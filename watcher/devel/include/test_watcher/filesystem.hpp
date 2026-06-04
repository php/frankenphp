#pragma once

#include "wtr/watcher.hpp"
#include <filesystem>
#include <fstream>
#include <random>
#include <string>

namespace wtr {
namespace test_watcher {

/*  Best to use local temporary directories because
    some platforms do not allow the system temporaries
    to be modified beyond creation */
inline auto make_local_tmp_dir()
{
  static auto dist = std::uniform_int_distribution<int>{0, 1000000};
  static auto gen = std::mt19937{std::random_device{}()};
  auto n = dist(gen);
  auto str_n = std::to_string(n);
  auto cur_path = std::filesystem::current_path();
  auto tmp_dir = cur_path / "out" / str_n;
  auto abs_tmp_dir = std::filesystem::absolute(tmp_dir);
  std::filesystem::create_directory(abs_tmp_dir.parent_path());
  std::filesystem::create_directory(abs_tmp_dir);
  return abs_tmp_dir;
}

inline auto create_regular_files(auto const& path, auto n)
{
  for (int i = 0; i < n; i++)
    std::ofstream{path + std::to_string(i) + std::string(".txt")};
}

inline auto create_regular_files(std::filesystem::path const& path, auto n)
{
  return create_regular_files(path.string(), n);
}

inline auto create_regular_files(std::vector<wtr::event>& events)
{
  for (auto& ev : events) std::ofstream{ev.path_name};
}

inline auto create_regular_files(auto const& paths)
{
  for (auto const& path : paths) std::ofstream{path};
}

inline auto create_directories(auto const& path, auto n)
{
  for (decltype(n) i = 0; i < n; i++)
    std::filesystem::create_directory(path / std::to_string(i));
}

} /* namespace test_watcher */
} /* namespace wtr */
