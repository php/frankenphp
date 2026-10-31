<?php

// Publishes an enum, then republishes it every 10ms, freeing the previous
// snapshot each time.
require __DIR__.'/../enum-suit.php';
$handle = new \FrankenPHP\WorkerHandle();
$publications = 0;
$handle->setVars('enum', ['suit' => EnumSuit::Hearts, 'n' => ++$publications]);
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, 0, 10000);
    $handle->setVars('enum', ['suit' => EnumSuit::Hearts, 'n' => ++$publications]);
}
