<?php

// The error paths of the task functions, from a request thread.
$cases = [
    'unknown' => fn () => frankenphp_send_task('nope', []),
    'payload' => fn () => frankenphp_send_task('echo', ['object' => new stdClass()]),
    'timeout' => fn () => frankenphp_send_task('echo', [], -1),
    'receive' => fn () => frankenphp_receive_task(),
    'update' => fn () => frankenphp_update_task(fopen('php://memory', 'r'), []),
    'read' => fn () => frankenphp_read_task(fopen('php://memory', 'r')),
];
foreach ($cases as $name => $case) {
    try {
        $case();
        echo $name, ": no exception\n";
    } catch (\Throwable $e) {
        echo $name, ': ', get_class($e), ': ', $e->getMessage(), "\n";
    }
}
