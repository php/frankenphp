<?php

require_once __DIR__.'/_executor.php';

return function () {
	echo "update 1: " . mercure_publish('foo', 'bar', true, 'myid', 'mytype', 10) . PHP_EOL;
	echo "update 2: " . mercure_publish(['baz', 'bar']) . PHP_EOL;
};
