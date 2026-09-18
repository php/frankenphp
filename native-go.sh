#!/bin/sh
# Set XCADDY_WHICH_GO to this script to build with the native Linux entry point.
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
if [ "${1:-}" = build ]; then
	shift
	exec "${FRANKENPHP_GO:-go}" run "$script_dir/internal/nativebuild/main.go" "$@"
fi
exec "${FRANKENPHP_GO:-go}" "$@"
