<?php

$handleFunc = function ($task) {
    var_dump($task);
    echo $_SERVER['CUSTOM_VAR'] ?? 'no custom var';
    return "task completed: ".$task;
};

$maxTasksBeforeRestarting = 1000;
$currentTask = 0;

while(frankenphp_handle_request($handleFunc) && $currentTask++ < $maxTasksBeforeRestarting) {
    // Keep handling tasks until there are no more tasks or the max limit is reached
}