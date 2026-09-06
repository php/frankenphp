<?php

// Reaches the ready point (a zero-timeout wait on the handle counts), then
// exits non-zero: a crash after readiness restarts right away and does not
// count toward max_consecutive_failures. Runs are counted in BG_COUNT_FILE.
set_time_limit(0);
file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, 0);
exit(1);
