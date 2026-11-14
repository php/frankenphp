<?php

// Closing the stream of a task leaves the task alone, on both sides: the
// handle hands out a fresh stream, over the descriptor the first one waited
// on, and the result still comes.
try {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'x', 'close_stream' => 1]);
    $first = $task->getStream();
    $read = [$first];
    $write = $except = null;
    stream_select($read, $write, $except, 0, 0);
    fclose($first);
    $second = $task->getStream();
    echo get_debug_type($first), ', ', get_debug_type($second), "\n";

    $read = [$second];
    echo stream_select($read, $write, $except, 5), " ready\n";
    while (null !== $update = $task->read()) {
        echo $update['result'], "\n";
    }
    echo var_export(feof($second), true), "\n";
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
