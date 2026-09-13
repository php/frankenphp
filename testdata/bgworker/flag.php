<?php

// Bg worker writing the two server variables identifying it, so the test
// can assert their exact values, then parking.
set_time_limit(0);
file_put_contents($_SERVER['BG_SENTINEL'], var_export([
    'worker' => $_SERVER['FRANKENPHP_WORKER'] ?? null,
    'background' => isset($_SERVER['FRANKENPHP_WORKER_BACKGROUND']) ? 'set' : 'unset',
], true));
$handle = frankenphp_get_worker_handle();
while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
