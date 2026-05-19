<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Lazy-start fixture for ensure() tests. Echoes its declared name through
// FRANKENPHP_WORKER and publishes it via set_vars so tests can read it
// back through frankenphp_get_vars.
$name = $_SERVER['FRANKENPHP_WORKER'] ?? 'unknown';
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    @touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . $name);
}
frankenphp_set_vars([
    'FRANKENPHP_WORKER' => $name,
    'count' => 1,
]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
