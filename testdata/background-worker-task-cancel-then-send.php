<?php

frankenphp_handle_request(function () {
    // Ensure background worker is running
    frankenphp_worker_get_vars('task-worker');

    // Send a task and immediately cancel it
    $task1 = frankenphp_worker_task_send('task-worker', ['input' => 'will-be-cancelled']);
    fclose($task1);

    // Small delay to let the background worker dequeue the cancelled task
    usleep(50_000);

    // Send a second task - the background worker must still be responsive
    $task2 = frankenphp_worker_task_send('task-worker', ['input' => 'should-work']);
    $update = frankenphp_worker_task_read($task2);

    echo null === $update ? 'EOF' : ($update['result'] ?? 'NO_RESULT');
});
