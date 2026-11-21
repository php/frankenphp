<?php

// Sends two slow tasks and follows both through an Io\Poll\Context: the
// handles go in as they are, no stream in sight.
use Io\Poll\{Context, Event};

try {
    $poll = new Context();
    $tasks = [];
    foreach (['a', 'b'] as $input) {
        $task = new \FrankenPHP\SentTaskHandle('pool', ['input' => $input, 'sleep_ms' => 200]);
        $tasks[spl_object_id($task)] = $task;
        $poll->add($task, [Event::Read], spl_object_id($task));
    }
    $results = [];
    $resources = count(get_resources());
    while ($tasks) {
        foreach ($poll->wait() as $watcher) {
            if (null === $update = $tasks[$watcher->getData()]->read()) {
                $watcher->remove();
                unset($tasks[$watcher->getData()]);
                continue;
            }
            $results[] = $update['result'];
        }
    }
    sort($results);
    // waiting through the context builds no stream and no resource
    echo json_encode($results), ' ', count(get_resources()) - $resources;
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
