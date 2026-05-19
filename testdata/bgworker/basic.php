<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Minimal background worker: publishes a small vars set then parks on the
// stop stream until FrankenPHP drains it.
frankenphp_set_vars([
    'message' => 'hello from background worker',
    'count' => 42,
    'ready_at' => microtime(true),
]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
