<?php

// Bg worker processing tasks. Each task sent wakes a thread through its
// handle, the tick consumes the wake-up and receive_task() dequeues the
// tasks, null when another thread got there first. The payload drives the
// answer: input echoes back in the last update, steps sends that many
// progress updates first, sleep_ms simulates that much work, cut short if
// the sender closes its stream meanwhile, mark touches a file at pickup,
// crash exits without completing the task. Exceptions land in BG_SENTINEL.
// BG_LOOP=if takes one task per wake-up instead of draining the queue.
set_time_limit(0);
$drain = 'if' !== ($_SERVER['BG_LOOP'] ?? 'while');
$handle = frankenphp_get_worker_handle();
while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
    while ($task = frankenphp_receive_task()) {
        [$stream, $payload] = $task;
        if (!empty($payload['crash'])) {
            exit(1);
        }
        try {
            if (!empty($payload['mark'])) {
                touch($payload['mark']);
            }
            if (!empty($payload['sleep_ms'])) {
                $read = [$stream];
                $write = $except = null;
                if (stream_select($read, $write, $except, intdiv($payload['sleep_ms'], 1000), 1000 * ($payload['sleep_ms'] % 1000)) > 0 && feof($stream)) {
                    throw new \RuntimeException('the sender closed the task before the update');
                }
            }
            for ($i = 1, $steps = $payload['steps'] ?? 0; $i <= $steps; ++$i) {
                frankenphp_update_task($stream, ['step' => $i, 'of' => $steps]);
            }
            frankenphp_update_task($stream, [
                'result' => 'processed:' . ($payload['input'] ?? ''),
                'worker' => $_SERVER['FRANKENPHP_WORKER_BACKGROUND'],
                'tag' => $_SERVER['BG_TAG'] ?? '',
                'thread' => $threadId ??= bin2hex(random_bytes(4)),
            ]);
        } catch (\Throwable $e) {
            if (!empty($_SERVER['BG_SENTINEL'])) {
                file_put_contents($_SERVER['BG_SENTINEL'], get_class($e) . ': ' . $e->getMessage());
            }
        } finally {
            fclose($stream);
        }
        if (!$drain) {
            break;
        }
    }
}
