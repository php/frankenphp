<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Probes the signaling stream returned by frankenphp_get_worker_handle():
// publishes its resource type and a basic metadata snapshot so the reader
// can confirm it is a real PHP stream resource, not a bogus zval.
$stream = frankenphp_get_worker_handle();

frankenphp_set_vars([
    'stream_type' => get_resource_type($stream),
    'is_resource' => is_resource($stream),
]);

$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
