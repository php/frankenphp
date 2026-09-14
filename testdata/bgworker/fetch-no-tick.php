<?php

// Broken background worker: fetches its handle but returns without ever
// calling frankenphp_worker_tick(). Fetching is not the ready point, the
// tick is, so this counts as a boot failure like early-return.php.
set_time_limit(0);
$stream = frankenphp_get_worker_handle();
