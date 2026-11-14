<?php

// A watcher left in a context on a task abandoned with an update unread stays
// quiet, though the worker still holds the task and its descriptor is only
// closed once both sides are done with it.
use Io\Poll\{Context, Event};
use Time\Duration;

try {
    $poll = new Context();
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'held', 'steps' => 1, 'hold_ms' => 300]);
    $poll->add($task, [Event::Read], 'task');
    // the update is there
    $ready = count($poll->wait(Duration::fromMilliseconds(5000)));
    $task->abandon();
    echo $ready, ' ready, then ', count($poll->wait(Duration::fromMilliseconds(50)));
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
