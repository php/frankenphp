<?php

// Non-worker mode: no frankenphp_handle_request loop
frankenphp_worker_get_vars('task-worker');

$task = frankenphp_worker_task_send('task-worker', ['input' => 'from-non-worker']);

$update = frankenphp_worker_task_read($task);

echo null === $update ? 'EOF' : ($update['result'] ?? 'NO_RESULT');
