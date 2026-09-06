<?php

// Long-lived bg worker parking with a blocking read instead of
// stream_select(): it counts as the wait that marks the worker ready too,
// and returns on the EOF of a drain.
set_time_limit(0);
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
$stream = frankenphp_get_worker_handle();
fgets($stream);
