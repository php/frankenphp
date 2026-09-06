<?php

// Bg worker publishing a value outside the whitelist, writing the exception
// to BG_SENTINEL, then parking.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
try {
    $handle->setVars(['object' => new stdClass()]);
    $result = 'no exception';
} catch (\Throwable $e) {
    $result = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], $result);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
