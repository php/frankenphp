<?php

// Bg worker publishing a new snapshot every 10 milliseconds, its tick
// counting them.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$namespace = $handle->getNamespaces()['vars'][0];
$tick = 0;
$handle->setVars($namespace, ['tick' => $tick]);
while ($handle->tick(0.01)) {
    $handle->setVars($namespace, ['tick' => ++$tick]);
}
