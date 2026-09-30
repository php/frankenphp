<?php

require_once __DIR__.'/_executor.php';

return function () {
	try {
		mercure_publish('foo');
	} catch (RuntimeException $e) {
		echo "error: " . $e->getMessage() . "\n";
	}
};
