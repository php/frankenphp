<?php

// Crash-and-recover fixture. On the first boot (no marker), creates the
// marker and exit(1) so the bg worker thread observes a non-zero exit
// status. On the second boot (marker present), touches a "restarted"
// sentinel and parks on the stop pipe. The test asserts the restarted
// sentinel appears, proving the crash-restart loop ran.
set_time_limit(0);

$marker = $_SERVER['BG_CRASH_MARKER'] ?? '';
$restarted = $_SERVER['BG_RESTARTED_SENTINEL'] ?? '';

if ($marker === '' || $restarted === '') {
    // Misconfigured: don't loop forever on a missing env.
    exit(2);
}

if (!file_exists($marker)) {
    file_put_contents($marker, '1');
    exit(1);
}

@touch($restarted);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
