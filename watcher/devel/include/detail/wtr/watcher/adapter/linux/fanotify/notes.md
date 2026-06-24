# Notes

## Stat Comparisons

```
enum class Attrib {
  dev,
  ino,
  mode,
  nlink,
  uid,
  gid,
  rdev,
  size,
  blksize,
  blocks,
  atim,
  mtim,
  ctim,
  none,
};

inline auto
which_attribs(struct stat const* const prev, struct stat const* const curr)
  -> std::vector<Attrib>
{
  return {
    (prev->st_dev != curr->st_dev) ? Attrib::dev : Attrib::none,
    (prev->st_ino != curr->st_ino) ? Attrib::ino : Attrib::none,
    (prev->st_mode != curr->st_mode) ? Attrib::mode : Attrib::none,
    (prev->st_nlink != curr->st_nlink) ? Attrib::nlink : Attrib::none,
    (prev->st_uid != curr->st_uid) ? Attrib::uid : Attrib::none,
    (prev->st_gid != curr->st_gid) ? Attrib::gid : Attrib::none,
    (prev->st_rdev != curr->st_rdev) ? Attrib::rdev : Attrib::none,
    (prev->st_size != curr->st_size) ? Attrib::size : Attrib::none,
    (prev->st_blksize != curr->st_blksize) ? Attrib::blksize : Attrib::none,
    (prev->st_blocks != curr->st_blocks) ? Attrib::blocks : Attrib::none,
    (prev->st_atim.tv_sec != curr->st_atim.tv_sec
     || prev->st_atim.tv_nsec != curr->st_atim.tv_nsec)
      ? Attrib::atim
      : Attrib::none,
    (prev->st_mtim.tv_sec != curr->st_mtim.tv_sec
     || prev->st_mtim.tv_nsec != curr->st_mtim.tv_nsec)
      ? Attrib::mtim
      : Attrib::none,
    (prev->st_ctim.tv_sec != curr->st_ctim.tv_sec
     || prev->st_ctim.tv_nsec != curr->st_ctim.tv_nsec)
      ? Attrib::ctim
      : Attrib::none,
  };
}


```

## Fanotify Flag Debugging

```
inline auto dbg_info_hdr =
  [](fanotify_event_metadata const* const mtd, auto const& cb) -> void
{
  return;
  auto info = (fanotify_event_info_fid*)(mtd + 1);
  auto infoty = info->hdr.info_type;
  cb(
    {{std::string("Info Type: ")
      + (infoty & FAN_EVENT_INFO_TYPE_FID ? "FID," : "")
      + (infoty & FAN_EVENT_INFO_TYPE_DFID ? "DFID," : "")
      + (infoty & FAN_EVENT_INFO_TYPE_DFID_NAME ? "DFID_NAME," : "")
      + (infoty & FAN_EVENT_INFO_TYPE_PIDFD ? "PIDFD," : "")},
     ::wtr::watcher::event::effect_type::other,
     ::wtr::watcher::event::path_type::watcher});
};

inline auto dbg_mask =
  [](fanotify_event_metadata const* const mtd, auto const& cb) -> void
{
  return;
  unsigned mask = mtd->mask;
  cb(
    {{std::string("Flags: ") + (mask & FAN_ACCESS ? "ACCESS," : "")
      + (mask & FAN_OPEN ? "OPEN," : "")
      + (mask & FAN_OPEN_EXEC ? "OPEN_EXEC," : "")
      + (mask & FAN_ATTRIB ? "ATTRIB," : "")
      + (mask & FAN_CREATE ? "CREATE," : "")
      + (mask & FAN_DELETE ? "DELETE," : "")
      + (mask & FAN_DELETE_SELF ? "DELETE_SELF," : "")
      + (mask & FAN_FS_ERROR ? "FS_ERROR," : "")
      + (mask & FAN_RENAME ? "RENAME," : "")
      + (mask & FAN_MOVED_FROM ? "MOVED_FROM," : "")
      + (mask & FAN_MOVED_TO ? "MOVED_TO," : "")
      + (mask & FAN_MOVE_SELF ? "MOVE_SELF," : "")
      + (mask & FAN_MODIFY ? "MODIFY," : "")
      + (mask & FAN_CLOSE_WRITE ? "CLOSE_WRITE," : "")
      + (mask & FAN_CLOSE_NOWRITE ? "CLOSE_NOWRITE," : "")
      + (mask & FAN_Q_OVERFLOW ? "Q_OVERFLOW," : "")
      + (mask & FAN_UNLIMITED_QUEUE ? "UNLIMITED_QUEUE," : "")
      + (mask & FAN_ACCESS_PERM ? "ACCESS_PERM," : "")
      + (mask & FAN_OPEN_PERM ? "OPEN_PERM," : "")
      + (mask & FAN_OPEN_EXEC_PERM ? "OPEN_EXEC_PERM," : "")
      + (mask & FAN_CLOSE ? "CLOSE," : "")
      // + (fan_flags & FAN_MOVE ? "MOVE," : "") // Synonym for from+to
      + (mask & FAN_ONDIR ? "ONDIR," : "")},
     ::wtr::watcher::event::effect_type::other,
     ::wtr::watcher::event::path_type::watcher});
};

```
