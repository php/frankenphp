<?php

$numberOfRequests = 0;
$printNumberOfRequests = function () use (&$numberOfRequests) {
    $numberOfRequests++;
    echo "requests:$numberOfRequests";
};

while (frankenphp_handle_request($printNumberOfRequests)) {

}
