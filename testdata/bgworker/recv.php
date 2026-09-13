<?php

// Long-lived bg worker parking in stream_socket_recvfrom(), a blocking
// receive that reaches the stream through its transport rather than the
// read op: it returns on the EOF of a drain too.
set_time_limit(0);
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
frankenphp_worker_tick();
stream_socket_recvfrom(frankenphp_get_worker_handle(), 1);
