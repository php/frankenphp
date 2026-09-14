<?php

// Records what frankenphp_worker_tick() returns: true twice while running,
// then false twice once drained, the second one proving the drain sticks.
$handle = frankenphp_get_worker_handle();
$seen = [];
$seen[] = frankenphp_worker_tick() ? 'true' : 'false';
$seen[] = frankenphp_worker_tick() ? 'true' : 'false';
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));

$read = [$handle];
$write = $except = null;
stream_select($read, $write, $except, null);

$seen[] = frankenphp_worker_tick() ? 'true' : 'false';
$seen[] = frankenphp_worker_tick() ? 'true' : 'false';
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));
