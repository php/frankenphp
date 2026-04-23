#!/bin/bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
APP="$DIR/profiles/app"
MODE=${WORKER:+worker}
CADDYFILE="${1:-$APP/Caddyfile.${MODE:-regular}}"
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
trap '
	set +e
	if [ -n "${SPID:-}" ] && kill -0 "$SPID" 2>/dev/null; then
		kill "$SPID" 2>/dev/null
		for _ in $(seq 1 20); do
			kill -0 "$SPID" 2>/dev/null || break
			sleep 0.1
		done
		kill -9 "$SPID" 2>/dev/null
	fi
	wait 2>/dev/null
	exit 0
' EXIT
DEADLINE=$((SECONDS + 3))
until curl -fsS localhost:22019/config/ >/dev/null 2>&1; do
	[ "$SECONDS" -ge "$DEADLINE" ] && {
		echo "admin :22019 did not respond within 3s" >&2
		exit 1
	}
	sleep 0.2
done

printf "%-20s %12s %10s %10s %10s\n" "script" "req/s" "avg" "p50" "p99"
sum=0
for s in "${SCRIPTS[@]}"; do
	out=$(wrk -t4 -c256 -d"${BENCH_SEC:-8}s" --latency "http://localhost:22080/index.php?s=$s" 2>/dev/null || true)
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
