<?php

// HTTP worker reading the vars of "silent" in its bootstrap, then serving
// what it got.
try {
    $vars = frankenphp_get_vars('silent');
} catch (\Throwable $e) {
    $vars = get_class($e).': '.$e->getMessage();
}
while (frankenphp_handle_request(function () use ($vars): void {
    echo json_encode($vars);
})) {
}
