<?php

// Reaches the ready point, then ignores its handle and sleeps on the first
// run: only the force-kill can end that run, which is what the reboot test
// proves. Later runs park properly so the shutdown stays quick.
set_time_limit(0);
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, 0);
if (!file_exists($_SERVER['BG_ONCE'])) {
    @touch($_SERVER['BG_ONCE']);
    sleep(60);
}
@touch($_SERVER['BG_SENTINEL']);
$read = [$stream];
stream_select($read, $write, $except, null);
