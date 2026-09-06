<?php

// HTTP worker echoing what identified it at boot: its name, and whether the
// background flag was set.
$name = $_SERVER['FRANKENPHP_WORKER'] ?? 'unset';
$mode = isset($_SERVER['FRANKENPHP_WORKER_BACKGROUND']) ? 'background' : 'http';
$handler = static function () use ($name, $mode) {
    echo "$name $mode";
};
while (frankenphp_handle_request($handler)) {
}
