<?php

// HTTP worker reading a vars namespace in each request. The first read of the
// run copies the snapshot, the next ones hand that copy out again while the
// snapshot doesn't change, which allocates nothing: "kept" tells whether the
// read of this request did so.
$handler = static function (): void {
    $before = memory_get_usage();
    $vars = frankenphp_get_vars($_GET['ns']);
    $kept = memory_get_usage() === $before;

    // a write separates the array of the caller, never the copy kept
    $vars['written'] = true;

    echo json_encode(['kept' => $kept, 'vars' => frankenphp_get_vars($_GET['ns'])]);
};

while (frankenphp_handle_request($handler)) {
}
