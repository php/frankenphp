<?php

// Sends two slow tasks to a pool of two threads and gathers the results
// with stream_select(): both are processed at once, by different threads.
try {
    $tasks = [
        frankenphp_send_task('pool', ['input' => 'a', 'sleep_ms' => 300]),
        frankenphp_send_task('pool', ['input' => 'b', 'sleep_ms' => 300]),
    ];
    $threads = [];
    while ($tasks) {
        $read = $tasks;
        $write = $except = null;
        if (!stream_select($read, $write, $except, 5)) {
            throw new \RuntimeException('stream_select() timed out');
        }
        foreach ($read as $i => $stream) {
            if (null === $update = frankenphp_read_task($stream)) {
                fclose($stream);
                unset($tasks[$i]);
                continue;
            }
            $threads[$update['result']] = $update['thread'];
        }
    }
    ksort($threads);
    echo json_encode(array_keys($threads)), "\n", 2 === count(array_unique($threads)) ? 'two threads' : 'one thread';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
