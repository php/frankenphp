<?php

// Long-lived bg worker that does NOT call set_time_limit(0): parking past
// max_execution_time must not kill it, FrankenPHP disables the limit for
// background runs. Counts its runs in BG_COUNT_FILE, so a run cut short
// shows up as a second line.
file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
$stream = frankenphp_get_worker_handle();
fgets($stream);
