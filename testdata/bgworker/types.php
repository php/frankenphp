<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Type-validation fixture. Exercises every branch of the persistent-zval
// helper from inside a bg worker: int values/keys, nested arrays, and
// rejected types (objects, references). Aggregates the outcome into a
// RESULTS string and publishes it along with an enum case so the reader
// can verify enum round-trip.
enum BgTestStatus {
    case Active;
    case Inactive;
}

$results = [];

try {
    frankenphp_set_vars(['KEY' => 123]);
    $results[] = 'INT_VAL:allowed';
} catch (\Throwable $e) {
    $results[] = 'INT_VAL:blocked';
}

try {
    frankenphp_set_vars([0 => 'val']);
    $results[] = 'INT_KEY:allowed';
} catch (\Throwable $e) {
    $results[] = 'INT_KEY:blocked';
}

try {
    frankenphp_set_vars(['nested' => ['a' => 1, 'b' => [true, null]]]);
    $results[] = 'NESTED:allowed';
} catch (\Throwable $e) {
    $results[] = 'NESTED:blocked';
}

try {
    frankenphp_set_vars(['KEY' => new \stdClass()]);
    $results[] = 'OBJECT:allowed';
} catch (\ValueError $e) {
    $results[] = 'OBJECT:blocked';
}

try {
    $ref = 'hello';
    $arr = ['KEY' => &$ref];
    frankenphp_set_vars($arr);
    $results[] = 'REFERENCE:allowed';
} catch (\ValueError $e) {
    $results[] = 'REFERENCE:blocked';
}

frankenphp_set_vars([
    'status' => BgTestStatus::Active,
    'RESULTS' => implode(',', $results),
]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
