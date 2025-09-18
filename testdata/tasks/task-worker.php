<?php

$handleFunc = function ($task) {
    echo "$task";
    echo $_SERVER['CUSTOM_VAR'] ?? 'no custom var';
};

$maxRequests = 1000;
$currentRequest = 0;

while(frankenphp_handle_task($handleFunc) && $currentRequest++ < $maxRequests) {
    // Keep handling tasks until there are no more tasks or the max limit is reached
}