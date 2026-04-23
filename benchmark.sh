#!/bin/bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
APP="$DIR/profiles/app"
CADDYFILE="${1:-$APP/Caddyfile.regular}"
PER=${BENCH_SEC:-8}
FRANKENPHP_BIN="${FRANKENPHP_BIN:-$DIR/caddy/frankenphp/frankenphp${PGO:+-pgo}}"
echo "$FRANKENPHP_BIN"
[ -x "$FRANKENPHP_BIN" ] || {
	echo "FRANKENPHP_BIN not executable: $FRANKENPHP_BIN" >&2
	exit 1
}

SCRIPTS=()
for f in "$APP"/*.php; do
	n=$(basename "$f" .php)
	[ "$n" = "index" ] || SCRIPTS+=("$n")
done

(cd "$APP" && exec "$FRANKENPHP_BIN" run --config "$CADDYFILE" >/dev/null 2>&1) &
SPID=$!
trap 'kill $SPID 2>/dev/null || true; wait $SPID 2>/dev/null || true' EXIT
until curl -fsS localhost:22019/config/ >/dev/null 2>&1; do sleep 0.2; done

printf "%-20s %12s %10s %10s %10s\n" "script" "req/s" "avg" "p50" "p99"
sum=0
for s in "${SCRIPTS[@]}"; do
	out=$(wrk -t4 -c32 -d"${PER}s" --latency "http://localhost:22080/index.php?s=$s" 2>/dev/null || true)
	read -r rps avg p50 p99 < <(awk '
		/Requests\/sec:/ { rps = $2 }
		/^    Latency / && !avg { avg = $2 }
		/     50%/ { p50 = $2 }
		/     99%/ { p99 = $2 }
		END { print rps+0, avg, p50, p99 }
	' <<<"$out")
	printf "%-20s %12s %10s %10s %10s\n" "$s" "$rps" "$avg" "$p50" "$p99"
	sum=$(awk -v a="$sum" -v b="$rps" 'BEGIN { print a + b }')
done
printf "%-20s %12s\n" "total req/s" "$sum"
