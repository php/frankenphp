<?php

// Reaches the ready point, then ignores its handle and sleeps on the first
// run: only the force-kill can end that run, which is what the reboot test
// proves. Later runs park properly so the shutdown stays quick.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$handle->tick();
if (!file_exists($_SERVER['BG_ONCE'])) {
    @touch($_SERVER['BG_ONCE']);
    sleep(60);
}
@touch($_SERVER['BG_SENTINEL']);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
