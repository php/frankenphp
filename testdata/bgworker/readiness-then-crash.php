<?php

// Signals readiness via frankenphp_get_worker_handle(), then exits
// non-zero. Drives the post-readiness crash-loop abort test.
set_time_limit(0);

frankenphp_get_worker_handle();
exit(1);
