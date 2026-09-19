<?php

// The error paths of the task handles, from a request thread.
$cases = [
    'unknown' => fn () => new \FrankenPHP\SentTaskHandle('nope', []),
    'payload' => fn () => new \FrankenPHP\SentTaskHandle('echo', ['object' => new stdClass()]),
    'timeout' => fn () => new \FrankenPHP\SentTaskHandle('echo', [], -1),
    'received' => fn () => new \FrankenPHP\ReceivedTaskHandle(),
    'worker' => fn () => new \FrankenPHP\WorkerHandle(),
];
foreach ($cases as $name => $case) {
    try {
        $case();
        echo $name, ": no exception\n";
    } catch (\Throwable $e) {
        echo $name, ': ', get_class($e), ': ', $e->getMessage(), "\n";
    }
}
