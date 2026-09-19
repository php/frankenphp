<?php

// Bg worker completing each task, then calling every method of the handle
// again: the outcomes land in BG_SENTINEL, one line per call.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
    while ($task = $handle->receive()) {
        $task->complete(['result' => 'over']);
        $lines = [];
        foreach ([
            'getPayload' => fn () => json_encode($task->getPayload()),
            'update' => fn () => $task->update(['late' => true]),
            'complete' => fn () => $task->complete(),
            'complete-data' => fn () => $task->complete(['late' => true]),
            'getStream' => fn () => get_debug_type($task->getStream()),
        ] as $name => $case) {
            try {
                $result = $case();
                $lines[] = $name . ': ' . (null === $result ? 'ok' : $result);
            } catch (\Throwable $e) {
                $lines[] = $name . ': ' . get_class($e) . ': ' . $e->getMessage();
            }
        }
        file_put_contents($_SERVER['BG_SENTINEL'], implode("\n", $lines));
    }
}
