<?php

// Records what the tick returns: true twice while running, then false
// twice once drained, the second one proving the drain sticks.
$handle = new \FrankenPHP\WorkerHandle();
$seen = [];
$seen[] = $handle->tick() ? 'true' : 'false';
$seen[] = $handle->tick() ? 'true' : 'false';
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));

$read = [$handle->getStream()];
$write = $except = null;
stream_select($read, $write, $except, null);

$seen[] = $handle->tick() ? 'true' : 'false';
$seen[] = $handle->tick() ? 'true' : 'false';
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));
