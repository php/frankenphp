<?php

ignore_user_abort(true);

require_once __DIR__.'/_executor.php';

return function () {
	if($_GET['finish'] ?? false) {
		frankenphp_finish_request();
	}

	echo 'hi';
	flush();
	$status = (string) connection_status();
	error_log("request {$_GET['i']}: " . $status);
};
