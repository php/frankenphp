<?php

// The constructor refuses to build a handle outside a background worker.
// unserialize() and Reflection do not call it, so they must be refused as
// well, and the methods must stand on their own: reaching the Go side
// from a request thread panics and takes the process with it.
$report = static function (string $what, callable $attempt): void {
    try {
        $result = $attempt();
        echo $what, ': ', get_debug_type($result), "\n";
    } catch (\Throwable $e) {
        echo $what, ': ', get_class($e), ': ', $e->getMessage(), "\n";
    }
};

$report('unserialize', static fn () => unserialize('O:23:"FrankenPHP\WorkerHandle":0:{}'));
$report('reflection', static fn () => (new ReflectionClass(\FrankenPHP\WorkerHandle::class))->newInstanceWithoutConstructor());
$report('construct', static fn () => new \FrankenPHP\WorkerHandle());
