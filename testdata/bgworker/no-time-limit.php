<?php

// Long-lived bg worker relying on the engine for its parking: it does not
// call set_time_limit(0), so max_execution_time must not cut the run
// short, and it does not set a stream timeout, so default_socket_timeout
// must not end the read either. Counts its runs in BG_COUNT_FILE, so
// anything that interrupts the park shows up as a second line.
file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
$stream = frankenphp_get_worker_handle();
fgets($stream);
