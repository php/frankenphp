<?php

// The stream of a task kept past its handle: the task went with the handle,
// so the stream is closed and waiting on it reaches nothing.
$task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'x']);
$stream = $task->getStream();
while (null !== $task->read()) {
}
unset($task);
echo is_resource($stream) ? 'open' : 'closed', "\n";

// the same with handles overwritten in a loop, before the tasks are done
$streams = [];
foreach (['a', 'b'] as $input) {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => $input, 'sleep_ms' => 50]);
    $streams[] = $task->getStream();
}
unset($task);
echo implode(',', array_map(fn ($s) => is_resource($s) ? 'open' : 'closed', $streams)), "\n";
