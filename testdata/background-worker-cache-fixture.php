<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Stable bg worker: publishes one snapshot and never updates.
frankenphp_set_vars([
    'marker' => 'cached-value',
]);

$stream = frankenphp_get_worker_handle();
if ($stream !== null) {
    $read = [$stream];
    $write = null;
    $except = null;
    stream_select($read, $write, $except, null);
}
