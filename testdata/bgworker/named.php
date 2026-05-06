<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park.
set_time_limit(0);

// Lazy-start fixture for ensure() tests. Touches a per-name sentinel under
// $_SERVER['BG_SENTINEL_DIR'] so tests can confirm the right instance ran.
// Catch-all instances see their declared name through FRANKENPHP_WORKER.
$name = $_SERVER['FRANKENPHP_WORKER'] ?? 'unknown';
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    @touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . $name);
}

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
