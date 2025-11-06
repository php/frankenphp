<?php

$numberOfRequests = 0;
$printNumberOfRequests = function () use (&$numberOfRequests) {
    $numberOfRequests++;
    echo "requests:$numberOfRequests";
    sleep(1);
};

while (frankenphp_handle_request($printNumberOfRequests)) {

}
