<?php

require_once __DIR__.'/_executor.php';

frankenphp_log("default level message");

return function () {
	frankenphp_log("some debug message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_DEBUG, [
		"key int"    => 1,
	]);

	frankenphp_log("some info message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_INFO, [
		"key string" => "string",
	]);

	frankenphp_log("some warn message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_WARN);

	frankenphp_log("some error message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_ERROR, [
		"err" => ["a", "v"],
	]);
};
