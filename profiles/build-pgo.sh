#!/bin/bash
set -euo pipefail

# Generate a PGO profile by hammering frankenphp with wrk in both regular and
# worker mode, then merging the two pprof samples into
# caddy/frankenphp/default.pgo. Go auto-detects ./default.pgo in the main
# package, so direct `go build` and the Dockerfile pick it up with no flag changes.
# xcaddy builds in a temp dir and needs
# `--pgo $(go mod download -json github.com/dunglas/frankenphp@latest | jq -r .Dir)/caddy/frankenphp/default.pgo`.

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
export BENCH_SEC=${BENCH_SEC:-8}
N=$(find "$HERE/app" -maxdepth 1 -name "*.php" ! -name "index.php" | wc -l)
TOTAL=$((BENCH_SEC * N))

(cd "$ROOT/caddy/frankenphp" && go build -pgo=off -o frankenphp)

collect() {
	"$HERE/benchmark.sh" "$1" >/dev/null &
	BPID=$!
	until curl -fsS localhost:22019/config/ >/dev/null 2>&1; do sleep 0.2; done
	curl -fsS --max-time $((TOTAL + 30)) "localhost:22019/debug/pprof/profile?seconds=$TOTAL" -o "$2"
	wait $BPID
}

collect "$HERE/app/Caddyfile.regular" "$HERE/regular.pgo"
collect "$HERE/app/Caddyfile.worker" "$HERE/worker.pgo"

export CGO_CFLAGS="${CGO_CFLAGS:-} -fno-sanitize=undefined"
PPROF="$(go env GOPATH)/bin/pprof"
[ -x "$PPROF" ] || go install github.com/google/pprof@latest
"$PPROF" -proto "$HERE/regular.pgo" "$HERE/worker.pgo" >"$ROOT/caddy/frankenphp/default.pgo"

(cd "$ROOT/caddy/frankenphp" && go build -o frankenphp-pgo)
