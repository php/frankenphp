<?php

// Broken background worker: fetches its handle but returns without ever
// waiting on it. Fetching is not the ready point, waiting is, so this counts
// as a boot failure like early-return.php.
set_time_limit(0);
$stream = frankenphp_get_worker_handle();
