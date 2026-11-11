<?php

// Waits in tick() itself, BG_TIMEOUT seconds at most, without a limit when it
// is "null": each line of BG_SENTINEL is a return of tick(), with the
// milliseconds it took.
$handle = new \FrankenPHP\WorkerHandle();
$timeout = 'null' === $_SERVER['BG_TIMEOUT'] ? null : (float) $_SERVER['BG_TIMEOUT'];
try {
    $handle->tick(-1.0);
} catch (\ValueError $e) {
    file_put_contents($_SERVER['BG_SENTINEL'], $e->getMessage() . "\n", FILE_APPEND);
}
do {
    $start = hrtime(true);
    $running = $handle->tick($timeout);
    file_put_contents($_SERVER['BG_SENTINEL'], sprintf("%s %d\n", $running ? 'tick' : 'drained', (hrtime(true) - $start) / 1e6), FILE_APPEND);
} while ($running);
