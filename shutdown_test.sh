#!/usr/bin/env bash
# Exercise process shutdown against the locally built FrankenPHP CLI.
set -euo pipefail

frankenphp_binary=${FRANKENPHP_BINARY:-./caddy/frankenphp/frankenphp}

server_pid=
slow_pid=
directory=

cleanup() {
	for pid in "$slow_pid" "$server_pid"; do
		if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
			kill -KILL "$pid" 2>/dev/null || true
			wait "$pid" 2>/dev/null || true
		fi
	done
}

on_exit() {
	local status=$?
	if (( status != 0 )) && [[ -n "$directory" && -f "$directory/server.log" ]]; then
		cat "$directory/server.log" >&2
	fi
	cleanup
	exit "$status"
}
trap on_exit EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
request() {
	local status error=0
	status=$(curl --silent --max-time 1 --output "$2" --write-out '%{http_code}' \
		"http://127.0.0.1:18080$1") || error=$?
	case "$error" in
		0) printf '%s' "$status" ;;
		7) printf 'closed' ;;
		# An accepted connection can close while HTTP shutdown begins.
		52|56) printf 'closing' ;;
		*) printf 'error-%s' "$error" ;;
	esac
}

# Fresh processes exercise Caddy's non-deterministic app shutdown order.
for attempt in {1..5}; do
	directory=$(mktemp -d)
	cat > "$directory/Caddyfile" <<CADDY
{
	debug
	admin off
	auto_https off
	shutdown_delay 500ms
	grace_period 10s
	frankenphp {
		num_threads 2
	}
}
http://127.0.0.1:18080 {
	root * $directory
	handle /health {
		@stopping vars {http.shutting_down} true
		respond @stopping "stopping" 503
		respond "ready" 200
	}
	php_server
}
CADDY
	cat > "$directory/index.php" <<'PHP'
<?php
set_time_limit(20);
if (isset($_GET['slow'])) {
	$deadline = hrtime(true) + 10_000_000_000;
	file_put_contents(__DIR__ . '/started', '1');
	while (!is_file(__DIR__ . '/release')) {
		if (hrtime(true) >= $deadline) {
			http_response_code(504);
			exit("Request was not released after HTTP draining\n");
		}
		usleep(10_000);
		clearstatcache();
	}
}
echo "ok\n";
PHP

	"$frankenphp_binary" run --config "$directory/Caddyfile" > "$directory/server.log" 2>&1 &
	server_pid=$!
	deadline=$((SECONDS + 20))
	until [[ "$(request / "$directory/response")" == 200 ]] && [[ "$(cat "$directory/response")" == ok ]]; do
		kill -0 "$server_pid" 2>/dev/null || fail 'Server exited before becoming ready'
		(( SECONDS < deadline )) || fail 'PHP did not become ready'
		sleep 0.05
	done

	curl --silent --show-error --fail --max-time 15 \
		'http://127.0.0.1:18080/?slow=1' > "$directory/slow-response" &
	slow_pid=$!
	deadline=$((SECONDS + 10))
	until [[ -f "$directory/started" ]]; do
		(( SECONDS < deadline )) || fail 'Slow PHP request did not start'
		sleep 0.02
	done
	kill -0 "$slow_pid" 2>/dev/null || fail 'Slow request finished before SIGTERM'
	kill -TERM "$server_pid"

	served_during_delay=false
	drained_inflight=false
	deadline=$((SECONDS + 20))
	while kill -0 "$server_pid" 2>/dev/null; do
		(( SECONDS < deadline )) || fail 'Server did not exit within the shutdown budget'
		health=$(request /health /dev/null)
		status=$(request / "$directory/response")
		[[ "$health" != error-* ]] || fail "Health request failed with curl exit code ${health#error-}"
		[[ "$status" != error-* ]] || fail "PHP request failed with curl exit code ${status#error-}"
		if [[ "$status" != closed && "$status" != closing ]]; then
			[[ "$status" == 200 ]] || fail "PHP returned HTTP $status during shutdown"
			[[ "$(cat "$directory/response")" == ok ]] || fail 'PHP returned an incomplete response'
			if [[ "$health" == 503 ]]; then served_during_delay=true; fi
		elif [[ "$status" == closed && "$served_during_delay" == true ]] && kill -0 "$slow_pid" 2>/dev/null; then
			drained_inflight=true
			touch "$directory/release"
		fi
		sleep 0.02
	done
	wait "$server_pid" || fail 'Server did not exit successfully'
	server_pid=
	grep -Fq '"msg":"FrankenPHP shut down"' "$directory/server.log" || fail 'PHP shutdown did not complete'
	wait "$slow_pid" || fail 'In-flight PHP request failed'
	slow_pid=
	[[ "$(cat "$directory/slow-response")" == ok ]] || fail 'In-flight response was incomplete'
	[[ "$served_during_delay" == true ]] || fail 'No successful PHP request during shutdown_delay'
	[[ "$drained_inflight" == true ]] || fail 'In-flight request did not span HTTP draining'
	echo "FrankenPHP graceful shutdown: $attempt/5 passed"
done

directory=$(mktemp -d)
cat > "$directory/Caddyfile" <<CADDY
{
	admin off
	auto_https off
	shutdown_delay 500ms
	grace_period 10s
	frankenphp {
		num_threads 2
		worker $directory/index.php 1
	}
}
http://127.0.0.1:18080 {
	root * $directory
	php_server
}
CADDY
cat > "$directory/index.php" <<'PHP'
<?php
$handler = static function (): void {
	echo "worker-ok\n";
};
while (frankenphp_handle_request($handler)) {
}
file_put_contents(__DIR__ . '/worker-stopped', 'cleanup-complete');
PHP

"$frankenphp_binary" run --config "$directory/Caddyfile" > "$directory/server.log" 2>&1 &
server_pid=$!
deadline=$((SECONDS + 20))
until [[ "$(request / "$directory/response")" == 200 ]] && [[ "$(cat "$directory/response")" == worker-ok ]]; do
	kill -0 "$server_pid" 2>/dev/null || fail 'Server exited before the worker became ready'
	(( SECONDS < deadline )) || fail 'Worker did not become ready'
	sleep 0.05
done
[[ ! -e "$directory/worker-stopped" ]] || fail 'Worker cleanup ran before SIGTERM'
kill -TERM "$server_pid"
deadline=$((SECONDS + 20))
while kill -0 "$server_pid" 2>/dev/null; do
	(( SECONDS < deadline )) || fail 'Worker shutdown exceeded the shutdown budget'
	sleep 0.02
done
wait "$server_pid" || fail 'Server did not exit successfully after worker shutdown'
server_pid=
[[ -f "$directory/worker-stopped" ]] || fail 'Worker cleanup did not run after the request loop'
[[ "$(cat "$directory/worker-stopped")" == cleanup-complete ]] || fail 'Worker cleanup did not complete'
echo 'FrankenPHP worker graceful shutdown: passed'
