<?php

// Bg worker sending a task to the worker named in BG_TARGET while booting,
// writing the result (or the exception) to BG_SENTINEL, then parking.
set_time_limit(0);
try {
    $task = new \FrankenPHP\SentTaskHandle($_SERVER['BG_TARGET'], ['input' => 'relayed']);
    $result = json_encode($task->read());
} catch (\Throwable $e) {
    $result = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], $result);
$handle = new \FrankenPHP\WorkerHandle();
$handle->tick();
fgets($handle->getStream());
