#!/bin/bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
export BENCH_SEC=${BENCH_SEC:-8}
N=$(find "$DIR/profiles/app" -maxdepth 1 -name "*.php" ! -name "index.php" | wc -l)
TOTAL=$((BENCH_SEC * N))

(cd "$DIR/caddy/frankenphp" && go build -pgo=off -o frankenphp)

collect() {
	"$DIR/benchmark.sh" "$1" >/dev/null &
	BPID=$!
	until curl -fsS localhost:2019/config/ >/dev/null 2>&1; do sleep 0.2; done
	curl -fsS --max-time $((TOTAL + 30)) "localhost:2019/debug/pprof/profile?seconds=$TOTAL" -o "$2"
	wait $BPID
}

collect "$DIR/profiles/app/Caddyfile.regular" "$DIR/profiles/regular.pgo"
collect "$DIR/profiles/app/Caddyfile.worker"  "$DIR/profiles/worker.pgo"

export CGO_CFLAGS="$CGO_CFLAGS -fno-sanitize=undefined"
PPROF="$(go env GOPATH)/bin/pprof"
[ -x "$PPROF" ] || go install github.com/google/pprof@latest
"$PPROF" -proto "$DIR/profiles/regular.pgo" "$DIR/profiles/worker.pgo" >"$DIR/profiles/merged.pgo"
(cd "$DIR/caddy/frankenphp" && go build -pgo="$DIR/profiles/merged.pgo" -o frankenphp-pgo)
