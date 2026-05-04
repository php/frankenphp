<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Crash-and-recover fixture. On the first boot (no marker), publishes
// count=1 then exit(1). On the second boot (marker present), publishes
// count=2 and parks on the stop stream. Tests that:
//   - vars stay readable across the restart (persistent storage),
//   - the worker is restarted automatically after the crash,
//   - the post-restart publish overwrites the first snapshot.
$marker = sys_get_temp_dir() . '/bg-worker-crash-' . getmypid();

if (!file_exists($marker)) {
    frankenphp_set_vars(['count' => 1, 'phase' => 'pre-crash']);
    file_put_contents($marker, '1');
    exit(1);
}

frankenphp_set_vars(['count' => 2, 'phase' => 'post-restart']);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
@unlink($marker);
