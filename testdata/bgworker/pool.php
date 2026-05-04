<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Pool worker: num > 1 means multiple threads share this worker name.
// Each thread runs in its own ZTS context but shares a single
// backgroundWorkerState (via the worker name), so set_vars writes from
// any pool thread are visible to readers.
frankenphp_set_vars([
    'name' => $_SERVER['FRANKENPHP_WORKER'] ?? 'unknown',
    'pid' => getmypid(),
]);

// Per-thread sentinel (random suffix) lets tests count distinct booted
// threads, which set_vars alone cannot show since the namespace is
// per-worker-name not per-thread.
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    $path = $_SERVER['BG_SENTINEL_DIR'] . '/pool-' . bin2hex(random_bytes(8));
    @touch($path);
}

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
