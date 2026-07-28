<?php

// Worker that sleeps inside the handler to simulate a stuck request blocking
// drain. Used to test the force-kill grace period.
//
// Before sleeping we touch a marker file whose path is passed via the
// SLEEP_MARKER header. The Go test polls for the file so it only arms
// RestartWorkers once the worker is proven to be inside sleep(), removing
// the fixed-time race of a bare time.Sleep on the caller side.
$fn = static function () {
    $marker = $_SERVER['HTTP_SLEEP_MARKER'] ?? '';
    if ($marker !== '') {
        touch($marker);
    }
    sleep(60);
    echo 'should not reach';
};

do {
    $ret = \frankenphp_handle_request($fn);
} while ($ret);
