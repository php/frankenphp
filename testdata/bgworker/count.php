<?php

// Long-lived bg worker appending a line to $_SERVER['BG_COUNT_FILE'] on
// every run, so a test can count how many times it was (re)started.
set_time_limit(0);
if (!empty($_SERVER['BG_COUNT_FILE'])) {
    @file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
}
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
