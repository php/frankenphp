<?php

if (!isset($_SERVER['FRANKENPHP_WORKER'])) {
    die("Error: This script must be run in worker mode (FRANKENPHP_WORKER not set to '1')\n");
}

$i = 0;
do {
    $ok = frankenphp_handle_request(function () use ($i): void {
        echo sprintf("Nested request: %d\n", $i);
    });
    $i++;
} while ($ok);
