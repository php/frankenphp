<?php

// Reaches the ready point, then ignores its handle and sleeps on the first
// run: only the force-kill can end that run, which is what the reboot test
// proves. Later runs park properly so the shutdown stays quick.
set_time_limit(0);
frankenphp_worker_tick();
if (!file_exists($_SERVER['BG_ONCE'])) {
    @touch($_SERVER['BG_ONCE']);
    sleep(60);
}
@touch($_SERVER['BG_SENTINEL']);
$handle = frankenphp_get_worker_handle();
while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
