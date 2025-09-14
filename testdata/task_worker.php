<?php

// A simple task worker example for FrankenPHP
$handleFunc = function ($task) {
    // Simulate a long-running task
    sleep(2);
    file_put_contents(__DIR__ .'/task_log.txt', date('Y-m-d H:i:s') . " - " . $task . PHP_EOL, FILE_APPEND);
    return "Task com pleted: " . $task;
};

while(frankenphp_handle_task($handleFunc)) {
}