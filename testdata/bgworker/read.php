<?php

// Long-lived bg worker parking with a blocking read instead of
// stream_select(): the read returns on the EOF of a drain too.
set_time_limit(0);
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
frankenphp_worker_tick();
fgets(frankenphp_get_worker_handle());
