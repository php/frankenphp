#pragma once

#if (defined(__linux__) || __ANDROID_API__) \
  && ! defined(WATER_WATCHER_USE_WARTHOG)

#include <linux/version.h>

#if KERNEL_VERSION(2, 7, 0) > LINUX_VERSION_CODE
#error "Define 'WATER_WATCHER_USE_WARTHOG' on kernel versions < 2.7.0"
#endif

#include "wtr/watcher.hpp"
#include <unistd.h>

namespace detail::wtr::watcher::adapter {

inline auto watch =
  [](auto const& path, auto const& cb, auto const& living) -> bool
{
  auto platform_watch = [&](auto make_sysres, auto do_ev_recv) -> result
  {
    auto sr = make_sysres(path.c_str(), cb, living);
    auto is_ev_of = [&](int nth, int fd) -> bool
    { return sr.ep.interests[nth].data.fd == fd; };

    while (sr.ok < result::complete) {
      int ep_c =
        epoll_wait(sr.ep.fd, sr.ep.interests, sr.ep.q_ulim, sr.ep.wake_ms);
      if (ep_c < 0) {
        if (errno != EINTR) sr.ok = result::e_sys_api_epoll;
      }
      else
        for (int n = 0; n < ep_c; ++n)
          if (is_ev_of(n, sr.il.fd))
            sr.ok = result::complete;
          else if (is_ev_of(n, sr.ke.fd))
            sr.ok = do_ev_recv(cb, sr);
          else
            sr.ok = result::e_sys_api_epoll;
    }

    /*  We aren't worried about losing data after
        a failed call to `close()` (whereas closing
        a file descriptor in use for, say, writing
        would be a problem). Linux will eventually
        close the file descriptor regardless of the
        return value of `close()`.
        We are only interested in failures about
        bad file descriptors. We would probably
        hit that if we failed to create a valid
        file descriptor, on, say, an out-of-fds
        device or a machine running some odd OS.
        Everything else is fine to pass on, and
        the out-of-file-descriptors case will be
        handled in `make_sysres()`.
    */
    return close(sr.ke.fd), close(sr.ep.fd), sr.ok;
  };

  /*  e_sys_api_fanotify

      It is possible to be unable to use fanotify, even
      with the CAP_SYS_ADMIN capability. For example,
      the system calls we need may be limited because
      we're running within a cgroup.
      IOW -- We're probably inside a container if we're
      root but lack permission to use fanotify.

      The same error applies to being built on a kernel
      which doesn't have the fanotify api.
  */
  auto try_fanotify = [&]()
  {
#if (KERNEL_VERSION(5, 9, 0) <= LINUX_VERSION_CODE) && ! __ANDROID_API__
    if (geteuid() == 0)
      return platform_watch(fanotify::make_sysres, fanotify::do_ev_recv);
#endif
    return result::e_sys_api_fanotify;
  };

  auto r = try_fanotify();
  if (r == result::e_sys_api_fanotify)
    r = platform_watch(inotify::make_sysres, inotify::do_ev_recv);
  if (r >= result::e)
    return send_msg(r, path.c_str(), cb), false;
  else
    return true;
};

} /*  namespace detail::wtr::watcher::adapter */

#endif
