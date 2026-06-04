#pragma once

#if (defined(__linux__) || __ANDROID_API__) \
  && ! defined(WATER_WATCHER_USE_WARTHOG)

#include "wtr/watcher.hpp"
#include <dirent.h>
#include <errno.h>
#include <limits.h>
#include <stdio.h>
#include <string.h>
#include <string>
#include <sys/epoll.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <utility>

namespace detail::wtr::watcher::adapter {

/*  Must maintain that an enumerated value
    less than complete is either pending or
    a warning (which means "keep running"),
    that complete is a successful, terminal
    result (means "stop, ok"), and that a
    value greater than complete is an error
    which we should send along to the user
    after we (attempt to) clean up. */
enum class result : unsigned short {
  pending = 0,
  w,
  w_sys_not_watched,
  w_sys_phantom,
  w_sys_bad_fd,
  w_sys_bad_meta,
  w_sys_q_overflow,
  w_self_q_overflow,
  w_sys_partial,
  complete,
  e,
  e_sys_api_inotify,
  e_sys_api_fanotify,
  e_sys_api_epoll,
  e_sys_api_read,
  e_sys_api_eventfd,
  e_sys_ret,
  e_sys_lim_kernel_version,
  e_self_noent,
  e_self_ev_recv,
};

struct ep {
  /*  Number of events allowed to be
      given to do_event_recv. The same
      value is usually returned by a
      call to `epoll_wait`). Any number
      between 1 and INT_MAX should be
      fine... But lower is fine for us.
      We don't lose events if we 'miss'
      them, the events are still waiting
      in the next call to `epoll_wait`.
  */
  static constexpr auto q_ulim = 64;
  /*  The delay, in milliseconds, while
      `epoll_wait` will 'pause' us for
      until we are woken up. We check if
      we are still alive through the fd
      from our semaphore-like eventfd.
  */
  static constexpr auto wake_ms = -1;

  int fd = -1;
  epoll_event interests[q_ulim]{};
};

inline constexpr auto to_str(result r)
{
  // clang-format off
  switch (r) {
    case result::pending:                            return "pending@";
    case result::w:                                  return "w@";
    case result::w_sys_not_watched:                  return "w/sys/not_watched@";
    case result::w_sys_phantom:                      return "w/sys/phantom@";
    case result::w_sys_bad_fd:                       return "w/sys/bad_fd@";
    case result::w_sys_bad_meta:                     return "w/sys/bad_meta@";
    case result::w_sys_q_overflow:                   return "w/sys/q_overflow@";
    case result::w_self_q_overflow:                  return "w/self/q_overflow@";
    case result::w_sys_partial:                      return "w/sys/partial@";
    case result::complete:                           return "complete@";
    case result::e:                                  return "e@";
    case result::e_sys_api_inotify:                  return "e/sys/api/inotify@";
    case result::e_sys_api_fanotify:                 return "e/sys/api/fanotify@";
    case result::e_sys_api_epoll:                    return "e/sys/api/epoll@";
    case result::e_sys_api_read:                     return "e/sys/api/read@";
    case result::e_sys_api_eventfd:                  return "e/sys/api/eventfd@";
    case result::e_sys_ret:                          return "e/sys/ret@";
    case result::e_sys_lim_kernel_version:           return "e/sys/lim/kernel_version@";
    case result::e_self_noent:                       return "e/self/noent@";
    case result::e_self_ev_recv:                     return "e/self/ev_recv@";
    default:                                         return "e/unknown@";
  }
  // clang-format on
};

inline auto send_msg = [](result r, auto path, auto const& cb)
{
  using et = enum ::wtr::watcher::event::effect_type;
  using pt = enum ::wtr::watcher::event::path_type;
  auto msg = std::string{to_str(r)};
  cb({msg + path, et::other, pt::watcher});
};

inline auto make_ep(int ev_fs_fd, int ev_il_fd) -> ep
{
#if __ANDROID_API__
  int fd = epoll_create(1);
#else
  int fd = epoll_create1(EPOLL_CLOEXEC);
#endif
  auto want_ev_fs = epoll_event{.events = EPOLLIN, .data{.fd = ev_fs_fd}};
  auto want_ev_il = epoll_event{.events = EPOLLIN, .data{.fd = ev_il_fd}};
  bool ctl_ok = fd >= 0
             && epoll_ctl(fd, EPOLL_CTL_ADD, ev_fs_fd, &want_ev_fs) >= 0
             && epoll_ctl(fd, EPOLL_CTL_ADD, ev_il_fd, &want_ev_il) >= 0;
  if (! ctl_ok && fd >= 0) close(fd), fd = -1;
  return ep{.fd = fd};
}

inline auto is_dir(char const* const path) -> bool
{
  struct stat s;
  return stat(path, &s) == 0 && S_ISDIR(s.st_mode);
}

/*  $ echo time wtr.watcher / -ms 1
      | sudo bash -E
      ...
      real 0m25.094s
      user 0m4.091s
      sys  0m20.856s

    $ sudo find / -type d
      | wc -l
      ...
      784418

    We could parallelize this, but
    it's never going to be instant.

    It might be worth it to start
    watching before we're done this
    hot find-and-mark path, despite
    not having a full picture.
*/
template<class Fn>
inline auto walkdir_do(char const* const path, Fn const& f) -> void
{
  if (DIR* d = opendir(path)) {
    f(path);
    while (dirent* de = readdir(d)) {
      char next[PATH_MAX];
      char real[PATH_MAX];
      if (de->d_type != DT_DIR) continue;
      if (strcmp(de->d_name, ".") == 0) continue;
      if (strcmp(de->d_name, "..") == 0) continue;
      if (snprintf(next, PATH_MAX, "%s/%s", path, de->d_name) <= 0) continue;
      if (! realpath(next, real)) continue;
      walkdir_do(real, f);
    }
    (void)closedir(d);
  }
}

} /*  namespace detail::wtr::watcher::adapter */

#endif
