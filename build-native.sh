#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$script_dir/go.sh" run "$script_dir/internal/nativebuild/main.go" "$@"
