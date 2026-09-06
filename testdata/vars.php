<?php

try {
    echo json_encode(frankenphp_get_vars($_GET['name'] ?? 'publisher'));
} catch (\Throwable $e) {
    echo get_class($e) . ': ' . $e->getMessage();
}
