<?php

// Bg worker writing the two server variables identifying it, so the test
// can assert their exact values, then parking.
set_time_limit(0);
file_put_contents($_SERVER['BG_SENTINEL'], var_export([
    'worker' => $_SERVER['FRANKENPHP_WORKER'] ?? null,
    'background' => isset($_SERVER['FRANKENPHP_WORKER_BACKGROUND']) ? 'set' : 'unset',
], true));
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
