<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Records both FRANKENPHP_WORKER_BACKGROUND and FRANKENPHP_WORKER
// from $_SERVER so the test can confirm both are populated.
frankenphp_set_vars([
    'name' => $_SERVER['FRANKENPHP_WORKER'] ?? 'MISSING',
    'is_background' => $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] ?? 'MISSING',
]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
