<?php

// Sends a task and prints its updates, one JSON line each, then "done".
// The query string builds the payload (input, steps, sleep_ms, pause_ms, crash, drop, abort, mark),
// ns picks the task namespace, timeout bounds the pickup, close_early abandons
// the task instead of reading it, once the worker touched the mark file at
// pickup when there is one.
try {
    $payload = array_intersect_key($_GET, array_flip(['input', 'steps', 'sleep_ms', 'pause_ms', 'crash', 'drop', 'abort', 'mark']));
    $payload = array_map(fn ($v) => is_numeric($v) ? (int) $v : $v, $payload);
    $task = new \FrankenPHP\SentTaskHandle($_GET['ns'] ?? 'echo', $payload, isset($_GET['timeout']) ? (float) $_GET['timeout'] : 30.0);
    if (isset($_GET['close_early'])) {
        // a task abandoned before its pickup never reaches the worker
        for ($i = 0; isset($_GET['mark']) && !file_exists($_GET['mark']) && $i < 5000; ++$i) {
            usleep(1000);
        }
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
