<?php

// Occupies the only thread of the worker (touching the file given as mark
// at pickup, and keeping the stream so the worker stays busy), then waits
// without timeout for a second task to be picked up: only a drain of the
// calling thread, shutdown or restart, ends that wait.
try {
    $slow = frankenphp_send_task('echo', ['input' => 'slow', 'sleep_ms' => 1500, 'mark' => $_GET['mark']]);
    frankenphp_send_task('echo', ['input' => 'never'], null);
    echo 'picked up';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
