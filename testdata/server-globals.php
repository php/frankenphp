<?php

ignore_user_abort(true);

$handler = static function() {
    echo "SCRIPT_NAME: " . ($_SERVER['SCRIPT_NAME'] ?? '(not set)') . "<br>";
    echo "SCRIPT_FILENAME: " . ($_SERVER['SCRIPT_FILENAME'] ?? '(not set)') . "<br>";
    echo "PHP_SELF: " . ($_SERVER['PHP_SELF'] ?? '(not set)') . "<br>";
    echo "PATH_INFO: " . ($_SERVER['PATH_INFO'] ?? '(not set)') . "<br>";
    echo "DOCUMENT_ROOT: " . ($_SERVER['DOCUMENT_ROOT'] ?? '(not set)') . "<br>";
    echo "REQUEST_URI: " . ($_SERVER['REQUEST_URI'] ?? '(not set)') . "<br>";
};

if (isset($_SERVER['FRANKENPHP_WORKER'])) {
    for ($nbRequests = 0, $running = true; $running; ++$nbRequests) {
        $running = \frankenphp_handle_request($handler);
        gc_collect_cycles();
    }
} else {
    $handler();
}
