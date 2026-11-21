<?php

// The stream of a sent task only reaches its end once the updates the worker
// left behind are read.
try {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'x', 'steps' => 2]);
    $stream = $task->getStream();
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, 5);
    // time for the worker to complete the task
    usleep(100_000);
    $before = feof($stream);
    for ($updates = 0; null !== $task->read(); ++$updates) {
    }
    echo json_encode(['before' => $before, 'updates' => $updates, 'after' => feof($stream)]);
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
