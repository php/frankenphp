<?php

// Touches BG_EAGER_SENTINEL once readiness is reached, then parks.
// Drives the eager catch-all readiness test.
set_time_limit(0);

$sentinel = $_SERVER['BG_EAGER_SENTINEL'] ?? '';
if ($sentinel !== '') {
    @touch($sentinel);
}

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
