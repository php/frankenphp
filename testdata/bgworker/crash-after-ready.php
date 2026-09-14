<?php

// Reaches the ready point, the first frankenphp_worker_tick(), then exits
// non-zero: a crash after readiness restarts with the backoff and does not
// count toward max_consecutive_failures. Runs are counted in BG_COUNT_FILE.
set_time_limit(0);
file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
frankenphp_worker_tick();
exit(1);
