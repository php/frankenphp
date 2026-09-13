<?php

// Crashes on its first BG_FAIL_UNTIL boots, then parks: the boot-failure
// backoff up to a success. Boots are counted in BG_COUNT_FILE.
set_time_limit(0);
$count = (int) @file_get_contents($_SERVER['BG_COUNT_FILE']) + 1;
file_put_contents($_SERVER['BG_COUNT_FILE'], (string) $count);
if ($count <= (int) $_SERVER['BG_FAIL_UNTIL']) {
    exit(1);
}
@touch($_SERVER['BG_SENTINEL']);
$handle = frankenphp_get_worker_handle();
while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
