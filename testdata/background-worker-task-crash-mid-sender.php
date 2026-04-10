<?php

frankenphp_handle_request(function () {
    frankenphp_worker_get_vars('crash-mid-worker');

    $task = frankenphp_worker_task_send('crash-mid-worker', ['input' => 'hello']);

    // Background worker received the task but crashed before fclose
    // task_read should throw RuntimeException
    try {
        $update = frankenphp_worker_task_read($task);
        echo null === $update ? 'EOF' : ('GOT:' . json_encode($update));
    } catch (\RuntimeException $e) {
        echo 'CRASH:' . $e->getMessage();
    }
    fclose($task);
});
