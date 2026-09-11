<?php

// Long-lived bg worker parking in stream_socket_recvfrom(), a blocking
// receive that reaches the stream through its transport rather than the
// read op: it counts as the wait that marks the worker ready, and returns
// on the EOF of a drain.
set_time_limit(0);
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
$stream = frankenphp_get_worker_handle();
stream_socket_recvfrom($stream, 1);
