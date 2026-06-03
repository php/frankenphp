#! /usr/bin/env sh
set -e
cd /src
[ -d "$BUILD_DIR" ] || meson setup "$BUILD_DIR"
meson compile -C "$BUILD_DIR"
