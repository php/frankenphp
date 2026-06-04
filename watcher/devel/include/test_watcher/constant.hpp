#pragma once

#include <chrono>
#include <filesystem>

namespace wtr {
namespace test_watcher {

inline constexpr unsigned long mk_events_options = 0x00000000;
inline constexpr unsigned long mk_events_die_before = 0x00000001;
inline constexpr unsigned long mk_events_die_after = 0x00000010;
inline constexpr unsigned long mk_events_reverse = 0x00000100;

} /* namespace test_watcher */
} /* namespace wtr */
