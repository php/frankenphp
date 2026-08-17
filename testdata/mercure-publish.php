<?php

require_once __DIR__.'/_executor.php';

return function () {
	echo "update 1: " . mercure_publish('foo', 'bar', true, 'myid', 'mytype', 10) . "\n";
	echo "update 2: " . mercure_publish(['baz', 'bar']) . "\n";

	try {
		mercure_publish('/.well-known/mercure/subscriptions');
	} catch (ValueError $e) {
		echo "error 1: " . $e->getMessage() . "\n";
	}

	try {
		mercure_publish('foo', retry: -1);
	} catch (ValueError $e) {
		echo "error 2: " . $e->getMessage() . "\n";
	}
};
