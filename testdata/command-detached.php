<?php

foreach (['pcntl_fork', 'pcntl_exec', 'posix_setsid'] as $fn) {
    if (!function_exists($fn)) {
        fwrite(STDERR, "missing $fn (pcntl/posix not fully loaded)\n");
        exit(2);
    }
}

$ready = getenv('FRANKENPHP_TEST_DETACHED_READY');
if (!is_string($ready) || $ready === '' || $ready[0] !== '/'
    || !is_dir(dirname($ready)) || !is_writable(dirname($ready))
    || file_exists($ready) || is_link($ready)) {
    fwrite(STDERR, "readiness path must be an absolute, writable, initially absent path\n");
    exit(1);
}

$pid = pcntl_fork();
if ($pid === -1) {
    exit(1);
}
if ($pid === 0) {
    if (posix_setsid() === -1) {
        exit(1);
    }
    // Redirect after exec: the emulated CLI keeps process stdio open.
    // Keep stdin for the Go test's token, sent only after this CLI parent exits.
    pcntl_exec('/bin/sh', ['-c', 'exec >/dev/null 2>&1; set -C; printf ready > "$1" && IFS= read -r result && [ "$result" = survived ]', 'detached', $ready]);
    exit(1);
}

printf("CHILD=%d\n", $pid);
$deadline = microtime(true) + 5;
do {
    if (is_file($ready)) {
        exit(0);
    }
    usleep(1000);
} while (microtime(true) < $deadline);

fwrite(STDERR, "detached child did not exec within 5s\n");
exit(1);
