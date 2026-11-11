<?php

// Bg worker processing tasks. Each task sent wakes a thread through its
// handle, the tick consumes the wake-up and receive() dequeues the tasks,
// null when another thread got there first. The payload drives the answer:
// input echoes back in the last update, steps sends that many progress
// updates first, sleep_ms simulates that much work, cut short if the sender
// gives up meanwhile, pause_ms that much work no matter what, hold_ms that
// much once the updates are sent, mark touches a file at pickup, crash exits
// without completing the task, drop lets go of it without exiting, abort does
// so while keeping its handle, close_stream closes the stream of the task
// before completing it.
// Exceptions land in BG_SENTINEL. BG_LOOP=if takes one task per wake-up
// instead of draining the queue, and BG_LOOP=callback ticks once woken rather
// than before waiting, as the callback of an event loop does: nothing ticks
// between the last receive() and the next wait.
set_time_limit(0);
$drain = 'if' !== ($_SERVER['BG_LOOP'] ?? 'while');
$callback = 'callback' === ($_SERVER['BG_LOOP'] ?? 'while');
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($callback || $handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
    if ($callback && !$handle->tick()) {
        break;
    }
    while ($task = $handle->receive()) {
        $payload = $task->getPayload();
        if (!empty($payload['crash'])) {
            exit(1);
        }
        if (!empty($payload['drop'])) {
            // the next receive() replaces the handle, which aborts the task
            continue;
        }
        if (!empty($payload['abort'])) {
            // the handle stays, the task does not wait for it to go
            $aborted = $task;
            $task->abort();
            continue;
        }
        try {
            if (!empty($payload['mark'])) {
                touch($payload['mark']);
            }
            if (!empty($payload['close_stream'])) {
                // the task does not go with its stream
                fclose($task->getStream());
            }
            if (!empty($payload['sleep_ms'])) {
                $read = [$task->getStream()];
                $write = $except = null;
                if (stream_select($read, $write, $except, intdiv($payload['sleep_ms'], 1000), 1000 * ($payload['sleep_ms'] % 1000)) > 0 && feof($task->getStream())) {
                    throw new \RuntimeException('the sender closed the task before the update');
                }
            }
            if (!empty($payload['pause_ms'])) {
                usleep(1000 * $payload['pause_ms']);
            }
            for ($i = 1, $steps = $payload['steps'] ?? 0; $i <= $steps; ++$i) {
                if (!$task->update(['step' => $i, 'of' => $steps])) {
                    throw new \RuntimeException("update() returned false at step $i");
                }
            }
            if (!empty($payload['hold_ms'])) {
                usleep(1000 * $payload['hold_ms']);
            }
            $task->complete([
                'result' => 'processed:' . ($payload['input'] ?? ''),
                'worker' => $handle->getName(),
                'namespace' => $task->getNamespace(),
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
