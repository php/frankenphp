<?php
// Worker that tracks total requests handled across restarts.
// Uses a unique instance ID per worker script execution.
$instanceId = bin2hex(random_bytes(8));
$counter = 0;

while (frankenphp_handle_request(function () use (&$counter, $instanceId) {
    $counter++;
    echo "instance:$instanceId,count:$counter";
})) {}
