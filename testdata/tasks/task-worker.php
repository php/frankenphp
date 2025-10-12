<?php

$handleFunc = function ($task) {
    var_dump($task);
    echo $_SERVER['CUSTOM_VAR'] ?? 'no custom var';
};

$maxTasksBeforeRestarting = 1000;
$currentTask = 0;

while(frankenphp_handle_task($handleFunc) && $currentTask++ < $maxTasksBeforeRestarting) {
    // Keep handling tasks until there are no more tasks or the max limit is reached
}