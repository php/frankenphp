<?php

// Crashes on boots 1..BG_FAIL_UNTIL, then succeeds. Each boot writes a
// boot<N> marker so the script counts attempts; success path writes a
// "ready" sentinel.
set_time_limit(0);

$markerDir = $_SERVER['BG_MARKER_DIR'] ?? '';
if ($markerDir === '') {
    exit(2);
}

$attempt = 1;
while (file_exists($markerDir . '/boot' . $attempt)) {
    $attempt++;
}
@touch($markerDir . '/boot' . $attempt);

$failUntil = (int)($_SERVER['BG_FAIL_UNTIL'] ?? '2');
if ($attempt <= $failUntil) {
    exit(1);
}

@touch($markerDir . '/ready');
// set_vars + get_worker_handle together close the readiness channel on
// sidekicks; either alone is not sufficient.
frankenphp_set_vars([]);
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
