<?php

// Bg worker sending a task to the worker named in BG_TARGET while booting,
// writing the result (or the exception) to BG_SENTINEL, then parking.
set_time_limit(0);
try {
    $task = frankenphp_send_task($_SERVER['BG_TARGET'], ['input' => 'relayed']);
    $result = json_encode(frankenphp_read_task($task));
} catch (\Throwable $e) {
    $result = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], $result);
frankenphp_worker_tick();
fgets(frankenphp_get_worker_handle());
