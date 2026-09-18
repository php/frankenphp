<?php

// Long-lived bg worker parking in stream_socket_recvfrom(), a blocking
// receive that reaches the stream through its transport rather than the
// read op: it returns on the EOF of a drain too.
set_time_limit(0);
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}
$handle = new \FrankenPHP\WorkerHandle();
$handle->tick();
stream_socket_recvfrom($handle->getStream(), 1);
