<?php

// Bg worker processing tasks. Each task sent wakes a thread through its
// handle, the tick consumes the wake-up and receive() dequeues the tasks,
// null when another thread got there first. The payload drives the answer:
// input echoes back in the last update, steps sends that many progress
// updates first, sleep_ms simulates that much work, cut short if the sender
// closes its stream meanwhile, mark touches a file at pickup, crash exits
// without completing the task. Exceptions land in BG_SENTINEL. BG_LOOP=if
// takes one task per wake-up instead of draining the queue.
set_time_limit(0);
$drain = 'if' !== ($_SERVER['BG_LOOP'] ?? 'while');
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
    while ($task = $handle->receive()) {
        $payload = $task->getPayload();
        if (!empty($payload['crash'])) {
            exit(1);
        }
        try {
            if (!empty($payload['mark'])) {
                touch($payload['mark']);
            }
            if (!empty($payload['sleep_ms'])) {
                $read = [$task->getStream()];
                $write = $except = null;
                if (stream_select($read, $write, $except, intdiv($payload['sleep_ms'], 1000), 1000 * ($payload['sleep_ms'] % 1000)) > 0 && feof($task->getStream())) {
                    throw new \RuntimeException('the sender closed the task before the update');
                }
            }
            for ($i = 1, $steps = $payload['steps'] ?? 0; $i <= $steps; ++$i) {
                $task->update(['step' => $i, 'of' => $steps]);
            }
            $task->complete([
                'result' => 'processed:' . ($payload['input'] ?? ''),
                'worker' => $_SERVER['FRANKENPHP_WORKER_BACKGROUND'],
                'tag' => $_SERVER['BG_TAG'] ?? '',
                'thread' => $threadId ??= bin2hex(random_bytes(4)),
            ]);
        } catch (\Throwable $e) {
            if (!empty($_SERVER['BG_SENTINEL'])) {
                file_put_contents($_SERVER['BG_SENTINEL'], get_class($e) . ': ' . $e->getMessage());
            }
            $task->complete();
        }
        if (!$drain) {
            break;
        }
    }
}
