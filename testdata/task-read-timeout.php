<?php

// read() waits for the next update whatever the stream of the task was told:
// a wait that must end goes through stream_select() or a poll context.
try {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'x', 'sleep_ms' => 200]);
    stream_set_timeout($task->getStream(), 0, 10_000);
    while (null !== $update = $task->read()) {
        echo $update['result'], "\n";
    }
    echo 'done';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
