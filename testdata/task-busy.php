<?php

// Occupies the only thread of the worker with a slow task, then sends a
// second one with a short timeout: nobody picks it up in time, which its
// read() reports.
try {
    $slow = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'slow', 'sleep_ms' => 500]);
    try {
        $late = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'late'], 0.1);
        $late->read();
        echo "no timeout\n";
    } catch (\RuntimeException $e) {
        echo $e->getMessage(), "\n";
    }
    // read to the end: the result is the last update
    $update = null;
    while (null !== $next = $slow->read()) {
        $update = $next;
    }
    echo json_encode($update);
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
