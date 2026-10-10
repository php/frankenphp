<?php

require_once __DIR__.'/_executor.php';

return function () {
    foreach (['pcntl_fork', 'pcntl_exec', 'pcntl_waitpid', 'posix_kill', 'posix_getpid'] as $function) {
        if (!function_exists($function)) {
            echo "pcntl-unavailable";
            return;
        }
    }

    $marker = tempnam(sys_get_temp_dir(), 'frankenphp-double-fork-');
    unlink($marker);

    $pid = pcntl_fork();
    if ($pid === -1) {
        echo "fork-failed";
        return;
    }
    if ($pid === 0) {
        if (pcntl_fork() === 0) {
            pcntl_exec('/bin/sh', ['-c', 'sleep 0.2; touch "$1"', 'sh', $marker]);
        }
        // exit without PHP or Go teardown, like a daemonizer's intermediate process
        posix_kill(posix_getpid(), SIGKILL);
    }

    pcntl_waitpid($pid, $status);

    $deadline = microtime(true) + 3;
    do {
        clearstatcache();
        if (file_exists($marker)) {
            unlink($marker);
            echo "grandchild-alive";
            return;
        }
        usleep(10000);
    } while (microtime(true) < $deadline);

    echo "grandchild-killed";
};
