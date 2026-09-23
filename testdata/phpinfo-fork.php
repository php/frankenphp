<?php

require_once __DIR__.'/_executor.php';

return function () {
    foreach (['pcntl_fork', 'pcntl_waitpid', 'posix_kill'] as $function) {
        if (!function_exists($function)) {
            echo "pcntl-unavailable";
            return;
        }
    }

    $pid = pcntl_fork();
    if ($pid === -1) {
        echo "fork-failed";
        return;
    }
    if ($pid === 0) {
        ob_start();
        phpinfo();
        $output = ob_get_clean();
        exit(str_contains($output, '<td class="e">frankenphp </td>') ? 3 : 0);
    }

    $deadline = microtime(true) + 3;
    do {
        $waited = pcntl_waitpid($pid, $status, WNOHANG);
        if ($waited === $pid) {
            echo pcntl_wifexited($status) && pcntl_wexitstatus($status) === 0
                ? "child-safe"
                : "child-unsafe";
            return;
        }
        usleep(1000);
    } while (microtime(true) < $deadline);

    posix_kill($pid, SIGKILL);
    pcntl_waitpid($pid, $status);
    echo "child-timeout";
};
