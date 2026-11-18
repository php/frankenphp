#!/bin/bash
for ((i = 0; i < 100; i++)); do
	curl --no-progress-meter -o /dev/null http://localhost:2019/config/apps/frankenphp -: --no-progress-meter -o /dev/null -H 'Cache-Control: must-revalidate' -H 'Content-Type: application/json' --data-binary '{"workers":[{"file_name":"./index.php"}]}' -X PATCH http://localhost:2019/config/apps/frankenphp
done
