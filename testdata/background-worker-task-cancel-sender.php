<?php

frankenphp_handle_request(function () {
    // Ensure background worker is running
    frankenphp_worker_get_vars('task-worker');

    // Send a task then immediately cancel it
    $task = frankenphp_worker_task_send('task-worker', ['input' => 'cancelled']);
    fclose($task);

    echo 'cancelled';
});
