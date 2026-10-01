<?php

// HTTP worker echoing what identified it at boot: FRANKENPHP_WORKER as it
// always was, and whether the background variable was set.
$name = $_SERVER['FRANKENPHP_WORKER'] ?? 'unset';
$mode = isset($_SERVER['FRANKENPHP_WORKER_BACKGROUND']) ? 'background' : 'http';
$handler = static function () use ($name, $mode) {
    echo "$name $mode";
};
while (frankenphp_handle_request($handler)) {
}
