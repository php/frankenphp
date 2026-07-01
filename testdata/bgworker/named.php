<?php

// Long-lived bg worker that touches a per-name sentinel under
// $_SERVER['BG_SENTINEL_DIR'] so tests can confirm the right instance
// ran. The bg worker's $_SERVER['FRANKENPHP_WORKER'] value is the
// declared name, so the same fixture serves multiple distinct names
// across scopes.
set_time_limit(0);

$name = $_SERVER['FRANKENPHP_WORKER'] ?? 'unknown';
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    @touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . $name);
}

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
