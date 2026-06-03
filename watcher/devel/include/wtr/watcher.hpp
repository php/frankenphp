#pragma once

/*  This is the public interface.
    Include and use this file. */

// clang-format off
#include "wtr/watcher-/event.hpp"
#include "detail/wtr/watcher/semabin.hpp"
#include "detail/wtr/watcher/adapter/darwin/watch.hpp"
#include "detail/wtr/watcher/adapter/linux/sysres.hpp"
#include "detail/wtr/watcher/adapter/linux/fanotify/watch.hpp"
#include "detail/wtr/watcher/adapter/linux/inotify/watch.hpp"
#include "detail/wtr/watcher/adapter/linux/watch.hpp"
#include "detail/wtr/watcher/adapter/windows/watch.hpp"
#include "detail/wtr/watcher/adapter/warthog/watch.hpp"
#include "wtr/watcher-/watch.hpp"
// clang-format on
