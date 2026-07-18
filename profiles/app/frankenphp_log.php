<?php
header('Content-Type: text/plain; charset=utf-8');

if (function_exists('frankenphp_log')) {
    frankenphp_log('pgo profile request', FRANKENPHP_LOG_LEVEL_DEBUG, ['s' => 'fp_log']);
    frankenphp_log('pgo profile request', FRANKENPHP_LOG_LEVEL_INFO);
} else {
    error_log('frankenphp_log unavailable');
}

echo "ok\n";
