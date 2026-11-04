<?php

// The socket stays open for the lifetime of the PHP thread, like a persistent PDO connection.
$socket = pfsockopen('127.0.0.1', (int) $_GET['port'], $errorCode, $errorMessage, 5);
if ($socket === false) {
    http_response_code(500);
    echo "pfsockopen failed: $errorMessage";

    return;
}

echo 'ok';
