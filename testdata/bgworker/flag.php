<?php

// Bg worker writing the two worker variables as it sees them and its name,
// so the test can assert their exact values, then parking.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
file_put_contents($_SERVER['BG_SENTINEL'], var_export([
    'worker' => $_SERVER['FRANKENPHP_WORKER'] ?? 'unset',
    'background' => $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] ?? 'unset',
    'name' => $handle->getName(),
], true));
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
