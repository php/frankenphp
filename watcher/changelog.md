# Changelog

## 0.14.5

- Fixed cross-directory path name caching and reporting in the inotify/Linux adapter (#109, thanks to @unravel-dev for reporting).
- Updated the development (test) dependencies in the watcher-nodejs project.

## 0.14.4

- Fixed rename event reporting in the inotify adapter when there is no "moved-from" half of the event provided (#105, thanks to @scoueille and @Gautrot for reporting; introduced with fixes for #89).
- Addsd a regression test for issue #105.
- Fixed a rare misreporting error in the darwin/fsevents adapter where a rename event very quickly after a creation event on the same path could cause the creation event to be duplicated alongside the rename event.
- Only add CMake install rules if we are the main project, to be good neighbors when added as a submodule (#111, thanks to @unravel-dev)
- Fix Windows distribution archive structure to place headers in wtr/ dir (#108, thanks to @dunglas)
- Bumped the minor CMake version from 3.9 to 3.11, as versions less than 3.11 are becoming deprecated.
- Updated dependencies in the watcher-nodejs project.
- Fixed Windows old/new rename event path handling
- Fixed event loop IO failure condition handling on Windows (pending IO is not an error)

## 0.14.3

- Fixed duplicate creation events being reported on Linux in the inotify adapter when the event matched both the special directory creation path and the non-pending parse result path.

## 0.14.2

- Fixed rename event reporting in the inotify adapter when the associated event is still pending (#105, thanks to @scoueille and @Gautrot for reporting; introduced with fixes for #89).
- Prevented de-duplication of creation events from interfering with other types of events in the darwin/fsevents adapter (#106, thanks to @hkhk368 for reporting).

## 0.14.1

- Fixed Watcher-Go module import (#101, @dunglas).
- Updated development dependencies in the watcher-nodejs project.

## 0.14.0

Special thanks to @dunglas for their contributions in this release.

- Added a Go package for the Watcher (#96, #100, @dunglas)
- Upgraded the Github Actions checkout action to v6 (#97, @dunglas)
- Updated OS versions for Github Actions runners (#100, @dunglas)
- Added a Docker development environment for Go development (#99, @dunglas)
- Made public functions in `watcher-c` DLL-exported on Windows (#92)
- Made our CMake project play nice when added to another project (#93)

## 0.13.8

- Updated the badges in the readme, adding Homebrew, GNU Guix and Nix Packages.
- **PR#77, @carlocab**
  fix(build): fix generation of `*.pc` files
- Newline-terminated version output from `wtr.watcher -v`.

## 0.13.7

- Added `pkg-config` files for `watcher` and `watcher-c` (#82).
- Portability fixes in the `tool/test` suite (#84).
- Updated documentation in the Darwin adapter.
- Added test targets to CMake in the idiomatic way, i.e. `enable_testing()` and `add_test()` (#84).
- Added a version (`-v, --version`) flag to the CLI program `wtr.watcher` (#86).
- Fixed C++17 compatibility issues in the portable Warthog adapter, especially for BSDs (#87, #88).
- Added a C++17 compatibility check to Ubuntu CI jobs (#87, #88).
- Fixed a bug in the inotify backend when associated (rename, move) events were not adjacent in the read(2) event buffer (#89).
- (C++) Added an (in-place) assignment operator(`=`) to the `event` structure.
- Updated CodeQL CI to v3, the latest version as of September 2025.

## 0.13.6

Special thanks to @DanielCodesphere and @jordan-woyak for their important contributions in this release.

- **PR#77, DanielCodesphere**
  fix(inotify): add `IN_CLOEXEC` and `IN_NONBLOCK` flags to inotify init.
  This stops child processes from inheriting open inotify watches
- **PR#76, jordan-woyak**
  Linux: Fix `epoll_wait` failing with `EINTR`.
  `epoll_wait` can spontaneously fail with `EINTR`.
- Fixed Issue 73 (Rename events are occasionally split into two events on linux).
- **PR#78, DanielCodesphere**
  fix(inotify): correct event type for symlinks

## 0.13.5
- C++17 compatibility fixes on Linux (avoid captured structured bindings).

## 0.13.4

- readme: Grammar fixes.
- watcher-nodejs: Moved `node-gyp` to `devDependencies` from `dependencies`, where it should be.
- watcher-nodejs: Maintenance on the Node.js tests, which now ignore the very much non-idempotent `event.effectTime` field when comparing events.
- watcher-nodejs: Updated a transitive development dependency (`cross-spawn`) to a non-vulnerable version.

## 0.13.3

On Linux:
- Fixed the behavior of reporting on very quickly added nested directories.
  Previously, we'd mostly only report on what either inotify or fanotify told us about
  events on paths which we already had marked. If a nested directory (`a/b`) was added
  before we marked `a`, then we wouldn't report on `a/b`. Now, we will. The fix was to
  walk the newly-created directory tree (in this case, walking `a`), marking and reporting
  on every child we find within it.
- For inotify, fixed the reporting of the path name on associated events. Previously,
  we did not lookup which `(watch descript, path name)` pair the event was relative to.
  The kernel documents the behavior [here](https://github.com/torvalds/linux/blob/cabb4685d57ed50cd197498d2ac946ad5b6272e7/include/uapi/linux/inotify.h#L15-L27).
  Our behavior was changed to always perform this lookup on associated events, whereas
  before the behavior was to only perform this lookup on the "base" event. Note that
  currently, and probably for all time, the only associated event is a rename event.

## 0.13.2

CMake now:
- Does not optimize away our entire library (no `-fwhole-program` when building libraries)
- Uses a linker version script on linux when building the watcher-c shared library
- Defaults to *not* build sanitizer and test targets
- Adds an RPATH to the library targets (i.e. `watcher-c`) when building for Apple targets
- Uses a more conventional `BUILD_TESTING` to control whether test targets are produced (instead of `BUILD_TEST`)

Various documentation improvements in the readme and CMake build file.

## 0.13.1

Added `watcher-c`, as a (versioned) shared library, as a static library, and with headers, to the CMake build file.

They are included in the "lib" and "include" components, respectively.

## 0.13.0

Added a Rust crate for the Watcher.

```rust
use futures::StreamExt;
use wtr_watcher::Watch;

#[tokio::main(flavor = "current_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let show = |e| async move { println!("{e:?}") };
    let events = Watch::try_new(".")?;
    events.for_each(show).await;
    Ok(())
}
```

The crate is available on [crates.io](https://crates.io/crates/wtr-watcher).

Made the Github release CI depend on Wheels being built (and from uploading stale wheels because of that).

Various documentations improvements:
- Example usage for the new Rust crate
- PyPi and Crates.io badges in the readme

Various chores around the watcher-nodejs project:
- Simplified the napi bindings
  - Left more of the type definitions to the typescript side (instead of the napi side)
  - Various C paranoia fixes, out of caution, in case napi doesn't initialize various things

Fixed (was missing) the `effect_time` field in the watcher-nodejs project's event type.

Made the Python project consistent with our other languages:
- The event object's field are in the same order as C/Rust/Js
- The output of the "example" CLI is the same JSON event format as the other languages (was previously a Python object dump)
- The effect time is represented as a number (nanosecond since epoch), not a datetime (which loses precision and is inconsistent with the other languages)
- The associated path name is an optional string, not a maybe empty string

## 0.12.2

Added CI for Github releases and Python/pip publishing.

Updated the readme to reflect the Python package name: `wtr-watcher`.

Fixed shared library loading, naming in the Python ffi.

## 0.12.1

Meson learned how to install the header files for the C library, thank you @toge (in #56)

Fixed an automatic version bump in the release script for some of our newer build files.

## 0.12.0

Various documentation improvements in the readme, especially around new features.
The readme for the C project was contributed by @AlliBalliBaba in #53

Added (heavily optimized) builds in CI for our "important" artifacts:
- The "minimal" CLI: `tw`
- The "full" CLI: `wtr.watcher`
- The new C (currently built as a shared) library: `watcher.so.<version>` and `watcher-c.h`
- Python wheels

These artifacts are available for a wide range of platforms. The CLI and C
shared library are built for these triplets:
- `aarch64-apple-darwin`
- `aarch64-pc-windows-msvc`
- `aarch64-unknown-linux-gnu`
- `aarch64-unknown-linux-musl`
- `armv7-unknown-linux-gnueabihf`
- `armv7-unknown-linux-musleabihf`
- `x86_64-pc-windows-msvc`
- `x86_64-unknown-linux-gnu`
- `x86_64-unknown-linux-musl`

The Python wheels are built for these platforms Linux and Apple on Python 3.8 and up.
On Linux, the musl and gnu libcs, and the architectures `x86_64` and `aarch64`, are supported. 
On Apple, only `aarch64` is supported (because of a bug on intel runners; see the workflow).

Added support for using this project in C, Python and Node.js:
- C support as a shared or static library (one with a C-ABI)
- Node.js support as an NPM package for a "native addon"
- Python support as a Wheel for an FFI bridge to the C shared library

Each of the new bindings have a basic test suite.

Some examples for these new languages.
Each of the examples has similar behavior:
- Watch for and display all events
- Watch for events on every filesystem under the root directory `/`
- Stop when any terminal input is received

### C

```c
#include "wtr/watcher-c.h"
#include <stdio.h>

void callback(struct wtr_watcher_event event, void* _ctx) {
    printf(
        "path name: %s, effect type: %d path type: %d, effect time: %lld, associated path name: %s\n",
        event.path_name,
        event.effect_type,
        event.path_type,
        event.effect_time,
        event.associated_path_name ? event.associated_path_name : ""
    );
}

int main() {
    void* watcher = wtr_watcher_open("/", callback, NULL);
    getchar();
    return ! wtr_watcher_close(watcher);
}
```

### Python

```python
from watcher import Watch

with Watch("/", print):
    input()
```

### Node.js/TypeScript/JavaScript

```javascript
import * as watcher from 'watcher';

var w = watcher.watch('/', (event) => {
  console.log(event);
});

process.stdin.on('data', () => {
  w.close();
  process.exit();
});
```

## 0.11.1

Documentation and formatting.

## 0.11.0

"Access" events -- which only modify the access time on a file or directory -- are no longer reported.
Refs: #43, #46 and d9d9d8b

Windows learned how to properly report rename events, thanks to @iwubcode in #45.

Various housekeeping. Added `tw` to the Nix flake, updated the flake's lockfile. Some formatting.

## 0.10.1

CI jobs will uplaod some of their artifacts. I don't recommend using them yet because we don't compile to known architectures. (We just compile to "target doubles", using whatever architecture the runner is on.)

Build artifacts are attested to with this [fancy new action](https://github.com/actions/attest).

Updated the readme and directory tree display.

Removed some unused, experimental sync routines from the Darwin implementation.

Removed the sources for a minimally reproducible Darwin issue in Dispatch. I'm not sure how to file a bug report to Apple, or if it's worth the time, and we've mostly worked around this issue.

Improved some of the internal error handling for stale files on the Linux `fanotify` adapter.

Minor maintenance on our CI. Updated some dependencies, cleaned some things up. Made the caches *actually* work.

Removed some cruft around our git tree. Some old "tellfiles" and small readmes were removed.
- The tellfiles were similar to what the Just (command runner) does. A little system I hacked up for myself in a shell script. These became unnecessary as our tools (directory) became nicer.
- The readmes come from a time when I believed that if something lacks documentation, it is unjustified complexity or bloat. I overlooked that a directory structure can just be useful for organization, so some of the readmes ended up verbose and meaningless.

Dropping CI for some platforms speeds up the pipeline a bit and reduces cost. So, CI for some of our older platforms was removed. Generally, only the two latest platform versions of some OS will be tested in CI. Namely:
- Android NDKs
- Ubuntu LTS releases
- MacOS versions
- But -- Only the most recent Windows Server. The one previous is from 2019. At the time of writing, that was released 5 years ago. (Which is why I dropped CI for it.)

## 0.10.0

Added an associated event to the fields of the event object. (The event object is recursive now.) This field stores a renamed-to effect type, associated with the rename-from “parent” event. Currently, the only possible associated event is a renamed-to event. In the future, we may store other kinds of associated events there. Ownership changes are a good candidate. This kind of structure allows more room for future changes if needed, although admittedly the intended use of associated events may be a bit less obvious to the user than I’d like. 

Added a set of shell-based test suites. These tests are focused on accuracy and portability. (They’re integration tests.)

Added a rapid open-and-close stress test. Fixed a related issue on Darwin’s watcher. 

Added support for building with Bazel. 

Fixed the link behind our Conan badge. 

Some files were moved around and simplified. Among them, the CLI was re-written to be the ~100 line program it should be. 

Exceptions and run-time type information were removed from our CLI builds on non-Windows platforms. (We never used RTTI, exceptions were used in a scarce few places. This cuts the binary down by a couple dozen kilobytes, weighing it in around 50-80kb, depending on the platform.)

Replaced a constant delay with a timeout in the "simple" test.

Updated the flake lockfile.

Synchronize access to a shared context, between us and the OS, on Darwin, because the OS lies about stopping execution, and we need to be tidy.

The `watch` constructor makes the given `path` absolute or, if
the path cannot be made absolute (likely because it doesn't exist),
then the watcher refuses to watch.

Many thanks to @toge for all of their work on Conan for us.

## 0.9.5

Fixed an error on windows which asked us to stop checking buffers for null (so we did).

Temporarily disabled the Ubuntu 20.04 runner because it cannot find the generated Makefile.

## 0.9.4

Set up an older Ubuntu runner, 20.04, alongside the latest, for CI.

Updated changelog for `0.9.3`.

## 0.9.3

Defaulted CMake use C++17 for this project.

Made `flake.nix` work more correctly and gave components CMake more granularity.

Removed stray and unused `#include <bit>`.

## 0.9.2

Removed stray Makefile.

## 0.9.1

Replaced usage of "alternative operators" -- `and`, `or`, `not` -- with their
"traditional" counterparts -- `&&`, `||`, `!`. This fixes compilation issues
on MSVC.

Android's CI pipeline uses just the latest three NDKs. We have limited cache.

Fixed automatic (semantic) version bumping in `flake.nix` from `tool/release`.

Added the `test-bin` and `san-bin` components, for our sanitized variants and
executable test targets, and a "proper" header-only library target, `wtr.hdr_watcher`,
to the CMake build file.

@toge: Add missing include for `std::atomic<T>` in the Linux/inotify adapter.

Updated documentation around the `event` object and what this library thinks of safety as.

Minor fixup on the CLI program: We use canonical, not absolute, paths. So, a path that
would have been displayed as `some/path/.` is now displayed as `some/path`. We also
`.close()` the watcher before scope exit, which fixes up json parsing in non-infinite
(and non-streaming) runs.

## 0.9.0

### Cli (0.9.0)

The watcher now runs forever when no timeout is passed in, just as it says.
(That's a bugfix.)

### API (0.9.0)

The API is now RAII-safe.
This hadn't been part of the API for a long time because I had been exploring
bindings to other languages. I had some concerns about an instance of the
watcher being destroyed across an FFI boundary. If those bindings are made, we
could write a separate C-style API for them to use. Manually closing the watcher
is still just fine, which is how it had been done before.

The `wtr::watch` API should still be intact for construction. There will be some
differences in how the objects are stored:

Before we, might spell "some watchers, in a vector" like this:

```cpp
auto lifetimes = std::vector<std::function<bool()>>{};
// Or `invoke_result_t<wtr::watch("", [](auto){});`
```

We can spell it like this now:

```cpp
auto lifetimes = std::vector<std::unique_ptr<wtr::watch>>{};
// ^ More normal
```

Destroying the watchers with the associated `.close()` function is still valid,
but is not typically necessary.

The `event` structure's field names were changed up a bit:

| old   | new         |
|-------|-------------|
| where | path_name   |
| kind  | path_type   |
| when  | effect_time |
| what  | effect_type |

### Etc (0.9.0)

This library (and all the programs in `src/*`) work with C++17 and up.

The Nix package was renamed from `wtr-watcher` to `watcher`. We can namespace
things differently in Nix if we want to.

The clang-format file avoids some oddities around namespaces and line breaks.

Fixed up the Linux adapters. There was a case, when `inotify` events were delivered
in a batch from a call to `read()`, such as on very closely timed events, where
we would skip over the rest of the buffer. There was a similar case, on the
`fanotify` adapter, where we mistakenly returned early after the first event in the
case of batched events from a call to `read()`.

Added a failsafe to the Linux adapters which prevents a hypothetically infinite loop.

For the Linux `inotify` adapter, within the event receive/parse/send loop, the calls
to `inotify_[add,rm]_watch` were moved from immediately *after* sending the event along
to a user's callback to immediately *before* that callback. Timing is important in that
loop. If we received a `create` or `destroy` event on a directory, we need to be sure
to mark it before events (including self-destruction events) happen on that directory.
The window can be small.

Removed `platform.hpp` as not needed. Platform definitions aren't complicated
enough to have a separate header for.

Removed `adapter.hpp` as not needed. The (minor) functionality it provided
was moved into `watch.hpp`.

The benchmarking programs are more clear about what they're benchmarking, and how.
They're still not *really* a benchmark suite, more of a performance regression test.

The concurrent watcher tests compare the *set* of events they receive against
the *set* of events we expect. Comparing the *order* of the events in those
tests was always a bit wrong, and the tests occasionally told us something was amiss.

Made the comment style more consistent. Removed the `@brief` comment sections
and normalized most comments to look like this:

```
/*  This is a
    multi-line
    comment. */
/*  This is a single-line comment. */
```

### Build (0.9.0)

The relative source and module paths seem to behave oddly on Windows and Nix
(flakes). The CMake files used to live around the `build/in` and
`build/in/cmake` directories. For now, it seems easiest to leave them in the
project's root directory.

If you had been building like this:

`cmake -S build/in -B build/out ...`

You should build like this now:

`cmake -S . -B build/out ...`

The sanitizer build don't live in subdirectories. They live alongside the other
binaries, affixed with the sanitizer name.

The unit test and bench targets aren't broken up by their test name. All the unit
tests have a single target. The benches have another target.

A build directory which previously looked like this:

```
build/out/this:
    nosan:
        wtr.watcher
        wtr.test_watcher.<test name>
        wtr.bench_watcher.<bench name>
        <other binaries ...>
    asan:
        wtr.watcher
        <other binaries ...>
    <other sanitizers ...>
```

Will now look like this:

```
build/out/this:
    wtr.watcher
    wtr.test_watcher
    wtr.bench_watcher
    wtr.watcher.<sanitizer name>
    wtr.test_watcher.<sanitizer name>
    wtr.bench_watcher.<sanitizer name>
```

Multi-configuration build systems will still break up all of the targets into
subdirectories by their build-configuration name. Those build systems are typical
of IDEs. The configuration names are typically `Debug`, `Release`, `MinSizeRel`
and `RelWithDebInfo`. (That behavior hasn't changed.)

Otherwise, the Nix flake was brushed up a bit to better support CMake's
`FetchContent` feature and Darwin's system libraries, and the `tool/build` script
checks for a `build` directory before (re)generating CMake files in it.

## 0.8.8

Fixed some build errors on g++ 10.2.1 around `using shorthand = big_long::enum_name;` syntax.
Made amalgamation (`tool/hone --header-content-amalgam`) much faster by skipping the formatting pass.

## 0.8.7

Removed the directory cache from the `fanotify` adapter. There were some cases where this cache produced misleading results.

## 0.8.6

Added *0.8.[5, 4]* to the changelog.

## 0.8.5

Added a blocking operation to the `tiny_watcher` example where the user's work would go. (In this case, reading something from `stdin`.)

## 0.8.4

Fixed a compilation error on MSVC. Evidently, MSVC has some difficulty knowing the difference between an `enum` and an `enum class` depending on its use, regardless of its declared type.

## 0.8.3

`std::filesystem` throws exceptions *even when we use the `std::error_code`* interfaces. Unfortunately, this library needs to build with exceptions for now as a workaround. The alternative is a crash. This was a regression and could happen when watching not-really-a-filesystem parts of the filesystem, like `/proc`, or just everything, `/`.

In the future, we could use a POSIX API to avoid `std::filesystem`'s reliance on exceptions.

## 0.8.2

Removed a stray `printf()` from the linux/inotify adapter.

## 0.8.1

Fixed a for loop with bad pointer math on the linux/inotify adapter.

## 0.8.0

The Darwin adapter is more skeptical of the kernel. We check for null on the context passed to us in `event_recv`, even though it shouldn't be possible for it to be null.

The `operator<<`s for `event` are more user-customizable. Their signatures look like this now:

```cpp
template<class Char, class CharTraits>
inline std::ostream& operator<<(std::basic_ostream<Char, CharTraits>& os, enum kind const& k) noexcept;
```

Users should be able to work off of `std::basic_ostream<Char, CharTraits>` if they want to send it to something other than the standard streams.

This is a new minor release because I'm not sure if the `basic_ostream` changes are ABI-compatible with `0.7.0`.

## 0.7.0

CLI code is a bit simpler. Usage is more helpful. Plans for filters.

The type of the `fanotify` adapter's loop variable for handle hashes is a `decltype` of the bytes. Not a char. This is correct.

This release should have been `0.6.1`.

## 0.6.0

### 0.6.0 API

`wtr::watcher::die` doesn't exist. Instead, closing the watcher is dependant on there *being* a watch: `watch()` returns a *unique* way for you to close it.

```cpp
// Object-like style

auto lifetime = wtr::watch(".", [](auto e){cout << e;});

lifetime.close();

// Function-like style

auto die = wtr::watch(".", [](auto e){cout << e;});

die();
```

`watch()` isn't an object. It's a function that returns a (structure containing a) named function, `close()`. `()` also resolve to `close()`, so it can be anonymous, too.

Instead of `#include <watcher/watcher.hpp>`, now you include `<wtr/watcher.hpp>`. This lines up with the namespace.

The namespace `wtr::watcher` is inlined, so calling `wtr::watcher::watch(...)` is the same as `wtr::watch(...)`. Inline namespaces are generally used for versioning, not simplifying. I still think it's a worthwhile change for the user.

### 0.6.0 Internal Behavior

Every watcher is unique. (Before, we tried to do fancy stuff by counting watchers on each path. I thought that would be useful someday to coalesce events. The watchers are so efficient that this doesn't seem to matter. If the OS wants to batch event reporting across "listeners", which it probably already does, then that's fine.)

## 0.5.6

### 0.5.6 Housekeeping

Everything we use from every file we include is documented.

The header slugs in (this) changelog don't conflict.

## 0.5.5

### 0.5.5 Behavior

When two or more watches are created on the same path,
they are closed in the order they are created.

### 0.5.5 Housekeeping

A few spare exceptionless functions were marked noexcept.

The delay between setting up test directories, used in the unit tests, is lower (10ms).

We need that delay because the kernel APIs, other than `inotify` and `fanotify`, don't tell us
when they're *acutally* ready to watch. And, when they are ready to watch, sometimes they (because
of batching) pick up filesystem events slightly before we *asked* them to watch.

(That's perfectly fine when the user is in control, but the unit tests currently expect events to
flow in a predetermined order.)

The `wtr::watcher::detail` namespace mirrors a real path: `include/watcher/detail`.

The regular expressions checking whether to build no targets or some targets actually work.

Shellcheck warnings were cleaned up on `build/build`.

## 0.5.4

The Linux adapter checks the kernel version (at compile-time) to select an API that actually exists.

## 0.5.3

The Linux `fanotify` adapter for linux is implemented.
It is selected if the user is (effectively) root and not on Android.
The `inotify` adapter is used in those cases.

The Darwin `FSEvents` adapter has independent resources per each (user) invocation of `watcher::watch`.

Documentation for `watcher::die` is more clear.

More than half (66%) of the codebase (3429 lines) is documentation (39%) and unit tests (27%).
The significant lines of code and documentation are current in the readme.

`tiny-main` is now `tiny_main`. We may rename this to `tiny_watcher` in the future.

The `tool/hone` amalgamation uses a valid identifier for an `ifdef` guard.

### 0.5.3 Internal

The `watcher::detail::adapter::watch_ctl` function is now `watcher::detail::adapter::adapter`.
(This is the only function in `watcher/adapter/adapter.hpp`.)

`watcher/adapter/adapter.hpp` is documented.

`watcher::detail::adapter::adapter::path_id` is a (constant) variable, not a function.

`watcher::detail::adapter::adapter::living_container` uses `std::string` as the key type, not an integral type.

The Windows adapter is given some much needed love.

Heavier reliance on using `watcher/watcher.hpp` internally.

## 0.5.2

The changelog for `0.5.0` is updated. Again.

## 0.5.1

The changelog for `0.5.0` is updated.

## 0.5.0

Gives support for several watchers. When watchers exist for the same path,
*Watcher* thinks of its resources on that path like a shared pointer.

A warning from the thread sanitizer is gone.

*Watcher* and its unit tests pass all sanitizers and unit tests without warnings.

The readme is more readable. More notes exist, and links to them are used.

A ConanCenter badge is beside the other badges in the readme.

Instead of overloads and templates for path types, `std::filesystem::path` is used.

Define a project roadmap for release `1.0.0`.

Unit tests are more robust. A "Simple" unit test is added.

## 0.4.3

Single-include amalgamation.

Consider automatic amalgamation.

## 0.4.2

Public functions defined in headers prefer `inline` over `static`.

Threads may not race when `epoll_wait` resumes on long wait periods despite `living()` being false.

No warnings in `tsan/wtr.test_watcher.test_event_targets`.

## 0.4.1

Clean misplaced file

## 0.4.0

Linux inotify adapter does not loop away from epoll wait.

The unit tests are more robust.

The namespace `water` is now `wtr`.

`wtr::watcher::event` holds a `string`, not `char*`, in `event.where`.

The `build/build` program is more intuitive. See `build/build --help`.

CI runners, especially those for macOS and Linux, have become more efficient and thorough. The use of `build/build` allows many CI runners to build and test across build types and sanitizers more easily than otherwsie. Some platform specifics for the Android and Windows are being sorted out. In the meantime, Android and Windows are using the "old" runner jobs.

## 0.3.3

The Android adapter is identical to the Linux `inotify`-based adapter except for its handling of `inotify_init` and the associated watch options.

Android was given a CI job.

## 0.3.2

Toge's portability PR was merged.

## 0.3.1

The `inotify`-based adapter for Linux is refined.

## 0.3.0

The `inotify`-based adapter for Linux is introduced. Concerns about its accuracy are carefully considered.

Niall's suggestions prompted future work being planned for improvements on the design of all adapters.

## 0.2.9

The `warthog` adapter is fixed on Windows. The Windows CI runner is nudged.

## 0.2.8

Tooling in `tool/release` is refined to match the typical commit syntax. This currently applies to the release hooks in `.version` and `build/in/CMakeLists.txt`.

## 0.2.7

`tool/release` writes to `.version` and `tool/hone` prepends `#pragma once` to the output from `--header-content-amalgam`.

The Windows CI runner is nudged.

## 0.2.6

Directory tree around `watch` is refined.

`tool/sloc` is introduced (to count the significant lines of codes).

`src/tiny-main.cpp` is introduced.

## 0.2.5

CI is implemented through GitHub Actions.

The MIT license is chosen.

## 0.2.4

A single-header artifact is created.

`tool/hone` is introduced to generate a single-header amalgamation of this project.

Documentation in the `README.md` is heavily updated.

## 0.2.3

`tool/release` is made more clear and the EBNF is made valid (a comma was missing).

## 0.2.2

`tool/release` is usable and fully-featured.

`tool/release` may show its usage as EBNF.

C++20 Concepts are removed for being very interesting but not beneficial enough to an API with a very small surface.

Comments are refactoring according to the MISRA guidelines.

## 0.2.1

`tell dbun` is refactored.

The `FSEvent` adapter is made more safe.

## 0.2.0

The first public release includes:

- The `event` object and the `watch` function
- Stream operator overloads for JSON output
- Event parsing control
- The CLI program
- Documentation throughout
