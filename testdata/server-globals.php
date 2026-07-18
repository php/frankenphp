<?php

ignore_user_abort(true);

$server_vars = [
    'SCRIPT_NAME',
    'SCRIPT_FILENAME',
    'PHP_SELF',
    'PATH_INFO',
    'DOCUMENT_ROOT',
    'DOCUMENT_URI',
    'REQUEST_URI',
];
$handler = static function() use ($server_vars) {
    foreach ($server_vars as $var) {
        $value = $_SERVER[$var] ?? '(not set)';
        echo $value !== '' ? "$var: $value\n" : "$var:\n";
    }
};

if (isset($_SERVER['FRANKENPHP_WORKER'])) {
    for ($nbRequests = 0, $running = true; $running; ++$nbRequests) {
        $running = \frankenphp_handle_request($handler);
        gc_collect_cycles();
    }
} else {
    $handler();
}
