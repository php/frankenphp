<?php

require __DIR__ . '/background-worker-helper.php';

enum TestStatus {
    case Active;
    case Inactive;
}

$results = [];

// int values allowed
try {
    frankenphp_worker_set_vars(['KEY' => 123]);
    $results[] = 'INT_VAL:allowed';
} catch (\Throwable $e) {
    $results[] = 'INT_VAL:blocked';
}

// int keys allowed
try {
    frankenphp_worker_set_vars([0 => 'val']);
    $results[] = 'INT_KEY:allowed';
} catch (\Throwable $e) {
    $results[] = 'INT_KEY:blocked';
}

// nested arrays allowed
try {
    frankenphp_worker_set_vars(['nested' => ['a' => 1, 'b' => [true, null]]]);
    $results[] = 'NESTED:allowed';
} catch (\Throwable $e) {
    $results[] = 'NESTED:blocked';
}

// objects rejected
try {
    frankenphp_worker_set_vars(['KEY' => new \stdClass()]);
    $results[] = 'OBJECT:allowed';
} catch (\ValueError $e) {
    $results[] = 'OBJECT:blocked';
}

// references rejected
try {
    $ref = 'hello';
    frankenphp_worker_set_vars(['KEY' => &$ref]);
    $results[] = 'REFERENCE:allowed';
} catch (\ValueError $e) {
    $results[] = 'REFERENCE:blocked';
}

// enums allowed - final set_vars with all results
try {
    $results[] = 'ENUM:allowed'; // if we get here, the call below will confirm it
    frankenphp_worker_set_vars(['status' => TestStatus::Active, 'RESULTS' => implode(',', $results)]);
} catch (\Throwable $e) {
    $results[array_key_last($results)] = 'ENUM:blocked';
    frankenphp_worker_set_vars(['RESULTS' => implode(',', $results)]);
}

while (!background_worker_should_stop(30)) {
}
