<?php

// Bg worker publishing a snapshot, then parking. BG_PUBLISH_DELAY_MS delays
// the publication so readers booting concurrently have to wait for it.
set_time_limit(0);
if (!empty($_SERVER['BG_PUBLISH_DELAY_MS'])) {
    usleep(1000 * (int) $_SERVER['BG_PUBLISH_DELAY_MS']);
}
$handle = new \FrankenPHP\WorkerHandle();
$handle->setVars([
    'answer' => 42,
    'value' => $_SERVER['BG_PUBLISH_VALUE'] ?? 'default',
    'nested' => ['a' => 1, 'list' => [true, null, 1.5, 'x']],
    'worker' => $_SERVER['FRANKENPHP_WORKER_BACKGROUND'],
]);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
