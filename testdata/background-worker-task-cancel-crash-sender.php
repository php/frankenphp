<?php

frankenphp_handle_request(function () {
    frankenphp_worker_get_vars('cancel-crash-worker');

    $task = frankenphp_worker_task_send('cancel-crash-worker', ['input' => 'test']);
    // Wait a bit for background worker to receive the task, then cancel
    usleep(50_000);
    fclose($task);

    echo 'cancelled';
});
