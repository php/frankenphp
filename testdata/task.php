<?php

// Sends a task and prints its updates, one JSON line each, then "done".
// The query string builds the payload (input, steps, sleep_ms, crash, mark),
// name picks the worker, timeout bounds the pickup and close_early abandons
// the task instead of reading it.
try {
    $payload = array_intersect_key($_GET, array_flip(['input', 'steps', 'sleep_ms', 'crash', 'mark']));
    $payload = array_map(fn ($v) => is_numeric($v) ? (int) $v : $v, $payload);
    $task = new \FrankenPHP\SentTaskHandle($_GET['name'] ?? 'echo', $payload, isset($_GET['timeout']) ? (float) $_GET['timeout'] : 30.0);
    if (isset($_GET['close_early'])) {
        $task->abandon();
        echo 'closed';

        return;
    }
    while (null !== $update = $task->read()) {
        echo json_encode($update), "\n";
    }
    echo 'done';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
