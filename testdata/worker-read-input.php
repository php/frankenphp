<?php

$count = 0;
$finally = 0;
$handler = static function () use (&$count, &$finally) {
    $count++;
    try {
        $read = isset($_GET['ignore-body']) ? 0 : strlen(file_get_contents('php://input'));
        echo "count=$count finally=$finally read=$read";
    } finally {
        $finally++;
    }
};

while (frankenphp_handle_request($handler)) {
}
