<?php

// Works for 150ms, then returns: a worker that processes a batch and lets
// FrankenPHP re-run it, which must not be throttled. Records when each run
// started, so a test can see whether the interval between them grows.
file_put_contents($_SERVER['BG_COUNT_FILE'], microtime(true) . "\n", FILE_APPEND);
(new \FrankenPHP\WorkerHandle())->tick();
usleep(150000);
