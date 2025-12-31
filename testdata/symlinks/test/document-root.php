<?php

if (isset($_SERVER['FRANKENPHP_WORKER']) && $_SERVER['FRANKENPHP_WORKER'] === '1') {
    $i = 0;
    do {
        $ok = frankenphp_handle_request(function () use ($i): void {
            echo "DOCUMENT_ROOT=" . $_SERVER['DOCUMENT_ROOT'] . "\n";
        });
        $i++;
    } while ($ok);
} else {
    echo "DOCUMENT_ROOT=" . $_SERVER['DOCUMENT_ROOT'] . "\n";
}
