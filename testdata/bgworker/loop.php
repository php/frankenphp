<?php

// Long-lived bg worker that never ticks on its own initiative: its loop only
// calls frankenphp_worker_tick() once the handle is readable, the shape of a
// script driven by an event loop. The wake-up FrankenPHP sends at start is
// what makes it ready.
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
$handle = frankenphp_get_worker_handle();
do {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
} while (frankenphp_worker_tick());
