<?php

// Occupies the only thread of the worker with a slow task, then sends a
// second one with a short timeout: nobody picks it up in time.
try {
    $slow = frankenphp_send_task('echo', ['input' => 'slow', 'sleep_ms' => 500]);
    try {
        frankenphp_send_task('echo', ['input' => 'late'], 0.1);
        echo "no timeout\n";
    } catch (\RuntimeException $e) {
        echo $e->getMessage(), "\n";
    }
    echo json_encode(frankenphp_read_task($slow));
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
