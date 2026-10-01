<?php

// Bg worker reporting what its handle implements: Io\Poll\Handle comes from
// the poll API on 8.6, from FrankenPHP below it.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
file_put_contents($_SERVER['BG_SENTINEL'], json_encode([
    'handle' => $handle instanceof \Io\Poll\Handle,
    'internal' => (new ReflectionClass(\Io\Poll\Handle::class))->isInternal(),
]));
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
