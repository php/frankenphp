# Watcher

[![Conan Center](https://img.shields.io/conan/v/watcher)](https://conan.io/center/recipes/watcher)
[![Rust/Cargo Crate](https://img.shields.io/crates/v/wtr-watcher.svg)](https://crates.io/crates/wtr-watcher)
[![PyPI/Pip Package](https://badge.fury.io/py/wtr-watcher.svg)](https://badge.fury.io/py/wtr-watcher)
[![Go Module](https://pkg.go.dev/badge/github.com/e-dant/watcher/watcher-go.svg)](https://pkg.go.dev/github.com/e-dant/watcher/watcher-go)
[![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/wtr-watcher.svg)](https://repology.org/project/wtr-watcher/versions)
[![nixpkgs stable 25.05 package](https://repology.org/badge/version-for-repo/nix_stable_25_05/wtr-watcher.svg)](https://repology.org/project/wtr-watcher/versions)
[![nixpkgs unstable package](https://repology.org/badge/version-for-repo/nix_unstable/wtr-watcher.svg)](https://repology.org/project/wtr-watcher/versions)
[![GNU Guix package](https://repology.org/badge/version-for-repo/gnuguix/wtr-watcher.svg)](https://repology.org/project/wtr-watcher/versions)
[![Builds and Publishing for Distribution](https://github.com/e-dant/watcher/actions/workflows/dist.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/dist.yml)
[![CodeQL Tests](https://github.com/e-dant/watcher/actions/workflows/codeql.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/codeql.yml)
[![Linux Tests](https://github.com/e-dant/watcher/actions/workflows/ubuntu.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/ubuntu.yml)
[![macOS Tests](https://github.com/e-dant/watcher/actions/workflows/macos.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/macos.yml)
[![Android Compilation Tests](https://github.com/e-dant/watcher/actions/workflows/android.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/android.yml)
[![Windows Tests](https://github.com/e-dant/watcher/actions/workflows/windows.yml/badge.svg)](https://github.com/e-dant/watcher/actions/workflows/windows.yml)

## Quick Start

<details>
<summary>C++</summary>

```cpp
#include "wtr/watcher.hpp"
#include <iostream>
#include <string>

using namespace std;
using namespace wtr;

// The event type, and every field within it, has
// string conversions and stream operators. All
// kinds of strings -- Narrow, wide and weird ones.
// If we don't want particular formatting, we can
// json-serialize and show the event like this:
//   some_stream << event
// Here, we'll apply our own formatting.
auto show(event e) {
  cout << to<string>(e.effect_type) + ' '
        + to<string>(e.path_type)   + ' '
        + to<string>(e.path_name)
        + (e.associated ? " -> " + to<string>(e.associated->path_name) : "")
       << endl;
}

auto main() -> int {
  // Watch the current directory asynchronously,
  // calling the provided function on each event.
  auto watcher = watch(".", show);

  // Do some work. (We'll just wait for a newline.)
  getchar();

  // The watcher would close itself around here,
  // though we can check and close it ourselves.
  return watcher.close() ? 0 : 1;
}
```

```sh
# Sigh
PLATFORM_EXTRAS=$(test "$(uname)" = Darwin && echo '-isysroot /Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk -framework CoreFoundation -framework CoreServices')
# Build
eval c++ -std=c++17 -Iinclude src/wtr/tiny_watcher/main.cpp -o watcher $PLATFORM_EXTRAS
# Run
./watcher
```
</details>

<details>
<summary>C</summary>

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
    void* watcher = wtr_watcher_open(".", callback, NULL);
    getchar();
    return ! wtr_watcher_close(watcher);
}
```
</details>

<details>
<summary>Python</summary>

```sh
pip install wtr-watcher
```

```python
from watcher import Watch

with Watch(".", print):
    input()
```
</details>

<details>
<summary>Rust</summary>

```sh
cargo add wtr-watcher tokio futures
```

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
</details>

<details>
<summary>Node.js</summary>

```javascript
import * as watcher from 'watcher';

var w = watcher.watch('.', (event) => {
  console.log(event);
});

process.stdin.on('data', () => {
  w.close();
  process.exit();
});
```
</details>

<details>
<summary>Go</summary>

```go
package main

import (
	"fmt"
	"log/slog"

	"github.com/e-dant/watcher/watcher-go"
)

func main() {
	w := watcher.NewWatcher("/path/to/dir", func(e *watcher.Event) {
		slog.Info("filesystem event", "event", e)
	})
	defer w.Close()

	// Wait for a new line to exit
	_, _ = fmt.Scanln()
}
```
</details>

The output of each above will be something like this, depending on the example:

```
modify file /home/e-dant/dev/watcher/.git/refs/heads/next.lock
rename file /home/e-dant/dev/watcher/.git/refs/heads/next.lock -> /home/e-dant/dev/watcher/.git/refs/heads/next
create file /home/e-dant/dev/watcher/.git/HEAD.lock
```

Enjoy!

---

## Tell Me More

A filesystem event watcher which is

1. Friendly
> I try to keep the [1579](https://github.com/e-dant/watcher/blob/release/tool/sl)
lines that make up the runtime of *Watcher* [relatively simple](https://github.com/e-dant/watcher/blob/release/include/wtr/watcher.hpp)
and the API practical:
```cpp
auto w = watch(path, [](event ev) { cout << ev; });
```
```sh
wtr.watcher ~
```

2. Modular
> *Watcher* may be used as **a library, a program, or both**.
If you aren't looking to create something with the library, no worries.
Just use ours and you've got yourself a filesystem watcher which prints
filesystem events as JSON. Neat. Here's how:
```bash
# The main branch is the (latest) release branch.
git clone https://github.com/e-dant/watcher.git && cd watcher
# Via Nix
nix run | grep -oE 'cmake-is-tough'
# With the build script
tool/build --no-build-test --no-run && cd out/this/Release # Build the release version for the host platform.
./wtr.watcher | grep -oE 'needle-in-a-haystack/.+"' # Use it, pipe it, whatever. (This is an .exe on Windows.)
```

3. Efficient
> You can watch an *entire filesystem* with this project.
In [almost all cases](https://github.com/e-dant/watcher/tree/release#exceptions-to-efficient-scanning),
we use a near-zero amount of resources and make
[efficient use of the cache](https://github.com/e-dant/watcher/tree/release#cache-efficiency).
We regularly test that the overhead of detecting and sending an event to the user is
an order of magnitude less than the filesystem operations being measured.

4. Well Tested
> We run this project through
[unit tests against all available sanitiziers](https://github.com/e-dant/watcher/actions).
This code tries hard to be thread, memory, bounds, type and resource-safe. What we lack
from the language, we try to make up for with testing. For some practical definition of safety,
this project probably fits.

5. Dependency Minimal
> *Watcher* depends on the C++ Standard Library. For efficiency,
we [leverage the OS](https://github.com/e-dant/watcher/tree/release#os-apis-used)
when possible on Linux, Darwin and Windows. For testing and
debugging, we use [Snitch](https://github.com/cschreib/snitch) and
[Sanitizers](https://clang.llvm.org/docs/index.html).

6. Portable
> *Watcher* is runnable almost anywhere. The only requirement
is a filesystem.

---

## Usage

### Project Content

The important pieces are the (header-only) library and the (optional) CLI program.

- C++ Header-Only Library: `include/wtr/watcher.hpp`.
  Include this to use *Watcher* in your C++ project. Copying this into your project, and
  including it as `#include "wtr/watcher.hpp"` (or similar) is sufficient to get up and
  running in this language. Some extra documentation and high-level library internals can
  be found in the [event](https://github.com/e-dant/watcher/blob/release/devel/include/wtr/watcher-/event.hpp)
  and [watch](https://github.com/e-dant/watcher/blob/release/devel/include/wtr/watcher-/watch.hpp) headers.
- C Library: `watcher-c`.
  Build this to use *Watcher* from C or through an FFI in other languages.
- Full CLI Program: `src/wtr/watcher/main.cpp`.
  Build this to use *Watcher* from the command line. The output is an exhaustive JSON stream.
- Minimal CLI Program: `src/wtr/tiny_watcher/main.cpp`.
  A very minimal, more human-readable, CLI program. The source for this is almost identical
  to the example usage for C++.

A directory tree is [in the notes below](https://github.com/e-dant/watcher/tree/release#namespaces-and-the-directory-tree).

### The Library

The two fundamental building blocks here are:
  - The `watch` function or class (depending on the language)
  - The `event` object (or similarly named, again depending on the language)

`watch` takes a path, which is a string-like thing, and a
callback, with is a function-like thing. For example, passing
`watch` a character array and a closure would work well in C++.

Examples for a variety of languages can be found in the [Quick Start](https://github.com/e-dant/watcher/tree/release#quick-start).
The API is relatively consistent across languages.

The watcher will happily continue watching until you stop
it or it hits an unrecoverable error.

The `event` object is used to pass information about
filesystem events to the callback given (by you) to `watch`.

The `event` object will contain:
  - `path_name`, which is an absolute path to the event.
  - `path_type`, the type of path. One of:
    - `dir`
    - `file`
    - `hard_link`
    - `sym_link`
    - `watcher`
    - `other`
  - `effect_type`, "what happened". One of:
    - `rename`
    - `modify`
    - `create`
    - `destroy`
    - `owner`
    - `other`
  - `effect_time`, the time of the event in nanoseconds since epoch.
  - `associated` (an event, C++) or `associated_path_name` (all other implementations, a single path name):
    - For the C++ implementation, this is a recursive structure. Another event, associated with "this" one, is stored here.
      The only events stored here, currently, are the renamed-to part of rename events.
    - For all other implementations, this field represents the path name of an associated event.
    - The implementation in C++, a recursive structure, was chosen to future-proof the library in the event that
      we need to support other associated events.

(Note that, for JavaScript, we use the camel-case, to be consistent with that language's ecosystem.)

#### State Changes and Special Events

The `watcher` type is special.

Events with this type will include messages from
the watcher. You may recieve error messages or
important status updates.

This format was chosen to support asynchronous messages
from the watcher in a generic, portable format.

Two of the most important "watcher" events are the
initial "live" event and the final "die" event.

The message appears prepended to the watched base path.

For example, after opening a watcher at `/a/path`, you may receive these
messages from the watcher:
- `s/self/live@/a/path`
- `e/self/die@/a/path`

The messages always begin with either an `s`, indicating a
successful operation, a `w`, indicating a non-fatal warning,
or an `e`, indicating a fatal error.

Importantly, closing the watcher will always produce an error if
- The `self/live` message has not yet been sent; or, in other words,
  if the watcher has not fully started. In this case, the watcher
  will immediately close after fully opening and report an error
  in all calls to close.
- Any repeated calls to close the watcher. In other words, it is
  considered an error to close a watcher which has already been
  closed, or which does not exist. For the C API, this is also true
  for passing a null object to close.

The last event will always be a `destroy` event from the watcher.
You can parse it like this, for some event `ev`:

```cpp
ev.path_type == path_type::watcher && ev.effect_type == effect_type::destroy;
```

Happy hacking.

### Your Project

This project tries to make it easy for you to work with
filesystem events. I think good tools are easy to use. If
this project is not ergonomic, file an issue.

Here is a snapshot of the output taken while preparing this
commit, right before writing this paragraph.

```json
{
  "1666393024210001000": {
    "path_name": "/home/edant/dev/watcher/.git/logs/HEAD",
    "effect_type": "modify",
    "path_type": "file"
  },
  "1666393024210026000": {
    "path_name": "/home/edant/dev/watcher/.git/logs/refs/heads/next",
    "effect_type": "modify",
    "path_type": "file"
  },
  "1666393024210032000": {
    "path_name": "/home/edant/dev/watcher/.git/refs/heads/next.lock",
    "effect_type": "create",
    "path_type": "other"
  }
}
```

Which is pretty cool.

A capable program is [here](https://github.com/e-dant/watcher/blob/release/src/wtr/watcher/main.cpp).

## Consume

This project is accessible through:
- Conan: Includes the C++ header/library
- Nix: Provides isolation, determinism, includes header, cli, test and benchmark targets
- Bazel: Provides isolation, includes the C++ header/library and cli targets
- `tool/build`: Includes the C++ header/library, cli, test and benchmark targets
- `tool/cross`: Includes the `watcher-c` shared library and header, cross-compiled for many platforms
- CMake: Includes the single-header C++ library, the `watcher-c` library (static and shared), cli, test and benchmark targets
- Just copying the header file

<details>
<summary>Conan</summary>

See the [package here](https://conan.io/center/recipes/watcher).
</details>

<details>
<summary>Nix</summary>

```sh
nix build # To just build
nix run # Build the default target, then run without arguments
nix run . -- / | jq # Build and run, watch the root directory, pipe it to jq
nix develop # Enter an isolated development shell with everything needed to explore this project
```
</details>

<details>
<summary>Bazel</summary>

```sh
bazel build cli # Build, but don't run, the cli
bazel build hdr # Ditto, for the single-header
bazel run cli # Run the cli program without arguments
```
</details>

<details>
<summary>`tool/build`</summary>

```sh
tool/build
cd out/this/Release

# watches the current directory forever
./wtr.watcher
# watches some path for 10 seconds
./wtr.watcher 'your/favorite/path' -s 10
```

This will take care of some platform-specifics, building the
release, debug, and sanitizer variants, and running some tests.
</details>

<details>
<summary>CMake</summary>

```sh
cmake -S . -B out
cmake --build out --config Release
cd out

# watches the current directory forever
./wtr.watcher
# watches some path for 10 seconds
./wtr.watcher 'your/favorite/path' -s 10
```
</details>

## Bugs & Limitations

<details>
<summary>"Access" events are ignored</summary>

Watchers on all platforms intentionally ignore
modification events which only change the access
time on a file or directory.

The utility of those events was questionable.

It seemed more harmful than good. Other watchers,
like Microsoft's C# watcher, ignore them by default.
Some user applications rely on modification events
to know when themselves to reload a file.

Better, more complete solutions exist, and these
defaults might again change.

Providing a way to ignore events from a process-id,
a shorthand from "this" process, and a way to specify
which kinds of event sources we are interested in
are good candidates for more complete solutions.
</details>

<details>
<summary>Safety and C++</summary>

I was comfortable with C++ when I first wrote
this. I later rewrote this project in Rust as
an experiment. There are benefits and drawbacks
to Rust. Some things were a bit safer to express,
other things were definitely not. The necessity
of doing pointer math on some variably-sized
opaque types from the kernel, for example, is not
safer to express in Rust. Other things are safer,
but this project doesn't benefit much from them.

Rust really shines in usability and expression.
That might be enough of a reason to use it.
Among other things, we could work with async
traits and algebraic types for great good.

I'm not sure if there is a language that can
"just" make the majority of the code in this
project safe by definition.

The guts of this project, the adapters, talk
to the kernel. They are bound to use unsafe,
ill-typed, caveat-rich system-level interfaces.

The public API is just around 100 lines, is
well-typed, well-tested, and human-verifiable.
Not much happens there.

Creating an FFI by exposing the adapters with
a C ABI might be worthwhile. Most languages
should be able to hook into that.

The safety of the platform adapters necessarily
depends on each platform's documentation for
their interfaces. Like with all system-level
interfaces, as long as we ensure the correct
pre-and-post-conditions, and those conditions
are well-defined, we should be fine.
</details>

<details>
<summary>Platform-specific adapter selection</summary>

Among the platform-specific [implementations](https://github.com/e-dant/watcher/tree/release/devel/include/detail/wtr/watcher/adapter),
the `FSEvents` API is used on Darwin and the
`ReadDirectoryChanges` API is used on Windows.
There is some extra work we do to select the best
adapter on Linux. The `fanotify` adapter is used
when the kernel version is greater than 5.9, the
containing process has root priveleges, and the
necessary system calls are otherwise allowed.
The system calls associated with `fanotify` may
be disallowed when inside a container or cgroup,
despite the necessary priviledges and kernel
version. The `inotify` adapter is used otherwise.
You can find the selection code for Linux [here](https://github.com/e-dant/watcher/blob/next/devel/include/detail/wtr/watcher/adapter/linux/watch.hpp).

The namespaces for our [adapters](https://github.com/e-dant/watcher/tree/release/devel/include/detail/wtr/watcher/adapter)
are inline. When the (internal) `detail::...::watch()`
function is [invoked](https://github.com/e-dant/watcher/blob/release/devel/include/wtr/watcher-/watch.hpp#L65),
it resolves to one (and only one) platform-specifc
implementation of the `watch()` function. One symbol,
many platforms, where the platforms are inline
namespaces.
</details>

<details>
<summary>Exceptions to Efficient Scanning</summary>

Efficiency takes a hit when we bring out the `warthog`,
our platform-independent adapter. This adapter is used
on platforms that lack better alternatives, such as (not
Darwin) BSD and Solaris (because `warthog` beats `kqueue`).

*Watcher* is still relatively efficient when it has no
alternative better than `warthog`. As a thumb-rule,
scanning more than one-hundred-thousand paths with `warthog`
might stutter.

I'll keep my eyes open for better kernel APIs on BSD.
</details>

<details>
<summary>Ready State</summary>

There is no reliable way to communicate when a
watcher is ready to send events to the callback.

For a few thousand paths, this may take a few
milliseconds. For tens-of-thousands of paths,
consider waiting a few seconds.
</details>

<details>
<summary>Unsupported events</summary>

None of the platform-specific implementations provide
information on what attributes were changed from.
This makes supporting those events dependant on storing
this information ourselves. Storing maps of paths to
`stat` structures, diffing them on attribute changes,
is a non-insignificant memory commitment.

The owner and attribute events are unsupported because
I'm not sure how to support those events efficienty.
</details>

<details>
<summary>Unsupported filesystems</summary>

Special filesystems, including `/proc` and `/sys`,
cannot be watched with `inotify`, `fanotify` or the
`warthog`. Future work may involve dispatching ebpf
programs for the kernel to use. This would allow us
to monitor for `modify` events on some of those
special filesystem.
</details>

<details>
<summary>Resource limitations</summary>

The number of watched files is limited when `inotify`
is used.
</details>

## Relevant OS APIs Used

<details>
<summary>Linux</summary>
- `inotify`
- `fanotify`
- `epoll`
- `eventfd`
</details>
<details>
<summary>Darwin</summary>
- `FSEvents`
- `dispatch`
</details>
<details>
<summary>Windows</summary>
- `ReadDirectoryChangesW`
- `IoCompletionPort`
</details>

## Minimum C++ Version

For the header-only library and the tiny-watcher,
C++17 and up should be fine.

We might use C++20 coroutines someday.

## Cache Efficiency

```sh
$ tool/gen-event/dir &
$ tool/gen-event/file &
$ valgrind --tool=cachegrind wtr.watcher ~ -s 30
```

```txt
I   refs:      797,368,564
I1  misses:          6,807
LLi misses:          2,799
I1  miss rate:        0.00%
LLi miss rate:        0.00%

D   refs:      338,544,669  (224,680,988 rd   + 113,863,681 wr)
D1  misses:         35,331  (     24,823 rd   +      10,508 wr)
LLd misses:         11,884  (      8,121 rd   +       3,763 wr)
D1  miss rate:         0.0% (        0.0%     +         0.0%  )

LLd miss rate:         0.0% (        0.0%     +         0.0%  )
LL refs:            42,138  (     31,630 rd   +      10,508 wr)
LL misses:          14,683  (     10,920 rd   +       3,763 wr)
LL miss rate:          0.0% (        0.0%     +         0.0%  )
```

## Namespaces and the Directory Tree

Namespaces and symbols closely follow the directories in the `devel/include` folder.
Inline namespaces are in directories with the `-` affix.

For example, `wtr::watch` is inside the file `devel/include/wtr/watcher-/watch.hpp`.
The namespace `watcher` in `wtr::watcher::watch` is anonymous by this convention.

More in depth: the function `::detail::wtr::watcher::adapter::watch()` is defined inside
one (and only one!) of the files `devel/include/detail/wtr/watcher/adapter/*/watch.hpp`,
where `*` is decided at compile-time (depending on the host's operating system).

All of the headers in `devel/include` are amalgamated into `include/wtr/watcher.hpp`
and an include guard is added to the top. The include guard doesn't change with the
release version. In the future, it might.


```
watcher
├── src
│  └── wtr
│     ├── watcher
│     │  └── main.cpp
│     └── tiny_watcher
│        └── main.cpp
├── out
├── include
│  └── wtr
│     └── watcher.hpp
└── devel
   ├── src
   │  └── wtr
   └── include
      ├── wtr
      │  ├── watcher.hpp
      │  └── watcher-
      │     ├── watch.hpp
      │     └── event.hpp
      └── detail
         └── wtr
            └── watcher
               ├── semabin.hpp
               └── adapter
                  ├── windows
                  │  └── watch.hpp
                  ├── warthog
                  │  └── watch.hpp
                  ├── linux
                  │  ├── watch.hpp
                  │  ├── sysres.hpp
                  │  ├── inotify
                  │  │  └── watch.hpp
                  │  └── fanotify
                  │     └── watch.hpp
                  └── darwin
                     └── watch.hpp
```

> You can run [`tool/tree`](https://github.com/e-dant/watcher/blob/release/tool/tree) to view this tree locally.

<details>
<summary>Comparison with Similar Projects</summary>

```yml
https://github.com/notify-rs/notify:
  lines of code: 2799
  lines of tests: 475
  lines of docs: 1071
  implementation languages: rust
  interface languages: rust
  supported platforms: linux, windows, darwin, bsd
  kernel apis: inotify, readdirectorychanges, fsevents, kqueue
  non-blocking: yes
  dependencies: none
  tests: yes
  static analysis: yes (borrow checked, memory and concurrency safe language)

https://github.com/e-dant/watcher:
  lines of code: 1579
  lines of tests: 881
  lines of docs: 1977
  implementation languages: cpp
  interface languages: cpp, shells
  supported platforms: linux, darwin, windows, bsd
  kernel apis: inotify, fanotify, fsevents, readdirectorychanges
  non-blocking: yes
  dependencies: none
  tests: yes
  static analysis: yes

https://github.com/facebook/watchman.git:
  lines of code: 37435
  lines of tests: unknown
  lines of docs: unknown
  implementation languages: cpp, c
  interface languages: cpp, js, java, python, ruby, rust, shells
  supported platforms: linux, darwin, windows, maybe bsd
  kernel apis: inotify, fsevents, readdirectorychanges
  non-blocking: yes
  dependencies: none
  tests: yes (many)
  static analysis: yes (all available)

https://github.com/p-ranav/fswatch:
  lines of code: 245
  lines of tests: 19
  lines of docs: 114
  implementation languages: cpp
  interface languages: cpp, shells
  supported platforms: linux, darwin, windows, bsd
  kernel apis: inotify
  non-blocking: maybe
  dependencies: none
  tests: some
  static analysis: none

https://github.com/tywkeene/go-fsevents:
  lines of code: 413
  lines of tests: 401
  lines of docs: 384
  implementation languages: go
  interface languages: go
  supported platforms: linux
  kernel apis: inotify
  non-blocking: yes
  dependencies: yes
  tests: yes
  static analysis: none (gc language)

https://github.com/radovskyb/watcher:
  lines of code: 552
  lines of tests: 767
  lines of docs: 399
  implementation languages: go
  interface languages: go
  supported platforms: linux, darwin, windows
  kernel apis: none
  non-blocking: no
  dependencies: none
  tests: yes
  static analysis: none

https://github.com/parcel-bundler/watcher:
  lines of code: 2862
  lines of tests: 474
  lines of docs: 791
  implementation languages: cpp
  interface languages: js
  supported platforms: linux, darwin, windows, maybe bsd
  kernel apis: fsevents, inotify, readdirectorychanges
  non-blocking: yes
  dependencies: none
  tests: some (js bindings)
  static analysis: none (interpreted language)

https://github.com/atom/watcher:
  lines of code: 7789
  lines of tests: 1864
  lines of docs: 1334
  implementation languages: cpp
  interface languages: js
  supported platforms: linux, darwin, windows, maybe bsd
  kernel apis: inotify, fsevents, readdirectorychanges
  non-blocking: yes
  dependencies: none
  tests: some (js bindings)
  static analysis: none

https://github.com/paulmillr/chokidar:
  lines of code: 1544
  lines of tests: 1823
  lines of docs: 1377
  implementation languages: js
  interface languages: js
  supported platforms: linux, darwin, windows, bsd
  kernel apis: fsevents
  non-blocking: maybe
  dependencies: yes
  tests: yes (many)
  static analysis: none (interpreted language)

https://github.com/Axosoft/nsfw:
  lines of code: 2536
  lines of tests: 1085
  lines of docs: 148
  implementation languages: cpp
  interface languages: js
  supported platforms: linux, darwin, windows, maybe bsd
  kernel apis: fsevents
  non-blocking: maybe
  dependencies: yes (many)
  tests: yes (js bindings)
  static analysis: none

https://github.com/canton7/SyncTrayzor:
  lines of code: 17102
  lines of tests: 0
  lines of docs: 2303
  implementation languages: c#
  interface languages: c#
  supported platforms: windows
  kernel apis: unknown
  non-blocking: yes
  dependencies: unknown
  tests: none
  static analysis: none (managed language)

https://github.com/g0t4/Rx-FileSystemWatcher:
  lines of code: 360
  lines of tests: 0
  lines of docs: 46
  implementation languages: c#
  interface languages: c#
  supported platforms: windows
  kernel apis: unknown
  non-blocking: yes
  dependencies: unknown
  tests: yes
  static analysis: none (managed language)

```
</details>
