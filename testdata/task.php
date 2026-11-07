<?php

// Sends a task and prints its updates, one JSON line each, then "done".
// The query string builds the payload (input, steps, sleep_ms, crash, mark),
// ns picks the task namespace, timeout bounds the pickup, close_early abandons
// the task instead of reading it and wait_for holds the reads until that file
// exists.
try {
    $payload = array_intersect_key($_GET, array_flip(['input', 'steps', 'sleep_ms', 'crash', 'mark']));
    $payload = array_map(fn ($v) => is_numeric($v) ? (int) $v : $v, $payload);
    $task = new \FrankenPHP\SentTaskHandle($_GET['ns'] ?? 'echo', $payload, isset($_GET['timeout']) ? (float) $_GET['timeout'] : 30.0);
    if (isset($_GET['close_early'])) {
        $task->abandon();
        echo 'closed';

        return;
    }
    for ($i = 0; isset($_GET['wait_for']) && !file_exists($_GET['wait_for']) && $i < 1000; ++$i) {
        usleep(10_000);
    }
    while (null !== $update = $task->read()) {
        echo json_encode($update), "\n";
    }
    echo 'done';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
