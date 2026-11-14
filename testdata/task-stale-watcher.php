<?php

// A watcher left in a context past the end of its task stays quiet: the
// descriptor it holds is closed with the task, not handed to the next one,
// whose signals it would get.
use Io\Poll\{Context, Event};
use Time\Duration;

try {
    // keeps the only thread of the worker busy
    $busy = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'busy', 'sleep_ms' => 200]);
    $poll = new Context();
    $ended = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'ended']);
    $poll->add($ended, [Event::Read], 'ended');
    $ended->abandon();

    $next = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'next']);
    $read = [$next->getStream()];
    $write = $except = null;
    stream_select($read, $write, $except, 5);
    // the result of the next task is there, unread, and its descriptor readable
    $woken = array_map(static fn ($watcher) => $watcher->getData(), $poll->wait(Duration::fromMilliseconds(50)));
    echo json_encode($woken), ' ', $next->read()['result'];
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
