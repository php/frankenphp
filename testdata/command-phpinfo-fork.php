<?php

if (!function_exists('pcntl_fork') || !function_exists('pcntl_waitpid')) {
    fwrite(STDERR, "pcntl unavailable\n");
    exit(2);
}

$pid = pcntl_fork();
if ($pid === -1) {
    fwrite(STDERR, "fork failed\n");
    exit(1);
}
if ($pid === 0) {
    ob_start();
    phpinfo();
    $output = ob_get_clean();
    fwrite(STDERR, str_contains($output, "\nfrankenphp => ") ? "child-frankenphp\n" : "child-safe\n");
    exit(0);
}

$waited = pcntl_waitpid($pid, $status);
if ($waited !== $pid || !pcntl_wifexited($status) || pcntl_wexitstatus($status) !== 0) {
    fwrite(STDERR, "child failed\n");
    exit(1);
}

fwrite(STDERR, "parent-ok\n");
