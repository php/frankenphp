<?php

// Broken background worker: takes a handle but returns without ever
// ticking it. Taking one is not the ready point, the tick is, so this
// counts as a boot failure like early-return.php.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
