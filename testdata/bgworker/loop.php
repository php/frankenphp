<?php

// Long-lived bg worker that never ticks on its own initiative: its loop
// only ticks once the handle is readable, the shape of a script driven by
// an event loop. The wake-up FrankenPHP sends at start is what makes it
// ready.
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
do {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
} while ($handle->tick());
