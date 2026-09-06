<?php

// Pool bg worker: num > 1 threads share the name; each touches a file of
// its own under BG_SENTINEL_DIR, then parks on its own handle.
set_time_limit(0);
@touch($_SERVER['BG_SENTINEL_DIR'] . DIRECTORY_SEPARATOR . bin2hex(random_bytes(8)));
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
