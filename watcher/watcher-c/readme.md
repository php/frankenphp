# Using the watcher library from C

The watcher can also be used from C by including the `watcher-c.h` file on compilation and in your project.

You can cross compile the library via the `watcher-c/cross-compile.sh` script or download the precompiled library from
the releases.

### Compiling the library from source

If you want to compile the library yourself you can follow the following example:

First check out a branch or release, here we'll use the 'next' branch.
You can then compile the library with meson or a tool of your choice, 
we will use `c++`.

```bash
curl -L https://github.com/e-dant/watcher/archive/refs/heads/next.tar.gz | tar xz
cd watcher-next/watcher-c
c++ -o libwatcher-c.so ./src/watcher-c.cpp -I ./include -I ../include -std=c++17 -fPIC -shared
```

This gives us a `libwatcher-c.so` library that you can now install on your system.
In most container based environments you can just copy the library like this:

```bash
cp libwatcher-c.so /usr/local/lib/libwatcher-c.so
ldconfig
```

### Using the library in your C code

After installing the library, copy the `watcher-c.h` file to your project and include it in your C code:

```c
#include "watcher-c.h"
```

You will then be able to use the functions 
[provided by the library](https://github.com/e-dant/watcher?tab=readme-ov-file#the-library).
