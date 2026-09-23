#!/usr/bin/env bash

# Go removes its temporary test binary after it exits. Read native crash dumps
# before returning to Go, while the executable and its debug symbols still exist.
set -euo pipefail
shopt -s nullglob

status=0
"$@" || status=$?

if ((status != 0)); then
	for core in frankenphp-core.*; do
		gdb --batch --quiet \
			-iex 'set auto-load off' \
			-ex 'set debuginfod enabled off' \
			-ex 'set pagination off' \
			-ex 'set print frame-arguments none' \
			-ex 'thread apply all bt' \
			"$1" "$core" || true
	done
fi

exit "$status"
