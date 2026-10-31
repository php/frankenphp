<?php

// Bg worker publishing every vars namespace it declares, each snapshot naming
// its namespace and listing them all, then parking.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$namespaces = $handle->getNamespaces();
foreach ($namespaces['vars'] as $namespace) {
    $handle->setVars($namespace, ['namespace' => $namespace, 'all' => $namespaces]);
}
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
