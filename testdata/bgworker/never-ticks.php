<?php

// Bg worker that parks on its handle without ever calling tick(): it eats
// the wake-up sent at start, then waits for the drain, so only the boot
// timeout can end the wait Init() is in.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
fgets($stream);
fgets($stream);
