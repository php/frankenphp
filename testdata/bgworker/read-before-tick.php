<?php

// Reads the stream of its handle twice before ticking: the first read takes
// the wake-up sent at start, the second would block the start for good.
// The exception lands in BG_SENTINEL.
$stream = (new \FrankenPHP\WorkerHandle())->getStream();
fgets($stream);
try {
    fgets($stream);
} catch (\LogicException $e) {
    file_put_contents($_SERVER['BG_SENTINEL'], $e->getMessage());

    throw $e;
}
