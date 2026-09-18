<?php

// Long-lived background worker. FrankenPHP disables max_execution_time for
// background runs, see TestBackgroundWorkerParkingIsNotInterrupted, so the
// script does not have to.

// Touch the sentinel so the test can confirm the worker actually ran.
if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}

// Ready, then park on the handle until FrankenPHP drains us: the drain
// closes the other end of the socket pair, which lands as EOF here and
// makes the next tick return false.
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
