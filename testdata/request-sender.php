<?php

$message = $_GET["message"];
$workerName = $_GET["workerName"] ?? '';

frankenphp_send_request($message, $workerName);

echo "request sent";
