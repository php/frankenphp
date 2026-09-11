<?php

// Long-lived background worker. FrankenPHP disables max_execution_time for
// background runs, see TestBackgroundWorkerHasNoExecutionTimeout, so the
// script does not have to.

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
