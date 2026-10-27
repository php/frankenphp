<?php

// Publishes a snapshot larger than the memory_limit of its readers, then
// republishes it every 50ms, counting the publications in BG_SENTINEL.
ini_set('memory_limit', '-1');
$big = [];
for ($i = 0; $i < 60000; ++$i) {
    $big[] = str_repeat('x', 100).$i;
}
$handle = new \FrankenPHP\WorkerHandle();
$publications = 0;
$handle->setVars('big', $big);
file_put_contents($_SERVER['BG_SENTINEL'], (string) ++$publications);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, 0, 50000);
    $handle->setVars('big', $big);
    file_put_contents($_SERVER['BG_SENTINEL'], (string) ++$publications);
}
