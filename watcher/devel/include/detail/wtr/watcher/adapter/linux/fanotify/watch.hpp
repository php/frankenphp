#pragma once

#if (defined(__linux__) || __ANDROID_API__) \
  && ! defined(WATER_WATCHER_USE_WARTHOG)

#include <linux/version.h>

#if (KERNEL_VERSION(5, 9, 0) <= LINUX_VERSION_CODE) && ! __ANDROID_API__

#include "wtr/watcher.hpp"
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdio.h>
#include <string.h>
#include <string>
#include <sys/epoll.h>
#include <sys/fanotify.h>
#include <unistd.h>

namespace detail::wtr::watcher::adapter::fanotify {

/*  We request post-event reporting, non-blocking
    IO and unlimited marks for fanotify. We need
    sudo mode for the unlimited marks. If we were
    making a filesystem auditor, we might use:
      FAN_CLASS_PRE_CONTENT
      | FAN_UNLIMITED_QUEUE
      | FAN_UNLIMITED_MARKS
    and some writable fields within the callback.
*/

// clang-format off
struct ke_fa_ev {
  static constexpr auto buf_len = 4096;
  static constexpr auto c_ulim = buf_len / sizeof(fanotify_event_metadata);
  static constexpr auto init_io_flags
    = O_RDONLY
    | O_CLOEXEC
    | O_NONBLOCK;
  static constexpr auto init_flags
    = FAN_CLASS_NOTIF
    | FAN_REPORT_FID
    | FAN_REPORT_DIR_FID
    | FAN_REPORT_NAME
    | FAN_UNLIMITED_QUEUE
    | FAN_UNLIMITED_MARKS
    | FAN_NONBLOCK;
  /*  todo: Support change of ownership w/ FAN_ATTRIB */
  static constexpr auto recv_flags
    = FAN_ONDIR
    | FAN_CREATE
    | FAN_MODIFY
    | FAN_MOVE
    | FAN_DELETE
    | FAN_EVENT_ON_CHILD;

  int fd = -1;
  alignas(fanotify_event_metadata) char buf[buf_len]{0};
};

// clang-format on

struct sysres {
  result ok = result::e;
  ke_fa_ev ke{};
  semabin const& il{};
  adapter::ep ep{};
};

inline auto do_mark =
  [](char const* const dirpath, int fa_fd, auto const& cb) -> result
{
  auto e = result::w_sys_not_watched;
  char real[PATH_MAX];
  int anonymous_wd =
    realpath(dirpath, real) && is_dir(real)
      ? fanotify_mark(fa_fd, FAN_MARK_ADD, ke_fa_ev::recv_flags, AT_FDCWD, real)
      : -1;
  if (anonymous_wd == 0)
    return result::complete;
  else
    return send_msg(e, dirpath, cb), e;
};

/*  Grabs the resources we need from `fanotify`
    and `epoll`. Marks itself invalid on errors,
    sends diagnostics on warnings and errors.
    Walks the given base path, recursively,
    marking each directory along the way. */
inline auto make_sysres = [](
                            char const* const base_path,
                            auto const& cb,
                            semabin const& living) -> sysres
{
  int fa_fd = fanotify_init(ke_fa_ev::init_flags, ke_fa_ev::init_io_flags);
  if (fa_fd < 1) return sysres{.ok = result::e_sys_api_fanotify, .il = living};
  walkdir_do(base_path, [&](auto dir) { do_mark(dir, fa_fd, cb); });
  auto ep = make_ep(fa_fd, living.fd);
  if (ep.fd < 1)
    return close(fa_fd), sysres{.ok = result::e_sys_api_epoll, .il = living};
  return sysres{
    .ok = result::pending,
    .ke{.fd = fa_fd},
    .il = living,
    .ep = ep,
  };
};

/*  Parses a full path from an event's metadata.
    The shenanigans we do here depend on this event being
    `FAN_EVENT_INFO_TYPE_DFID_NAME`. The kernel passes us
    some info about the directory and the directory entry
    (the filename) of this event that doesn't exist when
    other event types are reported. In particular, we need
    a file descriptor to the directory (which we use
    `readlink` on) and a character string representing the
    name of the directory entry.
    TLDR: We need information for the full path of the
    event, information which is only reported inside this
    `if`.
    From the kernel:
    Variable size struct for
    dir file handle + child file handle + name
    [ Omitting definition of `fanotify_info` here ]
    (struct fanotify_fh)
      dir_fh starts at
        buf[0] (optional)
      dir2_fh starts at
        buf[dir_fh_totlen] (optional)
      file_fh starts at
        buf[dir_fh_totlen+dir2_fh_totlen]
      name starts at
        buf[dir_fh_totlen+dir2_fh_totlen+file_fh_totlen]
      ...
    The kernel guarentees that there is a null-terminated
    character string to the event's directory entry
    after the file handle to the directory.
    Confusing, right? */
inline auto
pathof(fanotify_event_metadata const* const mtd, int* ec) -> std::string
{
  constexpr size_t path_ulim = PATH_MAX - sizeof('\0');
  constexpr int ofl = O_RDONLY | O_CLOEXEC | O_PATH;
  auto dir_info = (fanotify_event_info_fid*)(mtd + 1);
  auto dir_fh = (file_handle*)(dir_info->handle);
  char path_buf[PATH_MAX];
  ssize_t file_name_offset = 0;
  /*  Directory name */
  int fd = open_by_handle_at(AT_FDCWD, dir_fh, ofl);
  if (fd <= 0) {
    *ec = -errno;
    return {};
  }
  char fs_ev_pidpath[32] = {0};
  snprintf(fs_ev_pidpath, sizeof(fs_ev_pidpath), "/proc/self/fd/%d", fd);
  file_name_offset = readlink(fs_ev_pidpath, path_buf, path_ulim);
  close(fd);
  path_buf[file_name_offset] = 0;
  /*  File name ("Directory entry")
      If we wrote the directory name before here, we
      can start writing the file name after its offset. */
  if (file_name_offset > 0) {
    char* file_name = ((char*)dir_fh->f_handle + dir_fh->handle_bytes);
    auto not_selfdir = strcmp(file_name, ".") != 0;
    auto beg = path_buf + file_name_offset;
    auto end = PATH_MAX - file_name_offset;
    if (file_name && not_selfdir) snprintf(beg, end, "/%s", file_name);
  }
  return {path_buf};
}

inline auto peek(fanotify_event_metadata const* const m, size_t read_len)
  -> fanotify_event_metadata const*
{
  if (m) {
    auto ev_len = m->event_len;
    auto next = (fanotify_event_metadata*)((char*)m + ev_len);
    return FAN_EVENT_OK(next, read_len - ev_len) ? next : nullptr;
  }
  else
    return nullptr;
}

struct Parsed {
  ::wtr::watcher::event ev;
  fanotify_event_metadata const* next = nullptr;
  unsigned this_len = 0;
};

// The error code is -errno if there was a system error which prevents
// correctly parsing the event, or a positive error if the event was
// malformed or unexpected (without a system error).
inline auto
parse_ev(fanotify_event_metadata const* const m, size_t read_len, int* ec)
  -> Parsed
{
  using ev = ::wtr::watcher::event;
  using ev_pt = enum ev::path_type;
  using ev_et = enum ev::effect_type;
  auto n = peek(m, read_len);
  auto pt = m->mask & FAN_ONDIR ? ev_pt::dir : ev_pt::file;
  auto et = m->mask & FAN_CREATE ? ev_et::create
          : m->mask & FAN_DELETE ? ev_et::destroy
          : m->mask & FAN_MODIFY ? ev_et::modify
          : m->mask & FAN_MOVE   ? ev_et::rename
                                 : ev_et::other;
  auto isfromto = [et](unsigned a, unsigned b) -> bool
  { return et == ev_et::rename && a & FAN_MOVED_FROM && b & FAN_MOVED_TO; };
  auto one = [&](auto* m) -> Parsed
  { return {ev(pathof(m, ec), et, pt), n, m->event_len}; };
  auto assoc = [&](auto* m, auto* n) -> Parsed
  {
    auto nn = peek(n, read_len);
    auto here_to_nnn = m->event_len + n->event_len;
    auto e = ev(ev(pathof(m, ec), et, pt), ev(pathof(n, ec), et, pt));
    return {e, nn, here_to_nnn};
  };
  return et != ev_et::rename        ? one(m)
       : ! n                        ? (*ec = 2, one(m))
       : isfromto(m->mask, n->mask) ? assoc(m, n)
       : isfromto(n->mask, m->mask) ? assoc(n, m)
       : (*ec = 2, one(m));
}

inline auto is_newdir = [](::wtr::watcher::event const& ev) -> bool
{
  return ev.effect_type == ::wtr::watcher::event::effect_type::create
      && ev.path_type == ::wtr::watcher::event::path_type::dir;
};

/*  Read some events from what fanotify gives
    us. Sends (the valid) events to the user's
    callback. Send a diagnostic to the user on
    warnings and errors. Returns false on errors.
    True otherwise.
    Notes:
    The `metadata->fd` field contains either a
    file descriptor or the value `FAN_NOFD`. File
    descriptors are always greater than 0 (but we
    will get the FAN_NOFD value, and we can grab
    other information from the /proc/self/<fd>
    filesystem, which we do in `pathof()`).
    `FAN_NOFD` represents an event queue overflow
    for `fanotify` listeners which are *not*
    monitoring file handles, such as mount/volume
    watchers. The file handle is in the metadata
    when an `fanotify` listener is monitoring
    events by their file handles.
    The `metadata->vers` field may differ between
    kernel versions, so we check it against the
    version we were compiled with. */
inline auto do_ev_recv = [](auto const& cb, sysres& sr) -> result
{
  auto ev_info = [](fanotify_event_metadata const* const m)
  { return (fanotify_event_info_fid*)(m + 1); };
  auto ev_has_dirname = [&](fanotify_event_metadata const* const m) -> bool
  { return ev_info(m)->hdr.info_type == FAN_EVENT_INFO_TYPE_DFID_NAME; };

  unsigned read_ev_count = 0;
  int read_len = read(sr.ke.fd, sr.ke.buf, sr.ke.buf_len);
  auto const* mtd = (fanotify_event_metadata*)(sr.ke.buf);
  if (read_len <= 0 && errno != EAGAIN)
    return result::pending;
  else if (read_len < 0)
    return result::e_sys_api_read;
  else
    while (mtd && FAN_EVENT_OK(mtd, read_len))
      if (read_ev_count++ > sr.ke.c_ulim)
        return result::e_sys_ret;
      else if (mtd->vers != FANOTIFY_METADATA_VERSION)
        return result::e_sys_lim_kernel_version;
      else if (mtd->fd != FAN_NOFD)
        return result::w_sys_bad_fd;
      else if (mtd->mask & FAN_Q_OVERFLOW)
        return result::w_sys_q_overflow;
      else if (! ev_has_dirname(mtd))
        return result::w_sys_bad_meta;
      else {
        int ec = 0;
        auto r = parse_ev(mtd, read_len, &ec);
        if (ec < 0) return result::w_sys_bad_fd;
        if (is_newdir(r.ev))
          walkdir_do(r.ev.path_name.c_str(), [&](auto dir) {
            do_mark(dir, sr.ke.fd, cb);
            cb({dir, r.ev.effect_type, r.ev.path_type});
          });
        else if (ec == 0)
          cb(r.ev);
        mtd = r.next;
        read_len -= r.this_len;
      }
  return result::pending;
};

} /*  namespace detail::wtr::watcher::adapter::fanotify */

#endif
#endif
