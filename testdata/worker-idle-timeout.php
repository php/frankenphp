<?php
// Reports, on each real request: the worker instance id, how many idle ticks
// have happened, and the value of count($_SERVER) captured during the last idle
// tick (-1 if no idle tick yet). A stable instance id across idle ticks proves
// the worker was NOT restarted.
$instanceId = bin2hex(random_bytes(8));
$idleCount = 0;
$idleServerCount = -1;

while (true) {
    $result = frankenphp_handle_request(function () use (&$idleCount, &$idleServerCount, $instanceId) {
        echo sprintf("instance:%s,idle:%d,idleServerCount:%d", $instanceId, $idleCount, $idleServerCount);
    });

    if ($result === false) {
        break;
    }

    if ($result === FRANKENPHP_REQUEST_IDLE_TIMEOUT) {
        $idleCount++;
        $idleServerCount = count($_SERVER);
    }
}
