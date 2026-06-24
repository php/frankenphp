#pragma once

#include "wtr/watcher.hpp"
#include <filesystem>
#include <future>

namespace wtr {
inline namespace watcher {

/*  An asynchronous filesystem watcher.

    Begins watching when constructed.

    Stops watching when the object's lifetime ends
    or when `.close()` is called.

    Closing the watcher is the only blocking operation.

    @param path:
      The root path to watch for filesystem events.

    @param callback:
      Something (such as a closure) to be called when events
      occur in the path being watched.

    This is an adaptor "switch" that chooses the ideal
    adaptor for the host platform.

    Every adapter monitors `path` for changes and invokes
    the `callback` with an `event` object when they occur.

    There are two things the user needs: `watch` and
    `event`.

    Typical use looks something like this:

      auto show(event e) {
        auto s = [](auto a) { return to<string>(a); };
        cout << s(e.effect_type) + ' '
              + s(e.path_type)   + ' '
              + s(e.path_name)
              + (e.associated
                 ? " -> " + s(e.associated->path_name)
                 : "")
             << endl;
      }

      auto w = watch(".", show);

    That's it.

    Happy hacking. */
class watch {
private:
  using sb = ::detail::wtr::watcher::semabin;
  sb living{};
  std::future<bool> watching{};

public:
  inline watch(
    std::filesystem::path const& path,
    event::callback const& callback) noexcept
      : watching{std::async(
          std::launch::async,
          [this, path, callback]
          {
            using ::detail::wtr::watcher::adapter::watch;
            auto ec = std::error_code{};
            auto abs_path = std::filesystem::absolute(path, ec);
            auto pre_ok = ! ec && std::filesystem::is_directory(abs_path, ec)
                       && ! ec && this->living.state() == sb::state::pending;
            auto live_msg =
              (pre_ok ? "s/self/live@" : "e/self/live@") + abs_path.string();
            callback(
              {live_msg,
               event::effect_type::create,
               event::path_type::watcher});
            auto post_ok = pre_ok && watch(abs_path, callback, this->living);
            auto die_msg =
              (post_ok ? "s/self/die@" : "e/self/die@") + abs_path.string();
            callback(
              {die_msg,
               event::effect_type::destroy,
               event::path_type::watcher});
            return pre_ok && post_ok;
          })}
  {}

  inline auto close() noexcept -> bool
  {
    return this->living.release() != sb::state::error
        && this->watching.valid() && this->watching.get();
  };

  inline ~watch() noexcept { this->close(); }
};

} /*  namespace watcher */
} /*  namespace wtr   */
