<?php

// Long-running CLI script that uses pcntl signals,
// simulating queue processors like Laravel Horizon or Symfony Messenger.

if (!extension_loaded('pcntl')) {
    fwrite(STDERR, "pcntl extension required\n");
    exit(2);
}

pcntl_async_signals(true);

$received = ['SIGUSR1' => 0, 'SIGUSR2' => 0, 'SIGALRM' => 0];

pcntl_signal(SIGUSR1, function () use (&$received) { $received['SIGUSR1']++; });
pcntl_signal(SIGUSR2, function () use (&$received) { $received['SIGUSR2']++; });
pcntl_signal(SIGALRM, function () use (&$received) { $received['SIGALRM']++; });

$pid = getmypid();
$attempts = 0;
$deadline = microtime(true) + 1.5;
while (microtime(true) < $deadline) {
    posix_kill($pid, SIGUSR1);
    posix_kill($pid, SIGUSR2);
    $attempts += 2;
    usleep(500);
}

pcntl_alarm(1);
$alarmDeadline = microtime(true) + 1.5;
while (microtime(true) < $alarmDeadline && $received['SIGALRM'] === 0) {
    usleep(1000);
}

if ($received['SIGUSR1'] === 0 || $received['SIGUSR2'] === 0) {
    fwrite(STDERR, "missed user signals: " . json_encode($received) . " of $attempts attempts\n");
    exit(1);
}
if ($received['SIGALRM'] === 0) {
    fwrite(STDERR, "missed SIGALRM from pcntl_alarm\n");
    exit(1);
}

echo "ok\n";
exit(0);
