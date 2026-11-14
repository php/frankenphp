<?php

// Long-lived bg worker that touches a per-name sentinel under
// $_SERVER['BG_SENTINEL_DIR'] so tests can confirm the right instance
// ran. The sentinel is named after the worker, so the same fixture serves
// several workers.
set_time_limit(0);

$handle = new \FrankenPHP\WorkerHandle();
if (!empty($_SERVER['BG_SENTINEL_DIR'])) {
    @touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . $handle->getName());
}

$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
