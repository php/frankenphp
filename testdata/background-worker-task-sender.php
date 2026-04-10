<?php

frankenphp_handle_request(function () {
    // First ensure the background worker is running
    frankenphp_worker_get_vars('task-worker');

    // Send a task
    $task = frankenphp_worker_task_send('task-worker', ['input' => 'hello']);

    // Read the result
    $update = frankenphp_worker_task_read($task);

    if (null === $update) {
        echo 'EOF';
    } else {
        echo $update['result'] ?? 'NO_RESULT';
    }
});
