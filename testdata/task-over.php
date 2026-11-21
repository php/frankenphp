<?php

// Calls every method of a sent task once it is complete, then once the
// handle is given up: one line per call.
$probe = function (string $phase, \FrankenPHP\SentTaskHandle $task) {
    foreach ([
        'read' => fn () => $task->read(),
        'getStream' => fn () => get_debug_type($task->getStream()),
        'abandon' => fn () => $task->abandon(),
    ] as $name => $case) {
        try {
            $result = $case();
            echo $phase, ' ', $name, ': ', null === $result ? 'ok' : $result, "\n";
        } catch (\Throwable $e) {
            echo $phase, ' ', $name, ': ', get_class($e), ': ', $e->getMessage(), "\n";
        }
    }
};

$task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'over']);
while (null !== $task->read()) {
}
$probe('completed', $task);
$probe('abandoned', $task);
