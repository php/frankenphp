<?php

// What the queue of a worker does while its only thread is busy with a slow
// task, which touches the mark file at pickup. case picks the scenario:
// - abandon: a task abandoned while it is queued never reaches the worker,
//   which would touch the never file at its pickup;
// - select: the timeout of a task waited on through stream_select() ends that
//   wait, and read() tells;
// - refused: a task refused on its timeout left the queue, so the thread that
//   becomes free before read() is called does not take it;
// - unwatched: the same when nothing waited on the task meanwhile.
// loose bounds below: the clock of hrtime() is not exactly the one the waits run on
$ms = static fn (int $since): int => (int) ((hrtime(true) - $since) / 1e6);
$drain = static function (\FrankenPHP\SentTaskHandle $task): ?array {
    $last = null;
    while (null !== $update = $task->read()) {
        $last = $update;
    }

    return $last;
};

try {
    $slow = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'slow', 'sleep_ms' => (int) ($_GET['sleep_ms'] ?? 300), 'mark' => $_GET['mark']]);
    for ($i = 0; !file_exists($_GET['mark']) && $i < 5000; ++$i) {
        usleep(1000);
    }

    switch ($_GET['case']) {
        case 'abandon':
            $queued = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'queued', 'mark' => $_GET['never']]);
            $queued->abandon();
            $drain($slow);
            echo $drain(new \FrankenPHP\SentTaskHandle('echo', ['input' => 'after']))['result'], "\n";
            echo file_exists($_GET['never']) ? 'processed' : 'never processed';
            break;

        case 'refused':
        case 'unwatched':
            $late = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'late', 'mark' => $_GET['never']], 0.1);
            if ('refused' === $_GET['case']) {
                $read = [$late->getStream()];
                $write = $except = null;
                stream_select($read, $write, $except, 5);
            }
            // the thread is free from here on, past the timeout of the task
            $drain($slow);
            usleep(50_000);
            $start = hrtime(true);
            try {
                $late->read();
                echo "no timeout\n";
            } catch (\RuntimeException $e) {
                echo $ms($start) < 50 ? 'at once' : 'after a wait', ': ', $e->getMessage(), "\n";
            }
            echo file_exists($_GET['never']) ? 'processed' : 'never processed';
            break;

        case 'select':
            $late = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'late'], 0.1);
            $start = hrtime(true);
            $read = [$late->getStream()];
            $write = $except = null;
            $ready = stream_select($read, $write, $except, 5);
            $waited = $ms($start);
            echo $ready, ' ready ', $waited >= 70 && $waited < 2000 ? 'at the timeout' : "after {$waited}ms", "\n";
            try {
                $late->read();
                echo 'no timeout';
            } catch (\RuntimeException $e) {
                echo $e->getMessage();
            }
            $drain($slow);
            break;
    }
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
