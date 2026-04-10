<?php

frankenphp_handle_request(function () {
    // First ensure the background worker is running
    frankenphp_worker_get_vars('pool-worker');

    // Send two tasks - with num=2, both should be processed
    $task1 = frankenphp_worker_task_send('pool-worker', ['input' => 'first']);
    $task2 = frankenphp_worker_task_send('pool-worker', ['input' => 'second']);

    // Read results from both tasks
    $r1 = frankenphp_worker_task_read($task1);
    $r2 = frankenphp_worker_task_read($task2);

    $results = [];
    if (null !== $r1) $results[] = $r1['result'];
    if (null !== $r2) $results[] = $r2['result'];

    sort($results);
    echo implode(',', $results);
});
