<?php

// Occupies the only thread of the worker with a slow task, then sends a
// second one with a short timeout: nobody picks it up in time.
try {
    $slow = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'slow', 'sleep_ms' => 500]);
    try {
        new \FrankenPHP\SentTaskHandle('echo', ['input' => 'late'], 0.1);
        echo "no timeout\n";
    } catch (\RuntimeException $e) {
        echo $e->getMessage(), "\n";
    }
    // read to the end: leaving with the task open would abandon it, and
    // the worker completes it right after the update below
    $update = null;
    while (null !== $next = $slow->read()) {
        $update = $next;
    }
    echo json_encode($update);
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
