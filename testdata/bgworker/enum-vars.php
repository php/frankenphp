<?php

// Publishes an enum in its namespace, the case BG_ENUM names when set, then
// republishes it every 10ms, freeing the previous snapshot each time.
require __DIR__.'/../enum-suit.php';
$handle = new \FrankenPHP\WorkerHandle();
$namespace = $handle->getNamespaces()['vars'][0];
$suit = constant($_SERVER['BG_ENUM'] ?? 'EnumSuit::Hearts');
$publications = 0;
$handle->setVars($namespace, ['suit' => $suit, 'n' => ++$publications]);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, 0, 10000);
    $handle->setVars($namespace, ['suit' => $suit, 'n' => ++$publications]);
}
