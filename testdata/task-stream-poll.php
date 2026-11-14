<?php

// Follows a task through a poller that takes the descriptor of its stream
// once and keeps it, as an event loop extension does: every update has to
// make it readable, without the script asking for the stream again.
use Io\Poll\{Context, Event};
use Time\Duration;

try {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'x', 'steps' => 3, 'sleep_ms' => 20]);
    $poll = new Context();
    $poll->add(new StreamPollHandle($task->getStream()), [Event::Read]);
    $events = 0;
    while ($poll->wait(Duration::fromMilliseconds(2000))) {
        ++$events;
        if (null === $task->read()) {
            echo "done after $events events";

            return;
        }
    }
    echo "missed a wake-up after $events events";
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
