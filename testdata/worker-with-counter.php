<?php

$numberOfRequests = 0;
$printNumberOfRequests = function () use (&$numberOfRequests) {
    $numberOfRequests++;
    echo "requests:$numberOfRequests";

    frankenphp_dispatch_task('task1');
    frankenphp_dispatch_task('task2');
    frankenphp_dispatch_task('task3');
    frankenphp_dispatch_task('task4');
};

while (frankenphp_handle_request($printNumberOfRequests)) {

}
