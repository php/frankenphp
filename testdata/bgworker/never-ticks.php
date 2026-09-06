<?php

// Bg worker that parks on its handle without ever calling tick(): it eats
// the wake-up sent at start, then selects until the drain, so only the boot
// timeout can end the wait Init() is in. With BG_FAIL_UNTIL, its first boots
// crash before that, counted in BG_COUNT_FILE.
set_time_limit(0);
if (isset($_SERVER['BG_FAIL_UNTIL'])) {
    $count = (int) @file_get_contents($_SERVER['BG_COUNT_FILE']) + 1;
    file_put_contents($_SERVER['BG_COUNT_FILE'], (string) $count);
    if ($count <= (int) $_SERVER['BG_FAIL_UNTIL']) {
        exit(1);
    }
}
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
fgets($stream);
$read = [$stream];
$write = $except = null;
stream_select($read, $write, $except, null);
