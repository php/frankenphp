<?php

// Sends a task and prints its updates, one JSON line each, then "done".
// The query string builds the payload (input, steps, sleep_ms, crash, mark),
// name picks the worker, timeout bounds the pickup and close_early abandons
// the task instead of reading it.
try {
    $payload = array_intersect_key($_GET, array_flip(['input', 'steps', 'sleep_ms', 'crash', 'mark']));
    $payload = array_map(fn ($v) => is_numeric($v) ? (int) $v : $v, $payload);
    $task = frankenphp_send_task($_GET['name'] ?? 'echo', $payload, isset($_GET['timeout']) ? (float) $_GET['timeout'] : 30.0);
    if (isset($_GET['close_early'])) {
        fclose($task);
        echo 'closed';

        return;
    }
    while (null !== $update = frankenphp_read_task($task)) {
        echo json_encode($update), "\n";
    }
    echo 'done';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
