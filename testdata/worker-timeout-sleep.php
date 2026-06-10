<?php

// Worker used by the worker-timeout tests (see workertimeout_internal_test.go).
//
// The handler touches a marker file (path passed via the Sleep-Marker header)
// right before sleeping, so the Go test can prove the worker is parked inside
// sleep() before relying on the timeout watchdog instead of racing a fixed
// time.Sleep. It then sleeps for the number of seconds given by the
// Sleep-Seconds header (default 60). The "completed" output after the sleep
// lets a test assert whether the request was interrupted or returned naturally.
//
// A persistent counter is echoed on every request so a test can prove the
// worker is healthy again (and which script instance is serving) after a
// timeout-induced restart.

$instance = bin2hex(random_bytes(4));
$count = 0;

$fn = static function () use ($instance, &$count): void {
    $count++;

    $marker = $_SERVER['HTTP_SLEEP_MARKER'] ?? '';
    if ($marker !== '') {
        touch($marker);
    }

    $seconds = (int) ($_SERVER['HTTP_SLEEP_SECONDS'] ?? 60);
    if ($seconds > 0) {
        sleep($seconds);
    }

    echo "instance:$instance,count:$count,completed";
};

do {
    $ret = \frankenphp_handle_request($fn);
} while ($ret);
