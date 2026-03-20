<?php

require __DIR__ . '/background-worker-helper.php';

$stream = frankenphp_worker_get_signaling_stream();

frankenphp_worker_set_vars(['READY' => '1']);

while (true) {
    $r = [$stream];
    $w = $e = [];
    if (!stream_select($r, $w, $e, 30)) {
        continue;
    }

    $signal = fgets($stream);

    if ("stop\n" === $signal) {
        break;
    }

    if ("task\n" === $signal) {
        if ($task = frankenphp_worker_task_receive()) {
            [$handle, $payload] = $task;
            // Simulate work - sleep briefly so both threads can be busy
            usleep(100000); // 100ms
            frankenphp_worker_task_update($handle, ['result' => 'processed:' . ($payload['input'] ?? 'none')]);
            fclose($handle);
        }
    }
}
