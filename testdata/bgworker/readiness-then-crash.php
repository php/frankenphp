<?php

// Signals readiness via frankenphp_set_vars() + frankenphp_get_worker_handle()
// (both are required to close the readiness channel on sidekicks), then
// exits non-zero. Drives the post-readiness crash-loop abort test.
set_time_limit(0);

frankenphp_set_vars([]);
frankenphp_get_worker_handle();
exit(1);
