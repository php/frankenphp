<?php

// Bg worker publishing a literal, which opcache makes an immutable array. The
// runs after the first publish it again only after BG_REPUBLISH_DELAY_MS, so
// readers meanwhile get the snapshot published before a reboot.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
if (file_exists($_SERVER['BG_SENTINEL'])) {
    usleep(1000 * (int) $_SERVER['BG_REPUBLISH_DELAY_MS']);
}
touch($_SERVER['BG_SENTINEL']);
$handle->setVars($handle->getNamespaces()['vars'][0], ['answer' => 42, 'nested' => ['a' => 1, 'list' => [true, null, 1.5, 'x']]]);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
