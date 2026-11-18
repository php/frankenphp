<?php

// fails the first boot only, before reaching frankenphp_handle_request()
$marker = $_SERVER['BOOT_MARKER'];
if (!file_exists($marker)) {
    touch($marker);
    exit(1);
}

do {
    $ok = frankenphp_handle_request(function (): void {
        echo 'booted after a failure';
    });
} while ($ok);
