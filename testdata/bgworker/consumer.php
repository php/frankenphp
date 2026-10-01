<?php

// Bg worker reading another worker's vars while booting, writing the result
// (or the exception) to BG_SENTINEL, then parking.
set_time_limit(0);
try {
    $result = json_encode(frankenphp_get_vars($_SERVER['BG_CONSUME']));
} catch (\Throwable $e) {
    $result = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], $result);
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
