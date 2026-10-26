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

	// an integer key holding null used to abort the whole process
	frankenphp_log("some null message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_INFO, [
		8 => null,
	]);

	// a cyclic array used to overflow the Go stack and abort the whole process
	$cyclic = ['tag' => 'cyclic'];
	$cyclic['self'] = &$cyclic;
	try {
		frankenphp_log("some cyclic message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_INFO, $cyclic);
	} catch (RuntimeException $e) {
		frankenphp_log("cyclic rejected {$_GET['i']}", FRANKENPHP_LOG_LEVEL_INFO, ['err' => $e->getMessage()]);
	}

	$shared = ['a', 'b'];
	frankenphp_log("some shared message {$_GET['i']}", FRANKENPHP_LOG_LEVEL_INFO, [
		'first'  => $shared,
		'second' => $shared,
	]);
};
