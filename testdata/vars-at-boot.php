<?php

// HTTP worker reading the vars of "silent" in its bootstrap, then serving
// what it got. With VARS_UNCAUGHT, a failed read ends the run instead.
try {
    $vars = frankenphp_get_vars('silent');
} catch (\Throwable $e) {
    if (!empty($_SERVER['VARS_UNCAUGHT'])) {
        throw $e;
    }
    $vars = get_class($e).': '.$e->getMessage();
}
while (frankenphp_handle_request(function () use ($vars): void {
    echo json_encode($vars);
})) {
}
