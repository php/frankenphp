<?php

// Long-lived bg worker that touches a per-name sentinel under
// $_SERVER['BG_SENTINEL_DIR'] so tests can confirm the right instance
// ran. The bg worker's $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] value is
// the declared name, so the same fixture serves multiple distinct names
// across scopes.
set_time_limit(0);

$name = $_SERVER['FRANKENPHP_WORKER_BACKGROUND'] ?? 'unknown';
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    @touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . $name);
}

$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
