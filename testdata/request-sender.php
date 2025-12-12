<?php

$message = $_GET["message"];
$workerName = $_GET["workerName"] ?? '';

frankenphp_send_request($message, $workerName);

// sleep to make sure request was received
// TODO: solve this test-restart race condition with Futures instead?
usleep(10_000);

echo "request sent";
