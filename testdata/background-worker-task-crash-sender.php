<?php

frankenphp_handle_request(function () {
    try {
        $task = frankenphp_worker_task_send('crash-task-worker', ['input' => 'test']);
        $update = frankenphp_worker_task_read($task);
        echo null === $update ? 'EOF' : 'GOT_DATA';
    } catch (\RuntimeException $e) {
        echo 'ERROR:' . $e->getMessage();
    }
});
