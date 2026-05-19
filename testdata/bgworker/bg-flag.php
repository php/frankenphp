<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Write a sentinel containing var_export() of FRANKENPHP_WORKER_BACKGROUND
// so the test can assert the exact value injected by C ($_SERVER flag).
if (!empty($_SERVER['BG_SENTINEL'])) {
    $value = $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] ?? 'MISSING';
    @file_put_contents($_SERVER['BG_SENTINEL'], var_export($value, true));
}

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
