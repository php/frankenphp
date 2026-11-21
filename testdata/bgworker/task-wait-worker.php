<?php

// Bg worker waiting for its tasks in tick() rather than on its stream: until
// one comes or the drain by default, BG_TICK_TIMEOUT seconds at a time when
// set, counting the waits that ended without a task. The answer carries what
// the payload asked to process, like task-worker.php. BG_SENTINEL gets a line
// once the loop ended, on the drain.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$timeout = isset($_SERVER['BG_TICK_TIMEOUT']) ? (float) $_SERVER['BG_TICK_TIMEOUT'] : null;
$idle = 0;
while ($handle->tick($timeout)) {
    for ($tasks = 0; $task = $handle->receive(); ++$tasks) {
        $payload = $task->getPayload();
        if (!empty($payload['sleep_ms'])) {
            usleep(1000 * $payload['sleep_ms']);
        }
        $task->complete([
            'result' => 'processed:' . ($payload['input'] ?? ''),
            'worker' => $handle->getName(),
            'thread' => $threadId ??= bin2hex(random_bytes(4)),
            'idle' => $idle,
        ]);
    }
    if (0 === $tasks) {
        ++$idle;
    }
}
if (!empty($_SERVER['BG_SENTINEL'])) {
    file_put_contents($_SERVER['BG_SENTINEL'], "drained after $idle waits without a task\n", FILE_APPEND);
}
