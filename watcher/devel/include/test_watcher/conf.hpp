#pragma once

#include <cmath>
#include <string>

namespace wtr {
namespace test_watcher {

auto seperator(auto const& test_name, char const symbol = '-')
{
  auto len = 79; /* same as catch2 */
  auto title = std::string("[ watcher/test/") + std::string(test_name) + " ]";
  auto gap_len = std::round((len - title.length()) / 2);
  auto side_bar = std::string(gap_len, symbol);
  return side_bar + title + side_bar;
}

} /* namespace test_watcher */
} /* namespace wtr */
