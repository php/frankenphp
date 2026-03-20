<?php

frankenphp_handle_request(function () {
    frankenphp_worker_get_vars('progress-worker');

    $task = frankenphp_worker_task_send('progress-worker', ['job' => 'resize']);

    // Read all updates until EOF
    $updates = [];
    while (null !== $update = frankenphp_worker_task_read($task)) {
        $updates[] = $update['status'] . ':' . ($update['percent'] ?? $update['result'] ?? '');
    }

    echo implode(',', $updates);
});
