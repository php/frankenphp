# Notes

## Darwin's Native Inter-Process Communication (Also inter-thread, ofc)

```
inline CFDataRef on_listening_port_response(
  CFMessagePortRef,  // on_port
  SInt32,            // msgid
  CFDataRef req,
  void*  // ctx
)
{
  long const len = 2;
  if (! req || CFDataGetLength(req) < len) { return nullptr; }
  // Should always be 2, but let wiggles happen.
  CFRange req_range = CFRangeMake(0, len);
  unsigned char req_bytes[len] = {0};
  CFDataGetBytes(req, req_range, req_bytes);
  req_bytes[len - 1] = 0;
  unsigned char res_byte_head = req_bytes[0] == 'x' ? 'o' : '?';
  unsigned char res_bytes[len] = {res_byte_head, 0};
  CFDataRef res = CFDataCreate(NULL, res_bytes, sizeof(res_bytes));
  return res;
}

inline CFMessagePortRef open_listening_port(char const* const port_name)
{
  CFStringRef cf_port_name =
    CFStringCreateWithCString(NULL, port_name, kCFStringEncodingASCII);

  CFMessagePortContext context = {0, nullptr, nullptr, nullptr, nullptr};
  CFMessagePortRef port = CFMessagePortCreateLocal(
    kCFAllocatorDefault,
    cf_port_name,
    on_listening_port_response,
    &context,
    nullptr);
  CFRunLoopSourceRef source =
    CFMessagePortCreateRunLoopSource(kCFAllocatorDefault, port, 0);
  auto sched = CFRunLoopGetCurrent();
  CFRunLoopAddSource(sched, source, kCFRunLoopCommonModes);
  CFRelease(source);
  CFRelease(cf_port_name);

  return port;
}

inline void send_msg_to_port(
  CFMessagePortRef to_port,
  unsigned char const* const message,
  unsigned message_len)
{
  CFDataRef data = CFDataCreate(NULL, message, message_len);
  CFMessagePortSendRequest(to_port, 0, data, 1, 1, kCFRunLoopDefaultMode, NULL);
  CFRelease(data);
}
```

## The "Rename Triplet"

```
  $ tree .
  .
  ├── a
  └── b
  When we cast:
  $ mv a b
```

A watcher tends to see three events from the kernel.
They tend to all be flagged with the rename type,
and they tend to happen in this order:
```
  a -> b -> b
```
It seems like a strong and consistent enough pattern.
I'll call this the "rename triplet".

The pattern breaks when events come rapidly or out
of order. Something like this is enough to see it:
```
  $ touch a
    mv a b
    mv b c
    mv c d
    mv d e
    mv e f
    mv f a # <- (Another caveat here later on)
```

We have a backup plan, though. We store the last
relevant path we saw, checking to see if it exists
in "this" rename event. If it doesn't, we assume
that we were renamed from that path.
The rename pair and the "lost last path" won't
usually be true at the same time. The pair's two
last paths are the same, and should exist. (I say
"usually" because there can be wild things at play;
some very high-priority process might remove a file
right before we `access` is to see if it's there,
scheduling might not be deterministic, blah, etc.)

## A nice `stat` function

```
inline auto stat(char const* const path)
{
  struct stat statbuf;
  memset(&statbuf, 0, sizeof(statbuf));
  stat(path, &statbuf);
  return statbuf;
}
```

## A "more full" implementation of the "Rename Triplet"

This includes the "total" rollover. The "total" rollover is
the only way to tell if a path was renamed *and* overwrote an
existing path. This implementation is missing some `stat`
checks -- It could use some re-assurances from matching inodes
and block sizes.

```
struct rename_triplet {
  std::hash<char const*> hash{};
  size_t hash0{};
  size_t hash1{};
  size_t hash2{};
  char from_path[PATH_MAX]{};
  char to_path[PATH_MAX]{};
  enum class result {
    none,
    from,
    to,
    total,
  } result{result::none};
  inline auto pathcp(char* dst, char const* const src, size_t nbytes) -> char const* {
    assert(nbytes < PATH_MAX);
    assert(nbytes > 0);
    assert(dst != nullptr);
    assert(src != nullptr);
    assert(dst != src);
    assert(dst < src || dst > src + nbytes);
    auto* at = static_cast<char const*>(memcpy(dst, src, nbytes));
    dst[nbytes] = 0;
    return at;
  }
  inline auto reset_state() -> void {
      hash0 = hash1 = hash2
    = from_path[0] = to_path[0]
    = 0;
    result = result::none;
  }
  // The "rename triplet" is "total" when three
  // consecutive calls to `rollover` satisfy:
  // - (On the first call)
  //   The first hash we received was unique,
  //   and we had no state when we received it.
  //   We store this path as the "from" path,
  //   and store the state as state::from.
  // - (On the second call)
  //   The second hash we received was *not* the same as the first,
  //   and we had state::from when we received it.
  //   We store this path as the "to" path,
  //   and store the state as state::to.
  // - (On the third call)
  //   The third hash we received was the same as the second,
  //   and we had state::to when we received it.
  //   We know the "from" and "to" paths are different.
  //   We've seen the "rename triplet",
  //   and store the state as state::total.
  // The fourth time we call `rollover`, we reset the state.
  // IOW, The fourth call is the same as the first.
  inline auto rollover(char const* const path, size_t path_len, unsigned flags) -> enum result {
    if (! path
     || ! (flags & kFSEventStreamEventFlagItemRenamed)) {
      std::cout << "Not a rename event." << std::endl;
      reset_state();
      return result;
    }
    else {
      if (result == result::total) {
        std::cout << "Already saw a rename triplet." << std::endl;
        reset_state();
      }

      // "Slides" each (hashed) path down one slot.
      // We keep a rolling window of 3 (hashed) paths.
      // We copy the path before hashing to guarentee
      // some pre-and-post conditions, such as size,
      // pointer validity and alignment. Copying here
      // means that we don't need to copy later on.
      // Because of how the triplet pattern works,
      // our last two paths will always be the same.
      // IOW, copying into `to_path` here will produce
      // the same bytes when we're in the `total` or
      // `to` state as copying in their `if` blocks.
      hash0 = hash1;
      hash1 = hash2;
      hash2 = hash(pathcp(to_path, path, path_len));

      if (  hash0 == hash1
         && hash1 != hash2
         && result == result::none)
      {
        pathcp(from_path, path, path_len);
        return result = result::from;
      }
      if (  hash0 != hash1
         && hash1 == hash2
         && result == result::from)
      {
        return result = result::to;
      }
      if (  hash0 == hash1
         && hash1 == hash2
         && result == result::to)
      {
        return result = result::total;
      }

      std::cout << "Not a rename triplet." << std::endl;
      reset_state();
      return result;
    }
  }
  inline auto rollover_dbg(char const* path, size_t path_len, unsigned flags) -> enum result {
    auto r = rollover(path, path_len, flags);

    std::cout
      << "[dbg/rollover]"
      << "\nhash0: " << hash0
      << "\nhash1: " << hash1
      << "\nhash2: " << hash2
      << "\npath: " << path
      << "\nflags: " << flags
      << "\nis rename ? " << (flags & kFSEventStreamEventFlagItemRenamed ? "true" : "false")
      << std::endl;

    dbg_flags(flags);

    if (r == result::from) {
      std::cout << "[dbg/rollover/from] -> partial rollover from " << from_path << std::endl;
    }
    if (r == result::to) {
      std::cout << "[dbg/rollover/to] -> partial rollover to " << to_path << std::endl;
    }
    if (r == result::total) {
      std::cout << "[dbg/rollover/total] => total rollover from " << from_path << " to " << to_path << std::endl;
    }
    return r;
  }
};
```

## Useful `stat` debugging:

```
inline auto stat_cmp_dbg(struct stat& last_stats, struct stat& these_stats) -> void {
  std::cout
    << "\nThese Stats <-> Last Stats: "
    << "\n dev:                   gt? " << (these_stats.st_dev                   > last_stats.st_dev ? " true" : "false")                   << " lt? " << (these_stats.st_dev                   < last_stats.st_dev ? " true" : "false")                   << " eq? " << (these_stats.st_dev                   == last_stats.st_dev ? " true" : "false")                   << " @ " << these_stats.st_dev                   << " (<-these-last->) " << last_stats.st_dev
    << "\n ino:                   gt? " << (these_stats.st_ino                   > last_stats.st_ino ? " true" : "false")                   << " lt? " << (these_stats.st_ino                   < last_stats.st_ino ? " true" : "false")                   << " eq? " << (these_stats.st_ino                   == last_stats.st_ino ? " true" : "false")                   << " @ " << these_stats.st_ino                   << " (<-these-last->) " << last_stats.st_ino
    << "\n mode:                  gt? " << (these_stats.st_mode                  > last_stats.st_mode ? " true" : "false")                  << " lt? " << (these_stats.st_mode                  < last_stats.st_mode ? " true" : "false")                  << " eq? " << (these_stats.st_mode                  == last_stats.st_mode ? " true" : "false")                  << " @ " << these_stats.st_mode                  << " (<-these-last->) " << last_stats.st_mode
    << "\n uid:                   gt? " << (these_stats.st_uid                   > last_stats.st_uid ? " true" : "false")                   << " lt? " << (these_stats.st_uid                   < last_stats.st_uid ? " true" : "false")                   << " eq? " << (these_stats.st_uid                   == last_stats.st_uid ? " true" : "false")                   << " @ " << these_stats.st_uid                   << " (<-these-last->) " << last_stats.st_uid
    << "\n gid:                   gt? " << (these_stats.st_gid                   > last_stats.st_gid ? " true" : "false")                   << " lt? " << (these_stats.st_gid                   < last_stats.st_gid ? " true" : "false")                   << " eq? " << (these_stats.st_gid                   == last_stats.st_gid ? " true" : "false")                   << " @ " << these_stats.st_gid                   << " (<-these-last->) " << last_stats.st_gid
    << "\n rdev:                  gt? " << (these_stats.st_rdev                  > last_stats.st_rdev ? " true" : "false")                  << " lt? " << (these_stats.st_rdev                  < last_stats.st_rdev ? " true" : "false")                  << " eq? " << (these_stats.st_rdev                  == last_stats.st_rdev ? " true" : "false")                  << " @ " << these_stats.st_rdev                  << " (<-these-last->) " << last_stats.st_rdev
    << "\n size:                  gt? " << (these_stats.st_size                  > last_stats.st_size ? " true" : "false")                  << " lt? " << (these_stats.st_size                  < last_stats.st_size ? " true" : "false")                  << " eq? " << (these_stats.st_size                  == last_stats.st_size ? " true" : "false")                  << " @ " << these_stats.st_size                  << " (<-these-last->) " << last_stats.st_size
    << "\n blocks:                gt? " << (these_stats.st_blocks                > last_stats.st_blocks ? " true" : "false")                << " lt? " << (these_stats.st_blocks                < last_stats.st_blocks ? " true" : "false")                << " eq? " << (these_stats.st_blocks                == last_stats.st_blocks ? " true" : "false")                << " @ " << these_stats.st_blocks                << " (<-these-last->) " << last_stats.st_blocks
    << "\n blksize:               gt? " << (these_stats.st_blksize               > last_stats.st_blksize ? " true" : "false")               << " lt? " << (these_stats.st_blksize               < last_stats.st_blksize ? " true" : "false")               << " eq? " << (these_stats.st_blksize               == last_stats.st_blksize ? " true" : "false")               << " @ " << these_stats.st_blksize               << " (<-these-last->) " << last_stats.st_blksize
    << "\n flags:                 gt? " << (these_stats.st_flags                 > last_stats.st_flags ? " true" : "false")                 << " lt? " << (these_stats.st_flags                 < last_stats.st_flags ? " true" : "false")                 << " eq? " << (these_stats.st_flags                 == last_stats.st_flags ? " true" : "false")                 << " @ " << these_stats.st_flags                 << " (<-these-last->) " << last_stats.st_flags
    << "\n gen:                   gt? " << (these_stats.st_gen                   > last_stats.st_gen ? " true" : "false")                   << " lt? " << (these_stats.st_gen                   < last_stats.st_gen ? " true" : "false")                   << " eq? " << (these_stats.st_gen                   == last_stats.st_gen ? " true" : "false")                   << " @ " << these_stats.st_gen                   << " (<-these-last->) " << last_stats.st_gen
    << "\n atimespec.tv_nsec:     gt? " << (these_stats.st_atimespec.tv_nsec     > last_stats.st_atimespec.tv_nsec ? " true" : "false")     << " lt? " << (these_stats.st_atimespec.tv_nsec     < last_stats.st_atimespec.tv_nsec ? " true" : "false")     << " eq? " << (these_stats.st_atimespec.tv_nsec     == last_stats.st_atimespec.tv_nsec ? " true" : "false")     << " @ " << these_stats.st_atimespec.tv_nsec     << " (<-these-last->) " << last_stats.st_atimespec.tv_nsec
    << "\n mtimespec.tv_nsec:     gt? " << (these_stats.st_mtimespec.tv_nsec     > last_stats.st_mtimespec.tv_nsec ? " true" : "false")     << " lt? " << (these_stats.st_mtimespec.tv_nsec     < last_stats.st_mtimespec.tv_nsec ? " true" : "false")     << " eq? " << (these_stats.st_mtimespec.tv_nsec     == last_stats.st_mtimespec.tv_nsec ? " true" : "false")     << " @ " << these_stats.st_mtimespec.tv_nsec     << " (<-these-last->) " << last_stats.st_mtimespec.tv_nsec
    << "\n ctimespec.tv_nsec:     gt? " << (these_stats.st_ctimespec.tv_nsec     > last_stats.st_ctimespec.tv_nsec ? " true" : "false")     << " lt? " << (these_stats.st_ctimespec.tv_nsec     < last_stats.st_ctimespec.tv_nsec ? " true" : "false")     << " eq? " << (these_stats.st_ctimespec.tv_nsec     == last_stats.st_ctimespec.tv_nsec ? " true" : "false")     << " @ " << these_stats.st_ctimespec.tv_nsec     << " (<-these-last->) " << last_stats.st_ctimespec.tv_nsec
    << "\n birthtimespec.tv_nsec: gt? " << (these_stats.st_birthtimespec.tv_nsec > last_stats.st_birthtimespec.tv_nsec ? " true" : "false") << " lt? " << (these_stats.st_birthtimespec.tv_nsec < last_stats.st_birthtimespec.tv_nsec ? " true" : "false") << " eq? " << (these_stats.st_birthtimespec.tv_nsec == last_stats.st_birthtimespec.tv_nsec ? " true" : "false") << " @ " << these_stats.st_birthtimespec.tv_nsec << " (<-these-last->) " << last_stats.st_birthtimespec.tv_nsec
    << std::endl;
}
```

## Useful flag debugging

```
  std::cout
      << "\nLast Flag eq This Flag (part) ? " << (
          ((flag & kFSEventStreamEventFlagItemCreated) &&       (last_flags & kFSEventStreamEventFlagItemCreated))  ? "created "
        : ((flag & kFSEventStreamEventFlagItemRemoved) &&       (last_flags & kFSEventStreamEventFlagItemRemoved))  ? "removed "
        : ((flag & kFSEventStreamEventFlagItemModified) &&      (last_flags & kFSEventStreamEventFlagItemModified)) ? "modified "
        : ((flag & kFSEventStreamEventFlagItemRenamed) &&       (last_flags & kFSEventStreamEventFlagItemRenamed))  ? "renamed "
        : ((flag & kFSEventStreamEventFlagItemInodeMetaMod) &&  (last_flags & kFSEventStreamEventFlagItemInodeMetaMod)) ? "inode_meta_mod "
        : ((flag & kFSEventStreamEventFlagItemFinderInfoMod) && (last_flags & kFSEventStreamEventFlagItemFinderInfoMod)) ? "finder_info_mod "
        : ((flag & kFSEventStreamEventFlagItemChangeOwner) &&   (last_flags & kFSEventStreamEventFlagItemChangeOwner)) ? "change_owner "
        : ((flag & kFSEventStreamEventFlagItemXattrMod) &&      (last_flags & kFSEventStreamEventFlagItemXattrMod)) ? "xattr_mod "
        : "no parts eq "
      )
      << std::endl;
```

## WIP On a Rename Pair more robust than checking for missing paths

```
struct rename_pair_pattern {
  std::hash<char const*> hash{};
  size_t hash0{};
  size_t hash1{};
  char from_path[PATH_MAX]{};
  char to_path[PATH_MAX]{};
  struct stat these_stats{};
  struct stat last_stats{};
  enum class result {
    none,
    from,
    to,
  } result{result::none};
  inline auto pathcp(char* dst, char const* const src, size_t nbytes) -> char const* {
    assert(nbytes <= PATH_MAX);
    assert(nbytes > 0);
    assert(dst != nullptr);
    assert(src != nullptr);
    assert(dst != src);
    assert(dst + nbytes < src || dst > src + nbytes);
    dst[nbytes] = 0;
    return static_cast<char const*>(memcpy(dst, src, nbytes));
  }
  inline auto reset_state() -> void {
      hash0 = hash1
    = from_path[0] = to_path[0]
    = 0;
    result = result::none;
  }
  // The "rename pair" is complete when two
  // consecutive calls to `rollover` satisfy:
  // - (On the first call)
  //   state::none -> either state::from or state::none
  //   state::from if:
  //     - The first hash we received was unique.
  //     - We had no state when we were called.
  //   state::none otherwise.
  //   Mutations:
  //     - On state::from:
  //       - Slides hashes
  //       - Stores the given path as the "to" path
  //       - Stores the given path as the "from" path
  //       - Stores the resulting state
  //     - On state::none:
  //       - Resets all state
  // - (On the second call)
  //   state::from -> either state::to or state::none
  //   state::to if:
  //     - The second hash we received was *not* the same as the first,
  //     - We had state::from when we were called.
  //   Mutations:
  //     - On state::from:
  //       - Slides hashes
  //       - Stores the given path as the "to" path
  //       - Stores the resulting state
  //     - On state::none:
  //       - Resets all state
  // The third time we call `rollover`, we reset the state.
  // IOW, The third call is the same as the first.
  //
  // "Slides" each (hashed) path down one slot.
  // We keep a rolling window of 2 (hashed) paths.
  // We copy the path before hashing to check some
  // pre-and-post conditions, such as size, pointer
  // validity and alignment.
  inline auto rollover(char const* const path, size_t path_len, unsigned flags) -> enum result {
    if (! path || ! (flags & kFSEventStreamEventFlagItemRenamed)) {
      std::cout << "Not a rename event." << std::endl;
      reset_state();
      return result;
    }
    if (result == result::to) {
      reset_state();
    }
    hash0 = hash1;
    hash1 = hash(pathcp(to_path, path, path_len));
    if (result == result::none) {
      pathcp(from_path, path, path_len);
      return result = result::from;
    }
    if (result == result::from
     && hash0 == hash1
     && hash0 > 0) {
      return result = result::to;
    }
    std::cout << "Not a rename pair." << std::endl;
    //reset_state();
    return result::none;//result;
  }

  inline auto rollover_dbg(char const* path, size_t path_len, unsigned flags) -> enum result {
    auto r = rollover(path, path_len, flags);

    std::cout
      << "[dbg/rollover]"
      << "\nhash0: " << hash0
      << "\nhash1: " << hash1
      << "\npath: " << path
      << "\nflags: " << flags
      << "\nis rename ? " << (flags & kFSEventStreamEventFlagItemRenamed ? "true" : "false")
      << std::endl;

    dbg_flags(flags);

    if (r == result::from) {
      std::cout << "[dbg/rollover/from] -> partial rollover from " << from_path << std::endl;
    }
    if (r == result::to) {
      std::cout << "[dbg/rollover/to] => rollover from " << from_path << " to " << to_path << std::endl;
    }
    return r;
  }
};

```

## Lifetime Management and Asio

```
/*  Keeping this here, away from the `while (living())
    ...` loop, because I'm thinking about moving all the
    lifetime management up a layer or two. Maybe the
    user-facing `watch` class can take over the sleep timer,
    threading, and closing the system's resources. Maybe we
    don't even need an adapter layer... Just a way to expose
    a concistent and non-blocking kernel API.

    The sleep timer and threading are probably unnecessary,
    anyway. Maybe there's some stop token or something more
    asio-like that we can use instead of the
    sleep/`living()` loop. Instead of threading, we should
    just become part of an `io_context` and let `asio` handle
    the runtime.

    I'm also thinking of ways use `asio` in this project.
    The `awaitable` coroutines look like they might fit.
    Might need to rip out the `callback` param. This is a
    relatively small project, so there isn't *too* much work
    to do. (Last words?) */
/*
inline auto open_watch(
  std::filesystem::path const& path,
  ::wtr::watcher::event::callback const& callback) noexcept
{
  return [sysres = open_event_stream(path, callback)]() noexcept -> bool
  { return close_event_stream(std::move(sysres)); };
}
*/
```

## Event Stream Context

To set up a context with some parameters, something like this, from the
`fswatch` project repo, could be used:

  ```cpp
  std::unique_ptr<FSEventStreamContext> context(
      new FSEventStreamContext());
  context->version         = 0;
  context->info            = nullptr;
  context->retain          = nullptr;
  context->release         = nullptr;
  context->copyDescription = nullptr;
  ```

## Inode and Time

To grab the inode and time information about an event, something like this, also
from `fswatch`, could be used:

  ```cpp
  time_t curr_time;
  time(&curr_time);
  auto cf_inode = static_cast<CFNumberRef>(CFDictionaryGetValue(
      _path_info_dict, kFSEventStreamEventExtendedFileIDKey));
  unsigned long inode;
  CFNumberGetValue(cf_inode, kCFNumberLongType, &inode);
  std::cout << "_path_cfstring "
            << std::string(CFStringGetCStringPtr(_path_cfstring,
            kCFStringEncodingUTF8))
            << " (time/inode " << curr_time << "/" << inode << ")"
            << std::endl;
  ```

## Parsing Flags as Pairs

```cpp
/* @brief
   Basic information about what happened to some path.
   this group is the important one.
   See note [Extra Event Flags] */

// clang-format off
inline constexpr std::array<flag_effect_pair_type, flag_effect_pair_count>
    flag_effect_pair{
      flag_effect_pair_type(kFSEventStreamEventFlagItemCreated,        event::effect_type::create),
      flag_effect_pair_type(kFSEventStreamEventFlagItemModified,       event::effect_type::modify),
      flag_effect_pair_type(kFSEventStreamEventFlagItemRemoved,        event::effect_type::destroy),
      flag_effect_pair_type(kFSEventStreamEventFlagItemRenamed,        event::effect_type::rename),
    };
inline constexpr std::array<flag_path_type_map_type, flag_to_path_type_count>
    flag_path_type_map{
      flag_path_type_map_type(kFSEventStreamEventFlagItemIsDir,          event::path_type::dir),
      flag_path_type_map_type(kFSEventStreamEventFlagItemIsFile,         event::path_type::file),
      flag_path_type_map_type(kFSEventStreamEventFlagItemIsSymlink,      event::path_type::sym_link),
      flag_path_type_map_type(kFSEventStreamEventFlagItemIsHardlink,     event::path_type::hard_link),
      flag_path_type_map_type(kFSEventStreamEventFlagItemIsLastHardlink, event::path_type::hard_link),
    };
// clang-format on
```

## Extra Event Flags

```cpp
    // path information, i.e. whether the path is a file, directory, etc.
    // we can get this info much more easily later on in `wtr/watcher/event`.

    flag_effect_map(kFSEventStreamEventFlagItemIsDir,          event::effect_type::dir),
    flag_effect_map(kFSEventStreamEventFlagItemIsFile,         event::effect_type::file),
    flag_effect_map(kFSEventStreamEventFlagItemIsSymlink,      event::effect_type::sym_link),
    flag_effect_map(kFSEventStreamEventFlagItemIsHardlink,     event::effect_type::hard_link),
    flag_effect_map(kFSEventStreamEventFlagItemIsLastHardlink, event::effect_type::hard_link),

    // path attribute events, such as the owner and some xattr data.
    // will be worthwhile soon to implement these.
    // @todo this.
    flag_effect_map(kFSEventStreamEventFlagItemXattrMod,       event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagOwnEvent,           event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagItemFinderInfoMod,  event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagItemInodeMetaMod,   event::effect_type::other),

    // some edge-cases which may be interesting later on.
    flag_effect_map(kFSEventStreamEventFlagNone,               event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagMustScanSubDirs,    event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagUserDropped,        event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagKernelDropped,      event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagEventIdsWrapped,    event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagHistoryDone,        event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagRootChanged,        event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagMount,              event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagUnmount,            event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagItemFinderInfoMod,  event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagItemIsLastHardlink, event::effect_type::other),
    flag_effect_map(kFSEventStreamEventFlagItemCloned,         event::effect_type::other),

    // for debugging, it may be useful to print everything.
    if (flag_recv & kFSEventStreamEventFlagMount)
      std::cout << "mount" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagUnmount)
      std::cout << "unmount" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagNone)
      std::cout << "none" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemCreated)
      std::cout << "created" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemModified)
      std::cout << "modified" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemRemoved)
      std::cout << "removed" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemRenamed)
      std::cout << "renamed" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemCloned)
      std::cout << "cloned" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagHistoryDone)
      std::cout << "history done" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagKernelDropped)
      std::cout << "kernel dropped" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagUserDropped)
      std::cout << "user dropped" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagOwnEvent)
      std::cout << "own event" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagRootChanged)
      std::cout << "root changed" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagEventIdsWrapped)
      std::cout << "event ids wrapped" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemChangeOwner)
      std::cout << "change owner" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemXattrMod)
      std::cout << "xattr mod" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemFinderInfoMod)
      std::cout << "finder info mod" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagItemInodeMetaMod)
      std::cout << "inode meta mod" << std::endl;
    if (flag_recv & kFSEventStreamEventFlagMustScanSubDirs)
      std::cout << "must scan sub dirs" << std::endl;
```
