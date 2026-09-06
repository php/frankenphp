<?php

// Long-lived background worker: disable PHP max_execution_time so the
// 30s default cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Touch the sentinel so the test can confirm the worker actually ran.
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}

// Park on the handle until FrankenPHP drains us (drain closes the other
// end of the socket pair, which lands as EOF here).
$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
