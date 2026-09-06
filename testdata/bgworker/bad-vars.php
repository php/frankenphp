<?php

// Bg worker publishing a value outside the whitelist, writing the exception
// to BG_SENTINEL, then parking.
set_time_limit(0);
try {
    frankenphp_set_vars(['object' => new stdClass()]);
    $result = 'no exception';
} catch (\Throwable $e) {
    $result = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], $result);
$handle = frankenphp_get_worker_handle();
while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
