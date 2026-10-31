<?php

// Bg worker publishing a value outside the whitelist, then in a namespace it
// does not declare, writing the exceptions to BG_SENTINEL, then publishing
// its own namespace and parking.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$namespace = $handle->getNamespaces()['vars'][0];
$errors = [];
try {
    $handle->setVars($namespace, ['object' => new stdClass()]);
} catch (\Throwable $e) {
    $errors[] = get_class($e) . ': ' . $e->getMessage();
}
try {
    $handle->setVars('undeclared', []);
} catch (\Throwable $e) {
    $errors[] = get_class($e) . ': ' . $e->getMessage();
}
file_put_contents($_SERVER['BG_SENTINEL'], implode("\n", $errors));
$handle->setVars($namespace, []);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
