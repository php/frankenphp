<?php

// Bg worker publishing on its first run only, so a run started by a reboot
// leaves the snapshot published before it in place. The snapshot is a
// literal, which opcache makes an immutable array.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
if (!file_exists($_SERVER['BG_SENTINEL'])) {
    touch($_SERVER['BG_SENTINEL']);
    $handle->setVars(['answer' => 42, 'nested' => ['a' => 1, 'list' => [true, null, 1.5, 'x']]]);
}
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
